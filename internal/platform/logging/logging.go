// Package logging stellt den zentral konfigurierten slog.Logger der
// Anwendung bereit. Lokal ein lesbares Textformat, in CI/Headless-Betrieb
// JSON, damit Logs maschinell auswertbar bleiben.
package logging

import (
	"io"
	"log/slog"
	"os"
)

// Option steuert den Aufbau des Loggers.
type Option func(*options)

type options struct {
	json   bool
	level  slog.Level
	writer io.Writer
}

// WithJSON schaltet auf JSON-Ausgabe um, z. B. für CI oder Headless-Betrieb.
func WithJSON(json bool) Option {
	return func(o *options) { o.json = json }
}

// WithLevel setzt das minimale Log-Level.
func WithLevel(level slog.Level) Option {
	return func(o *options) { o.level = level }
}

// WithWriter überschreibt das Standardziel os.Stderr — z. B. um zusätzlich
// in eine Log-Datei zu schreiben (io.MultiWriter). Wichtig vor allem für
// den Windows-Release-Build ohne Konsolenfenster (MIGRATIONSPLAN.md M5:
// "-H windowsgui"): ohne eigenes Konsolenfenster verschwindet alles, was
// nur nach os.Stderr geschrieben wird, spurlos — siehe
// cmd/dmarc-analyzer/cmd_web.go.
func WithWriter(w io.Writer) Option {
	return func(o *options) { o.writer = w }
}

// New erzeugt den Standard-Logger der Anwendung und setzt ihn als
// slog.Default. Aufruf einmalig in der Composition Root.
func New(opts ...Option) *slog.Logger {
	o := options{level: slog.LevelInfo, writer: os.Stderr}
	for _, opt := range opts {
		opt(&o)
	}

	handlerOpts := &slog.HandlerOptions{Level: o.level}

	var handler slog.Handler
	if o.json {
		handler = slog.NewJSONHandler(o.writer, handlerOpts)
	} else {
		handler = slog.NewTextHandler(o.writer, handlerOpts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}
