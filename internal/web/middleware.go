package web

import (
	"log/slog"
	"net/http"
	"net/url"
)

// requireHost lehnt Anfragen ab, deren Host-Header nicht exakt einer der
// erlaubten Adressen entspricht — Schutz vor DNS-Rebinding: eine fremde
// Webseite kann per JavaScript zwar eine Anfrage an 127.0.0.1 auslösen,
// aber (ohne DNS-Rebinding) keinen Host-Header setzen, der von der
// Browser-URL abweicht (MIGRATIONSPLAN.md Abschnitt 5).
//
// allowedHosts ist eine Funktion statt einer festen map: routes() baut
// den Handler-Baum bereits in New(), bevor bind() den tatsächlichen Port
// kennt — die erlaubten Hosts werden deshalb bei jeder Anfrage neu aus
// der inzwischen gebundenen Adresse berechnet (s.allowedHosts in
// server.go), nicht einmalig bei der Konstruktion eingefroren.
func requireHost(allowedHosts func() map[string]bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedHosts()[r.Host] {
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
const contentSecurityPolicy = "default-src 'self'; frame-ancestors 'none'; form-action 'self'; base-uri 'none'"

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// requireSession lehnt Anfragen ohne gültiges Sitzungs-Cookie mit 401 ab —
// die einzige Ausnahme (/anmelden) wird in routes.go bewusst außerhalb
// dieser Middleware verdrahtet, nicht durch eine Sonderregel hier drin.
func requireSession(a *auth, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.validSession(r) {
			http.Error(w, "Bitte über das Programm öffnen (dmarc-analyzer web).", http.StatusUnauthorized)
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
		if !a.validCSRFToken(token) {
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
func sameOrigin(r *http.Request) bool {
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		return err == nil && u.Host == r.Host
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" {
		return site == "same-origin" || site == "none"
	}
	return false
}
