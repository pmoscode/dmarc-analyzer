package imap

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

// dial baut die Netzwerkverbindung zum IMAP-Server auf und reicht sie an
// imapclient.New weiter. Die Dial*-Funktionen von imapclient sind nicht
// context-aware; deshalb wird hier selbst über net.Dialer/tls.Dialer mit
// DialContext verbunden — das respektiert ctx (Abbruch, Deadline) bereits
// beim Verbindungsaufbau.
func dial(ctx context.Context, acc account.MailAccount) (*imapclient.Client, error) {
	address := fmt.Sprintf("%s:%d", acc.Host, acc.Port)

	var conn net.Conn
	var err error
	if acc.UseTLS {
		// IMPLEMENTIERUNG.md Abschnitt 9: reguläre Zertifikatsprüfung
		// gegen den System-Trust-Store, InsecureSkipVerify wird an keiner
		// Stelle gesetzt.
		tlsDialer := &tls.Dialer{NetDialer: &net.Dialer{}}
		conn, err = tlsDialer.DialContext(ctx, "tcp", address)
	} else {
		conn, err = (&net.Dialer{}).DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return nil, fmt.Errorf("verbindung zu %s konnte nicht aufgebaut werden: %w", address, err)
	}

	return imapclient.New(conn, &imapclient.Options{}), nil
}
