// Package dmarcxml implementiert den Port sync.ReportParser: entpackt
// Anhänge (blankes .xml, .xml.gz, .zip) und parst DMARC-Aggregate-XML
// (RFC 7489 Anhang C) zu Domänenobjekten.
package dmarcxml

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

// defaultPercentage ist der von RFC 7489 vorgeschriebene Default für "pct",
// wenn das Feld im XML fehlt.
const defaultPercentage = 100

// Parser implementiert sync.ReportParser für DMARC-Aggregate-Reports (RUA)
// und toleriert dabei RFC-Abweichungen einzelner Provider: unbekannte
// Enum-Werte werden auf Unknown abgebildet statt den Import abzubrechen.
type Parser struct {
	// Clock liefert den Zeitpunkt für AggregateReport.ImportedAt.
	// Standardmäßig time.Now; für Tests austauschbar.
	Clock func() time.Time
}

var _ sync.ReportParser = (*Parser)(nil)

// NewParser erzeugt einen einsatzbereiten Parser.
func NewParser() *Parser {
	return &Parser{Clock: time.Now}
}

// Supports erkennt anhand von Dateiname und Inhalt, ob ein Anhang ein
// unterstütztes DMARC-Aggregate-Format ist: blankes .xml, .xml.gz/.gz oder
// .zip mit mindestens einer .xml-Datei.
func (p *Parser) Supports(attachment sync.RawAttachment) bool {
	name := strings.ToLower(attachment.Filename)
	switch {
	case strings.HasSuffix(name, ".xml"),
		strings.HasSuffix(name, ".gz"),
		strings.HasSuffix(name, ".zip"):
		return true
	default:
		// Fallback über Magic Bytes: manche Provider hängen nur die
		// Report-ID ohne aussagekräftige Endung an.
		return isGzip(attachment.Data) || isZip(attachment.Data) || looksLikeXML(attachment.Data)
	}
}

// Parse entpackt den Anhang und wandelt das erste enthaltene XML-Dokument
// zu einem AggregateReport um. Für .zip-Anhänge mit mehreren Reports siehe
// ParseAll.
func (p *Parser) Parse(ctx context.Context, attachment sync.RawAttachment) (*report.AggregateReport, error) {
	reports, err := p.ParseAll(ctx, attachment)
	if err != nil {
		return nil, err
	}
	return reports[0], nil
}

// ParseAll entpackt den Anhang vollständig und liefert alle enthaltenen
// Reports — bei .zip potenziell mehrere. Bricht bei ctx.Err() sofort ab
// (IMPLEMENTIERUNG.md Abschnitt 7.2: jeder Schritt respektiert
// context.Context).
func (p *Parser) ParseAll(ctx context.Context, attachment sync.RawAttachment) ([]*report.AggregateReport, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	xmlDocs, err := extractXML(attachment.Data)
	if err != nil {
		return nil, fmt.Errorf("anhang %q konnte nicht entpackt werden: %w", attachment.Filename, err)
	}

	reports := make([]*report.AggregateReport, 0, len(xmlDocs))
	for _, doc := range xmlDocs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		r, err := p.parseDocument(doc)
		if err != nil {
			return nil, fmt.Errorf("anhang %q: %w", attachment.Filename, err)
		}
		reports = append(reports, r)
	}

	return reports, nil
}

func (p *Parser) parseDocument(data []byte) (*report.AggregateReport, error) {
	var fb feedback
	decoder := xml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&fb); err != nil {
		return nil, fmt.Errorf("xml konnte nicht dekodiert werden: %w", err)
	}

	metadata, err := mapMetadata(fb.ReportMetadata)
	if err != nil {
		return nil, err
	}

	policy, err := mapPolicy(fb.PolicyPublished)
	if err != nil {
		return nil, err
	}

	records, err := mapRecords(fb.Records)
	if err != nil {
		return nil, err
	}

	return report.NewAggregateReport(metadata, policy, records, report.SourceReference{}, p.Clock())
}

func mapMetadata(m reportMetadata) (report.Metadata, error) {
	begin := time.Unix(m.DateRange.Begin, 0).UTC()
	end := time.Unix(m.DateRange.End, 0).UTC()

	dr, err := report.NewDateRange(begin, end)
	if err != nil {
		return report.Metadata{}, fmt.Errorf("ungültiger zeitraum: %w", err)
	}

	return report.Metadata{
		OrgName:          strings.TrimSpace(m.OrgName),
		Email:            strings.TrimSpace(m.Email),
		ExtraContactInfo: strings.TrimSpace(m.ExtraContactInfo),
		ReportID:         strings.TrimSpace(m.ReportID),
		Range:            dr,
		Errors:           m.Errors,
	}, nil
}

