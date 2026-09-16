package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	appstatistics "github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// runStats gibt die Dashboard-Kennzahlen für einen Zeitraum aus
// (UMSETZUNGSPLAN.md AP 4: "dmarc-analyzer stats",
// IMPLEMENTIERUNG.md Abschnitt 10.2).
func runStats(ctx context.Context, a *app, args []string) error {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	days := fs.Int("days", 30, "Zeitraum in Tagen, rückwirkend ab jetzt")
	domain := fs.String("domain", "", "auf eine veröffentlichte Policy-Domain filtern")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *days <= 0 {
		return fmt.Errorf("--days muss größer als 0 sein, war %d", *days)
	}

	end := time.Now().UTC()
	begin := end.Add(-time.Duration(*days) * 24 * time.Hour)
	period, err := report.NewDateRange(begin, end)
	if err != nil {
		return fmt.Errorf("zeitraum ist ungültig: %w", err)
	}

	comparison, err := a.stats.ComputeWithTrend(ctx, analysis.Query{Period: period, Domain: *domain})
	if err != nil {
		return fmt.Errorf("kennzahlen konnten nicht berechnet werden: %w", err)
	}

	printStats(comparison, *days, *domain)
	return nil
}

func printStats(cmp appstatistics.Comparison, days int, domain string) {
	fmt.Printf("Zeitraum: letzte %d Tage", days)
	if domain != "" {
		fmt.Printf(" — Domain %s", domain)
	}
	fmt.Println()

	c := cmp.Current
	fmt.Printf("Nachrichten gesamt:       %d\n", c.TotalMessages)
	fmt.Printf("DMARC-Pass-Rate:          %.1f%%\n", c.PassRate*100)
	fmt.Printf("DKIM-Alignment-Rate:      %.1f%%\n", c.DKIMAlignmentRate*100)
	fmt.Printf("SPF-Alignment-Rate:       %.1f%%\n", c.SPFAlignmentRate*100)
	fmt.Printf("Unterschiedliche Quellen: %d\n", c.DistinctSources)

	if len(c.VolumeByDisposition) > 0 {
		fmt.Println("Nach Disposition:")
		for disp, count := range c.VolumeByDisposition {
			fmt.Printf("  %-12s %d\n", disp, count)
		}
	}

	if cmp.HasPreviousPeriodData {
		trend := "→"
		switch {
		case cmp.PassRateTrend > 0.0005:
			trend = "↑"
		case cmp.PassRateTrend < -0.0005:
			trend = "↓"
		}
		fmt.Printf("Trend Pass-Rate ggü. Vorperiode: %s %+.1f Prozentpunkte\n", trend, cmp.PassRateTrend*100)
	}
}
