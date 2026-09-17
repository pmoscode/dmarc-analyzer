package web

import (
	"github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
)

// Dependencies bündelt die Use Cases, die die Web-Oberfläche braucht — in
// M0 nur die Kennzahlen fürs Dashboard (MIGRATIONSPLAN.md Abschnitt 2).
// Wächst mit den folgenden Meilensteinen um Berichte, Sendequellen,
// Konten, Sync und Import — analog zu internal/ui.Dependencies, das
// dieses Paket in M5 ersetzt.
type Dependencies struct {
	Statistics *statistics.UseCase
}
