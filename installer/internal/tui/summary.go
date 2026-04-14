package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"

	"github.com/chaseddevelopment/dotfiles/installer/internal/config"
	"github.com/chaseddevelopment/dotfiles/installer/internal/orchestrator"
)

// summaryModel displays the completion screen or dry-run plan.
type summaryModel struct {
	rows             []orchestrator.PlanRow
	steps            []stepResult
	dryRun           bool
	doctorMode       bool   // true when displaying doctor results
	criticalFailure  bool   // true if a critical tool failed
	logPath          string // path to install.log for display
	alreadyInstalled  int   // tools skipped because already present
	alreadyConfigured int   // components skipped because configs match

	// Names behind the counts above, rendered as dim manifest blocks
	// beneath the Results table so a clean no-op run still tells the
	// user exactly which tools and components were inspected instead
	// of leaving them to trust an opaque "46 already installed" pill.
	alreadyInstalledNames  []string
	alreadyConfiguredNames []string
	startTime        time.Time
	endTime          time.Time
	viewport         viewport.Model
	viewportReady    bool

	// Per-tool timing: tracked from TaskStartedMsg to TaskDoneMsg.
	startTimes map[string]time.Time
	durations  map[string]time.Duration

	// warnings holds best-effort post-install failures recorded
	// during the run. Rendered beneath the main summary table so
	// users see what didn't quite succeed even when the overall
	// install "passed".
	warnings *config.TrackedFailures

	// repoBlockedBody is set when `git pull --ff-only` hard-failed
	// (local changes or non-fast-forward). The full git output is
	// rendered on the summary so the user sees which files to stash
	// or commit before re-running.
	repoBlockedBody string

	// shellReloadPending mirrors AppModel.shellReloadPending so the
	// completion view can show the "shell will reload on quit" hint
	// and retune the footer. Set before rendering by updateSummary.
	shellReloadPending bool
}

func newSummaryModel(dryRun bool) summaryModel {
	return summaryModel{
		dryRun:     dryRun,
		startTimes: make(map[string]time.Time),
		durations:  make(map[string]time.Duration),
	}
}

// formatDuration returns a compact human-readable duration string.
func formatDuration(d time.Duration) string {
	switch {
	case d < time.Second:
		ms := d.Milliseconds()
		if ms == 0 {
			return "<1ms"
		}
		return fmt.Sprintf("%dms", ms)
	case d < time.Minute:
		return fmt.Sprintf("%.1fs", d.Seconds())
	default:
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", m, s)
	}
}

func (m summaryModel) View(width, height int) string {
	if m.dryRun {
		return m.dryRunView(width, height)
	}
	return m.completionView(width, height)
}

