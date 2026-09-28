package report_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

func TestNewPublishedPolicy_PercentageMustBeInRange(t *testing.T) {
	t.Parallel()

	domain, err := report.NewDomainName("example.com")
	require.NoError(t, err)

	tests := []struct {
		name       string
		percentage int
		wantErr    bool
	}{
		{name: "lower bound", percentage: 0},
		{name: "upper bound", percentage: 100},
		{name: "typical value", percentage: 100},
		{name: "negative", percentage: -1, wantErr: true},
		{name: "above 100", percentage: 101, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := report.NewPublishedPolicy(
				domain,
				report.PolicyNone,
				report.PolicyReject,
				report.AlignmentRelaxed,
				report.AlignmentStrict,
				tt.percentage,
				"",
			)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
