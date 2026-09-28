package report

import (
	"fmt"
	"net/netip"
	"strings"
	"time"
)

// SourceIP is the IP address of a sending source. Immutable, without
// identity — a value object in the DDD sense.
type SourceIP struct {
	addr netip.Addr
}

// NewSourceIP parses an IPv4 or IPv6 address.
func NewSourceIP(s string) (SourceIP, error) {
	addr, err := netip.ParseAddr(strings.TrimSpace(s))
	if err != nil {
		return SourceIP{}, fmt.Errorf("invalid source IP %q: %w", s, err)
	}
	return SourceIP{addr: addr}, nil
}

// String returns the textual representation of the address.
func (ip SourceIP) String() string {
	return ip.addr.String()
}

// IsValid reports whether the address is set (zero-value detection).
func (ip SourceIP) IsValid() bool {
	return ip.addr.IsValid()
}

// DomainName is a normalized domain name (lowercased, without
// leading/trailing whitespace). Value object.
type DomainName struct {
	value string
}

// NewDomainName normalizes and validates a domain name. An empty domain
// name is invalid as a business rule — callers with optional domain
// fields (e.g. envelope_from) deliberately forgo this type and use a raw
// string.
func NewDomainName(s string) (DomainName, error) {
	normalized := strings.ToLower(strings.TrimSpace(s))
	if normalized == "" {
		return DomainName{}, fmt.Errorf("domain name must not be empty")
	}
	return DomainName{value: normalized}, nil
}

// String returns the normalized domain name.
func (d DomainName) String() string {
	return d.value
}

// DateRange is a period with Begin < End, both in UTC.
type DateRange struct {
	Begin time.Time
	End   time.Time
}

// NewDateRange enforces Begin < End and normalizes both points in time to UTC.
func NewDateRange(begin, end time.Time) (DateRange, error) {
	if !begin.Before(end) {
		return DateRange{}, fmt.Errorf("begin (%s) must be before end (%s)", begin, end)
	}
	return DateRange{Begin: begin.UTC(), End: end.UTC()}, nil
}

// IsZero reports whether the period is unset.
func (r DateRange) IsZero() bool {
	return r.Begin.IsZero() && r.End.IsZero()
}

// Disposition is the action actually applied by the recipient (RFC 7489
// section 3.1.2, field "disposition" in policy_evaluated).
type Disposition string

// Values for Disposition (RFC 7489 section 3.1.2).
const (
	DispositionNone       Disposition = "none"
	DispositionQuarantine Disposition = "quarantine"
	DispositionReject     Disposition = "reject"
	// DispositionUnknown covers values that deviate from the RFC —
	// providers don't always stick to the RFC, and such reports should
	// still be importable (see IMPLEMENTIERUNG.md section 6.2).
	DispositionUnknown Disposition = "unknown"
)

// ParseDisposition converts a raw XML value into a Disposition. Unknown
// values are never discarded, but mapped to DispositionUnknown.
func ParseDisposition(s string) Disposition {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case string(DispositionNone):
		return DispositionNone
	case string(DispositionQuarantine):
		return DispositionQuarantine
	case string(DispositionReject):
		return DispositionReject
	default:
		return DispositionUnknown
	}
}

// Policy is a published DMARC policy (fields "p"/"sp" in
// policy_published). Has the same range of values as Disposition, but is
// a different statement in business terms (intent instead of applied
// action) — hence its own type.
type Policy string

// Values for Policy (RFC 7489, fields "p"/"sp").
const (
	PolicyNone       Policy = "none"
	PolicyQuarantine Policy = "quarantine"
	PolicyReject     Policy = "reject"
	PolicyUnknown    Policy = "unknown"
)

// ParsePolicy converts a raw XML value into a Policy. Unknown values are
// mapped to PolicyUnknown, not discarded.
func ParsePolicy(s string) Policy {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case string(PolicyNone):
		return PolicyNone
	case string(PolicyQuarantine):
		return PolicyQuarantine
	case string(PolicyReject):
		return PolicyReject
	default:
		return PolicyUnknown
	}
}

// AlignmentMode is the alignment mode for DKIM or SPF: relaxed (r) or
// strict (s), fields "adkim"/"aspf".
type AlignmentMode string

