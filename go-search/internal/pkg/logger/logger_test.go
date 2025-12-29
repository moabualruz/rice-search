package logger

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name   string
		level  string
		format string
	}{
		{"debug text", "debug", "text"},
		{"info json", "info", "json"},
		{"warn text", "warn", "text"},
		{"error json", "error", "json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := New(tt.level, tt.format)
			if logger == nil {
				t.Fatal("New() returned nil")
			}
			if logger.Logger == nil {
				t.Fatal("New() returned logger with nil slog.Logger")
			}
		})
	}
}

func TestLogger_WithContext(t *testing.T) {
	logger := New("info", "text")

	// Context without request ID
	ctx := context.Background()
	l := logger.WithContext(ctx)
	if l == nil {
		t.Fatal("WithContext() returned nil")
	}

	// Context with request ID
	ctx = context.WithValue(ctx, "request_id", "req-123")
	l = logger.WithContext(ctx)
	if l == nil {
		t.Fatal("WithContext() with request_id returned nil")
	}
}

func TestLogger_WithStore(t *testing.T) {
	logger := New("info", "text")

	l := logger.WithStore("test-store")
	if l == nil {
		t.Fatal("WithStore() returned nil")
	}
}

func TestLogger_WithError(t *testing.T) {
	logger := New("info", "text")

	l := logger.WithError(context.DeadlineExceeded)
	if l == nil {
		t.Fatal("WithError() returned nil")
	}
}

func TestDefault(t *testing.T) {
	logger := Default()
	if logger == nil {
		t.Fatal("Default() returned nil")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"unknown", slog.LevelInfo}, // default
		{"", slog.LevelInfo},        // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := parseLevel(tt.input); got != tt.want {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestLogger_OutputFormat(t *testing.T) {
	t.Run("json format", func(t *testing.T) {
		var buf bytes.Buffer
		handler := slog.NewJSONHandler(&buf, nil)
		logger := &Logger{Logger: slog.New(handler)}

		logger.Info("test message")

		output := buf.String()
		if !strings.Contains(output, `"msg":"test message"`) {
			t.Errorf("JSON output should contain msg field, got: %s", output)
		}
	})

	t.Run("text format", func(t *testing.T) {
		var buf bytes.Buffer
		handler := slog.NewTextHandler(&buf, nil)
		logger := &Logger{Logger: slog.New(handler)}

		logger.Info("test message")

		output := buf.String()
		if !strings.Contains(output, "test message") {
			t.Errorf("Text output should contain message, got: %s", output)
		}
	})
}
