package sqlite

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/sources"
)

const (
	defaultSourcePageSize = 50
	maxSourcePageSize     = 500
)

// SourceStatsRepository implements sources.Repository against SQLite:
// SQL aggregation over records grouped by source IP, analogous to
// StatisticsRepository — enrichment (PTR/service detection) is filled in
// afterwards by the application layer (see domain/sources.Stat.Enrichment).
type SourceStatsRepository struct {
	db *sql.DB
}

var _ sources.Repository = (*SourceStatsRepository)(nil)

// NewSourceStatsRepository creates a ready-to-use repository.
func NewSourceStatsRepository(db *sql.DB) *SourceStatsRepository {
	return &SourceStatsRepository{db: db}
}

// Query returns a page of statistics aggregated by source IP.
func (r *SourceStatsRepository) Query(ctx context.Context, q sources.Query) (sources.Page, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = defaultSourcePageSize
	}
	if limit > maxSourcePageSize {
		limit = maxSourcePageSize
	}

	whereClauses, args := sourcesWhere(q)
	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	orderBy, cursorSQL, cursorArgs, err := sourcesOrderAndCursor(q)
	if err != nil {
		return sources.Page{}, err
	}
	args = append(args, cursorArgs...)

	// limit+1: request one extra result to detect whether another page
	// exists without a separate COUNT(*) (like reportquery.go).
	args = append(args, limit+1)

	query := fmt.Sprintf(`
		SELECT source_ip, total, passed, dkim_passed, spf_passed, first_seen, last_seen FROM (
			SELECT rec.source_ip AS source_ip,
				SUM(rec.message_count) AS total,
				COALESCE(SUM(CASE WHEN rec.dkim_result = 'pass' OR rec.spf_result = 'pass' THEN rec.message_count ELSE 0 END), 0) AS passed,
				COALESCE(SUM(CASE WHEN rec.dkim_result = 'pass' THEN rec.message_count ELSE 0 END), 0) AS dkim_passed,
				COALESCE(SUM(CASE WHEN rec.spf_result = 'pass' THEN rec.message_count ELSE 0 END), 0) AS spf_passed,
				MIN(rep.date_begin) AS first_seen,
				MAX(rep.date_end) AS last_seen
			FROM records rec
			JOIN reports rep ON rep.id = rec.report_id
			%s
			GROUP BY rec.source_ip
		) agg
		%s
		%s
		LIMIT ?`, whereSQL, cursorSQL, orderBy)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return sources.Page{}, fmt.Errorf("could not aggregate sending sources: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var stats []sources.Stat
	for rows.Next() {
		var rawIP string
		var total, passed, dkimPassed, spfPassed, firstSeen, lastSeen int64
		if err := rows.Scan(&rawIP, &total, &passed, &dkimPassed, &spfPassed, &firstSeen, &lastSeen); err != nil {
			return sources.Page{}, fmt.Errorf("could not read sending-source row: %w", err)
		}
		ip, err := report.NewSourceIP(rawIP)
		if err != nil {
			return sources.Page{}, fmt.Errorf("stored source IP %q is invalid: %w", rawIP, err)
		}
		stats = append(stats, sources.Stat{
			SourceIP:     ip,
			TotalCount:   int(total),
			PassRate:     rate(passed, total),
			DKIMPassRate: rate(dkimPassed, total),
			SPFPassRate:  rate(spfPassed, total),
			FirstSeen:    time.Unix(firstSeen, 0).UTC(),
			LastSeen:     time.Unix(lastSeen, 0).UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return sources.Page{}, fmt.Errorf("could not fully read sending sources: %w", err)
	}

	page := sources.Page{Stats: stats}
	if len(stats) > limit {
		page.Stats = stats[:limit]
		last := page.Stats[len(page.Stats)-1]
		page.NextCursor = sourceCursor{SourceIP: last.SourceIP.String(), Total: int64(last.TotalCount)}.encode()
	}
	return page, nil
}

func sourcesWhere(q sources.Query) ([]string, []any) {
	var clauses []string
	var args []any

	if q.Period != nil {
		clauses = append(clauses, "rep.date_begin < ? AND rep.date_end > ?")
		args = append(args, q.Period.End.Unix(), q.Period.Begin.Unix())
	}
	if q.Domain != "" {
		clauses = append(clauses, "rep.policy_domain = ?")
		args = append(args, q.Domain)
	}
	return clauses, args
}

// sourcesOrderAndCursor returns the ORDER BY clause and — if q.Cursor is
// set — the WHERE clause and parameters for the second and subsequent
// pages. Since source_ip is unique per row after grouping, the keyset
// comparison (unlike reportquery.go) doesn't need an extra ID column as a
// tiebreaker.
func sourcesOrderAndCursor(q sources.Query) (orderBy, cursorSQL string, args []any, err error) {
	var cur sourceCursor
	if q.Cursor != "" {
		cur, err = decodeSourceCursor(q.Cursor)
		if err != nil {
			return "", "", nil, err
		}
	}

	if q.SortField == sources.SortByIP {
		orderBy = "ORDER BY source_ip ASC"
		if q.Cursor != "" {
			cursorSQL = "WHERE source_ip > ?"
			args = []any{cur.SourceIP}
		}
		return orderBy, cursorSQL, args, nil
	}

	// Default: SortByVolume, largest source first.
	orderBy = "ORDER BY total DESC, source_ip DESC"
	if q.Cursor != "" {
		cursorSQL = "WHERE (total, source_ip) < (?, ?)"
		args = []any{cur.Total, cur.SourceIP}
	}
	return orderBy, cursorSQL, args, nil
}

// sourceCursor is the internal, typed form of sources.Query.Cursor /
// sources.Page.NextCursor.
type sourceCursor struct {
	SourceIP string `json:"ip"`
	Total    int64  `json:"t,omitempty"`
}

func (c sourceCursor) encode() string {
	data, err := json.Marshal(c)
	if err != nil {
		panic(fmt.Sprintf("sourceCursor could not be encoded: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeSourceCursor(s string) (sourceCursor, error) {
	var c sourceCursor
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return sourceCursor{}, fmt.Errorf("could not decode cursor: %w", err)
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return sourceCursor{}, fmt.Errorf("cursor has an invalid format: %w", err)
	}
	return c, nil
}
