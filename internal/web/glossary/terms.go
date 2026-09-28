// Package glossary provides term explanations around DMARC — the same
// data as formerly internal/ui/glossary/terms.go (MIGRATIONSPLAN.md
// milestone M2: "glossary page and term tooltips").
package glossary

// Term is a glossary entry: term and explanation.
//
// Content taken over unchanged from internal/ui/glossary/terms.go (still
// duplicated there until M5, see the MIGRATIONSPLAN.md M1 note on
// "i18n and glossary data moved": actually moving it now would pull the
// rug out from under internal/ui — the default UI until M3 — or force a
// forbidden ui→web dependency).
type Term struct {
	Name       string
	Definition string
}

// Terms are all glossary entries, in display order.
var Terms = []Term{
	{
		Name:       "DMARC",
		Definition: "Domain-based Message Authentication, Reporting & Conformance (RFC 7489). Defines how a recipient handles messages that fail SPF and DKIM, and provides the domain owner with reports about it.",
	},
	{
		Name:       "SPF",
		Definition: "Sender Policy Framework — checks whether the sending mail server is authorized for the domain in the envelope-from (via a DNS TXT record).",
	},
	{
		Name:       "DKIM",
		Definition: "DomainKeys Identified Mail — cryptographically signs messages; the recipient checks the signature against a public key published in DNS.",
	},
	{
		Name:       "Alignment",
		Definition: "Whether the domain checked by SPF/DKIM matches the visible sender (header-from) — without alignment, a passing SPF/DKIM doesn't count for DMARC.",
	},
	{
		Name:       "DMARC pass rate",
		Definition: "The share of messages where DKIM or SPF pass after alignment (dkim=pass OR spf=pass in policy_evaluated).",
	},
	{
		Name:       "Disposition",
		Definition: "The action the recipient actually applied: none, quarantine (spam folder), or reject.",
	},
	{
		Name:       "Policy (p=)",
		Definition: "The desired policy published by the domain owner in DNS — the same values as disposition, but intent rather than the action actually applied.",
	},
	{
		Name:       "pct",
		Definition: "The percentage of messages the policy should be applied to — allows a gradual rollout from p=none to p=reject.",
	},
	{
		Name:       "Aggregate Report (RUA)",
		Definition: "The summarized, typically daily DMARC report that this program fetches and evaluates — contains volume and results per sending source, not individual messages.",
	},
	{
		Name:       "Source IP",
		Definition: "The IP address of the mail server that actually sent the message — the basis of the sending-source view.",
	},
	{
		Name:       "rDNS / PTR",
		Definition: "Reverse DNS lookup of an IP address to a hostname — helps attribute a source IP to a known service provider.",
	},
	{
		Name:       "Classification (sending source)",
		Definition: "Shows whether DKIM and SPF should be considered separately: \"Authorized\" means both pass. \"Authorized (likely forwarding)\" means only DKIM passes — DMARC still passes, but the pattern is typical of mail forwarding, because the DKIM signature survives a forward while SPF almost always breaks (the forwarding IP isn't in the domain's SPF record). \"Unconfirmed — review\" means DKIM predominantly fails — a genuine spoof couldn't pass DKIM, since it lacks the domain's private key, so this is worth a closer look.",
	},
	{
		Name:       "Message volume per day",
		Definition: "One bar per day, stacked from passed (green) and failed (red) messages per DMARC. Shows whether individual days had outliers in volume or failures.",
	},
	{
		Name:       "Top sending sources",
		Definition: "The sending sources (IP addresses) with the highest message volume in the selected period. Bar height shows the message count, color shows that source's pass rate (red = 0%, green = 100%). One source can dwarf all others in volume — that's normal when a large mail provider sends most of the messages.",
	},
	{
		Name:       "Disposition breakdown",
		Definition: "Donut chart of the action recipients actually applied (see the \"Disposition\" term): none, quarantine, reject, or unknown.",
	},
	{
		Name:       "Sending source × day (pass rate)",
		Definition: "Each row is a sending source, each column a day in the selected period. A cell's color shows that source's pass rate on that day (red = low, green = high). An empty cell means that source sent no messages that day.",
	},
	{
		Name:       "DKIM selector",
		Definition: "Identifies which DKIM public key was used to verify the signature (published as a DNS TXT record under <selector>._domainkey.<domain>) — a domain can use several selectors at once, e.g. one per sending service.",
	},
	{
		Name:       "SPF scope",
		Definition: "What the checked SPF domain was compared against: \"mfrom\" (envelope-from, the usual case) or \"helo\" (the name given in the SMTP HELO/EHLO).",
	},
}

// ByName looks up a term by name. ok is false if the term doesn't exist
// in the glossary (a caller bug).
func ByName(name string) (Term, bool) {
	for _, t := range Terms {
		if t.Name == name {
			return t, true
		}
	}
	return Term{}, false
}
