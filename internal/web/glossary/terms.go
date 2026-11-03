// Package glossary provides term explanations around DMARC — the same
// data as formerly internal/ui/glossary/terms.go (MIGRATIONSPLAN.md
// milestone M2: "glossary page and term tooltips").
package glossary

// Term is a glossary entry: term and explanation.
//
// Definitions are written for readers without an e-mail background: the
// first sentence says in plain words what the term means, technical
// details (RFC field names, DNS record syntax) come last if at all.
// Definitions may contain "\n" — the tooltip (app.css, .glossary-hint)
// and the dialog side panel (.record-detail-glossary-text) both render
// with white-space: pre-line, so value lists can go one per line.
type Term struct {
	Name       string
	Definition string
}

// Terms are all glossary entries, in display order.
var Terms = []Term{
	{
		Name:       "DMARC",
		Definition: "Ein Schutzverfahren gegen gefälschte Absender. Damit legen Sie als Inhaber einer Domain fest, was Empfänger mit E-Mails tun sollen, die angeblich von Ihrer Domain kommen, aber die Echtheitsprüfungen SPF und DKIM nicht bestehen. Außerdem schicken Ihnen die Empfänger regelmäßig Berichte darüber, wer in Ihrem Namen E-Mails verschickt — genau diese Berichte wertet dieses Programm aus.",
	},
	{
		Name:       "SPF",
		Definition: "Eine öffentliche Liste der Mailserver, die E-Mails für Ihre Domain verschicken dürfen — vergleichbar mit einer Gästeliste am Eingang. Der Empfänger prüft: Steht der Server, der die E-Mail gerade abgeliefert hat, auf dieser Liste?\n\nTechnisch: Die Liste steht als TXT-Eintrag im DNS Ihrer Domain und gilt für den technischen Absender (Envelope-From).\n\nDie SPF-Spalte in den Tabellen zeigt, ob SPF bestanden wurde und dabei zur sichtbaren Absenderadresse passt (siehe „Alignment“).",
	},
	{
		Name:       "DKIM",
		Definition: "Eine digitale Unterschrift, die der versendende Mailserver an jede E-Mail anhängt — vergleichbar mit einem Siegel auf einem Brief. Der Empfänger prüft die Unterschrift mit einem öffentlichen Schlüssel aus dem DNS Ihrer Domain. Passt sie, stammt die E-Mail von einem berechtigten Absender und wurde unterwegs nicht verändert.\n\nDie DKIM-Spalte in den Tabellen zeigt, ob DKIM bestanden wurde und dabei zur sichtbaren Absenderadresse passt (siehe „Alignment“).",
	},
	{
		Name:       "Alignment",
		Definition: "Auf Deutsch etwa „Übereinstimmung“. Eine bestandene SPF- oder DKIM-Prüfung zählt für DMARC nur, wenn die dabei geprüfte Domain zur sichtbaren Absenderadresse passt (siehe „Header-From“). Sonst könnte ein Fälscher eine eigene, korrekt eingerichtete Domain prüfen lassen und trotzdem Ihre Adresse als Absender anzeigen.\n\nDie Alignment-Raten in der Übersicht zeigen den Anteil der E-Mails, bei denen die Prüfung samt Übereinstimmung bestanden wurde.\n\nIm Bericht steht dazu der Modus:\n• relaxed (locker, Standard): Subdomains zählen mit — mail.ihre-domain.de passt zu ihre-domain.de\n• strict (streng): Die Domains müssen exakt gleich sein",
	},
	{
		Name:       "DMARC-Pass-Rate",
		Definition: "Der Anteil Ihrer E-Mails, die die DMARC-Prüfung bestanden haben, also als echt erkannt wurden. Dafür reicht es, wenn DKIM oder SPF bestanden ist — jeweils passend zur sichtbaren Absenderadresse (siehe „Alignment“).\n\nIdeal ist ein Wert nahe 100 %. Ein niedriger Wert heißt: Entweder verschickt jemand gefälschte E-Mails in Ihrem Namen, oder ein eigener Versanddienst (z. B. ein Newsletter-Tool) ist noch nicht richtig eingerichtet.",
	},
	{
		Name:       "Disposition",
		Definition: "Was der Empfänger mit der E-Mail tatsächlich gemacht hat:\n• none (keine Maßnahme): normal zugestellt\n• quarantine (Quarantäne): in den Spam-Ordner verschoben\n• reject (zurückgewiesen): gar nicht angenommen\n\nDas kann von Ihrer veröffentlichten Richtlinie abweichen — am Ende entscheidet der Empfänger selbst.",
	},
	{
		Name:       "Policy (p=)",
		Definition: "Ihre Anweisung an alle Empfänger, was mit E-Mails passieren soll, die die DMARC-Prüfung nicht bestehen. Sie steht öffentlich im DNS Ihrer Domain:\n• none: nur beobachten, nichts unternehmen (gut für den Einstieg)\n• quarantine: in den Spam-Ordner verschieben\n• reject: abweisen (stärkster Schutz)\n\n„sp“ ist dieselbe Anweisung für Subdomains (z. B. shop.ihre-domain.de).",
	},
	{
		Name:       "pct",
		Definition: "Für wie viel Prozent der durchgefallenen E-Mails Ihre Richtlinie tatsächlich gelten soll. 100 % (der Standard) heißt: für alle. Ein kleinerer Wert erlaubt eine vorsichtige Umstellung — z. B. erst 10 % der verdächtigen E-Mails abweisen und dann schrittweise mehr.",
	},
	{
		Name:       "Aggregate Report (RUA)",
		Definition: "Eine meist tägliche Zusammenfassung, die E-Mail-Anbieter (z. B. Google, Microsoft, GMX) an Sie schicken: Wie viele E-Mails sie von welchem Server in Ihrem Namen erhalten haben und ob diese die Prüfungen bestanden haben. Dieses Programm holt die Berichte aus dem Postfach und wertet sie aus. Einzelne E-Mails oder deren Inhalte stehen nicht darin.",
	},
	{
		Name:       "Quell-IP",
		Definition: "Die Internetadresse (IP-Adresse) des Servers, der die E-Mails tatsächlich verschickt hat. Jede Quell-IP zählt als eigene Sendequelle.\n\nTauchen Server auf, die Sie nicht kennen, lohnt ein Blick in die Ansicht „Sendequellen“.",
	},
	{
		Name:       "rDNS / PTR",
		Definition: "Der Name, der zu einer IP-Adresse hinterlegt ist — sozusagen das Namensschild des Servers. Endet er z. B. auf google.com oder outlook.com, wissen Sie, welcher Anbieter hinter der Sendequelle steckt. Nicht jede IP-Adresse hat einen solchen Namen.",
	},
	{
		Name:       "Einordnung (Sendequelle)",
		Definition: "Eine Einschätzung, ob dieser Server berechtigt in Ihrem Namen sendet:\n• Autorisiert (DKIM & SPF): Beide Prüfungen bestehen — alles in Ordnung.\n• Autorisiert (vermutlich Weiterleitung): Nur DKIM besteht, DMARC ist trotzdem bestanden. Typisch für weitergeleitete E-Mails: Die digitale Unterschrift (DKIM) übersteht das Weiterleiten, die Serverliste (SPF) passt danach aber nicht mehr. Meist harmlos.\n• Teilweise bestätigt — prüfen: DKIM besteht nur manchmal. Ein Blick lohnt sich.\n• Nicht bestätigt — prüfen: DKIM besteht überwiegend nicht. Ein Fälscher kann keine gültige DKIM-Unterschrift erzeugen, weil ihm der geheime Schlüssel fehlt. Ist das ein eigener Dienst, der noch nicht richtig eingerichtet ist — oder ein Fälschungsversuch?",
	},
	{
		Name:       "Nachrichtenvolumen pro Tag",
		Definition: "Wie viele E-Mails pro Tag in Ihrem Namen verschickt wurden — grün die, die DMARC bestanden haben, rot die durchgefallenen. So sehen Sie auf einen Blick, ob es an einzelnen Tagen ungewöhnlich viele E-Mails oder auffällig viele Fehlschläge gab.",
	},
	{
		Name:       "Top-Sendequellen",
		Definition: "Die Server (IP-Adressen), die im gewählten Zeitraum die meisten E-Mails in Ihrem Namen verschickt haben. Die Höhe eines Balkens zeigt die Anzahl der E-Mails, seine Farbe den Anteil bestandener Prüfungen (rot = 0 %, grün = 100 %).\n\nDass ein Server alle anderen weit überragt, ist normal — meist ist das Ihr eigener Mailanbieter.",
	},
	{
		Name:       "Verteilung nach Disposition",
		Definition: "Was die Empfänger mit Ihren E-Mails gemacht haben (siehe „Disposition“): normal zugestellt (Keine Maßnahme, grün), in den Spam-Ordner verschoben (Quarantäne, gelb), abgewiesen (Zurückgewiesen, rot) oder nicht angegeben (Unbekannt, grau). Je größer der grüne Anteil, desto besser.",
	},
	{
		Name:       "Sendequelle × Tag (Pass-Rate)",
		Definition: "Jede Zeile ist ein Server (Sendequelle), jede Spalte ein Tag. Die Farbe eines Kästchens zeigt, welcher Anteil der E-Mails dieses Servers an diesem Tag die DMARC-Prüfung bestanden hat (rot = wenige, grün = alle). Ein leeres Kästchen heißt: An diesem Tag hat der Server nichts verschickt.\n\nSo erkennen Sie, ab welchem Tag ein Server Probleme hatte.",
	},
	{
		Name:       "DKIM-Selector",
		Definition: "Ein Name, der angibt, mit welchem Schlüssel die DKIM-Unterschrift erstellt wurde. Eine Domain kann mehrere Schlüssel gleichzeitig haben, oft einen pro Versanddienst (z. B. einen für den eigenen Mailserver und einen für das Newsletter-Tool). Am Selector erkennen Sie also, welcher Dienst die E-Mail unterschrieben hat.\n\nTechnisch: Der Schlüssel steht im DNS unter <selector>._domainkey.<domain>.",
	},
	{
		Name:       "SPF-Scope",
		Definition: "Welche Absenderangabe für die SPF-Prüfung verwendet wurde:\n• mfrom: der technische Absender (siehe „Envelope-From“) — der Normalfall\n• helo: der Name, mit dem sich der sendende Server beim Empfänger vorgestellt hat\n\nFehlt der Wert (bei älteren importierten Berichten möglich), ist ebenfalls „mfrom“ gemeint.",
	},
	{
		Name:       "DKIM-Ergebnis",
		Definition: "Das Ergebnis der Prüfung genau dieser einen Unterschrift:\n• pass: Unterschrift gültig\n• fail: Unterschrift ungültig, z. B. weil die E-Mail unterwegs verändert wurde\n• none: keine Unterschrift vorhanden\n• neutral: Unterschrift vorhanden, Prüfung aber nicht eindeutig\n• policy: Unterschrift gültig, erfüllt aber eine Zusatzanforderung des Empfängers nicht\n• temperror: vorübergehender Fehler, z. B. Schlüssel kurzzeitig nicht abrufbar\n• permerror: dauerhafter Fehler, z. B. fehlerhaft eingetragener Schlüssel\n\nAchtung: Hier kann „pass“ stehen, während die DKIM-Spalte der Tabelle „fail“ zeigt. Die Tabelle prüft zusätzlich, ob die unterschreibende Domain zur sichtbaren Absenderadresse passt (siehe „Alignment“).",
	},
	{
		Name:       "SPF-Ergebnis",
		Definition: "Das Ergebnis dieser einen SPF-Prüfung:\n• pass: Server darf für die Domain senden\n• fail: Server darf ausdrücklich nicht senden (Eintrag „-all“)\n• softfail: Server darf vermutlich nicht senden, die Domain verlangt aber keine harte Ablehnung (Eintrag „~all“, oft eine Übergangslösung)\n• neutral: Die Domain macht bewusst keine Aussage (Eintrag „?all“)\n• none: Die Domain hat keine SPF-Liste\n• temperror: vorübergehender Fehler, z. B. DNS nicht erreichbar\n• permerror: dauerhafter Fehler, z. B. fehlerhaft eingetragene SPF-Liste\n\nAchtung: Die SPF-Spalte der Tabelle prüft zusätzlich, ob die Domain zur sichtbaren Absenderadresse passt (siehe „Alignment“), und kann deshalb abweichen.",
	},
	{
		Name:       "Header-From",
		Definition: "Die Domain der Absenderadresse, die der Empfänger in seinem E-Mail-Programm sieht. Beispiel: Steht im Postfach „von newsletter@ihre-domain.de“, steht hier ihre-domain.de.\n\nGenau diese Domain schützt DMARC: SPF oder DKIM müssen zu ihr passen (siehe „Alignment“).",
	},
	{
		Name:       "Envelope-From",
		Definition: "Der technische Absender — wie der Absender auf dem Briefumschlag, während die sichtbare Absenderadresse (Header-From) eher dem Briefkopf entspricht. An diese Adresse gehen Fehlermeldungen, wenn eine E-Mail nicht zugestellt werden kann, und gegen diese Domain prüft SPF.\n\nMeist ist sie gleich der sichtbaren Absenderdomain. Bei externen Versanddiensten (Newsletter-Tool, Mailingliste) kann sie abweichen:\n• bounce.ihre-domain.de bei Absender newsletter@ihre-domain.de: dieselbe Domain — passt.\n• fremder-dienst.example.com bei Absender ihre-domain.de: passt nicht. SPF zählt dann nicht für DMARC, selbst wenn die SPF-Prüfung an sich bestanden ist.",
	},
	{
		Name:       "Envelope-To",
		Definition: "Die Domain des Empfängers (z. B. gmail.com oder web.de). Bei E-Mails an viele Empfänger stehen hier entsprechend viele verschiedene Domains. Wird im Bericht nur zur Information mitgeliefert und spielt für die DMARC-Bewertung keine Rolle.",
	},
	{
		Name:       "Override-Grund",
		Definition: "Warum der Empfänger von Ihrer Richtlinie abgewichen ist — z. B. eine E-Mail trotz „reject“ zugestellt hat:\n• forwarded: Der Empfänger vermutet eine Weiterleitung\n• sampled_out: Die E-Mail fiel durch den Prozentsatz heraus (siehe „pct“)\n• trusted_forwarder: bekannter, vertrauenswürdiger Weiterleitungsdienst\n• mailing_list: Mailingliste\n• local_policy: eigene Regel des Empfängers\n• other: sonstiger Grund\n\nDer Kommentar dahinter ist freier Text des Empfängers, oft ein technisches Kürzel. Beispiel: „arc=fail“ heißt, ein zusätzlicher Weiterleitungsnachweis (ARC) war ebenfalls ungültig.",
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
