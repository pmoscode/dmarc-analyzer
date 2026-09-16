// Package account enthält das Aggregate MailAccount, den Typ Secret für
// den sicheren Umgang mit Zugangsdaten im Speicher, sowie die Ports
// Repository und CredentialStore (IMPLEMENTIERUNG.md Abschnitt 6.4 und 9).
package account

import (
	"fmt"
	"strings"
	"time"
)

// AccountID identifiziert ein MailAccount eindeutig. Von der
// Persistenzschicht vergeben (UUID o. ä.), nicht vom Domänenmodell.
//
// Umbenennung zu "ID" würde mit dem Feld MailAccount.ID kollidieren
// (Feld und Typ hießen dann identisch "ID ID") — dieselbe Ausnahme wie bei
// report.ReportID, siehe AGENTS.md.
//
//nolint:revive // "AccountID" stuttert als account.AccountID, aber eine
type AccountID string

// defaultMailbox ist das IMAP-Standardpostfach, wenn keines angegeben wird.
const defaultMailbox = "INBOX"

// MailAccount ist das Aggregate für ein konfiguriertes IMAP-Postfach, aus
// dem DMARC-Aggregate-Reports abgeholt werden. Enthält bewusst kein
// Passwort — das liegt ausschließlich im Schlüsselbund
// (IMPLEMENTIERUNG.md Abschnitt 9, Regel "Passwort nie in der DB").
type MailAccount struct {
	ID          AccountID
	DisplayName string
	Host        string
	Port        int
	Username    string
	Mailbox     string
	// UseTLS steuert IMAPS (Port 993 typischerweise). Klartext-IMAP ist nur
	// mit expliziter Bestätigung in der UI vorgesehen (Abschnitt 9) — diese
	// Bestätigung ist Sache der UI-Schicht, nicht dieser Invariante; das
	// Domänenmodell lässt UseTLS=false bewusst zu, weil es sonst nicht
	// gegen einen lokalen Test-Server ohne TLS nutzbar wäre.
	UseTLS    bool
	CreatedAt time.Time
}

// NewMailAccount erzwingt die naheliegenden Invarianten: ID, Host und
// Username dürfen nicht leer sein, Port muss ein gültiger TCP-Port sein.
// Mailbox fällt auf "INBOX" zurück, wenn leer.
func NewMailAccount(id AccountID, displayName, host string, port int, username, mailbox string, useTLS bool, createdAt time.Time) (*MailAccount, error) {
	if strings.TrimSpace(string(id)) == "" {
		return nil, fmt.Errorf("account ohne id ist ungültig")
	}
	if strings.TrimSpace(host) == "" {
		return nil, fmt.Errorf("account ohne host ist ungültig")
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("port muss zwischen 1 und 65535 liegen, war %d", port)
	}
	if strings.TrimSpace(username) == "" {
		return nil, fmt.Errorf("account ohne benutzername ist ungültig")
	}

	if strings.TrimSpace(mailbox) == "" {
		mailbox = defaultMailbox
	}
	if strings.TrimSpace(displayName) == "" {
		displayName = host
	}

	return &MailAccount{
		ID:          id,
		DisplayName: displayName,
		Host:        host,
		Port:        port,
		Username:    username,
		Mailbox:     mailbox,
		UseTLS:      useTLS,
		CreatedAt:   createdAt.UTC(),
	}, nil
}
