package account_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

func validArgs() (id account.AccountID, displayName, host string, port int, username, mailbox string, useTLS bool, createdAt time.Time) {
	return "acc-1", "Mein Postfach", "imap.example.com", 993, "user@example.com", "INBOX", true, time.Now()
}

func TestNewMailAccount_ValidInput(t *testing.T) {
	t.Parallel()

	id, displayName, host, port, username, mailbox, useTLS, createdAt := validArgs()
	acc, err := account.NewMailAccount(id, displayName, host, port, username, mailbox, useTLS, createdAt)

	require.NoError(t, err)
	require.Equal(t, id, acc.ID)
	require.Equal(t, displayName, acc.DisplayName)
	require.Equal(t, host, acc.Host)
	require.Equal(t, port, acc.Port)
	require.Equal(t, username, acc.Username)
	require.Equal(t, mailbox, acc.Mailbox)
	require.True(t, acc.UseTLS)
	require.Equal(t, time.UTC, acc.CreatedAt.Location())
}

func TestNewMailAccount_EmptyID_IsInvalid(t *testing.T) {
	t.Parallel()

	_, displayName, host, port, username, mailbox, useTLS, createdAt := validArgs()
	_, err := account.NewMailAccount("", displayName, host, port, username, mailbox, useTLS, createdAt)
	require.Error(t, err)

	_, err = account.NewMailAccount("   ", displayName, host, port, username, mailbox, useTLS, createdAt)
	require.Error(t, err)
}

func TestNewMailAccount_EmptyHost_IsInvalid(t *testing.T) {
	t.Parallel()

	id, displayName, _, port, username, mailbox, useTLS, createdAt := validArgs()
	_, err := account.NewMailAccount(id, displayName, "", port, username, mailbox, useTLS, createdAt)
	require.Error(t, err)
}

func TestNewMailAccount_PortMustBeInValidRange(t *testing.T) {
	t.Parallel()

	id, displayName, host, _, username, mailbox, useTLS, createdAt := validArgs()

	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{name: "unterer Rand", port: 1},
		{name: "typisch: IMAPS", port: 993},
		{name: "oberer Rand", port: 65535},
		{name: "null", port: 0, wantErr: true},
		{name: "negativ", port: -1, wantErr: true},
		{name: "zu groß", port: 65536, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := account.NewMailAccount(id, displayName, host, tt.port, username, mailbox, useTLS, createdAt)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestNewMailAccount_EmptyUsername_IsInvalid(t *testing.T) {
	t.Parallel()

	id, displayName, host, port, _, mailbox, useTLS, createdAt := validArgs()
	_, err := account.NewMailAccount(id, displayName, host, port, "", mailbox, useTLS, createdAt)
	require.Error(t, err)
}

func TestNewMailAccount_EmptyMailbox_DefaultsToINBOX(t *testing.T) {
	t.Parallel()

	id, displayName, host, port, username, _, useTLS, createdAt := validArgs()
	acc, err := account.NewMailAccount(id, displayName, host, port, username, "", useTLS, createdAt)
	require.NoError(t, err)
	require.Equal(t, "INBOX", acc.Mailbox)
}

func TestNewMailAccount_EmptyDisplayName_DefaultsToHost(t *testing.T) {
	t.Parallel()

	id, _, host, port, username, mailbox, useTLS, createdAt := validArgs()
	acc, err := account.NewMailAccount(id, "", host, port, username, mailbox, useTLS, createdAt)
	require.NoError(t, err)
	require.Equal(t, host, acc.DisplayName)
}

func TestNewMailAccount_UseTLSFalse_IsAllowed(t *testing.T) {
	// Klartext-IMAP ist erlaubt (die Bestätigung dafür ist Sache der UI,
	// nicht dieser Invariante) — sonst wäre der Adapter nicht gegen einen
	// TLS-losen Test-Server nutzbar (siehe internal/infra/imap-Tests).
	t.Parallel()

	id, displayName, host, port, username, mailbox, _, createdAt := validArgs()
	acc, err := account.NewMailAccount(id, displayName, host, port, username, mailbox, false, createdAt)
	require.NoError(t, err)
	require.False(t, acc.UseTLS)
}
