// Package logging stellt den zentral konfigurierten slog.Logger der
// Anwendung bereit. Lokal ein lesbares Textformat, in CI/Headless-Betrieb
// JSON, damit Logs maschinell auswertbar bleiben.
package logging

import (
	"log/slog"
	"os"
)

// Option steuert den Aufbau des Loggers.
type Option func(*options)

type options struct {
	json  bool
	level slog.Level
}

// WithJSON schaltet auf JSON-Ausgabe um — im Container-Betrieb sinnvoll,
// wenn eine Log-Aggregation strukturierte Zeilen erwartet.
func WithJSON(json bool) Option {
	return func(o *options) { o.json = json }
}

// WithLevel setzt das minimale Log-Level.
func WithLevel(level slog.Level) Option {
	return func(o *options) { o.level = level }
}

// New erzeugt den Standard-Logger der Anwendung und setzt ihn als
// slog.Default. Aufruf einmalig in der Composition Root.
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
