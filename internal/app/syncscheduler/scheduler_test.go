package syncscheduler_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/syncjob"
	"github.com/pmoscode/dmarc-analyzer/internal/app/syncscheduler"
)

type fakeJob struct {
	starts int32
	err    error
}

func (f *fakeJob) Start() error {
	atomic.AddInt32(&f.starts, 1)
	return f.err
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRun_IntervalDisabled_NeverStartsAndReturnsImmediately(t *testing.T) {
	job := &fakeJob{}
	sched := syncscheduler.NewScheduler(job, 0, silentLogger())

	done := make(chan struct{})
	go func() {
		sched.Run(context.Background())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run() mit interval<=0 hätte sofort zurückkehren müssen")
	}
	require.Equal(t, int32(0), atomic.LoadInt32(&job.starts))
}

func TestRun_TriggersImmediatelyOnStart(t *testing.T) {
	job := &fakeJob{}
	sched := syncscheduler.NewScheduler(job, time.Hour, silentLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	sched.Run(ctx)

	require.Equal(t, int32(1), atomic.LoadInt32(&job.starts))
}

func TestRun_TriggersAgainAfterEachInterval(t *testing.T) {
	job := &fakeJob{}
	sched := syncscheduler.NewScheduler(job, 10*time.Millisecond, silentLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Millisecond)
	defer cancel()

	sched.Run(ctx)

	require.GreaterOrEqual(t, atomic.LoadInt32(&job.starts), int32(3))
}

func TestRun_AlreadyRunningError_IsNotLoggedAsFailure(t *testing.T) {
	job := &fakeJob{err: syncjob.ErrAlreadyRunning}
	sched := syncscheduler.NewScheduler(job, 10*time.Millisecond, silentLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Millisecond)
	defer cancel()

	sched.Run(ctx)

	require.GreaterOrEqual(t, atomic.LoadInt32(&job.starts), int32(2))
}

func TestRun_OtherStartError_KeepsRunning(t *testing.T) {
	job := &fakeJob{err: errors.New("kaputt")}
	sched := syncscheduler.NewScheduler(job, 10*time.Millisecond, silentLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Millisecond)
	defer cancel()

	sched.Run(ctx)

	require.GreaterOrEqual(t, atomic.LoadInt32(&job.starts), int32(2))
}

func TestRun_StopsWhenContextCancelled(t *testing.T) {
	job := &fakeJob{}
	sched := syncscheduler.NewScheduler(job, time.Hour, silentLogger())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		sched.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run() ist nach Kontextabbruch nicht zurückgekehrt")
	}
}
