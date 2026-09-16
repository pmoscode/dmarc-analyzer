package exportdata_test

import (
	"bytes"
	"encoding/csv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/exportdata"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

func testReport(t *testing.T, reportID string) report.AggregateReport {
	t.Helper()

	domain, err := report.NewDomainName("example.com")
	require.NoError(t, err)
	dateRange, err := report.NewDateRange(
		time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)
	policy, err := report.NewPublishedPolicy(domain, report.PolicyQuarantine, report.PolicyReject, report.AlignmentStrict, report.AlignmentRelaxed, 100, "")
	require.NoError(t, err)

	sourceIP, err := report.NewSourceIP("203.0.113.5")
	require.NoError(t, err)
	headerFrom, err := report.NewDomainName("example.com")
	require.NoError(t, err)
	rec, err := report.NewRecord(
		sourceIP, 42,
		report.PolicyEvaluation{Disposition: report.DispositionNone, DKIM: report.AuthResultPass, SPF: report.AuthResultFail},
		report.Identifiers{HeaderFrom: headerFrom, EnvelopeFrom: "bounce.example.com"},
		report.AuthResults{},
	)
	require.NoError(t, err)

	r, err := report.NewAggregateReport(
		report.Metadata{OrgName: "google.com", ReportID: reportID, Range: dateRange},
		policy, []report.Record{rec}, report.SourceReference{}, time.Now(),
	)
	require.NoError(t, err)
	return *r
}

func TestWriteReportsCSV_HeaderAndRows(t *testing.T) {
	t.Parallel()

	reports := []report.AggregateReport{testReport(t, "report-1"), testReport(t, "report-2")}

	var buf bytes.Buffer
	require.NoError(t, exportdata.WriteReportsCSV(&buf, reports))

	rows, err := csv.NewReader(&buf).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 3) // Kopfzeile + 2 Reports

	require.Equal(t, []string{
		"OrgName", "ReportID", "Domain", "PeriodBegin", "PeriodEnd",
		"Policy", "SubdomainPolicy", "Percentage", "DKIMAlignment", "SPFAlignment",
	}, rows[0])

	require.Equal(t, "google.com", rows[1][0])
	require.Equal(t, "report-1", rows[1][1])
	require.Equal(t, "example.com", rows[1][2])
	require.Equal(t, "reject", rows[1][5])
	require.Equal(t, "report-2", rows[2][1])
}

func TestWriteReportsCSV_EmptyList_OnlyHeader(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, exportdata.WriteReportsCSV(&buf, nil))

	rows, err := csv.NewReader(&buf).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

func TestWriteRecordsCSV_HeaderAndRows(t *testing.T) {
	t.Parallel()

	r := testReport(t, "report-1")

	var buf bytes.Buffer
	require.NoError(t, exportdata.WriteRecordsCSV(&buf, &r))

	rows, err := csv.NewReader(&buf).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 2) // Kopfzeile + 1 Record

	require.Equal(t, []string{
		"SourceIP", "MessageCount", "Disposition", "DKIM", "SPF", "HeaderFrom", "EnvelopeFrom", "EnvelopeTo",
	}, rows[0])
	require.Equal(t, "203.0.113.5", rows[1][0])
	require.Equal(t, "42", rows[1][1])
	require.Equal(t, "pass", rows[1][3])
	require.Equal(t, "fail", rows[1][4])
	require.Equal(t, "bounce.example.com", rows[1][6])
}

func TestWriteRecordsCSV_NoRecords_OnlyHeader(t *testing.T) {
	t.Parallel()

	r := testReport(t, "report-1")
	r.Records = nil

	var buf bytes.Buffer
	require.NoError(t, exportdata.WriteRecordsCSV(&buf, &r))

	rows, err := csv.NewReader(&buf).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

// failingWriter liefert bei jedem Write einen Fehler — prüft, dass
// Schreibfehler durchgereicht werden, statt still ignoriert zu werden.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errWrite
}

var errWrite = &writeError{}

type writeError struct{}

func (*writeError) Error() string { return "schreibfehler" }

func TestWriteReportsCSV_WriteError_IsReported(t *testing.T) {
	t.Parallel()

	err := exportdata.WriteReportsCSV(failingWriter{}, []report.AggregateReport{testReport(t, "report-1")})
	require.Error(t, err)
}
