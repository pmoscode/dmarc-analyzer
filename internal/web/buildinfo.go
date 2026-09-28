package web

// shortCommitLen is the length the commit hash is truncated to in the
// UI — the same length "git log --oneline" typically uses.
const shortCommitLen = 7

// BuildInfo describes the running build (version and git commit) for
// display below the wordmark in the header (layout.html). Both are
// determined in the composition root (cmd/dmarc-analyzer/version.go); the
// web package only displays them.
type BuildInfo struct {
	// Version is e.g. a tag ("v1.2.0") or "dev".
	Version string
	// Commit is the full git commit hash, empty if unknown.
	Commit string
}

// ShortCommit returns the commit hash truncated to shortCommitLen. A
// suffix like "-dirty" (see cmd/dmarc-analyzer/version.go) is preserved,
// so a build from a modified working directory stays recognizable in the
// UI.
func (b BuildInfo) ShortCommit() string {
	hash, suffix := b.Commit, ""
	for i, r := range b.Commit {
		if r == '-' {
			hash, suffix = b.Commit[:i], b.Commit[i:]
			break
		}
	}
	if len(hash) > shortCommitLen {
		hash = hash[:shortCommitLen]
	}
	return hash + suffix
}
