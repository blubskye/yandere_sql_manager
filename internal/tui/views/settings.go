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

	"github.com/blubskye/yandere_sql_manager/internal/db"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SettingsView shows and allows editing of MariaDB system variables
type SettingsView struct {
	conn       *db.Connection
	width      int
	height     int

	variables  []db.Variable
	cursor     int
	editing    bool
	editInput  textinput.Model
	showGlobal bool
	filter     string
	filtering  bool
	filterInput textinput.Model

	err        error
	statusMsg  string
}

// NewSettingsView creates a new settings view
func NewSettingsView(conn *db.Connection, width, height int) *SettingsView {
	editInput := textinput.New()
	editInput.Placeholder = "Enter new value"
	editInput.CharLimit = 256

	filterInput := textinput.New()
	filterInput.Placeholder = "Filter variables..."
	filterInput.CharLimit = 64

	return &SettingsView{
		conn:        conn,
		width:       width,
		height:      height,
		editInput:   editInput,
		filterInput: filterInput,
	}
}

// Init initializes the view
func (v *SettingsView) Init() tea.Cmd {
	return v.loadVariables
}

func (v *SettingsView) loadVariables() tea.Msg {
	var variables []db.Variable
	var err error

	if v.filter != "" {
		if v.showGlobal {
			variables, err = v.conn.GetGlobalVariables(v.filter)
		} else {
			variables, err = v.conn.GetVariables(v.filter)
		}
	} else {
		// Load common variables by default
		variables, err = v.conn.GetCommonVariables()
	}

	if err != nil {
		return err
	}
	return variablesLoadedMsg{variables: variables}
}

type variablesLoadedMsg struct {
	variables []db.Variable
}

type variableSetMsg struct {
	name  string
	value string
}

// Update handles messages
func (v *SettingsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle filtering mode
		if v.filtering {
			switch msg.String() {
			case "enter":
				v.filter = v.filterInput.Value()
				v.filtering = false
				v.filterInput.Blur()
				return v, v.loadVariables
			case "esc":
				v.filtering = false
				v.filterInput.Blur()
				v.filterInput.SetValue(v.filter)
				return v, nil
			default:
				var cmd tea.Cmd
				v.filterInput, cmd = v.filterInput.Update(msg)
				return v, cmd
			}
		}

		// Handle editing mode
		if v.editing {
			switch msg.String() {
			case "enter":
				return v, v.setVariable()
			case "esc":
				v.editing = false
				v.editInput.Blur()
				return v, nil
			default:
				var cmd tea.Cmd
				v.editInput, cmd = v.editInput.Update(msg)
				return v, cmd
			}
		}

		// Normal mode
		switch msg.String() {
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
			if v.cursor < len(v.variables)-1 {
				v.cursor++
			}
		case "enter":
			if len(v.variables) > 0 {
				v.editing = true
				v.editInput.SetValue(v.variables[v.cursor].Value)
				v.editInput.Focus()
				return v, textinput.Blink
			}
		case "g":
			v.showGlobal = !v.showGlobal
			v.cursor = 0
			return v, v.loadVariables
		case "r":
			return v, v.loadVariables
		case "/":
			v.filtering = true
			v.filterInput.Focus()
			return v, textinput.Blink
		case "c":
			// Clear filter
			v.filter = ""
			v.filterInput.SetValue("")
			v.cursor = 0
			return v, v.loadVariables
		}

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height

	case variablesLoadedMsg:
		v.variables = msg.variables
		if v.cursor >= len(v.variables) {
			v.cursor = 0
		}
		v.err = nil
		return v, nil

	case variableSetMsg:
		v.editing = false
		v.editInput.Blur()
		v.statusMsg = fmt.Sprintf("Set %s = %s", msg.name, msg.value)
		return v, v.loadVariables

	case error:
		v.err = msg
		v.editing = false
		v.editInput.Blur()
		return v, nil
	}

	return v, nil
}

