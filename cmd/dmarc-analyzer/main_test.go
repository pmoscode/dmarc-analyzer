package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRun_NoArgs_ReturnsNil(t *testing.T) {
	require.NoError(t, run(nil))
}

func TestRun_UnknownArg_ReturnsError(t *testing.T) {
	err := run([]string{"irgendwas"})

	require.Error(t, err)
	require.ErrorContains(t, err, "irgendwas")
}
