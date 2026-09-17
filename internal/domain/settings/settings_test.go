package settings_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/settings"
)

func TestDefault_MatchesDocumentedDefaults(t *testing.T) {
	got := settings.Default()

	require.Equal(t, settings.DefaultRetentionMonths, got.RetentionMonths)
	require.Equal(t, settings.DefaultSyncIntervalMinutes, got.SyncIntervalMinutes)
	require.NoError(t, got.Validate())
}

func TestSettings_Validate(t *testing.T) {
	tests := []struct {
		name    string
		s       settings.Settings
		wantErr bool
	}{
		{"zero values are valid (off)", settings.Settings{}, false},
		{"positive values are valid", settings.Settings{RetentionMonths: 24, SyncIntervalMinutes: 60}, false},
		{"negative retention is invalid", settings.Settings{RetentionMonths: -1}, true},
		{"negative sync interval is invalid", settings.Settings{SyncIntervalMinutes: -1}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.s.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
