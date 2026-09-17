package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pmoscode/dmarc-analyzer/internal/web"
)

// runWeb startet die eingebettete Web-Oberfläche (siehe MIGRATIONSPLAN.md).
// Lebenszyklus bewusst nur über Strg+C/SIGTERM (Entscheidung E-3, siehe
// dort): kein Auto-Ende, kein Beenden-Knopf — der Prozess soll sich wie
// ein gewöhnlicher Server-Dienst verhalten (auch mit Blick auf einen
// späteren Docker-Betrieb).
func runWeb(ctx context.Context, a *app, args []string) error {
	fs := flag.NewFlagSet("web", flag.ContinueOnError)
	addr := fs.String("adresse", "", "feste Adresse (z. B. 127.0.0.1:8080) — leer: zufälliger freier Port")
	noBrowser := fs.Bool("kein-browser", false, "Browser nicht automatisch öffnen")
	dev := fs.Bool("entwicklung", false, "Vorlagen/Statik von der Festplatte laden statt eingebettet (Arbeitsverzeichnis muss die Repository-Wurzel sein)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Einzelinstanz-Erkennung (MIGRATIONSPLAN.md Abschnitt 3/Meilenstein
	// M1): läuft bereits ein Server, holt sich dieser zweite Aufruf nur
	// einen frischen Anmeldelink und beendet sich sofort, statt einen
	// weiteren Server zu starten. Ein Fehler hier bedeutet meist einen
	// verwaisten instance.json-Eintrag (Prozess tot) — dann normal
	// weiterstarten, s. web.New()/writeInstanceFile() überschreibt ihn.
	if inst, ok := web.FindRunningInstance(); ok {
		if loginURL, err := web.RequestLoginURL(ctx, inst); err == nil {
			fmt.Printf("dmarc-analyzer läuft bereits: %s\n", loginURL)
			if !*noBrowser {
				if err := web.OpenBrowser(loginURL); err != nil {
					fmt.Fprintf(os.Stderr, "Browser konnte nicht automatisch geöffnet werden — bitte den Anmeldelink von Hand öffnen: %v\n", err)
				}
			}
			return nil
		}
	}

	srv, err := web.New(web.Dependencies{
		Statistics: a.stats,
		Reports:    a.queries,
		Sources:    a.sourceStats,
	}, web.Options{Addr: *addr, Dev: *dev})
	if err != nil {
		return fmt.Errorf("web-oberfläche konnte nicht aufgebaut werden: %w", err)
	}

	// Eigener, auf diesen Aufruf begrenzter signal-Kontext statt den
	// übergebenen ctx global umzustellen — sync/import/stats/account
	// sollen von dieser Änderung im Lebenszyklus unberührt bleiben (siehe
	// AGENTS.md: Composition Root verdrahtet nur, was der jeweilige
	// Unterbefehl tatsächlich braucht).
	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	loginURL, err := srv.Start(signalCtx)
	if err != nil {
		return fmt.Errorf("web-oberfläche konnte nicht gestartet werden: %w", err)
	}

	fmt.Printf("dmarc-analyzer läuft: http://%s\n", srv.Addr())
	fmt.Printf("Anmeldelink (60 s gültig): %s\n", loginURL)
	fmt.Println("Strg+C zum Beenden.")

	if !*noBrowser {
		if err := web.OpenBrowser(loginURL); err != nil {
			fmt.Fprintf(os.Stderr, "Browser konnte nicht automatisch geöffnet werden — bitte den Anmeldelink von Hand öffnen: %v\n", err)
		}
	}

	<-signalCtx.Done()
	fmt.Println("\nWird beendet …")

	return srv.Shutdown(context.Background())
}
