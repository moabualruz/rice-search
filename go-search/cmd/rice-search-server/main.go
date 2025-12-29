// Package main provides the Rice Search server binary.
// This server exposes gRPC and HTTP endpoints for search, indexing, and store management.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ricesearch/rice-search/internal/bus"
	"github.com/ricesearch/rice-search/internal/config"
	"github.com/ricesearch/rice-search/internal/grpcserver"
	"github.com/ricesearch/rice-search/internal/index"
	"github.com/ricesearch/rice-search/internal/ml"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
	"github.com/ricesearch/rice-search/internal/qdrant"
	"github.com/ricesearch/rice-search/internal/search"
	"github.com/ricesearch/rice-search/internal/store"
	"github.com/ricesearch/rice-search/internal/web"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "rice-search-server",
		Short: "Rice Search Server - gRPC + HTTP code search platform",
		Long: `Rice Search Server provides intelligent code search capabilities via gRPC and HTTP.

The server exposes:
  - gRPC API on :50051 (configurable) for CLI and programmatic access
  - HTTP API on :8080 (configurable) for REST and Web UI access
  - Unix socket (optional, non-Windows) for lowest latency local access

Examples:
  rice-search-server                           # Start with defaults
  rice-search-server --grpc-port 50051         # Custom gRPC port
  rice-search-server --http-port 8080          # Custom HTTP port
  rice-search-server --unix-socket /tmp/rs.sock # Enable Unix socket`,
		RunE:         runServer,
		SilenceUsage: true,
	}

	// Server flags
	rootCmd.Flags().StringP("config", "c", "", "config file path")
	rootCmd.Flags().BoolP("verbose", "v", false, "verbose logging")
	rootCmd.Flags().Int("grpc-port", 50051, "gRPC server port")
	rootCmd.Flags().Int("http-port", 8080, "HTTP server port")
	rootCmd.Flags().String("host", "0.0.0.0", "server host")
	rootCmd.Flags().String("unix-socket", "", "Unix socket path (disabled on Windows)")
	rootCmd.Flags().String("qdrant", "", "Qdrant URL (overrides config)")

	// Version command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("rice-search-server %s\n", version)
			fmt.Printf("  commit: %s\n", commit)
			fmt.Printf("  built:  %s\n", date)
		},
	})

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runServer(cmd *cobra.Command, args []string) error {
	configPath, _ := cmd.Flags().GetString("config")
	verbose, _ := cmd.Flags().GetBool("verbose")
	grpcPort, _ := cmd.Flags().GetInt("grpc-port")
	httpPort, _ := cmd.Flags().GetInt("http-port")
	host, _ := cmd.Flags().GetString("host")
	unixSocket, _ := cmd.Flags().GetString("unix-socket")
	qdrantURL, _ := cmd.Flags().GetString("qdrant")

	// Setup logger
	logLevel := "info"
	if verbose {
		logLevel = "debug"
	}
	log := logger.New(logLevel, "text")

	log.Info("Starting Rice Search Server",
		"version", version,
		"grpc_port", grpcPort,
		"http_port", httpPort,
	)

	// Load config
	appCfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override from flags
	if cmd.Flags().Changed("http-port") {
		appCfg.Port = httpPort
	}
	if cmd.Flags().Changed("host") {
		appCfg.Host = host
	}
	if qdrantURL != "" {
		appCfg.Qdrant.URL = qdrantURL
	}

	// Initialize event bus
	eventBus := bus.NewMemoryBus()
	defer eventBus.Close()

	// Initialize Qdrant client
	qdrantCfg := qdrant.DefaultClientConfig()
	if appCfg.Qdrant.URL != "" {
		h, p, err := parseQdrantURL(appCfg.Qdrant.URL)
		if err != nil {
			return fmt.Errorf("invalid Qdrant URL: %w", err)
		}
		qdrantCfg.Host = h
		qdrantCfg.Port = p
	}
	if appCfg.Qdrant.APIKey != "" {
		qdrantCfg.APIKey = appCfg.Qdrant.APIKey
	}

	qc, err := qdrant.NewClient(qdrantCfg)
	if err != nil {
		return fmt.Errorf("failed to connect to Qdrant: %w", err)
	}
	defer qc.Close()
	log.Info("Connected to Qdrant", "host", qdrantCfg.Host, "port", qdrantCfg.Port)

	// Initialize ML service
	mlSvc, err := ml.NewService(appCfg.ML, log)
	if err != nil {
		return fmt.Errorf("failed to create ML service: %w", err)
	}
	defer mlSvc.Close()

	// Load ML models
	log.Info("Loading ML models...")
	if err := mlSvc.LoadModels(); err != nil {
		log.Warn("Some ML models failed to load", "error", err)
	}

	// Initialize store service
	storeCfg := store.ServiceConfig{
		StoragePath:   "./data/stores",
		EnsureDefault: true,
	}
	storeSvc, err := store.NewService(qc, storeCfg)
	if err != nil {
		return fmt.Errorf("failed to create store service: %w", err)
	}

	// Initialize index pipeline
	pipelineCfg := index.DefaultPipelineConfig()
	indexSvc := index.NewPipeline(pipelineCfg, mlSvc, qc, log)

	// Initialize search service
	searchCfg := search.DefaultConfig()
	searchSvc := search.NewService(mlSvc, qc, log, searchCfg)

	// Start gRPC server
	grpcCfg := grpcserver.Config{
		TCPAddr:        fmt.Sprintf("%s:%d", host, grpcPort),
		UnixSocketPath: unixSocket,
		Version:        version,
		Commit:         commit,
		BuildDate:      date,
	}
	grpcSrv := grpcserver.New(grpcCfg, log, mlSvc, qc, storeSvc, indexSvc, searchSvc)
	if err := grpcSrv.Start(); err != nil {
		return fmt.Errorf("failed to start gRPC server: %w", err)
	}
	defer grpcSrv.Stop()

	// Create unified HTTP server with Web UI + REST API
	mux := http.NewServeMux()

	// Register Web UI routes (uses gRPC server as backend)
	webHandler := web.NewHandler(grpcSrv, log)
	webHandler.RegisterRoutes(mux)

	// Register REST API routes
	registerAPIRoutes(mux, searchSvc, storeSvc, indexSvc, mlSvc, qc, log, version)

	// Create HTTP server
	httpAddr := fmt.Sprintf("%s:%d", host, httpPort)
	httpSrv := &http.Server{
		Addr:         httpAddr,
		Handler:      corsMiddleware(loggingMiddleware(mux, log)),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	// Start HTTP server in background
	go func() {
		log.Info("Starting HTTP server", "addr", httpAddr, "web_ui", "enabled")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server error", "error", err)
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	log.Info("Shutdown signal received")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Error("HTTP shutdown error", "error", err)
	}
	grpcSrv.Stop()

	log.Info("Server stopped")
	return nil
}

// parseQdrantURL extracts host and gRPC port from a Qdrant URL.
func parseQdrantURL(rawURL string) (string, int, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", 0, err
	}

	h := u.Hostname()
	if h == "" {
		h = "localhost"
	}

	portStr := u.Port()
	httpPort := 6333
	if portStr != "" {
		httpPort, err = strconv.Atoi(portStr)
		if err != nil {
			return "", 0, fmt.Errorf("invalid port: %s", portStr)
		}
	}

	// gRPC port = HTTP port + 1
	grpcPort := httpPort + 1
	return h, grpcPort, nil
}

