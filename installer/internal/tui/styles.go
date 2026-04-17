package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

// ---------------------------------------------------------------------------
// TokyoNight Night palette
// ---------------------------------------------------------------------------

var (
	catBase     = lipgloss.Color("#1a1b26") // Darkest – full-screen bg
	catSurface0 = lipgloss.Color("#292e42")
	catOverlay0 = lipgloss.Color("#545c7e")
	catOverlay1 = lipgloss.Color("#565f89")
	catSubtext0 = lipgloss.Color("#9aa5ce")
	catSubtext1 = lipgloss.Color("#a9b1d6")
	catText     = lipgloss.Color("#c0caf5")
	catSapphire = lipgloss.Color("#2ac3de")
	catGreen    = lipgloss.Color("#9ece6a")
	catYellow   = lipgloss.Color("#e0af68")
	catRed      = lipgloss.Color("#f7768e")
	catMauve    = lipgloss.Color("#bb9af7")
)

// ---------------------------------------------------------------------------
// Styles — panel-interior styles carry explicit Background(catSurface0) so
// that child ANSI resets (\x1b[0m) never leak catBase into the panel.
// Screen-level styles no longer need explicit Background(catBase) because
// tea.View.BackgroundColor sets the terminal background at the VT level.
// The `cat*` identifier prefix is historical (palette was originally
// Catppuccin Mocha) — kept to avoid churn across styles.go/tests.
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
			Foreground(catOverlay1)

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

	// Footer (screen level – outside panels; VT bg handles catBase)
	footerStyle    = lipgloss.NewStyle().Foreground(catOverlay0).MarginTop(1)
	footerKeyStyle = lipgloss.NewStyle().Foreground(catSubtext0)

	// Section headers inside panels
	stepStyle = lipgloss.NewStyle().Bold(true).Foreground(catSapphire).Background(catSurface0)
)

// panelGap wraps bare whitespace/newlines with the panel interior background
// so ANSI resets from child styles never leak the terminal's default background.
func panelGap(s string) string {
	return lipgloss.NewStyle().Background(catSurface0).Render(s)
}

// ---------------------------------------------------------------------------
// Banner — clean styled text, no ASCII art
// ---------------------------------------------------------------------------

func renderBanner(_ int, version string, plat *platform.Platform) string {
	bannerTitle := lipgloss.NewStyle().
		Bold(true).
		Foreground(catMauve).
		Render("  dotsetup")

	dot := lipgloss.NewStyle().Foreground(catOverlay0).Render(" · ")
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
	dot := lipgloss.NewStyle().Foreground(catOverlay0).Render(" · ")
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

// footerBlock renders a centered footer beneath a panel of the given
// content width.
func footerBlock(cw int, hints ...string) string {
	return lipgloss.NewStyle().
		Width(panelOuterWidth(cw)).
		AlignHorizontal(lipgloss.Center).
		Render(renderFooter(hints...))
}

// thinRule renders a dim horizontal line.
func thinRule(width int) string {
	w := width - 4
	if w < 10 {
		w = 10
	}
	return dimStyle.Render(strings.Repeat("━", w))
}

