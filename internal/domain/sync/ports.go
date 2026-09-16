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

// ReportParser wandelt einen Rohanhang in Domänenobjekte. Über Supports()
// wird die passende Implementierung gewählt — so kommen RUF und TLS-RPT
// später additiv hinzu, ohne bestehenden Code zu ändern (Open/Closed,
// IMPLEMENTIERUNG.md Abschnitt 6.4).
type ReportParser interface {
	Supports(attachment RawAttachment) bool
	Parse(ctx context.Context, attachment RawAttachment) (*report.AggregateReport, error)
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
