package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// defaultPageSize und maxPageSize begrenzen Query, wenn Limit unbesetzt
// oder unplausibel groß ist — eine UI-Tabelle soll nie versehentlich
// hunderttausende Zeilen auf einmal anfordern können.
const (
	defaultPageSize = 50
	maxPageSize     = 500
)

// errGroupBySourceIPUnsupported: siehe Kommentar bei GroupBySourceIP in
// internal/domain/report/repository.go.
var errGroupBySourceIPUnsupported = errors.New("groupby source_ip ist auf report-ebene nicht sinnvoll abbildbar")

// Query filtert, sortiert und paginiert gespeicherte Reports. Gibt gemäß
// dem Port-Vertrag (siehe domain/report/repository.go) AggregateReport-Werte
// ohne geladene Records zurück.
func (repo *ReportRepository) Query(ctx context.Context, q report.Query) (report.Page, error) {
	if q.GroupBy == report.GroupBySourceIP {
		return report.Page{}, errGroupBySourceIPUnsupported
	}

	limit := q.Limit
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}

	where, args := buildWhere(q)
	orderBy := buildOrderBy(q)

	if q.Cursor != "" {
		cursorWhere, cursorArgs, err := buildCursorWhere(q)
		if err != nil {
			return report.Page{}, err
		}
		where = append(where, cursorWhere)
		args = append(args, cursorArgs...)
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	// limit+1: ein zusätzliches Ergebnis anfordern, um ohne separates
	// COUNT(*) zu erkennen, ob eine weitere Seite existiert.
	reports, err := loadReports(ctx, repo.db, whereClause, args, orderBy, limit+1, false)
	if err != nil {
		return report.Page{}, err
	}

	page := report.Page{Reports: reports}
	if len(reports) > limit {
		page.Reports = reports[:limit]
		last := page.Reports[len(page.Reports)-1]
		page.NextCursor = buildCursor(q, last).encode()
	}

	return page, nil
}

func buildWhere(q report.Query) ([]string, []any) {
	var clauses []string
	var args []any

	if q.Period != nil {
		// Überlappung des Report-Zeitraums mit dem angefragten Zeitraum.
		clauses = append(clauses, "r.date_begin < ? AND r.date_end > ?")
		args = append(args, q.Period.End.Unix(), q.Period.Begin.Unix())
	}
	if q.Domain != "" {
		clauses = append(clauses, "r.policy_domain = ?")
		args = append(args, q.Domain)
	}
	if q.OrgName != "" {
		clauses = append(clauses, "r.org_name = ?")
		args = append(args, q.OrgName)
	}
	if q.SourceIP != "" {
		clauses = append(clauses, "EXISTS (SELECT 1 FROM records rec WHERE rec.report_id = r.id AND rec.source_ip = ?)")
		args = append(args, q.SourceIP)
	}
	if q.Disposition != "" {
		clauses = append(clauses, "EXISTS (SELECT 1 FROM records rec WHERE rec.report_id = r.id AND rec.disposition = ?)")
		args = append(args, string(q.Disposition))
	}

	return clauses, args
}

// sortColumn liefert die SQL-Spalte für ein SortField. Unbekannte/leere
// Werte fallen auf date_begin zurück — ein Report ohne Sortierangabe soll
// trotzdem eine stabile, nützliche Reihenfolge bekommen (neueste zuerst
// wäre Sache des Aufrufers über SortDirection).
func sortColumn(field report.SortField) string {
	switch field {
	case report.SortByOrgName:
		return "r.org_name"
	case report.SortByDomain:
		return "r.policy_domain"
	case report.SortByDateBegin:
		return "r.date_begin"
	default:
		return "r.date_begin"
	}
}

func groupColumn(g report.GroupBy) (string, bool) {
	switch g {
	case report.GroupByOrg:
		return "r.org_name", true
	case report.GroupByDomain:
		return "r.policy_domain", true
	default:
		return "", false
	}
}

func buildOrderBy(q report.Query) string {
	dir := "ASC"
	if q.SortDirection == report.SortDescending {
		dir = "DESC"
	}

	var parts []string
	if col, ok := groupColumn(q.GroupBy); ok {
		parts = append(parts, col+" "+dir)
	}
	parts = append(parts, sortColumn(q.SortField)+" "+dir, "r.id "+dir)

	return "ORDER BY " + strings.Join(parts, ", ")
}

