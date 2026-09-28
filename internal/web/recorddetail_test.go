package web

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

func TestBuildRecordDetailView_MapsRawAuthResultsAndReasons(t *testing.T) {
	identifiers := report.Identifiers{HeaderFrom: mustDomainName("example.com"), EnvelopeFrom: "bounce@example.com", EnvelopeTo: "empfang@example.org"}
	auth := report.AuthResults{
		DKIM: []report.DKIMAuthResult{{Domain: "example.com", Selector: "sel1", Result: report.AuthResultFail}},
		SPF:  []report.SPFAuthResult{{Domain: "example.net", Scope: "mfrom", Result: report.AuthResultFail}},
	}
	reasons := []report.PolicyOverrideReason{{Type: "forwarded", Comment: "bekannter Verteiler"}}

	got := buildRecordDetailView(identifiers, auth, reasons)

	require.Equal(t, "example.com", got.HeaderFrom)
	require.Equal(t, "bounce@example.com", got.EnvelopeFrom)
	require.Equal(t, "empfang@example.org", got.EnvelopeTo)
	require.Equal(t, []dkimResultView{{Domain: "example.com", Selector: "sel1", Result: "fail"}}, got.DKIMResults)
	require.Equal(t, []spfResultView{{Domain: "example.net", Scope: "mfrom", Result: "fail"}}, got.SPFResults)
	require.Equal(t, []reasonView{{Type: "forwarded", Comment: "bekannter Verteiler"}}, got.Reasons)
}

func TestAuthResultTone(t *testing.T) {
	require.Equal(t, "good", authResultTone(report.AuthResultPass))
	require.Equal(t, "critical", authResultTone(report.AuthResultFail))
	require.Equal(t, "warning", authResultTone(report.AuthResultNeutral))
}

func TestDispositionTone(t *testing.T) {
	require.Equal(t, "good", dispositionTone(report.DispositionNone))
	require.Equal(t, "critical", dispositionTone(report.DispositionReject))
	require.Equal(t, "warning", dispositionTone(report.DispositionQuarantine))
}
