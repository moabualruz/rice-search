package query

import (
	"context"
	"testing"

	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

func TestModelBasedUnderstandingDisabled(t *testing.T) {
	log := logger.Default()
	model := NewModelBasedUnderstanding(log)
	ctx := context.Background()

	// Should be disabled by default
	if model.IsEnabled() {
		t.Error("expected model to be disabled by default")
	}

	// Parse should return error when disabled
	result, err := model.Parse(ctx, "test query")
	if err != ErrModelNotEnabled {
		t.Errorf("expected ErrModelNotEnabled, got %v", err)
	}
	if result != nil {
		t.Error("expected nil result when disabled")
	}
}

func TestModelBasedUnderstandingEnable(t *testing.T) {
	log := logger.Default()
	model := NewModelBasedUnderstanding(log)

	// Enable
	model.SetEnabled(true)
	if !model.IsEnabled() {
		t.Error("expected model to be enabled")
	}

	// Disable
	model.SetEnabled(false)
	if model.IsEnabled() {
		t.Error("expected model to be disabled")
	}
}

func TestModelBasedUnderstandingNotImplemented(t *testing.T) {
	log := logger.Default()
	model := NewModelBasedUnderstanding(log)
	ctx := context.Background()

	// Enable model
	model.SetEnabled(true)

	// Parse should still return error (not implemented yet)
	result, err := model.Parse(ctx, "test query")
	if err != ErrModelNotEnabled {
		t.Errorf("expected ErrModelNotEnabled (not implemented), got %v", err)
	}
	if result != nil {
		t.Error("expected nil result (not implemented)")
	}
}

func TestModelBasedUnderstandingEmptyQuery(t *testing.T) {
	log := logger.Default()
	model := NewModelBasedUnderstanding(log)
	ctx := context.Background()

	model.SetEnabled(true)

	// Even when enabled, empty query should return error (not implemented)
	result, err := model.Parse(ctx, "")
	if err != ErrModelNotEnabled {
		t.Errorf("expected ErrModelNotEnabled, got %v", err)
	}
	if result != nil {
		t.Error("expected nil result")
	}
}
