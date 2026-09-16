package components_test

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/components"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/i18n"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/uitest"
)

func TestFilterBar_CurrentPeriod_DefaultsTo30Days(t *testing.T) {
	f := components.NewFilterBar()

	got := f.CurrentPeriod()
	gotDays := got.End.Sub(got.Begin).Hours() / 24
	require.InDelta(t, 30, gotDays, 0.01)
}

func TestFilterBar_CurrentDomain_TrimsWhitespace(t *testing.T) {
	f := components.NewFilterBar()

	w := test.NewWindow(f)
	defer w.Close()

	entries := uitest.FindEntries(f)
	require.Len(t, entries, 1)
	entries[0].SetText("  example.com  ")

	require.Equal(t, "example.com", f.CurrentDomain())
}

func TestFilterBar_ApplyButton_NotifiesOnChangedWithSelectedPeriodAndDomain(t *testing.T) {
	f := components.NewFilterBar()

	w := test.NewWindow(f)
	defer w.Close()

	entries := uitest.FindEntries(f)
	require.Len(t, entries, 1)
	entries[0].SetText("example.com")

	var gotPeriod report.DateRange
	var gotDomain string
	called := false
	f.OnChanged = func(period report.DateRange, domain string) {
		called = true
		gotPeriod = period
		gotDomain = domain
	}

	btn := uitest.FindButton(t, f, i18n.FilterApply)
	require.NotNil(t, btn)
	test.Tap(btn)

	require.True(t, called)
	require.Equal(t, "example.com", gotDomain)
	require.InDelta(t, 30, gotPeriod.End.Sub(gotPeriod.Begin).Hours()/24, 0.01)
}
