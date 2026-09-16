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

// newTestDBWithAccount öffnet eine Test-DB und legt darin bereits den
// Account "acc-1" an — sync_state.account_id referenziert accounts(id)
// per Fremdschlüssel, ohne einen vorhandenen Account schlägt jedes Save
// mit FOREIGN KEY constraint failed fehl.
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
	require.NoError(t, err, "kein gespeicherter Fortschritt ist kein Fehlerfall")
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
	require.Equal(t, uint32(2), got.LastUID, "Save muss den vorhandenen Eintrag aktualisieren, nicht duplizieren")
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
	accountRepo := sqlite.NewAccountRepository(db)

	require.NoError(t, stateRepo.Save(ctx, domainsync.State{AccountID: "acc-1", Mailbox: "INBOX", LastUID: 5}))
	require.NoError(t, accountRepo.Delete(ctx, "acc-1"))

	// ON DELETE CASCADE im Schema: mit dem Account verschwindet auch sein
	// Sync-Fortschritt, kein verwaister Datensatz.
	got, err := stateRepo.Load(ctx, "acc-1", "INBOX")
	require.NoError(t, err)
	require.Zero(t, got.LastUID)
}
