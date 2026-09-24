package main

import (
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommitFromBuildSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		settings []debug.BuildSetting
		want     string
	}{
		{name: "keine vcs-informationen", settings: nil, want: ""},
		{
			name:     "sauberer checkout",
			settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc123"}, {Key: "vcs.modified", Value: "false"}},
			want:     "abc123",
		},
		{
			name:     "veränderter checkout",
			settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc123"}, {Key: "vcs.modified", Value: "true"}},
			want:     "abc123-dirty",
		},
		{
			name:     "modified ohne revision",
			settings: []debug.BuildSetting{{Key: "vcs.modified", Value: "true"}},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, commitFromBuildSettings(tt.settings))
		})
	}
}
