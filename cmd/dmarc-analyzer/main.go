// Command dmarc-analyzer is the application's Composition Root: here,
// and only here, concrete adapters are wired to the use cases
// (see IMPLEMENTIERUNG.md section 4.1).
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/pmoscode/dmarc-analyzer/internal/platform/logging"
)

// version and commit are set at build time via -ldflags (see the
// Taskfile.yml "build" task, Dockerfile, .github/workflows/ci.yml). For
// local development builds, version stays "dev"; an empty commit is
// filled in at runtime from the Go build information (see version.go).
var (
	version = "dev"
	commit  = ""
)

const usage = `dmarc-analyzer ` + `%s` + `

Runs as a Docker container, fully configured via environment variables
(see docs/features/deployment.md). Without arguments it starts the web
UI. For diagnostics/maintenance via "docker exec" the following
subcommands are available:

Usage:
  dmarc-analyzer web                        Start the web UI (also the default with no arguments)
  dmarc-analyzer sync                       Sync the configured account
  dmarc-analyzer import <path>...           Import DMARC reports from files (.eml, .xml, .xml.gz, .zip)
  dmarc-analyzer stats [--days N] [--domain D]
                                             Print metrics for the last N days (default: 30)
  dmarc-analyzer healthcheck                 Docker HEALTHCHECK: checks whether the HTTP server responds
`

// main decides whether main.go itself needs to add an argument (no
// subcommand → "web", MIGRATIONSPLAN.md M3) before run() processes the
// actual subcommand.
func main() {
	args := os.Args[1:]

	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Printf(usage, version)
		return
	}

	if len(args) == 0 {
		// The web UI has been the default with no arguments since M3.
		args = []string{"web"}
	}

	if err := run(context.Background(), args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		fmt.Printf(usage, version)
		return nil
	}

	cmd, rest := args[0], args[1:]

	// healthcheck deliberately runs without logger setup and without
	// newApp()/envconfig.Load() below — Docker calls it every few
	// seconds, a log entry per call would be pure noise, and a liveness
	// check shouldn't fail because of a briefly invalid IMAP/OIDC
	// configuration (see cmd_healthcheck.go).
	if cmd == "healthcheck" {
		return runHealthcheck(ctx, rest)
	}

	logger := logging.New()
	logger.Info("dmarc-analyzer started", "version", version, "commit", resolvedCommit())

	handler, ok := subcommands[cmd]
	if !ok {
		return fmt.Errorf("unknown subcommand %q — see 'dmarc-analyzer' without arguments for help", cmd)
	}

	// Composition Root: only here, after the subcommand has been
	// recognized as known, are concrete adapters wired (ENV
	// configuration read, real database opened). An unknown subcommand
	// or plain help output triggers no I/O at all — important both for
	// tests and so a typo doesn't accidentally create the database.
	application, err := newApp(ctx)
	if err != nil {
		return fmt.Errorf("application could not be initialized: %w", err)
	}
	defer func() { _ = application.Close() }()

	return handler(ctx, application, rest)
}

// subcommands maps subcommand names to their handlers.
var subcommands = map[string]func(context.Context, *app, []string) error{
	"web":    runWeb,
	"sync":   runSync,
	"import": runImport,
	"stats":  runStats,
}
