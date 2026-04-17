package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/chaseddevelopment/dotfiles/installer/internal/backup"
	"github.com/chaseddevelopment/dotfiles/installer/internal/config"
	"github.com/chaseddevelopment/dotfiles/installer/internal/engine"
	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/orchestrator"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
	"github.com/chaseddevelopment/dotfiles/installer/internal/state"
	"github.com/chaseddevelopment/dotfiles/installer/internal/pkgmgr"
)

// Phase represents the current UI phase.
type Phase int

const (
	PhaseMainMenu Phase = iota
	PhaseOptionsMenu
	PhaseComponentPicker
	PhaseBackupPicker
	PhaseDpkgRepair
	PhaseInstalling
	PhaseFailurePrompt
	PhaseSummary
)

// InstallMode represents the user's chosen operation.
type InstallMode int

const (
	ModeInstall InstallMode = iota
	ModeCustomInstall
	ModeDryRun
	ModeUpdate
	ModeRestore
	ModeDoctor
	ModeUninstall
	ModeExit
)

// AppConfig holds shared state across all phases.
type AppConfig struct {
	Mode               InstallMode
	DryRun             bool
	SkipPackages       bool
	SkipUpdate         bool
	SkipDevTools       bool
	Verbose            bool
	CleanBackup        bool
	ForceReinstall     bool
	SelectedComponents []string
	Platform           *platform.Platform
	PkgMgr             pkgmgr.PackageManager
	RootDir            string
	LogFile            *executor.LogFile
	Runner             *executor.Runner
	State              *state.Store
	SelectedBackup     string // path chosen by backup picker
	PlanRows           []orchestrator.PlanRow
	// Failures collects best-effort setup warnings for the summary
	// screen. Initialized fresh for each run in startInstall.
	Failures *config.TrackedFailures
}

// AppModel is the top-level Bubble Tea model.
type AppModel struct {
	phase    Phase
	config   *AppConfig
	mainMenu     mainMenuModel
	options      optionsMenuModel
	picker       componentPickerModel
	backupPicker backupPickerModel
	progress     progressModel
	summary      summaryModel
	width    int
	height   int
	quitting bool

	// Parallel engine event channel.
	eventCh <-chan engine.Event

	// cancelEngine cancels the engine context, stopping all running
	// tasks and preventing goroutine leaks on Ctrl+C or critical failure.
	cancelEngine context.CancelFunc

	// failedTaskLabel is set when a critical task fails, used by
	// the failure prompt to display which tool failed.
	failedTaskLabel string
	failedTaskErr   error

	// dpkgState is populated when the pre-flight dpkg health probe
	// flags an inconsistent state. It holds the reason text + audit
	// output that the PhaseDpkgRepair modal displays.
	dpkgState pkgmgr.DpkgState
	// dpkgApt references the concrete Apt manager so the modal can
	// set UserApprovedRepair based on user choice.
	dpkgApt *pkgmgr.Apt

	startTime time.Time

	// shellReloadPending is set once install or update mode finishes
	// successfully. It's sticky — the user can navigate back to the
	// menu and do more, but any eventual `q`/ctrl-c quit will exit
	// with code 10, which install.sh interprets as "exec into a
	// fresh login shell" so the user lands in a session with the
	// freshly-symlinked configs loaded.
	shellReloadPending bool
}

// ShellReloadPending reports whether the installer has armed the
// post-quit shell reload. main.go reads this after the Bubble Tea
// program exits and maps it to os.Exit(10) for install.sh to pick
// up. Exposed as a method so the field can stay unexported.
func (m AppModel) ShellReloadPending() bool {
	return m.shellReloadPending
}

// armShellReloadIfApplicable sets shellReloadPending when the
// current mode is one the user would want to reload their shell
// after (install / custom install / update — all mutate the config
// tree). Doctor / restore / uninstall are skipped: doctor made no
// changes, and restore/uninstall intentionally put the user back
// to a state where a reloaded zsh could be counterproductive.
// Skipped on critical failure — no point dropping them into a
// half-built shell.
func (m *AppModel) armShellReloadIfApplicable() {
	if m.config == nil {
		return
	}
	if m.summary.criticalFailure {
		return
	}
	switch m.config.Mode {
	case ModeInstall, ModeCustomInstall, ModeUpdate:
		if !m.shellReloadPending {
			m.shellReloadPending = true
			if m.config.Runner != nil && m.config.Runner.Log != nil {
				m.config.Runner.Log.Write(
					"shell reload armed: will exec fresh login " +
						"shell on quit",
				)
			}
		}
	}
}

// NewApp creates the initial application model.
func NewApp(cfg *AppConfig) AppModel {
	return AppModel{
		phase:    PhaseMainMenu,
		config:   cfg,
		mainMenu: newMainMenu(),
		options:  newOptionsMenu(),
		picker:   newComponentPicker(),
		progress: newProgressModel(),
		summary:  newSummaryModel(cfg.DryRun),
	}
}

