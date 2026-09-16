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

	// UID 3 kommt vor UID 1 und 2 an (schnellerer Worker) — der Fortschritt
	// darf trotzdem nicht über die Lücke hinaus vorrücken.
	last, advanced := p.markDone(3)
	require.False(t, advanced)
	require.Equal(t, uint32(0), last)

	last, advanced = p.markDone(1)
	require.True(t, advanced)
	require.Equal(t, uint32(1), last)

	// UID 2 fehlt weiterhin — Fortschritt bleibt bei 1, obwohl 3 schon da ist.
	last, advanced = p.markDone(2)
	require.True(t, advanced)
	require.Equal(t, uint32(3), last, "mit UID 2 schließt sich die Lücke bis zur bereits wartenden UID 3")
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
