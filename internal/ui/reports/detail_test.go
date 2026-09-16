package reports

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/uitest"
)

func fullTestReport(t *testing.T) *report.AggregateReport {
	t.Helper()
	domain, err := report.NewDomainName("example.com")
	require.NoError(t, err)
	dr, err := report.NewDateRange(time.Now().Add(-time.Hour), time.Now())
	require.NoError(t, err)
	policy, err := report.NewPublishedPolicy(domain, report.PolicyReject, report.PolicyNone, report.AlignmentStrict, report.AlignmentRelaxed, 100, "")
	require.NoError(t, err)

	sourceIP, err := report.NewSourceIP("203.0.113.5")
	require.NoError(t, err)
	headerFrom, err := report.NewDomainName("example.com")
	require.NoError(t, err)
	rec, err := report.NewRecord(sourceIP, 7,
		report.PolicyEvaluation{Disposition: report.DispositionReject, DKIM: report.AuthResultFail, SPF: report.AuthResultPass},
		report.Identifiers{HeaderFrom: headerFrom}, report.AuthResults{},
	)
	require.NoError(t, err)

	r, err := report.NewAggregateReport(
		report.Metadata{OrgName: "google.com", ReportID: "report-1", Range: dr},
		policy, []report.Record{rec}, report.SourceReference{}, time.Now(),
	)
	require.NoError(t, err)
	return r
}

func TestNewDetailView_RendersOrgAndDomain(t *testing.T) {
	obj := NewDetailView(fullTestReport(t))
	require.NotNil(t, uitest.FindLabel(obj, "google.com"))
	require.NotNil(t, uitest.FindLabel(obj, "example.com"))
}

func TestNewDetailView_ZeroRecords_ShowsPlaceholder(t *testing.T) {
	r := fullTestReport(t)
	r.Records = nil
	obj := NewDetailView(r)
	require.NotNil(t, uitest.FindLabel(obj, "Keine Sendequellen in diesem Bericht."))
}
