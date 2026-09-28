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
		{name: "no vcs information", settings: nil, want: ""},
		{
			name:     "clean checkout",
			settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc123"}, {Key: "vcs.modified", Value: "false"}},
			want:     "abc123",
		},
		{
			name:     "modified checkout",
			settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc123"}, {Key: "vcs.modified", Value: "true"}},
			want:     "abc123-dirty",
		},
		{
			name:     "modified without revision",
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
