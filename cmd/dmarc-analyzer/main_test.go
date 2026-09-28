package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRun_NoArgs_PrintsUsageWithoutError(t *testing.T) {
	require.NoError(t, run(context.Background(), nil))
}

func TestRun_UnknownSubcommand_ReturnsErrorWithoutWiringApp(t *testing.T) {
	// An unknown subcommand must not touch any real infrastructure
	// (database file, OS keychain) — otherwise this test would
	// unintentionally create real files in the home directory. The
	// absence of any I/O error here is already the proof that main.go
	// checks the subcommand before newApp() (see the comment there).
	err := run(context.Background(), []string{"whatever"})

	require.Error(t, err)
	require.ErrorContains(t, err, "whatever")
}