func (v *SettingsView) setVariable() tea.Cmd {
	if v.cursor >= len(v.variables) {
		return nil
	}

	varName := v.variables[v.cursor].Name
	varValue := v.editInput.Value()

	return func() tea.Msg {
		err := v.conn.SetVariable(varName, varValue, v.showGlobal)
		if err != nil {
			return err
		}
		return variableSetMsg{name: varName, value: varValue}
	}
}

// View renders the view
func (v *SettingsView) View() string {
	var b strings.Builder

	// Title
	scope := "Session"
	if v.showGlobal {
		scope = "Global"
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("System Variables (%s)", scope)))
	b.WriteString("\n\n")

	// Filter input (when filtering)
	if v.filtering {
		b.WriteString("Filter: ")
		b.WriteString(v.filterInput.View())
		b.WriteString("\n\n")
	} else if v.filter != "" {
		b.WriteString(mutedStyle.Render(fmt.Sprintf("Filter: %s (press 'c' to clear)", v.filter)))
		b.WriteString("\n\n")
	}

	// Error message
	if v.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", v.err)))
		b.WriteString("\n\n")
	}

	// Status message
	if v.statusMsg != "" && v.err == nil {
		b.WriteString(successStyle.Render(v.statusMsg))
		b.WriteString("\n\n")
	}

	// Variables list
	if len(v.variables) == 0 {
		b.WriteString(mutedStyle.Render("No variables found."))
		b.WriteString("\n")
	} else {
		// Calculate max name width for alignment
		maxNameWidth := 0
		for _, variable := range v.variables {
			if len(variable.Name) > maxNameWidth {
				maxNameWidth = len(variable.Name)
			}
		}
		if maxNameWidth > 35 {
			maxNameWidth = 35
		}

		// Determine visible range
		visibleHeight := v.height - 12
		if visibleHeight < 5 {
			visibleHeight = 5
		}

		startIdx := 0
		if v.cursor >= visibleHeight {
			startIdx = v.cursor - visibleHeight + 1
		}

		endIdx := startIdx + visibleHeight
		if endIdx > len(v.variables) {
			endIdx = len(v.variables)
		}

		for i := startIdx; i < endIdx; i++ {
			variable := v.variables[i]
			name := variable.Name
			if len(name) > maxNameWidth {
				name = name[:maxNameWidth-3] + "..."
			}

			// Pad name for alignment
			paddedName := fmt.Sprintf("%-*s", maxNameWidth, name)

			// Truncate value if too long
			value := variable.Value
			maxValueWidth := v.width - maxNameWidth - 10
			if maxValueWidth < 20 {
				maxValueWidth = 20
			}
			if len(value) > maxValueWidth {
				value = value[:maxValueWidth-3] + "..."
			}

			if i == v.cursor {
				if v.editing {
					// Show edit input
					b.WriteString(focusedStyle.Render(paddedName))
					b.WriteString(" = ")
					b.WriteString(v.editInput.View())
				} else {
					// Highlighted row
					rowStyle := lipgloss.NewStyle().
						Foreground(lipgloss.Color("#FFFFFF")).
						Background(lipgloss.Color("#FF69B4")).
						Bold(true)
					b.WriteString(rowStyle.Render(fmt.Sprintf(" %s = %s ", paddedName, value)))
				}
			} else {
				b.WriteString(fmt.Sprintf(" %s = %s", paddedName, value))
			}
			b.WriteString("\n")
		}

		// Scroll indicator
		if len(v.variables) > visibleHeight {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("\n[%d/%d]", v.cursor+1, len(v.variables))))
		}
	}

	b.WriteString("\n\n")

	// Help
	var help string
	if v.filtering {
		help = "Enter: Apply filter | Esc: Cancel"
	} else if v.editing {
		help = "Enter: Save | Esc: Cancel"
	} else {
		help = "↑↓: Navigate | Enter: Edit | /: Filter | c: Clear filter | g: Toggle Global/Session | r: Refresh | Esc: Back"
	}
	b.WriteString(helpStyle.Render(help))

	return b.String()
}