func (m AppModel) Init() tea.Cmd {
	// Don't start the spinner tick here — it dies during PhaseMainMenu
	// because updateMainMenu doesn't forward spinner.TickMsg. The tick
	// chain is started in startInstall() when actually needed.
	return nil
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle global keys.
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch msg.String() {
		case "ctrl+c":
			if m.cancelEngine != nil {
				m.cancelEngine()
			}
			m.quitting = true
			return m, tea.Quit
		case "q":
			if m.phase != PhaseInstalling {
				m.quitting = true
				return m, tea.Quit
			}
		}
	}

	// Handle window size — capture locally and forward to every
	// sub-model that owns a viewport / layout. Summary must also
	// receive resizes; its viewport re-init happens in updateSummary.
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = msg.Width
		m.height = msg.Height
		m.progress, _ = m.progress.Update(msg)
		m.mainMenu, _ = m.mainMenu.Update(msg)
		m.options, _ = m.options.Update(msg)
		m.picker, _ = m.picker.Update(msg)
		m.backupPicker, _ = m.backupPicker.Update(msg)
		// Eagerly resize summary viewports so a resize arriving
		// outside PhaseSummary is still reflected when we get there.
		if m.summary.dryRun {
			m.summary.initViewport(msg.Width, msg.Height)
		} else if m.summary.doctorMode {
			m.summary.initDoctorViewport(msg.Width, msg.Height)
		}
	}

	switch m.phase {
	case PhaseMainMenu:
		return m.updateMainMenu(msg)
	case PhaseOptionsMenu:
		return m.updateOptionsMenu(msg)
	case PhaseComponentPicker:
		return m.updateComponentPicker(msg)
	case PhaseBackupPicker:
		return m.updateBackupPicker(msg)
	case PhaseDpkgRepair:
		return m.updateDpkgRepair(msg)
	case PhaseInstalling:
		return m.updateInstalling(msg)
	case PhaseFailurePrompt:
		return m.updateFailurePrompt(msg)
	case PhaseSummary:
		return m.updateSummary(msg)
	}

	return m, nil
}

func (m AppModel) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}

	w := m.width
	if w == 0 {
		w = 80
	}

	var content string
	switch m.phase {
	case PhaseMainMenu:
		cw := contentWidth(w)
		fullW := panelOuterWidth(cw)
		banner := renderBanner(w, Version, m.config.Platform)
		bannerBlock := lipgloss.NewStyle().
			Width(fullW).
			AlignHorizontal(lipgloss.Center).
			Render(banner)
		menu := m.mainMenu.View(w)
		content = lipgloss.JoinVertical(lipgloss.Center, bannerBlock, menu)
	case PhaseOptionsMenu:
		content = m.options.View(w)
	case PhaseComponentPicker:
		content = m.picker.View(w)
	case PhaseBackupPicker:
		content = m.backupPicker.View(w)
	case PhaseDpkgRepair:
		content = m.dpkgRepairView(w)
	case PhaseInstalling:
		content = m.progress.View(w)
	case PhaseFailurePrompt:
		content = m.failurePromptView(w)
	case PhaseSummary:
		m.summary.shellReloadPending = m.shellReloadPending
		content = m.summary.View(w, m.height)
	}

	// Wrap the content in a full-screen container.
	// tea.View.BackgroundColor = catBase sets the terminal background
	// at the VT level, so we no longer need explicit Background(catBase).
	if m.width > 0 && m.height > 0 {
		content = lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			AlignHorizontal(lipgloss.Center).
			AlignVertical(lipgloss.Center).
			Render(content)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.BackgroundColor = catBase
	return v
}

// Version is injected from main.
var Version = "dev"

// Commit is the build-time git SHA from main. Rendered in summary
// so users (and incident responders) can pin the exact installer
// that ran — the dock incident had a binary 21 commits behind main
// with nothing surfaced anywhere in the UI.
var Commit = ""

// CriticalFailure returns true if a critical install task failed and
// the user aborted. Exposed so main.go can propagate a non-zero exit
// code to shell scripts and CI wrappers instead of lying with 0.
func (m AppModel) CriticalFailure() bool {
	return m.summary.criticalFailure
}

// --------------------------------------------------------------------------
// Phase update handlers
// --------------------------------------------------------------------------

