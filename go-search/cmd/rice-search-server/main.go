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
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"

	"github.com/ricesearch/rice-search/internal/bus"
	"github.com/ricesearch/rice-search/internal/config"
	"github.com/ricesearch/rice-search/internal/connection"
	"github.com/ricesearch/rice-search/internal/grpcserver"
	"github.com/ricesearch/rice-search/internal/index"
	"github.com/ricesearch/rice-search/internal/metrics"
	"github.com/ricesearch/rice-search/internal/ml"
	"github.com/ricesearch/rice-search/internal/models"
	apperrors "github.com/ricesearch/rice-search/internal/pkg/errors"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
	"github.com/ricesearch/rice-search/internal/pkg/middleware"
	"github.com/ricesearch/rice-search/internal/qdrant"
	"github.com/ricesearch/rice-search/internal/query"
	"github.com/ricesearch/rice-search/internal/search"
	"github.com/ricesearch/rice-search/internal/search/reranker"
	"github.com/ricesearch/rice-search/internal/settings"
	"github.com/ricesearch/rice-search/internal/store"
	"github.com/ricesearch/rice-search/internal/web"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"

	// inFlightCounter tracks the number of active HTTP requests
	inFlightCounter int64

	// serverReady indicates whether the server is ready to accept requests
	// Set to false during shutdown to fail readiness checks
	serverReady atomic.Bool
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

