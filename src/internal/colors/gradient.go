// Package colors provides color manipulation utilities for generating
// gradient palettes from a single primary color.
package colors

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Palette represents a complete color palette generated from a primary color.
// This matches the structure in theme-palettes.nix.
type Palette struct {
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Gradient    Gradient `json:"gradient"`
	Text        Text     `json:"text"`
	Zellij      Zellij   `json:"zellij"`
}

// Gradient contains the 7 gradient stops for the powerline prompt.
type Gradient struct {
	Lightest  string `json:"lightest"`  // OS icon bg - brightest
	Primary   string `json:"primary"`   // Directory bg - THE MAIN ACCENT COLOR
	Secondary string `json:"secondary"` // User/host bg
	MidDark   string `json:"midDark"`   // Git section bg
	Dark      string `json:"dark"`      // Languages bg
	Darker    string `json:"darker"`    // Time bg
	Darkest   string `json:"darkest"`   // Nix shell bg - near black
}

// Text contains text colors for various contexts.
type Text struct {
	OnLightest string `json:"onLightest"` // Dark text on lightest bg
	OnMedium   string `json:"onMedium"`   // Black for path (high contrast)
	Accent     string `json:"accent"`     // Accent fg on dark bg
	Muted      string `json:"muted"`      // Git count, subtle
	MutedLight string `json:"mutedLight"` // Time text
}

// Zellij contains colors for the zellij terminal multiplexer.
type Zellij struct {
	Bg        string `json:"bg"`
	Fg        string `json:"fg"`
	Frame     string `json:"frame"`
	Black     string `json:"black"`
	White     string `json:"white"`
	Highlight string `json:"highlight"`
}

// HSL represents a color in Hue-Saturation-Lightness format.
type HSL struct {
	H float64 // Hue: 0-360
	S float64 // Saturation: 0-1
	L float64 // Lightness: 0-1
}

// RGB represents a color in Red-Green-Blue format.
type RGB struct {
	R uint8 // 0-255
	G uint8 // 0-255
	B uint8 // 0-255
}

// GeneratePalette creates a complete palette from a primary hex color.
// The primary color is used to derive all other colors in the palette.
func GeneratePalette(hostname, primaryHex string) (*Palette, error) {
	primary, err := ParseHex(primaryHex)
	if err != nil {
		return nil, fmt.Errorf("invalid primary color: %w", err)
	}

	hsl := primary.ToHSL()

	// Generate gradient stops by adjusting lightness
	// The primary is around 50-60% lightness, we create a gradient from light to dark
	gradient := Gradient{
		Lightest:  adjustLightness(hsl, 0.75).ToRGB().ToHex(), // Very light
		Primary:   primaryHex,                                  // User's chosen color
		Secondary: adjustLightness(hsl, hsl.L*0.75).ToRGB().ToHex(),
		MidDark:   adjustLightness(hsl, 0.20).ToRGB().ToHex(),
		Dark:      adjustLightness(hsl, 0.12).ToRGB().ToHex(),
		Darker:    adjustLightness(hsl, 0.08).ToRGB().ToHex(),
		Darkest:   adjustLightness(hsl, 0.05).ToRGB().ToHex(),
	}

	// Generate text colors
	// onLightest: dark version for reading on light bg
	// onMedium: pure black for maximum contrast
	// accent: bright version for reading on dark bg
	// muted: very dark, subtle
	// mutedLight: medium lightness for secondary info
	text := Text{
		OnLightest: adjustLightness(hsl, 0.10).ToRGB().ToHex(),
		OnMedium:   "#000000",
		Accent:     adjustLightness(hsl, 0.70).ToRGB().ToHex(),
		Muted:      adjustLightness(hsl, 0.12).ToRGB().ToHex(),
		MutedLight: adjustLightness(hsl, 0.55).ToRGB().ToHex(),
	}

	// Generate zellij colors
	zellij := Zellij{
		Bg:        primaryHex,
		Fg:        gradient.Secondary,
		Frame:     primaryHex,
		Black:     gradient.Darkest,
		White:     adjustLightness(hsl, 0.95).ToRGB().ToHex(),
		Highlight: gradient.Lightest,
	}

	return &Palette{
		Name:        fmt.Sprintf("Custom (%s)", hostname),
		Category:    "custom",
		Description: fmt.Sprintf("User-defined color for %s", hostname),
		Gradient:    gradient,
		Text:        text,
		Zellij:      zellij,
	}, nil
}