// registerAPIRoutes registers REST API endpoints.
func registerAPIRoutes(mux *http.ServeMux, searchSvc *search.Service, storeSvc *store.Service, indexSvc *index.Pipeline, mlSvc ml.Service, qc *qdrant.Client, log *logger.Logger, version string) {
	// Health endpoints
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /v1/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"version": version,
			"commit":  commit,
			"date":    date,
		})
	})

	mux.HandleFunc("GET /v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		health := mlSvc.Health()
		status := "healthy"
		if !health.Healthy {
			status = "degraded"
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  status,
			"version": version,
			"ml":      health,
		})
	})

	// Search handler
	searchHandler := search.NewHandler(searchSvc)
	searchHandler.RegisterRoutesLegacy(mux)

	// Store handler
	mux.HandleFunc("GET /v1/stores", func(w http.ResponseWriter, r *http.Request) {
		stores, err := storeSvc.ListStores(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"stores": stores})
	})

	mux.HandleFunc("POST /v1/stores", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		newStore := store.NewStore(req.Name)
		newStore.DisplayName = req.DisplayName
		newStore.Description = req.Description
		if err := storeSvc.CreateStore(r.Context(), newStore); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(newStore)
	})

	mux.HandleFunc("GET /v1/stores/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		s, err := storeSvc.GetStore(r.Context(), name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s)
	})

	mux.HandleFunc("DELETE /v1/stores/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if err := storeSvc.DeleteStore(r.Context(), name); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /v1/stores/{name}/stats", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		stats, err := storeSvc.GetStoreStats(r.Context(), name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})

	// Index handlers
	mux.HandleFunc("POST /v1/stores/{name}/index", func(w http.ResponseWriter, r *http.Request) {
		storeName := r.PathValue("name")
		var req struct {
			Files []struct {
				Path     string `json:"path"`
				Content  string `json:"content"`
				Language string `json:"language"`
			} `json:"files"`
			Force bool `json:"force"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		docs := make([]*index.Document, len(req.Files))
		for i, f := range req.Files {
			doc := index.NewDocument(f.Path, f.Content)
			if f.Language != "" {
				doc.Language = f.Language
			}
			docs[i] = doc
		}

		result, err := indexSvc.Index(r.Context(), index.IndexRequest{
			Store:     storeName,
			Documents: docs,
			Force:     req.Force,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	mux.HandleFunc("GET /v1/stores/{name}/index/stats", func(w http.ResponseWriter, r *http.Request) {
		storeName := r.PathValue("name")
		stats, err := indexSvc.GetStats(r.Context(), storeName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})
}

// corsMiddleware adds CORS headers to responses.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs HTTP requests.
func loggingMiddleware(next http.Handler, log *logger.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status
		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		log.Debug("HTTP request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.status,
			"duration", time.Since(start),
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
