package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/chaseddevelopment/dotfiles/installer/internal/backup"
	"github.com/chaseddevelopment/dotfiles/installer/internal/config"
)

// ---------------------------------------------------------------------------
// Main Menu
// ---------------------------------------------------------------------------

type mainMenuModel struct {
	items  []menuItem
	cursor int
}

type menuItem struct {
	icon  string
	label string
	desc  string
	mode  InstallMode
}

func newMainMenu() mainMenuModel {
	return mainMenuModel{
		items: []menuItem{
			{icon: " ", label: "Install", desc: "Full installation (recommended)", mode: ModeInstall},
			{icon: " ", label: "Custom Install", desc: "Pick individual components", mode: ModeCustomInstall},
			{icon: " ", label: "Dry Run", desc: "Preview changes without modifying", mode: ModeDryRun},
			{icon: "󰚰 ", label: "Update", desc: "Update all installed tools", mode: ModeUpdate},
			{icon: " ", label: "Restore", desc: "Restore from a backup", mode: ModeRestore},
			{icon: " ", label: "Doctor", desc: "Verify installation health", mode: ModeDoctor},
			{icon: " ", label: "Uninstall", desc: "Remove component symlinks", mode: ModeUninstall},
			{icon: " ", label: "Exit", desc: "", mode: ModeExit},
		},
	}
}

func (m mainMenuModel) Update(msg tea.Msg) (mainMenuModel, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m mainMenuModel) View(width int) string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("  Choose an action"))
	b.WriteString(panelGap("\n\n"))

	for i, item := range m.items {
		isSelected := i == m.cursor

		// Cursor
		cursor := panelGap("  ")
		if isSelected {
			cursor = cursorStyle.Render("▸ ")
		}

		// Icon
		icon := iconDimStyle.Render(item.icon)
		if isSelected {
			icon = iconStyle.Render(item.icon)
		}

		// Label
		label := menuDimStyle.Render(item.label)
		if isSelected {
			label = selectedStyle.Render(item.label)
		}

		// Description
		desc := ""
		if item.desc != "" && isSelected {
			desc = panelGap("  ") + descStyle.Render(item.desc)
		}

		b.WriteString(fmt.Sprintf("%s%s%s%s%s", cursor, icon, label, desc, panelGap("\n")))
	}

	content := b.String()

	// Wrap in panel
	w := contentWidth(width)
	panel := panelStyle.Width(w).Render(content)

	return lipgloss.JoinVertical(lipgloss.Left, panel, footerBlock(w, "↑/↓ navigate", "enter select", "q quit"))
}

func (m mainMenuModel) selected() InstallMode {
	if m.cursor >= len(m.items) {
		return 0
	}
	return m.items[m.cursor].mode
}

// ---------------------------------------------------------------------------
// Options Menu (toggles)
// ---------------------------------------------------------------------------

type optionsMenuModel struct {
	options []toggleOption
	cursor  int
}

type toggleOption struct {
	key     string // stable identifier for config mapping
	label   string
	enabled bool
}

func newOptionsMenu() optionsMenuModel {
	return optionsMenuModel{
		options: []toggleOption{
			{key: "skip_update", label: "Skip system update"},
			{key: "skip_packages", label: "Skip packages"},
			{key: "force_reinstall", label: "Force reinstall"},
			{key: "verbose", label: "Verbose output"},
			{key: "clean_backup", label: "Clean backup after"},
			// Defaults on: server operators explicitly flip this off
			// to skip Go/.NET/uv/Bun on machines that only need the
			// terminal + nvim for logs/yaml/docker.
			{key: "install_dev_tools", label: "Install dev tools (Go, .NET, uv, Bun)", enabled: true},
		},
	}
}

// optionEnabled returns the enabled state for the given key.
func (m optionsMenuModel) optionEnabled(key string) bool {
	for _, opt := range m.options {
		if opt.key == key {
			return opt.enabled
		}
	}
	return false
}

func (m optionsMenuModel) Update(msg tea.Msg) (optionsMenuModel, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "space", "x":
			m.options[m.cursor].enabled = !m.options[m.cursor].enabled
		}
	}
	return m, nil
}

