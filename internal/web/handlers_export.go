package web

import (
	"encoding/csv"
	"net/http"

	"github.com/pmoscode/dmarc-analyzer/internal/app/exportdata"
)

// exportPageSize ist die Seitengröße, mit der ein CSV-Export intern aus
// dem Repository nachlädt — größer als reportsPageSize/sourcesPageSize
// (weniger Datenbank-Roundtrips für einen typischerweise großen Export),
// aber weiterhin eine Seite nach der anderen: der gesamte gefilterte
// Bestand liegt nie komplett im Speicher (MIGRATIONSPLAN.md Meilenstein
// M4: "CSV-Export des gesamten gefilterten Bestands, gestreamt").
const exportPageSize = 500

// handleExportReportsCSV liefert ALLE Reports, die dem aktuellen Filter
// entsprechen, als CSV-Download — im Unterschied zur früheren
// Fyne-Oberfläche (die nur die bereits geladenen, sichtbaren Seiten
// exportierte, siehe internal/ui/reports.View.exportCSV) lädt dieser
// Export selbst nach, seitenweise, direkt in die Antwort geschrieben.
func (s *Server) handleExportReportsCSV(w http.ResponseWriter, r *http.Request) {
	filter, err := parseReportsFilter(r)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	h := w.Header()
	h.Set("Content-Type", "text/csv; charset=utf-8")
	h.Set("Content-Disposition", `attachment; filename="berichte.csv"`)
	h.Set("X-Content-Type-Options", "nosniff")

	cw := csv.NewWriter(w)
	if err := exportdata.WriteReportsCSVHeader(cw); err != nil {
		s.logExportError(r, err)
		return
	}
	flusher, _ := w.(http.Flusher)

	cursor := ""
	for {
		page, err := s.deps.Reports.List(r.Context(), filter.query(cursor, exportPageSize))
		if err != nil {
			s.logExportError(r, err)
			return
		}
		for _, rep := range page.Reports {
			if err := exportdata.WriteReportCSVRow(cw, rep); err != nil {
				s.logExportError(r, err)
				return
			}
		}
		// Nach jeder Seite flushen, statt am Ende: bei einem großen
		// gefilterten Bestand soll der Browser den Download-Fortschritt
		// sehen, nicht erst nach dem letzten Byte alles auf einmal.
		cw.Flush()
		if err := cw.Error(); err != nil {
			s.logExportError(r, err)
			return
		}
		if flusher != nil {
			flusher.Flush()
		}

		if page.NextCursor == "" {
			return
		}
		cursor = page.NextCursor
	}
}

// handleExportSourcesCSV: siehe handleExportReportsCSV, für die
// Sendequellen-Ansicht.
func (s *Server) handleExportSourcesCSV(w http.ResponseWriter, r *http.Request) {
	filter := parseSourcesFilter(r)

	h := w.Header()
	h.Set("Content-Type", "text/csv; charset=utf-8")
	h.Set("Content-Disposition", `attachment; filename="quellen.csv"`)
	h.Set("X-Content-Type-Options", "nosniff")

	cw := csv.NewWriter(w)
	if err := exportdata.WriteSourceStatsCSVHeader(cw); err != nil {
		s.logExportError(r, err)
		return
	}
	flusher, _ := w.(http.Flusher)

	cursor := ""
	for {
		q, err := filter.query(cursor, exportPageSize)
		if err != nil {
			s.logExportError(r, err)
			return
		}
		page, err := s.deps.Sources.List(r.Context(), q)
		if err != nil {
			s.logExportError(r, err)
			return
		}
		for _, stat := range page.Stats {
			if err := exportdata.WriteSourceStatCSVRow(cw, stat); err != nil {
				s.logExportError(r, err)
				return
			}
		}
		cw.Flush()
		if err := cw.Error(); err != nil {
			s.logExportError(r, err)
			return
		}
		if flusher != nil {
			flusher.Flush()
		}

		if page.NextCursor == "" {
			return
		}
		cursor = page.NextCursor
	}
}

// handleExportDomainsCSV: siehe handleExportReportsCSV, für die
// Domains-Ansicht.
func (s *Server) handleExportDomainsCSV(w http.ResponseWriter, r *http.Request) {
	filter := parseDomainsFilter(r)

	h := w.Header()
	h.Set("Content-Type", "text/csv; charset=utf-8")
	h.Set("Content-Disposition", `attachment; filename="domains.csv"`)
	h.Set("X-Content-Type-Options", "nosniff")

	cw := csv.NewWriter(w)
	if err := exportdata.WriteDomainStatsCSVHeader(cw); err != nil {
		s.logExportError(r, err)
		return
	}
	flusher, _ := w.(http.Flusher)

	cursor := ""
	for {
		q, err := filter.query(cursor, exportPageSize)
		if err != nil {
			s.logExportError(r, err)
			return
		}
		page, err := s.deps.Domains.List(r.Context(), q)
		if err != nil {
			s.logExportError(r, err)
			return
		}
		for _, stat := range page.Stats {
			if err := exportdata.WriteDomainStatCSVRow(cw, stat); err != nil {
				s.logExportError(r, err)
				return
			}
		}
		cw.Flush()
		if err := cw.Error(); err != nil {
			s.logExportError(r, err)
			return
		}
		if flusher != nil {
			flusher.Flush()
		}

		if page.NextCursor == "" {
			return
		}
		cursor = page.NextCursor
	}
}

// logExportError protokolliert einen Fehler, der mitten im Streamen
// eines CSV-Downloads auftritt — anders als s.serverError kann hier kein
// HTTP-Fehlerstatus mehr gesendet werden (der 200er-Status samt Kopfzeilen
// ist beim ersten geschriebenen Byte schon beim Browser angekommen); der
// Download bricht für den Nutzer sichtbar unvollständig ab, serverseitig
// bleibt wenigstens die Ursache im Log.
func (s *Server) logExportError(r *http.Request, err error) {
	s.logger.Error("csv-export abgebrochen", "path", r.URL.Path, "error", err)
}
