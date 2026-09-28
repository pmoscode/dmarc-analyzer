package sqlite_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	domainsync "github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/sqlite"
)

// newTestDBWithAccount opens a test DB and already creates the account
// "acc-1" in it — sync_state.account_id references accounts(id) via a
// foreign key, so without an existing account every Save fails with
// FOREIGN KEY constraint failed.
func newTestDBWithAccount(t *testing.T) *sql.DB {
	t.Helper()

	db := newTestDB(t)
	acc := newTestAccount(t, "acc-1")
	require.NoError(t, sqlite.NewAccountRepository(db).Save(context.Background(), acc))
	return db
}

func TestSyncStateRepository_Load_ReturnsZeroValueWhenNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := sqlite.NewSyncStateRepository(newTestDBWithAccount(t))

	state, err := repo.Load(ctx, "acc-1", "INBOX")
	require.NoError(t, err, "no stored progress is not an error case")
	require.Equal(t, domainsync.State{AccountID: "acc-1", Mailbox: "INBOX"}, state)
}

func TestSyncStateRepository_SaveAndLoad_RoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := sqlite.NewSyncStateRepository(newTestDBWithAccount(t))

	want := domainsync.State{
		AccountID:   "acc-1",
		Mailbox:     "INBOX",
		UIDValidity: 12345,
		LastUID:     42,
		LastSyncAt:  time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC),
	}
	require.NoError(t, repo.Save(ctx, want))

	got, err := repo.Load(ctx, "acc-1", "INBOX")
	require.NoError(t, err)
	require.Equal(t, want.AccountID, got.AccountID)
	require.Equal(t, want.Mailbox, got.Mailbox)
	require.Equal(t, want.UIDValidity, got.UIDValidity)
	require.Equal(t, want.LastUID, got.LastUID)
	require.True(t, want.LastSyncAt.Equal(got.LastSyncAt))
}

func TestSyncStateRepository_Save_UpsertsExistingState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := sqlite.NewSyncStateRepository(newTestDBWithAccount(t))

	require.NoError(t, repo.Save(ctx, domainsync.State{AccountID: "acc-1", Mailbox: "INBOX", LastUID: 1}))
	require.NoError(t, repo.Save(ctx, domainsync.State{AccountID: "acc-1", Mailbox: "INBOX", LastUID: 2}))

	got, err := repo.Load(ctx, "acc-1", "INBOX")
	require.NoError(t, err)
	require.Equal(t, uint32(2), got.LastUID, "Save must update the existing entry, not duplicate it")
}

func TestSyncStateRepository_Save_WithoutTimestamp_DefaultsToNow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := sqlite.NewSyncStateRepository(newTestDBWithAccount(t))

	before := time.Now().Add(-time.Second)
	require.NoError(t, repo.Save(ctx, domainsync.State{AccountID: "acc-1", Mailbox: "INBOX"}))
	after := time.Now().Add(time.Second)

	got, err := repo.Load(ctx, "acc-1", "INBOX")
	require.NoError(t, err)
	require.True(t, got.LastSyncAt.After(before) && got.LastSyncAt.Before(after))
}

func TestSyncStateRepository_DifferentMailboxesAreIndependent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := sqlite.NewSyncStateRepository(newTestDBWithAccount(t))

	require.NoError(t, repo.Save(ctx, domainsync.State{AccountID: "acc-1", Mailbox: "INBOX", LastUID: 10}))
	require.NoError(t, repo.Save(ctx, domainsync.State{AccountID: "acc-1", Mailbox: "Archive", LastUID: 20}))

	inbox, err := repo.Load(ctx, "acc-1", "INBOX")
	require.NoError(t, err)
	require.Equal(t, uint32(10), inbox.LastUID)

	archive, err := repo.Load(ctx, "acc-1", "Archive")
	require.NoError(t, err)
	require.Equal(t, uint32(20), archive.LastUID)
}

func TestSyncStateRepository_DeletingAccount_CascadesState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDBWithAccount(t)
	stateRepo := sqlite.NewSyncStateRepository(db)

	require.NoError(t, stateRepo.Save(ctx, domainsync.State{AccountID: "acc-1", Mailbox: "INBOX", LastUID: 5}))
	// No more AccountRepository.Delete (accounts come from ENV, no
	// CRUD) — the deletion here exclusively tests the schema's
	// ON DELETE CASCADE property, hence directly via SQL.
	_, err := db.ExecContext(ctx, "DELETE FROM accounts WHERE id = ?", "acc-1")
	require.NoError(t, err)

	// ON DELETE CASCADE in the schema: the account's sync progress
	// disappears along with it, no orphaned record.
	got, err := stateRepo.Load(ctx, "acc-1", "INBOX")
	require.NoError(t, err)
	require.Zero(t, got.LastUID)
}
