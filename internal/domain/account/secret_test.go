package account_test

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

func TestSecret_StringNeverLeaksValue(t *testing.T) {
	t.Parallel()

	s := account.NewSecretFromString("hunter2")

	require.Equal(t, "***", s.String())
	require.NotContains(t, s.String(), "hunter2")
}

func TestSecret_FormattingVerbsNeverLeakValue(t *testing.T) {
	t.Parallel()

	const value = "super-geheimes-app-passwort"
	s := account.NewSecretFromString(value)
	hexEncoded := hex.EncodeToString([]byte(value))

	tests := []string{
		fmt.Sprintf("%v", s),
		fmt.Sprintf("%+v", s),
		fmt.Sprintf("%#v", s),
	}

	for _, formatted := range tests {
		require.NotContains(t, formatted, value, "Format-String: %q", formatted)
		// %#v zeigt bei Structs ohne GoString() das rohe Byte-Slice
		// hex-kodiert statt als ASCII-Substring — ein reiner
		// NotContains(value)-Check hätte diese Leck-Variante nicht
		// erkannt (siehe GoString() in secret.go).
		require.False(t, strings.Contains(strings.ToLower(formatted), hexEncoded),
			"Format-String enthält den Wert hex-kodiert: %q", formatted)
	}
}

func TestSecret_MarshalJSON_NeverLeaksValue(t *testing.T) {
	t.Parallel()

	s := account.NewSecretFromString("json-geheimnis")

	data, err := json.Marshal(s)
	require.NoError(t, err)
	require.Equal(t, `"***"`, string(data))

	type wrapper struct {
		Password account.Secret `json:"password"`
	}
	data, err = json.Marshal(wrapper{Password: s})
	require.NoError(t, err)
	require.NotContains(t, string(data), "json-geheimnis")
	require.Contains(t, string(data), `"password":"***"`)
}

func TestSecret_ZeroOverwritesUnderlyingBytesAcrossCopies(t *testing.T) {
	t.Parallel()

	s := account.NewSecretFromString("zu-loeschendes-geheimnis")
	require.False(t, s.IsZero())
	exposedBefore := append([]byte(nil), s.Expose()...)
	require.Equal(t, "zu-loeschendes-geheimnis", string(exposedBefore))

	// Eine "Kopie" des Secret (Value-Typ) teilt dasselbe Backing-Array —
	// Zero() auf einer Kopie muss auch beim Original wirken.
	copyOfSecret := s
	copyOfSecret.Zero()

	for _, b := range s.Expose() {
		require.Zero(t, b, "nach Zero() muss jedes Byte 0 sein")
	}
}

func TestSecret_IsZero(t *testing.T) {
	t.Parallel()

	var empty account.Secret
	require.True(t, empty.IsZero())

	nonEmpty := account.NewSecretFromString("x")
	require.False(t, nonEmpty.IsZero())
}

func TestNewSecret_CopiesInput_CallerCanOverwriteOriginal(t *testing.T) {
	t.Parallel()

	original := []byte("original-geheimnis")
	s := account.NewSecret(original)

	// Aufrufer überschreibt sein eigenes Slice — das Secret darf davon
	// nicht betroffen sein (NewSecret kopiert).
	for i := range original {
		original[i] = 'x'
	}

	require.Equal(t, "original-geheimnis", string(s.Expose()))
}
