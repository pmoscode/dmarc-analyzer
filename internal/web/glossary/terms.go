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
		Definition: "Womit die geprüfte SPF-Domain verglichen wurde: „mfrom“ (Envelope-From — der Regelfall, und laut RFC 7489 auch der Standardwert, wenn ein Bericht das Feld gar nicht mitliefert) oder „helo“ (der im SMTP-HELO/EHLO angegebene Name). Ein leeres Scope-Feld bei einem älteren, bereits importierten Bericht bedeutet ebenfalls „mfrom“ — diese Berichte wurden importiert, bevor der fehlende Wert hier automatisch ergänzt wurde.",
	},
	{
		Name:       "DKIM-Ergebnis",
		Definition: "Das Ergebnis dieser einzelnen Signaturprüfung — nicht zu verwechseln mit der DKIM-Spalte in der Tabelle, die zusätzlich das Alignment berücksichtigt (siehe Begriff „Alignment“): eine Signatur kann hier „pass“ sein und die Tabellenspalte trotzdem „fail“ zeigen, wenn die signierende Domain nicht zum Header-From passt. Mögliche Werte: „pass“ (Signatur gültig), „fail“ (Signatur vorhanden, aber ungültig — z. B. weil die Nachricht nach dem Signieren verändert wurde), „none“ (keine Signatur vorhanden), „policy“ (Signatur gültig, verstößt aber gegen eine zusätzliche Anforderung des Empfängers), „neutral“ (Signatur vorhanden, Prüfung nicht eindeutig), „temperror“ (vorübergehender Fehler bei der Prüfung, z. B. ein DNS-Timeout beim Abruf des öffentlichen Schlüssels) oder „permerror“ (dauerhafter Fehler, z. B. ein fehlerhaft formatierter Schlüssel).",
	},
	{
		Name:       "SPF-Ergebnis",
		Definition: "Das Ergebnis dieser einzelnen SPF-Prüfung — nicht zu verwechseln mit der SPF-Spalte in der Tabelle, die zusätzlich das Alignment berücksichtigt (siehe Begriff „Alignment“). Mögliche Werte: „pass“ (Server autorisiert), „fail“ (Server laut SPF-Record ausdrücklich NICHT autorisiert, Eintrag „-all“), „softfail“ (Server wahrscheinlich nicht autorisiert, die Domain wollte aber keine harte Ablehnung — Eintrag „~all“, oft eine bewusste Übergangseinstellung während der SPF-Record noch verfeinert wird), „neutral“ (Domaininhaber macht bewusst keine Aussage, Eintrag „?all“), „none“ (kein SPF-Record gefunden), „temperror“ (vorübergehender Fehler, z. B. ein DNS-Timeout) oder „permerror“ (dauerhafter Fehler, z. B. ein fehlerhaft formatierter SPF-Record).",
	},
	{
		Name:       "Header-From",
		Definition: "Die Domain der im E-Mail-Programm sichtbar angezeigten Absenderadresse (Feld „From:“ im Nachrichtenkopf — hier wird nur der Domain-Teil erfasst, nicht die volle Adresse) — das sieht der Empfänger tatsächlich. DMARC prüft, ob SPF und/oder DKIM mit GENAU DIESER Domain übereinstimmen (siehe Begriff „Alignment“), nicht mit der technischen Absenderdomain (Envelope-From). Beispiel: Eine Nachricht, die im Postfach als „von newsletter@ihre-domain.de“ erscheint, hat hier ihre-domain.de stehen.",
	},
	{
		Name:       "Envelope-From",
		Definition: "Die Domain des technischen Absenders aus dem SMTP-Envelope (Kommando „MAIL FROM“ — hier wird nur der Domain-Teil erfasst, nicht die volle Adresse) — legt fest, wohin Unzustellbarkeitsmeldungen (Bounces) gehen, und ist die Domain, gegen die SPF geprüft wird. Beim direkten Versand über den eigenen Mailserver ist sie meist identisch mit der Domain aus Header-From — genau das sehen Sie hier in der Regel. Sie kann aber abweichen, wenn ein externer Versanddienst (Newsletter-Tool, Mailingliste) im Spiel ist. Positiv-Beispiel: bounce.ihre-domain.de bei sichtbarem Absender newsletter@ihre-domain.de — beide gehören zur selben Domain, SPF-Alignment besteht. Negativ-Beispiel: fremder-dienst.example.com bei einer Nachricht, die als „von ihre-domain.de“ angezeigt wird — die Domains weichen ab, SPF-Alignment besteht nicht, selbst wenn die SPF-Prüfung selbst (Autorisierung des sendenden Servers) besteht.",
	},
	{
		Name:       "Envelope-To",
		Definition: "Die Domain des technischen Empfängers aus dem SMTP-Envelope (Kommando „RCPT TO“ — hier wird nur der Domain-Teil erfasst, nicht die volle Adresse). Bei ausgehender Mail an viele verschiedene Empfänger (z. B. eine Rundmail an Eltern) sieht man hier entsprechend viele unterschiedliche Empfänger-Domains (z. B. gmail.com, web.de, outlook.com) — nicht die eigene Domain. Für die DMARC-Bewertung selbst ohne Bedeutung, wird vom Bericht nur zur Nachvollziehbarkeit mitgeliefert.",
	},
	{
		Name:       "Override-Grund",
		Definition: "Ein vom Empfänger dokumentierter Grund, warum die tatsächlich angewendete Maßnahme (Disposition) von der veröffentlichten Richtlinie abweicht — z. B. weil eine Nachricht trotz p=reject dennoch zugestellt wurde. Häufige Werte: „forwarded“ (der Empfänger vermutet eine Weiterleitung), „sampled_out“ (die Nachricht fiel durch den pct-Anteil heraus, siehe Begriff „pct“, und wurde deshalb nicht nach Richtlinie behandelt), „trusted_forwarder“ (bekannter, vertrauenswürdiger Weiterleitungsdienst), „mailing_list“ (Mailingliste), „local_policy“ (eine eigene Regel des Empfängers) oder „other“ (sonstiger Grund). Der optionale Kommentar dahinter ist reiner Freitext des Empfängers und oft technisches Kürzel statt Prosa — z. B. bedeutet „arc=fail“ bei „local_policy“, dass der Empfänger auch die ARC-Signatur (Authenticated Received Chain, eine Art Weiterleitungsnachweis) geprüft hat und diese ebenfalls nicht bestand.",
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
