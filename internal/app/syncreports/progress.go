package syncreports

// progressTracker turns possibly out-of-order completed UIDs (a result of
// concurrent processing, IMPLEMENTIERUNG.md section 7.3) into a gapless
// progress boundary: LastUID only advances as long as no smaller UID is
// still open. This way, a crash loses at most the messages currently
// being processed, never one already saved (IMPLEMENTIERUNG.md section
// 7.1, step 6).
//
// Not concurrency-safe — called exclusively by the single writer
// goroutine in usecase.go, which consumes results serially via a channel
// anyway.
type progressTracker struct {
	lastUID   uint32
	completed map[uint32]struct{}
}

func newProgressTracker(startAfter uint32) *progressTracker {
	return &progressTracker{lastUID: startAfter, completed: make(map[uint32]struct{})}
}

// markDone marks uid as completed (either successfully saved OR
// definitively moved to the error quarantine — both count as "done" in
// terms of progress) and returns the new gapless upper bound, plus
// whether it changed compared to the last call.
func (p *progressTracker) markDone(uid uint32) (lastUID uint32, advanced bool) {
	p.completed[uid] = struct{}{}

	for {
		next := p.lastUID + 1
		if _, ok := p.completed[next]; !ok {
			break
		}
		delete(p.completed, next)
		p.lastUID = next
		advanced = true
	}

	return p.lastUID, advanced
}
