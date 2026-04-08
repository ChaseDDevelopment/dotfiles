package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

// PlanRow holds one row of the dry-run summary table.
type PlanRow struct {
	Component string
	Action    string
	Status    string
}

// summaryModel displays the completion screen or dry-run plan.
type summaryModel struct {
	rows            []PlanRow
	steps           []stepResult
	dryRun          bool
	criticalFailure bool // true if a critical tool failed
	startTime       time.Time
	endTime         time.Time
	viewport        viewport.Model
	viewportReady   bool
}

func newSummaryModel(dryRun bool) summaryModel {
	return summaryModel{dryRun: dryRun}
}

func (m summaryModel) View(width, height int) string {
	if m.dryRun {
		return m.dryRunView(width, height)
	}
	return m.completionView(width)
}

func (m summaryModel) completionView(width int) string {
	w := contentWidth(width)
	var b strings.Builder

	centerWrap := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(w - 6).
		Background(catSurface0)

	if m.criticalFailure {
		header := errorStyle.Render("  ✗  Install Aborted — Critical Tool Failed  ✗")
		b.WriteString(centerWrap.Render(header))
	} else {
		header := titleStyle.Render("  ✦  Setup Complete  ✦")
		b.WriteString(centerWrap.Render(header))
	}
	b.WriteString("\n\n")

	// Stats row.
	succeeded := 0
	failed := 0
	for _, s := range m.steps {
		if s.success {
			succeeded++
		} else {
			failed++
		}
	}

	elapsed := m.endTime.Sub(m.startTime).Round(100 * time.Millisecond)
	dot := statsDividerStyle.Render(" · ")

	statsLine := statsStyle.Render(fmt.Sprintf("%d steps", len(m.steps))) + dot +
		statsStyle.Render(elapsed.String())
	b.WriteString(centerWrap.Render(statsLine))
	b.WriteString("\n\n")

	// Success/failure counts.
	counts := successStyle.Render(fmt.Sprintf("✓ %d succeeded", succeeded))
	if failed > 0 {
		counts += "     " + errorStyle.Render(fmt.Sprintf("✗ %d failed", failed))
	}
	b.WriteString(centerWrap.Render(counts))

	// Quick start section — inside the panel.
	b.WriteString("\n\n")
	b.WriteString(stepStyle.Render("  Quick Start"))
	b.WriteString("\n")
	b.WriteString(thinRule(w))
	b.WriteString("\n")

	quickItems := []struct{ cmd, desc string }{
		{"exec zsh", "Reload shell"},
		{"tmux", "Start tmux"},
		{"nvim", "Open Neovim"},
	}
	for _, item := range quickItems {
		b.WriteString(fmt.Sprintf(" %s  %s\n",
			selectedStyle.Render(fmt.Sprintf("%-16s", item.cmd)),
			descStyle.Render(item.desc),
		))
	}

	// Wrap everything in panel.
	borderColor := catGreen
	if m.criticalFailure {
		borderColor = catRed
	}
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Background(catSurface0).
		Padding(1, 2).
		Width(w).
		Render(b.String())

	footer := renderFooter("q exit", "enter exit")
	footerBlock := lipgloss.NewStyle().
		Width(panelOuterWidth(w)).
		AlignHorizontal(lipgloss.Center).
		Background(catBase).
		Render(footer)

	return lipgloss.JoinVertical(lipgloss.Left, panel, footerBlock)
}

