package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// defaultHealthcheckPort passt zu envconfig.Load()s Vorgabe für
// DMARC_LISTEN_ADDR (":8080") — hier eigenständig, weil runHealthcheck
// bewusst nicht durch newApp()/envconfig.Load() geht (siehe dort).
const defaultHealthcheckPort = "8080"

// runHealthcheck prüft, ob der lokale HTTP-Server antwortet — für das
// Dockerfile-`HEALTHCHECK` (Distroless-Images haben kein curl/wget, ein
// zweiter Aufruf dieser ohnehin vorhandenen Binärdatei braucht kein
// zusätzliches Werkzeug im Image). Bewusst NICHT über newApp()/
// envconfig.Load() verdrahtet: eine Liveness-Prüfung soll nicht an einer
// kurzzeitig ungültigen IMAP-/OIDC-Konfiguration scheitern, sondern nur
// daran, ob der HTTP-Server selbst antwortet.
func runHealthcheck(ctx context.Context, _ []string) error {
	port := defaultHealthcheckPort
	if addr := os.Getenv("DMARC_LISTEN_ADDR"); addr != "" {
		if _, p, err := net.SplitHostPort(addr); err == nil && p != "" {
			port = p
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	//nolint:gosec // G704: host ist fest "127.0.0.1" — nur der Port kommt
	// aus DMARC_LISTEN_ADDR (bzw. net.SplitHostPort-Fallback), das ist
	// dieselbe Umgebungsvariable, mit der auch der eigene HTTP-Server
	// dieses Containers gebunden wurde. Kein extern beeinflussbares Ziel.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+port+"/gesund", nil)
	if err != nil {
		return fmt.Errorf("healthcheck-anfrage konnte nicht gebaut werden: %w", err)
	}

	//nolint:gosec // G704: siehe Begründung an der Anfrage oben.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("healthcheck fehlgeschlagen: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck: unerwarteter status %d", resp.StatusCode)
	}
	return nil
}
