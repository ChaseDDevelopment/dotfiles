package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/chaseddevelopment/dotfiles/installer/internal/backup"
	"github.com/chaseddevelopment/dotfiles/installer/internal/config"
	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
	"github.com/chaseddevelopment/dotfiles/installer/internal/pkgmgr"
	"github.com/chaseddevelopment/dotfiles/installer/internal/registry"
	"github.com/chaseddevelopment/dotfiles/installer/internal/update"
)

// Phase represents the current UI phase.
type Phase int

const (
	PhaseMainMenu Phase = iota
	PhaseOptionsMenu
	PhaseComponentPicker
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
	SelectedComponents []string
	Platform           *platform.Platform
	PkgMgr             pkgmgr.PackageManager
	RootDir            string
	LogFile            *executor.LogFile
	Runner             *executor.Runner
	PlanRows           []PlanRow
}

// AppModel is the top-level Bubble Tea model.
type AppModel struct {
	phase     Phase
	config    *AppConfig
	mainMenu  mainMenuModel
	options   optionsMenuModel
	picker    componentPickerModel
	progress  progressModel
	summary   summaryModel
	width        int
	height       int
	quitting     bool
	stepIdx      int
	installSteps []installStep
	startTime    time.Time
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
	return m.progress.Init()
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle global keys.
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "ctrl+c":
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
	case PhaseInstalling:
		return m.updateInstalling(msg)
	case PhaseSummary:
		return m.updateSummary(msg)
	}

	return m, nil
}

func (m AppModel) View() string {
	if m.quitting {
		return ""
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
			Background(catBase).
			Render(banner)
		menu := m.mainMenu.View(w)
		content = lipgloss.JoinVertical(lipgloss.Center, bannerBlock, menu)
	case PhaseOptionsMenu:
		content = m.options.View(w)
	case PhaseComponentPicker:
		content = m.picker.View(w)
	case PhaseInstalling:
		content = m.progress.View(w)
	case PhaseSummary:
		content = m.summary.View(w)
	}

	// Wrap the content in a full-screen container with catBase background.
	// Using Style.Render instead of lipgloss.Place ensures that ALL
	// whitespace — including JoinVertical centering padding between the
	// banner, panel, and footer — gets the catBase background.
	if m.width > 0 && m.height > 0 {
		content = lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			AlignHorizontal(lipgloss.Center).
			AlignVertical(lipgloss.Center).
			Background(catBase).
			Render(content)
	}

	return content
}

// Version is injected from main.
var Version = "dev"

// --------------------------------------------------------------------------
// Phase update handlers
// --------------------------------------------------------------------------

func (m AppModel) updateMainMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "enter" {
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
		case ModeUpdate, ModeRestore:
			m.phase = PhaseInstalling
			return m, m.startInstall()
		case ModeCustomInstall:
			m.phase = PhaseOptionsMenu
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
	if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "enter" {
		m.config.SkipUpdate = m.options.optionEnabled("skip_update")
		m.config.SkipPackages = m.options.optionEnabled("skip_packages")
		m.config.Verbose = m.options.optionEnabled("verbose")
		m.config.Runner.Verbose = m.config.Verbose
		m.config.CleanBackup = m.options.optionEnabled("clean_backup")

		if m.config.Mode == ModeCustomInstall {
			m.phase = PhaseComponentPicker
		} else {
			m.phase = PhaseInstalling
			return m, m.startInstall()
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.options, cmd = m.options.Update(msg)
	return m, cmd
}

func (m AppModel) updateComponentPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "enter" {
		m.config.SelectedComponents = m.picker.selectedComponents()
		m.phase = PhaseInstalling
		return m, m.startInstall()
	}
	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)
	return m, cmd
}

func (m AppModel) updateInstalling(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case stepDoneMsg:
		var cmd tea.Cmd
		m.progress, cmd = m.progress.Update(msg)

		// Abort if a critical tool failed.
		if msg.critical && !msg.success {
			m.summary.steps = m.progress.steps
			m.summary.endTime = time.Now()
			m.summary.criticalFailure = true
			m.phase = PhaseSummary
			return m, cmd
		}

		// Chain the next step.
		if m.stepIdx < len(m.installSteps) {
			next := m.installSteps[m.stepIdx]
			m.stepIdx++
			return m, tea.Batch(cmd, m.runStepCmd(next))
		}

		// All done.
		m.summary.steps = m.progress.steps
		m.summary.endTime = time.Now()
		m.phase = PhaseSummary
		return m, cmd

	default:
		// Copy verbose output lines from Runner on each tick.
		if m.config.Verbose && m.config.Runner != nil {
			m.progress.recentLines = m.config.Runner.RecentLinesSnapshot()
		}
		var cmd tea.Cmd
		m.progress, cmd = m.progress.Update(msg)
		return m, cmd
	}
}

