// Package server provides an alternative HTTP server implementation.
//
// Deprecated: This package is not currently used. The main server binary
// (cmd/rice-search-server) uses grpcserver + direct http.ServeMux instead.
// This package is kept for potential future use or as reference implementation.
// Consider removal if not needed after review.
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
	"github.com/ricesearch/rice-search/internal/connection"
	"github.com/ricesearch/rice-search/internal/index"
	"github.com/ricesearch/rice-search/internal/metrics"
	"github.com/ricesearch/rice-search/internal/ml"
	"github.com/ricesearch/rice-search/internal/models"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
	"github.com/ricesearch/rice-search/internal/qdrant"
	"github.com/ricesearch/rice-search/internal/search"
	"github.com/ricesearch/rice-search/internal/settings"
	"github.com/ricesearch/rice-search/internal/store"
	"github.com/ricesearch/rice-search/internal/web"
)

// Server is the main HTTP server that wires all services together.
type Server struct {
	cfg        Config
	appCfg     *config.Config
	log        *logger.Logger
	httpServer *http.Server

	// Core Services
	bus    bus.Bus
	ml     ml.Service
	qdrant *qdrant.Client
	store  *store.Service
	index  *index.Pipeline
	search *search.Service

	// New Services (Phase 1)
	connection  *connection.Service
	modelReg    *models.Registry
	mapperSvc   *models.MapperService
	metrics     *metrics.Metrics
	settingsSvc *settings.Service

	// Handlers
	searchHandler *search.Handler
	healthHandler *search.HealthHandler
	storeHandler  *StoreHandler
	indexHandler  *IndexHandler
	webHandler    *web.Handler

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
		cfg:    cfg,
		appCfg: &appCfg,
		log:    log,
	}

	// Initialize event bus (factory pattern with fallback)
	eventBus, err := bus.NewBus(appCfg.Bus)
	if err != nil {
		log.Warn("Failed to initialize configured bus, falling back to memory bus", "error", err, "type", appCfg.Bus.Type)
		eventBus = bus.NewMemoryBus()
	} else {
		log.Info("Initialized event bus", "type", appCfg.Bus.Type)
	}
	s.bus = eventBus

	// Initialize metrics
	s.metrics = metrics.New()
	log.Info("Initialized metrics")

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

	// Inject metrics into ML service
	if mlImpl, ok := s.ml.(*ml.ServiceImpl); ok {
		mlImpl.SetMLMetrics(s.metrics)    // For ML operation metrics
		mlImpl.SetCacheMetrics(s.metrics) // For embedding cache metrics
	}

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

	// Initialize index pipeline (events disabled in deprecated server)
	pipelineCfg := index.DefaultPipelineConfig()
	s.index = index.NewPipeline(pipelineCfg, s.ml, s.qdrant, log, nil)

	// Initialize search service (query understanding and events disabled in deprecated server)
	searchCfg := search.DefaultConfig()
	s.search = search.NewService(s.ml, s.qdrant, log, searchCfg, nil, nil, nil)

	// Initialize connection service
	if appCfg.Connection.Enabled {
		connCfg := connection.ServiceConfig{
			StoragePath: appCfg.Connection.StoragePath,
		}
		connSvc, err := connection.NewService(s.bus, connCfg)
		if err != nil {
			log.Warn("Failed to create connection service, continuing without it", "error", err)
		} else {
			s.connection = connSvc
			log.Info("Initialized connection service", "storage", connCfg.StoragePath)
		}
	}

	// Initialize model registry
	modelRegCfg := models.RegistryConfig{
		StoragePath:  appCfg.Models.RegistryPath,
		ModelsDir:    appCfg.ML.ModelsDir,
		LoadDefaults: true,
	}
	modelReg, err := models.NewRegistry(modelRegCfg, log)
	if err != nil {
		log.Warn("Failed to create model registry, continuing without it", "error", err)
	} else {
		s.modelReg = modelReg
		log.Info("Initialized model registry")
	}

	// Initialize mapper service
	if s.modelReg != nil {
		modelStorage := models.NewFileStorage(appCfg.Models.MappersPath)
		mapperSvc, err := models.NewMapperService(modelStorage, s.modelReg, log)
		if err != nil {
			log.Warn("Failed to create mapper service, continuing without it", "error", err)
		} else {
			s.mapperSvc = mapperSvc
			log.Info("Initialized mapper service")
		}
	}

	// Initialize settings service
	settingsCfg := settings.ServiceConfig{
		StoragePath:  "./data/settings",
		LoadDefaults: true,
		AuditLogPath: appCfg.Settings.AuditPath,
		AuditEnabled: appCfg.Settings.AuditEnabled,
	}
	settingsSvc, err := settings.NewService(settingsCfg, s.bus, log)
	if err != nil {
		log.Warn("Failed to create settings service, continuing with defaults", "error", err)
	} else {
		s.settingsSvc = settingsSvc
		log.Info("Initialized settings service")

		// Subscribe to settings changes to update services at runtime
		s.subscribeToSettingsChanges()
	}

	// Initialize handlers
	s.searchHandler = search.NewHandler(s.search)
	healthChecker := search.NewHealthChecker(s.ml, s.qdrant)
	s.healthHandler = search.NewHealthHandler(healthChecker, cfg.Version)
	s.storeHandler = NewStoreHandler(s.store)
	s.indexHandler = NewIndexHandler(s.index, s.store)

	// Initialize web handler (with local gRPC adapter)
	if appCfg.EnableWeb {
		grpcAdapter := NewLocalGRPCAdapter(s)
		s.webHandler = web.NewHandler(
			grpcAdapter,
			log,
			&appCfg,
			s.modelReg,
			s.mapperSvc,
			s.connection,
			s.settingsSvc,
			s.metrics,
			appCfg.Qdrant.URL,
			nil, // eventLogger - optional, for debugging only
		)
		log.Info("Initialized web UI handler")
	}

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
	s.healthHandler.RegisterRoutes(mux)

	// Search endpoints
	s.searchHandler.RegisterRoutes(mux)

	// Store endpoints
	s.storeHandler.RegisterRoutes(mux)

	// Index endpoints
	s.indexHandler.RegisterRoutes(mux)

	// Web UI routes (if enabled)
	if s.webHandler != nil {
		s.webHandler.RegisterRoutes(mux)
		s.log.Info("Registered Web UI routes")
	}

	// Prometheus metrics endpoint (independent of web handler)
	if s.metrics != nil {
		mux.Handle("GET /metrics", s.metrics.Handler())
		s.log.Info("Registered /metrics endpoint")
	}

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

