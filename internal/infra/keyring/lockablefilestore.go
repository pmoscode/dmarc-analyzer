package keyring

import (
	"fmt"
	"sync"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

// LockableFileStore umhüllt FileStore mit einem Sperrzustand
// (MIGRATIONSPLAN.md Erweiterung 9.4): direkt nach dem Anlegen gesperrt,
// jeder Aufruf liefert account.ErrCredentialStoreLocked, bis Unlock() mit
// der Master-Passphrase aufgerufen wurde. Anders als der bisherige,
// blockierende Konsolen-Prompt (cmd/dmarc-analyzer/wire.go
// readMasterPassphrase) fragt dieser Adapter nie auf der Konsole nach —
// er ist für den Web-Server gedacht, der beim Doppelklick-Start keine
// Konsole zum Fragen hat; die Oberfläche zeigt stattdessen /entsperren.
type LockableFileStore struct {
	dir string

	mu       sync.RWMutex
	unlocked *FileStore
}

// NewLockableFileStore erzeugt einen gesperrten Store für dir — dasselbe
// Verzeichnis, das FileStore auch unentsperrt schon benutzen würde (Salt
// liegt dort, unabhängig vom Sperrzustand).
func NewLockableFileStore(dir string) *LockableFileStore {
	return &LockableFileStore{dir: dir}
}

// Locked meldet, ob der Store noch auf Unlock() wartet.
func (s *LockableFileStore) Locked() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.unlocked == nil
}

// Unlock leitet aus passphrase den Schlüssel ab und entsperrt den Store.
// Prüft die Passphrase NICHT aktiv gegen ein bestehendes Geheimnis (siehe
// FileStore.NewFileStore-Dokumentation: der Salt existiert unabhängig von
// einem gespeicherten Secret) — eine falsche Passphrase fällt erst beim
// nächsten Retrieve() eines tatsächlich gespeicherten Kontos auf
// (Entschlüsselung schlägt fehl). Der Aufrufer (handlers_unlock.go)
// verifiziert deshalb zusätzlich gegen ein vorhandenes Konto, falls eines
// existiert.
func (s *LockableFileStore) Unlock(passphrase account.Secret) error {
	store, err := NewFileStore(s.dir, passphrase)
	if err != nil {
		return fmt.Errorf("schlüsselspeicher konnte nicht entsperrt werden: %w", err)
	}

	s.mu.Lock()
	s.unlocked = store
	s.mu.Unlock()
	return nil
}

// Lock versetzt den Store zurück in den gesperrten Zustand — für den
// Fall, dass sich eine per Unlock() angenommene Passphrase nachträglich
// als falsch herausstellt (siehe handlers_unlock.go).
func (s *LockableFileStore) Lock() {
	s.mu.Lock()
	s.unlocked = nil
	s.mu.Unlock()
}

func (s *LockableFileStore) current() (*FileStore, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.unlocked == nil {
		return nil, account.ErrCredentialStoreLocked
	}
	return s.unlocked, nil
}

// Store erfüllt account.CredentialStore.
func (s *LockableFileStore) Store(accountID account.AccountID, secret account.Secret) error {
	store, err := s.current()
	if err != nil {
		return err
	}
	return store.Store(accountID, secret)
}

// Retrieve erfüllt account.CredentialStore.
func (s *LockableFileStore) Retrieve(accountID account.AccountID) (account.Secret, error) {
	store, err := s.current()
	if err != nil {
		return account.Secret{}, err
	}
	return store.Retrieve(accountID)
}

// Delete erfüllt account.CredentialStore.
func (s *LockableFileStore) Delete(accountID account.AccountID) error {
	store, err := s.current()
	if err != nil {
		return err
	}
	return store.Delete(accountID)
}
