// Package imap implements the sync.MessageSource port against an IMAP
// mailbox (github.com/emersion/go-imap/v2).
package imap

import (
	"context"
	"errors"
	"fmt"

	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

// Adapter implements sync.MessageSource against an IMAP mailbox: TLS
// enforced (provided MailAccount.UseTLS is set — the confirmation for
// plaintext IMAP lives in the UI, see IMPLEMENTIERUNG.md section 9),
// UID-based, using BODY.PEEK and EXAMINE instead of SELECT so the mailbox
// stays untouched for other clients.
type Adapter struct {
	client *imapclient.Client
}

var _ sync.MessageSource = (*Adapter)(nil)

// errNotConnected is returned by FetchNew/Close when Connect has not yet
// been called successfully.
var errNotConnected = errors.New("not connected — Connect must be called first")

// NewAdapter creates an unconnected Adapter. Connect must be called before
// FetchNew.
func NewAdapter() *Adapter {
	return &Adapter{}
}

// Connect establishes the connection (with backoff on transient network
// errors, see backoff.go) and logs in. A login error is NOT retried — a
// wrong password doesn't become correct by trying again, and repeated
// failed attempts can get accounts locked out by the provider.
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
		return fmt.Errorf("failed to establish connection to %s:%d: %w", acc.Host, acc.Port, dialErr)
	}

	loginErr := runCtx(ctx, client, func() error {
		return client.Login(acc.Username, string(secret.Expose())).Wait()
	})
	if loginErr != nil {
		_ = client.Close()
		return fmt.Errorf(
			"login as %q failed — an app password is required when two-factor authentication is enabled: %w",
			acc.Username, loginErr,
		)
	}

	a.client = client
	return nil
}

// Close logs out and closes the connection. A logout error (e.g. because
// the connection was already closed by a cancelled context) is not a
// reason to fail Close itself — the connection is closed afterward
// regardless.
func (a *Adapter) Close() error {
	if a.client == nil {
		return nil
	}
	_ = a.client.Logout().Wait()
	return a.client.Close()
}
