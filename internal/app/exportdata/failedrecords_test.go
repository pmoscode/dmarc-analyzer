package exportdata_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/exportdata"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/failedrecords"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

func TestWriteFailedRecordsCSV_HeaderAndRows(t *testing.T) {
	ip, err := report.NewSourceIP("203.0.113.1")
	require.NoError(t, err)
	domain, err := report.NewDomainName("example.com")
	require.NoError(t, err)
	headerFrom, err := report.NewDomainName("example.com")
	require.NoError(t, err)

	var buf bytes.Buffer
	err = exportdata.WriteFailedRecordsCSV(&buf, []failedrecords.Record{{
		ReportID:     42,
		OrgName:      "Google",
		PolicyDomain: domain,
		PeriodBegin:  time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:    time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		SourceIP:     ip,
		Count:        7,
		Disposition:  report.DispositionReject,
		DKIM:         report.AuthResultFail,
		SPF:          report.AuthResultFail,
		Identifiers:  report.Identifiers{HeaderFrom: headerFrom, EnvelopeFrom: "bounce@example.com", EnvelopeTo: "empfang@example.org"},
	}})
	require.NoError(t, err)

	out := buf.String()
	require.Contains(t, out, "OrgName,ReportID,PolicyDomain,PeriodBegin,PeriodEnd,SourceIP,MessageCount,Disposition,DKIM,SPF,HeaderFrom,EnvelopeFrom,EnvelopeTo")
	require.Contains(t, out, "Google,42,example.com,")
	require.Contains(t, out, "203.0.113.1,7,reject,fail,fail,example.com,bounce@example.com,empfang@example.org")
}

func TestWriteFailedRecordsCSV_EmptyList_OnlyHeader(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, exportdata.WriteFailedRecordsCSV(&buf, nil))
	require.Equal(t, "OrgName,ReportID,PolicyDomain,PeriodBegin,PeriodEnd,SourceIP,MessageCount,Disposition,DKIM,SPF,HeaderFrom,EnvelopeFrom,EnvelopeTo\n", buf.String())
}
