package web

import (
	"log/slog"
	"net/http"
	"net/url"
)

// requireHost rejects requests whose Host header doesn't exactly match
// the public hostname (from DMARC_OIDC_REDIRECT_URL, see server.go
// Server.allowedHost) — protection against Host header spoofing (e.g. if
// the container accidentally listens on multiple networks). The reverse
// proxy in front of the container usually passes the external Host
// header through unchanged.
func requireHost(allowedHost string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != allowedHost {
			http.Error(w, "invalid host", http.StatusMisdirectedRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// securityHeaders sets CSP and other security headers on every response
// (MIGRATIONSPLAN.md section 5). No inline JavaScript, no external
// sources — everything (htmx, Chart.js, our own JS) comes from /static/,
// embedded in the binary.
//
// form-action allows the OIDC issuer's origin (s.oidcIssuerOrigin) in
// addition to 'self': /abmelden forwards the form response via 303 to
// Authentik's end_session_endpoint (RP-initiated logout,
// internal/web/oidc.go:endSessionURL) — that's necessarily a different
// origin. Without this extension the browser blocks exactly that
// redirect ("violates ... form-action 'self'"); the local session is then
// ended, but Authentik never learns about the logout, and the next
// request (GET /anmelden immediately redirects to Authentik without an
// intermediate step, see handlers_login.go) logs back in right away via
// the still-active Authentik session — logout ends up having no effect
// at all (verified 2026-09-19, Chrome DevTools console).
func buildContentSecurityPolicy(oidcIssuerOrigin string) string {
	formAction := "form-action 'self'"
	if oidcIssuerOrigin != "" {
		formAction += " " + oidcIssuerOrigin
	}
	return "default-src 'self'; frame-ancestors 'none'; " + formAction + "; base-uri 'none'"
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	csp := buildContentSecurityPolicy(s.oidcIssuerOrigin)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// requireSession redirects requests without a valid session cookie to
// /anmelden — the exceptions (/anmelden, /anmelden/callback, /gesund) are
// deliberately wired up in routes.go outside this middleware, not via a
// special case in here.
func requireSession(a *auth, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.validSession(r) {
			http.Redirect(w, r, "/anmelden", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// recoverPanic catches panics in handlers so a single broken request
// doesn't take down the entire server process.
func recoverPanic(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("panic in handler", "error", rec, "path", r.URL.Path)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// csrfTokenHeader and csrfTokenField are the two places a state-changing
// form can carry its CSRF token — header for htmx requests (via
// hx-headers), form field for an ordinary <form method="post">
// (MIGRATIONSPLAN.md section 5: "CSRF token (form field or hx-headers)").
const (
	//nolint:gosec // G101: not a secret, just the header/field name a
	// CSRF token is transmitted in — the value itself never comes from
	// this constant.
	csrfTokenHeader = "X-CSRF-Token"
	csrfTokenField  = "csrf_token"
)

// safeMethods are methods that, per HTTP semantics, don't change state —
// requireCSRF lets them through unchecked, as it must everywhere GET
// requests (page views, /api/diagramme/*) run through the same handler
// tree as future POST forms.
func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

// requireCSRF protects state-changing requests (POST/PUT/PATCH/DELETE) in
// addition to SameSite=Strict (MIGRATIONSPLAN.md section 5: "CSRF"). Two
// independent checks must both pass:
//
//  1. sameOrigin(r): the Origin header, or Sec-Fetch-Site as a fallback,
//     must show the same origin as r.Host.
//  2. A valid CSRF token (header or form field, see above) that matches
//     the instance's session token.
//
// Currently (M1) no handler registers a state-changing route under the
// protected tree — the middleware is already fully wired up and tested,
// so later forms (accounts, sync, import from M3 on) only need to use it,
// not build it themselves.
func requireCSRF(a *auth, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		if !sameOrigin(r) {
			http.Error(w, "invalid origin", http.StatusForbidden)
			return
		}

		token := r.Header.Get(csrfTokenHeader)
		if token == "" {
			token = r.PostFormValue(csrfTokenField)
		}
		if !a.validCSRFToken(r, token) {
			http.Error(w, "invalid or missing CSRF token", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// sameOrigin checks Origin, or Sec-Fetch-Site as a fallback, against
// r.Host. If both headers are missing, it rejects to be safe — any
// browser new enough for fetch()/htmx sends at least one of the two on a
// POST request.
//
// An Origin of "null" counts as a missing header here: browsers send this
// literal value instead of a real origin for ordinary (not
// fetch/htmx-triggered) form POSTs on pages with Referrer-Policy:
// no-referrer (see securityHeaders) — reproducible with the "Start sync"
// form. Sec-Fetch-Site stays reliable in that case, because the browser
// sets it from the actual navigation context regardless of the referrer
// policy, not from a value the page can influence.
func sameOrigin(r *http.Request) bool {
	if origin := r.Header.Get("Origin"); origin != "" && origin != "null" {
		u, err := url.Parse(origin)
		return err == nil && u.Host == r.Host
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" {
		return site == "same-origin" || site == "none"
	}
	return false
}
