// Package keyring implementiert den Port account.CredentialStore: OSStore
// gegen den Betriebssystem-Schlüsselbund (github.com/zalando/go-keyring),
// FileStore als verschlüsselter Dateispeicher-Fallback für Linux ohne
// Secret Service (IMPLEMENTIERUNG.md Abschnitt 9).
package keyring

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/scrypt"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

// scrypt-Parameter nach den von golang.org/x/crypto/scrypt empfohlenen
// interaktiven Werten (Stand 2017, für interaktive Logins, nicht für
// Massenverarbeitung — hier passend, da nur beim Programmstart einmalig
// aufgerufen).
const (
	scryptN      = 1 << 15 // 32768
	scryptR      = 8
	scryptP      = 1
	scryptKeyLen = 32 // AES-256
	saltFileName = "salt"
	saltSize     = 16
)

// errWrongPassphrase wird zurückgegeben, wenn die Entschlüsselung fehlschlägt
// — bei GCM praktisch immer gleichbedeutend mit einer falschen Passphrase
// oder einer manipulierten Datei, beides meldet GCM nicht getrennt.
var errWrongPassphrase = errors.New("entschlüsselung fehlgeschlagen — falsche master-passphrase oder beschädigte datei")

// FileStore ist der verschlüsselte Dateispeicher-Fallback für Systeme ohne
// OS-Schlüsselbund (IMPLEMENTIERUNG.md Abschnitt 9, Zeile "Linux-Fallback"):
// AES-256-GCM, der Schlüssel wird einmalig beim Öffnen per scrypt aus einer
// Master-Passphrase abgeleitet. Ein Secret pro Datei unterhalb von dir,
// benannt nach der AccountID.
type FileStore struct {
	dir  string
	aead cipher.AEAD
}

// NewFileStore öffnet (und legt bei Bedarf an) den verschlüsselten
// Dateispeicher unter dir. Der Salt für die Schlüsselableitung wird beim
// ersten Aufruf erzeugt und danach in dir/salt persistiert — dieselbe
// Passphrase muss bei jedem weiteren Programmstart erneut abgefragt werden
// (IMPLEMENTIERUNG.md Abschnitt 9: "Abfrage beim Programmstart").
func NewFileStore(dir string, passphrase account.Secret) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("verzeichnis %q konnte nicht angelegt werden: %w", dir, err)
	}

	salt, err := loadOrCreateSalt(filepath.Join(dir, saltFileName))
	if err != nil {
		return nil, err
	}

	key, err := scrypt.Key(passphrase.Expose(), salt, scryptN, scryptR, scryptP, scryptKeyLen)
	if err != nil {
		return nil, fmt.Errorf("schlüssel konnte nicht abgeleitet werden: %w", err)
	}
	defer zero(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes-cipher konnte nicht erzeugt werden: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm konnte nicht erzeugt werden: %w", err)
	}

	return &FileStore{dir: dir, aead: aead}, nil
}

func loadOrCreateSalt(path string) ([]byte, error) {
	existing, err := os.ReadFile(path)
	if err == nil {
		if len(existing) != saltSize {
			return nil, fmt.Errorf("salt-datei %q hat unerwartete länge %d", path, len(existing))
		}
		return existing, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("salt-datei %q konnte nicht gelesen werden: %w", path, err)
	}

	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("salt konnte nicht erzeugt werden: %w", err)
	}
	if err := os.WriteFile(path, salt, 0o600); err != nil {
		return nil, fmt.Errorf("salt-datei %q konnte nicht geschrieben werden: %w", path, err)
	}
	return salt, nil
}

// Store verschlüsselt secret mit einer frischen, zufälligen Nonce und
// schreibt es in eine Datei benannt nach accountID.
func (s *FileStore) Store(accountID account.AccountID, secret account.Secret) error {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("nonce konnte nicht erzeugt werden: %w", err)
	}

	ciphertext := s.aead.Seal(nonce, nonce, secret.Expose(), []byte(accountID))

	if err := os.WriteFile(s.path(accountID), ciphertext, 0o600); err != nil {
		return fmt.Errorf("geheimnis für %q konnte nicht gespeichert werden: %w", accountID, err)
	}
	return nil
}

// Retrieve entschlüsselt das für accountID gespeicherte Secret.
func (s *FileStore) Retrieve(accountID account.AccountID) (account.Secret, error) {
	data, err := os.ReadFile(s.path(accountID))
	if errors.Is(err, os.ErrNotExist) {
		return account.Secret{}, account.ErrCredentialNotFound
	}
	if err != nil {
		return account.Secret{}, fmt.Errorf("geheimnis für %q konnte nicht gelesen werden: %w", accountID, err)
	}

	nonceSize := s.aead.NonceSize()
	if len(data) < nonceSize {
		return account.Secret{}, fmt.Errorf("gespeichertes geheimnis für %q ist beschädigt (zu kurz)", accountID)
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]

	plaintext, err := s.aead.Open(nil, nonce, ciphertext, []byte(accountID))
	if err != nil {
		return account.Secret{}, errWrongPassphrase
	}
	defer zero(plaintext)

	return account.NewSecret(plaintext), nil
}

// Delete entfernt das gespeicherte Secret. Kein Fehler, wenn keines
// existierte — Delete ist idempotent.
func (s *FileStore) Delete(accountID account.AccountID) error {
	err := os.Remove(s.path(accountID))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("geheimnis für %q konnte nicht gelöscht werden: %w", accountID, err)
	}
	return nil
}

func (s *FileStore) path(accountID account.AccountID) string {
	return filepath.Join(s.dir, string(accountID)+".enc")
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
