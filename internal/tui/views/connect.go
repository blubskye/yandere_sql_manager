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

// ConnectView is the connection form view
type ConnectView struct {
	inputs       []textinput.Model
	focused      int
	profiles     []string
	selectedProf int
	showProfiles bool
	cfg          *config.Config
	connCfg      *db.ConnectionConfig
	err          error
	connecting   bool
	width        int
	height       int
}

const (
	inputHost = iota
	inputPort
	inputUser
	inputPassword
	inputDatabase
)

// NewConnectView creates a new connect view
func NewConnectView(cfg *config.Config, connCfg *db.ConnectionConfig) *ConnectView {
	v := &ConnectView{
		inputs:  make([]textinput.Model, 5),
		cfg:     cfg,
		connCfg: connCfg,
	}

	// Host input
	v.inputs[inputHost] = textinput.New()
	v.inputs[inputHost].Placeholder = "localhost"
	v.inputs[inputHost].Focus()
	v.inputs[inputHost].PromptStyle = focusedStyle
	v.inputs[inputHost].TextStyle = focusedStyle

	// Port input
	v.inputs[inputPort] = textinput.New()
	v.inputs[inputPort].Placeholder = "3306"

	// User input
	v.inputs[inputUser] = textinput.New()
	v.inputs[inputUser].Placeholder = "root"

	// Password input
	v.inputs[inputPassword] = textinput.New()
	v.inputs[inputPassword].Placeholder = "password"
	v.inputs[inputPassword].EchoMode = textinput.EchoPassword
	v.inputs[inputPassword].EchoCharacter = '•'

	// Database input
	v.inputs[inputDatabase] = textinput.New()
	v.inputs[inputDatabase].Placeholder = "(optional)"

	// Load profiles
	v.profiles = cfg.ListProfiles()

	// Apply initial connection config if provided
	if connCfg != nil {
		v.inputs[inputHost].SetValue(connCfg.Host)
		v.inputs[inputPort].SetValue(strconv.Itoa(connCfg.Port))
		v.inputs[inputUser].SetValue(connCfg.User)
		v.inputs[inputPassword].SetValue(connCfg.Password)
		v.inputs[inputDatabase].SetValue(connCfg.Database)
	} else if cfg.DefaultProfile != "" {
		// Try to load default profile
		if p, err := cfg.GetProfile(cfg.DefaultProfile); err == nil {
			v.applyProfile(p)
		}
	}

	return v
}

func (v *ConnectView) applyProfile(p *config.Profile) {
	v.inputs[inputHost].SetValue(p.Host)
	if p.Port > 0 {
		v.inputs[inputPort].SetValue(strconv.Itoa(p.Port))
	}
	v.inputs[inputUser].SetValue(p.User)
	v.inputs[inputPassword].SetValue(p.Password)
	v.inputs[inputDatabase].SetValue(p.Database)
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
			return v, tea.Quit

		case "tab", "down":
			if v.showProfiles {
				v.selectedProf++
				if v.selectedProf >= len(v.profiles) {
					v.selectedProf = 0
				}
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
			return v, v.connect()

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

	// Handle input updates
	if !v.showProfiles {
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
	v.inputs[v.focused].Blur()
	v.inputs[v.focused].PromptStyle = blurredStyle
	v.inputs[v.focused].TextStyle = blurredStyle

	v.focused++
	if v.focused >= len(v.inputs) {
		v.focused = 0
	}

	v.inputs[v.focused].Focus()
	v.inputs[v.focused].PromptStyle = focusedStyle
	v.inputs[v.focused].TextStyle = focusedStyle
}

func (v *ConnectView) prevInput() {
	v.inputs[v.focused].Blur()
	v.inputs[v.focused].PromptStyle = blurredStyle
	v.inputs[v.focused].TextStyle = blurredStyle

	v.focused--
	if v.focused < 0 {
		v.focused = len(v.inputs) - 1
	}

	v.inputs[v.focused].Focus()
	v.inputs[v.focused].PromptStyle = focusedStyle
	v.inputs[v.focused].TextStyle = focusedStyle
}

func (v *ConnectView) connect() tea.Cmd {
	v.connecting = true
	v.err = nil

	return func() tea.Msg {
		host := v.inputs[inputHost].Value()
		if host == "" {
			host = "localhost"
		}

		portStr := v.inputs[inputPort].Value()
		port := 3306
		if portStr != "" {
			if p, err := strconv.Atoi(portStr); err == nil {
				port = p
			}
		}

		cfg := db.ConnectionConfig{
			Host:     host,
			Port:     port,
			User:     v.inputs[inputUser].Value(),
			Password: v.inputs[inputPassword].Value(),
			Database: v.inputs[inputDatabase].Value(),
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

	// Connection form
	b.WriteString(titleStyle.Render("Connect to MariaDB"))
	b.WriteString("\n\n")

	labels := []string{"Host:", "Port:", "User:", "Password:", "Database:"}
	for i, input := range v.inputs {
		if i == v.focused {
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
