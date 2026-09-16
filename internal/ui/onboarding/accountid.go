package onboarding

import "github.com/google/uuid"

// newAccountID erzeugt eine neue, eindeutige Konto-ID — dieselbe
// Vorgehensweise wie in internal/ui/settings.View und
// cmd/dmarc-analyzer/cmd_account.go.
func newAccountID() string {
	return uuid.NewString()
}
