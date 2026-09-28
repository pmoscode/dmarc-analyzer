package sqlite

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/failedrecords"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

const (
	defaultFailedRecordsPageSize = 50
	maxFailedRecordsPageSize     = 500
)

// FailedRecordsRepository implements failedrecords.Repository against
// SQLite: cross-report search for records where DMARC was not passed
// (neither dkim_result nor spf_result is "pass").
type FailedRecordsRepository struct {
	db *sql.DB
}

var _ failedrecords.Repository = (*FailedRecordsRepository)(nil)

// NewFailedRecordsRepository creates a ready-to-use repository.
func NewFailedRecordsRepository(db *sql.DB) *FailedRecordsRepository {
	return &FailedRecordsRepository{db: db}
}

// Query returns a page of failed records for q.
func (r *FailedRecordsRepository) Query(ctx context.Context, q failedrecords.Query) (failedrecords.Page, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = defaultFailedRecordsPageSize
	}
	if limit > maxFailedRecordsPageSize {
		limit = maxFailedRecordsPageSize
	}

	whereClauses, args := failedRecordsWhere(q)
	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "AND " + strings.Join(whereClauses, " AND ")
	}

	orderBy, cursorSQL, cursorArgs, err := failedRecordsOrderAndCursor(q)
	if err != nil {
		return failedrecords.Page{}, err
	}
	args = append(args, cursorArgs...)

	// limit+1: like reportquery.go/domainstatsrepo.go — request one extra
	// result to detect whether another page exists without a separate
	// COUNT(*).
	args = append(args, limit+1)

	query := fmt.Sprintf(`
		SELECT rec.id, rec.source_ip, rec.message_count, rec.disposition, rec.dkim_result, rec.spf_result,
		       rec.header_from, rec.envelope_from, rec.envelope_to,
		       rep.id, rep.org_name, rep.policy_domain, rep.date_begin, rep.date_end
		FROM records rec
		JOIN reports rep ON rep.id = rec.report_id
		WHERE rec.dkim_result != 'pass' AND rec.spf_result != 'pass'
		%s
		%s
		%s
		LIMIT ?`, whereSQL, cursorSQL, orderBy)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return failedrecords.Page{}, fmt.Errorf("could not query failed records: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type rawRow struct {
		recordID     int64
		sourceIP     string
		count        int
		disposition  string
		dkimResult   string
		spfResult    string
		headerFrom   string
		envelopeFrom sql.NullString
		envelopeTo   sql.NullString
		reportID     int64
		orgName      string
		policyDomain string
		dateBegin    int64
		dateEnd      int64
	}

	var rawRows []rawRow
	for rows.Next() {
		var rr rawRow
		if err := rows.Scan(&rr.recordID, &rr.sourceIP, &rr.count, &rr.disposition, &rr.dkimResult, &rr.spfResult,
			&rr.headerFrom, &rr.envelopeFrom, &rr.envelopeTo,
			&rr.reportID, &rr.orgName, &rr.policyDomain, &rr.dateBegin, &rr.dateEnd); err != nil {
			return failedrecords.Page{}, fmt.Errorf("could not read failed-record row: %w", err)
		}
		rawRows = append(rawRows, rr)
	}
	if err := rows.Err(); err != nil {
		return failedrecords.Page{}, fmt.Errorf("could not fully read failed records: %w", err)
	}

	hasMore := len(rawRows) > limit
	if hasMore {
		rawRows = rawRows[:limit]
	}

	// Load the raw data (DKIM/SPF check results, override reasons) for
	// exactly this page — an IN clause over a small, known set of record
	// IDs (≤ page size) is fine, see AGENTS.md ("if the filter value is
	// already known, join instead"/loadReportErrors in reportquery.go);
	// only an unbounded list would be a problem.
	recordIDs := make([]int64, len(rawRows))
	for i, rr := range rawRows {
		recordIDs[i] = rr.recordID
	}
	reasons, err := loadReasonsByRecordIDs(ctx, r.db, recordIDs)
	if err != nil {
		return failedrecords.Page{}, err
	}
	dkimResults, err := loadDKIMResultsByRecordIDs(ctx, r.db, recordIDs)
	if err != nil {
		return failedrecords.Page{}, err
	}
	spfResults, err := loadSPFResultsByRecordIDs(ctx, r.db, recordIDs)
	if err != nil {
		return failedrecords.Page{}, err
	}

	records := make([]failedrecords.Record, len(rawRows))
	for i, rr := range rawRows {
		sourceIP, err := report.NewSourceIP(rr.sourceIP)
		if err != nil {
			return failedrecords.Page{}, fmt.Errorf("stored source IP %q is invalid: %w", rr.sourceIP, err)
		}
		headerFrom, err := report.NewDomainName(rr.headerFrom)
		if err != nil {
			return failedrecords.Page{}, fmt.Errorf("stored header_from %q is invalid: %w", rr.headerFrom, err)
		}
		policyDomain, err := report.NewDomainName(rr.policyDomain)
		if err != nil {
			return failedrecords.Page{}, fmt.Errorf("stored policy domain %q is invalid: %w", rr.policyDomain, err)
		}

		records[i] = failedrecords.Record{
			ReportID:     report.ReportID(rr.reportID),
			OrgName:      rr.orgName,
			PolicyDomain: policyDomain,
			PeriodBegin:  time.Unix(rr.dateBegin, 0).UTC(),
			PeriodEnd:    time.Unix(rr.dateEnd, 0).UTC(),
			SourceIP:     sourceIP,
			Count:        rr.count,
			Disposition:  report.ParseDisposition(rr.disposition),
			DKIM:         report.ParseAuthResultValue(rr.dkimResult),
			SPF:          report.ParseAuthResultValue(rr.spfResult),
			Identifiers: report.Identifiers{
				HeaderFrom:   headerFrom,
				EnvelopeFrom: rr.envelopeFrom.String,
				EnvelopeTo:   rr.envelopeTo.String,
			},
			Auth: report.AuthResults{
				DKIM: dkimResults[rr.recordID],
				SPF:  spfResults[rr.recordID],
			},
			Reasons: reasons[rr.recordID],
		}
	}

	page := failedrecords.Page{Records: records}
	if hasMore {
		last := rawRows[len(rawRows)-1]
		page.NextCursor = buildFailedRecordCursor(q.SortField, last.dateBegin, last.sourceIP, last.recordID).encode()
	}
	return page, nil
}

