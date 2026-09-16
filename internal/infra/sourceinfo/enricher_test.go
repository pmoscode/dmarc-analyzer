package sourceinfo

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// fakeResolver ersetzt den echten DNS-Resolver in Tests — kein Netzwerk,
// deterministisch, zählt Aufrufe zum Nachweis des Caches.
type fakeResolver struct {
	names map[string][]string
	err   error
	calls int
}

func (f *fakeResolver) LookupAddr(_ context.Context, addr string) ([]string, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.names[addr], nil
}

func mustSourceIP(t *testing.T, s string) report.SourceIP {
	t.Helper()
	ip, err := report.NewSourceIP(s)
	require.NoError(t, err)
	return ip
}

func TestEnricher_Enrich_ResolvesHostnameAndKnownService(t *testing.T) {
	resolver := &fakeResolver{names: map[string][]string{
		"203.0.113.1": {"mail-sor-f41.google.com."},
	}}
	e := &Enricher{resolver: resolver}

	got := e.Enrich(context.Background(), mustSourceIP(t, "203.0.113.1"))

	require.Equal(t, "mail-sor-f41.google.com", got.Hostname, "trailing dot muss entfernt werden")
	require.Equal(t, "Google Workspace", got.Service)
}

func TestEnricher_Enrich_UnknownHostname_NoServiceButHostnameSet(t *testing.T) {
	resolver := &fakeResolver{names: map[string][]string{
		"203.0.113.2": {"mail.some-random-host.example."},
	}}
	e := &Enricher{resolver: resolver}

	got := e.Enrich(context.Background(), mustSourceIP(t, "203.0.113.2"))

	require.Equal(t, "mail.some-random-host.example", got.Hostname)
	require.Empty(t, got.Service)
}

func TestEnricher_Enrich_LookupFails_ReturnsEmptyEnrichmentNoPanic(t *testing.T) {
	resolver := &fakeResolver{err: errors.New("kein ptr-eintrag")}
	e := &Enricher{resolver: resolver}

	got := e.Enrich(context.Background(), mustSourceIP(t, "203.0.113.3"))

	require.Empty(t, got.Hostname)
	require.Empty(t, got.Service)
}

func TestEnricher_Enrich_CachesResultAcrossCalls(t *testing.T) {
	resolver := &fakeResolver{names: map[string][]string{
		"203.0.113.4": {"mail.sendgrid.net."},
	}}
	e := &Enricher{resolver: resolver}

	ip := mustSourceIP(t, "203.0.113.4")
	first := e.Enrich(context.Background(), ip)
	second := e.Enrich(context.Background(), ip)

	require.Equal(t, first, second)
	require.Equal(t, 1, resolver.calls, "zweiter Aufruf muss aus dem Cache kommen, nicht erneut auflösen")
}

func TestDetectService_MatchesKnownSuffixesCaseInsensitively(t *testing.T) {
	require.Equal(t, "Microsoft 365", detectService("MAIL.PROTECTION.OUTLOOK.COM"))
	require.Equal(t, "Mailchimp", detectService("mail123.mcdlv.net"))
	require.Equal(t, "Brevo (vormals Sendinblue)", detectService("mta1.brevo.com"))
	require.Empty(t, detectService("unbekannt.example.com"))
	require.Empty(t, detectService(""))
}
