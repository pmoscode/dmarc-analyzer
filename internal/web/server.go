// Package web implementiert die eingebettete Web-Oberfläche, die die
// Fyne-Oberfläche ablöst (siehe MIGRATIONSPLAN.md). Ruft wie zuvor
// internal/ui ausschließlich Use Cases aus internal/app auf, nie direkt
// einen Infra-Adapter (AGENTS.md).
package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/platform/paths"
)

// Options steuert Adresse, Browserverhalten und Vorlagen-Quelle.
type Options struct {
	// Addr ist eine feste Adresse ("127.0.0.1:8080"). Leer bedeutet:
	// zufälliger freier Port auf 127.0.0.1 (Migrationsplan E-4).
	Addr string
	// Dev liest Vorlagen/Statik von der Festplatte statt eingebettet —
	// für Entwicklung ohne Neubau bei jeder Änderung (Arbeitsverzeichnis
	// muss die Repository-Wurzel sein).
	Dev bool
	// Logger — nil verwendet slog.Default().
	Logger *slog.Logger
}

// Server ist die eingebettete Web-Oberfläche.
type Server struct {
	deps    Dependencies
	logger  *slog.Logger
	auth    *auth
	views   *views
	devMode bool

	staticFS fs.FS

	httpServer   *http.Server
	listener     net.Listener
	instancePath string
}

// New baut den Server auf (Vorlagen laden, Instanz-Geheimnis erzeugen),
// bindet aber noch keinen Port — das übernimmt Start(). Getrennt, damit
// Konstruktionsfehler (z. B. kaputte Vorlage) sich ohne Netzwerk-Seiteneffekt
// melden.
func New(deps Dependencies, opts Options) (*Server, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	a, err := newAuth()
	if err != nil {
		return nil, err
	}

	v, err := newViews(opts.Dev)
	if err != nil {
		return nil, fmt.Errorf("vorlagen konnten nicht geladen werden: %w", err)
	}

	sfs, err := staticFS(opts.Dev)
	if err != nil {
		return nil, err
	}

	s := &Server{
		deps:     deps,
		logger:   logger,
		auth:     a,
		views:    v,
		devMode:  opts.Dev,
		staticFS: sfs,
	}
	s.httpServer = &http.Server{
		Handler:           s.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	if err := s.bind(opts.Addr); err != nil {
		return nil, err
	}

	return s, nil
}

// bind bindet den Server an addr (leer: zufälliger Port auf 127.0.0.1).
// Lehnt jede nicht-loopback Adresse ab (MIGRATIONSPLAN.md Abschnitt 5:
// "Nur Loopback") — auch wenn addr per --adresse explizit gesetzt wurde.
func (s *Server) bind(addr string) error {
	if addr == "" {
		addr = "127.0.0.1:0"
	}

	ln, err := new(net.ListenConfig).Listen(context.Background(), "tcp", addr)
	if err != nil {
		return fmt.Errorf("adresse %q konnte nicht gebunden werden: %w", addr, err)
	}

	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok || !tcpAddr.IP.IsLoopback() {
		_ = ln.Close()
		return fmt.Errorf("nur Loopback-Adressen (127.0.0.1) sind erlaubt, nicht %q", addr)
	}

	s.listener = ln
	return nil
}

// Start startet den Server im Hintergrund, schreibt instance.json und
// liefert die Einmal-Anmelde-URL, mit der der Browser eine Sitzung
// erhält.
func (s *Server) Start(context.Context) (string, error) {
	go func() {
		if err := s.httpServer.Serve(s.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("server beendet", "error", err)
		}
	}()

	if err := s.writeInstanceFile(); err != nil {
		return "", err
	}

	code, err := s.auth.issueCode()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("http://%s/anmelden?code=%s", s.listener.Addr().String(), code), nil
}

// Shutdown fährt den Server sauber herunter (offene Anfragen fertig) und
// entfernt instance.json (MIGRATIONSPLAN.md E-3, Abschnitt 5
// "Aufräumen").
func (s *Server) Shutdown(ctx context.Context) error {
	err := s.httpServer.Shutdown(ctx)
	if s.instancePath != "" {
		_ = os.Remove(s.instancePath)
	}
	return err
}

// Addr liefert die tatsächlich gebundene Adresse ("127.0.0.1:PORT").
func (s *Server) Addr() string {
	return s.listener.Addr().String()
}

// allowedHosts wird bei jeder Anfrage neu aus der gebundenen Adresse
// berechnet (siehe middleware.go requireHost) statt einmalig bei der
// Konstruktion — dadurch kann s.routes() bereits in New() gebaut werden,
// bevor bind() den Port kennt.
func (s *Server) allowedHosts() map[string]bool {
	tcpAddr, ok := s.listener.Addr().(*net.TCPAddr)
	if !ok {
		return nil
	}
	port := tcpAddr.Port
	return map[string]bool{
		fmt.Sprintf("127.0.0.1:%d", port): true,
		fmt.Sprintf("localhost:%d", port): true,
	}
}

// instanceFile ist der Inhalt von instance.json — Grundlage für die
// Einzelinstanz-Erkennung (Meilenstein M1) und POST /intern/code.
type instanceFile struct {
	Port   int    `json:"port"`
	PID    int    `json:"pid"`
	Secret string `json:"secret"`
}

func (s *Server) writeInstanceFile() error {
	dir, err := paths.ConfigDir()
	if err != nil {
		return err
	}
	s.instancePath = filepath.Join(dir, "instance.json")

	tcpAddr, ok := s.listener.Addr().(*net.TCPAddr)
	if !ok {
		return fmt.Errorf("adresse des servers ist unerwartet keine TCP-Adresse")
	}

	//nolint:gosec // G117: Secret gehört hier absichtlich hinein —
	// instance.json ist die dokumentierte Ablage des Instanz-Geheimnisses
	// (MIGRATIONSPLAN.md Abschnitt 5), mit Rechten 0600 geschrieben
	// (siehe unten).
	data, err := json.Marshal(instanceFile{
		Port:   tcpAddr.Port,
		PID:    os.Getpid(),
		Secret: s.auth.instanceSecret,
	})
	if err != nil {
		return fmt.Errorf("instanzdatei konnte nicht kodiert werden: %w", err)
	}

	if err := os.WriteFile(s.instancePath, data, 0o600); err != nil {
		return fmt.Errorf("instanzdatei konnte nicht geschrieben werden: %w", err)
	}
	return nil
}
