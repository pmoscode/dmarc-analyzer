package account

// maskedValue replaces every rendering of a Secret — via %v, %s, JSON or
// any other formatting, the real value is structurally unreachable
// (IMPLEMENTIERUNG.md section 9, rule "never log the password").
const maskedValue = "***"

// Secret holds a credential secret (password, app password) in memory.
// String() and MarshalJSON() always return "***" — a leak via
// fmt.Sprintf("%v", ...) or a JSON dump is thereby structurally excluded,
// not merely by caller discipline.
type Secret struct {
	value []byte
}

// NewSecret copies value — the caller keeps control of its own slice and
// can overwrite it independently.
func NewSecret(value []byte) Secret {
	cp := make([]byte, len(value))
	copy(cp, value)
	return Secret{value: cp}
}

// NewSecretFromString is a convenience function for callers that have the
// value as a string (e.g. from a UI form field).
func NewSecretFromString(value string) Secret {
	return NewSecret([]byte(value))
}

// String satisfies fmt.Stringer and always masks — even for an empty or
// zero-value Secret, so the output can't be used to infer whether a
// secret is set at all.
func (s Secret) String() string {
	return maskedValue
}

// GoString satisfies fmt.GoStringer for the %#v format verb. Without this
// method, %#v would print the struct including the raw value slice as a
// byte list — String() is not used by %#v, which would be a separate leak
// (hex-encoded instead of ASCII, but trivially reversible).
func (s Secret) GoString() string {
	return "account.Secret{" + maskedValue + "}"
}

// MarshalJSON satisfies json.Marshaler and always masks.
func (s Secret) MarshalJSON() ([]byte, error) {
	return []byte(`"` + maskedValue + `"`), nil
}

// IsZero reports whether no secret is set.
func (s Secret) IsZero() bool {
	return len(s.value) == 0
}

// Expose returns the raw bytes for immediate use (e.g. IMAP LOGIN). Only
// call it right at the moment of actual use, don't cache or pass on the
// result. Call Zero() immediately afterward once the secret is no longer
// needed.
func (s Secret) Expose() []byte {
	return s.value
}

// Zero overwrites the underlying bytes with zeros. Since value is a slice,
// this affects every copy of this Secret that shares the same backing
// array (IMPLEMENTIERUNG.md section 9, rule "short lifetime in memory"). A
// copy passed via Expose() to a string-based API (e.g. imapclient.Login)
// can NOT be reached this way anymore — Go strings are immutable. This is
// a known, deliberately accepted limitation: without unsafe tricks, a
// password that already exists as a string can't be actively overwritten,
// only the original []byte storage.
func (s Secret) Zero() {
	for i := range s.value {
		s.value[i] = 0
	}
}
