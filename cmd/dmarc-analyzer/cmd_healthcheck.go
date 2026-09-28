package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// defaultHealthcheckPort matches envconfig.Load()'s default for
// DMARC_LISTEN_ADDR (":8080") — duplicated here because runHealthcheck
// deliberately does not go through newApp()/envconfig.Load() (see below).
const defaultHealthcheckPort = "8080"

// runHealthcheck checks whether the local HTTP server responds — for the
// Dockerfile `HEALTHCHECK` (distroless images have no curl/wget, so a
// second call to this binary, which is already present, needs no extra
// tool in the image). Deliberately NOT wired through newApp()/
// envconfig.Load(): a liveness check shouldn't fail because of a briefly
// invalid IMAP/OIDC configuration — only because the HTTP server itself
// isn't responding.
func runHealthcheck(ctx context.Context, _ []string) error {
	port := defaultHealthcheckPort
	if addr := os.Getenv("DMARC_LISTEN_ADDR"); addr != "" {
		if _, p, err := net.SplitHostPort(addr); err == nil && p != "" {
			port = p
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	//nolint:gosec // G704: host is fixed as "127.0.0.1" — only the port
	// comes from DMARC_LISTEN_ADDR (or the net.SplitHostPort fallback),
	// which is the same environment variable this container's own HTTP
	// server was bound to. Not an externally influenceable target.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+port+"/gesund", nil)
	if err != nil {
		return fmt.Errorf("failed to build healthcheck request: %w", err)
	}

	//nolint:gosec // G704: see rationale on the request above.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("healthcheck failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck: unexpected status %d", resp.StatusCode)
	}
	return nil
}