func (m AppModel) updateSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "enter", "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

// --------------------------------------------------------------------------
// Install orchestration
// --------------------------------------------------------------------------

// installStep is a generic step that can be a tool install, update, or component setup.
type installStep struct {
	label    string
	critical bool // if true, failure aborts the entire install
	fn       func() error
}

func (m *AppModel) startInstall() tea.Cmd {
	var steps []installStep

	switch m.config.Mode {
	case ModeUpdate:
		steps = m.buildUpdateSteps()
	case ModeRestore:
		steps = m.buildRestoreSteps()
	default:
		steps = m.buildInstallSteps()
	}

	if m.config.DryRun {
		m.summary.rows = m.config.PlanRows
		m.phase = PhaseSummary
		return nil
	}

	m.installSteps = steps
	m.stepIdx = 0
	m.startTime = time.Now()
	m.summary.startTime = m.startTime
	m.progress.verbose = m.config.Verbose

	if len(steps) == 0 {
		m.summary.steps = nil
		m.phase = PhaseSummary
		return nil
	}

	first := m.installSteps[0]
	m.stepIdx = 1
	return m.runStepCmd(first)
}

func (m *AppModel) buildInstallSteps() []installStep {
	var steps []installStep
	runner := m.config.Runner
	mgr := m.config.PkgMgr
	plat := m.config.Platform

	// Package installation.
	if !m.config.SkipPackages {
		tools := registry.AllTools()
		for _, t := range tools {
			t := t // capture
			if !registry.ShouldInstall(&t, plat) {
				continue
			}
			if registry.IsInstalled(&t) {
				m.config.PlanRows = append(m.config.PlanRows, PlanRow{
					Component: t.Name, Action: "Package", Status: "already installed",
				})
				continue
			}
			m.config.PlanRows = append(m.config.PlanRows, PlanRow{
				Component: t.Name, Action: "Package", Status: "would install",
			})
			steps = append(steps, installStep{
				label:    fmt.Sprintf("Installing %s", t.Name),
				critical: t.Critical,
				fn: func() error {
					ic := &registry.InstallContext{Runner: runner, PkgMgr: mgr}
					return registry.ExecuteInstall(context.Background(), &t, ic, plat)
				},
			})
		}
	}

	// Component setup (symlinks + hooks).
	bm := backup.NewManager(m.config.DryRun)
	for _, comp := range config.AllComponents() {
		comp := comp // capture
		if !m.config.IsComponentSelected(comp.Name) {
			continue
		}
		m.config.PlanRows = append(m.config.PlanRows, PlanRow{
			Component: comp.Name, Action: "Setup", Status: "would configure",
		})
		steps = append(steps, installStep{
			label: fmt.Sprintf("Setting up %s", comp.Name),
			fn: func() error {
				sc := &config.SetupContext{
					Runner:   runner,
					RootDir:  m.config.RootDir,
					Backup:   bm,
					DryRun:   m.config.DryRun,
					Platform: plat,
				}
				return config.SetupComponent(context.Background(), comp, sc)
			},
		})
	}

	return steps
}

func (m *AppModel) buildUpdateSteps() []installStep {
	var steps []installStep
	updateSteps := update.AllSteps(m.config.Runner, m.config.PkgMgr, m.config.Platform)
	for _, s := range updateSteps {
		s := s
		steps = append(steps, installStep{
			label: fmt.Sprintf("Updating %s", s.Name),
			fn: func() error {
				return s.Fn(context.Background())
			},
		})
	}
	return steps
}

func (m *AppModel) buildRestoreSteps() []installStep {
	return []installStep{
		{
			label: "Restoring from backup",
			fn: func() error {
				backups, err := backup.ListBackups()
				if err != nil || len(backups) == 0 {
					return fmt.Errorf("no backups found")
				}
				// Use the most recent backup.
				return backup.Restore(backups[0].Path, m.config.DryRun)
			},
		},
	}
}

func (m *AppModel) runStepCmd(step installStep) tea.Cmd {
	return tea.Sequence(
		func() tea.Msg { return stepStartMsg{label: step.label} },
		func() tea.Msg {
			err := step.fn()
			return stepDoneMsg{
				label:    step.label,
				success:  err == nil,
				critical: step.critical,
				err:      err,
			}
		},
	)
}

// IsComponentSelected checks if a component should be set up.
func (cfg *AppConfig) IsComponentSelected(name string) bool {
	if cfg.Mode != ModeCustomInstall {
		return true
	}
	for _, c := range cfg.SelectedComponents {
		if c == "All" || c == name {
			return true
		}
	}
	return false
}

