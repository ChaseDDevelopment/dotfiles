package tui

import (
	"fmt"
	"strings"
	"time"

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
}

func newSummaryModel(dryRun bool) summaryModel {
	return summaryModel{dryRun: dryRun}
}

func (m summaryModel) View(width int) string {
	if m.dryRun {
		return m.dryRunView(width)
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

func (m summaryModel) dryRunView(width int) string {
	w := contentWidth(width)
	var b strings.Builder

	// Header.
	header := warnStyle.Bold(true).Render("DRY RUN — No changes were made")
	b.WriteString(lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(w - 6).
		Background(catSurface0).
		Render(header))
	b.WriteString("\n\n")

	if len(m.rows) == 0 {
		b.WriteString(dimStyle.Render("  No actions planned."))
		b.WriteString("\n")
	} else {
		// Table header — pad plain text first, then apply style.
		b.WriteString(fmt.Sprintf("  %s %s %s\n",
			tableHeaderStyle.Width(28).Render("Component"),
			tableHeaderStyle.Width(12).Render("Action"),
			tableHeaderStyle.Render("Status"),
		))
		b.WriteString(dimStyle.Render("  "+strings.Repeat("─", w-8)))
		b.WriteString("\n")

		for _, row := range m.rows {
			var status string
			switch row.Status {
			case "would install", "would configure":
				status = warnStyle.Render(row.Status)
			case "already installed":
				status = successStyle.Render(row.Status)
			default:
				status = dimStyle.Render(row.Status)
			}
			b.WriteString(fmt.Sprintf("  %s %s %s\n",
				tableCellStyle.Width(28).Render(row.Component),
				tableCellStyle.Width(12).Render(row.Action),
				status,
			))
		}
	}

	// Wrap in panel.
	panel := dryRunPanelStyle.Width(w).Render(b.String())
	footer := renderFooter("q exit", "enter exit")
	footerBlock := lipgloss.NewStyle().
		Width(panelOuterWidth(w)).
		AlignHorizontal(lipgloss.Center).
		Background(catBase).
		Render(footer)

	return lipgloss.JoinVertical(lipgloss.Left, panel, footerBlock)
}