// dryRunTableRows renders the styled table body rows as a single string.
// Every character is inside a styled span with explicit Background() to
// prevent transparency leaks from SGR resets between spans.
func (m summaryModel) dryRunTableRows(w int) string {
	innerW := w - dryRunPanelStyle.GetHorizontalPadding() -
		dryRunPanelStyle.GetHorizontalBorderSize()

	const (
		indentW = 2
		compW   = 28
		actionW = 12
	)
	statusW := innerW - indentW - compW - actionW
	if statusW < 10 {
		statusW = 10
	}

	indent := lipgloss.NewStyle().
		Width(indentW).Background(catSurface0).Render("")

	var b strings.Builder
	for _, row := range m.rows {
		var statusCell string
		switch row.Status {
		case "would install", "would configure", "would replace":
			statusCell = warnStyle.Width(statusW).Render(row.Status)
		case "already installed", "already configured":
			statusCell = successStyle.Width(statusW).Render(row.Status)
		default:
			statusCell = dimStyle.Width(statusW).Render(row.Status)
		}
		// Concatenate styled spans directly — no bare characters.
		b.WriteString(indent)
		b.WriteString(tableCellStyle.Width(compW).Render(row.Component))
		b.WriteString(tableCellStyle.Width(actionW).Render(row.Action))
		b.WriteString(statusCell)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// dryRunFixedHeaderHeight returns the number of lines used by the fixed
// header above the scrollable table body (title + blank + table header + rule).
const dryRunFixedHeaderLines = 4

func (m summaryModel) dryRunView(width, height int) string {
	w := contentWidth(width)

	innerW := w - dryRunPanelStyle.GetHorizontalPadding() -
		dryRunPanelStyle.GetHorizontalBorderSize()

	// Full-width wrapper ensures no transparency leaks between styled spans.
	fullRow := lipgloss.NewStyle().
		Width(innerW).
		Background(catSurface0)

	// --- Fixed header ---
	var hdr strings.Builder
	header := warnStyle.Bold(true).Render("DRY RUN — No changes were made")
	hdr.WriteString(lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(innerW).
		Background(catSurface0).
		Render(header))
	hdr.WriteString("\n")
	hdr.WriteString(fullRow.Render(""))
	hdr.WriteString("\n")

	if len(m.rows) == 0 {
		hdr.WriteString(fullRow.Render(dimStyle.Render("  No actions planned.")))
		hdr.WriteString("\n")

		panel := dryRunPanelStyle.Width(w).Render(hdr.String())
		footer := renderFooter("q exit", "enter exit")
		footerBlock := lipgloss.NewStyle().
			Width(panelOuterWidth(w)).
			AlignHorizontal(lipgloss.Center).
			Background(catBase).
			Render(footer)
		return lipgloss.JoinVertical(lipgloss.Left, panel, footerBlock)
	}

	// Table header row + divider (part of fixed header).
	// Each span carries its own Background — no bare characters between spans.
	const (
		indentW = 2
		compW   = 28
		actionW = 12
	)
	statusW := innerW - indentW - compW - actionW
	if statusW < 10 {
		statusW = 10
	}
	thIndent := lipgloss.NewStyle().
		Width(indentW).Background(catSurface0).Render("")
	hdr.WriteString(thIndent)
	hdr.WriteString(tableHeaderStyle.Width(compW).Render("Component"))
	hdr.WriteString(tableHeaderStyle.Width(actionW).Render("Action"))
	hdr.WriteString(tableHeaderStyle.Width(statusW).Render("Status"))
	hdr.WriteString("\n")
	hdr.WriteString(dimStyle.Width(innerW).Render(
		"  " + strings.Repeat("─", innerW-4)))

	headerStr := hdr.String()

	// --- Scrollable table body via viewport ---
	tableBody := m.dryRunTableRows(w)

	// Calculate available height for the viewport.
	// Panel border (top+bottom) = 2, panel padding (top+bottom) = 2,
	// footer = 2 lines (margin-top 1 + content 1), fixed header lines.
	panelChrome := dryRunPanelStyle.GetVerticalBorderSize() +
		dryRunPanelStyle.GetVerticalPadding()
	footerLines := 2
	availableRows := height - panelChrome - footerLines - dryRunFixedHeaderLines
	if availableRows < 5 {
		availableRows = 5
	}

	// If all rows fit, skip viewport overhead.
	totalRows := len(m.rows)
	needsScroll := totalRows > availableRows

	var body string
	if needsScroll && m.viewportReady {
		body = m.viewport.View()
	} else {
		body = tableBody
	}

	// Combine fixed header + body inside the panel.
	panelContent := headerStr + "\n" + body
	panel := dryRunPanelStyle.Width(w).Render(panelContent)

	// Footer with scroll hint.
	hints := "q exit"
	if needsScroll {
		hints = "↑↓ scroll · q exit"
	}
	footer := renderFooter(hints, "enter exit")
	footerBlock := lipgloss.NewStyle().
		Width(panelOuterWidth(w)).
		AlignHorizontal(lipgloss.Center).
		Background(catBase).
		Render(footer)

	return lipgloss.JoinVertical(lipgloss.Left, panel, footerBlock)
}

// initViewport sets up the viewport for the dry-run table body.
func (m *summaryModel) initViewport(width, height int) {
	w := contentWidth(width)
	innerW := w - dryRunPanelStyle.GetHorizontalPadding() -
		dryRunPanelStyle.GetHorizontalBorderSize()
	panelChrome := dryRunPanelStyle.GetVerticalBorderSize() +
		dryRunPanelStyle.GetVerticalPadding()
	footerLines := 2
	availableRows := height - panelChrome - footerLines - dryRunFixedHeaderLines
	if availableRows < 5 {
		availableRows = 5
	}

	m.viewport = viewport.New(innerW, availableRows)
	m.viewport.SetContent(m.dryRunTableRows(w))
	m.viewport.Style = lipgloss.NewStyle().
		Width(innerW).
		Background(catSurface0)
	m.viewportReady = true
}
