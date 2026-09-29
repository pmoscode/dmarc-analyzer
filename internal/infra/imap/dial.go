package imap

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

// dial establishes the network connection to the IMAP server and hands it
// off to imapclient.New. imapclient's Dial* functions are not
// context-aware; that's why we connect here ourselves via
// net.Dialer/tls.Dialer with DialContext — this already honors ctx
// (cancellation, deadline) during connection setup.
func dial(ctx context.Context, acc account.MailAccount) (*imapclient.Client, error) {
	address := fmt.Sprintf("%s:%d", acc.Host, acc.Port)

	var conn net.Conn
	var err error
	if acc.UseTLS {
		// IMPLEMENTIERUNG.md section 9: regular certificate verification
		// against the system trust store, InsecureSkipVerify is never set
		// anywhere.
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
