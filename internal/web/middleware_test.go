package web

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func TestRequireHost_AllowedHost_PassesThrough(t *testing.T) {
	handler := requireHost("127.0.0.1:1234", okHandler())

	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://127.0.0.1:1234/", nil)
	r.Host = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireHost_ForeignHost_Rejected(t *testing.T) {
	handler := requireHost("127.0.0.1:1234", okHandler())

	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://evil.example.com/", nil)
	r.Host = "evil.example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	require.Equal(t, http.StatusMisdirectedRequest, rec.Code)
}

func TestRequireHost_DNSRebindingAttempt_Rejected(t *testing.T) {
	// Eine fremde Domain, die selbst auf 127.0.0.1 auflöst (DNS-Rebinding):
	// der Host-Header trägt trotzdem den fremden Namen, nicht den
	// erlaubten öffentlichen Hostnamen.
	handler := requireHost("dmarc.example.com", okHandler())

	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://rebind.example.com/", nil)
	r.Host = "rebind.example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	require.Equal(t, http.StatusMisdirectedRequest, rec.Code)
}

func TestBuildContentSecurityPolicy_TableDriven(t *testing.T) {
	tests := []struct {
		name             string
		oidcIssuerOrigin string
		wantFormAction   string
	}{
		{
			name:             "ohne issuer-origin nur 'self'",
			oidcIssuerOrigin: "",
			wantFormAction:   "form-action 'self'",
		},
		{
			name:             "mit issuer-origin zusaetzlich erlaubt",
			oidcIssuerOrigin: "https://auth.example.com",
			// form-action muss die Issuer-Origin enthalten, sonst blockiert der
			// Browser den RP-Initiated-Logout-Redirect zu end_session_endpoint
			// (siehe securityHeaders-Kommentar) — genau der Bug, der hier
			// verhindert werden soll.
			wantFormAction: "form-action 'self' https://auth.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csp := buildContentSecurityPolicy(tt.oidcIssuerOrigin)

			require.Contains(t, csp, tt.wantFormAction)
			require.Contains(t, csp, "default-src 'self'")
			require.Contains(t, csp, "frame-ancestors 'none'")
			require.Contains(t, csp, "base-uri 'none'")
			require.NotContains(t, csp, "unsafe-inline")
		})
	}
}

func TestServerSecurityHeaders_SetsCSPAndRelatedHeaders(t *testing.T) {
	s := &Server{oidcIssuerOrigin: "https://auth.example.com"}
	handler := s.securityHeaders(okHandler())

	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	csp := rec.Header().Get("Content-Security-Policy")
	require.Contains(t, csp, "form-action 'self' https://auth.example.com")
	require.NotContains(t, csp, "unsafe-inline")
	require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	require.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
}

func TestRequireSession_NoCookie_RedirectsToLogin(t *testing.T) {
	a := newAuth()
	handler := requireSession(a, okHandler())

	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	require.Equal(t, http.StatusSeeOther, rec.Code)
	require.Equal(t, "/anmelden", rec.Header().Get("Location"))
}

func TestRequireSession_ValidCookie_PassesThrough(t *testing.T) {
	a := newAuth()
	cookieValue, _, err := a.createSession("admin@example.com")
	require.NoError(t, err)
	handler := requireSession(a, okHandler())

	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: cookieValue})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestRecoverPanic_HandlerPanics_Returns500WithoutCrashingProcess(t *testing.T) {
	panicking := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("etwas ist kaputt")
	})
	handler := recoverPanic(slog.New(slog.DiscardHandler), panicking)

	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	require.NotPanics(t, func() { handler.ServeHTTP(rec, r) })
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

// newAuthenticatedRequest baut eine POST-Anfrage gegen
// "http://127.0.0.1:1234/konten" mit gültigem Sitzungs-Cookie und liefert
// zusätzlich das zugehörige CSRF-Token — die meisten requireCSRF-Tests
// brauchen beides: ohne eine gültige Sitzung schlägt validCSRFToken schon
// an sessionFromRequest, nicht am eigentlich zu testenden Token-Vergleich.
func newAuthenticatedRequest(t *testing.T, a *auth) (*http.Request, string) {
	t.Helper()
	cookieValue, csrfToken, err := a.createSession("admin@example.com")
	require.NoError(t, err)

	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "http://127.0.0.1:1234/konten", nil)
	r.Host = "127.0.0.1:1234"
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: cookieValue})
	return r, csrfToken
}

