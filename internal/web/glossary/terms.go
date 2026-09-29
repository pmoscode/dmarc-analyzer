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
		Name:       "Einordnung (Sendequelle)",
		Definition: "Zeigt, ob DKIM und SPF getrennt betrachtet werden sollten: „Autorisiert“ bedeutet, beide bestehen. „Autorisiert (vermutlich Weiterleitung)“ bedeutet, nur DKIM besteht — DMARC besteht trotzdem, aber das Muster ist typisch für Mail-Weiterleitung, weil die DKIM-Signatur eine Weiterleitung übersteht, SPF dabei aber fast immer bricht (die weiterleitende IP steht nicht im SPF-Record der Domain). „Nicht bestätigt — prüfen“ bedeutet, DKIM besteht überwiegend nicht — eine echte Fälschung könnte DKIM nicht bestehen, weil ihr dafür der private Schlüssel der Domain fehlt, das lohnt also einen genaueren Blick.",
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
	{
		Name:       "DKIM-Selector",
		Definition: "Kennzeichnet, welcher öffentliche DKIM-Schlüssel zur Prüfung der Signatur verwendet wurde (veröffentlicht als DNS-TXT-Eintrag unter <selector>._domainkey.<domain>) — eine Domain kann mehrere Selektoren gleichzeitig nutzen, z. B. je Versanddienst.",
	},
	{
		Name:       "SPF-Scope",
		Definition: "Womit die geprüfte SPF-Domain verglichen wurde: „mfrom“ (Envelope-From, der Regelfall) oder „helo“ (der im SMTP-HELO/EHLO angegebene Name).",
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
