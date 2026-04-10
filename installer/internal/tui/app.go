package tui

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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
	PhaseInstalling
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
	eventCh <-chan any

	// cancelEngine cancels the engine context, stopping all running
	// tasks and preventing goroutine leaks on Ctrl+C or critical failure.
	cancelEngine context.CancelFunc

	startTime time.Time
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

	// Handle window size.
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = msg.Width
		m.height = msg.Height
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
	case PhaseInstalling:
		return m.updateInstalling(msg)
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
	case PhaseInstalling:
		content = m.progress.View(w)
	case PhaseSummary:
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
		return m, listenCmd(m.eventCh)

	case engine.TaskDoneMsg:
		m.progress.markDone(msg.ID, msg.Err)
		// Save state incrementally so progress survives crashes.
		if msg.Err == nil {
			m.saveState()
		}
		// Abort if a critical tool failed.
		if msg.Critical && msg.Err != nil {
			if m.cancelEngine != nil {
				m.cancelEngine()
			}
			m.summary.steps = m.progress.steps
			m.summary.endTime = time.Now()
			m.summary.criticalFailure = true
			m.phase = PhaseSummary
			return m, drainCmd(m.eventCh)
		}
		// Transition to summary if all tasks are finished —
		// don't wait solely for AllDoneMsg which can be missed.
		if m.progress.allFinished() {
			m.summary.steps = m.progress.steps
			m.summary.endTime = time.Now()
			m.phase = PhaseSummary
			m.saveState()
			return m, drainCmd(m.eventCh)
		}
		return m, listenCmd(m.eventCh)

	case engine.TaskSkippedMsg:
		m.progress.markSkipped(msg.ID, msg.Label, msg.Reason)
		return m, listenCmd(m.eventCh)

	case engine.AllDoneMsg:
		m.summary.steps = m.progress.steps
		m.summary.endTime = time.Now()
		m.phase = PhaseSummary
		m.saveState()
		return m, nil

	default:
		// Forward spinner ticks, progress frames, etc.
		if m.config.Verbose && m.config.Runner != nil {
			m.progress.recentLines = m.config.Runner.RecentLinesSnapshot()
		}
		var cmd tea.Cmd
		m.progress, cmd = m.progress.Update(msg)
		return m, cmd
	}
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
	if m.summary.dryRun && m.summary.viewportReady {
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
	m.config.PlanRows = nil
	m.config.SelectedComponents = nil
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

// syncRepo does a fast-forward git pull to ensure configs are
// up-to-date before applying. Failures are logged but do not block
// the install — the user may be offline or have local changes.
func (m *AppModel) syncRepo() {
	if m.config.Runner == nil || m.config.RootDir == "" {
		return
	}
	ctx, cancel := context.WithTimeout(
		context.Background(), 15*time.Second,
	)
	defer cancel()
	if err := m.config.Runner.RunInDir(
		ctx, m.config.RootDir, "git", "pull", "--ff-only",
	); err != nil {
		m.config.Runner.Log.Write(fmt.Sprintf(
			"NOTE: git pull --ff-only skipped: %v", err,
		))
	}
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
	return &orchestrator.BuildConfig{
		Runner:         m.config.Runner,
		PkgMgr:         m.config.PkgMgr,
		Platform:       m.config.Platform,
		State:          m.config.State,
		RootDir:        m.config.RootDir,
		DryRun:         m.config.DryRun,
		ForceReinstall: m.config.ForceReinstall,
		SkipPackages:   m.config.SkipPackages,
		CleanBackup:    m.config.CleanBackup,
		SelectedBackup: m.config.SelectedBackup,
		SelectedComps:  comps,
		Version:        Version,
	}
}

func (m *AppModel) applyResult(r orchestrator.BuildResult) []engine.Task {
	m.config.PlanRows = append(m.config.PlanRows, r.PlanRows...)
	m.summary.alreadyInstalled += r.AlreadyInstalled
	m.summary.alreadyConfigured += r.AlreadyConfigured
	return r.Tasks
}

func (m *AppModel) startInstall() tea.Cmd {
	// Sync dotfiles repo before install/update (best-effort).
	if m.config.Mode != ModeRestore && m.config.Mode != ModeDoctor {
		m.syncRepo()
	}

	bc := m.buildConfig()
	var tasks []engine.Task

	switch m.config.Mode {
	case ModeUpdate:
		tasks = m.applyResult(orchestrator.BuildUpdateTasks(bc))
	case ModeRestore:
		tasks = m.applyResult(orchestrator.BuildRestoreTasks(bc))
	case ModeDoctor:
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
func listenCmd(ch <-chan any) tea.Cmd {
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
func drainCmd(ch <-chan any) tea.Cmd {
	return func() tea.Msg {
		for range ch {
			// Discard until channel is closed.
		}
		return engine.AllDoneMsg{}
	}
}


