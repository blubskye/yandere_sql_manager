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
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// QueryView is the SQL query editor
type QueryView struct {
	conn      *db.Connection
	database  string
	textarea  textarea.Model
	results   table.Model
	columns   []string
	rows      [][]string
	affected  int64
	width     int
	height    int
	err       error
	showResults bool
	history   []string
	historyIdx int
}

// NewQueryView creates a new query view
func NewQueryView(conn *db.Connection, database string, width, height int) *QueryView {
	ta := textarea.New()
	ta.Placeholder = "Enter SQL query..."
	ta.Focus()
	ta.SetWidth(width - 4)
	ta.SetHeight(8)
	ta.CharLimit = 10000
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = true

	t := table.New(
		table.WithFocused(false),
		table.WithHeight(height - 16),
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

	return &QueryView{
		conn:     conn,
		database: database,
		textarea: ta,
		results:  t,
		width:    width,
		height:   height,
		history:  make([]string, 0),
		historyIdx: -1,
	}
}

// Init initializes the view
func (v *QueryView) Init() tea.Cmd {
	if v.database != "" {
		v.conn.UseDatabase(v.database)
	}
	return textarea.Blink
}

// Update handles messages
func (v *QueryView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if v.showResults {
				v.showResults = false
				v.textarea.Focus()
				return v, nil
			}
			return v, func() tea.Msg {
				return SwitchViewMsg{
					View:     "databases",
				}
			}
		case "ctrl+enter", "f5":
			return v, v.executeQuery()
		case "ctrl+up":
			// Previous history
			if len(v.history) > 0 && v.historyIdx < len(v.history)-1 {
				v.historyIdx++
				v.textarea.SetValue(v.history[len(v.history)-1-v.historyIdx])
			}
			return v, nil
		case "ctrl+down":
			// Next history
			if v.historyIdx > 0 {
				v.historyIdx--
				v.textarea.SetValue(v.history[len(v.history)-1-v.historyIdx])
			} else if v.historyIdx == 0 {
				v.historyIdx = -1
				v.textarea.SetValue("")
			}
			return v, nil
		case "tab":
			if v.showResults {
				v.showResults = false
				v.textarea.Focus()
			} else if len(v.rows) > 0 {
				v.showResults = true
				v.textarea.Blur()
			}
			return v, nil
		case "q":
			if v.showResults {
				return v, tea.Quit
			}
		case "ctrl+c":
			return v, tea.Quit
		}

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		v.textarea.SetWidth(msg.Width - 4)
		v.results.SetHeight(msg.Height - 16)

	case queryResult:
		v.columns = msg.columns
		v.rows = msg.rows
		v.affected = msg.affected
		v.err = nil
		v.updateResultsTable()
		if len(v.rows) > 0 {
			v.showResults = true
			v.textarea.Blur()
		}
		return v, nil

	case error:
		v.err = msg
		v.showResults = false
		return v, nil
	}

	// Update textarea or results table
	var cmd tea.Cmd
	if v.showResults {
		v.results, cmd = v.results.Update(msg)
	} else {
		v.textarea, cmd = v.textarea.Update(msg)
	}
	return v, cmd
}

func (v *QueryView) executeQuery() tea.Cmd {
	sql := strings.TrimSpace(v.textarea.Value())
	if sql == "" {
		return nil
	}

	// Add to history
	if len(v.history) == 0 || v.history[len(v.history)-1] != sql {
		v.history = append(v.history, sql)
		if len(v.history) > 100 {
			v.history = v.history[1:]
		}
	}
	v.historyIdx = -1

	return func() tea.Msg {
		// Determine if this is a SELECT/SHOW query
		upperSQL := strings.ToUpper(strings.TrimSpace(sql))
		isQuery := strings.HasPrefix(upperSQL, "SELECT") ||
			strings.HasPrefix(upperSQL, "SHOW") ||
			strings.HasPrefix(upperSQL, "DESCRIBE") ||
			strings.HasPrefix(upperSQL, "EXPLAIN")

		if isQuery {
			result, err := v.conn.Query(sql)
			if err != nil {
				return err
			}
			return queryResult{
				columns: result.Columns,
				rows:    result.Rows,
			}
		}

		affected, err := v.conn.Execute(sql)
		if err != nil {
			return err
		}
		return queryResult{affected: affected}
	}
}

type queryResult struct {
	columns  []string
	rows     [][]string
	affected int64
}

func (v *QueryView) updateResultsTable() {
	if len(v.columns) == 0 {
		return
	}

	// Calculate column widths
	maxWidth := 30
	colWidths := make([]int, len(v.columns))
	for i, col := range v.columns {
		colWidths[i] = min(len(col)+2, maxWidth)
	}
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
			if len(cell) > maxWidth-3 {
				cell = cell[:maxWidth-6] + "..."
			}
			r[j] = cell
		}
		rows[i] = r
	}

	v.results.SetColumns(cols)
	v.results.SetRows(rows)
}

// View renders the view
func (v *QueryView) View() string {
	var b strings.Builder

	// Title
	title := "SQL Query"
	if v.database != "" {
		title = fmt.Sprintf("SQL Query - %s", v.database)
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	// Query input
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FF69B4")).
		Padding(0, 1)
	if !v.showResults {
		inputStyle = inputStyle.BorderForeground(lipgloss.Color("#FF1493"))
	}
	b.WriteString(inputStyle.Render(v.textarea.View()))
	b.WriteString("\n\n")

	// Error or results
	if v.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", v.err)))
		b.WriteString("\n\n")
	} else if len(v.rows) > 0 {
		resultStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF69B4"))
		if v.showResults {
			resultStyle = resultStyle.BorderForeground(lipgloss.Color("#FF1493"))
		}
		b.WriteString(resultStyle.Render(v.results.View()))
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render(fmt.Sprintf("%d row(s) returned", len(v.rows))))
		b.WriteString("\n")
	} else if v.affected > 0 {
		b.WriteString(successStyle.Render(fmt.Sprintf("Query OK, %d row(s) affected", v.affected)))
		b.WriteString("\n\n")
	}

	// Help
	help := "Ctrl+Enter/F5: Execute | Tab: Switch focus | Ctrl+↑↓: History | Esc: Back"
	b.WriteString(helpStyle.Render(help))

	return b.String()
}