func failedRecordsWhere(q failedrecords.Query) ([]string, []any) {
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
	if q.SourceIP != "" {
		clauses = append(clauses, "rec.source_ip = ?")
		args = append(args, q.SourceIP)
	}
	return clauses, args
}

// failedRecordsOrderAndCursor returns the ORDER BY clause and — if
// q.Cursor is set — the WHERE clause and parameters for the second and
// subsequent pages. rec.id is the tiebreaker in both orderings (unique,
// needed for stable keyset pagination).
func failedRecordsOrderAndCursor(q failedrecords.Query) (orderBy, cursorSQL string, args []any, err error) {
	var cur failedRecordCursor
	if q.Cursor != "" {
		cur, err = decodeFailedRecordCursor(q.Cursor)
		if err != nil {
			return "", "", nil, err
		}
	}

	if q.SortField == failedrecords.SortBySourceIP {
		orderBy = "ORDER BY rec.source_ip ASC, rec.id ASC"
		if q.Cursor != "" {
			cursorSQL = "AND (rec.source_ip, rec.id) > (?, ?)"
			args = []any{cur.SourceIP, cur.RecordID}
		}
		return orderBy, cursorSQL, args, nil
	}

	// Default: SortByDate, newest report first.
	orderBy = "ORDER BY rep.date_begin DESC, rec.id DESC"
	if q.Cursor != "" {
		cursorSQL = "AND (rep.date_begin, rec.id) < (?, ?)"
		args = []any{cur.DateBegin, cur.RecordID}
	}
	return orderBy, cursorSQL, args, nil
}

