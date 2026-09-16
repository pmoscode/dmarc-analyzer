package dmarcxml

import "encoding/xml"

// Die folgenden Structs bilden das XML-Schema aus RFC 7489 Anhang C ab.
// Bewusst string-basiert und ohne eigene Validierung: die Umwandlung in
// Domänentypen (inkl. Unknown-Fallback für RFC-abweichende Enum-Werte)
// passiert erst beim Mapping in parser.go. So bleibt das Schema tolerant
// gegenüber Providern, die vom RFC abweichen — ein strenger Typ (z. B. int
// für "pct") würde den ganzen Report an einer einzigen fehlerhaften Stelle
// scheitern lassen, noch bevor wir selbst entscheiden konnten, wie tolerant
// wir sein wollen.
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

// dateRange nutzt int64 statt time.Time: das XML liefert Unix-Sekunden als
// Zahl, keine ISO-Zeitangabe.
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
	// Percentage bleibt String: "pct" ist laut RFC optional (Default 100),
	// das unterscheiden wir erst beim Mapping zwischen "fehlt" und "0".
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