func runServer(cmd *cobra.Command, _ []string) error {
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

	// Initialize rate limiter if enabled
	var rateLimiter *middleware.RateLimiter
	if appCfg.Security.RateLimit > 0 {
		rlCfg := middleware.RateLimiterConfig{
			RequestsPerSecond: float64(appCfg.Security.RateLimit),
			Burst:             appCfg.Security.RateLimit * 2,
			CleanupInterval:   time.Minute,
		}
		rateLimiter = middleware.NewRateLimiter(rlCfg)
		log.Info("Rate limiting enabled", "requests_per_second", appCfg.Security.RateLimit)
	}

	// Initialize metrics EARLY (needed for instrumented bus and ML wiring)
	// Uses Redis persistence if configured, otherwise falls back to memory
	metricsSvc := metrics.NewWithConfig(appCfg.Metrics.Persistence, appCfg.Metrics.RedisURL)
	log.Info("Initialized metrics", "persistence", appCfg.Metrics.Persistence)

	// Initialize event bus
	innerBus := bus.NewMemoryBus()
	defer func() { _ = innerBus.Close() }()

	// Initialize event logger (wraps event bus)
	eventLogger, err := bus.NewEventLogger(appCfg.Bus.EventLogPath, appCfg.Bus.EventLogEnabled)
	if err != nil {
		return fmt.Errorf("failed to create event logger: %w", err)
	}
	defer func() { _ = eventLogger.Close() }()

	// Wrap bus with logging if enabled, then instrument with metrics
	var eventBus bus.Bus
	if appCfg.Bus.EventLogEnabled {
		eventBus = bus.NewInstrumentedBus(bus.NewLoggedBus(innerBus, eventLogger, log), metricsSvc)
		log.Info("Event logging enabled", "path", appCfg.Bus.EventLogPath)
	} else {
		eventBus = bus.NewInstrumentedBus(innerBus, metricsSvc)
		log.Info("Event logging disabled")
	}
	log.Info("Event bus instrumented with metrics")

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
	// Always use timeout from config (has default of 30s if not set)
	qdrantCfg.Timeout = appCfg.Qdrant.Timeout

	qc, err := qdrant.NewClient(qdrantCfg)
	if err != nil {
		return fmt.Errorf("failed to connect to Qdrant: %w", err)
	}
	defer func() { _ = qc.Close() }()
	log.Info("Connected to Qdrant", "host", qdrantCfg.Host, "port", qdrantCfg.Port)

	// Initialize ML service
	mlSvc, err := ml.NewService(appCfg.ML, log)
	if err != nil {
		return fmt.Errorf("failed to create ML service: %w", err)
	}
	defer func() { _ = mlSvc.Close() }()

	// Load ML models
	log.Info("Loading ML models...")
	if err := mlSvc.LoadModels(); err != nil {
		log.Warn("Some ML models failed to load", "error", err)
	}

	// Register ML event handlers
	mlHandler := ml.NewEventHandler(mlSvc, eventBus, log)
	if err := mlHandler.Register(context.Background()); err != nil {
		log.Warn("Failed to register ML event handlers", "error", err)
	} else {
		log.Info("Registered ML event handlers (embed, sparse, rerank)")
	}

	// Initialize store service with event bus
	storeCfg := store.ServiceConfig{
		StoragePath:   "./data/stores",
		EnsureDefault: true,
	}
	storeSvc, err := store.NewService(qc, storeCfg, eventBus)
	if err != nil {
		return fmt.Errorf("failed to create store service: %w", err)
	}

	// Initialize index pipeline with event bus
	pipelineCfg := index.DefaultPipelineConfig()
	indexSvc := index.NewPipeline(pipelineCfg, mlSvc, qc, log, eventBus)

	// Register metrics event subscriber for automatic metric collection (metricsSvc created earlier)
	metricsSubscriber := metrics.NewEventSubscriber(metricsSvc, eventBus)
	if err := metricsSubscriber.SubscribeToEvents(context.Background()); err != nil {
		log.Warn("Failed to subscribe metrics to events", "error", err)
	} else {
		log.Info("Registered metrics event subscriber (13 topics)")
	}

	// Wire ML service with metrics
	mlSvc.SetMLMetrics(metricsSvc)    // For ML operation metrics (embed, rerank, sparse)
	mlSvc.SetCacheMetrics(metricsSvc) // For embedding cache hit/miss tracking
	log.Info("Wired ML metrics (operations + cache)")

	// Initialize query understanding service (Option B/C fallback)
	querySvc := query.NewService(log)
	log.Info("Initialized query understanding service")

	// Initialize search service with query understanding, event bus, and metrics
	searchCfg := search.DefaultConfig()
	searchSvc := search.NewService(mlSvc, qc, log, searchCfg, querySvc, eventBus, metricsSvc)

	// Initialize and wire multi-pass reranker
	multiPassReranker := reranker.NewMultiPassReranker(mlSvc, log)
	multiPassAdapter := reranker.NewAdapter(multiPassReranker)
	searchSvc.SetMultiPassReranker(multiPassAdapter)
	log.Info("Initialized multi-pass reranker (two-pass with early exit)")

	// Wire monitoring service to search service (will be set after monSvc is created)
	var monitoringSvc *connection.MonitoringService

	// Initialize model registry
	var modelReg *models.Registry
	modelRegCfg := models.RegistryConfig{
		StoragePath:  "./data/models",
		ModelsDir:    appCfg.ML.ModelsDir,
		LoadDefaults: true,
	}
	modelReg, err = models.NewRegistry(modelRegCfg, log)
	if err != nil {
		log.Warn("Failed to create model registry, continuing without it", "error", err)
		modelReg = nil
	} else {
		log.Info("Initialized model registry")
	}

	// Initialize mapper service (requires model registry)
	var mapperSvc *models.MapperService
	if modelReg != nil {
		modelStorage := models.NewFileStorage("./data/models")
		mapperSvc, err = models.NewMapperService(modelStorage, modelReg, log)
		if err != nil {
			log.Warn("Failed to create mapper service, continuing without it", "error", err)
			mapperSvc = nil
		} else {
			log.Info("Initialized mapper service")
		}
	}

	// Initialize connection service
	var connSvc *connection.Service
	connSvc, err = connection.NewService(eventBus, connection.ServiceConfig{
		StoragePath: "./data/connections",
	})
	if err != nil {
		log.Warn("Failed to create connection service, continuing without it", "error", err)
		connSvc = nil
	} else {
		log.Info("Initialized connection service")
	}

	// Initialize settings service
	var settingsSvc *settings.Service
	settingsSvc, err = settings.NewService(settings.ServiceConfig{
		StoragePath:  "./data/settings",
		LoadDefaults: true,
	}, eventBus, log)
	if err != nil {
		log.Warn("Failed to create settings service, continuing without it", "error", err)
		settingsSvc = nil
	} else {
		log.Info("Initialized settings service")
	}

	// Initialize connection monitoring service
	if connSvc != nil {
		monCfg := connection.DefaultMonitoringConfig()
		monitoringSvc = connection.NewMonitoringService(connSvc, eventBus, log, monCfg)
		ctx := context.Background()
		monitoringSvc.Start(ctx)
		log.Info("Initialized connection monitoring service",
			"search_spike_multiplier", monCfg.SearchSpikeMultiplier,
			"inactivity_threshold", monCfg.InactivityThreshold,
		)

		// Subscribe to alert events and log them
		err := eventBus.Subscribe(ctx, bus.TopicAlertTriggered, func(alertCtx context.Context, event bus.Event) error {
			alert, ok := event.Payload.(connection.Alert)
			if !ok {
				return nil
			}

			logFields := []interface{}{
				"type", alert.Type,
				"severity", alert.Severity,
				"connection_id", alert.ConnectionID,
				"message", alert.Message,
			}

			switch alert.Severity {
			case "high", "critical":
				log.Warn("Connection security alert", logFields...)
			case "medium":
				log.Info("Connection security alert", logFields...)
			default:
				log.Debug("Connection security alert", logFields...)
			}

			return nil
		})
		if err != nil {
			log.Warn("Failed to subscribe to alert events", "error", err)
		}
	}

	// Wire monitoring service to search service
	searchSvc.SetMonitoringService(monitoringSvc)

	// Initialize detailed health checker
	detailedHealthChecker := search.NewDetailedHealthChecker(search.DetailedHealthCheckerConfig{
		MLService: mlSvc,
		Qdrant:    qc,
		QdrantURL: appCfg.Qdrant.URL,
		Bus:       eventBus,
		Stores:    storeSvc,
		Version:   version,
		GitCommit: commit,
	})
	log.Info("Initialized detailed health checker")

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
	webHandler := web.NewHandler(
		grpcSrv,           // GRPCClient
		log,               // Logger
		appCfg,            // Config
		modelReg,          // Model registry
		mapperSvc,         // Mapper service
		connSvc,           // Connection service
		settingsSvc,       // Settings service
		metricsSvc,        // Metrics
		appCfg.Qdrant.URL, // qdrantURL
		eventLogger,       // Event logger
	)
	webHandler.RegisterRoutes(mux)

	// Register REST API routes
	registerAPIRoutes(mux, searchSvc, storeSvc, indexSvc, mlSvc, qc, log, version, detailedHealthChecker)

	// Build middleware chain
	handler := http.Handler(mux)
	handler = inFlightMiddleware(handler)
	handler = loggingMiddleware(handler, log)
	handler = corsMiddleware(handler)
	if rateLimiter != nil {
		handler = rateLimiter.Middleware(handler)
	}
	handler = recoveryMiddleware(handler, log)

	// Create HTTP server with middleware chain
	httpAddr := fmt.Sprintf("%s:%d", host, httpPort)
	httpSrv := &http.Server{
		Addr:         httpAddr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	// Start HTTP server in background
	go func() {
		serverReady.Store(true)
		log.Info("Starting HTTP server", "addr", httpAddr, "web_ui", "enabled")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server error", "error", err)
		}
	}()

	// Wait for shutdown signal (platform-specific: Unix includes SIGQUIT, Windows does not)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, shutdownSignals...)

	<-sigCh
	log.Info("Shutdown signal received")

	// Graceful shutdown with in-flight request draining
	shutdownTimeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Stop accepting new requests
	log.Info("Setting server to not ready...")
	serverReady.Store(false)
	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Error("HTTP shutdown error", "error", err)
	}

	// Wait for in-flight requests to complete
	log.Info("Draining in-flight requests...")
	if drainInFlight(shutdownTimeout, log) {
		log.Info("All in-flight requests completed")
	} else {
		remaining := atomic.LoadInt64(&inFlightCounter)
		log.Warn("Shutdown timeout reached with pending requests", "remaining", remaining)
	}

	// Close services that need cleanup
	if settingsSvc != nil {
		if err := settingsSvc.Close(); err != nil {
			log.Warn("Error closing settings service", "error", err)
		} else {
			log.Info("Closed settings service")
		}
	}

	if err := metricsSvc.Close(); err != nil {
		log.Warn("Error closing metrics service", "error", err)
	} else {
		log.Info("Closed metrics service")
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
func registerAPIRoutes(mux *http.ServeMux, searchSvc *search.Service, storeSvc *store.Service, indexSvc *index.Pipeline, mlSvc ml.Service, _ *qdrant.Client, _ *logger.Logger, version string, detailedHealthChecker *search.DetailedHealthChecker) {
	// Health endpoints
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"version":    version,
			"git_commit": commit,
			"build_time": date,
			"go_version": "go1.21+",
		})
	})

	mux.HandleFunc("GET /v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := detailedHealthChecker.CheckDetailed(r.Context())
		_ = json.NewEncoder(w).Encode(status)
	})

	// Readiness probe - returns 503 if server is shutting down or ML service is not healthy
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Check if server is shutting down
		if !serverReady.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "not_ready", "reason": "shutting_down"})
			return
		}

		// Check ML health
		health := mlSvc.Health()
		if !health.Healthy {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "not_ready", "reason": "ml_unhealthy"})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})

	// Search handler
	searchHandler := search.NewHandler(searchSvc)
	searchHandler.RegisterRoutes(mux)

	// Store handler
	mux.HandleFunc("GET /v1/stores", func(w http.ResponseWriter, r *http.Request) {
		stores, err := storeSvc.ListStores(r.Context())
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"stores": stores})
	})

	mux.HandleFunc("POST /v1/stores", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		newStore := store.NewStore(req.Name)
		newStore.DisplayName = req.DisplayName
		newStore.Description = req.Description
		if err := storeSvc.CreateStore(r.Context(), newStore); err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(newStore)
	})

	mux.HandleFunc("GET /v1/stores/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		s, err := storeSvc.GetStore(r.Context(), name)
		if err != nil {
			writeAPIError(w, http.StatusNotFound, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s)
	})

	mux.HandleFunc("DELETE /v1/stores/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if err := storeSvc.DeleteStore(r.Context(), name); err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /v1/stores/{name}/stats", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		stats, err := storeSvc.GetStoreStats(r.Context(), name)
		if err != nil {
			writeAPIError(w, http.StatusNotFound, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(stats)
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
			writeAPIError(w, http.StatusBadRequest, err)
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
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})

	mux.HandleFunc("GET /v1/stores/{name}/index/stats", func(w http.ResponseWriter, r *http.Request) {
		storeName := r.PathValue("name")
		stats, err := indexSvc.GetStats(r.Context(), storeName)
		if err != nil {
			writeAPIError(w, http.StatusNotFound, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(stats)
	})

	mux.HandleFunc("DELETE /v1/stores/{name}/index", func(w http.ResponseWriter, r *http.Request) {
		storeName := r.PathValue("name")
		var req struct {
			Paths      []string `json:"paths"`
			PathPrefix string   `json:"path_prefix"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}

		// Delete by prefix if specified
		if req.PathPrefix != "" {
			if err := indexSvc.DeleteByPrefix(r.Context(), storeName, req.PathPrefix); err != nil {
				writeAPIError(w, http.StatusInternalServerError, err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"deleted_prefix": req.PathPrefix,
			})
			return
		}

		// Delete specific paths
		if len(req.Paths) == 0 {
			writeAPIError(w, http.StatusBadRequest, apperrors.ValidationError("paths or path_prefix required"))
			return
		}
		if err := indexSvc.Delete(r.Context(), storeName, req.Paths); err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"deleted_count": len(req.Paths),
			"paths":         req.Paths,
		})
	})

	mux.HandleFunc("POST /v1/stores/{name}/index/sync", func(w http.ResponseWriter, r *http.Request) {
		storeName := r.PathValue("name")
		var req struct {
			CurrentPaths []string `json:"current_paths"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		if len(req.CurrentPaths) == 0 {
			writeAPIError(w, http.StatusBadRequest, apperrors.ValidationError("current_paths required"))
			return
		}

		removed, err := indexSvc.Sync(r.Context(), storeName, req.CurrentPaths)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"removed": removed,
		})
	})

	mux.HandleFunc("POST /v1/stores/{name}/index/reindex", func(w http.ResponseWriter, r *http.Request) {
		storeName := r.PathValue("name")
		var req struct {
			Files []struct {
				Path     string `json:"path"`
				Content  string `json:"content"`
				Language string `json:"language"`
			} `json:"files"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
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

		result, err := indexSvc.Reindex(r.Context(), index.IndexRequest{
			Store:     storeName,
			Documents: docs,
		})
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})

	mux.HandleFunc("GET /v1/stores/{name}/index/files", func(w http.ResponseWriter, r *http.Request) {
		storeName := r.PathValue("name")

		// Parse pagination params
		page := 1
		pageSize := 50
		if pageStr := r.URL.Query().Get("page"); pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
				page = p
			}
		}
		if sizeStr := r.URL.Query().Get("page_size"); sizeStr != "" {
			if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 {
				pageSize = s
			}
		}

		files, total := indexSvc.ListFiles(storeName, page, pageSize)
		totalPages := (total + pageSize - 1) / pageSize

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"files":       files,
			"total":       total,
			"page":        page,
			"page_size":   pageSize,
			"total_pages": totalPages,
		})
	})

	// Convenience search endpoint (uses default store)
	mux.HandleFunc("POST /v1/search", func(w http.ResponseWriter, r *http.Request) {
		var req search.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		// Use default store if not specified
		if req.Store == "" {
			req.Store = "default"
		}
		resp, err := searchSvc.Search(r.Context(), req)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	// ML HTTP endpoint wrappers (expose internal ML service via HTTP)
	mux.HandleFunc("POST /v1/ml/embed", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Texts []string `json:"texts"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		if len(req.Texts) == 0 {
			writeAPIError(w, http.StatusBadRequest, apperrors.ValidationError("texts array required"))
			return
		}
		embeddings, err := mlSvc.Embed(r.Context(), req.Texts)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"embeddings": embeddings,
			"count":      len(embeddings),
		})
	})

	mux.HandleFunc("POST /v1/ml/sparse", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Texts []string `json:"texts"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		if len(req.Texts) == 0 {
			writeAPIError(w, http.StatusBadRequest, apperrors.ValidationError("texts array required"))
			return
		}
		vectors, err := mlSvc.SparseEncode(r.Context(), req.Texts)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"vectors": vectors,
			"count":   len(vectors),
		})
	})

	mux.HandleFunc("POST /v1/ml/rerank", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string   `json:"query"`
			Documents []string `json:"documents"`
			TopK      int      `json:"top_k"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		if req.Query == "" || len(req.Documents) == 0 {
			writeAPIError(w, http.StatusBadRequest, apperrors.ValidationError("query and documents required"))
			return
		}
		if req.TopK == 0 {
			req.TopK = len(req.Documents)
		}
		results, err := mlSvc.Rerank(r.Context(), req.Query, req.Documents, req.TopK)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"results": results,
			"count":   len(results),
		})
	})
}

