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

	const value = "super-secret-app-password"
	s := account.NewSecretFromString(value)
	hexEncoded := hex.EncodeToString([]byte(value))

	tests := []string{
		fmt.Sprintf("%v", s),
		fmt.Sprintf("%+v", s),
		fmt.Sprintf("%#v", s),
	}

	for _, formatted := range tests {
		require.NotContains(t, formatted, value, "format string: %q", formatted)
		// For structs without GoString(), %#v prints the raw byte slice
		// hex-encoded instead of as an ASCII substring — a plain
		// NotContains(value) check wouldn't have caught this leak variant
		// (see GoString() in secret.go).
		require.False(t, strings.Contains(strings.ToLower(formatted), hexEncoded),
			"format string contains the value hex-encoded: %q", formatted)
	}
}

func TestSecret_MarshalJSON_NeverLeaksValue(t *testing.T) {
	t.Parallel()

	s := account.NewSecretFromString("json-secret")

	data, err := json.Marshal(s)
	require.NoError(t, err)
	require.Equal(t, `"***"`, string(data))

	type wrapper struct {
		Password account.Secret `json:"password"`
	}
	data, err = json.Marshal(wrapper{Password: s})
	require.NoError(t, err)
	require.NotContains(t, string(data), "json-secret")
	require.Contains(t, string(data), `"password":"***"`)
}

func TestSecret_ZeroOverwritesUnderlyingBytesAcrossCopies(t *testing.T) {
	t.Parallel()

	s := account.NewSecretFromString("secret-to-be-erased")
	require.False(t, s.IsZero())
	exposedBefore := append([]byte(nil), s.Expose()...)
	require.Equal(t, "secret-to-be-erased", string(exposedBefore))

	// A "copy" of the Secret (value type) shares the same backing array —
	// Zero() on a copy must also affect the original.
	copyOfSecret := s
	copyOfSecret.Zero()

	for _, b := range s.Expose() {
		require.Zero(t, b, "every byte must be 0 after Zero()")
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

	original := []byte("original-secret")
	s := account.NewSecret(original)

	// Caller overwrites its own slice — the Secret must not be affected
	// (NewSecret copies).
	for i := range original {
		original[i] = 'x'
	}

	require.Equal(t, "original-secret", string(s.Expose()))
}
