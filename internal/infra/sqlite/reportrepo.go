package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	sqlite3 "modernc.org/sqlite/lib"

	sqlitedriver "modernc.org/sqlite"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// ReportRepository implementiert report.Repository gegen SQLite.
type ReportRepository struct {
	db *sql.DB
}

var _ report.Repository = (*ReportRepository)(nil)

// NewReportRepository erzeugt ein einsatzbereites Repository. db muss
// bereits über Open() geöffnet (und damit migriert) sein.
func NewReportRepository(db *sql.DB) *ReportRepository {
	return &ReportRepository{db: db}
}

// ErrDuplicateReport wird von Save zurückgegeben, wenn ein Report mit
// derselben fachlichen Identität (report.Key) bereits gespeichert ist.
// Aufrufer sollten stattdessen i. d. R. vorher Exists prüfen — dieser
// Fehler ist die letzte Verteidigungslinie gegen eine Race Condition
// zwischen Exists und Save.
var ErrDuplicateReport = errors.New("report mit dieser org_name/report_id/date_begin-kombination existiert bereits")

// Save speichert einen Report vollständig oder gar nicht: Report, Records,
// Auth-Ergebnisse und Reasons laufen in einer Transaktion
// (IMPLEMENTIERUNG.md Abschnitt 7.2/8.2). Bei Erfolg setzt Save r.ID auf
// den vergebenen Primärschlüssel.
func (repo *ReportRepository) Save(ctx context.Context, r *report.AggregateReport) error {
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("transaktion konnte nicht gestartet werden: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op, wenn bereits committed

	reportID, err := insertReport(ctx, tx, r)
	if err != nil {
		if isUniqueConstraintError(err) {
			return ErrDuplicateReport
		}
		return fmt.Errorf("report konnte nicht gespeichert werden: %w", err)
	}

	if err := insertReportErrors(ctx, tx, reportID, r.Metadata.Errors); err != nil {
		return err
	}

	if err := insertRecords(ctx, tx, reportID, r.Records); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("transaktion konnte nicht committed werden: %w", err)
	}

	r.ID = report.ReportID(reportID)
	return nil
}

