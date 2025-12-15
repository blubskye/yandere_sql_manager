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
	"strconv"
	"strings"

	"github.com/blubskye/yandere_sql_manager/internal/config"
	"github.com/blubskye/yandere_sql_manager/internal/db"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Styles are defined in styles.go

// ConnectedMsg is sent when a connection is established
type ConnectedMsg struct {
	Conn *db.Connection
}

// Database type options
var dbTypes = []string{"mariadb", "postgres"}

// ConnectView is the connection form view
type ConnectView struct {
	inputs         []textinput.Model
	focused        int
	dbTypeIndex    int // 0 = mariadb, 1 = postgres
	showTypeMenu   bool
	profiles       []string
	selectedProf   int
	showProfiles   bool
	cfg            *config.Config
	connCfg        *db.ConnectionConfig
	err            error
	connecting     bool
	width          int
	height         int
}

const (
	inputType = iota // Type selector (not a text input)
	inputHost
	inputPort
	inputUser
	inputPassword
	inputDatabase
)

// NewConnectView creates a new connect view
func NewConnectView(cfg *config.Config, connCfg *db.ConnectionConfig) *ConnectView {
	v := &ConnectView{
		inputs:  make([]textinput.Model, 5), // 5 text inputs (type is handled separately)
		cfg:     cfg,
		connCfg: connCfg,
		focused: inputType, // Start focused on type selector
	}

	// Host input (index 0 in inputs slice, but inputHost-1 since type is not a text input)
	v.inputs[0] = textinput.New()
	v.inputs[0].Placeholder = "localhost"

	// Port input
	v.inputs[1] = textinput.New()
	v.inputs[1].Placeholder = "3306"

	// User input
	v.inputs[2] = textinput.New()
	v.inputs[2].Placeholder = "root"

	// Password input
	v.inputs[3] = textinput.New()
	v.inputs[3].Placeholder = "password"
	v.inputs[3].EchoMode = textinput.EchoPassword
	v.inputs[3].EchoCharacter = '•'

	// Database input
	v.inputs[4] = textinput.New()
	v.inputs[4].Placeholder = "(optional)"

	// Load profiles
	v.profiles = cfg.ListProfiles()

	// Apply initial connection config if provided
	if connCfg != nil {
		v.setDbType(string(connCfg.Type))
		v.inputs[0].SetValue(connCfg.Host)
		v.inputs[1].SetValue(strconv.Itoa(connCfg.Port))
		v.inputs[2].SetValue(connCfg.User)
		v.inputs[3].SetValue(connCfg.Password)
		v.inputs[4].SetValue(connCfg.Database)
	} else if cfg.DefaultProfile != "" {
		// Try to load default profile
		if p, err := cfg.GetProfile(cfg.DefaultProfile); err == nil {
			v.applyProfile(p)
		}
	}

	return v
}

// setDbType sets the database type and updates the port placeholder
func (v *ConnectView) setDbType(t string) {
	for i, dbType := range dbTypes {
		if dbType == t {
			v.dbTypeIndex = i
			break
		}
	}
	v.updatePortPlaceholder()
}

// updatePortPlaceholder updates the port placeholder based on selected db type
func (v *ConnectView) updatePortPlaceholder() {
	if v.dbTypeIndex == 0 {
		v.inputs[1].Placeholder = "3306"
	} else {
		v.inputs[1].Placeholder = "5432"
	}
}

func (v *ConnectView) applyProfile(p *config.Profile) {
	t := p.Type
	if t == "" {
		t = "mariadb"
	}
	v.setDbType(t)
	v.inputs[0].SetValue(p.Host) // Host
	if p.Port > 0 {
		v.inputs[1].SetValue(strconv.Itoa(p.Port)) // Port
	}
	v.inputs[2].SetValue(p.User)     // User
	v.inputs[3].SetValue(p.Password) // Password
	v.inputs[4].SetValue(p.Database) // Database
}

