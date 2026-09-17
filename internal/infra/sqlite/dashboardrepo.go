package sqlite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// dayBucketExpr ordnet einen Report dem UTC-Tag von date_begin zu, nicht
// anteilig mehreren Tagen. DMARC-Aggregate-Reports sind nach RFC 7489 fast
// immer Ein-Tages-Zeiträume — für die seltene Ausnahme (längerer Zeitraum)
// verschiebt sich die Zuordnung auf den ersten Tag, was für die
// Zeitreihen-/Heatmap-Visualisierung eine vertretbare Vereinfachung ist.
// date_begin ist laut migrations/0001_init.sql bereits Unix-Sekunden UTC.
const dayBucketExpr = "(rep.date_begin / 86400) * 86400"

// DailyVolumes berechnet das Nachrichtenvolumen je Tag im Zeitraum von q,
// aufgeteilt nach Pass/Fail (dkim_result = 'pass' OR spf_result = 'pass').
func (r *StatisticsRepository) DailyVolumes(ctx context.Context, q analysis.Query) ([]analysis.DailyVolume, error) {
	where, args := statsWhere(q)

	query := fmt.Sprintf(`
		SELECT %s AS day_bucket,
			COALESCE(SUM(CASE WHEN rec.dkim_result = 'pass' OR rec.spf_result = 'pass' THEN rec.message_count ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN rec.dkim_result != 'pass' AND rec.spf_result != 'pass' THEN rec.message_count ELSE 0 END), 0)
		FROM records rec
		JOIN reports rep ON rep.id = rec.report_id
		%s
		GROUP BY day_bucket
		ORDER BY day_bucket ASC`, dayBucketExpr, where)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("tägliches volumen konnte nicht berechnet werden: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []analysis.DailyVolume
	for rows.Next() {
		var dayUnix int64
		var pass, fail int64
		if err := rows.Scan(&dayUnix, &pass, &fail); err != nil {
			return nil, fmt.Errorf("tageszeile konnte nicht gelesen werden: %w", err)
		}
		result = append(result, analysis.DailyVolume{
			Day:  time.Unix(dayUnix, 0).UTC(),
			Pass: int(pass),
			Fail: int(fail),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("tägliches volumen konnte nicht vollständig gelesen werden: %w", err)
	}
	return result, nil
}

// TopSources liefert die nach Volumen absteigend sortierten Sendequellen
// im Zeitraum von q, begrenzt auf limit Einträge.
func (r *StatisticsRepository) TopSources(ctx context.Context, q analysis.Query, limit int) ([]analysis.SourceVolume, error) {
	where, args := statsWhere(q)
	args = append(args, limit)

	query := fmt.Sprintf(`
		SELECT rec.source_ip,
			SUM(rec.message_count),
			COALESCE(SUM(CASE WHEN rec.dkim_result = 'pass' OR rec.spf_result = 'pass' THEN rec.message_count ELSE 0 END), 0)
		FROM records rec
		JOIN reports rep ON rep.id = rec.report_id
		%s
		GROUP BY rec.source_ip
		ORDER BY SUM(rec.message_count) DESC
		LIMIT ?`, where)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("top-sendequellen konnten nicht berechnet werden: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []analysis.SourceVolume
	for rows.Next() {
		var rawIP string
		var total, passed int64
		if err := rows.Scan(&rawIP, &total, &passed); err != nil {
			return nil, fmt.Errorf("sendequellen-zeile konnte nicht gelesen werden: %w", err)
		}
		ip, err := report.NewSourceIP(rawIP)
		if err != nil {
			return nil, fmt.Errorf("gespeicherte quell-ip %q ist ungültig: %w", rawIP, err)
		}
		result = append(result, analysis.SourceVolume{
			SourceIP: ip,
			Total:    int(total),
			PassRate: rate(passed, total),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("top-sendequellen konnten nicht vollständig gelesen werden: %w", err)
	}
	return result, nil
}

// Heatmap berechnet die Quelle-×-Tag-Pass-Rate-Matrix für die
// sourceLimit volumenstärksten Quellen im Zeitraum von q. Tage ohne
// Nachrichten einer Quelle werden als HasData=false markiert, statt eine
// irreführende Pass-Rate von 0 zu melden.
func (r *StatisticsRepository) Heatmap(ctx context.Context, q analysis.Query, sourceLimit int) (analysis.Heatmap, error) {
	topSources, err := r.TopSources(ctx, q, sourceLimit)
	if err != nil {
		return analysis.Heatmap{}, err
	}
	if len(topSources) == 0 {
		return analysis.Heatmap{}, nil
	}

	sourceIndex := make(map[string]int, len(topSources))
	heatmap := analysis.Heatmap{Sources: make([]report.SourceIP, len(topSources))}
	for i, s := range topSources {
		heatmap.Sources[i] = s.SourceIP
		sourceIndex[s.SourceIP.String()] = i
	}

	heatmap.Days = daysInRange(q.Period)
	dayIndex := make(map[int64]int, len(heatmap.Days))
	heatmap.Cells = make([][]analysis.HeatmapCell, len(topSources))
	for i := range heatmap.Cells {
		heatmap.Cells[i] = make([]analysis.HeatmapCell, len(heatmap.Days))
	}
	for j, d := range heatmap.Days {
		dayIndex[d.Unix()] = j
	}

	cells, err := r.heatmapCells(ctx, q, topSources)
	if err != nil {
		return analysis.Heatmap{}, err
	}
	for _, c := range cells {
		si, ok := sourceIndex[c.sourceIP]
		if !ok {
			continue
		}
		di, ok := dayIndex[c.dayUnix]
		if !ok {
			continue
		}
		heatmap.Cells[si][di] = analysis.HeatmapCell{PassRate: rate(c.passed, c.total), HasData: true, Total: int(c.total)}
	}

	return heatmap, nil
}

type heatmapRow struct {
	sourceIP string
	dayUnix  int64
	total    int64
	passed   int64
}

func (r *StatisticsRepository) heatmapCells(ctx context.Context, q analysis.Query, topSources []analysis.SourceVolume) ([]heatmapRow, error) {
	where, args := statsWhere(q)

	placeholders := make([]string, len(topSources))
	for i, s := range topSources {
		placeholders[i] = "?"
		args = append(args, s.SourceIP.String())
	}

	query := fmt.Sprintf(`
		SELECT rec.source_ip, %s AS day_bucket,
			SUM(rec.message_count),
			COALESCE(SUM(CASE WHEN rec.dkim_result = 'pass' OR rec.spf_result = 'pass' THEN rec.message_count ELSE 0 END), 0)
		FROM records rec
		JOIN reports rep ON rep.id = rec.report_id
		%s AND rec.source_ip IN (%s)
		GROUP BY rec.source_ip, day_bucket`,
		dayBucketExpr, where, strings.Join(placeholders, ", "))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("heatmap konnte nicht berechnet werden: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []heatmapRow
	for rows.Next() {
		var row heatmapRow
		if err := rows.Scan(&row.sourceIP, &row.dayUnix, &row.total, &row.passed); err != nil {
			return nil, fmt.Errorf("heatmap-zeile konnte nicht gelesen werden: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("heatmap konnte nicht vollständig gelesen werden: %w", err)
	}
	return result, nil
}

// daysInRange listet jeden UTC-Tagesbeginn von period.Begin (abgerundet)
// bis period.End (exklusiv) — die vollständige Tagesachse der Heatmap,
// unabhängig davon, ob an einem Tag Daten vorliegen.
func daysInRange(period report.DateRange) []time.Time {
	start := period.Begin.UTC().Truncate(24 * time.Hour)
	end := period.End.UTC()

	var days []time.Time
	for d := start; d.Before(end); d = d.Add(24 * time.Hour) {
		days = append(days, d)
	}
	return days
}
