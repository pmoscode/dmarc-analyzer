package account

// maskedValue ersetzt jede Ausgabe eines Secret — über %v, %s, JSON oder
// sonst eine Formatierung ist der echte Wert strukturell nicht erreichbar
// (IMPLEMENTIERUNG.md Abschnitt 9, Regel "Passwort nie im Log").
const maskedValue = "***"

// Secret hält ein Zugangsdaten-Geheimnis (Passwort, App-Passwort) im
// Speicher. String() und MarshalJSON() geben ausnahmslos "***" zurück —
// ein Leak über fmt.Sprintf("%v", ...) oder einen JSON-Dump ist damit
// strukturell ausgeschlossen, nicht nur durch Disziplin beim Aufrufer.
type Secret struct {
	value []byte
}

// NewSecret kopiert value — der Aufrufer behält die Kontrolle über sein
// eigenes Slice und kann es unabhängig überschreiben.
func NewSecret(value []byte) Secret {
	cp := make([]byte, len(value))
	copy(cp, value)
	return Secret{value: cp}
}

// NewSecretFromString ist eine Komfortfunktion für Aufrufer, die den Wert
// als string vorliegen haben (z. B. aus einem UI-Formularfeld).
func NewSecretFromString(value string) Secret {
	return NewSecret([]byte(value))
}

// String erfüllt fmt.Stringer und maskiert immer — auch bei einem leeren
// oder Nullwert-Secret, damit sich aus der Ausgabe nicht ableiten lässt,
// ob überhaupt ein Geheimnis gesetzt ist.
func (s Secret) String() string {
	return maskedValue
}

// GoString erfüllt fmt.GoStringer für den Format-Verb %#v. Ohne diese
// Methode würde %#v die Struktur inklusive des rohen value-Slices als
// Byte-Liste ausgeben — String() wird von %#v nicht benutzt, das wäre ein
// eigenständiges Leck (hex-kodiert statt ASCII, aber trivial rückführbar).
func (s Secret) GoString() string {
	return "account.Secret{" + maskedValue + "}"
}

// MarshalJSON erfüllt json.Marshaler und maskiert immer.
func (s Secret) MarshalJSON() ([]byte, error) {
	return []byte(`"` + maskedValue + `"`), nil
}

// IsZero meldet, ob kein Geheimnis gesetzt ist.
func (s Secret) IsZero() bool {
	return len(s.value) == 0
}

// Expose liefert die Rohbytes für den unmittelbaren Gebrauch (z. B.
// IMAP-LOGIN). Nur für den Moment der tatsächlichen Verwendung aufrufen,
// das Ergebnis nicht zwischenspeichern oder weiterreichen. Direkt danach
// Zero() aufrufen, sobald das Secret nicht mehr gebraucht wird.
func (s Secret) Expose() []byte {
	return s.value
}

// Zero überschreibt die zugrunde liegenden Bytes mit Nullen. Da value ein
// Slice ist, wirkt das auf alle Kopien dieses Secret, die dasselbe
// Backing-Array teilen (IMPLEMENTIERUNG.md Abschnitt 9, Regel "Kurze
// Lebensdauer im Speicher"). Eine über Expose() an eine string-basierte
// API (z. B. imapclient.Login) übergebene Kopie kann dadurch NICHT mehr
// erreicht werden — Go-Strings sind unveränderlich. Das ist eine bekannte,
// hier bewusst akzeptierte Grenze: ohne unsafe-Tricks lässt sich ein
// bereits als string vorliegendes Passwort nicht aktiv überschreiben, nur
// der ursprüngliche []byte-Speicher.
func (s Secret) Zero() {
	for i := range s.value {
		s.value[i] = 0
	}
}
