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

// redirectBack leitet auf die Seite zurück, von der die Anfrage kam
// (Referer-Header, nur der Pfad — nie das ganze, potenziell fremde URL,
// siehe unten), sonst auf "/". Für Formulare wie /abgleich, die von
// praktisch jeder Seite aus ausgelöst werden können (der Sync-Knopf sitzt
// in layout.html, also auf jeder Seite) und danach dorthin zurückkehren
// sollen, statt immer auf die Übersicht zu springen.
func redirectBack(w http.ResponseWriter, r *http.Request) {
	target := "/"
	if ref := r.Referer(); ref != "" {
		if u, err := url.Parse(ref); err == nil && u.Path != "" && !strings.HasPrefix(u.Path, "//") {
			// Nur den Pfad (+ Query) übernehmen, nie Schema/Host aus dem
			// Referer — der stammt aus einem Browser-Header, dem CSRF-
			// /Host-geprüften Ursprung zum Trotz kein Grund, ihn als
			// Redirect-Ziel wörtlich zu vertrauen (Open-Redirect-Vorsicht).
			// "//" als Pfadanfang zusätzlich ausgeschlossen: ein
			// schema-relatives "//evil.example" würde der Browser als
			// eigenständiges Redirect-Ziel behandeln, nicht als Pfad
			// dieses Servers.
			target = u.Path
			if u.RawQuery != "" {
				target += "?" + u.RawQuery
			}
		}
	}
	//nolint:gosec // G710: target ist oben auf einen reinen, mit "/" (nicht
	// "//") beginnenden Pfad ohne Schema/Host eingeschränkt — kein Ziel
	// außerhalb dieses Servers erreichbar.
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// handleSyncStart stößt einen Sync-Lauf über alle konfigurierten Konten
// an (MIGRATIONSPLAN.md Abschnitt 7: "POST /abgleich"). Läuft bereits
// einer, ist das kein Fehler — der Nutzer sieht ohnehin den laufenden
// Fortschritt (siehe /ereignisse).
func (s *Server) handleSyncStart(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.SyncJob.Start(); err != nil && !errors.Is(err, syncjob.ErrAlreadyRunning) {
		s.serverError(w, r, err)
		return
	}
	redirectBack(w, r)
}

// handleSyncCancel bricht einen laufenden Sync ab (MIGRATIONSPLAN.md
// Abschnitt 7: "POST /abgleich/abbrechen") — kein Fehler, wenn keiner
// läuft.
func (s *Server) handleSyncCancel(w http.ResponseWriter, r *http.Request) {
	s.deps.SyncJob.Cancel()
	redirectBack(w, r)
}

// sseState ist die JSON-Form eines syncjob.State für /ereignisse — eigene
// Feldnamen (lowerCamelCase, verschachtelte progress/total-Objekte) statt
// syncjob.State direkt zu marshalen, damit static/app.js ein stabiles,
// bewusst gestaltetes Format bekommt statt der Go-internen Struktur.
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

// handleEvents liefert den Sync-Fortschritt als Server-Sent Events
// (MIGRATIONSPLAN.md Abschnitt 7: "GET /ereignisse"). Ein Ereignis sofort
// beim Verbindungsaufbau (aktueller Stand), danach eines je Änderung, bis
// der Browser die Verbindung schließt (r.Context() endet dann).
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE wird von diesem Server nicht unterstützt", http.StatusInternalServerError)
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
