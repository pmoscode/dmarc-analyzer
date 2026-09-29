package imap

import (
	"context"
	"fmt"
	"time"
)

// maxAttempts implements IMPLEMENTIERUNG.md section 7.2: "on transient
// IMAP errors, exponentially staggered retry (3 attempts), then a clean
// abort with an understandable message."
const maxAttempts = 3

// backoffDelays are the wait times before the 2nd and 3rd attempt
// (exponential). A fixed, small table instead of a bit shift on the
// attempt number — more readable with only three attempts, and without any
// integer-conversion questions.
var backoffDelays = [maxAttempts - 1]time.Duration{
	200 * time.Millisecond,
	400 * time.Millisecond,
}

// retry runs fn up to maxAttempts times, with exponentially increasing wait
// time between attempts. Aborts immediately once ctx is done — even during
// the wait between two attempts.
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
