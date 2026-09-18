package glossary

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestByName_KnownTerm_ReturnsTerm(t *testing.T) {
	term, ok := ByName("DMARC")
	require.True(t, ok)
	require.Contains(t, term.Definition, "RFC 7489")
}

func TestByName_UnknownTerm_ReturnsFalse(t *testing.T) {
	_, ok := ByName("nicht im glossar")
	require.False(t, ok)
}
