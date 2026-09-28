package web

import (
	"encoding/csv"
	"net/http"

	"github.com/pmoscode/dmarc-analyzer/internal/app/exportdata"
)

// exportPageSize is the page size a CSV export uses to load from the
// repository internally — bigger than reportsPageSize/sourcesPageSize
// (fewer database round trips for a typically large export), but still
// one page at a time: the entire filtered set is never fully in memory
// (MIGRATIONSPLAN.md milestone M4: "CSV export of the entire filtered
// set, streamed").
const exportPageSize = 500

// handleExportReportsCSV returns ALL reports matching the current filter
// as a CSV download — unlike the former Fyne UI (which only exported the
// pages already loaded and visible, see internal/ui/reports.View.exportCSV)
// this export loads more itself, page by page, written directly into the
// response.
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
		// Flush after every page instead of at the end: for a large
		// filtered set, the browser should see download progress, not get
		// everything at once after the last byte.
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

// handleExportSourcesCSV: see handleExportReportsCSV, for the sending
// sources view.
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

// handleExportDomainsCSV: see handleExportReportsCSV, for the domains
// view.
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

// handleExportFailedRecordsCSV: see handleExportReportsCSV, for the
// failures view.
func (s *Server) handleExportFailedRecordsCSV(w http.ResponseWriter, r *http.Request) {
	filter := parseFailedRecordsFilter(r)

	h := w.Header()
	h.Set("Content-Type", "text/csv; charset=utf-8")
	h.Set("Content-Disposition", `attachment; filename="fehlschlaege.csv"`)
	h.Set("X-Content-Type-Options", "nosniff")

	cw := csv.NewWriter(w)
	if err := exportdata.WriteFailedRecordsCSVHeader(cw); err != nil {
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
		page, err := s.deps.FailedRecords.List(r.Context(), q)
		if err != nil {
			s.logExportError(r, err)
			return
		}
		for _, rec := range page.Records {
			if err := exportdata.WriteFailedRecordCSVRow(cw, rec); err != nil {
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

// logExportError logs an error that occurs in the middle of streaming a
// CSV download — unlike s.serverError, no HTTP error status can be sent
// anymore at this point (the 200 status plus headers already reached the
// browser with the first byte written); the download visibly breaks off
// incomplete for the user, but the server-side log at least keeps the
// cause.
func (s *Server) logExportError(r *http.Request, err error) {
	s.logger.Error("csv export aborted", "path", r.URL.Path, "error", err)
}
