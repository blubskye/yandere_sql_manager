// YSM - Yandere SQL Manager
// Copyright (C) 2025 blubskye
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
//
// Source code: https://github.com/blubskye/yandere_sql_manager

package views

import (
	"fmt"
	"strings"

	"github.com/blubskye/yandere_sql_manager/internal/config"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// KeybindingsView allows customizing keybindings
type KeybindingsView struct {
	width  int
	height int

	keybindings *config.KeyBindings
	views       []string
	actions     []config.KeyAction
	currentView int
	cursor      int

	editing       bool
	waitingForKey bool
	editInput     textinput.Model
	newKey        string

	err       error
	statusMsg string
}

// NewKeybindingsView creates a new keybindings settings view
func NewKeybindingsView(width, height int) *KeybindingsView {
	kb, _ := config.LoadKeyBindings()
	if kb == nil {
		kb = config.DefaultKeyBindings()
	}

	editInput := textinput.New()
	editInput.Placeholder = "Press any key..."
	editInput.CharLimit = 32

	return &KeybindingsView{
		width:       width,
		height:      height,
		keybindings: kb,
		views:       config.ViewNames(),
		actions:     getActionsForView("global"),
		currentView: 0,
		editInput:   editInput,
	}
}

func getActionsForView(viewName string) []config.KeyAction {
	allActions := config.AllActions()

	switch viewName {
	case "global":
		return allActions["Navigation"]
	case "databases":
		return append(allActions["Navigation"], allActions["Views"]...)
	case "tables":
		actions := allActions["Navigation"]
		actions = append(actions, config.ActionQuery, config.ActionImport, config.ActionExport)
		return actions
	case "browser":
		actions := allActions["Navigation"]
		actions = append(actions, config.ActionEdit, config.ActionDelete)
		return actions
	case "query":
		return []config.KeyAction{
			config.ActionSave,
			config.ActionCancel,
		}
	case "settings":
		return []config.KeyAction{
			config.ActionToggleGlobal,
			config.ActionClearFilter,
			config.ActionEdit,
		}
	case "users":
		actions := allActions["Navigation"]
		actions = append(actions, config.ActionCreate, config.ActionDelete, config.ActionEdit)
		return actions
	case "backup":
		actions := allActions["Navigation"]
		actions = append(actions, config.ActionCreate, config.ActionDelete)
		return actions
	case "dashboard":
		return []config.KeyAction{
			config.ActionToggleAutoRefresh,
			config.ActionNextTab,
			config.ActionPrevTab,
			config.ActionRefresh,
		}
	case "cluster":
		return []config.KeyAction{
			config.ActionToggleAutoRefresh,
			config.ActionTab1,
			config.ActionTab2,
			config.ActionTab3,
			config.ActionTab4,
			config.ActionRefresh,
		}
	default:
		return allActions["Navigation"]
	}
}

// Init initializes the view
func (v *KeybindingsView) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (v *KeybindingsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		keyStr := msg.String()

		// If waiting for a key press to set binding
		if v.waitingForKey {
			// Capture any key except escape (which cancels)
			if keyStr == "esc" {
				v.waitingForKey = false
				v.editing = false
				v.statusMsg = ""
				return v, nil
			}

			// Set the new keybinding
			viewName := v.views[v.currentView]
			action := v.actions[v.cursor]
			if err := v.keybindings.SetKey(viewName, action, keyStr); err != nil {
				v.err = err
			} else {
				v.statusMsg = fmt.Sprintf("Set %s to '%s'~ <3", action, keyStr)
				// Save immediately
				if err := v.keybindings.Save(); err != nil {
					v.err = fmt.Errorf("saved but couldn't persist: %v", err)
				}
			}
			v.waitingForKey = false
			v.editing = false
			return v, nil
		}

		// Normal mode
		switch keyStr {
		case "esc":
			return v, func() tea.Msg {
				return SwitchViewMsg{View: "databases"}
			}
		case "q", "ctrl+c":
			return v, tea.Quit
		case "up", "k":
			if v.cursor > 0 {
				v.cursor--
			}
		case "down", "j":
			if v.cursor < len(v.actions)-1 {
				v.cursor++
			}
		case "left", "h":
			if v.currentView > 0 {
				v.currentView--
				v.actions = getActionsForView(v.views[v.currentView])
				v.cursor = 0
			}
		case "right", "l":
			if v.currentView < len(v.views)-1 {
				v.currentView++
				v.actions = getActionsForView(v.views[v.currentView])
				v.cursor = 0
			}
		case "enter":
			if len(v.actions) > 0 {
				v.editing = true
				v.waitingForKey = true
				v.statusMsg = "Press a key to bind~ (Esc to cancel)"
				return v, nil
			}
		case "d":
			// Reset current binding to default
			if len(v.actions) > 0 {
				viewName := v.views[v.currentView]
				action := v.actions[v.cursor]
				defaults := config.DefaultKeyBindings()
				defaultKey := defaults.GetKey(viewName, action)
				if defaultKey != "" {
					v.keybindings.SetKey(viewName, action, defaultKey)
					v.keybindings.Save()
					v.statusMsg = fmt.Sprintf("Reset %s to default '%s'~", action, defaultKey)
				}
			}
		case "r":
			// Reset all to defaults
			v.keybindings = config.DefaultKeyBindings()
			if err := v.keybindings.Save(); err != nil {
				v.err = err
			} else {
				v.statusMsg = "Reset all keybindings to defaults~ <3"
			}
		}

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
	}

	return v, nil
}

