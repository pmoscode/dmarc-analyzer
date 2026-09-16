// Package glossary stellt Begriffserklärungen rund um DMARC bereit
// (UMSETZUNGSPLAN.md AP-6-Checkliste: "Glossar und Tooltips für
// DMARC-Begriffe").
//
// Fyne v2.8 hat keine eingebauten Hover-Tooltips (geprüft: kein
// widget.Tooltip, kein Treffer für "Tooltip" im gesamten Modul). Als
// Ersatz öffnet ein kleiner "?"-Knopf neben einzelnen Kennzahlen denselben
// Text als Dialog (siehe ShowTerm) — funktional dasselbe Ziel (Begriff
// direkt an Ort und Stelle nachschlagen), nur per Tipp statt Hover.
package glossary

// Term ist ein Glossareintrag: Begriff und Erklärung.
type Term struct {
	Name       string
	Definition string
}

// Terms sind alle Glossareinträge, in Anzeigereihenfolge.
var Terms = []Term{
	{
		Name:       "DMARC",
		Definition: "Domain-based Message Authentication, Reporting & Conformance (RFC 7489). Legt fest, wie ein Empfänger mit Nachrichten umgeht, die SPF und DKIM nicht bestehen, und liefert dem Domaininhaber Berichte darüber.",
	},
	{
		Name:       "SPF",
		Definition: "Sender Policy Framework — prüft, ob der sendende Mailserver für die Domain im Envelope-From autorisiert ist (per DNS-TXT-Eintrag).",
	},
	{
		Name:       "DKIM",
		Definition: "DomainKeys Identified Mail — signiert Nachrichten kryptografisch; der Empfänger prüft die Signatur gegen einen im DNS veröffentlichten öffentlichen Schlüssel.",
	},
	{
		Name:       "Alignment",
		Definition: "Ob die bei SPF/DKIM geprüfte Domain mit der im sichtbaren Absender (Header-From) übereinstimmt — ohne Alignment zählt ein bestandenes SPF/DKIM nicht für DMARC.",
	},
	{
		Name:       "DMARC-Pass-Rate",
		Definition: "Anteil der Nachrichten, bei denen DKIM oder SPF nach Alignment bestehen (dkim=pass ODER spf=pass in policy_evaluated).",
	},
	{
		Name:       "Disposition",
		Definition: "Die vom Empfänger tatsächlich angewendete Maßnahme: none (keine), quarantine (Quarantäne/Spam-Ordner) oder reject (Zurückweisung).",
	},
	{
		Name:       "Policy (p=)",
		Definition: "Die vom Domaininhaber im DNS veröffentlichte gewünschte Richtlinie — dieselben Werte wie Disposition, aber Absicht statt tatsächlich angewendeter Maßnahme.",
	},
	{
		Name:       "pct",
		Definition: "Prozentsatz der Nachrichten, auf die die Richtlinie angewendet werden soll — erlaubt einen schrittweisen Rollout von p=none zu p=reject.",
	},
	{
		Name:       "Aggregate Report (RUA)",
		Definition: "Der zusammengefasste, täglich versendete DMARC-Bericht, den dieses Programm abholt und auswertet — enthält Volumen und Ergebnisse je Sendequelle, keine Einzelnachrichten.",
	},
	{
		Name:       "Quell-IP",
		Definition: "Die IP-Adresse des Mailservers, der die Nachricht tatsächlich gesendet hat — Grundlage der Sendequellen-Ansicht.",
	},
	{
		Name:       "rDNS / PTR",
		Definition: "Reverse-DNS-Auflösung einer IP-Adresse zu einem Hostnamen — hilft, eine Quell-IP einem bekannten Diensteanbieter zuzuordnen.",
	},
}

// ByName sucht einen Begriff nach Name. ok ist false, wenn der Begriff
// nicht im Glossar existiert (Programmierfehler beim Aufrufer, siehe
// ShowTerm).
func ByName(name string) (Term, bool) {
	for _, t := range Terms {
		if t.Name == name {
			return t, true
		}
	}
	return Term{}, false
}
