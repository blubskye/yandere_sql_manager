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
	"os"
	"path/filepath"
	"strings"

	"github.com/blubskye/yandere_sql_manager/internal/db"
	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type importPhase int

const (
	phaseSelectFile importPhase = iota
	phaseConfig
	phaseImporting
	phaseDone
)

// ImportView handles SQL file import
type ImportView struct {
	conn       *db.Connection
	database   string
	width      int
	height     int

	phase      importPhase
	filepicker filepicker.Model
	filePath   string

	targetDB   textinput.Model
	renameDB   textinput.Model
	focusedInput int

	progress   progress.Model
	progressPct float64

	err        error
	done       bool
}

// NewImportView creates a new import view
func NewImportView(conn *db.Connection, database string, width, height int) *ImportView {
	fp := filepicker.New()
	fp.AllowedTypes = []string{".sql", ".SQL"}
	fp.CurrentDirectory, _ = os.Getwd()
	fp.Height = height - 10
	fp.Styles.Selected = fp.Styles.Selected.
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#FF69B4"))

	targetDB := textinput.New()
	targetDB.Placeholder = "Database name"
	targetDB.SetValue(database)
	targetDB.Focus()

	renameDB := textinput.New()
	renameDB.Placeholder = "(optional) Rename to..."

	prog := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
	)

	return &ImportView{
		conn:       conn,
		database:   database,
		width:      width,
		height:     height,
		phase:      phaseSelectFile,
		filepicker: fp,
		targetDB:   targetDB,
		renameDB:   renameDB,
		progress:   prog,
	}
}

// Init initializes the view
func (v *ImportView) Init() tea.Cmd {
	return v.filepicker.Init()
}

// Update handles messages
func (v *ImportView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if v.phase == phaseImporting {
				return v, nil // Can't cancel during import
			}
			if v.phase == phaseConfig {
				v.phase = phaseSelectFile
				return v, nil
			}
			return v, func() tea.Msg {
				return SwitchViewMsg{View: "databases"}
			}
		case "q", "ctrl+c":
			if v.phase != phaseImporting {
				return v, tea.Quit
			}
		case "tab":
			if v.phase == phaseConfig {
				v.focusedInput = (v.focusedInput + 1) % 2
				if v.focusedInput == 0 {
					v.targetDB.Focus()
					v.renameDB.Blur()
				} else {
					v.targetDB.Blur()
					v.renameDB.Focus()
				}
			}
			return v, nil
		case "enter":
			if v.phase == phaseConfig {
				return v, v.startImport()
			}
			if v.phase == phaseDone {
				return v, func() tea.Msg {
					return SwitchViewMsg{View: "databases"}
				}
			}
		}

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		v.filepicker.Height = msg.Height - 10

	case importProgressMsg:
		v.progressPct = msg.percent
		return v, nil

	case importDoneMsg:
		v.phase = phaseDone
		v.done = true
		return v, nil

	case error:
		v.err = msg
		v.phase = phaseDone
		return v, nil
	}

	var cmd tea.Cmd

	switch v.phase {
	case phaseSelectFile:
		v.filepicker, cmd = v.filepicker.Update(msg)
		if didSelect, path := v.filepicker.DidSelectFile(msg); didSelect {
			v.filePath = path
			v.phase = phaseConfig
			// Try to infer database name from filename if not set
			if v.targetDB.Value() == "" {
				base := filepath.Base(path)
				ext := filepath.Ext(base)
				v.targetDB.SetValue(base[:len(base)-len(ext)])
			}
		}
		return v, cmd

	case phaseConfig:
		if v.focusedInput == 0 {
			v.targetDB, cmd = v.targetDB.Update(msg)
		} else {
			v.renameDB, cmd = v.renameDB.Update(msg)
		}
		return v, cmd
	}

	return v, nil
}

func (v *ImportView) startImport() tea.Cmd {
	v.phase = phaseImporting
	v.progressPct = 0

	targetDB := v.targetDB.Value()
	renameDB := v.renameDB.Value()

	return func() tea.Msg {
		opts := db.ImportOptions{
			FilePath: v.filePath,
			Database: targetDB,
			CreateDB: true,
			RenameDB: renameDB,
			OnProgress: func(bytesRead, totalBytes int64, statementsExecuted int64) {
				if totalBytes > 0 {
					// We can't easily send messages from here, progress will be approximate
				}
			},
		}

		if err := v.conn.ImportSQL(opts); err != nil {
			return err
		}

		return importDoneMsg{}
	}
}

type importProgressMsg struct {
	percent float64
}

type importDoneMsg struct{}

// View renders the view
func (v *ImportView) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Import SQL File"))
	b.WriteString("\n\n")

	switch v.phase {
	case phaseSelectFile:
		b.WriteString("Select a .sql file to import:\n\n")
		b.WriteString(v.filepicker.View())
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("Enter: Select | Esc: Cancel"))

	case phaseConfig:
		b.WriteString(fmt.Sprintf("File: %s\n\n", v.filePath))

		labels := []string{"Target Database:", "Rename To:"}
		inputs := []*textinput.Model{&v.targetDB, &v.renameDB}

		for i, input := range inputs {
			style := blurredStyle
			if i == v.focusedInput {
				style = focusedStyle
			}
			b.WriteString(style.Render(labels[i]))
			b.WriteString("\n")
			b.WriteString(input.View())
			b.WriteString("\n\n")
		}

		b.WriteString(helpStyle.Render("Tab: Switch field | Enter: Start Import | Esc: Back"))

	case phaseImporting:
		b.WriteString(fmt.Sprintf("Importing: %s\n\n", filepath.Base(v.filePath)))
		b.WriteString(v.progress.ViewAs(v.progressPct / 100))
		b.WriteString("\n\n")
		b.WriteString("Please wait...")

	case phaseDone:
		if v.err != nil {
			b.WriteString(errorStyle.Render(fmt.Sprintf("Import failed: %v", v.err)))
		} else {
			b.WriteString(successStyle.Render("Import completed successfully!"))
		}
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("Enter: Continue | Esc: Back"))
	}

	return b.String()
}
