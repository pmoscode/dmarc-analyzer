package web

import "github.com/pmoscode/dmarc-analyzer/internal/domain/report"

// recordDetailView ist die für die Anzeige aufbereitete Sicht auf die
// rohen Auth-Ergebnisse eines Records (rec.Auth), seine Absenderkennungen
// (rec.Identifiers) und etwaige Override-Gründe (rec.Evaluated.Reasons) —
// im Unterschied zu recordRowView.DKIM/SPF (die bereits vom Empfänger
// ausgewerteten, aligned Ergebnisse) die Rohdaten dahinter: welche
// DKIM-Signatur/SPF-Prüfung genau mit welchem Domain/Selector/Scope
// geprüft wurde. Gemeinsam genutzt von der Berichts-Detailseite
// (report_detail.html) und der berichtsübergreifenden
// Fehlschläge-Ansicht (failed_records.html) — beide werden unabhängig
// voneinander mit layout.html geparst (views.go: kein gemeinsamer
// Vorlagenbaum), deshalb liegt hier nur der Go-Baustein, das
// <details>-Markup pflegt jede Vorlage für sich.
type recordDetailView struct {
	HeaderFrom   string
	EnvelopeFrom string
	EnvelopeTo   string
	DKIMResults  []dkimResultView
	SPFResults   []spfResultView
	Reasons      []reasonView
}

// dkimResultView ist ein einzelnes rohes DKIM-Prüfergebnis
// (auth_results/dkim), fertig für die Vorlage aufbereitet.
type dkimResultView struct {
	Domain   string
	Selector string
	Result   string
}

// spfResultView ist ein einzelnes rohes SPF-Prüfergebnis
// (auth_results/spf), fertig für die Vorlage aufbereitet.
type spfResultView struct {
	Domain string
	Scope  string
	Result string
}

// reasonView ist ein Override-Grund (policy_evaluated/reason), fertig für
// die Vorlage aufbereitet.
type reasonView struct {
	Type    string
	Comment string
}

// buildRecordDetailView baut die Detailsicht aus den Rohdaten eines
// Records — nimmt bewusst die einzelnen Bestandteile entgegen statt eines
// report.Record, weil die Fehlschläge-Ansicht (handlers_failedrecords.go)
// dieselben Rohdaten über failedrecords.Record erhält, einen eigenen,
// berichtsübergreifenden Typ, keinen report.Record.
func buildRecordDetailView(identifiers report.Identifiers, auth report.AuthResults, reasons []report.PolicyOverrideReason) recordDetailView {
	dkim := make([]dkimResultView, len(auth.DKIM))
	for i, d := range auth.DKIM {
		dkim[i] = dkimResultView{Domain: d.Domain, Selector: d.Selector, Result: string(d.Result)}
	}
	spf := make([]spfResultView, len(auth.SPF))
	for i, sr := range auth.SPF {
		spf[i] = spfResultView{Domain: sr.Domain, Scope: sr.Scope, Result: string(sr.Result)}
	}
	reasonViews := make([]reasonView, len(reasons))
	for i, rr := range reasons {
		reasonViews[i] = reasonView{Type: rr.Type, Comment: rr.Comment}
	}

	return recordDetailView{
		HeaderFrom:   identifiers.HeaderFrom.String(),
		EnvelopeFrom: identifiers.EnvelopeFrom,
		EnvelopeTo:   identifiers.EnvelopeTo,
		DKIMResults:  dkim,
		SPFResults:   spf,
		Reasons:      reasonViews,
	}
}

// authResultTone liefert die Badge-Farbklasse (app.css: .badge-good/
// .badge-warning/.badge-critical) für einen Auth-Result-Wert — "gut" nur
// bei explizitem Bestehen, "kritisch" nur bei explizitem Fehlschlag,
// alles andere (softfail, neutral, policy, temperror, permerror, none,
// unknown) als Warnung, weil weder eindeutig bestanden noch eindeutig
// fehlgeschlagen.
func authResultTone(v report.AuthResultValue) string {
	switch v {
	case report.AuthResultPass:
		return "good"
	case report.AuthResultFail:
		return "critical"
	default:
		return "warning"
	}
}

// dispositionTone liefert die Badge-Farbklasse für eine angewendete
// Disposition.
func dispositionTone(d report.Disposition) string {
	switch d {
	case report.DispositionNone:
		return "good"
	case report.DispositionReject:
		return "critical"
	default:
		return "warning"
	}
}
