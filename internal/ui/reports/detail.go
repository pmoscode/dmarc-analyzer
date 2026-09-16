package reports

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/i18n"
)

// NewDetailView baut die Bericht-Detailansicht: Metadaten, veröffentlichte
// Richtlinie, Sendequellen (IMPLEMENTIERUNG.md Abschnitt 10.1). r muss
// vollständig geladen sein (inklusive Records, siehe
// report.Repository.FindByID-Dokumentation) — anders als die Zeilen der
// Berichtstabelle, die bewusst ohne Records auskommen.
func NewDetailView(r *report.AggregateReport) fyne.CanvasObject {
	metadata := widget.NewForm(
		widget.NewFormItem("Absender-Organisation", widget.NewLabel(r.Metadata.OrgName)),
		widget.NewFormItem("Report-ID", widget.NewLabel(r.Metadata.ReportID)),
		widget.NewFormItem("Zeitraum", widget.NewLabel(fmt.Sprintf("%s bis %s",
			r.Metadata.Range.Begin.Format("2006-01-02 15:04"), r.Metadata.Range.End.Format("2006-01-02 15:04")))),
	)

	policy := widget.NewForm(
		widget.NewFormItem("Domain", widget.NewLabel(r.Policy.Domain.String())),
		widget.NewFormItem("Richtlinie (p)", widget.NewLabel(string(r.Policy.Policy))),
		widget.NewFormItem("Subdomain-Richtlinie (sp)", widget.NewLabel(string(r.Policy.SubdomainPolicy))),
		widget.NewFormItem("DKIM-Alignment", widget.NewLabel(string(r.Policy.DKIMAlignment))),
		widget.NewFormItem("SPF-Alignment", widget.NewLabel(string(r.Policy.SPFAlignment))),
		widget.NewFormItem("Prozentsatz", widget.NewLabel(fmt.Sprintf("%d %%", r.Policy.Percentage))),
	)

	recordsTable := newRecordsTable(r.Records)

	content := container.NewVBox(
		widget.NewLabelWithStyle(i18n.ReportDetailMetadata, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		metadata,
		widget.NewSeparator(),
		widget.NewLabelWithStyle(i18n.ReportDetailPolicy, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		policy,
		widget.NewSeparator(),
		widget.NewLabelWithStyle(fmt.Sprintf("%s (%d)", i18n.ReportDetailRecords, len(r.Records)), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		recordsTable,
	)

	return container.NewVScroll(content)
}

// recordColumns sind die Spaltentitel der Sendequellen-Tabelle, in der
// Reihenfolge, die newRecordsTable auch für die Zellen benutzt.
var recordColumns = []string{
	i18n.ReportDetailColumnIP,
	i18n.ReportDetailColumnCount,
	i18n.ReportDetailColumnDisp,
	i18n.ReportDetailColumnDKIM,
	i18n.ReportDetailColumnSPF,
}

func newRecordsTable(records []report.Record) fyne.CanvasObject {
	if len(records) == 0 {
		return widget.NewLabel("Keine Sendequellen in diesem Bericht.")
	}

	table := widget.NewTableWithHeaders(
		func() (int, int) { return len(records), len(recordColumns) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			rec := records[id.Row]
			switch id.Col {
			case 0:
				label.SetText(rec.SourceIP.String())
			case 1:
				label.SetText(fmt.Sprintf("%d", rec.Count))
			case 2:
				label.SetText(string(rec.Evaluated.Disposition))
			case 3:
				label.SetText(string(rec.Evaluated.DKIM))
			case 4:
				label.SetText(string(rec.Evaluated.SPF))
			}
		},
	)
	table.CreateHeader = func() fyne.CanvasObject {
		return widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	}
	table.UpdateHeader = func(id widget.TableCellID, obj fyne.CanvasObject) {
		label := obj.(*widget.Label)
		if id.Col >= 0 && id.Col < len(recordColumns) {
			label.SetText(recordColumns[id.Col])
		}
	}
	table.SetColumnWidth(0, 140)
	table.SetColumnWidth(1, 100)
	table.SetColumnWidth(2, 110)

	// Feste Höhe, damit die Tabelle innerhalb des scrollbaren
	// Detail-Dialogs nicht unbegrenzt wächst.
	tableContainer := container.NewGridWrap(fyne.NewSize(600, 240), table)
	return tableContainer
}
