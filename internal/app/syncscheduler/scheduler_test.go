package syncscheduler_test

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/syncscheduler"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/settings"
)

type fakeSettingsRepository struct {
	mu sync.Mutex
	s  settings.Settings
}

func (f *fakeSettingsRepository) Load(context.Context) (settings.Settings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.s, nil
}

func (f *fakeSettingsRepository) Save(_ context.Context, s settings.Settings) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.s = s
	return nil
}

type fakeJob struct {
	mu       sync.Mutex
	starts   int32
	running  bool
	lastDone time.Time
}

func (f *fakeJob) Start() error {
	atomic.AddInt32(&f.starts, 1)
	return nil
}

func (f *fakeJob) Snapshot() syncscheduler.Snapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return syncscheduler.Snapshot{Running: f.running, LastActivity: f.lastDone}
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestTick_IntervalDisabled_NeverStarts(t *testing.T) {
	settingsRepo := &fakeSettingsRepository{s: settings.Settings{SyncIntervalMinutes: 0}}
	job := &fakeJob{}
	sched := syncscheduler.NewScheduler(settingsRepo, job, 10*time.Millisecond, silentLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Millisecond)
	defer cancel()

	sched.Run(ctx)

	require.Equal(t, int32(0), atomic.LoadInt32(&job.starts))
}

func TestTick_NeverSyncedBefore_StartsOnFirstDueTick(t *testing.T) {
	settingsRepo := &fakeSettingsRepository{s: settings.Settings{SyncIntervalMinutes: 60}}
	job := &fakeJob{}
	sched := syncscheduler.NewScheduler(settingsRepo, job, 10*time.Millisecond, silentLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	sched.Run(ctx)

	require.GreaterOrEqual(t, atomic.LoadInt32(&job.starts), int32(1))
}

func TestTick_AlreadyRunning_DoesNotStartAgain(t *testing.T) {
	settingsRepo := &fakeSettingsRepository{s: settings.Settings{SyncIntervalMinutes: 60}}
	job := &fakeJob{running: true}
	sched := syncscheduler.NewScheduler(settingsRepo, job, 10*time.Millisecond, silentLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Millisecond)
	defer cancel()

	sched.Run(ctx)

	require.Equal(t, int32(0), atomic.LoadInt32(&job.starts))
}

func TestTick_RecentlySynced_WaitsUntilIntervalElapsed(t *testing.T) {
	settingsRepo := &fakeSettingsRepository{s: settings.Settings{SyncIntervalMinutes: 60}}
	job := &fakeJob{lastDone: time.Now()}
	sched := syncscheduler.NewScheduler(settingsRepo, job, 10*time.Millisecond, silentLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Millisecond)
	defer cancel()

	sched.Run(ctx)

	require.Equal(t, int32(0), atomic.LoadInt32(&job.starts))
}

func TestTick_IntervalElapsed_StartsAgain(t *testing.T) {
	settingsRepo := &fakeSettingsRepository{s: settings.Settings{SyncIntervalMinutes: 60}}
	job := &fakeJob{lastDone: time.Now().Add(-2 * time.Hour)}
	sched := syncscheduler.NewScheduler(settingsRepo, job, 10*time.Millisecond, silentLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	sched.Run(ctx)

	require.GreaterOrEqual(t, atomic.LoadInt32(&job.starts), int32(1))
}

func TestRun_StopsWhenContextCancelled(t *testing.T) {
	settingsRepo := &fakeSettingsRepository{s: settings.Settings{SyncIntervalMinutes: 60}}
	job := &fakeJob{}
	sched := syncscheduler.NewScheduler(settingsRepo, job, time.Hour, silentLogger())
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