// writeAPIError writes a JSON error response with proper status code and sanitization.
// For 4xx errors, shows the error message. For 5xx errors, sanitizes internal details.
func writeAPIError(w http.ResponseWriter, status int, err error) {
	apperrors.WriteErrorWithStatus(w, status, err)
}

// recoveryMiddleware catches panics and returns a 500 error instead of crashing.
// This prevents uncaught panics from bringing down the entire server.
func recoveryMiddleware(next http.Handler, log *logger.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic with stack trace info
				log.Error("Panic recovered in HTTP handler",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
				)

				// Return sanitized error to client (don't leak internal details)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error":   "internal server error",
					"code":    "INTERNAL_ERROR",
					"message": "An unexpected error occurred. Please try again.",
				})
			}
		}()
		next.ServeHTTP(w, r)
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

// inFlightMiddleware tracks in-flight HTTP requests for graceful shutdown.
func inFlightMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&inFlightCounter, 1)
		defer atomic.AddInt64(&inFlightCounter, -1)
		next.ServeHTTP(w, r)
	})
}

// drainInFlight waits for all in-flight requests to complete or timeout.
// Returns true if all requests completed, false if timeout reached.
func drainInFlight(timeout time.Duration, log *logger.Logger) bool {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		count := atomic.LoadInt64(&inFlightCounter)
		if count == 0 {
			return true
		}

		if time.Now().After(deadline) {
			return false
		}

		select {
		case <-ticker.C:
			log.Info("Draining in-flight requests", "remaining", count)
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}
