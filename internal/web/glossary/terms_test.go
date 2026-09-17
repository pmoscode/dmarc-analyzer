package glossary

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"DMARC":                         "dmarc",
		"DMARC-Pass-Rate":               "dmarc-pass-rate",
		"pct":                           "pct",
		"Policy (p=)":                   "policy-p",
		"Sendequelle × Tag (Pass-Rate)": "sendequelle-tag-pass-rate",
		"Aggregate Report (RUA)":        "aggregate-report-rua",
	}
	for in, want := range cases {
		require.Equal(t, want, Slug(in), in)
	}
}

func TestSlug_AllTermsProduceNonEmptyDistinctSlugs(t *testing.T) {
	seen := map[string]string{}
	for _, term := range Terms {
		slug := Slug(term.Name)
		require.NotEmpty(t, slug, term.Name)
		if other, ok := seen[slug]; ok {
			t.Fatalf("Begriffe %q und %q erzeugen denselben Slug %q", term.Name, other, slug)
		}
		seen[slug] = term.Name
	}
}

func TestByName_KnownTerm_ReturnsTerm(t *testing.T) {
	term, ok := ByName("DMARC")
	require.True(t, ok)
	require.Contains(t, term.Definition, "RFC 7489")
}

func TestByName_UnknownTerm_ReturnsFalse(t *testing.T) {
	_, ok := ByName("nicht im glossar")
	require.False(t, ok)
}
