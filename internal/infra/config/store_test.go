package config_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/settings"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/config"
)

func TestStore_Load_NoFileYet_ReturnsDefaults(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "settings.json"))

	got, err := store.Load(context.Background())

	require.NoError(t, err)
	require.Equal(t, settings.Default(), got)
}

func TestStore_Save_And_Load_RoundTrip(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "settings.json"))
	ctx := context.Background()
	want := settings.Settings{RetentionMonths: 12, SyncIntervalMinutes: 30}

	require.NoError(t, store.Save(ctx, want))

	got, err := store.Load(ctx)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestStore_Save_RejectsInvalidSettings(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "settings.json"))

	err := store.Save(context.Background(), settings.Settings{RetentionMonths: -1})

	require.Error(t, err)
}

func TestStore_Load_CorruptFile_ReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, os.WriteFile(path, []byte("nicht-valides-json"), 0o600))
	store := config.NewStore(path)

	_, err := store.Load(context.Background())

	require.Error(t, err)
}

func TestStore_Save_OverwritesPreviousValue(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "settings.json"))
	ctx := context.Background()

	require.NoError(t, store.Save(ctx, settings.Settings{RetentionMonths: 12, SyncIntervalMinutes: 30}))
	require.NoError(t, store.Save(ctx, settings.Settings{RetentionMonths: 0, SyncIntervalMinutes: 0}))

	got, err := store.Load(ctx)
	require.NoError(t, err)
	require.Equal(t, settings.Settings{}, got)
}
