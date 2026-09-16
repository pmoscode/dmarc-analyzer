package report_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

func validSourceIP(t *testing.T) report.SourceIP {
	t.Helper()
	ip, err := report.NewSourceIP("203.0.113.5")
	require.NoError(t, err)
	return ip
}

func TestNewRecord_CountMustBePositive(t *testing.T) {
	t.Parallel()

	ip := validSourceIP(t)

	_, err := report.NewRecord(ip, 0, report.PolicyEvaluation{}, report.Identifiers{}, report.AuthResults{})
	require.Error(t, err)

	_, err = report.NewRecord(ip, -1, report.PolicyEvaluation{}, report.Identifiers{}, report.AuthResults{})
	require.Error(t, err)

	rec, err := report.NewRecord(ip, 42, report.PolicyEvaluation{}, report.Identifiers{}, report.AuthResults{})
	require.NoError(t, err)
	require.Equal(t, 42, rec.Count)
}

func TestNewRecord_RequiresValidSourceIP(t *testing.T) {
	t.Parallel()

	var zero report.SourceIP

	_, err := report.NewRecord(zero, 1, report.PolicyEvaluation{}, report.Identifiers{}, report.AuthResults{})
	require.Error(t, err)
}