func (m AppModel) updateMainMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok && msg.String() == "enter" {
		mode := m.mainMenu.selected()
		m.config.Mode = mode
		switch mode {
		case ModeExit:
			m.quitting = true
			return m, tea.Quit
		case ModeDryRun:
			m.config.DryRun = true
			m.config.Mode = ModeInstall
			m.summary = newSummaryModel(true)
			m.phase = PhaseOptionsMenu
		case ModeUpdate, ModeDoctor:
			m.phase = PhaseInstalling
			return m, m.startInstall()
		case ModeRestore:
			m.backupPicker = newBackupPicker()
			m.phase = PhaseBackupPicker
			return m, nil
		case ModeCustomInstall:
			m.phase = PhaseOptionsMenu
		case ModeUninstall:
			m.phase = PhaseComponentPicker
		case ModeInstall:
			m.phase = PhaseOptionsMenu
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.mainMenu, cmd = m.mainMenu.Update(msg)
	return m, cmd
}

func (m AppModel) updateOptionsMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch msg.String() {
		case "enter":
			m.config.SkipUpdate = m.options.optionEnabled("skip_update")
			m.config.SkipPackages = m.options.optionEnabled("skip_packages")
			m.config.Verbose = m.options.optionEnabled("verbose")
			m.config.Runner.Verbose = m.config.Verbose
			m.config.CleanBackup = m.options.optionEnabled("clean_backup")
			m.config.ForceReinstall = m.options.optionEnabled("force_reinstall")
			// install_dev_tools defaults on; SkipDevTools is its inverse
			// so the zero-value of BuildConfig keeps today's behavior.
			m.config.SkipDevTools = !m.options.optionEnabled("install_dev_tools")

			if m.config.Mode == ModeCustomInstall {
				m.phase = PhaseComponentPicker
			} else {
				m.phase = PhaseInstalling
				return m, m.startInstall()
			}
			return m, nil
		case "esc", "backspace":
			m.phase = PhaseMainMenu
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.options, cmd = m.options.Update(msg)
	return m, cmd
}

func (m AppModel) updateComponentPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch msg.String() {
		case "enter":
			selected := m.picker.selectedComponents()
			if len(selected) == 0 {
				return m, nil
			}
			m.config.SelectedComponents = selected
			m.phase = PhaseInstalling
			return m, m.startInstall()
		case "esc", "backspace":
			if m.config.Mode == ModeUninstall {
				m.phase = PhaseMainMenu
			} else {
				m.phase = PhaseOptionsMenu
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)
	return m, cmd
}

func (m AppModel) updateBackupPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch msg.String() {
		case "enter":
			sel := m.backupPicker.selected()
			if sel == "" {
				return m, nil
			}
			m.config.SelectedBackup = sel
			m.phase = PhaseInstalling
			return m, m.startInstall()
		case "esc", "backspace":
			m.phase = PhaseMainMenu
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.backupPicker, cmd = m.backupPicker.Update(msg)
	return m, cmd
}

func (m AppModel) updateInstalling(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case engine.TaskStartedMsg:
		m.progress.markActive(msg.ID, msg.Label)
		name := stripLabelPrefix(msg.Label)
		m.summary.startTimes[name] = time.Now()
		return m, listenCmd(m.eventCh)

	case engine.TaskDoneMsg:
		m.progress.markDone(msg.ID, msg.Err)
		name := m.progress.nameForID(msg.ID)
		if start, ok := m.summary.startTimes[name]; ok {
			m.summary.durations[name] = time.Since(start)
		}
		// Save state incrementally so progress survives crashes.
		if msg.Err == nil {
			m.saveState()
		}
		// Prompt user on critical tool failure.
		if msg.Critical && msg.Err != nil {
			m.failedTaskLabel = name
			m.failedTaskErr = msg.Err
			m.phase = PhaseFailurePrompt
			return m, listenCmd(m.eventCh)
		}
		// Transition to summary if all tasks are finished —
		// don't wait solely for AllDoneMsg which can be missed.
		if m.progress.allFinished() {
			m.summary.steps = m.progress.steps
			m.summary.endTime = time.Now()
			m.summary.warnings = m.config.Failures
			m.phase = PhaseSummary
			m.armShellReloadIfApplicable()
			m.saveState()
			if m.summary.doctorMode && m.width > 0 && m.height > 0 {
				m.summary.initDoctorViewport(m.width, m.height)
			}
			return m, drainCmd(m.eventCh)
		}
		return m, listenCmd(m.eventCh)

	case engine.TaskSkippedMsg:
		m.progress.markSkipped(msg.ID, msg.Label, msg.Reason)
		return m, listenCmd(m.eventCh)

	case engine.BatchProgressMsg:
		// One item inside a shared package-manager batch finished
		// (e.g. brew `==> Pouring ripgrep --`). Flip it to done so
		// the grid advances during the batch instead of freezing on
		// whichever task won the resource race. The owning task's
		// own TaskDoneMsg fires shortly after when its Run closure
		// runs its post-install — markDone is idempotent so the
		// second call won't double-count in the summary.
		if msg.Label != "" {
			m.progress.labelByID[msg.ID] = msg.Label
		}
		m.progress.markDone(msg.ID, nil)
		name := m.progress.nameForID(msg.ID)
		if start, ok := m.summary.startTimes[name]; ok {
			m.summary.durations[name] = time.Since(start)
		}
		return m, listenCmd(m.eventCh)

	case engine.AllDoneMsg:
		m.summary.steps = m.progress.steps
		m.summary.endTime = time.Now()
		m.summary.warnings = m.config.Failures
		m.phase = PhaseSummary
		m.armShellReloadIfApplicable()
		m.saveState()
		if m.summary.doctorMode && m.width > 0 && m.height > 0 {
			m.summary.initDoctorViewport(m.width, m.height)
		}
		return m, nil

	case repoSyncedMsg:
		// syncRepo finished (outcome already logged inside the Cmd).
		// Kick off the engine now.
		return m, m.runInstallTasks()

	case repoSyncBlockedMsg:
		// Hard failure: local changes block the pull, so continuing
		// would run a stale installer against newer configs. Abort
		// straight to summary with a clear, actionable error screen
		// rather than a 1-line NOTE in a 300-line log.
		m.summary.steps = nil
		m.summary.endTime = time.Now()
		m.summary.criticalFailure = true
		m.summary.repoBlockedBody = strings.TrimSpace(msg.body)
		m.phase = PhaseSummary
		return m, nil

	default:
		// Forward spinner ticks, progress frames, etc.
		if m.config.Verbose && m.config.Runner != nil {
			m.progress.recentLines = m.config.Runner.RecentLinesSnapshot()
			m.progress.syncVerboseViewport()
		}
		var cmd tea.Cmd
		m.progress, cmd = m.progress.Update(msg)
		return m, cmd
	}
}