// mlConfigChanged returns true if ML-related settings changed.
func (s *Server) mlConfigChanged(old, new settings.RuntimeConfig) bool {
	return old.EmbedGPU != new.EmbedGPU ||
		old.RerankGPU != new.RerankGPU ||
		old.QueryGPU != new.QueryGPU ||
		old.EmbedModel != new.EmbedModel ||
		old.RerankModel != new.RerankModel ||
		old.QueryModel != new.QueryModel ||
		old.QueryEnabled != new.QueryEnabled
}

// buildMLConfig constructs an MLConfig from RuntimeConfig settings.
func (s *Server) buildMLConfig(rc settings.RuntimeConfig) config.MLConfig {
	// Start with current ML config to preserve non-runtime-configurable fields
	mlCfg := s.appCfg.ML

	// Override with runtime settings
	if rc.EmbedModel != "" {
		mlCfg.EmbedModel = rc.EmbedModel
	}
	if rc.RerankModel != "" {
		mlCfg.RerankModel = rc.RerankModel
	}
	if rc.QueryModel != "" {
		mlCfg.QueryModel = rc.QueryModel
	}

	mlCfg.EmbedGPU = rc.EmbedGPU
	mlCfg.RerankGPU = rc.RerankGPU
	mlCfg.QueryGPU = rc.QueryGPU
	mlCfg.QueryModelEnabled = rc.QueryEnabled

	return mlCfg
}

// subscribeToSettingsChanges subscribes to settings change events
// and updates services with new configuration values.
func (s *Server) subscribeToSettingsChanges() {
	if s.bus == nil || s.settingsSvc == nil {
		return
	}

	// Subscribe to settings.changed events
	err := s.bus.Subscribe(context.Background(), bus.TopicSettingsChanged, func(ctx context.Context, event bus.Event) error {
		settingsEvent, ok := event.Payload.(settings.SettingsChangedEvent)
		if !ok {
			s.log.Warn("Invalid settings changed event payload")
			return nil
		}

		s.log.Info("Settings changed, updating services",
			"version", settingsEvent.NewConfig.Version,
			"changed_by", settingsEvent.ChangedBy,
		)

		// Update search service config
		if s.search != nil {
			newSearchCfg := search.Config{
				DefaultTopK:        settingsEvent.NewConfig.DefaultTopK,
				EnableReranking:    settingsEvent.NewConfig.DefaultRerank,
				RerankTopK:         settingsEvent.NewConfig.RerankCandidates,
				SparseWeight:       settingsEvent.NewConfig.SparseWeight,
				DenseWeight:        settingsEvent.NewConfig.DenseWeight,
				PrefetchMultiplier: 3, // Keep default multiplier
			}
			s.search.UpdateConfig(newSearchCfg)
		}

		// Reload ML models if ML config changed
		if s.mlConfigChanged(settingsEvent.OldConfig, settingsEvent.NewConfig) {
			s.log.Info("ML config changed, reloading models...")
			newMLCfg := s.buildMLConfig(settingsEvent.NewConfig)
			if err := s.ml.ReloadModelsWithConfig(newMLCfg); err != nil {
				s.log.Error("Failed to reload ML models with new config", "error", err)
				// Don't fail the entire settings update - log and continue
			} else {
				s.log.Info("Successfully reloaded ML models with new config")
			}
		}

		// Update query understanding service if QueryEnabled changed
		if settingsEvent.OldConfig.QueryEnabled != settingsEvent.NewConfig.QueryEnabled {
			if s.search != nil {
				if querySvc := s.search.QueryService(); querySvc != nil {
					querySvc.SetModelEnabled(settingsEvent.NewConfig.QueryEnabled)
					s.log.Info("Query understanding model enabled state changed",
						"enabled", settingsEvent.NewConfig.QueryEnabled)
				}
			}
		}

		// Index service config changes would also be handled here if needed

		return nil
	})

	if err != nil {
		s.log.Warn("Failed to subscribe to settings changes", "error", err)
	} else {
		s.log.Info("Subscribed to settings changes")
	}
}
