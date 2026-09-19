package web

import (
	"log/slog"
	"net/http"
	"net/url"
)

// requireHost lehnt Anfragen ab, deren Host-Header nicht exakt dem
// öffentlichen Hostnamen entspricht (aus DMARC_OIDC_REDIRECT_URL, siehe
// server.go Server.allowedHost) — Schutz gegen Host-Header-Fälschung
// (z. B. wenn der Container versehentlich auf mehreren Netzwerken lauscht).
// Der Reverse-Proxy vor dem Container reicht den externen Host-Header
// üblicherweise unverändert durch.
func requireHost(allowedHost string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != allowedHost {
			http.Error(w, "ungültiger Host", http.StatusMisdirectedRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// securityHeaders setzt CSP und weitere Sicherheits-Header auf jede
// Antwort (MIGRATIONSPLAN.md Abschnitt 5). Kein Inline-JavaScript, keine
// externen Quellen — alles (htmx, Chart.js, eigenes JS) kommt aus
// /static/, eingebettet in die Binärdatei.
//
// form-action erlaubt neben 'self' zusätzlich die Origin des OIDC-Issuers
// (s.oidcIssuerOrigin): /abmelden sendet die Formular-Antwort per 303 zu
// Authentiks end_session_endpoint weiter (RP-Initiated Logout,
// internal/web/oidc.go:endSessionURL) — das ist zwangsläufig eine andere
// Origin. Ohne diese Erweiterung blockiert der Browser genau diesen
// Redirect ("violates ... form-action 'self'"), die lokale Sitzung ist
// dann zwar beendet, aber Authentik bekommt den Logout nie mitgeteilt und
// die nächste Anfrage (GET /anmelden leitet ohne Zwischenschritt sofort zu
// Authentik weiter, siehe handlers_login.go) meldet über die weiterhin
// aktive Authentik-Session sofort wieder an — Abmelden wirkt dadurch
// komplett wirkungslos (verifiziert 2026-09-19, Chrome-DevTools-Konsole).
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

// requireSession leitet Anfragen ohne gültiges Sitzungs-Cookie auf
// /anmelden um — die Ausnahmen (/anmelden, /anmelden/callback, /gesund)
// werden in routes.go bewusst außerhalb dieser Middleware verdrahtet,
// nicht durch eine Sonderregel hier drin.
func requireSession(a *auth, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.validSession(r) {
			http.Redirect(w, r, "/anmelden", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// recoverPanic fängt Panics in Handlern ab, damit ein einzelner defekter
// Request nicht den ganzen Serverprozess beendet.
func recoverPanic(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("panic im handler", "error", rec, "path", r.URL.Path)
				http.Error(w, "interner Fehler", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// csrfTokenHeader und csrfTokenField sind die zwei Stellen, an denen ein
// zustandsänderndes Formular sein CSRF-Token mitgeben kann — Header für
// htmx-Anfragen (per hx-headers), Formularfeld für ein gewöhnliches
// <form method="post"> (MIGRATIONSPLAN.md Abschnitt 5: "CSRF-Token
// (Formularfeld bzw. hx-headers)").
const (
	//nolint:gosec // G101: kein Geheimnis, nur der Header-/Feldname, in
	// dem ein CSRF-Token übertragen wird — der Wert selbst kommt nie aus
	// dieser Konstante.
	csrfTokenHeader = "X-CSRF-Token"
	csrfTokenField  = "csrf_token"
)

// safeMethods sind Methoden, die laut HTTP-Semantik keinen Zustand
// ändern — requireCSRF lässt sie ungeprüft durch, wie es requireCSRF
// überall dort tun muss, wo GET-Anfragen (Seitenaufrufe, /api/diagramme/*)
// denselben Handler-Baum durchlaufen wie künftige POST-Formulare.
func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

// requireCSRF schützt zustandsändernde Anfragen (POST/PUT/PATCH/DELETE)
// zusätzlich zu SameSite=Strict (MIGRATIONSPLAN.md Abschnitt 5: "CSRF").
// Zwei unabhängige Prüfungen müssen beide bestehen:
//
//  1. sameOrigin(r): Origin- bzw. ersatzweise Sec-Fetch-Site-Header
//     müssen denselben Origin wie r.Host ausweisen.
//  2. Ein gültiges CSRF-Token (Header oder Formularfeld, siehe oben),
//     das mit dem Instanz-Sitzungs-Token übereinstimmt.
//
// Aktuell (M1) registriert kein Handler eine zustandsändernde Route unter
// dem protected-Baum — die Middleware ist bereits vollständig verdrahtet
// und getestet, damit spätere Formulare (Konten, Sync, Import ab M3) sie
// nur noch nutzen, nicht mehr selbst bauen müssen.
func requireCSRF(a *auth, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		if !sameOrigin(r) {
			http.Error(w, "ungültiger Origin", http.StatusForbidden)
			return
		}

		token := r.Header.Get(csrfTokenHeader)
		if token == "" {
			token = r.PostFormValue(csrfTokenField)
		}
		if !a.validCSRFToken(r, token) {
			http.Error(w, "ungültiges oder fehlendes CSRF-Token", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// sameOrigin prüft Origin bzw. ersatzweise Sec-Fetch-Site gegen r.Host.
// Fehlen beide Header, wird sicherheitshalber abgelehnt — jeder Browser,
// der neu genug für fetch()/htmx ist, sendet mindestens einen der beiden
// bei einer POST-Anfrage.
//
// Origin "null" zählt dabei wie ein fehlender Header: Browser senden
// diesen wörtlichen Wert statt echter Origin für normale (nicht per
// fetch/htmx ausgelöste) Formular-POSTs auf Seiten mit
// Referrer-Policy: no-referrer (siehe securityHeaders) — reproduzierbar
// beim "Abgleich starten"-Formular. Sec-Fetch-Site bleibt in dem Fall
// zuverlässig, weil der Browser es unabhängig von der Referrer-Policy
// aus dem tatsächlichen Navigationskontext setzt, nicht aus einem von
// der Seite beeinflussbaren Wert.
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
