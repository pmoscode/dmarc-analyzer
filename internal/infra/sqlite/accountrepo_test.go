package sqlite_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/sqlite"
)

func newTestAccount(t testing.TB, id account.AccountID) *account.MailAccount {
	t.Helper()
	acc, err := account.NewMailAccount(id, "Test-Konto", "imap.example.com", 993, "user@example.com", "INBOX", true, time.Now())
	require.NoError(t, err)
	return acc
}

func TestAccountRepository_SaveAndFindByID_RoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := sqlite.NewAccountRepository(newTestDB(t))

	acc := newTestAccount(t, "acc-1")
	require.NoError(t, repo.Save(ctx, acc))

	loaded, err := repo.FindByID(ctx, "acc-1")
	require.NoError(t, err)
	require.Equal(t, acc.ID, loaded.ID)
	require.Equal(t, acc.DisplayName, loaded.DisplayName)
	require.Equal(t, acc.Host, loaded.Host)
	require.Equal(t, acc.Port, loaded.Port)
	require.Equal(t, acc.Username, loaded.Username)
	require.Equal(t, acc.Mailbox, loaded.Mailbox)
	require.Equal(t, acc.UseTLS, loaded.UseTLS)
}

func TestAccountRepository_Save_UpsertsExistingAccount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := sqlite.NewAccountRepository(newTestDB(t))

	acc := newTestAccount(t, "acc-1")
	require.NoError(t, repo.Save(ctx, acc))

	acc.DisplayName = "Neuer Name"
	require.NoError(t, repo.Save(ctx, acc))

	loaded, err := repo.FindByID(ctx, "acc-1")
	require.NoError(t, err)
	require.Equal(t, "Neuer Name", loaded.DisplayName)

	all, err := repo.FindAll(ctx)
	require.NoError(t, err)
	require.Len(t, all, 1, "Upsert darf keinen zweiten Datensatz anlegen")
}

func TestAccountRepository_FindByID_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := sqlite.NewAccountRepository(newTestDB(t))

	_, err := repo.FindByID(ctx, "nie-angelegt")
	require.Error(t, err)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestAccountRepository_FindAll_OrderedByDisplayName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := sqlite.NewAccountRepository(newTestDB(t))

	accZ := newTestAccount(t, "acc-z")
	accZ.DisplayName = "Z-Konto"
	accA := newTestAccount(t, "acc-a")
	accA.DisplayName = "A-Konto"
	require.NoError(t, repo.Save(ctx, accZ))
	require.NoError(t, repo.Save(ctx, accA))

	all, err := repo.FindAll(ctx)
	require.NoError(t, err)
	require.Len(t, all, 2)
	require.Equal(t, "A-Konto", all[0].DisplayName)
	require.Equal(t, "Z-Konto", all[1].DisplayName)
}
