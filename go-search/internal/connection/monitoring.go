package connection

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ricesearch/rice-search/internal/bus"
	"github.com/ricesearch/rice-search/internal/metrics"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

// MonitoringService monitors connections for security anomalies.
type MonitoringService struct {
	connSvc *Service
	bus     bus.Bus
	log     *logger.Logger

	// Per-connection metrics
	connMetrics map[string]*ConnectionMetrics
	mu          sync.RWMutex

	// Alert deduplication
	recentAlerts map[string]time.Time
	alertMu      sync.RWMutex

	// Configuration
	searchSpikeMultiplier float64
	inactivityThreshold   time.Duration
	alertDedupWindow      time.Duration
}

// ConnectionMetrics tracks real-time metrics for a single connection.
type ConnectionMetrics struct {
	SearchRate         *metrics.MetricHistory // Searches per time bucket
	LastIP             string
	LastSeen           time.Time
	BaselineSearchRate float64 // Rolling baseline for spike detection
}

// Alert represents a security alert.
type Alert struct {
	Type         string                 `json:"type"`
	Severity     string                 `json:"severity"` // low, medium, high, critical
	ConnectionID string                 `json:"connection_id"`
	Message      string                 `json:"message"`
	Timestamp    time.Time              `json:"timestamp"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// MonitoringConfig configures the monitoring service.
type MonitoringConfig struct {
	SearchSpikeMultiplier float64       // Alert if searches exceed baseline by this factor (default: 3.0)
	InactivityThreshold   time.Duration // Alert if inactive for this duration (default: 2 hours)
	AlertDedupWindow      time.Duration // Suppress duplicate alerts within this window (default: 1 hour)
}

// DefaultMonitoringConfig returns default monitoring configuration.
func DefaultMonitoringConfig() MonitoringConfig {
	return MonitoringConfig{
		SearchSpikeMultiplier: 3.0,
		InactivityThreshold:   2 * time.Hour,
		AlertDedupWindow:      1 * time.Hour,
	}
}

// NewMonitoringService creates a new connection monitoring service.
func NewMonitoringService(connSvc *Service, eventBus bus.Bus, log *logger.Logger, cfg MonitoringConfig) *MonitoringService {
	if cfg.SearchSpikeMultiplier == 0 {
		cfg.SearchSpikeMultiplier = 3.0
	}
	if cfg.InactivityThreshold == 0 {
		cfg.InactivityThreshold = 2 * time.Hour
	}
	if cfg.AlertDedupWindow == 0 {
		cfg.AlertDedupWindow = 1 * time.Hour
	}

	return &MonitoringService{
		connSvc:               connSvc,
		bus:                   eventBus,
		log:                   log,
		connMetrics:           make(map[string]*ConnectionMetrics),
		recentAlerts:          make(map[string]time.Time),
		searchSpikeMultiplier: cfg.SearchSpikeMultiplier,
		inactivityThreshold:   cfg.InactivityThreshold,
		alertDedupWindow:      cfg.AlertDedupWindow,
	}
}

// Start starts the monitoring service and subscribes to events.
func (m *MonitoringService) Start(ctx context.Context) error {
	// Subscribe to connection.seen events
	if err := m.bus.Subscribe(ctx, TopicConnectionSeen, m.handleConnectionSeen); err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", TopicConnectionSeen, err)
	}

	// Subscribe to connection.registered events
	if err := m.bus.Subscribe(ctx, TopicConnectionRegistered, m.handleConnectionRegistered); err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", TopicConnectionRegistered, err)
	}

	// Start periodic anomaly detection
	go m.runDetectionLoop(ctx)

	m.log.Info("Connection monitoring service started")
	return nil
}

// handleConnectionSeen updates metrics and checks for IP changes.
func (m *MonitoringService) handleConnectionSeen(ctx context.Context, event bus.Event) error {
	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		m.log.Warn("Invalid payload for connection.seen event")
		return nil
	}

	connID, _ := payload["connection_id"].(string)
	ip, _ := payload["ip"].(string)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Get or create metrics for this connection
	cm, exists := m.connMetrics[connID]
	if !exists {
		cm = &ConnectionMetrics{
			SearchRate: metrics.NewMetricHistory(5*time.Minute, 12), // 5-min buckets, 1 hour retention
		}
		m.connMetrics[connID] = cm
	}

	// Check for IP change
	if cm.LastIP != "" && cm.LastIP != ip && ip != "" {
		m.triggerAlert(ctx, Alert{
			Type:         "ip_change",
			Severity:     "medium",
			ConnectionID: connID,
			Message:      fmt.Sprintf("IP changed from %s to %s", cm.LastIP, ip),
			Metadata: map[string]interface{}{
				"old_ip": cm.LastIP,
				"new_ip": ip,
			},
		})
	}

	// Update metrics
	if ip != "" {
		cm.LastIP = ip
	}
	cm.LastSeen = time.Now()

	return nil
}

// handleConnectionRegistered tracks new connections.
func (m *MonitoringService) handleConnectionRegistered(ctx context.Context, event bus.Event) error {
	conn, ok := event.Payload.(*Connection)
	if !ok {
		m.log.Warn("Invalid payload for connection.registered event")
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Initialize metrics if not exists
	if _, exists := m.connMetrics[conn.ID]; !exists {
		m.connMetrics[conn.ID] = &ConnectionMetrics{
			SearchRate: metrics.NewMetricHistory(5*time.Minute, 12),
			LastIP:     conn.LastIP,
			LastSeen:   conn.LastSeenAt,
		}
	}

	return nil
}

// RecordSearch records a search event for anomaly detection.
// This should be called by the search service after each search.
func (m *MonitoringService) RecordSearch(connID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cm, exists := m.connMetrics[connID]
	if !exists {
		cm = &ConnectionMetrics{
			SearchRate: metrics.NewMetricHistory(5*time.Minute, 12),
		}
		m.connMetrics[connID] = cm
	}

	cm.SearchRate.RecordCount()
}

// runDetectionLoop periodically checks for anomalies.
func (m *MonitoringService) runDetectionLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.detectAnomalies(ctx)
		case <-ctx.Done():
			m.log.Info("Monitoring loop stopped")
			return
		}
	}
}

// detectAnomalies checks all connections for suspicious patterns.
func (m *MonitoringService) detectAnomalies(ctx context.Context) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()

	for connID, cm := range m.connMetrics {
		// Check for unusual inactivity
		if !cm.LastSeen.IsZero() && now.Sub(cm.LastSeen) > m.inactivityThreshold {
			m.triggerAlert(ctx, Alert{
				Type:         "unusual_inactivity",
				Severity:     "low",
				ConnectionID: connID,
				Message:      fmt.Sprintf("Inactive for %v", now.Sub(cm.LastSeen).Round(time.Minute)),
				Metadata: map[string]interface{}{
					"last_seen":           cm.LastSeen,
					"inactivity_duration": now.Sub(cm.LastSeen).String(),
				},
			})
		}

		// Check for search rate spikes
		m.checkSearchSpike(ctx, connID, cm)
	}
}

// checkSearchSpike detects abnormal search rate increases.
func (m *MonitoringService) checkSearchSpike(ctx context.Context, connID string, cm *ConnectionMetrics) {
	history := cm.SearchRate.GetHistoryWithCurrent()
	if len(history) < 3 {
		return // Not enough data for baseline
	}

	// Calculate baseline from historical data (excluding current bucket)
	var sum float64
	var count int
	for i := 0; i < len(history)-1; i++ {
		sum += history[i].Value
		count++
	}
	if count == 0 {
		return
	}

	baseline := sum / float64(count)
	cm.BaselineSearchRate = baseline

	// Check if current rate exceeds threshold
	currentRate := history[len(history)-1].Value
	if currentRate > baseline*m.searchSpikeMultiplier && baseline > 0 {
		m.triggerAlert(ctx, Alert{
			Type:         "search_rate_spike",
			Severity:     "medium",
			ConnectionID: connID,
			Message:      fmt.Sprintf("Search rate spike detected: %.1f searches/5min (baseline: %.1f)", currentRate, baseline),
			Metadata: map[string]interface{}{
				"current_rate":  currentRate,
				"baseline_rate": baseline,
				"multiplier":    currentRate / baseline,
			},
		})
	}
}

// triggerAlert publishes an alert event with deduplication.
func (m *MonitoringService) triggerAlert(ctx context.Context, alert Alert) {
	// Deduplication check
	key := fmt.Sprintf("%s:%s", alert.Type, alert.ConnectionID)

	m.alertMu.Lock()
	if lastAlert, exists := m.recentAlerts[key]; exists {
		if time.Since(lastAlert) < m.alertDedupWindow {
			m.alertMu.Unlock()
			return // Suppress duplicate
		}
	}
	m.recentAlerts[key] = time.Now()
	m.alertMu.Unlock()

	// Set timestamp
	alert.Timestamp = time.Now()

	// Log the alert
	m.log.Warn("Security alert triggered",
		"type", alert.Type,
		"severity", alert.Severity,
		"connection", alert.ConnectionID,
		"message", alert.Message,
	)

	// Publish to event bus
	if m.bus != nil {
		event := bus.Event{
			Type:    bus.TopicAlertTriggered,
			Source:  "connection-monitoring",
			Payload: alert,
		}
		_ = m.bus.Publish(ctx, bus.TopicAlertTriggered, event)
	}
}

// GetConnectionMetrics returns current metrics for a connection.
func (m *MonitoringService) GetConnectionMetrics(connID string) (*ConnectionMetrics, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cm, exists := m.connMetrics[connID]
	if !exists {
		return nil, false
	}

	// Return a copy
	return &ConnectionMetrics{
		SearchRate:         cm.SearchRate,
		LastIP:             cm.LastIP,
		LastSeen:           cm.LastSeen,
		BaselineSearchRate: cm.BaselineSearchRate,
	}, true
}

// GetAllAlerts returns recent alerts (for debugging/admin UI).
func (m *MonitoringService) GetAllAlerts() map[string]time.Time {
	m.alertMu.RLock()
	defer m.alertMu.RUnlock()

	alerts := make(map[string]time.Time, len(m.recentAlerts))
	for k, v := range m.recentAlerts {
		alerts[k] = v
	}
	return alerts
}

// CleanupStaleMetrics removes metrics for connections not seen in a long time.
func (m *MonitoringService) CleanupStaleMetrics(maxAge time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	removed := 0

	for connID, cm := range m.connMetrics {
		if !cm.LastSeen.IsZero() && now.Sub(cm.LastSeen) > maxAge {
			delete(m.connMetrics, connID)
			removed++
		}
	}

	// Also cleanup old alerts
	m.alertMu.Lock()
	for key, alertTime := range m.recentAlerts {
		if now.Sub(alertTime) > m.alertDedupWindow*2 {
			delete(m.recentAlerts, key)
		}
	}
	m.alertMu.Unlock()

	if removed > 0 {
		m.log.Info("Cleaned up stale connection metrics", "removed", removed)
	}

	return removed
}