func insertReport(ctx context.Context, tx *sql.Tx, r *report.AggregateReport) (int64, error) {
	const stmt = `
		INSERT INTO reports (
			org_name, org_email, org_extra_contact_info, report_id,
			date_begin, date_end,
			policy_domain, policy_p, policy_sp, policy_adkim, policy_aspf, policy_pct, policy_fo,
			account_id, mailbox, message_uid, filename,
			imported_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	res, err := tx.ExecContext(ctx, stmt,
		r.Metadata.OrgName, r.Metadata.Email, r.Metadata.ExtraContactInfo, r.Metadata.ReportID,
		r.Metadata.Range.Begin.Unix(), r.Metadata.Range.End.Unix(),
		r.Policy.Domain.String(), string(r.Policy.Policy), string(r.Policy.SubdomainPolicy),
		string(r.Policy.DKIMAlignment), string(r.Policy.SPFAlignment), r.Policy.Percentage, r.Policy.FailureOptions,
		nullableString(r.SourceRef.AccountID), nullableString(r.SourceRef.Mailbox),
		nullableUint32(r.SourceRef.MessageUID), nullableString(r.SourceRef.Filename),
		r.ImportedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return 0, err
	}

	return res.LastInsertId()
}

func insertReportErrors(ctx context.Context, tx *sql.Tx, reportID int64, messages []string) error {
	if len(messages) == 0 {
		return nil
	}

	stmt, err := tx.PrepareContext(ctx, "INSERT INTO report_errors (report_id, message) VALUES (?, ?)")
	if err != nil {
		return fmt.Errorf("report_errors-statement konnte nicht vorbereitet werden: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, msg := range messages {
		if _, err := stmt.ExecContext(ctx, reportID, msg); err != nil {
			return fmt.Errorf("report_errors konnte nicht gespeichert werden: %w", err)
		}
	}
	return nil
}

// insertRecords fügt alle Records eines Reports per vorbereiteter
// Statements ein (Batch-Insert innerhalb der laufenden Transaktion,
// IMPLEMENTIERUNG.md Abschnitt 8.2).
func insertRecords(ctx context.Context, tx *sql.Tx, reportID int64, records []report.Record) error {
	if len(records) == 0 {
		return nil
	}

	recordStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO records (
			report_id, source_ip, message_count, disposition, dkim_result, spf_result,
			header_from, envelope_from, envelope_to
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("records-statement konnte nicht vorbereitet werden: %w", err)
	}
	defer func() { _ = recordStmt.Close() }()

	reasonStmt, err := tx.PrepareContext(ctx,
		"INSERT INTO record_reasons (record_id, type, comment) VALUES (?, ?, ?)")
	if err != nil {
		return fmt.Errorf("record_reasons-statement konnte nicht vorbereitet werden: %w", err)
	}
	defer func() { _ = reasonStmt.Close() }()

	dkimStmt, err := tx.PrepareContext(ctx,
		"INSERT INTO auth_results_dkim (record_id, domain, selector, result, human_result) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("auth_results_dkim-statement konnte nicht vorbereitet werden: %w", err)
	}
	defer func() { _ = dkimStmt.Close() }()

	spfStmt, err := tx.PrepareContext(ctx,
		"INSERT INTO auth_results_spf (record_id, domain, scope, result) VALUES (?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("auth_results_spf-statement konnte nicht vorbereitet werden: %w", err)
	}
	defer func() { _ = spfStmt.Close() }()

	for _, rec := range records {
		res, err := recordStmt.ExecContext(ctx,
			reportID, rec.SourceIP.String(), rec.Count,
			string(rec.Evaluated.Disposition), string(rec.Evaluated.DKIM), string(rec.Evaluated.SPF),
			rec.Identifiers.HeaderFrom.String(),
			nullableString(rec.Identifiers.EnvelopeFrom), nullableString(rec.Identifiers.EnvelopeTo),
		)
		if err != nil {
			return fmt.Errorf("record konnte nicht gespeichert werden: %w", err)
		}

		recordID, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("record-id konnte nicht ermittelt werden: %w", err)
		}

		for _, reason := range rec.Evaluated.Reasons {
			if _, err := reasonStmt.ExecContext(ctx, recordID, reason.Type, reason.Comment); err != nil {
				return fmt.Errorf("record_reason konnte nicht gespeichert werden: %w", err)
			}
		}

		for _, dkim := range rec.Auth.DKIM {
			if _, err := dkimStmt.ExecContext(ctx, recordID, dkim.Domain, dkim.Selector, string(dkim.Result), dkim.HumanResult); err != nil {
				return fmt.Errorf("auth_results_dkim konnte nicht gespeichert werden: %w", err)
			}
		}

		for _, spf := range rec.Auth.SPF {
			if _, err := spfStmt.ExecContext(ctx, recordID, spf.Domain, spf.Scope, string(spf.Result)); err != nil {
				return fmt.Errorf("auth_results_spf konnte nicht gespeichert werden: %w", err)
			}
		}
	}

	return nil
}

// Exists prüft die fachliche Identität (Abschnitt 6.3), Grundlage der
// Deduplizierung beim Import.
func (repo *ReportRepository) Exists(ctx context.Context, key report.Key) (bool, error) {
	const stmt = `SELECT 1 FROM reports WHERE org_name = ? AND report_id = ? AND date_begin = ? LIMIT 1`

	var found int
	err := repo.db.QueryRowContext(ctx, stmt, key.OrgName, key.ReportID, key.DateBegin.Unix()).Scan(&found)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("existenz konnte nicht geprüft werden: %w", err)
	default:
		return true, nil
	}
}

// FindByID lädt einen Report vollständig, inklusive aller Records.
func (repo *ReportRepository) FindByID(ctx context.Context, id report.ReportID) (*report.AggregateReport, error) {
	reports, err := loadReports(ctx, repo.db, "WHERE r.id = ?", []any{int64(id)}, "", 0, true)
	if err != nil {
		return nil, err
	}
	if len(reports) == 0 {
		return nil, fmt.Errorf("report %d: %w", id, sql.ErrNoRows)
	}
	return &reports[0], nil
}

func isUniqueConstraintError(err error) bool {
	var sqliteErr *sqlitedriver.Error
	return errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableUint32(v uint32) any {
	if v == 0 {
		return nil
	}
	return v
}

// toUint32 konvertiert einen aus SQLite gelesenen int64 (message_uid, dort
// ohne eigenes UNSIGNED-Konzept gespeichert) zurück nach uint32 — mit
// Bereichsprüfung statt stillem Abschneiden, falls die Datei jemals von
// außerhalb dieses Programms verändert wurde.
func toUint32(v int64) (uint32, error) {
	if v < 0 || v > math.MaxUint32 {
		return 0, fmt.Errorf("wert %d liegt außerhalb des uint32-bereichs", v)
	}
	return uint32(v), nil
}
