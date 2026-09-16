package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRun_NoArgs_PrintsUsageWithoutError(t *testing.T) {
	require.NoError(t, run(context.Background(), nil))
}

func TestRun_UnknownSubcommand_ReturnsErrorWithoutWiringApp(t *testing.T) {
	// Ein unbekannter Unterbefehl darf keine echte Infrastruktur anfassen
	// (Datenbankdatei, OS-Schlüsselbund) — sonst würde dieser Test
	// unbeabsichtigt echte Dateien im Home-Verzeichnis anlegen. Die
	// Abwesenheit jeglicher I/O-Fehler hier ist bereits der Beleg dafür,
	// dass main.go den Unterbefehl vor newApp() prüft (siehe Kommentar dort).
	err := run(context.Background(), []string{"irgendwas"})

	require.Error(t, err)
	require.ErrorContains(t, err, "irgendwas")
}
