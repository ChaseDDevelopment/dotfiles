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
	catSurface0  = lipgloss.Color("#313244")
	catSurface1  = lipgloss.Color("#45475a")
	catOverlay0  = lipgloss.Color("#6c7086")
	catOverlay1  = lipgloss.Color("#7f849c")
	catSubtext0  = lipgloss.Color("#a6adc8")
	catSubtext1  = lipgloss.Color("#bac2de")
	catText      = lipgloss.Color("#cdd6f4")
	catLavender  = lipgloss.Color("#b4befe")
	catSapphire  = lipgloss.Color("#74c7ec")
	catGreen     = lipgloss.Color("#a6e3a1")
	catYellow    = lipgloss.Color("#f9e2af")
	catRed       = lipgloss.Color("#f38ba8")
	catMauve     = lipgloss.Color("#cba6f7")
	catPink      = lipgloss.Color("#f5c2e7")
)

// ---------------------------------------------------------------------------
// Styles — NO Background() on inline styles, only on containers
// ---------------------------------------------------------------------------

var (
	// Containers — these are the ONLY styles with Background
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

	// Text hierarchy
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(catMauve).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(catOverlay1)

	dimStyle = lipgloss.NewStyle().
			Foreground(catOverlay0)

	// Status
	successStyle = lipgloss.NewStyle().Foreground(catGreen)
	warnStyle    = lipgloss.NewStyle().Foreground(catYellow)
	errorStyle   = lipgloss.NewStyle().Foreground(catRed)

	// Menu
	selectedStyle = lipgloss.NewStyle().Foreground(catSapphire).Bold(true)
	menuDimStyle  = lipgloss.NewStyle().Foreground(catSubtext1)
	cursorStyle   = lipgloss.NewStyle().Foreground(catMauve)
	cursorAltStyle = lipgloss.NewStyle().Foreground(catLavender)
	checkStyle    = lipgloss.NewStyle().Foreground(catGreen)
	uncheckStyle  = lipgloss.NewStyle().Foreground(catOverlay0)
	descStyle     = lipgloss.NewStyle().Foreground(catSubtext0)
	iconStyle     = lipgloss.NewStyle().Foreground(catMauve)
	iconDimStyle  = lipgloss.NewStyle().Foreground(catSubtext0)

	// Progress
	progressDoneStyle   = lipgloss.NewStyle().Foreground(catGreen)
	progressActiveStyle = lipgloss.NewStyle().Foreground(catMauve).Bold(true)
	progressQueuedStyle = lipgloss.NewStyle().Foreground(catOverlay0)
	progressFailedStyle = lipgloss.NewStyle().Foreground(catRed)

	// Summary
	statsStyle        = lipgloss.NewStyle().Foreground(catSubtext1)
	statsDividerStyle = lipgloss.NewStyle().Foreground(catOverlay0)

	// Table
	tableHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(catMauve).PaddingRight(3)
	tableCellStyle   = lipgloss.NewStyle().Foreground(catText).PaddingRight(3)

	// Footer (outside panels)
	footerStyle    = lipgloss.NewStyle().Foreground(catOverlay0).MarginTop(1)
	footerKeyStyle = lipgloss.NewStyle().Foreground(catSubtext0)

	// Section headers inside panels
	stepStyle = lipgloss.NewStyle().Bold(true).Foreground(catSapphire)
	infoStyle = lipgloss.NewStyle().Foreground(catSubtext0)

	// Unused but kept for reference
	panelTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(catMauve)
)

// ---------------------------------------------------------------------------
// Banner — clean styled text, no ASCII art
// ---------------------------------------------------------------------------

func renderBanner(_ int, version string, plat *platform.Platform) string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(catMauve).
		Render("  dotsetup")

	dot := dimStyle.Render(" · ")
	sub := subtitleStyle.Render(" chaseddevelopment/dotfiles") +
		dot + subtitleStyle.Render(version) +
		dot + subtitleStyle.Render(plat.OSName+" "+plat.Arch.String())

	return title + "\n" + sub
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// renderFooter builds a key-hint footer line.
func renderFooter(hints ...string) string {
	parts := make([]string, len(hints))
	for i, h := range hints {
		parts[i] = footerKeyStyle.Render(h)
	}
	return footerStyle.Render(strings.Join(parts, dimStyle.Render(" · ")))
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

