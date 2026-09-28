// Package logging provides the application's centrally configured
// slog.Logger. A human-readable text format locally, JSON in CI/headless
// operation so logs stay machine-parseable.
package logging

import (
	"log/slog"
	"os"
)

// Option controls how the logger is built.
type Option func(*options)

type options struct {
	json  bool
	level slog.Level
}

// WithJSON switches to JSON output — useful in container operation when a
// log aggregator expects structured lines.
func WithJSON(json bool) Option {
	return func(o *options) { o.json = json }
}

// WithLevel sets the minimum log level.
func WithLevel(level slog.Level) Option {
	return func(o *options) { o.level = level }
}

// New creates the application's default logger and sets it as
// slog.Default. Called once from the composition root.
func New(opts ...Option) *slog.Logger {
	o := options{level: slog.LevelInfo}
	for _, opt := range opts {
		opt(&o)
	}

	handlerOpts := &slog.HandlerOptions{Level: o.level}

	var handler slog.Handler
	if o.json {
		handler = slog.NewJSONHandler(os.Stderr, handlerOpts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, handlerOpts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}
