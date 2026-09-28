package sqlite

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// queryCursor is the internal, typed form of report.Query.Cursor /
// report.Page.NextCursor. Only one of SortStr/SortInt is set, depending on
// whether the sort field is text (org_name, policy_domain) or a number
// (date_begin) — this keeps the value comparison exact when rebuilding the
// WHERE clause (no text parameters against INTEGER columns), without
// relying on SQLite's type-affinity conversion.
type queryCursor struct {
	GroupKey string `json:"g,omitempty"`
	SortStr  string `json:"ss,omitempty"`
	SortInt  int64  `json:"si,omitempty"`
	HasInt   bool   `json:"hi,omitempty"`
	ID       int64  `json:"id"`
}

// encode returns an opaque, URL-safe text representation of the cursor.
func (c queryCursor) encode() string {
	data, err := json.Marshal(c)
	if err != nil {
		// json.Marshal on this type can practically never fail (only
		// primitive fields) — an error here would be a programming error.
		panic(fmt.Sprintf("queryCursor could not be encoded: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

// decodeCursor parses a cursor string produced by encode().
func decodeCursor(s string) (queryCursor, error) {
	var c queryCursor
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return queryCursor{}, fmt.Errorf("could not decode cursor: %w", err)
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return queryCursor{}, fmt.Errorf("cursor has an invalid format: %w", err)
	}
	return c, nil
}
