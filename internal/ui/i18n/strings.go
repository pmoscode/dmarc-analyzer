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
	NavDashboard = "Übersicht"
	NavReports   = "Berichte"
	NavSources   = "Sendequellen"
	NavSettings  = "Einstellungen"
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

	ReportsGroupLabel  = "Gruppieren nach"
	ReportsGroupNone   = "Keine"
	ReportsGroupDomain = "Domain"
	ReportsGroupOrg    = "Organisation"

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

// Filter-/Gruppierungsleiste — wirkt laut UMSETZUNGSPLAN.md AP-6-Checkliste
// auf Übersicht, Berichte und Sendequellen gemeinsam.
const (
	FilterPeriodLabel  = "Zeitraum"
	FilterPeriod7Days  = "Letzte 7 Tage"
	FilterPeriod30Days = "Letzte 30 Tage"
	FilterPeriod90Days = "Letzte 90 Tage"
	FilterPeriod1Year  = "Letztes Jahr"
	FilterDomainLabel  = "Domain"
	FilterDomainHint   = "Alle Domains"
	FilterApply        = "Filter anwenden"
)

// Übersicht (Dashboard) — Kennzahlen-Kacheln und Diagramme
// (IMPLEMENTIERUNG.md Abschnitt 10.2/10.3).
const (
	DashboardTitle               = "Übersicht"
	DashboardTileTotalMessages   = "Nachrichten gesamt"
	DashboardTilePassRate        = "DMARC-Pass-Rate"
	DashboardTileDKIMAlignment   = "DKIM-Alignment-Rate"
	DashboardTileSPFAlignment    = "SPF-Alignment-Rate"
	DashboardTileDistinctSources = "Unterschiedliche Quellen"
	DashboardTrendUp             = "▲"
	DashboardTrendDown           = "▼"
	DashboardTrendFlat           = "→"
	DashboardTrendNoData         = "kein Vergleich zur Vorperiode möglich"
	DashboardTrendFmt            = "%s %+.1f Prozentpunkte ggü. Vorperiode"

	DashboardChartDailyVolume = "Nachrichtenvolumen pro Tag"
	DashboardChartTopSources  = "Top-Sendequellen"
	DashboardChartDisposition = "Verteilung nach Disposition"
	DashboardChartHeatmap     = "Sendequelle × Tag (Pass-Rate)"

	DashboardEmptyTitle  = "Noch keine Daten für diesen Zeitraum"
	DashboardEmptyDetail = "Nach dem ersten Abgleich erscheinen hier Kennzahlen und Diagramme."

	DashboardExportChartPNG = "Diagramm als PNG exportieren"
)

// Sendequellen — aggregiert nach Quell-IP (IMPLEMENTIERUNG.md Abschnitt
// 10.1).
const (
	SourcesTitle          = "Sendequellen"
	SourcesColumnIP       = "Quell-IP"
	SourcesColumnVolume   = "Volumen"
	SourcesColumnPassRate = "Pass-Rate"
	SourcesColumnHostname = "Hostname (PTR)"
	SourcesColumnService  = "Erkannter Dienst"
	SourcesUnknownValue   = "—"
	SourcesLoadMore       = "Weitere laden"
	SourcesEmptyTitle     = "Noch keine Sendequellen"
	SourcesEmptyDetail    = "Nach dem ersten Abgleich erscheinen hier die erkannten Sendequellen."
)

// Export gefilterter Ansichten (FEATURES.md Vorschlag 11.4).
const (
	ExportCSV          = "Als CSV exportieren"
	ExportSuccessTitle = "Export erfolgreich"
	ExportSuccessFmt   = "Gespeichert unter %s"
	ExportFailedTitle  = "Export fehlgeschlagen"
)

// Glossar — Begriffe rund um DMARC (UMSETZUNGSPLAN.md AP-6-Checkliste:
// "Glossar und Tooltips für DMARC-Begriffe"). Fyne v2.8 hat keine
// eingebauten Hover-Tooltips (geprüft) — ein "?"-Knopf neben Kennzahlen
// öffnet stattdessen denselben Text als Dialog, siehe
// internal/ui/glossary.
const (
	GlossaryTitle       = "Glossar"
	GlossaryButtonLabel = "?"
)
