// Package glossary stellt Begriffserklärungen rund um DMARC bereit —
// dieselben Daten wie zuvor internal/ui/glossary/terms.go (MIGRATIONSPLAN.md
// Meilenstein M2: "Glossarseite und Begriffs-Tooltips").
package glossary

// Term ist ein Glossareintrag: Begriff und Erklärung.
//
// Inhalt unverändert aus internal/ui/glossary/terms.go übernommen (bis
// M5 dort weiterhin dupliziert, siehe MIGRATIONSPLAN.md M1-Notiz zu
// "i18n und Glossar-Daten verschoben": ein echter Verschub jetzt würde
// internal/ui — bis M3 Standardoberfläche — die Pakete unter den Füßen
// wegziehen bzw. eine verbotene ui→web-Abhängigkeit erzwingen).
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
	{
		Name:       "Nachrichtenvolumen pro Tag",
		Definition: "Ein Balken je Tag, gestapelt aus bestandenen (grün) und fehlgeschlagenen (rot) Nachrichten nach DMARC. Zeigt, ob es an einzelnen Tagen Ausreißer beim Volumen oder bei Fehlschlägen gab.",
	},
	{
		Name:       "Top-Sendequellen",
		Definition: "Die Sendequellen (IP-Adressen) mit dem größten Nachrichtenvolumen im gewählten Zeitraum. Die Balkenhöhe zeigt die Nachrichtenanzahl, die Farbe die Pass-Rate dieser Quelle (rot = 0 %, grün = 100 %). Eine Quelle kann alle anderen im Volumen deutlich überragen — das ist normal, wenn ein großer Mailanbieter den Großteil der Nachrichten sendet.",
	},
	{
		Name:       "Verteilung nach Disposition",
		Definition: "Donut-Diagramm der von den Empfängern tatsächlich angewendeten Maßnahme (siehe Begriff „Disposition“): Keine Maßnahme, Quarantäne, Zurückgewiesen oder Unbekannt.",
	},
	{
		Name:       "Sendequelle × Tag (Pass-Rate)",
		Definition: "Jede Zeile ist eine Sendequelle, jede Spalte ein Tag im gewählten Zeitraum. Die Farbe einer Zelle zeigt die Pass-Rate dieser Quelle an diesem Tag (rot = niedrig, grün = hoch). Eine leere Zelle bedeutet: An diesem Tag hat diese Quelle keine Nachrichten gesendet.",
	},
}

// ByName sucht einen Begriff nach Name. ok ist false, wenn der Begriff
// nicht im Glossar existiert (Programmierfehler beim Aufrufer).
func ByName(name string) (Term, bool) {
	for _, t := range Terms {
		if t.Name == name {
			return t, true
		}
	}
	return Term{}, false
}
