// Package server provides the HTTP server that wires all services together.
package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/ricesearch/rice-search/internal/bus"
	"github.com/ricesearch/rice-search/internal/config"
	"github.com/ricesearch/rice-search/internal/index"
	"github.com/ricesearch/rice-search/internal/ml"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
	"github.com/ricesearch/rice-search/internal/qdrant"
	"github.com/ricesearch/rice-search/internal/search"
	"github.com/ricesearch/rice-search/internal/store"
)

// Server is the main HTTP server that wires all services together.
type Server struct {
	cfg        Config
	log        *logger.Logger
	httpServer *http.Server

	// Services
	bus    bus.Bus
	ml     ml.Service
	qdrant *qdrant.Client
	store  *store.Service
	index  *index.Pipeline
	search *search.Service

	// Handlers
	searchHandler *search.Handler
	healthHandler *search.HealthHandler
	storeHandler  *StoreHandler
	indexHandler  *IndexHandler

	mu      sync.RWMutex
	started bool
}

// Config configures the server.
type Config struct {
	// Host is the address to bind to.
	Host string

	// Port is the HTTP port.
	Port int

	// Version is the application version.
	Version string

	// ReadTimeout is the HTTP read timeout.
	ReadTimeout time.Duration

	// WriteTimeout is the HTTP write timeout.
	WriteTimeout time.Duration

	// ShutdownTimeout is the graceful shutdown timeout.
	ShutdownTimeout time.Duration
}

// DefaultConfig returns sensible server defaults.
func DefaultConfig() Config {
	return Config{
		Host:            "0.0.0.0",
		Port:            8080,
		Version:         "dev",
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    60 * time.Second,
		ShutdownTimeout: 30 * time.Second,
	}
}

// New creates a new server with all dependencies.
func New(cfg Config, appCfg config.Config, log *logger.Logger) (*Server, error) {
	if cfg.Port == 0 {
		cfg = DefaultConfig()
	}

	s := &Server{
		cfg: cfg,
		log: log,
	}

	// Initialize event bus (in-memory for monolith)
	s.bus = bus.NewMemoryBus()

	// Initialize Qdrant client
	qdrantCfg := qdrant.DefaultClientConfig()

	// Parse Qdrant URL to extract host and port
	if appCfg.Qdrant.URL != "" {
		host, port, err := parseQdrantURL(appCfg.Qdrant.URL)
		if err != nil {
			return nil, fmt.Errorf("invalid Qdrant URL: %w", err)
		}
		qdrantCfg.Host = host
		qdrantCfg.Port = port
	}
	if appCfg.Qdrant.APIKey != "" {
		qdrantCfg.APIKey = appCfg.Qdrant.APIKey
	}

	qc, err := qdrant.NewClient(qdrantCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Qdrant client: %w", err)
	}
	s.qdrant = qc

	// Initialize ML service
	mlSvc, err := ml.NewService(appCfg.ML, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create ML service: %w", err)
	}
	s.ml = mlSvc

	// Initialize store service
	storeCfg := store.ServiceConfig{
		StoragePath:   "./data/stores",
		EnsureDefault: true,
	}
	storeSvc, err := store.NewService(s.qdrant, storeCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create store service: %w", err)
	}
	s.store = storeSvc

	// Initialize index pipeline
	pipelineCfg := index.DefaultPipelineConfig()
	s.index = index.NewPipeline(pipelineCfg, s.ml, s.qdrant, log)

	// Initialize search service
	searchCfg := search.DefaultConfig()
	s.search = search.NewService(s.ml, s.qdrant, log, searchCfg)

	// Initialize handlers
	s.searchHandler = search.NewHandler(s.search)
	healthChecker := search.NewHealthChecker(s.ml, s.qdrant)
	s.healthHandler = search.NewHealthHandler(healthChecker, cfg.Version)
	s.storeHandler = NewStoreHandler(s.store)
	s.indexHandler = NewIndexHandler(s.index, s.store)

	return s, nil
}

// parseQdrantURL extracts host and port from a Qdrant URL.
// Example: http://localhost:6333 -> localhost, 6334 (gRPC port)
func parseQdrantURL(rawURL string) (string, int, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", 0, err
	}

	host := u.Hostname()
	if host == "" {
		host = "localhost"
	}

	// Get port from URL, default to 6333 (HTTP)
	portStr := u.Port()
	httpPort := 6333
	if portStr != "" {
		httpPort, err = strconv.Atoi(portStr)
		if err != nil {
			return "", 0, fmt.Errorf("invalid port: %s", portStr)
		}
	}

	// Qdrant gRPC port is typically HTTP port + 1
	grpcPort := httpPort + 1

	return host, grpcPort, nil
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return fmt.Errorf("server already started")
	}
	s.started = true
	s.mu.Unlock()

	// Load ML models
	if svc, ok := s.ml.(*ml.ServiceImpl); ok {
		s.log.Info("Loading ML models...")
		if err := svc.LoadModels(); err != nil {
			s.log.Warn("Some ML models failed to load", "error", err)
			// Continue anyway - we can run in degraded mode
		}
	}

	// Setup routes
	mux := s.setupRoutes()

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  s.cfg.ReadTimeout,
		WriteTimeout: s.cfg.WriteTimeout,
	}

	s.log.Info("Starting HTTP server", "addr", addr)
	return s.httpServer.ListenAndServe()
}

// Stop gracefully stops the server.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}

	s.log.Info("Shutting down server...")

	// Shutdown HTTP server
	shutdownCtx, cancel := context.WithTimeout(ctx, s.cfg.ShutdownTimeout)
	defer cancel()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		s.log.Error("HTTP shutdown error", "error", err)
	}

	// Close services
	if s.qdrant != nil {
		s.qdrant.Close()
	}
	if s.ml != nil {
		s.ml.Close()
	}
	if s.bus != nil {
		s.bus.Close()
	}

	s.started = false
	s.log.Info("Server stopped")

	return nil
}

// setupRoutes configures all HTTP routes.
func (s *Server) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// Apply CORS middleware
	handler := search.CORSMiddleware(mux)

	// Health endpoints
	s.healthHandler.RegisterRoutesLegacy(mux)

	// Search endpoints
	s.searchHandler.RegisterRoutesLegacy(mux)

	// Store endpoints
	s.storeHandler.RegisterRoutes(mux)

	// Index endpoints
	s.indexHandler.RegisterRoutes(mux)

	// Wrap with logging
	return wrapWithLogging(handler, s.log)
}

// wrapWithLogging returns a mux with logging middleware.
func wrapWithLogging(handler http.Handler, log *logger.Logger) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create response writer wrapper to capture status
		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		handler.ServeHTTP(wrapped, r)

		log.Debug("HTTP request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.status,
			"duration", time.Since(start),
		)
	})
	return mux
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

// Health returns the server health status.
func (s *Server) Health() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.started
}
