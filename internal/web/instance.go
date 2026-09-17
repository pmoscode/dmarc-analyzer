package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pmoscode/dmarc-analyzer/internal/platform/paths"
)

// RunningInstance sind die aus instance.json gelesenen Verbindungsdaten
// einer möglicherweise bereits laufenden Instanz (MIGRATIONSPLAN.md
// Abschnitt 3: "Programm läuft bereits" — "Zweiter Start erkennt die
// laufende Instanz, lässt sie einen neuen Einmal-Link erzeugen, öffnet
// ihn im Browser und beendet sich sofort").
type RunningInstance struct {
	Port   int
	Secret string
}

// FindRunningInstance liest instance.json, falls vorhanden. ok ist
// false, wenn keine Datei existiert oder sie sich nicht lesen/parsen
// lässt oder unvollständig ist — ein verwaister oder kaputter Eintrag
// ist damit kein Fehler: der Aufrufer startet dann normal einen eigenen
// Server, der writeInstanceFile() den Eintrag überschreiben lässt
// (MIGRATIONSPLAN.md Abschnitt 5 "Aufräumen": "ein verwaister Eintrag
// ... wird beim nächsten Start übernommen").
func FindRunningInstance() (RunningInstance, bool) {
	dir, err := paths.ConfigDir()
	if err != nil {
		return RunningInstance{}, false
	}

	data, err := os.ReadFile(filepath.Join(dir, "instance.json"))
	if err != nil {
		return RunningInstance{}, false
	}

	var f instanceFile
	if err := json.Unmarshal(data, &f); err != nil {
		return RunningInstance{}, false
	}
	if f.Port <= 0 || f.Secret == "" {
		return RunningInstance{}, false
	}
	return RunningInstance{Port: f.Port, Secret: f.Secret}, true
}

// internalCodeResponse ist die Antwort von POST /intern/code.
type internalCodeResponse struct {
	URL string `json:"url"`
}

// RequestLoginURL fragt bei inst (siehe FindRunningInstance) einen
// frischen Einmal-Anmeldelink an — für den zweiten Programmstart
// (MIGRATIONSPLAN.md Abschnitt 3/7: "POST /intern/code"). Ein Fehler
// bedeutet meist einen verwaisten Eintrag (Prozess tot, Port
// zwischenzeitlich von einem anderen Programm belegt): der Aufrufer
// sollte dann selbst einen Server starten, nicht abbrechen.
func RequestLoginURL(ctx context.Context, inst RunningInstance) (string, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", inst.Port)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+addr+"/intern/code", strings.NewReader(inst.Secret))
	if err != nil {
		return "", fmt.Errorf("anfrage an laufende instanz konnte nicht gebaut werden: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("laufende instanz auf port %d antwortet nicht (vermutlich verwaister eintrag): %w", inst.Port, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("laufende instanz auf port %d lehnt anfrage ab (status %d)", inst.Port, resp.StatusCode)
	}

	var out internalCodeResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&out); err != nil {
		return "", fmt.Errorf("antwort der laufenden instanz konnte nicht gelesen werden: %w", err)
	}
	if out.URL == "" {
		return "", fmt.Errorf("laufende instanz auf port %d lieferte keine anmelde-url", inst.Port)
	}
	return out.URL, nil
}

// maxInternalCodeBodySize begrenzt den Anfragekörper von POST
// /intern/code — er trägt nur das Instanz-Geheimnis als Hex-Zeichenkette
// (64 Zeichen für 32 Zufallsbytes, siehe auth.go randomHex), niemals mehr.
const maxInternalCodeBodySize = 1024

// handleInternalCode stellt einer bereits laufenden Instanz einen
// frischen Einmal-Anmeldelink aus (MIGRATIONSPLAN.md Abschnitt 7: "Neuen
// Einmal-Code ausstellen (nur mit Instanz-Geheimnis, für den zweiten
// Programmstart)"). Authentifiziert über das Instanz-Geheimnis aus
// instance.json im Anfragekörper, nicht über eine Sitzung — ein zweiter
// Prozessstart hat noch keine; deshalb bewusst außerhalb von
// requireSession **und** requireCSRF verdrahtet (routes.go): es handelt
// sich nicht um eine browserseitige Anfrage mit Cookies, sondern um einen
// direkten HTTP-Aufruf des zweiten CLI-Prozesses.
func (s *Server) handleInternalCode(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxInternalCodeBodySize))
	if err != nil {
		http.Error(w, "anfrage konnte nicht gelesen werden", http.StatusBadRequest)
		return
	}

	if !s.auth.validInstanceSecret(string(body)) {
		http.Error(w, "ungültiges instanz-geheimnis", http.StatusUnauthorized)
		return
	}

	code, err := s.auth.issueCode()
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	s.writeJSON(w, r, internalCodeResponse{URL: "http://" + r.Host + "/anmelden?code=" + code})
}
