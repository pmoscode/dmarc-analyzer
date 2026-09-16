package glossary

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/pmoscode/dmarc-analyzer/internal/ui/i18n"
)

// NewView baut die vollständige Glossar-Liste als scrollbaren Inhalt.
func NewView() fyne.CanvasObject {
	box := container.NewVBox()
	for _, t := range Terms {
		detail := widget.NewLabel(t.Definition)
		detail.Wrapping = fyne.TextWrapWord
		box.Add(widget.NewLabelWithStyle(t.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		box.Add(detail)
	}
	return container.NewVScroll(box)
}

// ShowDialog öffnet das vollständige Glossar als Dialog.
func ShowDialog(window fyne.Window) {
	d := dialog.NewCustom(i18n.GlossaryTitle, i18n.ButtonOK, NewView(), window)
	d.Resize(fyne.NewSize(500, 500))
	d.Show()
}

// NewInfoButton erzeugt einen kleinen "?"-Knopf, der beim Antippen die
// Erklärung von termName als Dialog zeigt — Ersatz für Hover-Tooltips
// (siehe Paketdokumentation). termName muss ein Begriff aus Terms sein;
// ein unbekannter Name ist ein Programmierfehler und wird stillschweigend
// ignoriert (kein Panic für eine reine Hilfetext-Funktion).
func NewInfoButton(termName string, window fyne.Window) *widget.Button {
	return widget.NewButton(i18n.GlossaryButtonLabel, func() {
		term, ok := ByName(termName)
		if !ok {
			return
		}
		dialog.ShowInformation(term.Name, term.Definition, window)
	})
}
