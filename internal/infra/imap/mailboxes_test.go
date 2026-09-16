package imap_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	imapadapter "github.com/pmoscode/dmarc-analyzer/internal/infra/imap"
)

func TestListMailboxes_ReturnsAllSelectableMailboxesSorted(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	// DMARC-Berichte können in einem Unterordner statt in INBOX landen
	// (z. B. per Mailregel einsortiert) — genau dafür ist der Picker da.
	require.NoError(t, ts.user.Create("INBOX/DMARC", nil))
	require.NoError(t, ts.user.Create("Archive", nil))

	acc := ts.account(t)
	a := imapadapter.NewAdapter()
	require.NoError(t, a.Connect(context.Background(), acc, account.NewSecretFromString(testPassword)))
	defer func() { _ = a.Close() }()

	got, err := a.ListMailboxes(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"Archive", "INBOX", "INBOX/DMARC"}, got)
}

func TestListMailboxes_NotConnected_ReturnsError(t *testing.T) {
	t.Parallel()
	a := imapadapter.NewAdapter()

	_, err := a.ListMailboxes(context.Background())
	require.Error(t, err)
}