// buildCursorWhere baut den Keyset-Vergleich für die zweite und folgende
// Seiten. SQLite unterstützt Row-Value-Vergleiche
// ("WHERE (a, b, c) > (?, ?, ?)") seit 3.15 — das hält die Bedingung auch
// mit Gruppierungsspalte einfach und indexnutzbar.
func buildCursorWhere(q report.Query) (string, []any, error) {
	cur, err := decodeCursor(q.Cursor)
	if err != nil {
		return "", nil, err
	}

	op := ">"
	if q.SortDirection == report.SortDescending {
		op = "<"
	}

	var cols []string
	var args []any

	if col, ok := groupColumn(q.GroupBy); ok {
		cols = append(cols, col)
		args = append(args, cur.GroupKey)
	}

	cols = append(cols, sortColumn(q.SortField))
	if cur.HasInt {
		args = append(args, cur.SortInt)
	} else {
		args = append(args, cur.SortStr)
	}

	cols = append(cols, "r.id")
	args = append(args, cur.ID)

	placeholders := strings.Repeat("?, ", len(cols))
	placeholders = strings.TrimSuffix(placeholders, ", ")

	clause := fmt.Sprintf("(%s) %s (%s)", strings.Join(cols, ", "), op, placeholders)
	return clause, args, nil
}

// buildCursor liest die Sortier-/Gruppierungswerte aus dem zuletzt
// geladenen Report — direkt aus den bereits typisierten Domänenwerten,
// nicht erneut aus SQL abgeleitet.
func buildCursor(q report.Query, last report.AggregateReport) queryCursor {
	cur := queryCursor{ID: int64(last.ID)}

	if _, ok := groupColumn(q.GroupBy); ok {
		switch q.GroupBy {
		case report.GroupByOrg:
			cur.GroupKey = last.Metadata.OrgName
		case report.GroupByDomain:
			cur.GroupKey = last.Policy.Domain.String()
		}
	}

	switch q.SortField {
	case report.SortByOrgName:
		cur.SortStr = last.Metadata.OrgName
	case report.SortByDomain:
		cur.SortStr = last.Policy.Domain.String()
	default: // SortByDateBegin und unbekannte Werte, siehe sortColumn.
		cur.HasInt = true
		cur.SortInt = last.Metadata.Range.Begin.Unix()
	}

	return cur
}

