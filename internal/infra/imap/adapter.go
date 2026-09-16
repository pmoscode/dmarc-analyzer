// Package imap implementiert den Port sync.MessageSource gegen ein
// IMAP-Postfach (github.com/emersion/go-imap/v2).
package imap

import (
	"context"
	"errors"
	"fmt"

	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

// Adapter implementiert sync.MessageSource gegen ein IMAP-Postfach: TLS
// erzwungen (sofern MailAccount.UseTLS gesetzt ist — die Bestätigung für
// Klartext-IMAP liegt in der UI, siehe IMPLEMENTIERUNG.md Abschnitt 9),
// UID-basiert, mit BODY.PEEK und EXAMINE statt SELECT, damit das Postfach
// für andere Clients unberührt bleibt.
type Adapter struct {
	client *imapclient.Client
}

var _ sync.MessageSource = (*Adapter)(nil)

// errNotConnected wird von FetchNew/Close zurückgegeben, wenn Connect noch
// nicht erfolgreich aufgerufen wurde.
var errNotConnected = errors.New("nicht verbunden — Connect muss zuerst aufgerufen werden")

// NewAdapter erzeugt einen unverbundenen Adapter. Connect muss vor
// FetchNew aufgerufen werden.
func NewAdapter() *Adapter {
	return &Adapter{}
}

// Connect baut die Verbindung auf (mit Backoff bei temporären
// Netzwerkfehlern, siehe backoff.go) und meldet sich an. Ein
// Anmeldefehler wird NICHT wiederholt — ein falsches Passwort wird durch
// erneutes Versuchen nicht richtig, und wiederholte Fehlversuche können
// Konten beim Provider sperren.
func (a *Adapter) Connect(ctx context.Context, acc account.MailAccount, secret account.Secret) error {
	var client *imapclient.Client
	dialErr := retry(ctx, func() error {
		c, err := dial(ctx, acc)
		if err != nil {
			return err
		}
		client = c
		return nil
	})
	if dialErr != nil {
		return fmt.Errorf("verbindung zu %s:%d konnte nicht aufgebaut werden: %w", acc.Host, acc.Port, dialErr)
	}

	loginErr := runCtx(ctx, client, func() error {
		return client.Login(acc.Username, string(secret.Expose())).Wait()
	})
	if loginErr != nil {
		_ = client.Close()
		return fmt.Errorf(
			"anmeldung als %q fehlgeschlagen — bei aktivierter Zwei-Faktor-Authentifizierung wird ein App-Passwort benötigt: %w",
			acc.Username, loginErr,
		)
	}

	a.client = client
	return nil
}

// Close meldet ab und schließt die Verbindung. Ein Fehler beim Abmelden
// (z. B. weil die Verbindung durch einen abgebrochenen Kontext bereits
// geschlossen wurde) ist kein Grund, Close selbst scheitern zu lassen —
// die Verbindung wird ohnehin im Anschluss geschlossen.
func (a *Adapter) Close() error {
	if a.client == nil {
		return nil
	}
	_ = a.client.Logout().Wait()
	return a.client.Close()
}
