package main

import (
	"context"
	"errors"
	"fmt"
	"os"
)

// runImport imports DMARC reports from local files
// (UMSETZUNGSPLAN.md AP 4: "dmarc-analyzer import <path>").
func runImport(ctx context.Context, a *app, paths []string) error {
	if len(paths) == 0 {
		return errors.New("expected at least one file path, usage: dmarc-analyzer import <path> [<more paths>]")
	}

	result, err := a.importer.ImportPaths(ctx, paths)
	if err != nil {
		return fmt.Errorf("import could not be completed: %w", err)
	}

	fmt.Printf("%d new, %d skipped, %d failed\n", result.New, result.Skipped, result.Failed)
	for _, e := range result.Errors {
		fmt.Fprintf(os.Stderr, "  error: %v\n", e)
	}

	return nil
}
