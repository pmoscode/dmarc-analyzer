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

// DomainStatsRepository implementiert domainstats.Repository gegen
// SQLite: nach Policy-Domain gruppierte SQL-Aggregation über
// records/reports, analog zu SourceStatsRepository.
type DomainStatsRepository struct {
	db *sql.DB
}

var _ domainstats.Repository = (*DomainStatsRepository)(nil)

// NewDomainStatsRepository erzeugt ein einsatzbereites Repository.
func NewDomainStatsRepository(db *sql.DB) *DomainStatsRepository {
	return &DomainStatsRepository{db: db}
}

// Query liefert eine Seite nach Policy-Domain aggregierter Statistiken.
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

	// limit+1: ein zusätzliches Ergebnis anfordern, um ohne separates
	// COUNT(*) zu erkennen, ob eine weitere Seite existiert (wie
	// reportquery.go/sourcestatsrepo.go).
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
		return domainstats.Page{}, fmt.Errorf("domains konnten nicht aggregiert werden: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var stats []domainstats.Stat
	for rows.Next() {
		var rawDomain string
		var total, passed int64
		var reportCount, distinctSources int
		var firstSeen, lastSeen int64
		if err := rows.Scan(&rawDomain, &total, &passed, &reportCount, &distinctSources, &firstSeen, &lastSeen); err != nil {
			return domainstats.Page{}, fmt.Errorf("domain-zeile konnte nicht gelesen werden: %w", err)
		}
		domain, err := report.NewDomainName(rawDomain)
		if err != nil {
			return domainstats.Page{}, fmt.Errorf("gespeicherte policy-domain %q ist ungültig: %w", rawDomain, err)
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
		return domainstats.Page{}, fmt.Errorf("domains konnten nicht vollständig gelesen werden: %w", err)
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

// domainsOrderAndCursor liefert die ORDER-BY-Klausel sowie — falls
// q.Cursor gesetzt ist — die WHERE-Klausel und Parameter für die zweite
// und folgende Seiten. Da domain nach der Gruppierung je Zeile eindeutig
// ist, braucht der Keyset-Vergleich (wie bei sourcesOrderAndCursor) keine
// zusätzliche ID-Spalte als Tiebreaker.
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

	// Standard: SortByVolume, größte Domain zuerst.
	orderBy = "ORDER BY total DESC, domain DESC"
	if q.Cursor != "" {
		cursorSQL = "WHERE (total, domain) < (?, ?)"
		args = []any{cur.Total, cur.Domain}
	}
	return orderBy, cursorSQL, args, nil
}

// domainCursor ist die interne, typisierte Form von
// domainstats.Query.Cursor / domainstats.Page.NextCursor.
type domainCursor struct {
	Domain string `json:"d"`
	Total  int64  `json:"t,omitempty"`
}

func (c domainCursor) encode() string {
	data, err := json.Marshal(c)
	if err != nil {
		panic(fmt.Sprintf("domainCursor konnte nicht kodiert werden: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeDomainCursor(s string) (domainCursor, error) {
	var c domainCursor
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return domainCursor{}, fmt.Errorf("cursor konnte nicht dekodiert werden: %w", err)
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return domainCursor{}, fmt.Errorf("cursor hat ein ungültiges format: %w", err)
	}
	return c, nil
}
