package report_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

func TestNewSourceIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "gültiges IPv4", input: "203.0.113.5"},
		{name: "gültiges IPv6", input: "2001:db8::1"},
		{name: "mit Leerzeichen", input: "  203.0.113.5  "},
		{name: "leer", input: "", wantErr: true},
		{name: "kein IP-Format", input: "not-an-ip", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ip, err := report.NewSourceIP(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.True(t, ip.IsValid())
		})
	}
}

func TestSourceIP_String(t *testing.T) {
	t.Parallel()

	ip, err := report.NewSourceIP("203.0.113.5")
	require.NoError(t, err)
	require.Equal(t, "203.0.113.5", ip.String())
}

func TestNewDomainName(t *testing.T) {
	t.Parallel()

	domain, err := report.NewDomainName("  Example.COM  ")
	require.NoError(t, err)
	require.Equal(t, "example.com", domain.String())

	_, err = report.NewDomainName("   ")
	require.Error(t, err)
}

func TestNewDateRange(t *testing.T) {
	t.Parallel()

	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)

	dr, err := report.NewDateRange(begin, end)
	require.NoError(t, err)
	require.True(t, dr.Begin.Equal(begin))
	require.True(t, dr.End.Equal(end))

	_, err = report.NewDateRange(end, begin)
	require.Error(t, err, "Begin nach End muss abgelehnt werden")

	_, err = report.NewDateRange(begin, begin)
	require.Error(t, err, "Begin == End muss abgelehnt werden")
}

func TestDateRange_IsZero(t *testing.T) {
	t.Parallel()

	var zero report.DateRange
	require.True(t, zero.IsZero())

	dr, err := report.NewDateRange(
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)
	require.False(t, dr.IsZero())
}

func TestParseDisposition_UnknownValueIsNotDiscarded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  report.Disposition
	}{
		{name: "erkannt, klein", input: "none", want: report.DispositionNone},
		{name: "erkannt, groß", input: "QUARANTINE", want: report.DispositionQuarantine},
		{name: "erkannt, mit Leerzeichen", input: " reject ", want: report.DispositionReject},
		{name: "leer", input: "", want: report.DispositionUnknown},
		{name: "unbekannt", input: "unexpected", want: report.DispositionUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, report.ParseDisposition(tt.input))
		})
	}
}

func TestParsePolicy_UnknownValueIsNotDiscarded(t *testing.T) {
	t.Parallel()

	tests := map[string]report.Policy{
		"none":           report.PolicyNone,
		"quarantine":     report.PolicyQuarantine,
		"reject":         report.PolicyReject,
		"was-auch-immer": report.PolicyUnknown,
	}

	for input, want := range tests {
		require.Equal(t, want, report.ParsePolicy(input), "input %q", input)
	}
}

func TestParseAlignmentMode_UnknownValueIsNotDiscarded(t *testing.T) {
	t.Parallel()

	require.Equal(t, report.AlignmentRelaxed, report.ParseAlignmentMode("r"))
	require.Equal(t, report.AlignmentStrict, report.ParseAlignmentMode("s"))
	require.Equal(t, report.AlignmentUnknown, report.ParseAlignmentMode(""))
	require.Equal(t, report.AlignmentUnknown, report.ParseAlignmentMode("x"))
}

func TestParseAuthResultValue_CoversBothDKIMAndSPFEnums(t *testing.T) {
	t.Parallel()

	tests := map[string]report.AuthResultValue{
		"none":      report.AuthResultNone,
		"pass":      report.AuthResultPass,
		"fail":      report.AuthResultFail,
		"softfail":  report.AuthResultSoftfail,
		"neutral":   report.AuthResultNeutral,
		"policy":    report.AuthResultPolicy,
		"temperror": report.AuthResultTempError,
		"permerror": report.AuthResultPermError,
		"garbage":   report.AuthResultUnknown,
	}

	for input, want := range tests {
		require.Equal(t, want, report.ParseAuthResultValue(input), "input %q", input)
	}
}
