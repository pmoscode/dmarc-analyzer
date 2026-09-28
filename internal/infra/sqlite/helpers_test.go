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

// newTestDB opens a file-backed DB in a temporary directory — deliberately
// not an ":memory:" DB, so WAL and transactions are tested realistically
// (IMPLEMENTIERUNG.md section 12.2). Closed automatically at the end of the
// test.
func newTestDB(t testing.TB) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	db, err := sqlite.Open(context.Background(), path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	return db
}

// reportOpts controls newTestReport where a test needs to deviate from the
// default case.
type reportOpts struct {
	orgName     string
	domain      string
	reportID    string
	begin       time.Time
	end         time.Time
	sourceIP    string
	disposition report.Disposition
	// dkim/spf override the otherwise always-passing (aligned) auth
	// results of the generated record — for tests that specifically need
	// failed records (e.g. failedrecordsrepo_test.go). Empty means:
	// AuthResultPass as before.
	dkim report.AuthResultValue
	spf  report.AuthResultValue
}

// newTestReport builds a domain-valid AggregateReport with exactly one
// complete record (including a reason and one DKIM/SPF result each) — for
// tests that don't want to use the XML parser from internal/infra/dmarcxml,
// to stay independent of its details.
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
	if opts.dkim == "" {
		opts.dkim = report.AuthResultPass
	}
	if opts.spf == "" {
		opts.spf = report.AuthResultPass
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
			DKIM:        opts.dkim,
			SPF:         opts.spf,
			Reasons: []report.PolicyOverrideReason{
				{Type: "local_policy", Comment: "test-comment"},
			},
		},
		report.Identifiers{HeaderFrom: headerFrom, EnvelopeFrom: "bounce." + opts.domain},
		report.AuthResults{
			DKIM: []report.DKIMAuthResult{{Domain: opts.domain, Selector: "sel1", Result: opts.dkim}},
			SPF:  []report.SPFAuthResult{{Domain: opts.domain, Result: opts.spf}},
		},
	)
	require.NoError(t, err)

	r, err := report.NewAggregateReport(metadata, policy, []report.Record{rec}, report.SourceReference{}, time.Now())
	require.NoError(t, err)

	return r
}
