package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// loadRecords lädt alle Records eines Reports inklusive Reasons und
// Auth-Ergebnissen. Alle drei Zusatzabfragen filtern über
// "JOIN records ON ... WHERE records.report_id = ?" statt über eine
// IN-Klausel mit einem Platzhalter je Record-ID: Bei Reports mit
// tausenden Records macht allein das Vorbereiten eines Statements mit
// entsprechend vielen Platzhaltern die Abfrage unbrauchbar langsam
// (gemessen: > 3s bei 10.000 Records statt der angestrebten < 100ms) —
// der Join nutzt stattdessen den vorhandenen Index auf records(report_id).
func loadRecords(ctx context.Context, db *sql.DB, reportID int64) ([]report.Record, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, source_ip, message_count, disposition, dkim_result, spf_result,
		       header_from, envelope_from, envelope_to
		FROM records
		WHERE report_id = ?
		ORDER BY id`, reportID)
	if err != nil {
		return nil, fmt.Errorf("records konnten nicht geladen werden: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type recordRow struct {
		id           int64
		sourceIP     string
		count        int
		disposition  string
		dkimResult   string
		spfResult    string
		headerFrom   string
		envelopeFrom sql.NullString
		envelopeTo   sql.NullString
	}

	var rawRecords []recordRow
	for rows.Next() {
		var rr recordRow
		if err := rows.Scan(&rr.id, &rr.sourceIP, &rr.count, &rr.disposition, &rr.dkimResult, &rr.spfResult,
			&rr.headerFrom, &rr.envelopeFrom, &rr.envelopeTo); err != nil {
			return nil, fmt.Errorf("record-zeile konnte nicht gelesen werden: %w", err)
		}
		rawRecords = append(rawRecords, rr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("records konnten nicht vollständig gelesen werden: %w", err)
	}
	if len(rawRecords) == 0 {
		return nil, nil
	}

	reasons, err := loadRecordReasons(ctx, db, reportID)
	if err != nil {
		return nil, err
	}
	dkimResults, err := loadDKIMResults(ctx, db, reportID)
	if err != nil {
		return nil, err
	}
	spfResults, err := loadSPFResults(ctx, db, reportID)
	if err != nil {
		return nil, err
	}

	records := make([]report.Record, 0, len(rawRecords))
	for _, rr := range rawRecords {
		sourceIP, err := report.NewSourceIP(rr.sourceIP)
		if err != nil {
			return nil, fmt.Errorf("gespeicherte quell-ip %q ist ungültig: %w", rr.sourceIP, err)
		}
		headerFrom, err := report.NewDomainName(rr.headerFrom)
		if err != nil {
			return nil, fmt.Errorf("gespeicherter header_from %q ist ungültig: %w", rr.headerFrom, err)
		}

		evaluated := report.PolicyEvaluation{
			Disposition: report.ParseDisposition(rr.disposition),
			DKIM:        report.ParseAuthResultValue(rr.dkimResult),
			SPF:         report.ParseAuthResultValue(rr.spfResult),
			Reasons:     reasons[rr.id],
		}
		identifiers := report.Identifiers{
			HeaderFrom:   headerFrom,
			EnvelopeFrom: rr.envelopeFrom.String,
			EnvelopeTo:   rr.envelopeTo.String,
		}
		auth := report.AuthResults{
			DKIM: dkimResults[rr.id],
			SPF:  spfResults[rr.id],
		}

		rec, err := report.NewRecord(sourceIP, rr.count, evaluated, identifiers, auth)
		if err != nil {
			return nil, fmt.Errorf("gespeicherter record ist ungültig: %w", err)
		}
		records = append(records, rec)
	}

	return records, nil
}

func loadRecordReasons(ctx context.Context, db *sql.DB, reportID int64) (map[int64][]report.PolicyOverrideReason, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT rr.record_id, rr.type, rr.comment
		FROM record_reasons rr
		JOIN records rec ON rec.id = rr.record_id
		WHERE rec.report_id = ?
		ORDER BY rr.rowid`, reportID)
	if err != nil {
		return nil, fmt.Errorf("record_reasons konnten nicht geladen werden: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[int64][]report.PolicyOverrideReason)
	for rows.Next() {
		var recordID int64
		var reasonType, comment sql.NullString
		if err := rows.Scan(&recordID, &reasonType, &comment); err != nil {
			return nil, fmt.Errorf("record_reasons-zeile konnte nicht gelesen werden: %w", err)
		}
		result[recordID] = append(result[recordID], report.PolicyOverrideReason{
			Type:    reasonType.String,
			Comment: comment.String,
		})
	}
	return result, rows.Err()
}

func loadDKIMResults(ctx context.Context, db *sql.DB, reportID int64) (map[int64][]report.DKIMAuthResult, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT d.record_id, d.domain, d.selector, d.result, d.human_result
		FROM auth_results_dkim d
		JOIN records rec ON rec.id = d.record_id
		WHERE rec.report_id = ?
		ORDER BY d.rowid`, reportID)
	if err != nil {
		return nil, fmt.Errorf("auth_results_dkim konnten nicht geladen werden: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[int64][]report.DKIMAuthResult)
	for rows.Next() {
		var recordID int64
		var domain, selector, res, humanResult sql.NullString
		if err := rows.Scan(&recordID, &domain, &selector, &res, &humanResult); err != nil {
			return nil, fmt.Errorf("auth_results_dkim-zeile konnte nicht gelesen werden: %w", err)
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

func loadSPFResults(ctx context.Context, db *sql.DB, reportID int64) (map[int64][]report.SPFAuthResult, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT s.record_id, s.domain, s.scope, s.result
		FROM auth_results_spf s
		JOIN records rec ON rec.id = s.record_id
		WHERE rec.report_id = ?
		ORDER BY s.rowid`, reportID)
	if err != nil {
		return nil, fmt.Errorf("auth_results_spf konnten nicht geladen werden: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[int64][]report.SPFAuthResult)
	for rows.Next() {
		var recordID int64
		var domain, scope, res sql.NullString
		if err := rows.Scan(&recordID, &domain, &scope, &res); err != nil {
			return nil, fmt.Errorf("auth_results_spf-zeile konnte nicht gelesen werden: %w", err)
		}
		result[recordID] = append(result[recordID], report.SPFAuthResult{
			Domain: domain.String,
			Scope:  scope.String,
			Result: report.ParseAuthResultValue(res.String),
		})
	}
	return result, rows.Err()
}