// View renders the view
func (v *KeybindingsView) View() string {
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("Keybindings Settings~ <3"))
	b.WriteString("\n\n")

	// View tabs
	var tabs []string
	for i, view := range v.views {
		name := strings.Title(view)
		if i == v.currentView {
			tabs = append(tabs, lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#FF69B4")).
				Padding(0, 1).
				Bold(true).
				Render(name))
		} else {
			tabs = append(tabs, lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				Padding(0, 1).
				Render(name))
		}
	}
	b.WriteString(strings.Join(tabs, " "))
	b.WriteString("\n\n")

	// Error message
	if v.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", v.err)))
		b.WriteString("\n\n")
		v.err = nil
	}

	// Status message
	if v.statusMsg != "" {
		if v.waitingForKey {
			b.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFD700")).
				Bold(true).
				Render(v.statusMsg))
		} else {
			b.WriteString(successStyle.Render(v.statusMsg))
		}
		b.WriteString("\n\n")
	}

	// Keybindings list
	viewName := v.views[v.currentView]
	if len(v.actions) == 0 {
		b.WriteString(mutedStyle.Render("No configurable keybindings for this view."))
		b.WriteString("\n")
	} else {
		// Calculate max widths for alignment
		maxActionWidth := 0
		maxDescWidth := 0
		for _, action := range v.actions {
			if len(string(action)) > maxActionWidth {
				maxActionWidth = len(string(action))
			}
			desc := config.GetActionDescription(action)
			if len(desc) > maxDescWidth {
				maxDescWidth = len(desc)
			}
		}

		// Determine visible range
		visibleHeight := v.height - 14
		if visibleHeight < 5 {
			visibleHeight = 5
		}

		startIdx := 0
		if v.cursor >= visibleHeight {
			startIdx = v.cursor - visibleHeight + 1
		}

		endIdx := startIdx + visibleHeight
		if endIdx > len(v.actions) {
			endIdx = len(v.actions)
		}

		for i := startIdx; i < endIdx; i++ {
			action := v.actions[i]
			key := v.keybindings.GetKey(viewName, action)
			desc := config.GetActionDescription(action)

			// Format: [key] Description (action)
			keyDisplay := fmt.Sprintf("[%s]", key)
			if key == "" {
				keyDisplay = "[none]"
			}

			if i == v.cursor {
				if v.waitingForKey {
					// Waiting for key input
					b.WriteString(lipgloss.NewStyle().
						Foreground(lipgloss.Color("#FFFFFF")).
						Background(lipgloss.Color("#FFD700")).
						Bold(true).
						Render(fmt.Sprintf(" %-10s %-30s (press any key...)", keyDisplay, desc)))
				} else {
					// Selected row
					rowStyle := lipgloss.NewStyle().
						Foreground(lipgloss.Color("#FFFFFF")).
						Background(lipgloss.Color("#FF69B4")).
						Bold(true)
					b.WriteString(rowStyle.Render(fmt.Sprintf(" %-10s %-30s %s ", keyDisplay, desc, action)))
				}
			} else {
				b.WriteString(fmt.Sprintf(" %-10s %-30s %s", keyDisplay, desc, mutedStyle.Render(string(action))))
			}
			b.WriteString("\n")
		}

		// Scroll indicator
		if len(v.actions) > visibleHeight {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("\n[%d/%d]", v.cursor+1, len(v.actions))))
		}
	}

	b.WriteString("\n\n")

	// Help
	var help string
	if v.waitingForKey {
		help = "Press any key to bind | Esc: Cancel"
	} else {
		help = "←→: Switch view | ↑↓: Navigate | Enter: Change binding | d: Reset to default | r: Reset all | Esc: Back"
	}
	b.WriteString(helpStyle.Render(help))

	return b.String()
}

// keybindingsSavedMsg is sent when keybindings are saved
type keybindingsSavedMsg struct{}

// GetKeybindings returns the current keybindings
func (v *KeybindingsView) GetKeybindings() *config.KeyBindings {
	return v.keybindings
}
