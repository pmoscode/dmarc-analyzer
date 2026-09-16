package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// StatisticsRepository implementiert analysis.Repository gegen SQLite: die
// Kennzahlen werden per SQL-Aggregation berechnet, nicht durch Laden
// einzelner Records nach Go (siehe UMSETZUNGSPLAN.md AP 2, "Aggregationen
// für die Kennzahlen ... bewusst nicht in AP 2").
type StatisticsRepository struct {
	db *sql.DB
}

var _ analysis.Repository = (*StatisticsRepository)(nil)

// NewStatisticsRepository erzeugt ein einsatzbereites Repository.
func NewStatisticsRepository(db *sql.DB) *StatisticsRepository {
	return &StatisticsRepository{db: db}
}

// Compute berechnet die Statistics für q per SQL-Aggregation.
func (r *StatisticsRepository) Compute(ctx context.Context, q analysis.Query) (analysis.Statistics, error) {
	where, args := statsWhere(q)

	totals, err := r.computeTotals(ctx, where, args)
	if err != nil {
		return analysis.Statistics{}, err
	}

	byDisposition, err := r.computeVolumeByDisposition(ctx, where, args)
	if err != nil {
		return analysis.Statistics{}, err
	}
	totals.VolumeByDisposition = byDisposition

	return totals, nil
}

func statsWhere(q analysis.Query) (string, []any) {
	clause := "WHERE rep.date_begin < ? AND rep.date_end > ?"
	args := []any{q.Period.End.Unix(), q.Period.Begin.Unix()}

	if q.Domain != "" {
		clause += " AND rep.policy_domain = ?"
		args = append(args, q.Domain)
	}
	return clause, args
}

func (r *StatisticsRepository) computeTotals(ctx context.Context, where string, args []any) (analysis.Statistics, error) {
	query := fmt.Sprintf(`
		SELECT
			COALESCE(SUM(rec.message_count), 0),
			COALESCE(SUM(CASE WHEN rec.dkim_result = 'pass' OR rec.spf_result = 'pass' THEN rec.message_count ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN rec.dkim_result = 'pass' THEN rec.message_count ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN rec.spf_result = 'pass' THEN rec.message_count ELSE 0 END), 0),
			COUNT(DISTINCT rec.source_ip)
		FROM records rec
		JOIN reports rep ON rep.id = rec.report_id
		%s`, where)

	var total, passTotal, dkimPassTotal, spfPassTotal, distinctSources int64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&total, &passTotal, &dkimPassTotal, &spfPassTotal, &distinctSources)
	if err != nil {
		return analysis.Statistics{}, fmt.Errorf("kennzahlen konnten nicht berechnet werden: %w", err)
	}

	return analysis.Statistics{
		TotalMessages:     int(total),
		PassRate:          rate(passTotal, total),
		DKIMAlignmentRate: rate(dkimPassTotal, total),
		SPFAlignmentRate:  rate(spfPassTotal, total),
		DistinctSources:   int(distinctSources),
	}, nil
}

func (r *StatisticsRepository) computeVolumeByDisposition(ctx context.Context, where string, args []any) (map[report.Disposition]int, error) {
	query := fmt.Sprintf(`
		SELECT rec.disposition, COALESCE(SUM(rec.message_count), 0)
		FROM records rec
		JOIN reports rep ON rep.id = rec.report_id
		%s
		GROUP BY rec.disposition`, where)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("verteilung nach disposition konnte nicht berechnet werden: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[report.Disposition]int)
	for rows.Next() {
		var raw string
		var count int64
		if err := rows.Scan(&raw, &count); err != nil {
			return nil, fmt.Errorf("disposition-zeile konnte nicht gelesen werden: %w", err)
		}
		result[report.ParseDisposition(raw)] += int(count)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("verteilung nach disposition konnte nicht vollständig gelesen werden: %w", err)
	}
	return result, nil
}

// rate liefert part/total, oder 0 wenn total 0 ist (keine Division durch
// Null, kein NaN im Dashboard).
func rate(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total)
}
