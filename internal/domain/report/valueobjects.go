package report

import (
	"fmt"
	"net/netip"
	"strings"
	"time"
)

// SourceIP ist die IP-Adresse einer Sendequelle. Unveränderlich, ohne
// Identität — ein Value Object im Sinne von DDD.
type SourceIP struct {
	addr netip.Addr
}

// NewSourceIP parst eine IPv4- oder IPv6-Adresse.
func NewSourceIP(s string) (SourceIP, error) {
	addr, err := netip.ParseAddr(strings.TrimSpace(s))
	if err != nil {
		return SourceIP{}, fmt.Errorf("ungültige quell-ip %q: %w", s, err)
	}
	return SourceIP{addr: addr}, nil
}

// String liefert die textuelle Darstellung der Adresse.
func (ip SourceIP) String() string {
	return ip.addr.String()
}

// IsValid meldet, ob die Adresse gesetzt ist (Nullwert-Erkennung).
func (ip SourceIP) IsValid() bool {
	return ip.addr.IsValid()
}

// DomainName ist ein normalisierter Domainname (kleingeschrieben, ohne
// führende/folgende Leerzeichen). Value Object.
type DomainName struct {
	value string
}

// NewDomainName normalisiert und validiert einen Domainnamen. Ein leerer
// Domainname ist fachlich ungültig — Aufrufer mit optionalen Domainangaben
// (z. B. envelope_from) verzichten bewusst auf diesen Typ und nutzen einen
// rohen String.
func NewDomainName(s string) (DomainName, error) {
	normalized := strings.ToLower(strings.TrimSpace(s))
	if normalized == "" {
		return DomainName{}, fmt.Errorf("domainname darf nicht leer sein")
	}
	return DomainName{value: normalized}, nil
}

// String liefert den normalisierten Domainnamen.
func (d DomainName) String() string {
	return d.value
}

// DateRange ist ein Zeitraum mit Begin < End, beide in UTC.
type DateRange struct {
	Begin time.Time
	End   time.Time
}

// NewDateRange erzwingt Begin < End und normalisiert beide Zeitpunkte auf UTC.
func NewDateRange(begin, end time.Time) (DateRange, error) {
	if !begin.Before(end) {
		return DateRange{}, fmt.Errorf("beginn (%s) muss vor ende (%s) liegen", begin, end)
	}
	return DateRange{Begin: begin.UTC(), End: end.UTC()}, nil
}

// IsZero meldet, ob der Zeitraum unbesetzt ist.
func (r DateRange) IsZero() bool {
	return r.Begin.IsZero() && r.End.IsZero()
}

// Disposition ist die vom Empfänger tatsächlich angewendete Maßnahme
// (RFC 7489 Abschnitt 3.1.2, Feld "disposition" in policy_evaluated).
type Disposition string

// Werte für Disposition (RFC 7489 Abschnitt 3.1.2).
const (
	DispositionNone       Disposition = "none"
	DispositionQuarantine Disposition = "quarantine"
	DispositionReject     Disposition = "reject"
	// DispositionUnknown deckt RFC-abweichende Werte ab — Provider halten
	// sich nicht immer an den RFC, solche Reports sollen trotzdem
	// importiert werden können (siehe IMPLEMENTIERUNG.md Abschnitt 6.2).
	DispositionUnknown Disposition = "unknown"
)

// ParseDisposition wandelt einen rohen XML-Wert in eine Disposition um.
// Unbekannte Werte werden nie verworfen, sondern auf DispositionUnknown
// abgebildet.
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

// Policy ist eine veröffentlichte DMARC-Richtlinie (Felder "p"/"sp" in
// policy_published). Denselben Wertebereich wie Disposition, aber fachlich
// eine andere Aussage (Absicht statt angewendeter Maßnahme) — deshalb ein
// eigener Typ.
type Policy string

// Werte für Policy (RFC 7489, Felder "p"/"sp").
const (
	PolicyNone       Policy = "none"
	PolicyQuarantine Policy = "quarantine"
	PolicyReject     Policy = "reject"
	PolicyUnknown    Policy = "unknown"
)

