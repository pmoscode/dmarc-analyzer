package exportdata_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/exportdata"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/domainstats"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

func TestWriteDomainStatsCSV_HeaderAndRows(t *testing.T) {
	domain, err := report.NewDomainName("example.com")
	require.NoError(t, err)

	var buf bytes.Buffer
	err = exportdata.WriteDomainStatsCSV(&buf, []domainstats.Stat{{
		Domain: domain, TotalCount: 42, PassRate: 0.5, ReportCount: 3, DistinctSources: 2,
		FirstSeen: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		LastSeen:  time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC),
	}})
	require.NoError(t, err)

	out := buf.String()
	require.Contains(t, out, "Domain,TotalCount,PassRate,ReportCount,DistinctSources,FirstSeen,LastSeen")
	require.Contains(t, out, "example.com,42,0.5000,3,2,")
}

func TestWriteDomainStatsCSV_EmptyList_OnlyHeader(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, exportdata.WriteDomainStatsCSV(&buf, nil))
	require.Equal(t, "Domain,TotalCount,PassRate,ReportCount,DistinctSources,FirstSeen,LastSeen\n", buf.String())
}