func (m summaryModel) completionView(width, height int) string {
	w := contentWidth(width)
	var b strings.Builder

	centerWrap := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(w - 6).
		Background(catSurface0)

	if m.criticalFailure && m.repoBlockedBody != "" {
		header := errorStyle.Render("  ✗  Install Aborted — Repo Sync Blocked  ✗")
		b.WriteString(centerWrap.Render(header))
		b.WriteString(panelGap("\n\n"))
		b.WriteString(dimStyle.Render(
			"  Uncommitted local changes prevent `git pull --ff-only`.",
		))
		b.WriteString(panelGap("\n"))
		b.WriteString(dimStyle.Render(
			"  Stash or commit them, then re-run the installer.",
		))
		b.WriteString(panelGap("\n\n"))
		for _, line := range strings.Split(m.repoBlockedBody, "\n") {
			line = strings.TrimRight(line, "\r")
			if line == "" {
				continue
			}
			if max := w - 4; len(line) > max && max > 3 {
				line = line[:max-1] + "…"
			}
			b.WriteString(dimStyle.Render("  " + line))
			b.WriteString(panelGap("\n"))
		}
	} else if m.criticalFailure {
		header := errorStyle.Render("  ✗  Install Aborted — Critical Tool Failed  ✗")
		b.WriteString(centerWrap.Render(header))
	} else if m.doctorMode {
		header := titleStyle.Render("  ✦  Doctor Results  ✦")
		b.WriteString(centerWrap.Render(header))
	} else {
		header := titleStyle.Render("  ✦  Setup Complete  ✦")
		b.WriteString(centerWrap.Render(header))
	}
	b.WriteString(panelGap("\n\n"))

	// Filter out no-op housekeeping steps before the rest of this
	// view reasons about the results. The drift sweep succeeds
	// silently when there is no drift; counting it as "installed" or
	// even rendering its row on a clean run confused users into
	// thinking an install happened when nothing actually changed.
	visibleSteps := m.visibleSteps()

	// Categorize results. Skipped is distinct from failed — a
	// skipped step didn't run (usually because a dependency failed,
	// or `git pull` couldn't advance) whereas failed ran and errored.
	// Merging them hides the "tasks we silently didn't even try"
	// class of bug.
	installed := 0
	configured := 0
	failed := 0
	skipped := 0
	for _, s := range visibleSteps {
		switch {
		case s.action == "skipped":
			skipped++
		case !s.success:
			failed++
		case s.action == "install":
			installed++
		default:
			configured++
		}
	}

	// DEGRADED banner when anything failed, skipped, or emitted a
	// best-effort warning. A single missed step shouldn't hide in a
	// 300-line log — surface it here so the user sees the run
	// wasn't clean before they walk away from their terminal.
	warningCount := len(m.warnings.Snapshot())
	if failed+skipped+warningCount > 0 && !m.criticalFailure {
		parts := []string{}
		if failed > 0 {
			parts = append(parts, fmt.Sprintf("%d failed", failed))
		}
		if skipped > 0 {
			parts = append(parts, fmt.Sprintf("%d skipped", skipped))
		}
		if warningCount > 0 {
			parts = append(parts, fmt.Sprintf("%d warning(s)", warningCount))
		}
		degradedLine := warnStyle.Bold(true).Render(fmt.Sprintf(
			"  ⚠  DEGRADED — %s  ⚠", strings.Join(parts, ", "),
		))
		b.WriteString(panelGap("\n"))
		b.WriteString(centerWrap.Render(degradedLine))
		b.WriteString(panelGap("\n"))
	}

	if !m.startTime.IsZero() && !m.endTime.IsZero() {
		elapsed := m.endTime.Sub(m.startTime).Round(100 * time.Millisecond)
		statsLine := statsStyle.Render(elapsed.String())
		b.WriteString(centerWrap.Render(statsLine))
		b.WriteString(panelGap("\n\n"))
	}

	// Categorized counts.
	var parts []string
	if installed > 0 {
		parts = append(parts, successStyle.Render(
			fmt.Sprintf("✓ %d installed", installed)))
	}
	if configured > 0 {
		parts = append(parts, successStyle.Render(
			fmt.Sprintf("✓ %d configured", configured)))
	}
	if failed > 0 {
		parts = append(parts, errorStyle.Render(
			fmt.Sprintf("✗ %d failed", failed)))
	}
	if skipped > 0 {
		parts = append(parts, warnStyle.Render(
			fmt.Sprintf("⊘ %d skipped", skipped)))
	}
	if m.alreadyInstalled > 0 {
		parts = append(parts, dimStyle.Render(
			fmt.Sprintf("● %d already installed", m.alreadyInstalled)))
	}
	if m.alreadyConfigured > 0 {
		parts = append(parts, dimStyle.Render(
			fmt.Sprintf("● %d already configured", m.alreadyConfigured)))
	}
	if len(parts) == 0 {
		parts = append(parts, dimStyle.Render("No changes needed"))
	}
	b.WriteString(lipgloss.NewStyle().
		Background(catSurface0).
		Width(w - 4).
		Render("  " + strings.Join(parts, panelGap("   "))))

	// Results table — show what actually happened.
	// When in doctor mode, build the table body separately so it can
	// be placed inside a viewport for scrolling.
	var tableBody strings.Builder
	if len(visibleSteps) > 0 {
		b.WriteString(panelGap("\n\n"))
		b.WriteString(stepStyle.Render("  Results"))
		b.WriteString(panelGap("\n"))
		b.WriteString(thinRule(w))
		b.WriteString(panelGap("\n"))

		compW, statusW, durationW := resultsColumnWidths(w, visibleSteps)

		for _, s := range visibleSteps {
			tableBody.WriteString(renderResultRow(
				s, m.durations, compW, statusW, durationW,
			))
		}
	}

	// In doctor mode, use viewport for the results table when it
	// overflows the available terminal height.
	if m.doctorMode && m.viewportReady {
		b.WriteString(m.viewport.View())
	} else {
		b.WriteString(tableBody.String())
	}

	// Already-installed / already-configured manifest blocks. Counts
	// alone don't tell the user what was actually checked — if the
	// screen says "46 already installed" with nothing else, they
	// can't tell whether the installer even inspected the tools they
	// cared about. Listing names here turns the summary into a
	// verifiable manifest without flooding the Results table.
	if len(m.alreadyInstalledNames) > 0 {
		b.WriteString(panelGap("\n"))
		b.WriteString(renderAlreadyBlock(
			w,
			fmt.Sprintf("Already installed (%d)",
				len(m.alreadyInstalledNames)),
			m.alreadyInstalledNames,
		))
	}
	if len(m.alreadyConfiguredNames) > 0 {
		b.WriteString(panelGap("\n"))
		b.WriteString(renderAlreadyBlock(
			w,
			fmt.Sprintf("Already configured (%d)",
				len(m.alreadyConfiguredNames)),
			m.alreadyConfiguredNames,
		))
	}

	// Best-effort warnings — failures from post-install hooks that
	// don't fail the component but still deserve visibility.
	if snap := m.warnings.Snapshot(); len(snap) > 0 {
		b.WriteString(panelGap("\n"))
		b.WriteString(stepStyle.Render(fmt.Sprintf(
			"  Completed with %d warning(s)", len(snap),
		)))
		b.WriteString(panelGap("\n"))
		b.WriteString(thinRule(w))
		b.WriteString(panelGap("\n"))
		for _, row := range snap {
			line := fmt.Sprintf(
				"  • %s — %s: %v", row.Component, row.Step, row.Err,
			)
			if max := w - 4; len(line) > max && max > 3 {
				line = line[:max-1] + "…"
			}
			b.WriteString(dimStyle.Render(line) + panelGap("\n"))
		}
	}

	// Log file path.
	if m.logPath != "" && failed > 0 {
		b.WriteString(panelGap("\n"))
		b.WriteString(dimStyle.Render(
			fmt.Sprintf("  Log: %s", m.logPath)))
		b.WriteString(panelGap("\n"))
	}

	// Version / commit footer so the exact binary that ran is
	// visible without hunting through the log. Empty commit means
	// this is a local dev build; we still show the label so the
	// "not a release build" signal is obvious.
	if Version != "" || Commit != "" {
		shortCommit := Commit
		if len(shortCommit) > 7 {
			shortCommit = shortCommit[:7]
		}
		label := fmt.Sprintf("  dotsetup %s", Version)
		if shortCommit != "" {
			label += fmt.Sprintf(" (%s)", shortCommit)
		}
		b.WriteString(panelGap("\n"))
		b.WriteString(dimStyle.Render(label))
		b.WriteString(panelGap("\n"))
	}

	// Quick start section — only show items for tools that succeeded.
	if !m.doctorMode {
		succeeded := make(map[string]bool)
		for _, s := range m.steps {
			if s.success {
				succeeded[strings.ToLower(s.label)] = true
			}
		}

		type quickItem struct {
			cmd, desc, requires string
		}
		allQuick := []quickItem{
			{"exec zsh", "Reload shell", "zsh"},
			{"tmux", "Start tmux", "tmux"},
			{"nvim", "Open Neovim", "neovim"},
		}
		var quickItems []quickItem
		for _, item := range allQuick {
			if succeeded[item.requires] {
				quickItems = append(quickItems, item)
			}
		}

		if len(quickItems) > 0 {
			b.WriteString(panelGap("\n"))
			b.WriteString(stepStyle.Render("  Quick Start"))
			b.WriteString(panelGap("\n"))
			b.WriteString(thinRule(w))
			b.WriteString(panelGap("\n"))
			for _, item := range quickItems {
				b.WriteString(panelGap(" ") + selectedStyle.Render(fmt.Sprintf("%-16s", item.cmd)) +
					panelGap("  ") + descStyle.Render(item.desc) + panelGap("\n"))
			}
		}
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

	// Footer with scroll hint when doctor mode is scrollable.
	needsScroll := m.doctorMode && m.viewportReady
	quitHint := "q quit"
	if m.shellReloadPending {
		quitHint = "q reload & quit"
	}
	var footer string
	if needsScroll {
		footer = renderFooter("↑↓ scroll", "enter menu", quitHint)
	} else {
		footer = renderFooter("enter menu", quitHint)
	}
	footerBlock := lipgloss.NewStyle().
		Width(panelOuterWidth(w)).
		AlignHorizontal(lipgloss.Center).
		Render(footer)

	// Reload hint sits between the panel and the footer so the user
	// can't miss why `q` says "reload & quit".
	if m.shellReloadPending {
		reloadHint := lipgloss.NewStyle().
			Foreground(catMauve).
			Width(panelOuterWidth(w)).
			AlignHorizontal(lipgloss.Center).
			Render("↺ Your shell will reload when you quit")
		return lipgloss.JoinVertical(
			lipgloss.Left, panel, reloadHint, footerBlock,
		)
	}

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
		switch {
		case row.Status == "would install",
			row.Status == "would configure",
			row.Status == "would replace",
			strings.HasPrefix(row.Status, "outdated"):
			statusCell = warnStyle.Width(statusW).Render(row.Status)
		case row.Status == "already installed",
			row.Status == "already configured":
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
		footer := renderFooter("enter menu", "q quit")
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
	var footer string
	if needsScroll {
		footer = renderFooter("↑↓ scroll", "enter menu", "q quit")
	} else {
		footer = renderFooter("enter menu", "q quit")
	}
	footerBlock := lipgloss.NewStyle().
		Width(panelOuterWidth(w)).
		AlignHorizontal(lipgloss.Center).
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

	m.viewport = viewport.New(
		viewport.WithWidth(innerW),
		viewport.WithHeight(availableRows),
	)
	m.viewport.SetContent(m.dryRunTableRows(w))
	m.viewport.Style = lipgloss.NewStyle().
		Width(innerW).
		Background(catSurface0)
	m.viewportReady = true
}

// initDoctorViewport sets up the viewport for doctor mode results.
func (m *summaryModel) initDoctorViewport(width, height int) {
	w := contentWidth(width)

	// Estimate fixed header lines: title + blank + stats + blank +
	// counts + blank + "Results" + rule = ~8, plus panel chrome.
	const fixedHeaderLines = 8
	panelChrome := 2 + 2 // border (top+bottom) + padding (top+bottom)
	footerLines := 2
	availableRows := height - panelChrome - footerLines - fixedHeaderLines
	if availableRows < 5 {
		availableRows = 5
	}

	// Only set up viewport if results overflow.
	if len(m.steps) <= availableRows {
		m.viewportReady = false
		return
	}

	innerW := w - 4 - 2 // padding(1,2)*2=4 + border=2

	m.viewport = viewport.New(
		viewport.WithWidth(innerW),
		viewport.WithHeight(availableRows),
	)
	m.viewport.SetContent(m.doctorTableRows(w))
	m.viewport.Style = lipgloss.NewStyle().
		Width(innerW).
		Background(catSurface0)
	m.viewportReady = true
}

// doctorTableRows renders the doctor results table body for viewport
// content, matching the same format as completionView's table.
func (m summaryModel) doctorTableRows(width int) string {
	w := contentWidth(width)
	steps := m.visibleSteps()
	compW, statusW, durationW := resultsColumnWidths(w, steps)

	var b strings.Builder
	for _, s := range steps {
		b.WriteString(strings.TrimRight(renderResultRow(
			s, m.durations, compW, statusW, durationW,
		), "\n"))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// visibleSteps returns m.steps with silent-success housekeeping tasks
// filtered out. A successful sweep-repo-drift with no associated
// warning means there was no drift to restore; showing a row for it
// or counting it as "installed" misrepresents what the run did.
func (m summaryModel) visibleSteps() []stepResult {
	if len(m.steps) == 0 {
		return nil
	}
	repoWarned := false
	if m.warnings != nil {
		for _, w := range m.warnings.Snapshot() {
			if w.Component == "Repo" {
				repoWarned = true
				break
			}
		}
	}
	out := make([]stepResult, 0, len(m.steps))
	for _, s := range m.steps {
		if s.id == "sweep-repo-drift" && s.success && !repoWarned {
			continue
		}
		out = append(out, s)
	}
	return out
}

// resultsColumnWidths returns (name, status, duration) column widths
// sized to the longest label in the step set. Callers are responsible
// for rendering within the returned widths — labels exceeding compW
// get truncated in renderResultRow.
func resultsColumnWidths(
	w int, steps []stepResult,
) (compW, statusW, durationW int) {
	const (
		minCompW  = 14
		maxCompW  = 40
		durationW2 = 8
		minStatusW = 12
	)
	durationW = durationW2
	compW = minCompW
	for _, s := range steps {
		if l := len(s.label); l+2 > compW {
			compW = l + 2
		}
	}
	if compW > maxCompW {
		compW = maxCompW
	}
	statusW = w - 10 - compW - durationW
	if statusW < minStatusW {
		// Terminal is narrow; reclaim width from the name column
		// before letting status shrink past readability.
		over := minStatusW - statusW
		if compW-over >= minCompW {
			compW -= over
			statusW = minStatusW
		} else {
			statusW = minStatusW
		}
	}
	return compW, statusW, durationW
}

// renderResultRow emits one row of the Results table: truncated name
// (compW), status (statusW), duration (durationW). The action column
// was removed; status already encodes the action via its label and
// color, so repeating it was noise.
func renderResultRow(
	s stepResult,
	durations map[string]time.Duration,
	compW, statusW, durationW int,
) string {
	label := s.label
	if len(label) > compW-2 && compW > 3 {
		label = label[:compW-3] + "…"
	}

	var statusCell string
	switch {
	case !s.success:
		detail := "failed"
		if s.err != nil {
			msg := s.err.Error()
			maxLen := statusW - 2
			if maxLen > 0 && len(msg) > maxLen {
				msg = msg[:maxLen-1] + "…"
			}
			detail = msg
		}
		statusCell = errorStyle.Width(statusW).Render(detail)
	case s.action == "install":
		statusCell = successStyle.Width(statusW).Render("installed")
	case s.action == "configure":
		statusCell = successStyle.Width(statusW).Render("configured")
	case s.action == "sweep":
		statusCell = successStyle.Width(statusW).
			Render("configs restored")
	default:
		statusCell = dimStyle.Width(statusW).Render(s.status)
	}

	durationCell := dimStyle.Width(durationW).Render("")
	if d, ok := durations[s.label]; ok {
		durationCell = dimStyle.Width(durationW).
			Render(formatDuration(d))
	}

	return panelGap("  ") +
		tableCellStyle.Width(compW).Render(label) +
		statusCell +
		durationCell + panelGap("\n")
}

// renderAlreadyBlock renders a "Already installed (N)" heading
// followed by a dim soft-wrapped list of names. Width is set on the
// body style so lipgloss wraps long lists instead of overflowing the
// panel.
func renderAlreadyBlock(w int, heading string, names []string) string {
	if len(names) == 0 {
		return ""
	}
	body := lipgloss.NewStyle().
		Foreground(catOverlay1).
		Background(catSurface0).
		Width(w - 4).
		Render("  " + strings.Join(names, ", "))
	return stepStyle.Render("  "+heading) +
		panelGap("\n") + body + panelGap("\n")
}
