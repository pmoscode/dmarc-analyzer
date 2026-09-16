package main

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/manageaccount"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

func testAppForAccount() (*app, *fakeAccountRepository, *fakeCredentialStore) {
	accounts := newFakeAccountRepository()
	creds := newFakeCredentialStore()
	return &app{
		accounts: &manageaccount.UseCase{Accounts: accounts, Credentials: creds},
	}, accounts, creds
}

// withStdin ersetzt os.Stdin für die Dauer von fn — runAccountAdd liest das
// Passwort interaktiv, das simulieren Tests über einen Pipe-Reader.
func withStdin(t *testing.T, input string, fn func()) {
	t.Helper()

	r, w, err := os.Pipe()
	require.NoError(t, err)
	original := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = original }()

	go func() {
		_, _ = io.WriteString(w, input)
		_ = w.Close()
	}()

	fn()
}

func TestRunAccount_NoSubcommand_ReturnsError(t *testing.T) {
	t.Parallel()

	a, _, _ := testAppForAccount()
	err := runAccount(context.Background(), a, nil)
	require.Error(t, err)
}

func TestRunAccount_UnknownSubcommand_ReturnsError(t *testing.T) {
	t.Parallel()

	a, _, _ := testAppForAccount()
	err := runAccount(context.Background(), a, []string{"irgendwas"})
	require.Error(t, err)
}

func TestRunAccountAdd_MissingRequiredFlags_ReturnsError(t *testing.T) {
	t.Parallel()

	a, _, _ := testAppForAccount()
	err := runAccountAdd(context.Background(), a, []string{"--username", "user@example.com"})
	require.Error(t, err, "--host fehlt")
}

func TestRunAccountAdd_Success_StoresMetadataAndPassword(t *testing.T) {
	a, accounts, creds := testAppForAccount()

	withStdin(t, "app-passwort\n", func() {
		err := runAccountAdd(context.Background(), a, []string{
			"--host", "imap.example.com", "--username", "user@example.com",
		})
		require.NoError(t, err)
	})

	require.Len(t, accounts.accounts, 1)
	for id := range accounts.accounts {
		secret, err := creds.Retrieve(id)
		require.NoError(t, err)
		require.Equal(t, "app-passwort", string(secret.Expose()))
	}
}

func TestRunAccountList_Empty(t *testing.T) {
	t.Parallel()

	a, _, _ := testAppForAccount()
	require.NoError(t, runAccountList(context.Background(), a))
}

func TestRunAccountDelete_RemovesAccount(t *testing.T) {
	t.Parallel()

	a, accounts, _ := testAppForAccount()
	acc, err := account.NewMailAccount("acc-1", "Test", "imap.example.com", 993, "u@example.com", "INBOX", true, time.Now())
	require.NoError(t, err)
	accounts.accounts["acc-1"] = *acc

	require.NoError(t, runAccountDelete(context.Background(), a, []string{"acc-1"}))
	require.NotContains(t, accounts.accounts, account.AccountID("acc-1"))
}

func TestRunAccountDelete_MissingID_ReturnsError(t *testing.T) {
	t.Parallel()

	a, _, _ := testAppForAccount()
	err := runAccountDelete(context.Background(), a, nil)
	require.Error(t, err)
}

func TestRunAccountTest_UnknownAccount_ReturnsError(t *testing.T) {
	t.Parallel()

	a, _, _ := testAppForAccount()
	err := runAccountTest(context.Background(), a, []string{"nie-angelegt"})
	require.Error(t, err)
}
