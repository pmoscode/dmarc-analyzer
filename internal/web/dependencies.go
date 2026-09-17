package web

import (
	"github.com/pmoscode/dmarc-analyzer/internal/app/queryreports"
	"github.com/pmoscode/dmarc-analyzer/internal/app/sourcestats"
	"github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
)

// Dependencies bündelt die Use Cases, die die Web-Oberfläche braucht —
// analog zu internal/ui.Dependencies, das dieses Paket in M5 ersetzt.
// Wächst mit den folgenden Meilensteinen weiter um Konten, Sync und
// Import (M3).
type Dependencies struct {
	Statistics *statistics.UseCase
	Reports    *queryreports.UseCase
	Sources    *sourcestats.UseCase
}
