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
	// A foreign domain that itself resolves to 127.0.0.1 (DNS rebinding):
	// the Host header still carries the foreign name, not the allowed
	// public hostname.
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
			name:             "without issuer origin, only 'self'",
			oidcIssuerOrigin: "",
			wantFormAction:   "form-action 'self'",
		},
		{
			name:             "with issuer origin additionally allowed",
			oidcIssuerOrigin: "https://auth.example.com",
			// form-action must contain the issuer origin, otherwise the
			// browser blocks the RP-initiated logout redirect to
			// end_session_endpoint (see the securityHeaders comment) —
			// exactly the bug this is meant to prevent.
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
		panic("something is broken")
	})
	handler := recoverPanic(slog.New(slog.DiscardHandler), panicking)

	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	require.NotPanics(t, func() { handler.ServeHTTP(rec, r) })
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

// newAuthenticatedRequest builds a POST request against
// "http://127.0.0.1:1234/konten" with a valid session cookie and also
// returns the matching CSRF token — most requireCSRF tests need both:
// without a valid session, validCSRFToken already fails at
// sessionFromRequest, not at the actual token comparison under test.
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

	// Neither Origin/Sec-Fetch-Site nor token nor session cookie set —
	// GET must still pass through, CSRF only applies to state-changing
	// methods.
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

	// Neither Origin nor Sec-Fetch-Site set — reject to be safe instead
	// of assuming it's fine.
	r, token := newAuthenticatedRequest(t, a)
	r.Header.Set(csrfTokenHeader, token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireCSRF_PostWithValidTokenAndSecFetchSiteSameOrigin_PassesThrough(t *testing.T) {
	a := newAuth()
	handler := requireCSRF(a, okHandler())

	// No Origin header, but Sec-Fetch-Site: same-origin — some browsers
	// omit Origin on simple same-origin POSTs, but send Sec-Fetch-Site
	// (Fetch Metadata Request Headers).
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

	// Origin: null (literally) instead of a missing header — browsers
	// send this for ordinary (not fetch/htmx-triggered) form POSTs on
	// pages with Referrer-Policy: no-referrer, even when the request is
	// actually same-origin (reproduced with the "Start sync" form).
	// Sec-Fetch-Site stays reliable and must still work as a fallback
	// instead of failing on "null".
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

	// Origin: null without a Sec-Fetch-Site fallback must still be
	// rejected — this is exactly the case that occurs with a real
	// foreign sandboxed-iframe request (see the sameOrigin comment).
	r, token := newAuthenticatedRequest(t, a)
	r.Header.Set("Origin", "null")
	r.Header.Set(csrfTokenHeader, token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	require.Equal(t, http.StatusForbidden, rec.Code)
}
