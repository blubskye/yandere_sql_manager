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
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TablesView shows the list of tables in a database
type TablesView struct {
	conn     *db.Connection
	database string
	list     list.Model
	tables   []db.Table
	width    int
	height   int
	err      error
}

type tableItem struct {
	name   string
	engine string
	rows   int64
}

func (i tableItem) Title() string       { return i.name }
func (i tableItem) Description() string { return fmt.Sprintf("%s | %d rows", i.engine, i.rows) }
func (i tableItem) FilterValue() string { return i.name }

// NewTablesView creates a new tables view
func NewTablesView(conn *db.Connection, database string, width, height int) *TablesView {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#FF69B4")).
		Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("#FFB6C1")).
		Background(lipgloss.Color("#FF69B4"))

	l := list.New([]list.Item{}, delegate, width, height-4)
	l.Title = fmt.Sprintf("Tables in %s", database)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle

	return &TablesView{
		conn:     conn,
		database: database,
		list:     l,
		width:    width,
		height:   height,
	}
}

// Init initializes the view
func (v *TablesView) Init() tea.Cmd {
	return v.loadTables
}

func (v *TablesView) loadTables() tea.Msg {
	if err := v.conn.UseDatabase(v.database); err != nil {
		return err
	}

	tables, err := v.conn.ListTables()
	if err != nil {
		return err
	}
	return tables
}

// Update handles messages
func (v *TablesView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if item, ok := v.list.SelectedItem().(tableItem); ok {
				return v, func() tea.Msg {
					return SwitchViewMsg{
						View:     "browser",
						Database: v.database,
						Table:    item.name,
					}
				}
			}
		case "esc", "backspace":
			if !v.list.SettingFilter() {
				return v, func() tea.Msg {
					return SwitchViewMsg{View: "databases"}
				}
			}
		case "q":
			if !v.list.SettingFilter() {
				return v, tea.Quit
			}
		case "d":
			if !v.list.SettingFilter() {
				if item, ok := v.list.SelectedItem().(tableItem); ok {
					return v, func() tea.Msg {
						cols, err := v.conn.DescribeTable(item.name)
						if err != nil {
							return err
						}
						return describeResult{table: item.name, columns: cols}
					}
				}
			}
		case "s":
			if !v.list.SettingFilter() {
				return v, func() tea.Msg {
					return SwitchViewMsg{
						View:     "query",
						Database: v.database,
					}
				}
			}
		case "r":
			if !v.list.SettingFilter() {
				return v, v.loadTables
			}
		}

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		v.list.SetSize(msg.Width, msg.Height-4)

	case []db.Table:
		v.tables = msg
		items := make([]list.Item, len(msg))
		for i, t := range msg {
			items[i] = tableItem{name: t.Name, engine: t.Engine, rows: t.Rows}
		}
		v.list.SetItems(items)
		return v, nil

	case describeResult:
		// Show table structure in a popup or message
		// For now, just show in status
		return v, nil

	case error:
		v.err = msg
		return v, nil
	}

	var cmd tea.Cmd
	v.list, cmd = v.list.Update(msg)
	return v, cmd
}

type describeResult struct {
	table   string
	columns []db.Column
}

// View renders the view
func (v *TablesView) View() string {
	var b strings.Builder

	if v.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", v.err)))
		b.WriteString("\n\n")
	}

	b.WriteString(v.list.View())
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Enter: Browse | d: Describe | s: SQL | r: Refresh | Esc: Back | q: Quit"))

	return b.String()
}
