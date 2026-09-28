package syncreports

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProgressTracker_InOrderCompletion(t *testing.T) {
	t.Parallel()

	p := newProgressTracker(0)

	last, advanced := p.markDone(1)
	require.True(t, advanced)
	require.Equal(t, uint32(1), last)

	last, advanced = p.markDone(2)
	require.True(t, advanced)
	require.Equal(t, uint32(2), last)
}

func TestProgressTracker_OutOfOrderCompletion_OnlyAdvancesContiguously(t *testing.T) {
	t.Parallel()

	p := newProgressTracker(0)

	// UID 3 arrives before UID 1 and 2 (faster worker) — progress must
	// still not advance past the gap.
	last, advanced := p.markDone(3)
	require.False(t, advanced)
	require.Equal(t, uint32(0), last)

	last, advanced = p.markDone(1)
	require.True(t, advanced)
	require.Equal(t, uint32(1), last)

	// UID 2 is still missing — progress stays at 1, even though 3 is already there.
	last, advanced = p.markDone(2)
	require.True(t, advanced)
	require.Equal(t, uint32(3), last, "with UID 2, the gap closes up to the already-waiting UID 3")
}

func TestProgressTracker_StartsAfterGivenBaseline(t *testing.T) {
	t.Parallel()

	p := newProgressTracker(100)

	last, advanced := p.markDone(101)
	require.True(t, advanced)
	require.Equal(t, uint32(101), last)
}

func TestProgressTracker_DuplicateMarkDone_IsHarmless(t *testing.T) {
	t.Parallel()

	p := newProgressTracker(0)
	p.markDone(1)

	last, advanced := p.markDone(1)
	require.False(t, advanced)
	require.Equal(t, uint32(1), last)
}
