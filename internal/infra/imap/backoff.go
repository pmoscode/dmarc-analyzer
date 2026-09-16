package imap

import (
	"context"
	"fmt"
	"time"
)

// maxAttempts setzt IMPLEMENTIERUNG.md Abschnitt 7.2 um: "bei temporären
// IMAP-Fehlern exponentiell gestaffelte Wiederholung (3 Versuche), danach
// sauberer Abbruch mit verständlicher Meldung."
const maxAttempts = 3

// backoffDelays sind die Wartezeiten vor dem 2. und 3. Versuch
// (exponentiell). Eine feste, kleine Tabelle statt eines Bit-Shifts auf
// der Versuchsnummer — bei nur drei Versuchen lesbarer und ohne jede
// Ganzzahl-Konvertierungsfrage.
var backoffDelays = [maxAttempts - 1]time.Duration{
	200 * time.Millisecond,
	400 * time.Millisecond,
}

// retry führt fn bis zu maxAttempts Mal aus, mit exponentiell steigender
// Wartezeit zwischen den Versuchen. Bricht sofort ab, wenn ctx erledigt
// ist — auch während der Wartezeit zwischen zwei Versuchen.
func retry(ctx context.Context, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(backoffDelays[attempt-1]):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return fmt.Errorf("nach %d versuchen fehlgeschlagen: %w", maxAttempts, lastErr)
}