func mapPolicy(pub policyPublished) (report.PublishedPolicy, error) {
	domain, err := report.NewDomainName(pub.Domain)
	if err != nil {
		return report.PublishedPolicy{}, fmt.Errorf("ungültige policy-domain: %w", err)
	}

	percentage, err := parsePercentage(pub.Percentage)
	if err != nil {
		return report.PublishedPolicy{}, err
	}

	return report.NewPublishedPolicy(
		domain,
		report.ParsePolicy(pub.SubdomainPolicy),
		report.ParsePolicy(pub.Policy),
		parseAlignmentModeWithDefault(pub.ADKIM),
		parseAlignmentModeWithDefault(pub.ASPF),
		percentage,
		pub.FailureOptions,
	)
}

// parseAlignmentModeWithDefault wendet den RFC-7489-Default "r" (relaxed)
// an, wenn adkim/aspf im XML fehlen — viele Provider lassen beide Felder
// weg, weil "r" bereits der Default ist.
func parseAlignmentModeWithDefault(raw string) report.AlignmentMode {
	if strings.TrimSpace(raw) == "" {
		return report.AlignmentRelaxed
	}
	return report.ParseAlignmentMode(raw)
}

// parsePercentage wendet den RFC-7489-Default an, wenn "pct" fehlt.
func parsePercentage(raw string) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultPercentage, nil
	}

	pct, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("ungültiger pct-wert %q: %w", raw, err)
	}
	return pct, nil
}

func mapRecords(records []record) ([]report.Record, error) {
	mapped := make([]report.Record, 0, len(records))
	for i, rec := range records {
		r, err := mapRecord(rec)
		if err != nil {
			return nil, fmt.Errorf("record #%d: %w", i, err)
		}
		mapped = append(mapped, r)
	}
	return mapped, nil
}

func mapRecord(r record) (report.Record, error) {
	sourceIP, err := report.NewSourceIP(r.Row.SourceIP)
	if err != nil {
		return report.Record{}, fmt.Errorf("ungültige quell-ip: %w", err)
	}

	headerFrom, err := report.NewDomainName(r.Identifiers.HeaderFrom)
	if err != nil {
		return report.Record{}, fmt.Errorf("ungültiger header_from: %w", err)
	}

	evaluated := report.PolicyEvaluation{
		Disposition: report.ParseDisposition(r.Row.PolicyEvaluated.Disposition),
		DKIM:        report.ParseAuthResultValue(r.Row.PolicyEvaluated.DKIM),
		SPF:         report.ParseAuthResultValue(r.Row.PolicyEvaluated.SPF),
		Reasons:     mapReasons(r.Row.PolicyEvaluated.Reasons),
	}

	identifiers := report.Identifiers{
		HeaderFrom:   headerFrom,
		EnvelopeFrom: strings.TrimSpace(r.Identifiers.EnvelopeFrom),
		EnvelopeTo:   strings.TrimSpace(r.Identifiers.EnvelopeTo),
	}

	auth := report.AuthResults{
		DKIM: mapDKIMResults(r.AuthResults.DKIM),
		SPF:  mapSPFResults(r.AuthResults.SPF),
	}

	return report.NewRecord(sourceIP, r.Row.Count, evaluated, identifiers, auth)
}

func mapReasons(reasons []policyReason) []report.PolicyOverrideReason {
	mapped := make([]report.PolicyOverrideReason, 0, len(reasons))
	for _, reason := range reasons {
		mapped = append(mapped, report.PolicyOverrideReason{
			Type:    reason.Type,
			Comment: reason.Comment,
		})
	}
	return mapped
}

func mapDKIMResults(results []dkimAuthResult) []report.DKIMAuthResult {
	mapped := make([]report.DKIMAuthResult, 0, len(results))
	for _, res := range results {
		mapped = append(mapped, report.DKIMAuthResult{
			Domain:      res.Domain,
			Selector:    res.Selector,
			Result:      report.ParseAuthResultValue(res.Result),
			HumanResult: res.HumanResult,
		})
	}
	return mapped
}

func mapSPFResults(results []spfAuthResult) []report.SPFAuthResult {
	mapped := make([]report.SPFAuthResult, 0, len(results))
	for _, res := range results {
		mapped = append(mapped, report.SPFAuthResult{
			Domain: res.Domain,
			Scope:  res.Scope,
			Result: report.ParseAuthResultValue(res.Result),
		})
	}
	return mapped
}