// preflightDpkgHealth runs a read-only dpkg audit for apt-based
// systems. If the audit reports inconsistency, the method captures
// state + switches to PhaseDpkgRepair so the user can authorize a
// repair before any apt install runs. Returns (cmd, true) when the
// engine should be blocked; (nil, false) when the probe passes or
// the manager isn't apt.
func (m *AppModel) preflightDpkgHealth() (tea.Cmd, bool) {
	apt, ok := m.config.PkgMgr.(*pkgmgr.Apt)
	if !ok {
		return nil, false
	}
	// Consent flag resets per-session — the user must re-authorize
	// for each install run even if they previously approved. This
	// prevents a stale "yes" from a prior dotsetup invocation from
	// silently granting repair on a fresh run.
	apt.UserApprovedRepair = false
	state, err := apt.DetectDpkgHealth(context.Background())
	if err != nil {
		// A probe failure is logged but not fatal — let the engine
		// run; individual apt invocations still go through the
		// classifier and retry-after-heal path.
		if m.config.Runner != nil && m.config.Runner.Log != nil {
			m.config.Runner.Log.Write(fmt.Sprintf(
				"dpkg health probe failed: %v (continuing without pre-flight)",
				err,
			))
		}
		return nil, false
	}
	if state.Healthy {
		return nil, false
	}
	m.dpkgState = state
	m.dpkgApt = apt
	m.phase = PhaseDpkgRepair
	return nil, true
}

// updateDpkgRepair handles the user's decision on the pre-install
// dpkg-repair modal. R=authorize repair, S=skip (apt tools will
// surface as failed with a clear reason), A/ESC/Q=abort the run.
func (m AppModel) updateDpkgRepair(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "r", "R":
		if m.dpkgApt != nil {
			m.dpkgApt.UserApprovedRepair = true
		}
		if m.config.Runner != nil && m.config.Runner.Log != nil {
			m.config.Runner.Log.Write(
				"dpkg doctor: user authorized repair",
			)
		}
		return &m, m.runInstallTasks()
	case "s", "S":
		if m.dpkgApt != nil {
			m.dpkgApt.UserApprovedRepair = false
		}
		if m.config.Runner != nil && m.config.Runner.Log != nil {
			m.config.Runner.Log.Write(
				"dpkg doctor: user declined repair; apt tools will fail",
			)
		}
		return &m, m.runInstallTasks()
	case "a", "A", "q", "Q", "esc":
		if m.cancelEngine != nil {
			m.cancelEngine()
		}
		m.summary.endTime = time.Now()
		m.summary.criticalFailure = true
		m.phase = PhaseSummary
		if m.config.Runner != nil && m.config.Runner.Log != nil {
			m.config.Runner.Log.Write(
				"dpkg doctor: user aborted; no tool tasks ran",
			)
		}
		return m, nil
	}
	return m, nil
}