// Values for AlignmentMode (RFC 7489, fields "adkim"/"aspf").
const (
	AlignmentRelaxed AlignmentMode = "r"
	AlignmentStrict  AlignmentMode = "s"
	AlignmentUnknown AlignmentMode = "unknown"
)

// ParseAlignmentMode converts a raw XML value. If the value is missing
// (many providers omit adkim/aspf), RFC 7489 mandates the default "r" —
// the caller sets this default, not ParseAlignmentMode.
func ParseAlignmentMode(s string) AlignmentMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case string(AlignmentRelaxed):
		return AlignmentRelaxed
	case string(AlignmentStrict):
		return AlignmentStrict
	default:
		return AlignmentUnknown
	}
}

// AuthResultValue is a single result of a DKIM or SPF check. Both RFC
// enums (DKIM: none/pass/fail/policy/neutral/temperror/permerror; SPF:
// none/neutral/pass/fail/softfail/temperror/permerror) are kept in one
// shared type, since both overlap heavily and separate types wouldn't add
// business value.
type AuthResultValue string

// Values for AuthResultValue, unified from the DKIM and SPF enums of RFC
// 7489.
const (
	AuthResultNone      AuthResultValue = "none"
	AuthResultPass      AuthResultValue = "pass"
	AuthResultFail      AuthResultValue = "fail"
	AuthResultSoftfail  AuthResultValue = "softfail"
	AuthResultNeutral   AuthResultValue = "neutral"
	AuthResultPolicy    AuthResultValue = "policy"
	AuthResultTempError AuthResultValue = "temperror"
	AuthResultPermError AuthResultValue = "permerror"
	AuthResultUnknown   AuthResultValue = "unknown"
)

// ParseAuthResultValue converts a raw XML value. Unknown values are
// mapped to AuthResultUnknown, not discarded.
func ParseAuthResultValue(s string) AuthResultValue {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case string(AuthResultNone):
		return AuthResultNone
	case string(AuthResultPass):
		return AuthResultPass
	case string(AuthResultFail):
		return AuthResultFail
	case string(AuthResultSoftfail):
		return AuthResultSoftfail
	case string(AuthResultNeutral):
		return AuthResultNeutral
	case string(AuthResultPolicy):
		return AuthResultPolicy
	case string(AuthResultTempError):
		return AuthResultTempError
	case string(AuthResultPermError):
		return AuthResultPermError
	default:
		return AuthResultUnknown
	}
}

// PolicyOverrideReason explains why the applied disposition deviates from
// the published policy (element "reason" in policy_evaluated).
type PolicyOverrideReason struct {
	Type    string
	Comment string
}

// PolicyEvaluation is the result of the policy evaluation performed by the
// reporting recipient: applied disposition, aligned DKIM/SPF results, and
// any reasons for deviations.
type PolicyEvaluation struct {
	Disposition Disposition
	DKIM        AuthResultValue
	SPF         AuthResultValue
	Reasons     []PolicyOverrideReason
}

// PassesDMARC reports whether this record passes DMARC per RFC 7489: DKIM
// or SPF passes (after alignment) — DKIM and SPF here are already the
// aligned results evaluated by the reporting recipient from
// policy_evaluated, not raw auth results. The basis of the DMARC pass
// rate in IMPLEMENTIERUNG.md section 10.2.
func (e PolicyEvaluation) PassesDMARC() bool {
	return e.DKIM == AuthResultPass || e.SPF == AuthResultPass
}

// Identifiers are the sender data of a record relevant to the alignment
// check. HeaderFrom is always present per RFC 7489, EnvelopeFrom and
// EnvelopeTo are optional.
type Identifiers struct {
	HeaderFrom   DomainName
	EnvelopeFrom string
	EnvelopeTo   string
}

// DKIMAuthResult is a single DKIM check result (element "dkim" in
// auth_results).
type DKIMAuthResult struct {
	Domain      string
	Selector    string
	Result      AuthResultValue
	HumanResult string
}

// SPFAuthResult is a single SPF check result (element "spf" in
// auth_results).
type SPFAuthResult struct {
	Domain string
	Scope  string
	Result AuthResultValue
}

// AuthResults bundles all individual results of a record — a record can
// contain multiple DKIM signatures and SPF checks.
type AuthResults struct {
	DKIM []DKIMAuthResult
	SPF  []SPFAuthResult
}
