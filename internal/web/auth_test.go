package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAuth_IssueAndRedeemCode_Success(t *testing.T) {
	a, err := newAuth()
	require.NoError(t, err)

	code, err := a.issueCode()
	require.NoError(t, err)
	require.NotEmpty(t, code)

	require.True(t, a.redeemCode(code))
}

func TestAuth_RedeemCode_WrongCode_Fails(t *testing.T) {
	a, err := newAuth()
	require.NoError(t, err)

	_, err = a.issueCode()
	require.NoError(t, err)

	require.False(t, a.redeemCode("definitiv-falscher-code"))
}

func TestAuth_RedeemCode_IsSingleUse(t *testing.T) {
	a, err := newAuth()
	require.NoError(t, err)

	code, err := a.issueCode()
	require.NoError(t, err)

	require.True(t, a.redeemCode(code), "erster Versuch muss gelingen")
	require.False(t, a.redeemCode(code), "zweiter Versuch mit demselben Code muss scheitern")
}

func TestAuth_RedeemCode_ExpiredCode_Fails(t *testing.T) {
	a, err := newAuth()
	require.NoError(t, err)

	code, err := a.issueCode()
	require.NoError(t, err)

	// Ablaufzeit künstlich in die Vergangenheit setzen, statt echte 60s
	// im Test zu warten.
	a.mu.Lock()
	a.codeExpires = time.Now().Add(-time.Second)
	a.mu.Unlock()

	require.False(t, a.redeemCode(code))
}

func TestAuth_IssueCode_InvalidatesPreviousCode(t *testing.T) {
	a, err := newAuth()
	require.NoError(t, err)

	first, err := a.issueCode()
	require.NoError(t, err)

	_, err = a.issueCode()
	require.NoError(t, err)

	require.False(t, a.redeemCode(first), "ein neu ausgestellter Code muss den vorherigen ungültig machen")
}

func TestAuth_RedeemCode_EmptyCode_Fails(t *testing.T) {
	a, err := newAuth()
	require.NoError(t, err)

	_, err = a.issueCode()
	require.NoError(t, err)

	require.False(t, a.redeemCode(""))
}

func TestAuth_ValidSession_NoCookie_Fails(t *testing.T) {
	a, err := newAuth()
	require.NoError(t, err)

	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	require.False(t, a.validSession(r))
}

func TestAuth_ValidSession_WrongCookie_Fails(t *testing.T) {
	a, err := newAuth()
	require.NoError(t, err)

	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "fremdes-token"})

	require.False(t, a.validSession(r))
}

func TestAuth_SetSessionCookie_ThenValidSession_Succeeds(t *testing.T) {
	a, err := newAuth()
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	a.setSessionCookie(rec)

	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()

	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	for _, c := range resp.Cookies() {
		r.AddCookie(c)
	}

	require.True(t, a.validSession(r))
}

func TestAuth_SetSessionCookie_IsHttpOnlyAndStrict(t *testing.T) {
	a, err := newAuth()
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	a.setSessionCookie(rec)

	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()

	cookies := resp.Cookies()
	require.Len(t, cookies, 1)
	require.True(t, cookies[0].HttpOnly)
	require.Equal(t, http.SameSiteStrictMode, cookies[0].SameSite)
}

func TestNewAuth_GeneratesDistinctSecretsPerInstance(t *testing.T) {
	a1, err := newAuth()
	require.NoError(t, err)
	a2, err := newAuth()
	require.NoError(t, err)

	require.NotEqual(t, a1.instanceSecret, a2.instanceSecret)
	require.NotEqual(t, a1.sessionToken, a2.sessionToken)
	require.NotEqual(t, a1.instanceSecret, a1.sessionToken)
}
