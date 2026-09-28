package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// StatisticsRepository implements analysis.Repository against SQLite:
// the metrics are computed via SQL aggregation, not by loading individual
// records into Go (see UMSETZUNGSPLAN.md AP 2, "aggregations for the
// metrics ... deliberately not in AP 2").
type StatisticsRepository struct {
	db *sql.DB
}

var _ analysis.Repository = (*StatisticsRepository)(nil)

// NewStatisticsRepository creates a ready-to-use repository.
func NewStatisticsRepository(db *sql.DB) *StatisticsRepository {
	return &StatisticsRepository{db: db}
}

// Compute calculates the Statistics for q via SQL aggregation.
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
		return analysis.Statistics{}, fmt.Errorf("could not calculate metrics: %w", err)
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
		return nil, fmt.Errorf("could not calculate distribution by disposition: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[report.Disposition]int)
	for rows.Next() {
		var raw string
		var count int64
		if err := rows.Scan(&raw, &count); err != nil {
			return nil, fmt.Errorf("could not read disposition row: %w", err)
		}
		result[report.ParseDisposition(raw)] += int(count)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("could not fully read distribution by disposition: %w", err)
	}
	return result, nil
}

// rate returns part/total, or 0 if total is 0 (no division by zero, no
// NaN on the dashboard).
func rate(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total)
}