// dpkgRepairView renders the repair-consent modal. The reason text
// comes from DetectDpkgHealth — `dpkg --audit` output summarized,
// with stale /var/lib/dpkg/updates entries called out separately.
func (m AppModel) dpkgRepairView(width int) string {
	w := contentWidth(width)
	var b strings.Builder

	b.WriteString(errorStyle.Render("  ⚠  dpkg state inconsistent"))
	b.WriteString(panelGap("\n\n"))

	b.WriteString(panelGap("  "))
	b.WriteString(selectedStyle.Render("Reason: "))
	b.WriteString(panelGap(m.dpkgState.Reason))
	b.WriteString(panelGap("\n"))

	if m.dpkgState.AuditOutput != "" {
		b.WriteString(panelGap("  "))
		b.WriteString(dimStyle.Render("dpkg --audit: "))
		summary := m.dpkgState.AuditOutput
		if len(summary) > 240 {
			summary = summary[:240] + "…"
		}
		b.WriteString(panelGap(summary))
		b.WriteString(panelGap("\n"))
	}
	if len(m.dpkgState.StaleUpdates) > 0 {
		b.WriteString(panelGap("  "))
		b.WriteString(dimStyle.Render("Stale transactions: "))
		b.WriteString(panelGap(strings.Join(m.dpkgState.StaleUpdates, ", ")))
		b.WriteString(panelGap("\n"))
	}
	b.WriteString(panelGap("\n"))

	b.WriteString(panelGap("  Running "))
	b.WriteString(selectedStyle.Render("sudo dpkg --configure -a"))
	b.WriteString(panelGap(" will repair it. This is the standard\n"))
	b.WriteString(panelGap("  Debian/Ubuntu recovery and is generally safe.\n\n"))
	b.WriteString(panelGap("  Without repair, apt-based tool installs will fail.\n\n"))

	b.WriteString(panelGap("  "))
	b.WriteString(selectedStyle.Render("[R]"))
	b.WriteString(panelGap(" Run repair   "))
	b.WriteString(selectedStyle.Render("[S]"))
	b.WriteString(panelGap(" Skip (apt tools may fail)   "))
	b.WriteString(errorStyle.Render("[A]"))
	b.WriteString(panelGap(" Abort   "))
	b.WriteString(dimStyle.Render("(ESC = Abort)"))
	b.WriteString(panelGap("\n"))

	return panelStyle.Width(w).Render(b.String())
}

func (m AppModel) updateFailurePrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Process engine events while showing the prompt so progress
	// state stays current and events are not silently discarded.
	switch msg := msg.(type) {
	case engine.TaskStartedMsg:
		m.progress.markActive(msg.ID, msg.Label)
		name := stripLabelPrefix(msg.Label)
		m.summary.startTimes[name] = time.Now()
		return m, listenCmd(m.eventCh)
	case engine.TaskDoneMsg:
		m.progress.markDone(msg.ID, msg.Err)
		name := m.progress.nameForID(msg.ID)
		if start, ok := m.summary.startTimes[name]; ok {
			m.summary.durations[name] = time.Since(start)
		}
		if msg.Err == nil {
			m.saveState()
		}
		return m, listenCmd(m.eventCh)
	case engine.TaskSkippedMsg:
		m.progress.markSkipped(msg.ID, msg.Label, msg.Reason)
		return m, listenCmd(m.eventCh)
	case engine.BatchProgressMsg:
		if msg.Label != "" {
			m.progress.labelByID[msg.ID] = msg.Label
		}
		m.progress.markDone(msg.ID, nil)
		name := m.progress.nameForID(msg.ID)
		if start, ok := m.summary.startTimes[name]; ok {
			m.summary.durations[name] = time.Since(start)
		}
		return m, listenCmd(m.eventCh)
	case engine.AllDoneMsg:
		// Engine finished while user is deciding. Auto-transition to
		// summary so the prompt doesn't linger over a completed run;
		// drainCmd keeps listening in case late events race the close.
		m.progress.done = true
		m.summary.steps = m.progress.steps
		m.summary.endTime = time.Now()
		m.summary.warnings = m.config.Failures
		m.phase = PhaseSummary
		m.saveState()
		if m.summary.doctorMode && m.width > 0 && m.height > 0 {
			m.summary.initDoctorViewport(m.width, m.height)
		}
		return m, drainCmd(m.eventCh)

	case tea.KeyPressMsg:
		switch msg.String() {
		case "c", "s": // continue / skip
			// Return to installing phase without cancelling engine.
			// The engine already skipped dependents of the failed task.
			m.phase = PhaseInstalling
			// Check if all tasks are done (failure may have been the last).
			if m.progress.allFinished() {
				m.summary.steps = m.progress.steps
				m.summary.endTime = time.Now()
				m.phase = PhaseSummary
				m.saveState()
				if m.summary.doctorMode && m.width > 0 && m.height > 0 {
					m.summary.initDoctorViewport(m.width, m.height)
				}
				return m, drainCmd(m.eventCh)
			}
			return m, listenCmd(m.eventCh)
		case "a", "q": // abort
			if m.cancelEngine != nil {
				m.cancelEngine()
			}
			m.summary.steps = m.progress.steps
			m.summary.endTime = time.Now()
			m.summary.criticalFailure = true
			m.phase = PhaseSummary
			if m.summary.doctorMode && m.width > 0 && m.height > 0 {
				m.summary.initDoctorViewport(m.width, m.height)
			}
			return m, drainCmd(m.eventCh)
		}
	}
	return m, nil
}

