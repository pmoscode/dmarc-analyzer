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

// runStats prints the dashboard metrics for a time period
// (UMSETZUNGSPLAN.md AP 4: "dmarc-analyzer stats",
// IMPLEMENTIERUNG.md section 10.2).
func runStats(ctx context.Context, a *app, args []string) error {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	days := fs.Int("days", 30, "period in days, counting back from now")
	domain := fs.String("domain", "", "filter by a published policy domain")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *days <= 0 {
		return fmt.Errorf("--days must be greater than 0, was %d", *days)
	}

	end := time.Now().UTC()
	begin := end.Add(-time.Duration(*days) * 24 * time.Hour)
	period, err := report.NewDateRange(begin, end)
	if err != nil {
		return fmt.Errorf("period is invalid: %w", err)
	}

	comparison, err := a.stats.ComputeWithTrend(ctx, analysis.Query{Period: period, Domain: *domain})
	if err != nil {
		return fmt.Errorf("metrics could not be computed: %w", err)
	}

	printStats(comparison, *days, *domain)
	return nil
}

func printStats(cmp appstatistics.Comparison, days int, domain string) {
	fmt.Printf("Period: last %d days", days)
	if domain != "" {
		fmt.Printf(" — domain %s", domain)
	}
	fmt.Println()

	c := cmp.Current
	fmt.Printf("Total messages:      %d\n", c.TotalMessages)
	fmt.Printf("DMARC pass rate:     %.1f%%\n", c.PassRate*100)
	fmt.Printf("DKIM alignment rate: %.1f%%\n", c.DKIMAlignmentRate*100)
	fmt.Printf("SPF alignment rate:  %.1f%%\n", c.SPFAlignmentRate*100)
	fmt.Printf("Distinct sources:    %d\n", c.DistinctSources)

	if len(c.VolumeByDisposition) > 0 {
		fmt.Println("By disposition:")
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
		fmt.Printf("Pass rate trend vs. previous period: %s %+.1f percentage points\n", trend, cmp.PassRateTrend*100)
	}
}
