package imap

import (
	"context"
	"io"
)

// runCtx runs fn in a goroutine, making the call context-aware: if ctx is
// cancelled, closer is closed (which causes any currently blocking
// read/write in fn to return with an error) and ctx.Err() is returned
// instead of waiting for fn (IMPLEMENTIERUNG.md section 7.2: "every step
// honors context.Context"). Still waits for fn to return afterward, so no
// goroutine is left hanging.
func runCtx(ctx context.Context, closer io.Closer, fn func() error) error {
	done := make(chan error, 1)
	go func() { done <- fn() }()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		_ = closer.Close()
		<-done // fn must return promptly after the Close
		return ctx.Err()
	}
}
