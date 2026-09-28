package web

import "github.com/pmoscode/dmarc-analyzer/internal/domain/report"

// recordDetailView is the display-ready view of a record's raw auth
// results (rec.Auth), its sender identifiers (rec.Identifiers), and any
// override reasons (rec.Evaluated.Reasons) — unlike recordRowView.DKIM/SPF
// (the already recipient-evaluated, aligned results), the raw data behind
// them: which DKIM signature/SPF check was checked exactly, with which
// domain/selector/scope. Shared by the report detail page
// (report_detail.html) and the cross-report failures view
// (failed_records.html) — both are parsed independently with layout.html
// (views.go: no shared template tree), so only the Go building block
// lives here; each template maintains its own <details> markup.
type recordDetailView struct {
	HeaderFrom   string
	EnvelopeFrom string
	EnvelopeTo   string
	DKIMResults  []dkimResultView
	SPFResults   []spfResultView
	Reasons      []reasonView
}

// dkimResultView is a single raw DKIM check result (auth_results/dkim),
// ready-prepared for the template.
type dkimResultView struct {
	Domain   string
	Selector string
	Result   string
}

// spfResultView is a single raw SPF check result (auth_results/spf),
// ready-prepared for the template.
type spfResultView struct {
	Domain string
	Scope  string
	Result string
}

// reasonView is an override reason (policy_evaluated/reason),
// ready-prepared for the template.
type reasonView struct {
	Type    string
	Comment string
}

// buildRecordDetailView builds the detail view from a record's raw data
// — deliberately takes the individual pieces instead of a report.Record,
// because the failures view (handlers_failedrecords.go) receives the
// same raw data via failedrecords.Record, its own cross-report type, not
// report.Record.
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

// authResultTone returns the badge color class (app.css: .badge-good/
// .badge-warning/.badge-critical) for an auth result value — "good" only
// for an explicit pass, "critical" only for an explicit fail, everything
// else (softfail, neutral, policy, temperror, permerror, none, unknown)
// as a warning, since it's neither clearly passed nor clearly failed.
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

// dispositionTone returns the badge color class for an applied
// disposition.
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
