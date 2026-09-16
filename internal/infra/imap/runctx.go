package imap

import (
	"context"
	"io"
)

// runCtx führt fn in einer Goroutine aus und macht den Aufruf dadurch
// context-fähig: bricht ctx ab, wird closer geschlossen (das lässt jeden
// gerade blockierenden Lese-/Schreibvorgang in fn mit einem Fehler
// zurückkehren) und ctx.Err() geliefert, statt auf fn zu warten
// (IMPLEMENTIERUNG.md Abschnitt 7.2: "jeder Schritt respektiert
// context.Context"). Wartet danach trotzdem auf die Rückkehr von fn, damit
// keine Goroutine hängen bleibt.
func runCtx(ctx context.Context, closer io.Closer, fn func() error) error {
	done := make(chan error, 1)
	go func() { done <- fn() }()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		_ = closer.Close()
		<-done // fn muss nach dem Close zeitnah zurückkehren
		return ctx.Err()
	}
}