func (m AppModel) failurePromptView(width int) string {
	w := contentWidth(width)
	var b strings.Builder

	b.WriteString(errorStyle.Render("  Critical Tool Failed"))
	b.WriteString(panelGap("\n\n"))

	b.WriteString(panelGap("  "))
	b.WriteString(selectedStyle.Render(m.failedTaskLabel))
	b.WriteString(panelGap(" failed:\n"))
	b.WriteString(panelGap("  "))
	b.WriteString(dimStyle.Render(m.failedTaskErr.Error()))
	b.WriteString(panelGap("\n\n"))

	b.WriteString(panelGap("  "))
	b.WriteString(dimStyle.Render("Dependents of this tool will be skipped."))
	b.WriteString(panelGap("\n\n"))

	b.WriteString(panelGap("  "))
	b.WriteString(selectedStyle.Render("[c]"))
	b.WriteString(panelGap(" Continue without it"))
	b.WriteString(panelGap("\n"))
	b.WriteString(panelGap("  "))
	b.WriteString(errorStyle.Render("[a]"))
	b.WriteString(panelGap(" Abort install"))
	b.WriteString(panelGap("\n"))

	panel := panelStyle.Width(w).Render(b.String())
	return panel
}

func (m AppModel) updateSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case engine.AllDoneMsg:
		// Engine finished draining — nothing to do.
		return m, nil
	case engine.TaskStartedMsg, engine.TaskDoneMsg, engine.TaskSkippedMsg:
		// Straggler events from engine drain — ignore.
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.summary.dryRun {
			m.summary.initViewport(msg.Width, msg.Height)
		} else if m.summary.doctorMode {
			m.summary.initDoctorViewport(msg.Width, msg.Height)
		}
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter", "esc", "backspace":
			m.returnToMainMenu()
			return m, nil
		case "q":
			m.quitting = true
			return m, tea.Quit
		}
	}

	// Forward to viewport for scroll handling.
	useViewport := (m.summary.dryRun || m.summary.doctorMode) &&
		m.summary.viewportReady
	if useViewport {
		var cmd tea.Cmd
		m.summary.viewport, cmd = m.summary.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

// returnToMainMenu resets transient state and returns to the main menu.
func (m *AppModel) returnToMainMenu() {
	m.phase = PhaseMainMenu
	m.config.DryRun = false
	m.config.Verbose = false
	if m.config.Runner != nil {
		m.config.Runner.Verbose = false
	}
	m.config.PlanRows = nil
	m.config.SelectedComponents = nil
	m.config.Failures = nil
	m.cancelEngine = nil
	m.eventCh = nil
	m.progress = newProgressModel()
	m.summary = newSummaryModel(false)
	if m.config.LogFile != nil {
		m.summary.logPath = m.config.LogFile.Path()
	}
	m.options = newOptionsMenu()
	m.picker = newComponentPicker()
}

// repoSyncedMsg is emitted when the background git-pull finishes.
// Outcome is already logged inside the Cmd; the message is just a
// trigger to move on to engine setup.
type repoSyncedMsg struct{}

// repoSyncBlockedMsg is emitted when `git pull --ff-only` fails in
// a way that means the running installer is stale: uncommitted
// local changes or a non-fast-forward divergence. The installer
// binary is whatever the user last built/downloaded; continuing
// silently (as the old code did) means running an installer that
// doesn't match the checked-out configs. Body holds the captured
// git output so the summary screen can show which files block the
// pull — the user needs to stash/commit those to proceed cleanly.
type repoSyncBlockedMsg struct {
	body string
}

// syncRepoCmd returns a tea.Cmd that runs `git pull --ff-only` off
// the Update loop so the TUI stays responsive. Soft failures
// (network down, not-a-repo) are logged as warnings and install
// continues. Hard failures (local changes block pull, non-
// fast-forward) abort so the user isn't running a stale binary
// against newer configs without realizing it.
func (m AppModel) syncRepoCmd() tea.Cmd {
	runner := m.config.Runner
	rootDir := m.config.RootDir
	failures := m.config.Failures
	return func() tea.Msg {
		if runner == nil || rootDir == "" {
			return repoSyncedMsg{}
		}
		ctx, cancel := context.WithTimeout(
			context.Background(), 15*time.Second,
		)
		defer cancel()
		cmd := exec.CommandContext(ctx, "git", "pull", "--ff-only")
		cmd.Dir = rootDir
		out, err := cmd.CombinedOutput()
		body := string(out)
		if err == nil {
			if body != "" {
				runner.Log.WriteRaw([]byte(body))
			}
			return repoSyncedMsg{}
		}
		runner.Log.Write(fmt.Sprintf(
			"git pull --ff-only failed: %v", err,
		))
		if body != "" {
			runner.Log.WriteRaw([]byte(body))
		}
		lowered := strings.ToLower(body)
		hardFail := strings.Contains(lowered, "local changes") ||
			strings.Contains(lowered, "would be overwritten") ||
			strings.Contains(lowered, "not possible to fast-forward") ||
			strings.Contains(lowered, "non-fast-forward")
		if hardFail {
			// Auto-recover when every dirty tracked file lives under
			// configs/. This is the dock case: upstream tool install
			// scripts appended to symlinked files and blocked the
			// pull. Any edit the user actually cared about would be
			// committed; anything else is cruft we can restore from
			// HEAD. If drift spans outside configs/ (e.g. the user
			// was editing installer source), bail to the block
			// screen instead of nuking real work.
			drifted, derr := config.DetectRepoDrift(rootDir)
			if derr == nil && len(drifted) > 0 && bodyDriftInScope(body, drifted) {
				// One-shot manager per auto-restore. The
				// orchestrator uses its own manager later for
				// install-time backups; they don't share state, and
				// keeping them separate avoids lifecycle coupling.
				bm := backup.NewManager(false)
				backupDir, rerr := config.BackupAndReset(
					rootDir, bm, drifted,
				)
				if rerr == nil {
					runner.Log.Write(fmt.Sprintf(
						"Auto-restored %d drifted config file(s); "+
							"originals saved to %s",
						len(drifted), backupDir,
					))
					if failures != nil {
						failures.Record(
							"Repo",
							fmt.Sprintf("auto-restored %d file(s)", len(drifted)),
							fmt.Errorf("originals saved to %s", backupDir),
						)
					}
					// Retry the pull now that the working tree is
					// clean. If it still fails, fall through to the
					// block screen with the new output.
					cmd := exec.CommandContext(
						ctx, "git", "pull", "--ff-only",
					)
					cmd.Dir = rootDir
					retryOut, retryErr := cmd.CombinedOutput()
					if retryErr == nil {
						runner.Log.WriteRaw(retryOut)
						return repoSyncedMsg{}
					}
					body = string(retryOut)
				}
			}
			return repoSyncBlockedMsg{body: body}
		}
		// Soft failure: record as a visible warning so the summary
		// banner surfaces it, but don't abort — the user may be
		// offline and that's legitimate.
		if failures != nil {
			failures.Record(
				"Repo", "git pull --ff-only",
				fmt.Errorf("continuing with local checkout: %v", err),
			)
		}
		return repoSyncedMsg{}
	}
}

// bodyDriftInScope defends the auto-restore path: every dirty path
// git named in the merge-blocking error must match a path returned
// by DetectRepoDrift (which is already scoped to configs/). If
// anything else shows up, we refuse to auto-restore and let the
// user handle it manually via the block screen.
func bodyDriftInScope(body string, drifted []string) bool {
	set := make(map[string]struct{}, len(drifted))
	for _, p := range drifted {
		set[p] = struct{}{}
	}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		// git's "would be overwritten" message indents each path
		// with a tab. Other lines can be ignored.
		if !strings.HasPrefix(line, "configs/") {
			continue
		}
		if _, ok := set[line]; !ok {
			return false
		}
	}
	return true
}

