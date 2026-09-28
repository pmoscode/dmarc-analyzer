package web

import "net/http"

// handleHealthz is the unauthenticated healthcheck for Docker (the
// HEALTHCHECK instruction in the Dockerfile) or an orchestrator
// (liveness/readiness probe) — always responds 200 as long as the HTTP
// server itself is running. No database access: a single slow query
// shouldn't falsely mark the container as "unhealthy".
func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
