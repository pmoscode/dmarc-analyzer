// Package sync enthält den Sync-Fortschritt (State) sowie die Ports
// MessageSource und ReportParser (IMPLEMENTIERUNG.md Abschnitt 6.4).
package sync

import (
	"context"
	"iter"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// RawAttachment ist ein unverarbeiteter Anhang aus einer Quelle: Rohbytes
// plus die Metadaten, die zur Formaterkennung nötig sind.
type RawAttachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

// MessageDecoder zerlegt eine rohe Nachricht (RawMessage.Data) in ihre
// Anhänge — der protokollunabhängige Schritt zwischen MessageSource und
// ReportParser (siehe Kommentar an RawMessage). Implementiert in AP 4
// gegen github.com/emersion/go-message (internal/infra/mailmime). Eigener
// Port statt eines direkten Imports von internal/infra/mailmime aus der
// Anwendungsschicht — sonst würde syncreports an einer konkreten
// Infra-Technologie hängen statt an einem Port (DIP, IMPLEMENTIERUNG.md
// Abschnitt 4.2).
type MessageDecoder interface {
	Decode(data []byte) ([]RawAttachment, error)
}

// ReportParser wandelt einen Rohanhang in Domänenobjekte. Über Supports()
// wird die passende Implementierung gewählt — so kommen RUF und TLS-RPT
// später additiv hinzu, ohne bestehenden Code zu ändern (Open/Closed,
// IMPLEMENTIERUNG.md Abschnitt 6.4).
type ReportParser interface {
	Supports(attachment RawAttachment) bool
	Parse(ctx context.Context, attachment RawAttachment) (*report.AggregateReport, error)
}

// MultiReportParser ist eine optionale Erweiterung von ReportParser für
// Anhänge, die mehrere Reports enthalten können — bei DMARC-Aggregate-
// Reports insbesondere ein .zip mit mehreren XML-Dateien
// (internal/infra/dmarcxml.Parser implementiert dies bereits über
// ParseAll). Aufrufer (importfiles/syncreports) prüfen per Typ-Assertion,
// ob ein Parser diese Schnittstelle zusätzlich zu ReportParser erfüllt —
// derselbe optionale-Schnittstellen-Zuschnitt wie MailboxLister weiter
// unten. Ein Parser, der nur Parse() implementiert, liefert dann eben nur
// den einen Report, den Parse() zurückgibt — kein Fehler, nur weniger
// Funktionsumfang.
type MultiReportParser interface {
	ParseAll(ctx context.Context, attachment RawAttachment) ([]*report.AggregateReport, error)
}

// ParseAttachment liefert alle in attachment enthaltenen Reports —
// bevorzugt über MultiReportParser.ParseAll, falls parser das zusätzlich
// implementiert (z. B. für .zip-Anhänge mit mehreren XML-Dateien), sonst
// über das einzelne Parse() aus ReportParser. Gemeinsam genutzt von
// importfiles.UseCase und syncreports.UseCase, damit beide dasselbe
// Verhalten haben (vor dieser Funktion importierte importfiles einen
// .zip mit mehreren Reports nur unvollständig — es wurde stets nur der
// erste enthaltene Report gespeichert, siehe Regressionstest
// TestHandleImportSubmit_ZipWithMultipleReports_ImportsBoth in
// internal/web).
func ParseAttachment(ctx context.Context, parser ReportParser, attachment RawAttachment) ([]*report.AggregateReport, error) {
	if multi, ok := parser.(MultiReportParser); ok {
		return multi.ParseAll(ctx, attachment)
	}
	rep, err := parser.Parse(ctx, attachment)
	if err != nil {
		return nil, err
	}
	return []*report.AggregateReport{rep}, nil
}

// RawMessage ist eine unverarbeitete Nachricht aus einer Quelle: die
// vollständigen Rohbytes (bei IMAP: BODY.PEEK[], bei Datei-Import
// (AP 4/7): der Dateiinhalt) plus die UID, unter der die Quelle sie führt.
// Die MIME-Zerlegung in RawAttachment-Werte passiert bewusst NICHT hier,
// sondern als eigener, protokollunabhängiger Schritt in der
// Anwendungsschicht (AP 4) — dieselbe Logik verarbeitet dann sowohl
// IMAP-Nachrichten als auch importierte .eml-Dateien.
type RawMessage struct {
	UID  uint32
	Data []byte
}

// MessageSource liefert Rohnachrichten aus einer Quelle (v1: IMAP,
// internal/infra/imap). Bewusst technikneutral, damit später Dateiimport
// oder andere Protokolle ohne Änderung der Use Cases andocken können
// (IMPLEMENTIERUNG.md Abschnitt 6.4).
//
// Abweichend vom ursprünglichen Entwurf nimmt Connect das Secret separat
// entgegen statt es aus account.MailAccount zu lesen: MailAccount enthält
// laut Sicherheitsmodell (IMPLEMENTIERUNG.md Abschnitt 9) bewusst kein
// Passwort, das liegt ausschließlich im Schlüsselbund. Der Aufrufer holt es
// über account.CredentialStore und reicht es hier explizit durch.
type MessageSource interface {
	Connect(ctx context.Context, acc account.MailAccount, secret account.Secret) error
	// FetchNew liefert einen Iterator über neue Nachrichten ab state sowie
	// einen Baseline-State, der sofort persistierbar ist (insbesondere die
	// aktuelle UIDValidity) — auch wenn der Iterator noch nicht konsumiert
	// wurde. Den tatsächlichen Fortschritt (LastUID) schreibt der Aufrufer
	// nach jeder erfolgreich verarbeiteten RawMessage selbst fort
	// (IMPLEMENTIERUNG.md Abschnitt 7.1, Schritt 6) — FetchNew liefert dafür
	// mit jeder RawMessage deren UID.
	FetchNew(ctx context.Context, state State) (iter.Seq2[RawMessage, error], State, error)
	Close() error
}

// MailboxLister ist ein optionaler Zusatz-Port für Quellen, die die auf
// dem Server tatsächlich vorhandenen Postfächer/Ordner auflisten können —
// Grundlage für einen Ordner-Picker im Kontoformular, weil DMARC-Berichte
// nicht zwangsläufig im Wurzelpostfach (INBOX) landen, sondern z. B. über
// eine Mailregel in einen Unterordner sortiert sein können. Bewusst ein
// eigener, separater Port statt einer weiteren MessageSource-Methode:
// nicht jede denkbare Quelle (z. B. ein künftiger Datei-Adapter) kann das
// sinnvoll leisten — Aufrufer prüfen per Typassertion, ob eine
// MessageSource zusätzlich MailboxLister implementiert (siehe
// manageaccount.UseCase.ListMailboxes).
//
// Setzt wie FetchNew eine bereits erfolgreiche Connect()-Verbindung
// voraus.
type MailboxLister interface {
	ListMailboxes(ctx context.Context) ([]string, error)
}
