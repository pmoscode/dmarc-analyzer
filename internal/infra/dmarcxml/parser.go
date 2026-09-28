// Package dmarcxml implements the sync.ReportParser port: unpacks
// attachments (plain .xml, .xml.gz, .zip) and parses DMARC aggregate XML
// (RFC 7489 Appendix C) into domain objects.
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

// defaultPercentage is the default RFC 7489 mandates for "pct" when the
// field is missing from the XML.
const defaultPercentage = 100

// Parser implements sync.ReportParser for DMARC aggregate reports (RUA)
// and tolerates RFC deviations by individual providers: unknown enum
// values are mapped to Unknown instead of aborting the import.
type Parser struct {
	// Clock supplies the timestamp for AggregateReport.ImportedAt.
	// Defaults to time.Now; swappable in tests.
	Clock func() time.Time
}

var _ sync.ReportParser = (*Parser)(nil)

// NewParser creates a ready-to-use Parser.
func NewParser() *Parser {
	return &Parser{Clock: time.Now}
}

// Supports determines from filename and content whether an attachment is a
// supported DMARC aggregate format: plain .xml, .xml.gz/.gz, or .zip
// containing at least one .xml file.
func (p *Parser) Supports(attachment sync.RawAttachment) bool {
	name := strings.ToLower(attachment.Filename)
	switch {
	case strings.HasSuffix(name, ".xml"),
		strings.HasSuffix(name, ".gz"),
		strings.HasSuffix(name, ".zip"):
		return true
	default:
		// Fallback via magic bytes: some providers only append the
		// report ID without a meaningful extension.
		return isGzip(attachment.Data) || isZip(attachment.Data) || looksLikeXML(attachment.Data)
	}
}

// Parse unpacks the attachment and converts the first contained XML
// document into an AggregateReport. For .zip attachments with multiple
// reports, see ParseAll.
func (p *Parser) Parse(ctx context.Context, attachment sync.RawAttachment) (*report.AggregateReport, error) {
	reports, err := p.ParseAll(ctx, attachment)
	if err != nil {
		return nil, err
	}
	return reports[0], nil
}

// ParseAll fully unpacks the attachment and returns all contained
// reports — potentially several for .zip. Aborts immediately on
// ctx.Err() (IMPLEMENTIERUNG.md section 7.2: every step honors
// context.Context).
func (p *Parser) ParseAll(ctx context.Context, attachment sync.RawAttachment) ([]*report.AggregateReport, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	xmlDocs, err := extractXML(attachment.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack attachment %q: %w", attachment.Filename, err)
	}

	reports := make([]*report.AggregateReport, 0, len(xmlDocs))
	for _, doc := range xmlDocs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		r, err := p.parseDocument(doc)
		if err != nil {
			return nil, fmt.Errorf("attachment %q: %w", attachment.Filename, err)
		}
		reports = append(reports, r)
	}

	return reports, nil
}

func (p *Parser) parseDocument(data []byte) (*report.AggregateReport, error) {
	var fb feedback
	decoder := xml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&fb); err != nil {
		return nil, fmt.Errorf("failed to decode xml: %w", err)
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
		return report.Metadata{}, fmt.Errorf("invalid date range: %w", err)
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
		return report.PublishedPolicy{}, fmt.Errorf("invalid policy domain: %w", err)
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

// parseAlignmentModeWithDefault applies the RFC 7489 default "r" (relaxed)
// when adkim/aspf are missing from the XML — many providers omit both
// fields because "r" is already the default.
func parseAlignmentModeWithDefault(raw string) report.AlignmentMode {
	if strings.TrimSpace(raw) == "" {
		return report.AlignmentRelaxed
	}
	return report.ParseAlignmentMode(raw)
}

// parsePercentage applies the RFC 7489 default when "pct" is missing.
func parsePercentage(raw string) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultPercentage, nil
	}

	pct, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid pct value %q: %w", raw, err)
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
		return report.Record{}, fmt.Errorf("invalid source ip: %w", err)
	}

	headerFrom, err := report.NewDomainName(r.Identifiers.HeaderFrom)
	if err != nil {
		return report.Record{}, fmt.Errorf("invalid header_from: %w", err)
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