// Init initializes the view
func (v *ConnectView) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages
func (v *ConnectView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			if v.showProfiles {
				v.showProfiles = false
				return v, nil
			}
			if v.showTypeMenu {
				v.showTypeMenu = false
				return v, nil
			}
			return v, tea.Quit

		case "tab", "down":
			if v.showProfiles {
				v.selectedProf++
				if v.selectedProf >= len(v.profiles) {
					v.selectedProf = 0
				}
				return v, nil
			}
			if v.showTypeMenu {
				v.dbTypeIndex++
				if v.dbTypeIndex >= len(dbTypes) {
					v.dbTypeIndex = 0
				}
				v.updatePortPlaceholder()
				return v, nil
			}
			v.nextInput()
			return v, nil

		case "shift+tab", "up":
			if v.showProfiles {
				v.selectedProf--
				if v.selectedProf < 0 {
					v.selectedProf = len(v.profiles) - 1
				}
				return v, nil
			}
			if v.showTypeMenu {
				v.dbTypeIndex--
				if v.dbTypeIndex < 0 {
					v.dbTypeIndex = len(dbTypes) - 1
				}
				v.updatePortPlaceholder()
				return v, nil
			}
			v.prevInput()
			return v, nil

		case "enter":
			if v.showProfiles {
				if v.selectedProf < len(v.profiles) {
					if p, err := v.cfg.GetProfile(v.profiles[v.selectedProf]); err == nil {
						v.applyProfile(p)
					}
				}
				v.showProfiles = false
				return v, nil
			}
			if v.showTypeMenu {
				v.showTypeMenu = false
				return v, nil
			}
			// If on type field, show dropdown
			if v.focused == inputType {
				v.showTypeMenu = true
				return v, nil
			}
			return v, v.connect()

		case "left", "right":
			// Quick toggle for type selector when focused
			if v.focused == inputType && !v.showTypeMenu {
				if msg.String() == "left" {
					v.dbTypeIndex--
					if v.dbTypeIndex < 0 {
						v.dbTypeIndex = len(dbTypes) - 1
					}
				} else {
					v.dbTypeIndex++
					if v.dbTypeIndex >= len(dbTypes) {
						v.dbTypeIndex = 0
					}
				}
				v.updatePortPlaceholder()
				return v, nil
			}

		case "ctrl+p":
			if len(v.profiles) > 0 {
				v.showProfiles = !v.showProfiles
			}
			return v, nil
		}

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height

	case ConnectedMsg:
		v.connecting = false
		// This will be handled by the parent app
		return v, nil

	case error:
		v.connecting = false
		v.err = msg
		return v, nil
	}

	// Handle input updates (only for text inputs, not type selector)
	if !v.showProfiles && !v.showTypeMenu && v.focused > inputType {
		cmd := v.updateInputs(msg)
		return v, cmd
	}

	return v, nil
}

func (v *ConnectView) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(v.inputs))

	for i := range v.inputs {
		v.inputs[i], cmds[i] = v.inputs[i].Update(msg)
	}

	return tea.Batch(cmds...)
}

func (v *ConnectView) nextInput() {
	// Blur current text input if applicable
	if v.focused > inputType {
		idx := v.focused - 1 // Convert to inputs slice index
		v.inputs[idx].Blur()
		v.inputs[idx].PromptStyle = blurredStyle
		v.inputs[idx].TextStyle = blurredStyle
	}

	v.focused++
	if v.focused > inputDatabase {
		v.focused = inputType
	}

	// Focus new text input if applicable
	if v.focused > inputType {
		idx := v.focused - 1 // Convert to inputs slice index
		v.inputs[idx].Focus()
		v.inputs[idx].PromptStyle = focusedStyle
		v.inputs[idx].TextStyle = focusedStyle
	}
}

func (v *ConnectView) prevInput() {
	// Blur current text input if applicable
	if v.focused > inputType {
		idx := v.focused - 1 // Convert to inputs slice index
		v.inputs[idx].Blur()
		v.inputs[idx].PromptStyle = blurredStyle
		v.inputs[idx].TextStyle = blurredStyle
	}

	v.focused--
	if v.focused < inputType {
		v.focused = inputDatabase
	}

	// Focus new text input if applicable
	if v.focused > inputType {
		idx := v.focused - 1 // Convert to inputs slice index
		v.inputs[idx].Focus()
		v.inputs[idx].PromptStyle = focusedStyle
		v.inputs[idx].TextStyle = focusedStyle
	}
}

