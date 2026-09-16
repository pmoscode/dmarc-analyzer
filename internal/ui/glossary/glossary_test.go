package glossary_test

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/ui/glossary"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/uitest"
)

func TestByName_KnownTerm_ReturnsDefinition(t *testing.T) {
	term, ok := glossary.ByName("DMARC")
	require.True(t, ok)
	require.NotEmpty(t, term.Definition)
}

func TestByName_UnknownTerm_NotOK(t *testing.T) {
	_, ok := glossary.ByName("Kein Begriff")
	require.False(t, ok)
}

func TestNewView_RendersAllTerms(t *testing.T) {
	view := glossary.NewView()
	for _, term := range glossary.Terms {
		require.NotNil(t, uitest.FindLabel(view, term.Name), "begriff %q fehlt in der Ansicht", term.Name)
	}
}

func TestNewInfoButton_Tapped_ShowsDefinitionDialog(t *testing.T) {
	w := test.NewWindow(nil)
	defer w.Close()

	btn := glossary.NewInfoButton("SPF", w)
	w.SetContent(btn)

	require.NotPanics(t, func() { test.Tap(btn) })
}

func TestNewInfoButton_UnknownTerm_TappedDoesNothing(t *testing.T) {
	w := test.NewWindow(nil)
	defer w.Close()

	btn := glossary.NewInfoButton("Unbekannt", w)
	w.SetContent(btn)

	require.NotPanics(t, func() { test.Tap(btn) })
}
