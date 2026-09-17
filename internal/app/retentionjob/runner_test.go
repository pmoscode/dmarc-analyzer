package retentionjob_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/retentionjob"
)

type fakeApplier struct {
	calls int32
	err   error
}

func (f *fakeApplier) ApplyNow(context.Context) (int64, error) {
	atomic.AddInt32(&f.calls, 1)
	return 0, f.err
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRun_AppliesImmediatelyOnStart(t *testing.T) {
	applier := &fakeApplier{}
	r := retentionjob.NewRunner(applier, time.Hour, silentLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	r.Run(ctx)

	require.Equal(t, int32(1), atomic.LoadInt32(&applier.calls))
}

func TestRun_AppliesAgainAfterEachInterval(t *testing.T) {
	applier := &fakeApplier{}
	r := retentionjob.NewRunner(applier, 10*time.Millisecond, silentLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Millisecond)
	defer cancel()

	r.Run(ctx)

	require.GreaterOrEqual(t, atomic.LoadInt32(&applier.calls), int32(3))
}

func TestRun_StopsWhenContextCancelled(t *testing.T) {
	applier := &fakeApplier{}
	r := retentionjob.NewRunner(applier, time.Hour, silentLogger())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		r.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run() ist nach Kontextabbruch nicht zurückgekehrt")
	}
}

func TestRun_LogsErrorButKeepsRunning(t *testing.T) {
	applier := &fakeApplier{err: errors.New("kaputt")}
	r := retentionjob.NewRunner(applier, 10*time.Millisecond, silentLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Millisecond)
	defer cancel()

	r.Run(ctx)

	require.GreaterOrEqual(t, atomic.LoadInt32(&applier.calls), int32(2))
}