// saveState persists the install state to disk. Best-effort.
func (m *AppModel) saveState() {
	if m.config.State != nil {
		if err := m.config.State.Save(); err != nil && m.config.Runner != nil {
			m.config.Runner.Log.Write(
				fmt.Sprintf("WARNING: save state: %v", err),
			)
		}
	}
}

// --------------------------------------------------------------------------
// Install orchestration (parallel engine)
// --------------------------------------------------------------------------

const maxParallelWorkers = 5

func (m *AppModel) buildConfig() *orchestrator.BuildConfig {
	// Map nil-means-all for component selection.
	var comps []string
	if m.config.Mode == ModeCustomInstall {
		comps = m.config.SelectedComponents
	}
	if m.config.Failures == nil {
		m.config.Failures = config.NewTrackedFailures()
	}
	return &orchestrator.BuildConfig{
		Runner:         m.config.Runner,
		PkgMgr:         m.config.PkgMgr,
		Platform:       m.config.Platform,
		State:          m.config.State,
		RootDir:        m.config.RootDir,
		DryRun:         m.config.DryRun,
		ForceReinstall: m.config.ForceReinstall,
		SkipPackages:   m.config.SkipPackages,
		SkipUpdate:     m.config.SkipUpdate,
		SkipDevTools:   m.config.SkipDevTools,
		CleanBackup:    m.config.CleanBackup,
		SelectedBackup: m.config.SelectedBackup,
		SelectedComps:  comps,
		Version:        Version,
		Failures:       m.config.Failures,
	}
}

