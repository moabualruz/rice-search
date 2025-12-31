package observability

import (
	"context"
	"sync"
	"time"

	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

// Service provides observability features like query logging.
type Service struct {
	mu       sync.RWMutex
	queryLog []QueryLogEntry
	maxLogs  int
	log      *logger.Logger
}

// NewService creates a new observability service.
func NewService(log *logger.Logger) *Service {
	return &Service{
		queryLog: make([]QueryLogEntry, 0, 1000),
		maxLogs:  100000, // Keep last 100k queries
		log:      log,
	}
}

// LogQuery records a search query.
func (s *Service) LogQuery(entry QueryLogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.queryLog = append(s.queryLog, entry)

	// Trim if too large (simple FIFO)
	if len(s.queryLog) > s.maxLogs {
		// Remove first 10% to amortize resize cost
		removeCount := s.maxLogs / 10
		s.queryLog = s.queryLog[removeCount:]
	}
}

// GetQueriesInRange returns queries within a date range and optionally filtered by store.
func (s *Service) GetQueriesInRange(ctx context.Context, store string, from, to time.Time) ([]QueryLogEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []QueryLogEntry

	// Naive scan (optimization: could use binary search on timestamp if sorted,
	// but append guarantees sorted order by time usually)
	for _, entry := range s.queryLog {
		// Filter by store
		if store != "" && entry.Store != store {
			continue
		}

		// Filter by date range
		if entry.Timestamp.Before(from) || entry.Timestamp.After(to) {
			continue
		}

		results = append(results, entry)
	}

	return results, nil
}
