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
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// BrowserView shows table data
type BrowserView struct {
	conn     *db.Connection
	database string
	tableName string
	table    table.Model
	columns  []string
	rows     [][]string
	page     int
	pageSize int
	total    int64
	width    int
	height   int
	err      error
}

// NewBrowserView creates a new table browser view
func NewBrowserView(conn *db.Connection, database, tableName string, width, height int) *BrowserView {
	t := table.New(
		table.WithFocused(true),
		table.WithHeight(height-8),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#FF69B4")).
		BorderBottom(true).
		Bold(true).
		Foreground(lipgloss.Color("#FF69B4"))
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#FF69B4")).
		Bold(true)
	t.SetStyles(s)

	return &BrowserView{
		conn:     conn,
		database: database,
		tableName: tableName,
		table:    t,
		page:     0,
		pageSize: 50,
		width:    width,
		height:   height,
	}
}

// Init initializes the view
func (v *BrowserView) Init() tea.Cmd {
	return v.loadData
}

func (v *BrowserView) loadData() tea.Msg {
	if err := v.conn.UseDatabase(v.database); err != nil {
		return err
	}

	// Get total count
	total, err := v.conn.CountTableRows(v.tableName)
	if err != nil {
		return err
	}

	// Get data
	result, err := v.conn.GetTableData(v.tableName, v.pageSize, v.page*v.pageSize)
	if err != nil {
		return err
	}

	return browserData{
		columns: result.Columns,
		rows:    result.Rows,
		total:   total,
	}
}

type browserData struct {
	columns []string
	rows    [][]string
	total   int64
}

// Update handles messages
func (v *BrowserView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace":
			return v, func() tea.Msg {
				return SwitchViewMsg{
					View:     "tables",
					Database: v.database,
				}
			}
		case "q":
			return v, tea.Quit
		case "n", "right":
			// Next page
			maxPage := int(v.total) / v.pageSize
			if v.page < maxPage {
				v.page++
				return v, v.loadData
			}
		case "p", "left":
			// Previous page
			if v.page > 0 {
				v.page--
				return v, v.loadData
			}
		case "g":
			// Go to first page
			if v.page != 0 {
				v.page = 0
				return v, v.loadData
			}
		case "G":
			// Go to last page
			maxPage := int(v.total) / v.pageSize
			if v.page != maxPage {
				v.page = maxPage
				return v, v.loadData
			}
		case "r":
			return v, v.loadData
		}

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		v.table.SetHeight(msg.Height - 8)

	case browserData:
		v.columns = msg.columns
		v.rows = msg.rows
		v.total = msg.total
		v.updateTable()
		return v, nil

	case error:
		v.err = msg
		return v, nil
	}

	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return v, cmd
}

func (v *BrowserView) updateTable() {
	// Calculate column widths
	colWidths := make([]int, len(v.columns))
	maxWidth := (v.width - 4) / len(v.columns)
	if maxWidth < 10 {
		maxWidth = 10
	}
	if maxWidth > 40 {
		maxWidth = 40
	}

	for i, col := range v.columns {
		colWidths[i] = min(len(col)+2, maxWidth)
	}

	// Check data widths
	for _, row := range v.rows {
		for i, cell := range row {
			if i < len(colWidths) {
				w := min(len(cell)+2, maxWidth)
				if w > colWidths[i] {
					colWidths[i] = w
				}
			}
		}
	}

	// Create columns
	cols := make([]table.Column, len(v.columns))
	for i, name := range v.columns {
		cols[i] = table.Column{Title: name, Width: colWidths[i]}
	}

	// Create rows
	rows := make([]table.Row, len(v.rows))
	for i, row := range v.rows {
		r := make(table.Row, len(row))
		for j, cell := range row {
			// Truncate long values
			if len(cell) > maxWidth-3 {
				cell = cell[:maxWidth-6] + "..."
			}
			r[j] = cell
		}
		rows[i] = r
	}

	v.table.SetColumns(cols)
	v.table.SetRows(rows)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// View renders the view
func (v *BrowserView) View() string {
	var b strings.Builder

	// Title
	title := fmt.Sprintf("Table: %s.%s", v.database, v.tableName)
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	if v.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", v.err)))
		b.WriteString("\n\n")
	}

	// Table
	b.WriteString(v.table.View())
	b.WriteString("\n\n")

	// Pagination info
	start := v.page*v.pageSize + 1
	end := start + len(v.rows) - 1
	if end > int(v.total) {
		end = int(v.total)
	}
	pageInfo := fmt.Sprintf("Showing %d-%d of %d rows (Page %d)", start, end, v.total, v.page+1)
	b.WriteString(mutedStyle.Render(pageInfo))
	b.WriteString("\n")

	// Help
	b.WriteString(helpStyle.Render("←/p: Prev page | →/n: Next page | g/G: First/Last | r: Refresh | Esc: Back | q: Quit"))

	return b.String()
}