func TestRequireCSRF_GetRequest_NeverChecked(t *testing.T) {
	a := newAuth()
	handler := requireCSRF(a, okHandler())

	// Weder Origin/Sec-Fetch-Site noch Token noch Sitzungs-Cookie gesetzt
	// — GET muss trotzdem durchgehen, CSRF betrifft nur zustandsändernde
	// Methoden.
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://127.0.0.1:1234/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireCSRF_PostWithValidTokenAndOrigin_PassesThrough(t *testing.T) {
	a := newAuth()
	handler := requireCSRF(a, okHandler())

	r, token := newAuthenticatedRequest(t, a)
	r.Header.Set("Origin", "http://127.0.0.1:1234")
	r.Header.Set(csrfTokenHeader, token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireCSRF_PostWithValidTokenAsFormField_PassesThrough(t *testing.T) {
	a := newAuth()
	cookieValue, csrfToken, err := a.createSession("admin@example.com")
	require.NoError(t, err)
	handler := requireCSRF(a, okHandler())

	form := url.Values{"csrf_token": {csrfToken}}
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "http://127.0.0.1:1234/konten", strings.NewReader(form.Encode()))
	r.Host = "127.0.0.1:1234"
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "http://127.0.0.1:1234")
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: cookieValue})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireCSRF_PostWithoutToken_Returns403(t *testing.T) {
	a := newAuth()
	handler := requireCSRF(a, okHandler())

	r, _ := newAuthenticatedRequest(t, a)
	r.Header.Set("Origin", "http://127.0.0.1:1234")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireCSRF_PostWithWrongToken_Returns403(t *testing.T) {
	a := newAuth()
	handler := requireCSRF(a, okHandler())

	r, _ := newAuthenticatedRequest(t, a)
	r.Header.Set("Origin", "http://127.0.0.1:1234")
	r.Header.Set(csrfTokenHeader, "definitiv-falsches-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireCSRF_PostWithValidTokenButForeignOrigin_Returns403(t *testing.T) {
	a := newAuth()
	handler := requireCSRF(a, okHandler())

	r, token := newAuthenticatedRequest(t, a)
	r.Header.Set("Origin", "http://evil.example.com")
	r.Header.Set(csrfTokenHeader, token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireCSRF_PostWithoutOriginOrSecFetchSite_Returns403(t *testing.T) {
	a := newAuth()
	handler := requireCSRF(a, okHandler())

	// Weder Origin noch Sec-Fetch-Site gesetzt — sicherheitshalber
	// ablehnen statt anzunehmen, es sei schon in Ordnung.
	r, token := newAuthenticatedRequest(t, a)
	r.Header.Set(csrfTokenHeader, token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireCSRF_PostWithValidTokenAndSecFetchSiteSameOrigin_PassesThrough(t *testing.T) {
	a := newAuth()
	handler := requireCSRF(a, okHandler())

	// Kein Origin-Header, aber Sec-Fetch-Site: same-origin — manche
	// Browser lassen Origin bei einfachen same-origin-POSTs weg, senden
	// aber Sec-Fetch-Site (Fetch Metadata Request Headers).
	r, token := newAuthenticatedRequest(t, a)
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	r.Header.Set(csrfTokenHeader, token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireCSRF_PostWithNullOriginAndSecFetchSiteSameOrigin_PassesThrough(t *testing.T) {
	a := newAuth()
	handler := requireCSRF(a, okHandler())

	// Origin: null (wörtlich) statt eines fehlenden Headers — das
	// schicken Browser für gewöhnliche (nicht per fetch/htmx ausgelöste)
	// Formular-POSTs auf Seiten mit Referrer-Policy: no-referrer, auch
	// wenn die Anfrage tatsächlich same-origin ist (reproduziert beim
	// "Abgleich starten"-Formular). Sec-Fetch-Site bleibt zuverlässig und
	// muss weiterhin als Fallback greifen statt an "null" zu scheitern.
	r, token := newAuthenticatedRequest(t, a)
	r.Header.Set("Origin", "null")
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	r.Header.Set(csrfTokenHeader, token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireCSRF_PostWithNullOriginNoSecFetchSite_Returns403(t *testing.T) {
	a := newAuth()
	handler := requireCSRF(a, okHandler())

	// Origin: null ohne Sec-Fetch-Site-Fallback muss weiterhin abgelehnt
	// werden — genau dieser Fall tritt bei einer echten fremden
	// Sandbox-iframe-Anfrage auf (siehe sameOrigin-Kommentar).
	r, token := newAuthenticatedRequest(t, a)
	r.Header.Set("Origin", "null")
	r.Header.Set(csrfTokenHeader, token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	require.Equal(t, http.StatusForbidden, rec.Code)
}
