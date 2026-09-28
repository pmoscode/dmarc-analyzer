package main

import "runtime/debug"

// resolvedCommit returns the git commit of the running build: prefers
// the value set via -ldflags (commit in main.go), otherwise the revision
// embedded by "go build" itself (vcs.revision, only present when built
// from a git checkout — in the Docker build .git is missing, see
// .dockerignore, hence the COMMIT build arg there). A build from a
// modified working directory gets the "-dirty" suffix. Empty if none of
// that is available (e.g. "go run", "go test").
func resolvedCommit() string {
	if commit != "" {
		return commit
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return commitFromBuildSettings(info.Settings)
}

// commitFromBuildSettings reads vcs.revision/vcs.modified from the build
// settings — kept separate from resolvedCommit so it can be tested
// without real build information.
func commitFromBuildSettings(settings []debug.BuildSetting) string {
	var revision string
	var modified bool
	for _, s := range settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if revision != "" && modified {
		return revision + "-dirty"
	}
	return revision
}
