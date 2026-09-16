package keyring

import "github.com/pmoscode/dmarc-analyzer/internal/domain/account"

// probeAccountID ist die AccountID, unter der IsAvailable ein
// Kanarien-Geheimnis schreibt, liest und wieder löscht — nie eine echte
// Kontokennung.
const probeAccountID account.AccountID = "dmarc-analyzer-availability-probe"

// IsAvailable prüft, ob der OS-Schlüsselbund tatsächlich nutzbar ist
// (Store→Retrieve→Delete eines Kanarienwerts), statt aus einer bestimmten
// Fehlerklasse zu raten — auf Linux ohne laufenden Secret-Service-Dienst
// liefert go-keyring je nach D-Bus-Zustand unterschiedliche, nicht
// zuverlässig unterscheidbare Fehler. Die Composition Root (AP 4/5) ruft
// dies einmalig beim Programmstart auf, um zwischen OSStore und dem
// Datei-Fallback (FileStore) zu entscheiden.
func IsAvailable() bool {
	probe := account.NewSecretFromString("probe")
	store := NewOSStore()

	if err := store.Store(probeAccountID, probe); err != nil {
		return false
	}
	defer func() { _ = store.Delete(probeAccountID) }()

	if _, err := store.Retrieve(probeAccountID); err != nil {
		return false
	}
	return true
}
