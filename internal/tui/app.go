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

package tui

import (
	"fmt"

	"github.com/blubskye/yandere_sql_manager/internal/config"
	"github.com/blubskye/yandere_sql_manager/internal/db"
	"github.com/blubskye/yandere_sql_manager/internal/tui/views"
	tea "github.com/charmbracelet/bubbletea"
)

// ViewType represents the current view
type ViewType int

const (
	ViewConnect ViewType = iota
	ViewDatabases
	ViewTables
	ViewBrowser
	ViewQuery
	ViewImport
	ViewExport
)

// Model is the main application model
type Model struct {
	width  int
	height int

	conn    *db.Connection
	connCfg *db.ConnectionConfig
	cfg     *config.Config

	currentView ViewType
	views       map[ViewType]tea.Model

	err        error
	statusMsg  string
	quitting   bool
}

// New creates a new TUI application
func New(connCfg *db.ConnectionConfig) *Model {
	cfg, _ := config.Load()
	if cfg == nil {
		cfg = &config.Config{
			Profiles: make(map[string]config.Profile),
		}
	}

	m := &Model{
		connCfg:     connCfg,
		cfg:         cfg,
		currentView: ViewConnect,
		views:       make(map[ViewType]tea.Model),
	}

	// Initialize connect view
	m.views[ViewConnect] = views.NewConnectView(cfg, connCfg)

	return m
}

// Init initializes the application
func (m *Model) Init() tea.Cmd {
	return m.views[ViewConnect].Init()
}

// Update handles messages
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			if m.conn != nil {
				m.conn.Close()
			}
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Propagate to current view
		if view, ok := m.views[m.currentView]; ok {
			newView, cmd := view.Update(msg)
			m.views[m.currentView] = newView
			return m, cmd
		}

	// Handle connected message from connect view
	case views.ConnectedMsg:
		m.conn = msg.Conn
		m.statusMsg = "Connected!"
		m.currentView = ViewDatabases
		m.views[ViewDatabases] = views.NewDatabasesView(m.conn, m.width, m.height)
		return m, m.views[ViewDatabases].Init()

	// Handle view switching from views
	case views.SwitchViewMsg:
		return m.switchViewString(msg.View, msg.Database, msg.Table)

	case error:
		m.err = msg
		return m, nil
	}

	// Update current view
	if view, ok := m.views[m.currentView]; ok {
		newView, cmd := view.Update(msg)
		m.views[m.currentView] = newView
		return m, cmd
	}

	return m, nil
}

func (m *Model) switchViewString(viewName, database, table string) (tea.Model, tea.Cmd) {
	switch viewName {
	case "connect":
		m.currentView = ViewConnect
		if _, ok := m.views[ViewConnect]; !ok {
			m.views[ViewConnect] = views.NewConnectView(m.cfg, m.connCfg)
		}
	case "databases":
		m.currentView = ViewDatabases
		m.views[ViewDatabases] = views.NewDatabasesView(m.conn, m.width, m.height)
	case "tables":
		m.currentView = ViewTables
		m.views[ViewTables] = views.NewTablesView(m.conn, database, m.width, m.height)
	case "browser":
		m.currentView = ViewBrowser
		m.views[ViewBrowser] = views.NewBrowserView(m.conn, database, table, m.width, m.height)
	case "query":
		m.currentView = ViewQuery
		m.views[ViewQuery] = views.NewQueryView(m.conn, database, m.width, m.height)
	case "import":
		m.currentView = ViewImport
		m.views[ViewImport] = views.NewImportView(m.conn, database, m.width, m.height)
	case "export":
		m.currentView = ViewExport
		m.views[ViewExport] = views.NewExportView(m.conn, database, m.width, m.height)
	}

	if view, ok := m.views[m.currentView]; ok {
		return m, view.Init()
	}

	return m, nil
}

// View renders the application
func (m *Model) View() string {
	if m.quitting {
		return "Goodbye~ I'll be waiting for you...\n"
	}

	// Get current view
	var content string
	if view, ok := m.views[m.currentView]; ok {
		content = view.View()
	} else {
		content = "Loading..."
	}

	// Add status bar at bottom
	status := m.renderStatusBar()

	return content + "\n" + status
}

func (m *Model) renderStatusBar() string {
	var status string
	if m.conn != nil {
		dbName := m.conn.Config.Database
		if dbName == "" {
			dbName = "(none)"
		}
		status = fmt.Sprintf(" %s@%s:%d | DB: %s ",
			m.conn.Config.User, m.conn.Config.Host, m.conn.Config.Port, dbName)
	}

	if m.err != nil {
		status += errorStyle.Render(fmt.Sprintf(" | Error: %v", m.err))
	} else if m.statusMsg != "" {
		status += fmt.Sprintf(" | %s", m.statusMsg)
	}

	return statusBarStyle.Width(m.width).Render(status)
}

// Run starts the TUI application
func Run(connCfg *db.ConnectionConfig) error {
	p := tea.NewProgram(New(connCfg), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
