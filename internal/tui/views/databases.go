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
	"github.com/blubskye/yandere_sql_manager/internal/db"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SwitchViewMsg is sent to switch to a different view
type SwitchViewMsg struct {
	View     string
	Database string
	Table    string
}

// DatabasesView shows the list of databases
type DatabasesView struct {
	conn        *db.Connection
	list        list.Model
	databases   []db.Database
	width       int
	height      int
	err         error
	keybindings *config.KeyBindings
}

type dbItem struct {
	name string
}

func (i dbItem) Title() string       { return i.name }
func (i dbItem) Description() string { return "" }
func (i dbItem) FilterValue() string { return i.name }

// NewDatabasesView creates a new databases view
func NewDatabasesView(conn *db.Connection, width, height int) *DatabasesView {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#FF69B4")).
		Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("#FFB6C1")).
		Background(lipgloss.Color("#FF69B4"))

	l := list.New([]list.Item{}, delegate, width, height-4)
	l.Title = "Databases"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle

	// Load keybindings
	kb, _ := config.LoadKeyBindings()
	if kb == nil {
		kb = config.DefaultKeyBindings()
	}

	return &DatabasesView{
		conn:        conn,
		list:        l,
		width:       width,
		height:      height,
		keybindings: kb,
	}
}

// Init initializes the view
func (v *DatabasesView) Init() tea.Cmd {
	return v.loadDatabases
}

func (v *DatabasesView) loadDatabases() tea.Msg {
	databases, err := v.conn.ListDatabases()
	if err != nil {
		return err
	}
	return databases
}

// Update handles messages
func (v *DatabasesView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		// Handle keybindings when not filtering
		if !v.list.SettingFilter() {
			// Check against configured keybindings
			if v.keybindings.IsKey("databases", key, config.ActionSelect) || key == "enter" {
				if item, ok := v.list.SelectedItem().(dbItem); ok {
					return v, func() tea.Msg {
						return SwitchViewMsg{
							View:     "tables",
							Database: item.name,
						}
					}
				}
			}
			if v.keybindings.IsKey("databases", key, config.ActionQuit) {
				return v, tea.Quit
			}
			if v.keybindings.IsKey("databases", key, config.ActionImport) {
				var dbName string
				if item, ok := v.list.SelectedItem().(dbItem); ok {
					dbName = item.name
				}
				return v, func() tea.Msg {
					return SwitchViewMsg{
						View:     "import",
						Database: dbName,
					}
				}
			}
			if v.keybindings.IsKey("databases", key, config.ActionExport) {
				if item, ok := v.list.SelectedItem().(dbItem); ok {
					return v, func() tea.Msg {
						return SwitchViewMsg{
							View:     "export",
							Database: item.name,
						}
					}
				}
			}
			if v.keybindings.IsKey("databases", key, config.ActionQuery) {
				var dbName string
				if item, ok := v.list.SelectedItem().(dbItem); ok {
					dbName = item.name
				}
				return v, func() tea.Msg {
					return SwitchViewMsg{
						View:     "query",
						Database: dbName,
					}
				}
			}
			if v.keybindings.IsKey("databases", key, config.ActionRefresh) {
				return v, v.loadDatabases
			}
			if v.keybindings.IsKey("databases", key, config.ActionVariables) {
				return v, func() tea.Msg {
					return SwitchViewMsg{View: "settings"}
				}
			}
			if v.keybindings.IsKey("databases", key, config.ActionUsers) {
				return v, func() tea.Msg {
					return SwitchViewMsg{View: "users"}
				}
			}
			if v.keybindings.IsKey("databases", key, config.ActionBackup) {
				return v, func() tea.Msg {
					return SwitchViewMsg{View: "backup"}
				}
			}
			if v.keybindings.IsKey("databases", key, config.ActionNewDatabase) {
				return v, func() tea.Msg {
					return SwitchViewMsg{View: "setup"}
				}
			}
			if v.keybindings.IsKey("databases", key, config.ActionDashboard) {
				return v, func() tea.Msg {
					return SwitchViewMsg{View: "dashboard"}
				}
			}
			if v.keybindings.IsKey("databases", key, config.ActionCluster) {
				return v, func() tea.Msg {
					return SwitchViewMsg{View: "cluster"}
				}
			}
			if v.keybindings.IsKey("databases", key, config.ActionSettings) {
				return v, func() tea.Msg {
					return SwitchViewMsg{View: "keybindings"}
				}
			}
		}

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		v.list.SetSize(msg.Width, msg.Height-4)

	case []db.Database:
		v.databases = msg
		items := make([]list.Item, len(msg))
		for i, d := range msg {
			items[i] = dbItem{name: d.Name}
		}
		v.list.SetItems(items)
		return v, nil

	case error:
		v.err = msg
		return v, nil
	}

	var cmd tea.Cmd
	v.list, cmd = v.list.Update(msg)
	return v, cmd
}

// View renders the view
func (v *DatabasesView) View() string {
	var b strings.Builder

	if v.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", v.err)))
		b.WriteString("\n\n")
	}

	b.WriteString(v.list.View())
	b.WriteString("\n")

	// Build help text with actual configured keybindings
	help := fmt.Sprintf("Enter: Select | /: Filter | %s: New | %s: Stats | %s: Cluster | %s: Users | %s: Backup | %s: Import | %s: Export | %s: Refresh | %s: Keys | %s: Quit",
		v.keybindings.GetKey("databases", config.ActionNewDatabase),
		v.keybindings.GetKey("databases", config.ActionDashboard),
		v.keybindings.GetKey("databases", config.ActionCluster),
		v.keybindings.GetKey("databases", config.ActionUsers),
		v.keybindings.GetKey("databases", config.ActionBackup),
		v.keybindings.GetKey("databases", config.ActionImport),
		v.keybindings.GetKey("databases", config.ActionExport),
		v.keybindings.GetKey("databases", config.ActionRefresh),
		v.keybindings.GetKey("databases", config.ActionSettings),
		v.keybindings.GetKey("databases", config.ActionQuit),
	)
	b.WriteString(helpStyle.Render(help))

	return b.String()
}
