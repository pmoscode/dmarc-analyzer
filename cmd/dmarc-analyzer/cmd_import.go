package main

import (
	"context"
	"errors"
	"fmt"
	"os"
)

// runImport importiert DMARC-Reports aus lokalen Dateien
// (UMSETZUNGSPLAN.md AP 4: "dmarc-analyzer import <pfad>").
func runImport(ctx context.Context, a *app, paths []string) error {
	if len(paths) == 0 {
		return errors.New("mindestens ein Dateipfad erwartet, Aufruf: dmarc-analyzer import <pfad> [<weitere pfade>]")
	}

	result, err := a.importer.ImportPaths(ctx, paths)
	if err != nil {
		return fmt.Errorf("import konnte nicht abgeschlossen werden: %w", err)
	}

	fmt.Printf("%d neu, %d übersprungen, %d fehlerhaft\n", result.New, result.Skipped, result.Failed)
	for _, e := range result.Errors {
		fmt.Fprintf(os.Stderr, "  Fehler: %v\n", e)
	}

	return nil
}
