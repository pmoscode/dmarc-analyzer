package components_test

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/ui/components"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/uitest"
)

func TestNewEmptyState_RendersTitleAndDetail(t *testing.T) {
	obj := components.NewEmptyState("Titel", "Detailtext", "", nil)
	require.NotNil(t, obj)

	require.NotNil(t, uitest.FindLabel(obj, "Titel"))
	require.NotNil(t, uitest.FindLabel(obj, "Detailtext"))
}

func TestNewEmptyState_WithoutAction_NoButton(t *testing.T) {
	obj := components.NewEmptyState("Titel", "Detail", "", nil)
	require.Zero(t, uitest.CountWidgets(obj, func(o fyne.CanvasObject) bool {
		_, ok := o.(*widget.Button)
		return ok
	}))
}

func TestNewEmptyState_WithAction_ButtonTriggersCallback(t *testing.T) {
	called := false
	obj := components.NewEmptyState("Titel", "Detail", "Aktion", func() { called = true })

	w := test.NewWindow(obj)
	defer w.Close()

	btn := uitest.FindButton(t, obj, "Aktion")
	test.Tap(btn)
	require.True(t, called)
}
