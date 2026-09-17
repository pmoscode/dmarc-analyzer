package ui

import (
	"testing"

	"fyne.io/fyne/v2/theme"
	"github.com/stretchr/testify/require"
)

func TestAppTheme_Primary_DiffersByVariant(t *testing.T) {
	th := appTheme{}
	light := th.Color(theme.ColorNamePrimary, theme.VariantLight)
	dark := th.Color(theme.ColorNamePrimary, theme.VariantDark)
	require.NotEqual(t, light, dark, "Primärfarbe soll pro Variante eigens abgestimmt sein")
}

func TestAppTheme_Background_DiffersByVariant(t *testing.T) {
	th := appTheme{}
	light := th.Color(theme.ColorNameBackground, theme.VariantLight)
	dark := th.Color(theme.ColorNameBackground, theme.VariantDark)
	require.NotEqual(t, light, dark)
}

func TestAppTheme_StatusColors_AreFixedAcrossVariants(t *testing.T) {
	th := appTheme{}

	require.Equal(t,
		th.Color(theme.ColorNameSuccess, theme.VariantLight),
		th.Color(theme.ColorNameSuccess, theme.VariantDark),
		"Statusfarben sind laut dataviz-Skill bewusst modusunabhängig fest",
	)
	require.Equal(t,
		th.Color(theme.ColorNameError, theme.VariantLight),
		th.Color(theme.ColorNameError, theme.VariantDark),
	)
	require.Equal(t,
		th.Color(theme.ColorNameWarning, theme.VariantLight),
		th.Color(theme.ColorNameWarning, theme.VariantDark),
	)
}

func TestAppTheme_UnhandledColorName_FallsBackToDefaultTheme(t *testing.T) {
	th := appTheme{}
	got := th.Color(theme.ColorNameScrollBar, theme.VariantLight)
	want := theme.DefaultTheme().Color(theme.ColorNameScrollBar, theme.VariantLight)
	require.Equal(t, want, got)
}

func TestAppTheme_Size_OverridesRadiiAndPadding(t *testing.T) {
	th := appTheme{}
	require.Greater(t, th.Size(theme.SizeNameCardRadius), theme.DefaultTheme().Size(theme.SizeNameCardRadius))
	require.Greater(t, th.Size(theme.SizeNamePadding), theme.DefaultTheme().Size(theme.SizeNamePadding))
}

func TestAppTheme_Size_UnhandledFallsBackToDefaultTheme(t *testing.T) {
	th := appTheme{}
	require.Equal(t, theme.DefaultTheme().Size(theme.SizeNameText), th.Size(theme.SizeNameText))
}
