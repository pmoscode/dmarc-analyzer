package exportdata_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/exportdata"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/sources"
)

func TestWriteSourceStatsCSV_HeaderAndRows(t *testing.T) {
	ip, err := report.NewSourceIP("203.0.113.1")
	require.NoError(t, err)

	var buf bytes.Buffer
	err = exportdata.WriteSourceStatsCSV(&buf, []sources.Stat{{
		SourceIP: ip, TotalCount: 42, PassRate: 0.5, DKIMPassRate: 1, SPFPassRate: 0,
		Enrichment: sources.Enrichment{Hostname: "mail.example.com", Service: "Beispieldienst"},
		FirstSeen:  time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		LastSeen:   time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC),
	}})
	require.NoError(t, err)

	out := buf.String()
	require.Contains(t, out, "SourceIP,TotalCount,PassRate,DKIMPassRate,SPFPassRate,Hostname,Service,FirstSeen,LastSeen")
	require.Contains(t, out, "203.0.113.1,42,0.5000,1.0000,0.0000,mail.example.com,Beispieldienst")
}

func TestWriteSourceStatsCSV_EmptyList_OnlyHeader(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, exportdata.WriteSourceStatsCSV(&buf, nil))
	require.Equal(t, "SourceIP,TotalCount,PassRate,DKIMPassRate,SPFPassRate,Hostname,Service,FirstSeen,LastSeen\n", buf.String())
}
