// Package i18n bündelt zentral alle UI-Texte (deutsch, siehe
// IMPLEMENTIERUNG.md Abschnitt 1.3). Kein Text im Widget-Code — jede
// Beschriftung, jeder Fehlertext, jeder Tooltip kommt von hier
// (UMSETZUNGSPLAN.md AP 5-Checkliste).
package i18n

// Allgemein.
const (
	AppTitle    = "DMARC Analyzer"
	ButtonOK    = "OK"
	ButtonBack  = "Zurück"
	ButtonNext  = "Weiter"
	ButtonSave  = "Speichern"
	ButtonRetry = "Erneut versuchen"
)

// Navigation (Hauptfenster, linke Leiste).
const (
	NavReports  = "Berichte"
	NavSettings = "Einstellungen"
)

// Ersteinrichtungs-Assistent (UMSETZUNGSPLAN.md Abschnitt 3.1: geführte
// Ersteinrichtung in drei Schritten).
const (
	OnboardingTitle          = "Willkommen bei DMARC Analyzer"
	OnboardingIntro          = "Bevor es losgeht: ein E-Mail-Konto einrichten, aus dem DMARC-Berichte abgeholt werden."
	OnboardingStepAccount    = "1. Konto"
	OnboardingStepTest       = "2. Verbindung testen"
	OnboardingStepFirstSync  = "3. Erster Abgleich"
	OnboardingTestPrompt     = "Verbindung mit den eingegebenen Daten testen, bevor das Konto gespeichert wird."
	OnboardingTestButton     = "Verbindung testen"
	OnboardingTestRunning    = "Verbindung wird getestet …"
	OnboardingTestSuccess    = "Verbindung erfolgreich."
	OnboardingFirstSyncIntro = "Konto gespeichert. Jetzt die ersten Berichte abholen?"
	OnboardingFirstSyncNow   = "Jetzt abholen"
	OnboardingFirstSyncSkip  = "Später"
	OnboardingDone           = "Fertig — die Berichte sind bereit."
)

// Konto-Formular (Einstellungen und Ersteinrichtung gemeinsam).
const (
	AccountFieldDisplayName = "Anzeigename"
	AccountFieldHost        = "IMAP-Host"
	AccountFieldPort        = "Port"
	AccountFieldUsername    = "Benutzername"
	AccountFieldMailbox     = "Postfach"
	AccountFieldUseTLS      = "Verschlüsselte Verbindung (IMAPS)"
	AccountFieldPassword    = "Passwort / App-Passwort"

	AccountHelpPassword = "Bei aktivierter Zwei-Faktor-Authentifizierung wird ein App-Passwort benötigt, nicht das normale Kontopasswort."
	AccountHelpTLS      = "Nur deaktivieren, wenn der Mailserver keine verschlüsselte Verbindung anbietet — die Zugangsdaten würden sonst unverschlüsselt übertragen."

	AccountErrorHostRequired     = "IMAP-Host darf nicht leer sein."
	AccountErrorUsernameRequired = "Benutzername darf nicht leer sein."
	AccountErrorPasswordRequired = "Passwort darf nicht leer sein."
	AccountErrorPortInvalid      = "Port muss eine Zahl zwischen 1 und 65535 sein."
)

// Einstellungen — Kontoübersicht.
const (
	SettingsTitle         = "Konten"
	SettingsAddAccount    = "Konto hinzufügen"
	SettingsTestAccount   = "Verbindung testen"
	SettingsDeleteAccount = "Konto löschen"
	SettingsEmptyTitle    = "Noch kein Konto eingerichtet"
	SettingsEmptyDetail   = "Ohne Konto können keine Berichte abgeholt werden."
	SettingsEmptyAction   = "Konto hinzufügen"

	SettingsDeleteConfirmTitle = "Konto wirklich löschen?"
	SettingsDeleteConfirmBody  = "Die gespeicherten Zugangsdaten werden ebenfalls entfernt. Bereits abgeholte Berichte bleiben erhalten."
)

// Berichte — Tabelle und Detailansicht.
const (
	ReportsTitle        = "Berichte"
	ReportsColumnOrg    = "Absender-Organisation"
	ReportsColumnDomain = "Domain"
	ReportsColumnPeriod = "Zeitraum"
	ReportsColumnPolicy = "Richtlinie"
	ReportsLoadMore     = "Weitere laden"
	ReportsEmptyTitle   = "Noch keine Berichte"
	ReportsEmptyDetail  = "Nach dem ersten Abgleich erscheinen hier die eingegangenen DMARC-Berichte."
	ReportsEmptyAction  = "Jetzt abgleichen"
	ReportsEmptyNoMatch = "Keine Berichte für diese Filterung."

	ReportDetailTitle       = "Berichtsdetails"
	ReportDetailMetadata    = "Metadaten"
	ReportDetailPolicy      = "Veröffentlichte Richtlinie"
	ReportDetailRecords     = "Sendequellen"
	ReportDetailColumnIP    = "Quell-IP"
	ReportDetailColumnCount = "Nachrichten"
	ReportDetailColumnDisp  = "Disposition"
	ReportDetailColumnDKIM  = "DKIM"
	ReportDetailColumnSPF   = "SPF"
)

// Sync.
const (
	SyncButton      = "Jetzt abgleichen"
	SyncRunning     = "Abgleich läuft …"
	SyncCancel      = "Abbrechen"
	SyncCancelling  = "Wird abgebrochen …"
	SyncResultFmt   = "%d neu, %d übersprungen, %d fehlerhaft"
	SyncErrorPrefix = "Abgleich fehlgeschlagen: "
)

// Allgemeine Fehlermeldungen — Klartext statt technischer Ausnahmen
// (UMSETZUNGSPLAN.md Abschnitt 3.1: "Fehlermeldungen in Klartext").
const (
	ErrorGenericTitle  = "Etwas ist schiefgelaufen"
	ErrorLoadFailed    = "Daten konnten nicht geladen werden."
	ErrorConnectFailed = "Verbindung fehlgeschlagen — bitte Host, Port und Zugangsdaten prüfen."
)
