// Package logger provides structured logging utilities.
package logger

import (
	"context"
	"log/slog"
	"os"
)

// Logger wraps slog.Logger with additional context.
type Logger struct {
	*slog.Logger
}

// New creates a new logger with the specified level and format.
func New(level, format string) *Logger {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level: parseLevel(level),
	}

	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return &Logger{
		Logger: slog.New(handler),
	}
}

// WithContext returns a logger with context values.
func (l *Logger) WithContext(ctx context.Context) *Logger {
	// Extract request ID from context if present
	if reqID := ctx.Value("request_id"); reqID != nil {
		return &Logger{
			Logger: l.With("request_id", reqID),
		}
	}
	return l
}

// WithStore returns a logger with store context.
func (l *Logger) WithStore(store string) *Logger {
	return &Logger{
		Logger: l.With("store", store),
	}
}

// WithError returns a logger with error context.
func (l *Logger) WithError(err error) *Logger {
	return &Logger{
		Logger: l.With("error", err.Error()),
	}
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Default returns the default logger.
func Default() *Logger {
	return New("info", "text")
}