// ParseHex parses a hex color string (#rrggbb or #rgb) into RGB.
func ParseHex(hex string) (RGB, error) {
	hex = strings.TrimPrefix(hex, "#")

	// Expand shorthand (#rgb → #rrggbb)
	if len(hex) == 3 {
		hex = string(hex[0]) + string(hex[0]) +
			string(hex[1]) + string(hex[1]) +
			string(hex[2]) + string(hex[2])
	}

	if len(hex) != 6 {
		return RGB{}, fmt.Errorf("invalid hex color length: %d", len(hex))
	}

	r, err := strconv.ParseUint(hex[0:2], 16, 8)
	if err != nil {
		return RGB{}, fmt.Errorf("invalid red component: %w", err)
	}
	g, err := strconv.ParseUint(hex[2:4], 16, 8)
	if err != nil {
		return RGB{}, fmt.Errorf("invalid green component: %w", err)
	}
	b, err := strconv.ParseUint(hex[4:6], 16, 8)
	if err != nil {
		return RGB{}, fmt.Errorf("invalid blue component: %w", err)
	}

	return RGB{R: uint8(r), G: uint8(g), B: uint8(b)}, nil
}

// ToHex converts RGB to a hex string (#rrggbb).
func (c RGB) ToHex() string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// ToHSL converts RGB to HSL.
func (c RGB) ToHSL() HSL {
	r := float64(c.R) / 255.0
	g := float64(c.G) / 255.0
	b := float64(c.B) / 255.0

	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	delta := max - min

	// Lightness
	l := (max + min) / 2

	// Saturation
	var s float64
	if delta == 0 {
		s = 0
	} else {
		s = delta / (1 - math.Abs(2*l-1))
	}

	// Hue
	var h float64
	if delta == 0 {
		h = 0
	} else {
		switch max {
		case r:
			h = 60 * math.Mod((g-b)/delta, 6)
		case g:
			h = 60 * ((b-r)/delta + 2)
		case b:
			h = 60 * ((r-g)/delta + 4)
		}
	}

	if h < 0 {
		h += 360
	}

	return HSL{H: h, S: s, L: l}
}

// ToRGB converts HSL to RGB.
func (c HSL) ToRGB() RGB {
	// Clamp values
	h := math.Mod(c.H, 360)
	if h < 0 {
		h += 360
	}
	s := clamp(c.S, 0, 1)
	l := clamp(c.L, 0, 1)

	chroma := (1 - math.Abs(2*l-1)) * s
	x := chroma * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - chroma/2

	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = chroma, x, 0
	case h < 120:
		r, g, b = x, chroma, 0
	case h < 180:
		r, g, b = 0, chroma, x
	case h < 240:
		r, g, b = 0, x, chroma
	case h < 300:
		r, g, b = x, 0, chroma
	default:
		r, g, b = chroma, 0, x
	}

	return RGB{
		R: uint8(math.Round((r + m) * 255)),
		G: uint8(math.Round((g + m) * 255)),
		B: uint8(math.Round((b + m) * 255)),
	}
}

// adjustLightness creates a new HSL with the specified lightness.
func adjustLightness(hsl HSL, newL float64) HSL {
	return HSL{
		H: hsl.H,
		S: hsl.S,
		L: clamp(newL, 0, 1),
	}
}

// clamp restricts a value to a range.
func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// PresetPalettes contains the standard palette names available in theme-palettes.nix.
var PresetPalettes = []struct {
	Name    string `json:"name"`
	Primary string `json:"primary"`
}{
	{Name: "iceBlue", Primary: "#98b8d8"},
	{Name: "blue", Primary: "#769ff0"},
	{Name: "yellow", Primary: "#d4c060"},
	{Name: "green", Primary: "#68c878"},
	{Name: "orange", Primary: "#e09050"},
	{Name: "purple", Primary: "#9868d0"},
	{Name: "pink", Primary: "#e070a0"},
	{Name: "lightGray", Primary: "#a8aeb8"},
	{Name: "darkGray", Primary: "#686c70"},
	{Name: "midGray", Primary: "#909498"},
	{Name: "warmGray", Primary: "#a8a098"},
	{Name: "roseGold", Primary: "#c8a8a0"},
}

// IsPreset checks if a color matches a preset palette.
func IsPreset(hex string) (string, bool) {
	hex = strings.ToLower(hex)
	for _, p := range PresetPalettes {
		if strings.ToLower(p.Primary) == hex {
			return p.Name, true
		}
	}
	return "", false
}