// ParsePolicy wandelt einen rohen XML-Wert in eine Policy um. Unbekannte
// Werte werden auf PolicyUnknown abgebildet, nicht verworfen.
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

// AlignmentMode ist der Ausrichtungsmodus für DKIM oder SPF: relaxed (r)
// oder strict (s), Felder "adkim"/"aspf".
type AlignmentMode string

// Werte für AlignmentMode (RFC 7489, Felder "adkim"/"aspf").
const (
	AlignmentRelaxed AlignmentMode = "r"
	AlignmentStrict  AlignmentMode = "s"
	AlignmentUnknown AlignmentMode = "unknown"
)

// ParseAlignmentMode wandelt einen rohen XML-Wert um. Fehlt der Wert (viele
// Provider lassen adkim/aspf weg), gilt nach RFC 7489 der Default "r" —
// diesen Default setzt der Aufrufer, nicht ParseAlignmentMode.
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

// AuthResultValue ist ein Einzelergebnis einer DKIM- oder SPF-Prüfung.
// Beide RFC-Enums (DKIM: none/pass/fail/policy/neutral/temperror/
// permerror; SPF: none/neutral/pass/fail/softfail/temperror/permerror)
// werden in einem gemeinsamen Typ geführt, da sich beide stark
// überschneiden und getrennte Typen keinen fachlichen Mehrwert hätten.
type AuthResultValue string

// Werte für AuthResultValue, vereinigt aus den DKIM- und SPF-Enums von
// RFC 7489.
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

// ParseAuthResultValue wandelt einen rohen XML-Wert um. Unbekannte Werte
// werden auf AuthResultUnknown abgebildet, nicht verworfen.
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

// PolicyOverrideReason erklärt, warum die angewendete Disposition von der
// veröffentlichten Policy abweicht (Element "reason" in policy_evaluated).
type PolicyOverrideReason struct {
	Type    string
	Comment string
}

// PolicyEvaluation ist das Ergebnis der Policy-Auswertung durch den
// berichtenden Empfänger: angewendete Disposition, ausgerichtete
// (aligned) DKIM-/SPF-Ergebnisse und ggf. Gründe für Abweichungen.
type PolicyEvaluation struct {
	Disposition Disposition
	DKIM        AuthResultValue
	SPF         AuthResultValue
	Reasons     []PolicyOverrideReason
}

// PassesDMARC meldet, ob dieser Record DMARC nach RFC 7489 besteht: DKIM
// oder SPF bestehen (nach Alignment) — DKIM und SPF hier sind bereits die
// vom berichtenden Empfänger ausgewerteten (aligned) Ergebnisse aus
// policy_evaluated, keine rohen Auth-Results. Grundlage der DMARC-Pass-Rate
// in IMPLEMENTIERUNG.md Abschnitt 10.2.
func (e PolicyEvaluation) PassesDMARC() bool {
	return e.DKIM == AuthResultPass || e.SPF == AuthResultPass
}

// Identifiers sind die für die Alignment-Prüfung relevanten Absenderdaten
// eines Records. HeaderFrom ist nach RFC 7489 immer vorhanden, EnvelopeFrom
// und EnvelopeTo sind optional.
type Identifiers struct {
	HeaderFrom   DomainName
	EnvelopeFrom string
	EnvelopeTo   string
}

// DKIMAuthResult ist ein einzelnes DKIM-Prüfergebnis (Element "dkim" in
// auth_results).
type DKIMAuthResult struct {
	Domain      string
	Selector    string
	Result      AuthResultValue
	HumanResult string
}

// SPFAuthResult ist ein einzelnes SPF-Prüfergebnis (Element "spf" in
// auth_results).
type SPFAuthResult struct {
	Domain string
	Scope  string
	Result AuthResultValue
}

// AuthResults bündelt alle Einzelergebnisse eines Records — ein Record
// kann mehrere DKIM-Signaturen und SPF-Prüfungen enthalten.
type AuthResults struct {
	DKIM []DKIMAuthResult
	SPF  []SPFAuthResult
}
