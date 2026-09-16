// Package components enthält wiederverwendbare Fyne-Widgets, die von
// mehreren Ansichten genutzt werden (IMPLEMENTIERUNG.md Abschnitt 5).
package components

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// NewEmptyState baut eine zentrierte Leerzustand-Ansicht: Titel,
// erklärender Text, optional ein Aktions-Button. Jede Ansicht ohne Daten
// erklärt, warum sie leer ist und welcher Knopf weiterhilft
// (UMSETZUNGSPLAN.md Abschnitt 3.1: "Leerzustände mit
// Handlungsaufforderung").
func NewEmptyState(title, detail, actionLabel string, action func()) fyne.CanvasObject {
	titleLabel := widget.NewLabelWithStyle(title, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	detailLabel := widget.NewLabel(detail)
	detailLabel.Alignment = fyne.TextAlignCenter
	detailLabel.Wrapping = fyne.TextWrapWord

	items := []fyne.CanvasObject{
		widget.NewIcon(theme.InfoIcon()),
		titleLabel,
		detailLabel,
	}
	if actionLabel != "" && action != nil {
		btn := widget.NewButton(actionLabel, action)
		btn.Importance = widget.HighImportance
		items = append(items, btn)
	}

	content := container.NewVBox(items...)
	return container.NewCenter(content)
}
