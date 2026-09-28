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

// defaultPageSize and maxPageSize limit Query when Limit is unset or
// implausibly large — a UI table should never be able to accidentally
// request hundreds of thousands of rows at once.
const (
	defaultPageSize = 50
	maxPageSize     = 500
)

// errGroupBySourceIPUnsupported: see the comment on GroupBySourceIP in
// internal/domain/report/repository.go.
var errGroupBySourceIPUnsupported = errors.New("groupby source_ip cannot be meaningfully represented at the report level")

// Query filters, sorts, and paginates stored reports. Per the port
// contract (see domain/report/repository.go), returns AggregateReport
// values without loaded records.
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

	// limit+1: request one extra result to detect whether another page
	// exists without a separate COUNT(*).
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
		// Overlap of the report period with the requested period.
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

// sortColumn returns the SQL column for a SortField. Unknown/empty
// values fall back to date_begin — a report without a sort specification
// should still get a stable, useful order (newest first would be the
// caller's responsibility via SortDirection).
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

// buildCursorWhere builds the keyset comparison for the second and
// subsequent pages. SQLite has supported row-value comparisons
// ("WHERE (a, b, c) > (?, ?, ?)") since 3.15 — this keeps the condition
// simple and index-usable even with a grouping column.
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

// buildCursor reads the sort/group values from the last-loaded report —
// directly from the already-typed domain values, not re-derived from
// SQL.
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
	default: // SortByDateBegin and unknown values, see sortColumn.
		cur.HasInt = true
		cur.SortInt = last.Metadata.Range.Begin.Unix()
	}

	return cur
}

// loadReports loads reports per whereClause/args/orderBy/limit. If
// withRecords is set, records and their reasons and auth results are
// additionally loaded (for FindByID) — otherwise Records stays empty
// (for Query, see the port documentation).
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
		return nil, fmt.Errorf("could not load reports: %w", err)
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
		return nil, fmt.Errorf("could not fully read reports: %w", err)
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
		return report.AggregateReport{}, fmt.Errorf("could not read report row: %w", err)
	}

	return rowToAggregateReport(row)
}

func rowToAggregateReport(row reportRow) (report.AggregateReport, error) {
	domain, err := report.NewDomainName(row.policyDomain)
	if err != nil {
		return report.AggregateReport{}, fmt.Errorf("stored policy domain %q is invalid: %w", row.policyDomain, err)
	}

	dateRange, err := report.NewDateRange(
		time.Unix(row.dateBegin, 0).UTC(),
		time.Unix(row.dateEnd, 0).UTC(),
	)
	if err != nil {
		return report.AggregateReport{}, fmt.Errorf("stored period is invalid: %w", err)
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
		return report.AggregateReport{}, fmt.Errorf("stored policy is invalid: %w", err)
	}

	importedAt, err := time.Parse(time.RFC3339Nano, row.importedAt)
	if err != nil {
		return report.AggregateReport{}, fmt.Errorf("stored imported_at is invalid: %w", err)
	}

	messageUID, err := toUint32(row.messageUID.Int64)
	if err != nil {
		return report.AggregateReport{}, fmt.Errorf("stored message_uid is invalid: %w", err)
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
		return report.AggregateReport{}, fmt.Errorf("stored report is invalid: %w", err)
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
		return nil, fmt.Errorf("could not load report_errors: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[int64][]string)
	for rows.Next() {
		var reportID int64
		var message string
		if err := rows.Scan(&reportID, &message); err != nil {
			return nil, fmt.Errorf("could not read report_errors row: %w", err)
		}
		result[reportID] = append(result[reportID], message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("could not fully read report_errors: %w", err)
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
