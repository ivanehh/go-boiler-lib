package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"
)

// LoggerLevel represents logging levels
type LoggerLevel string

const (
	DebugLevel LoggerLevel = "debug"
	InfoLevel  LoggerLevel = "info"
	WarnLevel  LoggerLevel = "warn"
	ErrorLevel LoggerLevel = "error"
)

// LoggerConfig holds configuration for the logger
type LoggerConfig struct {
	Level      LoggerLevel
	JSONFormat bool
	Output     io.Writer
	TimeFormat string
	AddSource  bool

	// Additional outputs with format specification
	AdditionalOutputs []OutputConfig
}

// OutputConfig specifies an output destination with its format
type OutputConfig struct {
	Writer     io.Writer
	JSONFormat bool
}

// DefaultConfig returns the default logger configuration
func DefaultConfig() LoggerConfig {
	return LoggerConfig{
		Level:             InfoLevel,
		JSONFormat:        false,
		Output:            os.Stdout,
		TimeFormat:        time.RFC3339,
		AddSource:         false,
		AdditionalOutputs: []OutputConfig{},
	}
}

// Logger is a wrapper around slog.Logger with additional functionality
type Logger struct {
	slogger *slog.Logger
	config  LoggerConfig
	mu      sync.RWMutex
}

// New creates a new Logger instance with the provided configuration
func New(config LoggerConfig) *Logger {
	level := getLevelFromString(config.Level)
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: config.AddSource,
	}

	// Create handlers for each output
	var handlers []slog.Handler

	// Main output
	if config.Output != nil {
		if config.JSONFormat {
			handlers = append(handlers, slog.NewJSONHandler(config.Output, opts))
		} else {
			handlers = append(handlers, slog.NewTextHandler(config.Output, opts))
		}
	}

	// Additional outputs
	for _, outputConfig := range config.AdditionalOutputs {
		if outputConfig.Writer != nil {
			if outputConfig.JSONFormat {
				handlers = append(handlers, slog.NewJSONHandler(outputConfig.Writer, opts))
			} else {
				handlers = append(handlers, slog.NewTextHandler(outputConfig.Writer, opts))
			}
		}
	}

	// Create multi handler if we have multiple outputs
	var handler slog.Handler
	if len(handlers) > 1 {
		handler = NewMultiHandler(handlers...)
	} else if len(handlers) == 1 {
		handler = handlers[0]
	} else {
		// Fallback to stdout with text format if no outputs specified
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return &Logger{
		slogger: slog.New(handler),
		config:  config,
	}
}

// getLevelFromString converts LoggerLevel to slog.Level
func getLevelFromString(level LoggerLevel) slog.Level {
	switch level {
	case DebugLevel:
		return slog.LevelDebug
	case InfoLevel:
		return slog.LevelInfo
	case WarnLevel:
		return slog.LevelWarn
	case ErrorLevel:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// MultiHandler implements slog.Handler and writes to multiple handlers
type MultiHandler struct {
	handlers []slog.Handler
}

// NewMultiHandler creates a new MultiHandler that writes to multiple handlers
func NewMultiHandler(handlers ...slog.Handler) *MultiHandler {
	return &MultiHandler{handlers: handlers}
}

// Enabled implements slog.Handler.Enabled
func (h *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle implements slog.Handler.Handle
func (h *MultiHandler) Handle(ctx context.Context, r slog.Record) error {
	var firstErr error
	for _, handler := range h.handlers {
		if err := handler.Handle(ctx, r.Clone()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// WithAttrs implements slog.Handler.WithAttrs
func (h *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return NewMultiHandler(handlers...)
}

// WithGroup implements slog.Handler.WithGroup
func (h *MultiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return NewMultiHandler(handlers...)
}

// With returns a new Logger with the given attributes added to the context
func (l *Logger) With(attrs ...any) *Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	newLogger := &Logger{
		slogger: l.slogger.With(attrs...),
		config:  l.config,
	}
	return newLogger
}

// Debug logs a debug message with the given attributes
func (l *Logger) Debug(msg string, attrs ...any) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.slogger.Debug(msg, attrs...)
}

// Info logs an info message with the given attributes
func (l *Logger) Info(msg string, attrs ...any) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.slogger.Info(msg, attrs...)
}

// Warn logs a warning message with the given attributes
func (l *Logger) Warn(msg string, attrs ...any) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.slogger.Warn(msg, attrs...)
}

// Error logs an error message with the given attributes
func (l *Logger) Error(msg string, attrs ...any) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.slogger.Error(msg, attrs...)
}

// UpdateConfig updates the logger configuration dynamically
func (l *Logger) UpdateConfig(config LoggerConfig) {
	l.mu.Lock()
	defer l.mu.Unlock()

	level := getLevelFromString(config.Level)
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: config.AddSource,
	}

	// Create handlers for each output
	var handlers []slog.Handler

	// Main output
	if config.Output != nil {
		if config.JSONFormat {
			handlers = append(handlers, slog.NewJSONHandler(config.Output, opts))
		} else {
			handlers = append(handlers, slog.NewTextHandler(config.Output, opts))
		}
	}

	// Additional outputs
	for _, outputConfig := range config.AdditionalOutputs {
		if outputConfig.Writer != nil {
			if outputConfig.JSONFormat {
				handlers = append(handlers, slog.NewJSONHandler(outputConfig.Writer, opts))
			} else {
				handlers = append(handlers, slog.NewTextHandler(outputConfig.Writer, opts))
			}
		}
	}

	// Create multi handler if we have multiple outputs
	var handler slog.Handler
	if len(handlers) > 1 {
		handler = NewMultiHandler(handlers...)
	} else if len(handlers) == 1 {
		handler = handlers[0]
	} else {
		// Fallback to stdout with text format if no outputs specified
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	l.slogger = slog.New(handler)
	l.config = config
}
