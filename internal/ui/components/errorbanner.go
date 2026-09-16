package components

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ErrorBanner zeigt eine Fehlermeldung in Klartext, mit dem technischen
// Originaltext optional aufklappbar (UMSETZUNGSPLAN.md Abschnitt 3.1:
// "Fehlermeldungen in Klartext ... Technischer Originaltext aufklappbar").
// Unsichtbar (Hidden), solange kein Fehler vorliegt.
type ErrorBanner struct {
	widget.BaseWidget

	icon    *widget.Icon
	message *widget.Label
	details *widget.Accordion
}

// NewErrorBanner erzeugt ein anfangs verstecktes ErrorBanner.
func NewErrorBanner() *ErrorBanner {
	b := &ErrorBanner{
		icon:    widget.NewIcon(theme.ErrorIcon()),
		message: widget.NewLabel(""),
	}
	b.message.Wrapping = fyne.TextWrapWord
	b.details = widget.NewAccordion() // Items werden bei Show() gesetzt
	b.ExtendBaseWidget(b)
	b.Hide()
	return b
}

// SetError zeigt message in Klartext an und blendet das Banner ein;
// technicalDetail (z. B. err.Error()) steckt aufklappbar dahinter, leer
// bedeutet: kein aufklappbarer Teil. Zum Ausblenden das geerbte Hide()
// aus widget.BaseWidget verwenden.
func (b *ErrorBanner) SetError(message, technicalDetail string) {
	b.message.SetText(message)

	b.details.Items = nil
	if technicalDetail != "" {
		detailLabel := widget.NewLabel(technicalDetail)
		detailLabel.Wrapping = fyne.TextWrapWord
		b.details.Append(widget.NewAccordionItem("Technische Details", detailLabel))
	}
	b.details.Refresh()

	b.Show()
}

// CreateRenderer erfüllt fyne.Widget.
func (b *ErrorBanner) CreateRenderer() fyne.WidgetRenderer {
	content := container.NewBorder(nil, nil, b.icon, nil,
		container.NewVBox(b.message, b.details))
	return widget.NewSimpleRenderer(content)
}
