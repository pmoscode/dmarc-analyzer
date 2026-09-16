// Package uitest bündelt kleine Helfer für Fyne-Widget-Tests
// (fyne.io/fyne/v2/test), die mehrere internal/ui-Pakete gemeinsam
// brauchen — kein eigenes _test.go, damit andere Pakete es importieren
// können (Go-übliches Muster für geteilten Testcode).
package uitest

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// Walk durchläuft obj und alle enthaltenen CanvasObjects rekursiv (über
// Container.Objects bzw. den Widget-Renderer) und ruft visit für jedes auf.
func Walk(obj fyne.CanvasObject, visit func(fyne.CanvasObject)) {
	if obj == nil {
		return
	}
	visit(obj)

	if c, ok := obj.(*fyne.Container); ok {
		for _, child := range c.Objects {
			Walk(child, visit)
		}
		return
	}
	if w, ok := obj.(fyne.Widget); ok {
		if r := test.WidgetRenderer(w); r != nil {
			for _, child := range r.Objects() {
				Walk(child, visit)
			}
		}
	}
}

// FindButton sucht rekursiv den ersten Button mit dem gegebenen Text.
func FindButton(t *testing.T, obj fyne.CanvasObject, label string) *widget.Button {
	t.Helper()
	var found *widget.Button
	Walk(obj, func(o fyne.CanvasObject) {
		if found != nil {
			return
		}
		if b, ok := o.(*widget.Button); ok && b.Text == label {
			found = b
		}
	})
	if found == nil {
		t.Fatalf("button %q nicht gefunden", label)
	}
	return found
}

// FindLabel sucht rekursiv das erste Label mit exakt diesem Text.
func FindLabel(obj fyne.CanvasObject, text string) *widget.Label {
	var found *widget.Label
	Walk(obj, func(o fyne.CanvasObject) {
		if found != nil {
			return
		}
		if l, ok := o.(*widget.Label); ok && l.Text == text {
			found = l
		}
	})
	return found
}

// FindEntries sucht rekursiv alle Entry-Widgets in Dokumentreihenfolge —
// für Formulare mit mehreren Eingabefeldern, bei denen der
// Platzhaltertext eindeutiger ist als der (anfangs leere) Text.
func FindEntries(obj fyne.CanvasObject) []*widget.Entry {
	var found []*widget.Entry
	Walk(obj, func(o fyne.CanvasObject) {
		if e, ok := o.(*widget.Entry); ok {
			found = append(found, e)
		}
	})
	return found
}

// CountWidgets zählt, wie oft predicate über alle CanvasObjects in obj
// wahr wird.
func CountWidgets(obj fyne.CanvasObject, predicate func(fyne.CanvasObject) bool) int {
	count := 0
	Walk(obj, func(o fyne.CanvasObject) {
		if predicate(o) {
			count++
		}
	})
	return count
}
