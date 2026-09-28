// Package account contains the MailAccount aggregate, the Secret type for
// safely handling credentials in memory, and the Repository port. Secret
// has come from internal/infra/envconfig rather than a persisted keychain
// ever since the switch to pure ENV configuration.
package account

import (
	"fmt"
	"strings"
	"time"
)

// AccountID uniquely identifies a MailAccount. Assigned by the persistence
// layer (UUID or similar), not by the domain model.
//
// Renaming to "ID" would collide with the MailAccount.ID field (field and
// type would both be called "ID ID") — the same exception as
// report.ReportID, see AGENTS.md.
//
//nolint:revive // "AccountID" stutters as account.AccountID, but a
type AccountID string

// defaultMailbox is the default IMAP mailbox when none is given.
const defaultMailbox = "INBOX"

// MailAccount is the aggregate for a configured IMAP mailbox from which
// DMARC aggregate reports are fetched. Deliberately contains no password —
// that comes exclusively from an environment variable
// (internal/infra/envconfig) and is never stored in the DB.
type MailAccount struct {
	ID          AccountID
	DisplayName string
	Host        string
	Port        int
	Username    string
	Mailbox     string
	// UseTLS controls IMAPS (typically port 993). Plaintext IMAP is only
	// meant to be used with explicit confirmation in the UI (section 9) —
	// that confirmation is the UI layer's concern, not this invariant; the
	// domain model deliberately allows UseTLS=false, otherwise it couldn't
	// be used against a local test server without TLS.
	UseTLS    bool
	CreatedAt time.Time
}

// NewMailAccount enforces the obvious invariants: ID, Host and Username
// must not be empty, Port must be a valid TCP port. Mailbox falls back to
// "INBOX" when empty.
func NewMailAccount(id AccountID, displayName, host string, port int, username, mailbox string, useTLS bool, createdAt time.Time) (*MailAccount, error) {
	if strings.TrimSpace(string(id)) == "" {
		return nil, fmt.Errorf("account without id is invalid")
	}
	if strings.TrimSpace(host) == "" {
		return nil, fmt.Errorf("account without host is invalid")
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("port must be between 1 and 65535, was %d", port)
	}
	if strings.TrimSpace(username) == "" {
		return nil, fmt.Errorf("account without username is invalid")
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