func (m optionsMenuModel) View(width int) string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("  Options"))
	b.WriteString(panelGap("\n\n"))

	for i, opt := range m.options {
		isSelected := i == m.cursor

		cursor := panelGap("  ")
		if isSelected {
			cursor = cursorStyle.Render("▸ ")
		}

		check := uncheckStyle.Render("[○]")
		if opt.enabled {
			check = checkStyle.Render("[●]")
		}

		label := menuDimStyle.Render(opt.label)
		if isSelected {
			label = selectedStyle.Render(opt.label)
		}

		b.WriteString(fmt.Sprintf("%s%s %s%s", cursor, check, label, panelGap("\n")))
	}

	content := b.String()
	w := contentWidth(width)
	panel := panelStyle.Width(w).Render(content)
	return lipgloss.JoinVertical(lipgloss.Left, panel, footerBlock(w, "space toggle", "enter continue", "esc back"))
}

// ---------------------------------------------------------------------------
// Component Picker (multi-select)
// ---------------------------------------------------------------------------

type componentPickerModel struct {
	items  []componentItem
	cursor int
}

type componentItem struct {
	icon     string
	name     string
	selected bool
}

func newComponentPicker() componentPickerModel {
	items := []componentItem{{icon: "✦", name: "All"}}
	for _, c := range config.AllComponents() {
		items = append(items, componentItem{
			icon: c.Icon, name: c.Name,
		})
	}
	return componentPickerModel{items: items}
}

func (m componentPickerModel) Update(msg tea.Msg) (componentPickerModel, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "space", "x":
			m.items[m.cursor].selected = !m.items[m.cursor].selected
			if m.cursor == 0 {
				for i := 1; i < len(m.items); i++ {
					m.items[i].selected = m.items[0].selected
				}
			}
		}
	}
	return m, nil
}

func (m componentPickerModel) View(width int) string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("  Select Components"))
	b.WriteString(panelGap("\n\n"))

	for i, item := range m.items {
		isSelected := i == m.cursor

		cursor := panelGap("  ")
		if isSelected {
			cursor = cursorStyle.Render("▸ ")
		}

		check := uncheckStyle.Render("[○]")
		if item.selected {
			check = checkStyle.Render("[●]")
		}

		icon := iconDimStyle.Render(item.icon)
		if isSelected {
			icon = iconStyle.Render(item.icon)
		}

		label := menuDimStyle.Render(item.name)
		if isSelected {
			label = selectedStyle.Render(item.name)
		}

		b.WriteString(fmt.Sprintf("%s%s %s %s%s", cursor, check, icon, label, panelGap("\n")))
	}

	content := b.String()
	w := contentWidth(width)
	panel := panelStyle.Width(w).Render(content)
	return lipgloss.JoinVertical(lipgloss.Left, panel, footerBlock(w, "space toggle", "enter continue", "esc back"))
}

func (m componentPickerModel) selectedComponents() []string {
	var sel []string
	for _, item := range m.items {
		if item.selected {
			sel = append(sel, item.name)
		}
	}
	return sel
}

// ---------------------------------------------------------------------------
// Backup Picker (single-select)
// ---------------------------------------------------------------------------

type backupPickerModel struct {
	items  []backup.BackupInfo
	cursor int
	err    error
}

func newBackupPicker() backupPickerModel {
	backups, err := backup.ListBackups()
	return backupPickerModel{items: backups, err: err}
}

func (m backupPickerModel) Update(msg tea.Msg) (backupPickerModel, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m backupPickerModel) View(width int) string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("  Select Backup"))
	b.WriteString(panelGap("\n\n"))

	if m.err != nil || len(m.items) == 0 {
		b.WriteString(dimStyle.Render("  No backups found."))
		b.WriteString(panelGap("\n"))
	} else {
		for i, item := range m.items {
			isSelected := i == m.cursor

			cursor := panelGap("  ")
			if isSelected {
				cursor = cursorStyle.Render("▸ ")
			}

			label := menuDimStyle.Render(item.Date)
			if isSelected {
				label = selectedStyle.Render(item.Date)
			}

			path := ""
			if isSelected {
				path = panelGap("  ") + dimStyle.Render(item.Path)
			}

			b.WriteString(fmt.Sprintf(
				"%s%s%s%s", cursor, label, path, panelGap("\n"),
			))
		}
	}

	content := b.String()
	w := contentWidth(width)
	panel := panelStyle.Width(w).Render(content)
	return lipgloss.JoinVertical(lipgloss.Left, panel, footerBlock(w, "↑/↓ navigate", "enter select", "esc back"))
}

func (m backupPickerModel) selected() string {
	if len(m.items) == 0 {
		return ""
	}
	return m.items[m.cursor].Path
}

