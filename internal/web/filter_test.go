package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func newFilterRequest(t *testing.T, rawQuery string) *http.Request {
	t.Helper()
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?"+rawQuery, nil)
	return r
}

func TestParseFilterParams_NoQuery_UsesDefaults(t *testing.T) {
	f := parseFilterParams(newFilterRequest(t, ""))
	require.Equal(t, defaultPeriodDays, f.Days)
	require.Empty(t, f.Domain)
}

func TestParseFilterParams_ValidZeitraum_IsUsed(t *testing.T) {
	f := parseFilterParams(newFilterRequest(t, "zeitraum=7"))
	require.Equal(t, 7, f.Days)
}

func TestParseFilterParams_InvalidZeitraum_FallsBackToDefault(t *testing.T) {
	for _, raw := range []string{"zeitraum=13", "zeitraum=abc", "zeitraum=-30", "zeitraum="} {
		f := parseFilterParams(newFilterRequest(t, raw))
		require.Equal(t, defaultPeriodDays, f.Days, raw)
	}
}

func TestParseFilterParams_Domain_IsTrimmed(t *testing.T) {
	f := parseFilterParams(newFilterRequest(t, "domain=+example.com+"))
	require.Equal(t, "example.com", f.Domain)
}

func TestFilterParams_Options_MarksCurrentSelection(t *testing.T) {
	f := filterParams{Days: 90}
	opts := f.options()
	require.Len(t, opts, len(periodDayOptions))

	var selected []int
	for _, o := range opts {
		if o.Selected {
			selected = append(selected, o.Days)
		}
	}
	require.Equal(t, []int{90}, selected)
}

func TestFilterParams_Query_BuildsRangeEndingNow(t *testing.T) {
	f := filterParams{Days: 30, Domain: "example.com"}
	q, err := f.query()
	require.NoError(t, err)
	require.Equal(t, "example.com", q.Domain)
	require.WithinDuration(t, q.Period.End, q.Period.Begin.AddDate(0, 0, 30), 0)
}
