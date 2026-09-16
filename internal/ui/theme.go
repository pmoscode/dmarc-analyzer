package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// appTheme ergänzt Fynes Standard-Theme um eine eigene Akzentfarbe
// (IMPLEMENTIERUNG.md Abschnitt 10.4: "Eigenes Theme mit Farbpalette, die
// sowohl im hellen als auch im dunklen Modus funktioniert"). Alles andere
// (Kontrast, Light/Dark-Umschaltung) übernimmt Fynes ThemeVariant-System
// unverändert — das ist bereits geprüft barrierefrei, ein eigenes Rad hier
// würde nur Risiko ohne Nutzen hinzufügen.
type appTheme struct{}

var _ fyne.Theme = appTheme{}

// primaryColor ist ein gedecktes Blau, das in beiden Varianten
// ausreichenden Kontrast zum jeweiligen Hintergrund hat.
var primaryColor = color.NRGBA{R: 0x2f, G: 0x6f, B: 0xed, A: 0xff}

func (appTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNamePrimary {
		return primaryColor
	}
	return theme.DefaultTheme().Color(name, variant)
}

func (appTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (appTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (appTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}
