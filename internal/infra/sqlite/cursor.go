package sqlite

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// queryCursor ist die interne, typisierte Form von report.Query.Cursor /
// report.Page.NextCursor. Nur eines von SortStr/SortInt ist besetzt, je
// nachdem, ob das Sortierfeld Text (org_name, policy_domain) oder eine Zahl
// (date_begin) ist — so bleibt der Wertevergleich beim erneuten Aufbau der
// WHERE-Klausel exakt (keine Textparameter gegen INTEGER-Spalten), ohne auf
// SQLites Typaffinitäts-Konvertierung angewiesen zu sein.
type queryCursor struct {
	GroupKey string `json:"g,omitempty"`
	SortStr  string `json:"ss,omitempty"`
	SortInt  int64  `json:"si,omitempty"`
	HasInt   bool   `json:"hi,omitempty"`
	ID       int64  `json:"id"`
}

// encode liefert eine opake, URL-sichere Textdarstellung des Cursors.
func (c queryCursor) encode() string {
	data, err := json.Marshal(c)
	if err != nil {
		// json.Marshal auf diesem Typ kann praktisch nicht scheitern
		// (nur primitive Felder) — ein Fehler hier wäre ein Programmierfehler.
		panic(fmt.Sprintf("queryCursor konnte nicht kodiert werden: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

// decodeCursor parst eine von encode() erzeugte Cursor-Zeichenkette.
func decodeCursor(s string) (queryCursor, error) {
	var c queryCursor
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return queryCursor{}, fmt.Errorf("cursor konnte nicht dekodiert werden: %w", err)
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return queryCursor{}, fmt.Errorf("cursor hat ein ungültiges format: %w", err)
	}
	return c, nil
}
