package sqlite_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/sqlite"
)

// newTestDB öffnet eine Datei-DB in einem temporären Verzeichnis — bewusst
// keine ":memory:"-DB, damit WAL und Transaktionen realistisch getestet
// werden (IMPLEMENTIERUNG.md Abschnitt 12.2). Wird am Ende des Tests
// automatisch geschlossen.
func newTestDB(t testing.TB) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	db, err := sqlite.Open(context.Background(), path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	return db
}

// reportOpts steuert newTestReport, wo vom Standardfall abgewichen werden soll.
type reportOpts struct {
	orgName     string
	domain      string
	reportID    string
	begin       time.Time
	end         time.Time
	sourceIP    string
	disposition report.Disposition
}

// newTestReport baut einen fachlich gültigen AggregateReport mit genau
// einem vollständigen Record (inkl. Reason und je einem DKIM-/SPF-Ergebnis)
// — für Tests, die nicht den XML-Parser aus internal/infra/dmarcxml
// benutzen wollen, um von dessen Details unabhängig zu bleiben.
func newTestReport(t testing.TB, opts reportOpts) *report.AggregateReport {
	t.Helper()

	if opts.orgName == "" {
		opts.orgName = "test-org.example"
	}
	if opts.domain == "" {
		opts.domain = "example.com"
	}
	if opts.reportID == "" {
		opts.reportID = fmt.Sprintf("test-report-%d", time.Now().UnixNano())
	}
	if opts.begin.IsZero() {
		opts.begin = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	}
	if opts.end.IsZero() {
		opts.end = opts.begin.Add(24 * time.Hour)
	}
	if opts.sourceIP == "" {
		opts.sourceIP = "203.0.113.5"
	}
	if opts.disposition == "" {
		opts.disposition = report.DispositionNone
	}

	domain, err := report.NewDomainName(opts.domain)
	require.NoError(t, err)
	dateRange, err := report.NewDateRange(opts.begin, opts.end)
	require.NoError(t, err)

	metadata := report.Metadata{
		OrgName:  opts.orgName,
		Email:    "dmarc@" + opts.orgName,
		ReportID: opts.reportID,
		Range:    dateRange,
	}

	policy, err := report.NewPublishedPolicy(
		domain, report.PolicyReject, report.PolicyReject,
		report.AlignmentRelaxed, report.AlignmentRelaxed, 100, "",
	)
	require.NoError(t, err)

	sourceIP, err := report.NewSourceIP(opts.sourceIP)
	require.NoError(t, err)
	headerFrom, err := report.NewDomainName(opts.domain)
	require.NoError(t, err)

	rec, err := report.NewRecord(
		sourceIP, 3,
		report.PolicyEvaluation{
			Disposition: opts.disposition,
			DKIM:        report.AuthResultPass,
			SPF:         report.AuthResultPass,
			Reasons: []report.PolicyOverrideReason{
				{Type: "local_policy", Comment: "test-kommentar"},
			},
		},
		report.Identifiers{HeaderFrom: headerFrom, EnvelopeFrom: "bounce." + opts.domain},
		report.AuthResults{
			DKIM: []report.DKIMAuthResult{{Domain: opts.domain, Selector: "sel1", Result: report.AuthResultPass}},
			SPF:  []report.SPFAuthResult{{Domain: opts.domain, Result: report.AuthResultPass}},
		},
	)
	require.NoError(t, err)

	r, err := report.NewAggregateReport(metadata, policy, []report.Record{rec}, report.SourceReference{}, time.Now())
	require.NoError(t, err)

	return r
}