// loadReports lädt Reports gemäß whereClause/args/orderBy/limit. Ist
// withRecords gesetzt, werden zusätzlich Records, deren Reasons und
// Auth-Ergebnisse nachgeladen (für FindByID) — sonst bleibt Records leer
// (für Query, siehe Port-Dokumentation).
func loadReports(ctx context.Context, db *sql.DB, whereClause string, args []any, orderBy string, limit int, withRecords bool) ([]report.AggregateReport, error) {
	query := fmt.Sprintf(`
		SELECT
			r.id, r.org_name, r.org_email, r.org_extra_contact_info, r.report_id,
			r.date_begin, r.date_end,
			r.policy_domain, r.policy_p, r.policy_sp, r.policy_adkim, r.policy_aspf, r.policy_pct, r.policy_fo,
			r.account_id, r.mailbox, r.message_uid, r.filename, r.imported_at
		FROM reports r
		%s
		%s`, whereClause, orderBy)

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("reports konnten nicht geladen werden: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var reports []report.AggregateReport
	for rows.Next() {
		r, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		reports = append(reports, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reports konnten nicht vollständig gelesen werden: %w", err)
	}

	if withRecords {
		for i := range reports {
			records, err := loadRecords(ctx, db, int64(reports[i].ID))
			if err != nil {
				return nil, err
			}
			reports[i].Records = records
		}

		errs, err := loadReportErrors(ctx, db, reports)
		if err != nil {
			return nil, err
		}
		for i := range reports {
			reports[i].Metadata.Errors = errs[int64(reports[i].ID)]
		}
	}

	return reports, nil
}

type reportRow struct {
	id              int64
	orgName         string
	orgEmail        sql.NullString
	orgExtraContact sql.NullString
	reportID        string
	dateBegin       int64
	dateEnd         int64
	policyDomain    string
	policyP         sql.NullString
	policySP        sql.NullString
	policyADKIM     sql.NullString
	policyASPF      sql.NullString
	policyPct       sql.NullInt64
	policyFO        sql.NullString
	accountID       sql.NullString
	mailbox         sql.NullString
	messageUID      sql.NullInt64
	filename        sql.NullString
	importedAt      string
}

func scanReport(rows *sql.Rows) (report.AggregateReport, error) {
	var row reportRow
	err := rows.Scan(
		&row.id, &row.orgName, &row.orgEmail, &row.orgExtraContact, &row.reportID,
		&row.dateBegin, &row.dateEnd,
		&row.policyDomain, &row.policyP, &row.policySP, &row.policyADKIM, &row.policyASPF, &row.policyPct, &row.policyFO,
		&row.accountID, &row.mailbox, &row.messageUID, &row.filename, &row.importedAt,
	)
	if err != nil {
		return report.AggregateReport{}, fmt.Errorf("report-zeile konnte nicht gelesen werden: %w", err)
	}

	return rowToAggregateReport(row)
}

func rowToAggregateReport(row reportRow) (report.AggregateReport, error) {
	domain, err := report.NewDomainName(row.policyDomain)
	if err != nil {
		return report.AggregateReport{}, fmt.Errorf("gespeicherte policy-domain %q ist ungültig: %w", row.policyDomain, err)
	}

	dateRange, err := report.NewDateRange(
		time.Unix(row.dateBegin, 0).UTC(),
		time.Unix(row.dateEnd, 0).UTC(),
	)
	if err != nil {
		return report.AggregateReport{}, fmt.Errorf("gespeicherter zeitraum ist ungültig: %w", err)
	}

	pct := 0
	if row.policyPct.Valid {
		pct = int(row.policyPct.Int64)
	}

	policy, err := report.NewPublishedPolicy(
		domain,
		report.ParsePolicy(row.policySP.String),
		report.ParsePolicy(row.policyP.String),
		report.ParseAlignmentMode(row.policyADKIM.String),
		report.ParseAlignmentMode(row.policyASPF.String),
		pct,
		row.policyFO.String,
	)
	if err != nil {
		return report.AggregateReport{}, fmt.Errorf("gespeicherte policy ist ungültig: %w", err)
	}

	importedAt, err := time.Parse(time.RFC3339Nano, row.importedAt)
	if err != nil {
		return report.AggregateReport{}, fmt.Errorf("gespeichertes imported_at ist ungültig: %w", err)
	}

	messageUID, err := toUint32(row.messageUID.Int64)
	if err != nil {
		return report.AggregateReport{}, fmt.Errorf("gespeicherte message_uid ist ungültig: %w", err)
	}

	metadata := report.Metadata{
		OrgName:          row.orgName,
		Email:            row.orgEmail.String,
		ExtraContactInfo: row.orgExtraContact.String,
		ReportID:         row.reportID,
		Range:            dateRange,
	}

	sourceRef := report.SourceReference{
		AccountID:  row.accountID.String,
		Mailbox:    row.mailbox.String,
		MessageUID: messageUID,
		Filename:   row.filename.String,
	}

	r, err := report.NewAggregateReport(metadata, policy, nil, sourceRef, importedAt)
	if err != nil {
		return report.AggregateReport{}, fmt.Errorf("gespeicherter report ist ungültig: %w", err)
	}
	r.ID = report.ReportID(row.id)

	return *r, nil
}

func loadReportErrors(ctx context.Context, db *sql.DB, reports []report.AggregateReport) (map[int64][]string, error) {
	if len(reports) == 0 {
		return nil, nil
	}

	ids := make([]int64, len(reports))
	for i, r := range reports {
		ids[i] = int64(r.ID)
	}

	placeholders, args := inPlaceholders(ids)
	query := "SELECT report_id, message FROM report_errors WHERE report_id IN (" + placeholders + ") ORDER BY rowid"

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("report_errors konnten nicht geladen werden: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[int64][]string)
	for rows.Next() {
		var reportID int64
		var message string
		if err := rows.Scan(&reportID, &message); err != nil {
			return nil, fmt.Errorf("report_errors-zeile konnte nicht gelesen werden: %w", err)
		}
		result[reportID] = append(result[reportID], message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("report_errors konnten nicht vollständig gelesen werden: %w", err)
	}

	return result, nil
}

func inPlaceholders(ids []int64) (string, []any) {
	placeholders := strings.Repeat("?, ", len(ids))
	placeholders = strings.TrimSuffix(placeholders, ", ")

	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return placeholders, args
}