func (m *AppModel) applyResult(r orchestrator.BuildResult) []engine.Task {
	m.config.PlanRows = append(m.config.PlanRows, r.PlanRows...)
	m.summary.alreadyInstalled += r.AlreadyInstalled
	m.summary.alreadyConfigured += r.AlreadyConfigured
	m.summary.alreadyInstalledNames = append(
		m.summary.alreadyInstalledNames, r.AlreadyInstalledNames...,
	)
	m.summary.alreadyConfiguredNames = append(
		m.summary.alreadyConfiguredNames, r.AlreadyConfiguredNames...,
	)
	return r.Tasks
}

func (m *AppModel) startInstall() tea.Cmd {
	// Propagate DryRun to Runner so commands invoked during
	// planning are skipped correctly (Runner.Run checks r.DryRun).
	if m.config.Runner != nil {
		m.config.Runner.DryRun = m.config.DryRun
	}
	// Initialize Failures before syncRepoCmd so the auto-restore
	// path has somewhere to record the backup dir. buildConfig
	// checks for nil but that runs later, after the sync.
	if m.config.Failures == nil {
		m.config.Failures = config.NewTrackedFailures()
	}

	// For install/update, sync the repo off the Update loop. The
	// Cmd emits repoSyncedMsg; updateInstalling picks that up and
	// calls runInstallTasks. Doctor + Restore skip the sync.
	if m.config.Mode != ModeRestore && m.config.Mode != ModeDoctor {
		return m.syncRepoCmd()
	}
	return m.runInstallTasks()
}

// runInstallTasks builds the engine task graph and kicks off the
// worker pool. Split out from startInstall so we can run it after
// syncRepoCmd returns via repoSyncedMsg.
//
// Before the engine starts, it runs a read-only dpkg health probe
// on apt-based systems. If dpkg is inconsistent, the method
// transitions to PhaseDpkgRepair and returns nil — the engine
// will be kicked again by updateDpkgRepair once the user decides
// whether to authorize `sudo dpkg --configure -a`.
func (m *AppModel) runInstallTasks() tea.Cmd {
	// Pre-flight: only for fresh install / update (other modes don't
	// shell out to apt at session start).
	if m.config.Mode == ModeInstall || m.config.Mode == ModeCustomInstall || m.config.Mode == ModeUpdate {
		if cmd, blocked := m.preflightDpkgHealth(); blocked {
			return cmd
		}
	}

	bc := m.buildConfig()
	var tasks []engine.Task

	switch m.config.Mode {
	case ModeUpdate:
		tasks = m.applyResult(orchestrator.BuildUpdateTasks(bc))
	case ModeRestore:
		tasks = m.applyResult(orchestrator.BuildRestoreTasks(bc))
	case ModeDoctor:
		m.summary.doctorMode = true
		tasks = m.applyResult(orchestrator.BuildDoctorTasks(bc))
	case ModeUninstall:
		tasks = m.applyResult(orchestrator.BuildUninstallTasks(bc))
	default:
		tasks = m.applyResult(orchestrator.BuildInstallTasks(bc))
	}

	if m.config.DryRun {
		now := time.Now()
		m.summary.startTime = now
		m.summary.endTime = now
		m.summary.rows = m.config.PlanRows
		if m.width > 0 && m.height > 0 {
			m.summary.initViewport(m.width, m.height)
		}
		m.phase = PhaseSummary
		return nil
	}

	m.startTime = time.Now()
	m.summary.startTime = m.startTime
	m.progress.startedAt = m.startTime
	m.progress.verbose = m.config.Verbose

	if m.config.Verbose {
		m.config.Runner.EnableVerboseChannel(64)
	}

	if len(tasks) == 0 {
		m.summary.steps = nil
		m.summary.endTime = m.startTime
		m.phase = PhaseSummary
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelEngine = cancel
	m.eventCh = engine.Run(ctx, tasks, maxParallelWorkers)
	return tea.Batch(m.progress.Init(), listenCmd(m.eventCh))
}

// listenCmd returns a Bubble Tea command that blocks until the next
// engine event arrives, then delivers it as a tea.Msg.
func listenCmd(ch <-chan engine.Event) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return engine.AllDoneMsg{}
		}
		return msg
	}
}

// drainCmd consumes and discards remaining engine events after the
// TUI has transitioned away from the install phase (e.g., on critical
// failure). This prevents engine goroutines from blocking on sends.
func drainCmd(ch <-chan engine.Event) tea.Cmd {
	return func() tea.Msg {
		for range ch {
			// Discard until channel is closed.
		}
		return engine.AllDoneMsg{}
	}
}