func loadReasonsByRecordIDs(ctx context.Context, db *sql.DB, recordIDs []int64) (map[int64][]report.PolicyOverrideReason, error) {
	if len(recordIDs) == 0 {
		return nil, nil
	}
	placeholders, args := inPlaceholders(recordIDs)
	rows, err := db.QueryContext(ctx, `
		SELECT record_id, type, comment
		FROM record_reasons
		WHERE record_id IN (`+placeholders+`)
		ORDER BY rowid`, args...)
	if err != nil {
		return nil, fmt.Errorf("could not load record_reasons: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[int64][]report.PolicyOverrideReason)
	for rows.Next() {
		var recordID int64
		var reasonType, comment sql.NullString
		if err := rows.Scan(&recordID, &reasonType, &comment); err != nil {
			return nil, fmt.Errorf("could not read record_reasons row: %w", err)
		}
		result[recordID] = append(result[recordID], report.PolicyOverrideReason{
			Type:    reasonType.String,
			Comment: comment.String,
		})
	}
	return result, rows.Err()
}

func loadDKIMResultsByRecordIDs(ctx context.Context, db *sql.DB, recordIDs []int64) (map[int64][]report.DKIMAuthResult, error) {
	if len(recordIDs) == 0 {
		return nil, nil
	}
	placeholders, args := inPlaceholders(recordIDs)
	rows, err := db.QueryContext(ctx, `
		SELECT record_id, domain, selector, result, human_result
		FROM auth_results_dkim
		WHERE record_id IN (`+placeholders+`)
		ORDER BY rowid`, args...)
	if err != nil {
		return nil, fmt.Errorf("could not load auth_results_dkim: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[int64][]report.DKIMAuthResult)
	for rows.Next() {
		var recordID int64
		var domain, selector, res, humanResult sql.NullString
		if err := rows.Scan(&recordID, &domain, &selector, &res, &humanResult); err != nil {
			return nil, fmt.Errorf("could not read auth_results_dkim row: %w", err)
		}
		result[recordID] = append(result[recordID], report.DKIMAuthResult{
			Domain:      domain.String,
			Selector:    selector.String,
			Result:      report.ParseAuthResultValue(res.String),
			HumanResult: humanResult.String,
		})
	}
	return result, rows.Err()
}

func loadSPFResultsByRecordIDs(ctx context.Context, db *sql.DB, recordIDs []int64) (map[int64][]report.SPFAuthResult, error) {
	if len(recordIDs) == 0 {
		return nil, nil
	}
	placeholders, args := inPlaceholders(recordIDs)
	rows, err := db.QueryContext(ctx, `
		SELECT record_id, domain, scope, result
		FROM auth_results_spf
		WHERE record_id IN (`+placeholders+`)
		ORDER BY rowid`, args...)
	if err != nil {
		return nil, fmt.Errorf("could not load auth_results_spf: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[int64][]report.SPFAuthResult)
	for rows.Next() {
		var recordID int64
		var domain, scope, res sql.NullString
		if err := rows.Scan(&recordID, &domain, &scope, &res); err != nil {
			return nil, fmt.Errorf("could not read auth_results_spf row: %w", err)
		}
		result[recordID] = append(result[recordID], report.SPFAuthResult{
			Domain: domain.String,
			Scope:  scope.String,
			Result: report.ParseAuthResultValue(res.String),
		})
	}
	return result, rows.Err()
}

// failedRecordCursor is the internal, typed form of
// failedrecords.Query.Cursor/Page.NextCursor. Only the fields belonging to
// the active sort field are set (see failedRecordsOrderAndCursor).
type failedRecordCursor struct {
	DateBegin int64  `json:"d,omitempty"`
	SourceIP  string `json:"s,omitempty"`
	RecordID  int64  `json:"r"`
}

func buildFailedRecordCursor(sortField failedrecords.SortField, dateBegin int64, sourceIP string, recordID int64) failedRecordCursor {
	if sortField == failedrecords.SortBySourceIP {
		return failedRecordCursor{SourceIP: sourceIP, RecordID: recordID}
	}
	return failedRecordCursor{DateBegin: dateBegin, RecordID: recordID}
}

func (c failedRecordCursor) encode() string {
	data, err := json.Marshal(c)
	if err != nil {
		panic(fmt.Sprintf("failedRecordCursor could not be encoded: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeFailedRecordCursor(s string) (failedRecordCursor, error) {
	var c failedRecordCursor
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return failedRecordCursor{}, fmt.Errorf("could not decode cursor: %w", err)
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return failedRecordCursor{}, fmt.Errorf("cursor has an invalid format: %w", err)
	}
	return c, nil
}
