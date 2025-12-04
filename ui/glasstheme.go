package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// GlassTheme implements a beautiful glass morphism theme with Material Design
type GlassTheme struct {
	baseTheme fyne.Theme
}

func NewGlassTheme() *GlassTheme {
	return &GlassTheme{
		baseTheme: theme.LightTheme(),
	}
}

// Color returns colors with glass morphism effects
// Using NRGBA for proper alpha transparency
func (t *GlassTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		// Glass background - frosted glass effect with subtle blue tint
		// Very transparent for true glass morphism effect (Windows will add blur)
		return color.NRGBA{R: 245, G: 250, B: 255, A: 150} // Very transparent - Windows blur will make it visible
	case theme.ColorNameButton:
		// Material blue - vibrant and modern
		return color.NRGBA{R: 33, G: 150, B: 243, A: 255}
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 200, G: 200, B: 200, A: 150}
	case theme.ColorNameInputBackground:
		// Glass input background - frosted white with subtle tint
		return color.NRGBA{R: 255, G: 255, B: 255, A: 180} // More transparent
	case theme.ColorNameInputBorder:
		// Glass border with subtle blue tint
		return color.NRGBA{R: 200, G: 220, B: 240, A: 180}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 150, G: 150, B: 150, A: 200}
	case theme.ColorNamePrimary:
		// Material blue - vibrant accent
		return color.NRGBA{R: 33, G: 150, B: 243, A: 255}
	case theme.ColorNameHover:
		// Light blue hover with glass transparency
		return color.NRGBA{R: 187, G: 222, B: 251, A: 200}
	case theme.ColorNameFocus:
		// Focus color - Material blue ring
		return color.NRGBA{R: 33, G: 150, B: 243, A: 255}
	case theme.ColorNameSelection:
		// Selection highlight with glass morphism
		return color.NRGBA{R: 187, G: 222, B: 251, A: 160}
	case theme.ColorNameShadow:
		// Enhanced shadow for depth and glass effect
		return color.NRGBA{R: 100, G: 150, B: 200, A: 60}
	case theme.ColorNameForeground:
		// Text color - dark for contrast on glass
		return color.NRGBA{R: 30, G: 30, B: 30, A: 255}
	case theme.ColorNameSeparator:
		// Subtle glass separator
		return color.NRGBA{R: 200, G: 220, B: 240, A: 120}
	case theme.ColorNameDisabled:
		// Disabled text
		return color.NRGBA{R: 150, G: 150, B: 150, A: 180}
	case theme.ColorNameError:
		// Error color
		return color.NRGBA{R: 244, G: 67, B: 54, A: 255}
	case theme.ColorNameSuccess:
		// Success color
		return color.NRGBA{R: 76, G: 175, B: 80, A: 255}
	case theme.ColorNameWarning:
		// Warning color
		return color.NRGBA{R: 255, G: 152, B: 0, A: 255}
	}
	return t.baseTheme.Color(name, variant)
}

func (t *GlassTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.baseTheme.Font(style)
}

func (t *GlassTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.baseTheme.Icon(name)
}

func (t *GlassTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 16 // More generous padding for glass effect
	case theme.SizeNameScrollBar:
		return 18
	case theme.SizeNameScrollBarSmall:
		return 10
	case theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameInputBorder:
		return 2
	case theme.SizeNameInputRadius:
		return 12 // More rounded for glass effect
	case theme.SizeNameSelectionRadius:
		return 8
	case theme.SizeNameInlineIcon:
		return 24
	case theme.SizeNameInnerPadding:
		return 12
	case theme.SizeNameLineSpacing:
		return 6
	case theme.SizeNameText:
		return 14 // Slightly larger text
	}
	return t.baseTheme.Size(name)
}
