package sqlite

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/domainstats"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

const (
	defaultDomainPageSize = 50
	maxDomainPageSize     = 500
)

// DomainStatsRepository implements domainstats.Repository against
// SQLite: SQL aggregation over records/reports grouped by policy domain,
// analogous to SourceStatsRepository.
type DomainStatsRepository struct {
	db *sql.DB
}

var _ domainstats.Repository = (*DomainStatsRepository)(nil)

// NewDomainStatsRepository creates a ready-to-use repository.
func NewDomainStatsRepository(db *sql.DB) *DomainStatsRepository {
	return &DomainStatsRepository{db: db}
}

// Query returns a page of statistics aggregated by policy domain.
func (r *DomainStatsRepository) Query(ctx context.Context, q domainstats.Query) (domainstats.Page, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = defaultDomainPageSize
	}
	if limit > maxDomainPageSize {
		limit = maxDomainPageSize
	}

	whereClauses, args := domainsWhere(q)
	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	orderBy, cursorSQL, cursorArgs, err := domainsOrderAndCursor(q)
	if err != nil {
		return domainstats.Page{}, err
	}
	args = append(args, cursorArgs...)

	// limit+1: request one extra result to detect whether another page
	// exists without a separate COUNT(*) (like reportquery.go/
	// sourcestatsrepo.go).
	args = append(args, limit+1)

	query := fmt.Sprintf(`
		SELECT domain, total, passed, report_count, distinct_sources, first_seen, last_seen FROM (
			SELECT rep.policy_domain AS domain,
				SUM(rec.message_count) AS total,
				COALESCE(SUM(CASE WHEN rec.dkim_result = 'pass' OR rec.spf_result = 'pass' THEN rec.message_count ELSE 0 END), 0) AS passed,
				COUNT(DISTINCT rep.id) AS report_count,
				COUNT(DISTINCT rec.source_ip) AS distinct_sources,
				MIN(rep.date_begin) AS first_seen,
				MAX(rep.date_end) AS last_seen
			FROM records rec
			JOIN reports rep ON rep.id = rec.report_id
			%s
			GROUP BY rep.policy_domain
		) agg
		%s
		%s
		LIMIT ?`, whereSQL, cursorSQL, orderBy)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return domainstats.Page{}, fmt.Errorf("could not aggregate domains: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var stats []domainstats.Stat
	for rows.Next() {
		var rawDomain string
		var total, passed int64
		var reportCount, distinctSources int
		var firstSeen, lastSeen int64
		if err := rows.Scan(&rawDomain, &total, &passed, &reportCount, &distinctSources, &firstSeen, &lastSeen); err != nil {
			return domainstats.Page{}, fmt.Errorf("could not read domain row: %w", err)
		}
		domain, err := report.NewDomainName(rawDomain)
		if err != nil {
			return domainstats.Page{}, fmt.Errorf("stored policy domain %q is invalid: %w", rawDomain, err)
		}
		stats = append(stats, domainstats.Stat{
			Domain:          domain,
			TotalCount:      int(total),
			PassRate:        rate(passed, total),
			ReportCount:     reportCount,
			DistinctSources: distinctSources,
			FirstSeen:       time.Unix(firstSeen, 0).UTC(),
			LastSeen:        time.Unix(lastSeen, 0).UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return domainstats.Page{}, fmt.Errorf("could not fully read domains: %w", err)
	}

	page := domainstats.Page{Stats: stats}
	if len(stats) > limit {
		page.Stats = stats[:limit]
		last := page.Stats[len(page.Stats)-1]
		page.NextCursor = domainCursor{Domain: last.Domain.String(), Total: int64(last.TotalCount)}.encode()
	}
	return page, nil
}

func domainsWhere(q domainstats.Query) ([]string, []any) {
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

// domainsOrderAndCursor returns the ORDER BY clause and — if q.Cursor is
// set — the WHERE clause and parameters for the second and subsequent
// pages. Since domain is unique per row after grouping, the keyset
// comparison (like sourcesOrderAndCursor) doesn't need an extra ID column
// as a tiebreaker.
func domainsOrderAndCursor(q domainstats.Query) (orderBy, cursorSQL string, args []any, err error) {
	var cur domainCursor
	if q.Cursor != "" {
		cur, err = decodeDomainCursor(q.Cursor)
		if err != nil {
			return "", "", nil, err
		}
	}

	if q.SortField == domainstats.SortByDomain {
		orderBy = "ORDER BY domain ASC"
		if q.Cursor != "" {
			cursorSQL = "WHERE domain > ?"
			args = []any{cur.Domain}
		}
		return orderBy, cursorSQL, args, nil
	}

	// Default: SortByVolume, largest domain first.
	orderBy = "ORDER BY total DESC, domain DESC"
	if q.Cursor != "" {
		cursorSQL = "WHERE (total, domain) < (?, ?)"
		args = []any{cur.Total, cur.Domain}
	}
	return orderBy, cursorSQL, args, nil
}

// domainCursor is the internal, typed form of domainstats.Query.Cursor /
// domainstats.Page.NextCursor.
type domainCursor struct {
	Domain string `json:"d"`
	Total  int64  `json:"t,omitempty"`
}

func (c domainCursor) encode() string {
	data, err := json.Marshal(c)
	if err != nil {
		panic(fmt.Sprintf("domainCursor could not be encoded: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeDomainCursor(s string) (domainCursor, error) {
	var c domainCursor
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return domainCursor{}, fmt.Errorf("could not decode cursor: %w", err)
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return domainCursor{}, fmt.Errorf("cursor has an invalid format: %w", err)
	}
	return c, nil
}
