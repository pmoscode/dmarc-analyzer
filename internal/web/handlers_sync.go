package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/pmoscode/dmarc-analyzer/internal/app/syncjob"
)

// redirectBack redirects back to the page the request came from (the
// Referer header, path only — never the whole, potentially foreign URL,
// see below), otherwise to "/". For forms like /abgleich that can be
// triggered from practically any page (the sync button sits in
// layout.html, so on every page) and should return there afterward,
// instead of always jumping to the overview.
func redirectBack(w http.ResponseWriter, r *http.Request) {
	target := "/"
	if ref := r.Referer(); ref != "" {
		if u, err := url.Parse(ref); err == nil && u.Path != "" && !strings.HasPrefix(u.Path, "//") {
			// Only take over the path (+ query), never scheme/host from
			// the Referer — it comes from a browser header, and despite
			// the CSRF/host-checked origin there's no reason to trust it
			// literally as a redirect target (open-redirect caution).
			// "//" as a path prefix is also excluded: a scheme-relative
			// "//evil.example" would be treated by the browser as an
			// independent redirect target, not as a path on this server.
			target = u.Path
			if u.RawQuery != "" {
				target += "?" + u.RawQuery
			}
		}
	}
	//nolint:gosec // G710: target is restricted above to a plain path
	// starting with "/" (not "//"), with no scheme/host — no target
	// outside this server is reachable.
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// handleSyncStart kicks off a sync run across all configured accounts
// (MIGRATIONSPLAN.md section 7: "POST /abgleich"). If one is already
// running, that's not an error — the user sees the running progress
// anyway (see /ereignisse).
func (s *Server) handleSyncStart(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.SyncJob.Start(); err != nil && !errors.Is(err, syncjob.ErrAlreadyRunning) {
		s.serverError(w, r, err)
		return
	}
	redirectBack(w, r)
}

// handleSyncCancel cancels a running sync (MIGRATIONSPLAN.md section 7:
// "POST /abgleich/abbrechen") — not an error if none is running.
func (s *Server) handleSyncCancel(w http.ResponseWriter, r *http.Request) {
	s.deps.SyncJob.Cancel()
	redirectBack(w, r)
}

// sseState is the JSON form of a syncjob.State for /ereignisse — its own
// field names (lowerCamelCase, nested progress/total objects) instead of
// marshaling syncjob.State directly, so static/app.js gets a stable,
// deliberately designed format instead of the Go-internal structure.
type sseState struct {
	Status         string `json:"status"`
	CurrentAccount string `json:"currentAccount,omitempty"`
	Progress       struct {
		Processed int `json:"processed"`
		New       int `json:"new"`
		Skipped   int `json:"skipped"`
		Failed    int `json:"failed"`
	} `json:"progress"`
	Total struct {
		New     int `json:"new"`
		Skipped int `json:"skipped"`
		Failed  int `json:"failed"`
	} `json:"total"`
	Err string `json:"err,omitempty"`
}

func newSSEState(s syncjob.State) sseState {
	out := sseState{Status: string(s.Status), CurrentAccount: s.CurrentAccount}
	out.Progress.Processed = s.Progress.Processed
	out.Progress.New = s.Progress.New
	out.Progress.Skipped = s.Progress.Skipped
	out.Progress.Failed = s.Progress.Failed
	out.Total.New = s.Total.New
	out.Total.Skipped = s.Total.Skipped
	out.Total.Failed = s.Total.Failed
	if s.Err != nil {
		out.Err = s.Err.Error()
	}
	return out
}

// handleEvents delivers sync progress as Server-Sent Events
// (MIGRATIONSPLAN.md section 7: "GET /ereignisse"). One event
// immediately on connection (current state), then one per change, until
// the browser closes the connection (r.Context() then ends).
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "This server does not support SSE", http.StatusInternalServerError)
		return
	}

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch, unsubscribe := s.deps.SyncJob.Subscribe()
	defer unsubscribe()

	for {
		select {
		case state, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(newSSEState(state))
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
