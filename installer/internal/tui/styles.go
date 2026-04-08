package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

// ---------------------------------------------------------------------------
// Catppuccin Mocha palette
// ---------------------------------------------------------------------------

var (
	catBase     = lipgloss.Color("#1e1e2e") // Darkest – full-screen bg
	catSurface0 = lipgloss.Color("#313244")
	catOverlay0 = lipgloss.Color("#6c7086")
	catOverlay1  = lipgloss.Color("#7f849c")
	catSubtext0  = lipgloss.Color("#a6adc8")
	catSubtext1  = lipgloss.Color("#bac2de")
	catText      = lipgloss.Color("#cdd6f4")
	catLavender  = lipgloss.Color("#b4befe")
	catSapphire  = lipgloss.Color("#74c7ec")
	catGreen     = lipgloss.Color("#a6e3a1")
	catYellow    = lipgloss.Color("#f9e2af")
	catRed       = lipgloss.Color("#f38ba8")
	catMauve = lipgloss.Color("#cba6f7")
)

// ---------------------------------------------------------------------------
// Styles — every inline style carries an explicit Background so that child
// ANSI resets (\x1b[0m) never leak the terminal's transparent background.
//   • Panel-interior styles use catSurface0
//   • Screen-level styles (banner, footer) use catBase
// ---------------------------------------------------------------------------

var (
	// Containers
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(catMauve).
			Background(catSurface0).
			Padding(1, 2)

	dryRunPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(catYellow).
				Background(catSurface0).
				Padding(1, 2)

	// Text hierarchy (panel interior)
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(catMauve).
			Background(catSurface0).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(catOverlay1).
			Background(catBase)

	dimStyle = lipgloss.NewStyle().
			Foreground(catOverlay0).
			Background(catSurface0)

	// Status (panel interior)
	successStyle = lipgloss.NewStyle().Foreground(catGreen).Background(catSurface0)
	warnStyle    = lipgloss.NewStyle().Foreground(catYellow).Background(catSurface0)
	errorStyle   = lipgloss.NewStyle().Foreground(catRed).Background(catSurface0)

	// Menu (panel interior)
	selectedStyle  = lipgloss.NewStyle().Foreground(catSapphire).Bold(true).Background(catSurface0)
	menuDimStyle   = lipgloss.NewStyle().Foreground(catSubtext1).Background(catSurface0)
	cursorStyle = lipgloss.NewStyle().Foreground(catMauve).Background(catSurface0)
	checkStyle     = lipgloss.NewStyle().Foreground(catGreen).Background(catSurface0)
	uncheckStyle   = lipgloss.NewStyle().Foreground(catOverlay0).Background(catSurface0)
	descStyle      = lipgloss.NewStyle().Foreground(catSubtext0).Background(catSurface0)
	iconStyle      = lipgloss.NewStyle().Foreground(catMauve).Background(catSurface0)
	iconDimStyle   = lipgloss.NewStyle().Foreground(catSubtext0).Background(catSurface0)

	// Progress (panel interior)
	progressDoneStyle   = lipgloss.NewStyle().Foreground(catGreen).Background(catSurface0)
	progressActiveStyle = lipgloss.NewStyle().Foreground(catMauve).Bold(true).Background(catSurface0)
	progressQueuedStyle = lipgloss.NewStyle().Foreground(catOverlay0).Background(catSurface0)
	progressFailedStyle = lipgloss.NewStyle().Foreground(catRed).Background(catSurface0)

	// Summary (panel interior)
	statsStyle        = lipgloss.NewStyle().Foreground(catSubtext1).Background(catSurface0)
	statsDividerStyle = lipgloss.NewStyle().Foreground(catOverlay0).Background(catSurface0)

	// Table (panel interior)
	tableHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(catMauve).Background(catSurface0).PaddingRight(3)
	tableCellStyle   = lipgloss.NewStyle().Foreground(catText).Background(catSurface0).PaddingRight(3)

	// Footer (screen level – outside panels)
	footerStyle    = lipgloss.NewStyle().Foreground(catOverlay0).Background(catBase).MarginTop(1)
	footerKeyStyle = lipgloss.NewStyle().Foreground(catSubtext0).Background(catBase)

	// Section headers inside panels
	stepStyle = lipgloss.NewStyle().Bold(true).Foreground(catSapphire).Background(catSurface0)
)

// ---------------------------------------------------------------------------
// Banner — clean styled text, no ASCII art
// ---------------------------------------------------------------------------

func renderBanner(_ int, version string, plat *platform.Platform) string {
	bannerTitle := lipgloss.NewStyle().
		Bold(true).
		Foreground(catMauve).
		Background(catBase).
		Render("  dotsetup")

	dot := lipgloss.NewStyle().Foreground(catOverlay0).Background(catBase).Render(" · ")
	sub := subtitleStyle.Render(" chaseddevelopment/dotfiles") +
		dot + subtitleStyle.Render(version) +
		dot + subtitleStyle.Render(plat.OSName+" "+plat.Arch.String())

	return bannerTitle + "\n" + sub
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// renderFooter builds a key-hint footer line.
func renderFooter(hints ...string) string {
	dot := lipgloss.NewStyle().Foreground(catOverlay0).Background(catBase).Render(" · ")
	parts := make([]string, len(hints))
	for i, h := range hints {
		parts[i] = footerKeyStyle.Render(h)
	}
	return footerStyle.Render(strings.Join(parts, dot))
}

// panelOuterWidth returns the total rendered width of a panel, including
// its border, for a given content width.
func panelOuterWidth(cw int) int {
	return cw + panelStyle.GetHorizontalBorderSize()
}

// contentWidth returns a capped width for content.
func contentWidth(termWidth int) int {
	w := termWidth - 4
	if w > 80 {
		w = 80
	}
	if w < 40 {
		w = 40
	}
	return w
}

// thinRule renders a dim horizontal line.
func thinRule(width int) string {
	w := width - 4
	if w < 10 {
		w = 10
	}
	return dimStyle.Render(strings.Repeat("━", w))
}

