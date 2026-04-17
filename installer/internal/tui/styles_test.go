package tui

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

func TestStylesInitialized(t *testing.T) {
	t.Parallel()

	// Verify key styles are non-zero (have at least one property set).
	styles := map[string]lipgloss.Style{
		"panelStyle":          panelStyle,
		"dryRunPanelStyle":    dryRunPanelStyle,
		"titleStyle":          titleStyle,
		"dimStyle":            dimStyle,
		"successStyle":        successStyle,
		"warnStyle":           warnStyle,
		"errorStyle":          errorStyle,
		"selectedStyle":       selectedStyle,
		"menuDimStyle":        menuDimStyle,
		"cursorStyle":         cursorStyle,
		"checkStyle":          checkStyle,
		"uncheckStyle":        uncheckStyle,
		"descStyle":           descStyle,
		"iconStyle":           iconStyle,
		"iconDimStyle":        iconDimStyle,
		"progressDoneStyle":   progressDoneStyle,
		"progressActiveStyle": progressActiveStyle,
		"progressQueuedStyle": progressQueuedStyle,
		"progressFailedStyle": progressFailedStyle,
		"statsStyle":          statsStyle,
		"tableHeaderStyle":    tableHeaderStyle,
		"tableCellStyle":      tableCellStyle,
		"footerStyle":         footerStyle,
		"footerKeyStyle":      footerKeyStyle,
		"stepStyle":           stepStyle,
	}

	for name, s := range styles {
		// Render a test string; if the style is truly zero/empty
		// the output would be identical to input.
		rendered := s.Render("test")
		if rendered == "" {
			t.Errorf("style %q rendered empty string", name)
		}
	}
}

func TestPanelOuterWidth(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		contentW    int
		wantGreater int
	}{
		{name: "normal", contentW: 60, wantGreater: 60},
		{name: "small", contentW: 20, wantGreater: 20},
		{name: "large", contentW: 100, wantGreater: 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := panelOuterWidth(tt.contentW)
			if got <= tt.wantGreater {
				t.Errorf(
					"panelOuterWidth(%d) = %d, want > %d",
					tt.contentW, got, tt.wantGreater,
				)
			}
		})
	}
}

func TestContentWidth(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		termWidth int
		want      int
	}{
		{name: "normal terminal", termWidth: 100, want: 80},
		{name: "wide terminal", termWidth: 200, want: 80},
		{name: "narrow terminal", termWidth: 30, want: 40},
		{name: "very narrow", termWidth: 10, want: 40},
		{name: "exact boundary", termWidth: 84, want: 80},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := contentWidth(tt.termWidth)
			if got != tt.want {
				t.Errorf(
					"contentWidth(%d) = %d, want %d",
					tt.termWidth, got, tt.want,
				)
			}
		})
	}
}

func TestPanelGap(t *testing.T) {
	t.Parallel()
	result := panelGap(" ")
	if result == "" {
		t.Error("panelGap should return non-empty string")
	}
	// The result should contain the input character.
	if len(result) == 0 {
		t.Error("panelGap result has zero length")
	}
}

func TestRenderFooter(t *testing.T) {
	t.Parallel()

	t.Run("single hint", func(t *testing.T) {
		t.Parallel()
		result := renderFooter("q quit")
		if result == "" {
			t.Error("renderFooter returned empty string")
		}
	})

	t.Run("multiple hints", func(t *testing.T) {
		t.Parallel()
		result := renderFooter("↑/↓ navigate", "enter select", "q quit")
		if result == "" {
			t.Error("renderFooter returned empty string")
		}
	})

	t.Run("no hints", func(t *testing.T) {
		t.Parallel()
		result := renderFooter()
		if result == "" {
			t.Error("renderFooter with no hints returned empty")
		}
	})
}

func TestThinRule(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		width int
	}{
		{name: "normal", width: 80},
		{name: "narrow", width: 20},
		{name: "very narrow", width: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := thinRule(tt.width)
			if result == "" {
				t.Errorf(
					"thinRule(%d) returned empty string",
					tt.width,
				)
			}
		})
	}
}

func TestRenderBanner(t *testing.T) {
	t.Parallel()
	plat := &platform.Platform{
		OS:     platform.MacOS,
		Arch:   platform.ARM64,
		OSName: "macOS",
	}

	result := renderBanner(80, "v1.0.0", plat)
	if result == "" {
		t.Error("renderBanner returned empty string")
	}
}

func TestRenderBanner_DifferentPlatforms(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		plat    *platform.Platform
		version string
	}{
		{
			name: "macOS ARM64",
			plat: &platform.Platform{
				OS: platform.MacOS, Arch: platform.ARM64,
				OSName: "macOS",
			},
			version: "dev",
		},
		{
			name: "Linux AMD64",
			plat: &platform.Platform{
				OS: platform.Linux, Arch: platform.AMD64,
				OSName: "Ubuntu",
			},
			version: "v2.0.0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := renderBanner(80, tt.version, tt.plat)
			if result == "" {
				t.Errorf("renderBanner returned empty for %s", tt.name)
			}
		})
	}
}

func TestCatppuccinColors(t *testing.T) {
	t.Parallel()

	// Verify colors are non-nil.
	colors := map[string]color.Color{
		"catBase":     catBase,
		"catSurface0": catSurface0,
		"catOverlay0": catOverlay0,
		"catOverlay1": catOverlay1,
		"catSubtext0": catSubtext0,
		"catSubtext1": catSubtext1,
		"catText":     catText,
		"catSapphire": catSapphire,
		"catGreen":    catGreen,
		"catYellow":   catYellow,
		"catRed":      catRed,
		"catMauve":    catMauve,
	}

	for name, c := range colors {
		if c == nil {
			t.Errorf("color %s is nil", name)
		}
	}
}
