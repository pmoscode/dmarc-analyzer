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
	// fnErr is published via close(done), which happens-before every
	// receive from done — no extra synchronization needed. A struct{}
	// channel (instead of chan error) keeps errcheck from flagging the
	// discarding receive in the cancellation branch.
	var fnErr error
	done := make(chan struct{})
	go func() {
		fnErr = fn()
		close(done)
	}()

	select {
	case <-done:
		return fnErr
	case <-ctx.Done():
		_ = closer.Close()
		<-done // fn must return promptly after the Close; its error is superseded by ctx.Err()
		return ctx.Err()
	}
}
