package components_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/ui/components"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/uitest"
)

func TestErrorBanner_InitiallyHidden(t *testing.T) {
	b := components.NewErrorBanner()
	require.False(t, b.Visible())
}

func TestErrorBanner_SetError_ShowsMessage(t *testing.T) {
	b := components.NewErrorBanner()
	b.SetError("Verbindung fehlgeschlagen.", "dial tcp: connection refused")

	require.True(t, b.Visible())
	require.NotNil(t, uitest.FindLabel(b, "Verbindung fehlgeschlagen."))
}

func TestErrorBanner_SetError_WithoutDetail_StillShowsMessage(t *testing.T) {
	b := components.NewErrorBanner()
	b.SetError("Nachricht ohne Details.", "")

	require.True(t, b.Visible())
	require.NotNil(t, uitest.FindLabel(b, "Nachricht ohne Details."))
}

func TestErrorBanner_SetErrorTwice_ReplacesMessage(t *testing.T) {
	b := components.NewErrorBanner()
	b.SetError("Erste Meldung.", "")
	b.SetError("Zweite Meldung.", "")

	require.Nil(t, uitest.FindLabel(b, "Erste Meldung."))
	require.NotNil(t, uitest.FindLabel(b, "Zweite Meldung."))
}
