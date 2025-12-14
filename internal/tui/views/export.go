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
	"path/filepath"
	"strings"
	"time"

	"github.com/blubskye/yandere_sql_manager/internal/db"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type exportPhase int

const (
	exportPhaseConfig exportPhase = iota
	exportPhaseExporting
	exportPhaseDone
)

// ExportView handles database export
type ExportView struct {
	conn     *db.Connection
	database string
	width    int
	height   int

	phase    exportPhase

	outputPath   textinput.Model
	focusedInput int

	noData     bool
	noCreate   bool
	addDrop    bool

	progress     progress.Model
	currentTable string
	progressPct  float64

	err      error
	done     bool
	outputFile string
}

// NewExportView creates a new export view
func NewExportView(conn *db.Connection, database string, width, height int) *ExportView {
	// Default output filename
	timestamp := time.Now().Format("20060102_150405")
	defaultOutput := fmt.Sprintf("%s_%s.sql", database, timestamp)

	outputPath := textinput.New()
	outputPath.Placeholder = "Output filename"
	outputPath.SetValue(defaultOutput)
	outputPath.Focus()
	outputPath.Width = 50

	prog := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
	)

	return &ExportView{
		conn:       conn,
		database:   database,
		width:      width,
		height:     height,
		phase:      exportPhaseConfig,
		outputPath: outputPath,
		addDrop:    true,
		progress:   prog,
	}
}

// Init initializes the view
func (v *ExportView) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages
func (v *ExportView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if v.phase == exportPhaseExporting {
				return v, nil
			}
			return v, func() tea.Msg {
				return SwitchViewMsg{View: "databases"}
			}
		case "q", "ctrl+c":
			if v.phase != exportPhaseExporting {
				return v, tea.Quit
			}
		case "enter":
			if v.phase == exportPhaseConfig {
				return v, v.startExport()
			}
			if v.phase == exportPhaseDone {
				return v, func() tea.Msg {
					return SwitchViewMsg{View: "databases"}
				}
			}
		case "tab":
			if v.phase == exportPhaseConfig {
				// Cycle through options
				v.focusedInput = (v.focusedInput + 1) % 4
			}
			return v, nil
		case " ":
			if v.phase == exportPhaseConfig {
				switch v.focusedInput {
				case 1:
					v.noData = !v.noData
				case 2:
					v.noCreate = !v.noCreate
				case 3:
					v.addDrop = !v.addDrop
				}
			}
			return v, nil
		}

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height

	case exportProgressMsg:
		v.currentTable = msg.table
		v.progressPct = msg.percent
		return v, nil

	case exportDoneMsg:
		v.phase = exportPhaseDone
		v.done = true
		v.outputFile = msg.outputFile
		return v, nil

	case error:
		v.err = msg
		v.phase = exportPhaseDone
		return v, nil
	}

	var cmd tea.Cmd
	if v.phase == exportPhaseConfig && v.focusedInput == 0 {
		v.outputPath, cmd = v.outputPath.Update(msg)
	}
	return v, cmd
}

func (v *ExportView) startExport() tea.Cmd {
	v.phase = exportPhaseExporting
	v.progressPct = 0

	outputPath := v.outputPath.Value()
	if !filepath.IsAbs(outputPath) {
		// Use current directory
		outputPath, _ = filepath.Abs(outputPath)
	}

	return func() tea.Msg {
		opts := db.ExportOptions{
			FilePath:     outputPath,
			Database:     v.database,
			NoData:       v.noData,
			NoCreate:     v.noCreate,
			AddDropTable: v.addDrop,
			OnProgress: func(currentTable string, tableNum, totalTables int, rowsExported int64) {
				// Progress updates
			},
		}

		if err := v.conn.ExportSQL(opts); err != nil {
			return err
		}

		return exportDoneMsg{outputFile: outputPath}
	}
}

type exportProgressMsg struct {
	table   string
	percent float64
}

type exportDoneMsg struct {
	outputFile string
}

// View renders the view
func (v *ExportView) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(fmt.Sprintf("Export Database: %s", v.database)))
	b.WriteString("\n\n")

	switch v.phase {
	case exportPhaseConfig:
		// Output path
		pathStyle := blurredStyle
		if v.focusedInput == 0 {
			pathStyle = focusedStyle
		}
		b.WriteString(pathStyle.Render("Output File:"))
		b.WriteString("\n")
		b.WriteString(v.outputPath.View())
		b.WriteString("\n\n")

		// Options
		b.WriteString("Options:\n")

		options := []struct {
			label   string
			checked bool
			idx     int
		}{
			{"Structure only (no data)", v.noData, 1},
			{"Data only (no CREATE)", v.noCreate, 2},
			{"Add DROP TABLE", v.addDrop, 3},
		}

		for _, opt := range options {
			checkbox := "[ ]"
			if opt.checked {
				checkbox = "[x]"
			}
			style := blurredStyle
			if v.focusedInput == opt.idx {
				style = focusedStyle
			}
			b.WriteString(style.Render(fmt.Sprintf("  %s %s", checkbox, opt.label)))
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(helpStyle.Render("Tab: Next option | Space: Toggle | Enter: Export | Esc: Cancel"))

	case exportPhaseExporting:
		b.WriteString("Exporting...\n\n")
		if v.currentTable != "" {
			b.WriteString(fmt.Sprintf("Current table: %s\n", v.currentTable))
		}
		b.WriteString(v.progress.ViewAs(v.progressPct / 100))
		b.WriteString("\n\n")
		b.WriteString("Please wait...")

	case exportPhaseDone:
		if v.err != nil {
			b.WriteString(errorStyle.Render(fmt.Sprintf("Export failed: %v", v.err)))
		} else {
			b.WriteString(successStyle.Render("Export completed successfully!"))
			b.WriteString("\n\n")
			b.WriteString(fmt.Sprintf("Output: %s", v.outputFile))
		}
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("Enter: Continue | Esc: Back"))
	}

	return b.String()
}
