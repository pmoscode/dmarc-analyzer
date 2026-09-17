package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// appTheme ist ein vollständig eigenes Farbschema für hellen und dunklen
// Modus (IMPLEMENTIERUNG.md Abschnitt 10.4: "Eigenes Theme mit
// Farbpalette, die sowohl im hellen als auch im dunklen Modus
// funktioniert").
//
// Die Werte folgen der im dataviz-Skill dokumentierten, validierten
// Referenzpalette (Kategorie-Slot 1 als Primärfarbe, feste Statusfarben
// für Erfolg/Warnung/Fehler) — dieselbe Sprache wie die Diagrammfarben in
// internal/infra/charts, damit Oberfläche und Diagramme zusammen wirken
// statt zwei zufällig verschiedene Paletten nebeneinander zu zeigen.
// Vorherige Version delegierte alles außer der Primärfarbe an
// theme.DefaultTheme() — das ergab im dunklen Systemmodus ein
// unstrukturiertes, kontrastarmes "alles grau/schwarz" ohne eigene
// Identität; diese Version definiert Hintergrund/Oberflächen/Text/Trenner
// bewusst selbst.
type appTheme struct{}

var _ fyne.Theme = appTheme{}

var (
	// Primärfarbe: Kategorie-Slot 1 (Blau) der Referenzpalette.
	colorPrimaryLight = color.NRGBA{R: 0x2a, G: 0x78, B: 0xd6, A: 0xff}
	colorPrimaryDark  = color.NRGBA{R: 0x39, G: 0x87, B: 0xe5, A: 0xff}

	// Statuspalette — bewusst modusunabhängig fest (dataviz-Skill:
	// "Status palette (fixed — never themed)"), dieselben Werte wie
	// internal/infra/charts.colorGood/-Warning/-Critical.
	colorStatusGood     = color.NRGBA{R: 0x0c, G: 0xa3, B: 0x0c, A: 0xff}
	colorStatusWarning  = color.NRGBA{R: 0xfa, G: 0xb2, B: 0x19, A: 0xff}
	colorStatusCritical = color.NRGBA{R: 0xd0, G: 0x3b, B: 0x3b, A: 0xff}

	// Seiten-/Oberflächenebenen, hell: Page Plane ist der Fensterhintergrund,
	// Surface die leicht davon abgesetzte Ebene für Eingaben/Karten/Menüs.
	colorPagePlaneLight    = color.NRGBA{R: 0xf7, G: 0xf7, B: 0xf5, A: 0xff}
	colorSurfaceLight      = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	colorPrimaryInkLight   = color.NRGBA{R: 0x0b, G: 0x0b, B: 0x0b, A: 0xff}
	colorSecondaryInkLight = color.NRGBA{R: 0x52, G: 0x51, B: 0x4e, A: 0xff}
	colorMutedLight        = color.NRGBA{R: 0x89, G: 0x87, B: 0x81, A: 0xff}
	colorGridlineLight     = color.NRGBA{R: 0xe1, G: 0xe0, B: 0xd9, A: 0xff}

	// Seiten-/Oberflächenebenen, dunkel.
	colorPagePlaneDark    = color.NRGBA{R: 0x14, G: 0x14, B: 0x14, A: 0xff}
	colorSurfaceDark      = color.NRGBA{R: 0x1f, G: 0x1f, B: 0x1e, A: 0xff}
	colorPrimaryInkDark   = color.NRGBA{R: 0xf5, G: 0xf5, B: 0xf3, A: 0xff}
	colorSecondaryInkDark = color.NRGBA{R: 0xc3, G: 0xc2, B: 0xb7, A: 0xff}
	colorMutedDark        = color.NRGBA{R: 0x8f, G: 0x8d, B: 0x87, A: 0xff}
	colorGridlineDark     = color.NRGBA{R: 0x33, G: 0x33, B: 0x31, A: 0xff}
)

func withAlpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}

func (appTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	dark := variant == theme.VariantDark

	switch name {
	case theme.ColorNamePrimary, theme.ColorNameFocus:
		if dark {
			return colorPrimaryDark
		}
		return colorPrimaryLight
	case theme.ColorNameSuccess:
		return colorStatusGood
	case theme.ColorNameWarning:
		return colorStatusWarning
	case theme.ColorNameError:
		return colorStatusCritical
	case theme.ColorNameForegroundOnPrimary, theme.ColorNameForegroundOnSuccess,
		theme.ColorNameForegroundOnWarning, theme.ColorNameForegroundOnError:
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	case theme.ColorNameBackground:
		if dark {
			return colorPagePlaneDark
		}
		return colorPagePlaneLight
	case theme.ColorNameButton, theme.ColorNameInputBackground, theme.ColorNameMenuBackground,
		theme.ColorNameOverlayBackground, theme.ColorNameHeaderBackground,
		theme.ColorNameScrollBarBackground, theme.ColorNameDisabledButton:
		if dark {
			return colorSurfaceDark
		}
		return colorSurfaceLight
	case theme.ColorNameForeground:
		if dark {
			return colorPrimaryInkDark
		}
		return colorPrimaryInkLight
	case theme.ColorNamePlaceHolder:
		// Placeholder-/Hinweistext (z. B. "Alle Domains" im Filter, Hinttext
		// unter Formularfeldern) bleibt gut lesbar — nur Disabled soll
		// deutlich zurücktreten.
		if dark {
			return colorSecondaryInkDark
		}
		return colorSecondaryInkLight
	case theme.ColorNameDisabled:
		if dark {
			return colorMutedDark
		}
		return colorMutedLight
	case theme.ColorNameSeparator, theme.ColorNameInputBorder, theme.ColorNameShadow:
		if dark {
			return colorGridlineDark
		}
		return colorGridlineLight
	case theme.ColorNameHover:
		if dark {
			return withAlpha(colorPrimaryDark, 0x20)
		}
		return withAlpha(colorPrimaryLight, 0x14)
	case theme.ColorNamePressed, theme.ColorNameSelection:
		if dark {
			return withAlpha(colorPrimaryDark, 0x45)
		}
		return withAlpha(colorPrimaryLight, 0x30)
	case theme.ColorNameHyperlink:
		if dark {
			return colorPrimaryDark
		}
		return colorPrimaryLight
	}
	return theme.DefaultTheme().Color(name, variant)
}

func (appTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (appTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

// Size vergrößert Abstände und Eckenradien gegenüber Fynes eher engen,
// kantigen Standardwerten (z. B. CardRadius/ButtonRadius: 5, Padding: 4) —
// der Haupthebel für einen "moderneren", luftigeren Eindruck, ohne
// Textgrößen oder Interaktionsflächen zu verändern.
func (appTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 8
	case theme.SizeNameInnerPadding:
		return 14
	case theme.SizeNameButtonRadius, theme.SizeNameInputRadius:
		return 8
	case theme.SizeNameCardRadius:
		return 12
	case theme.SizeNameDialogRadius, theme.SizeNamePopupRadius:
		return 14
	case theme.SizeNameScrollBarRadius:
		return 6
	}
	return theme.DefaultTheme().Size(name)
}
