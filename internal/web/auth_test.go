package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAuth_BeginAndRedeemLogin_Success(t *testing.T) {
	a := newAuth()

	id, p, err := a.beginLogin()
	require.NoError(t, err)
	require.NotEmpty(t, id)
	require.NotEmpty(t, p.state)
	require.NotEmpty(t, p.nonce)
	require.NotEmpty(t, p.pkceVerifier)

	got, ok := a.redeemLogin(id, p.state)
	require.True(t, ok)
	require.Equal(t, p, got)
}

func TestAuth_RedeemLogin_WrongState_Fails(t *testing.T) {
	a := newAuth()
	id, _, err := a.beginLogin()
	require.NoError(t, err)

	_, ok := a.redeemLogin(id, "definitely-wrong-state")
	require.False(t, ok)
}

func TestAuth_RedeemLogin_UnknownID_Fails(t *testing.T) {
	a := newAuth()

	_, ok := a.redeemLogin("unknown-id", "some-state")
	require.False(t, ok)
}

func TestAuth_RedeemLogin_IsSingleUse(t *testing.T) {
	a := newAuth()
	id, p, err := a.beginLogin()
	require.NoError(t, err)

	_, ok := a.redeemLogin(id, p.state)
	require.True(t, ok, "first attempt must succeed")

	_, ok = a.redeemLogin(id, p.state)
	require.False(t, ok, "second attempt with the same ID must fail")
}

func TestAuth_RedeemLogin_Expired_Fails(t *testing.T) {
	a := newAuth()
	id, p, err := a.beginLogin()
	require.NoError(t, err)

	// Artificially set the expiry into the past, instead of actually
	// waiting 10 minutes in the test.
	a.mu.Lock()
	expired := a.pending[id]
	expired.expiresAt = time.Now().Add(-time.Second)
	a.pending[id] = expired
	a.mu.Unlock()

	_, ok := a.redeemLogin(id, p.state)
	require.False(t, ok)
}

func TestAuth_RedeemLogin_EmptyState_Fails(t *testing.T) {
	a := newAuth()
	id, _, err := a.beginLogin()
	require.NoError(t, err)

	_, ok := a.redeemLogin(id, "")
	require.False(t, ok)
}

func TestAuth_ValidSession_NoCookie_Fails(t *testing.T) {
	a := newAuth()

	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	require.False(t, a.validSession(r))
}

func TestAuth_ValidSession_WrongCookie_Fails(t *testing.T) {
	a := newAuth()

	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "foreign-token"})

	require.False(t, a.validSession(r))
}

func TestAuth_CreateSession_ThenValidSession_Succeeds(t *testing.T) {
	a := newAuth()

	cookieValue, _, err := a.createSession("admin@example.com")
	require.NoError(t, err)

	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: cookieValue})

	require.True(t, a.validSession(r))
}

func TestAuth_CreateSession_MultipleConcurrentSessions(t *testing.T) {
	a := newAuth()

	cookie1, csrf1, err := a.createSession("admin1@example.com")
	require.NoError(t, err)
	cookie2, csrf2, err := a.createSession("admin2@example.com")
	require.NoError(t, err)

	require.NotEqual(t, cookie1, cookie2)
	require.NotEqual(t, csrf1, csrf2)

	r1 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	r1.AddCookie(&http.Cookie{Name: sessionCookieName, Value: cookie1})
	require.True(t, a.validSession(r1))

	r2 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	r2.AddCookie(&http.Cookie{Name: sessionCookieName, Value: cookie2})
	require.True(t, a.validSession(r2))
}

func TestAuth_EndSession_RemovesSession(t *testing.T) {
	a := newAuth()
	cookieValue, _, err := a.createSession("admin@example.com")
	require.NoError(t, err)

	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: cookieValue})

	a.endSession(r)

	require.False(t, a.validSession(r))
}

func TestSetSessionCookie_IsHttpOnlySecureAndLax(t *testing.T) {
	rec := httptest.NewRecorder()
	setSessionCookie(rec, "value")

	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()

	cookies := resp.Cookies()
	require.Len(t, cookies, 1)
	require.True(t, cookies[0].HttpOnly)
	require.True(t, cookies[0].Secure)
	require.Equal(t, http.SameSiteLaxMode, cookies[0].SameSite)
}

func TestNewAuth_CreateSession_GeneratesDistinctValues(t *testing.T) {
	a := newAuth()

	cookie1, csrf1, err := a.createSession("a@example.com")
	require.NoError(t, err)
	cookie2, csrf2, err := a.createSession("b@example.com")
	require.NoError(t, err)

	require.NotEqual(t, cookie1, cookie2)
	require.NotEqual(t, csrf1, csrf2)
	require.NotEqual(t, cookie1, csrf1)
}