func (v *ConnectView) connect() tea.Cmd {
	v.connecting = true
	v.err = nil

	// Capture values for the goroutine
	dbTypeStr := dbTypes[v.dbTypeIndex]
	hostVal := v.inputs[0].Value() // Host
	portVal := v.inputs[1].Value() // Port
	userVal := v.inputs[2].Value() // User
	passVal := v.inputs[3].Value() // Password
	dbVal := v.inputs[4].Value()   // Database

	return func() tea.Msg {
		host := hostVal
		if host == "" {
			host = "localhost"
		}

		connType := db.DatabaseType(dbTypeStr)
		defaultPort := db.DefaultPort(connType)

		port := defaultPort
		if portVal != "" {
			if p, err := strconv.Atoi(portVal); err == nil {
				port = p
			}
		}

		cfg := db.ConnectionConfig{
			Type:     connType,
			Host:     host,
			Port:     port,
			User:     userVal,
			Password: passVal,
			Database: dbVal,
		}

		conn, err := db.Connect(cfg)
		if err != nil {
			return err
		}

		return ConnectedMsg{Conn: conn}
	}
}

// View renders the view
func (v *ConnectView) View() string {
	var b strings.Builder

	// Logo
	b.WriteString(logo())
	b.WriteString("\n\n")

	// Profile selector popup
	if v.showProfiles {
		b.WriteString(v.renderProfileSelector())
		return b.String()
	}

	// Type selector popup
	if v.showTypeMenu {
		b.WriteString(v.renderTypeSelector())
		return b.String()
	}

	// Connection form
	b.WriteString(titleStyle.Render("Connect to Database"))
	b.WriteString("\n\n")

	// Type selector (first field)
	if v.focused == inputType {
		b.WriteString(focusedStyle.Render("Type:"))
	} else {
		b.WriteString(blurredStyle.Render("Type:"))
	}
	b.WriteString("\n")
	typeDisplay := fmt.Sprintf("[ %s ]", dbTypes[v.dbTypeIndex])
	if v.focused == inputType {
		b.WriteString(focusedStyle.Render(typeDisplay))
		b.WriteString(mutedStyle.Render("  ←/→ to change, Enter for menu"))
	} else {
		b.WriteString(blurredStyle.Render(typeDisplay))
	}
	b.WriteString("\n\n")

	// Text input fields
	labels := []string{"Host:", "Port:", "User:", "Password:", "Database:"}
	for i, input := range v.inputs {
		fieldIndex := i + 1 // Offset by 1 since type is at index 0
		if fieldIndex == v.focused {
			b.WriteString(focusedStyle.Render(labels[i]))
		} else {
			b.WriteString(blurredStyle.Render(labels[i]))
		}
		b.WriteString("\n")
		b.WriteString(input.View())
		b.WriteString("\n\n")
	}

	// Error message
	if v.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", v.err)))
		b.WriteString("\n\n")
	}

	// Status
	if v.connecting {
		b.WriteString("Connecting...\n\n")
	}

	// Help
	help := []string{"Enter: Connect", "Tab: Next field", "Ctrl+C: Quit"}
	if len(v.profiles) > 0 {
		help = append(help, "Ctrl+P: Profiles")
	}
	b.WriteString(helpStyle.Render(strings.Join(help, " | ")))

	return b.String()
}

func (v *ConnectView) renderTypeSelector() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Select Database Type"))
	b.WriteString("\n\n")

	for i, t := range dbTypes {
		label := t
		if t == "mariadb" {
			label = "MariaDB / MySQL"
		} else if t == "postgres" {
			label = "PostgreSQL"
		}
		if i == v.dbTypeIndex {
			b.WriteString(focusedStyle.Render("→ " + label))
		} else {
			b.WriteString("  " + label)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Enter: Select | Esc: Cancel | ↑↓: Navigate"))

	return b.String()
}

func (v *ConnectView) renderProfileSelector() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Select Profile"))
	b.WriteString("\n\n")

	for i, name := range v.profiles {
		if i == v.selectedProf {
			b.WriteString(focusedStyle.Render("→ " + name))
		} else {
			b.WriteString("  " + name)
		}
		if name == v.cfg.DefaultProfile {
			b.WriteString(mutedStyle.Render(" (default)"))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Enter: Select | Esc: Cancel | ↑↓: Navigate"))

	return b.String()
}

func logo() string {
	return bannerStyle.Render(`
  ██╗   ██╗███████╗███╗   ███╗
  ╚██╗ ██╔╝██╔════╝████╗ ████║
   ╚████╔╝ ███████╗██╔████╔██║
    ╚██╔╝  ╚════██║██║╚██╔╝██║
     ██║   ███████║██║ ╚═╝ ██║
     ╚═╝   ╚══════╝╚═╝     ╚═╝
`) + subtitleStyle.Render("  Yandere SQL Manager - \"I'll never let your databases go~\"")
}
