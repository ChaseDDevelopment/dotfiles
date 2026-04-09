package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

	footer := renderFooter("↑/↓ navigate", "enter select", "q quit")
	footerBlock := lipgloss.NewStyle().
		Width(panelOuterWidth(w)).
		AlignHorizontal(lipgloss.Center).
		Render(footer)

	return lipgloss.JoinVertical(lipgloss.Left, panel, footerBlock)
}

func (m mainMenuModel) selected() InstallMode {
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
			{key: "verbose", label: "Verbose output"},
			{key: "clean_backup", label: "Clean backup after"},
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
	footer := renderFooter("space toggle", "enter continue", "esc back")
	footerBlock := lipgloss.NewStyle().
		Width(panelOuterWidth(w)).
		AlignHorizontal(lipgloss.Center).
		Render(footer)

	return lipgloss.JoinVertical(lipgloss.Left, panel, footerBlock)
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
	return componentPickerModel{
		items: []componentItem{
			{icon: "✦", name: "All"},
			{icon: " ", name: "Zsh"},
			{icon: " ", name: "Tmux"},
			{icon: " ", name: "Neovim"},
			{icon: " ", name: "Starship"},
			{icon: " ", name: "Atuin"},
			{icon: "󰊠", name: "Ghostty"},
			{icon: " ", name: "Yazi"},
			{icon: " ", name: "Git"},
		},
	}
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
	footer := renderFooter("space toggle", "enter continue", "esc back")
	footerBlock := lipgloss.NewStyle().
		Width(panelOuterWidth(w)).
		AlignHorizontal(lipgloss.Center).
		Render(footer)

	return lipgloss.JoinVertical(lipgloss.Left, panel, footerBlock)
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

