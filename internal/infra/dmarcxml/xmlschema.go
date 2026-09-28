package dmarcxml

import "encoding/xml"

// The following structs mirror the XML schema from RFC 7489 Appendix C.
// Deliberately string-based and without their own validation: conversion
// into domain types (including the Unknown fallback for RFC-deviating enum
// values) only happens during mapping in parser.go. This keeps the schema
// tolerant of providers that deviate from the RFC — a strict type (e.g. int
// for "pct") would fail the entire report at a single bad spot, before we
// could even decide for ourselves how tolerant to be.
type feedback struct {
	XMLName         xml.Name        `xml:"feedback"`
	ReportMetadata  reportMetadata  `xml:"report_metadata"`
	PolicyPublished policyPublished `xml:"policy_published"`
	Records         []record        `xml:"record"`
}

type reportMetadata struct {
	OrgName          string    `xml:"org_name"`
	Email            string    `xml:"email"`
	ExtraContactInfo string    `xml:"extra_contact_info"`
	ReportID         string    `xml:"report_id"`
	DateRange        dateRange `xml:"date_range"`
	Errors           []string  `xml:"error"`
}

// dateRange uses int64 instead of time.Time: the XML supplies Unix seconds
// as a number, not an ISO timestamp.
type dateRange struct {
	Begin int64 `xml:"begin"`
	End   int64 `xml:"end"`
}

type policyPublished struct {
	Domain          string `xml:"domain"`
	ADKIM           string `xml:"adkim"`
	ASPF            string `xml:"aspf"`
	Policy          string `xml:"p"`
	SubdomainPolicy string `xml:"sp"`
	// Percentage stays a string: "pct" is optional per RFC (default 100);
	// we only distinguish "missing" from "0" during mapping.
	Percentage     string `xml:"pct"`
	FailureOptions string `xml:"fo"`
}

type record struct {
	Row         row         `xml:"row"`
	Identifiers identifiers `xml:"identifiers"`
	AuthResults authResults `xml:"auth_results"`
}

type row struct {
	SourceIP        string          `xml:"source_ip"`
	Count           int             `xml:"count"`
	PolicyEvaluated policyEvaluated `xml:"policy_evaluated"`
}

type policyEvaluated struct {
	Disposition string         `xml:"disposition"`
	DKIM        string         `xml:"dkim"`
	SPF         string         `xml:"spf"`
	Reasons     []policyReason `xml:"reason"`
}

type policyReason struct {
	Type    string `xml:"type"`
	Comment string `xml:"comment"`
}

type identifiers struct {
	EnvelopeTo   string `xml:"envelope_to"`
	EnvelopeFrom string `xml:"envelope_from"`
	HeaderFrom   string `xml:"header_from"`
}

type authResults struct {
	DKIM []dkimAuthResult `xml:"dkim"`
	SPF  []spfAuthResult  `xml:"spf"`
}

type dkimAuthResult struct {
	Domain      string `xml:"domain"`
	Selector    string `xml:"selector"`
	Result      string `xml:"result"`
	HumanResult string `xml:"human_result"`
}

type spfAuthResult struct {
	Domain string `xml:"domain"`
	Scope  string `xml:"scope"`
	Result string `xml:"result"`
}
