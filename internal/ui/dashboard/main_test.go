package dashboard

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

// TestMain startet einmalig eine Headless-Test-App — Fyne-Widgets
// brauchen für Layout/Textmessung eine laufende fyne.App
// (fyne.CurrentApp()), auch ohne echtes Fenster.
func TestMain(m *testing.M) {
	test.NewApp()
	m.Run()
}
