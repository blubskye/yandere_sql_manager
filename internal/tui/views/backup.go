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

// BackupView shows the backup management interface
type BackupView struct {
	conn    *db.Connection
	list    list.Model
	backups []db.BackupMetadata
	width   int
	height  int
	err     error

	// Sub-views/modes
	mode          backupMode
	createForm    *backupCreateForm
	detailsView   *backupDetailsView
	restoreForm   *backupRestoreForm
	confirmDelete *confirmDeleteView
}

type backupMode int

const (
	backupModeList backupMode = iota
	backupModeCreate
	backupModeDetails
	backupModeRestore
	backupModeConfirmDelete
)

type backupItem struct {
	metadata db.BackupMetadata
}

func (i backupItem) Title() string {
	return i.metadata.ID
}
func (i backupItem) Description() string {
	return fmt.Sprintf("%s | %d DBs | %s",
		i.metadata.Timestamp.Format("2006-01-02 15:04"),
		len(i.metadata.Databases),
		db.FormatSize(i.metadata.TotalSize),
	)
}
func (i backupItem) FilterValue() string { return i.metadata.ID }

// Backup create form
type backupCreateForm struct {
	databases        []string
	selected         map[int]bool
	compressionIndex int
	focused          int // 0 = databases, 1 = compression
	dbCursor         int
	processing       bool
	progress         string
	err              error
}

var compressionOptions = []string{"none", "gzip", "xz", "zstd"}

// Backup details view
type backupDetailsView struct {
	metadata *db.BackupMetadata
}

// Backup restore form
type backupRestoreForm struct {
	metadata   *db.BackupMetadata
	databases  []string
	selected   map[int]bool
	dbCursor   int
	dropExist  bool
	processing bool
	progress   string
	err        error
}

// Confirm delete view
type confirmDeleteView struct {
	metadata *db.BackupMetadata
}

// NewBackupView creates a new backup view
func NewBackupView(conn *db.Connection, width, height int) *BackupView {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#FF69B4")).
		Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("#FFB6C1")).
		Background(lipgloss.Color("#FF69B4"))

	l := list.New([]list.Item{}, delegate, width, height-4)
	l.Title = "Backups"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle

	return &BackupView{
		conn:   conn,
		list:   l,
		width:  width,
		height: height,
		mode:   backupModeList,
	}
}

// Init initializes the view
func (v *BackupView) Init() tea.Cmd {
	return v.loadBackups
}

func (v *BackupView) loadBackups() tea.Msg {
	backups, err := db.ListBackups()
	if err != nil {
		return err
	}
	return backupsLoadedMsg{backups: backups}
}

type backupsLoadedMsg struct {
	backups []db.BackupMetadata
}
type databasesForBackupMsg struct {
	databases []string
}
type backupCreatedMsg struct {
	metadata *db.BackupMetadata
}
type backupRestoredMsg struct{}
type backupDeletedMsg struct{}
type backupProgressMsg struct {
	database string
	dbNum    int
	totalDBs int
}

// Update handles messages
func (v *BackupView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v.mode {
	case backupModeCreate:
		return v.updateCreateForm(msg)
	case backupModeDetails:
		return v.updateDetailsView(msg)
	case backupModeRestore:
		return v.updateRestoreForm(msg)
	case backupModeConfirmDelete:
		return v.updateConfirmDelete(msg)
	}

	return v.updateList(msg)
}

func (v *BackupView) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if item, ok := v.list.SelectedItem().(backupItem); ok {
				v.detailsView = &backupDetailsView{metadata: &item.metadata}
				v.mode = backupModeDetails
				return v, nil
			}
		case "c":
			if !v.list.SettingFilter() {
				return v, v.initCreateForm()
			}
		case "r":
			if !v.list.SettingFilter() {
				if item, ok := v.list.SelectedItem().(backupItem); ok {
					v.initRestoreForm(&item.metadata)
					return v, nil
				}
			}
		case "d":
			if !v.list.SettingFilter() {
				if item, ok := v.list.SelectedItem().(backupItem); ok {
					v.confirmDelete = &confirmDeleteView{metadata: &item.metadata}
					v.mode = backupModeConfirmDelete
					return v, nil
				}
			}
		case "R":
			if !v.list.SettingFilter() {
				return v, v.loadBackups
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
		}

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		v.list.SetSize(msg.Width, msg.Height-4)

	case backupsLoadedMsg:
		v.backups = msg.backups
		items := make([]list.Item, len(msg.backups))
		for i, b := range msg.backups {
			items[i] = backupItem{metadata: b}
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

func (v *BackupView) initCreateForm() tea.Cmd {
	v.createForm = &backupCreateForm{
		selected: make(map[int]bool),
	}
	v.mode = backupModeCreate

	return func() tea.Msg {
		databases, err := v.conn.ListDatabases()
		if err != nil {
			return err
		}
		var names []string
		for _, d := range databases {
			names = append(names, d.Name)
		}
		return databasesForBackupMsg{databases: names}
	}
}

func (v *BackupView) updateCreateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	form := v.createForm

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if form.processing {
			return v, nil
		}

		switch msg.String() {
		case "esc":
			v.mode = backupModeList
			v.createForm = nil
			return v, nil

		case "tab":
			form.focused = (form.focused + 1) % 2
			return v, nil

		case "up", "k":
			if form.focused == 0 && len(form.databases) > 0 {
				form.dbCursor--
				if form.dbCursor < 0 {
					form.dbCursor = len(form.databases) - 1
				}
			} else if form.focused == 1 {
				form.compressionIndex--
				if form.compressionIndex < 0 {
					form.compressionIndex = len(compressionOptions) - 1
				}
			}
			return v, nil

		case "down", "j":
			if form.focused == 0 && len(form.databases) > 0 {
				form.dbCursor++
				if form.dbCursor >= len(form.databases) {
					form.dbCursor = 0
				}
			} else if form.focused == 1 {
				form.compressionIndex++
				if form.compressionIndex >= len(compressionOptions) {
					form.compressionIndex = 0
				}
			}
			return v, nil

		case " ":
			if form.focused == 0 && len(form.databases) > 0 {
				form.selected[form.dbCursor] = !form.selected[form.dbCursor]
			}
			return v, nil

		case "a":
			// Select all / Deselect all
			if form.focused == 0 {
				allSelected := len(form.selected) == len(form.databases)
				form.selected = make(map[int]bool)
				if !allSelected {
					for i := range form.databases {
						form.selected[i] = true
					}
				}
			}
			return v, nil

		case "enter":
			form.processing = true
			return v, v.createBackup()
		}

	case databasesForBackupMsg:
		form.databases = msg.databases
		return v, nil

	case backupProgressMsg:
		form.progress = fmt.Sprintf("Backing up %s (%d/%d)...", msg.database, msg.dbNum, msg.totalDBs)
		return v, nil

	case backupCreatedMsg:
		v.mode = backupModeList
		v.createForm = nil
		return v, v.loadBackups

	case error:
		form.err = msg
		form.processing = false
		return v, nil
	}

	return v, nil
}

func (v *BackupView) createBackup() tea.Cmd {
	form := v.createForm

	// Get selected databases
	var databases []string
	for i, selected := range form.selected {
		if selected && i < len(form.databases) {
			databases = append(databases, form.databases[i])
		}
	}

	compression := db.CompressionNone
	switch compressionOptions[form.compressionIndex] {
	case "gzip":
		compression = db.CompressionGzip
	case "xz":
		compression = db.CompressionXZ
	case "zstd":
		compression = db.CompressionZstd
	}

	return func() tea.Msg {
		opts := db.BackupOptions{
			Databases:   databases,
			Compression: compression,
		}

		metadata, err := v.conn.CreateBackup(opts)
		if err != nil {
			return err
		}
		return backupCreatedMsg{metadata: metadata}
	}
}

func (v *BackupView) updateDetailsView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace", "q":
			v.mode = backupModeList
			v.detailsView = nil
			return v, nil
		case "r":
			v.initRestoreForm(v.detailsView.metadata)
			return v, nil
		case "d":
			v.confirmDelete = &confirmDeleteView{metadata: v.detailsView.metadata}
			v.mode = backupModeConfirmDelete
			return v, nil
		}
	}

	return v, nil
}

func (v *BackupView) initRestoreForm(metadata *db.BackupMetadata) {
	v.restoreForm = &backupRestoreForm{
		metadata: metadata,
		selected: make(map[int]bool),
	}
	// Pre-select all databases
	for i := range metadata.Databases {
		v.restoreForm.selected[i] = true
	}
	v.restoreForm.databases = metadata.Databases
	v.mode = backupModeRestore
}

func (v *BackupView) updateRestoreForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	form := v.restoreForm

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if form.processing {
			return v, nil
		}

		switch msg.String() {
		case "esc":
			if v.detailsView != nil {
				v.mode = backupModeDetails
			} else {
				v.mode = backupModeList
			}
			v.restoreForm = nil
			return v, nil

		case "up", "k":
			form.dbCursor--
			if form.dbCursor < 0 {
				form.dbCursor = len(form.databases) - 1
			}
			return v, nil

		case "down", "j":
			form.dbCursor++
			if form.dbCursor >= len(form.databases) {
				form.dbCursor = 0
			}
			return v, nil

		case " ":
			if len(form.databases) > 0 {
				form.selected[form.dbCursor] = !form.selected[form.dbCursor]
			}
			return v, nil

		case "d":
			form.dropExist = !form.dropExist
			return v, nil

		case "enter":
			form.processing = true
			return v, v.restoreBackup()
		}

	case backupRestoredMsg:
		v.mode = backupModeList
		v.restoreForm = nil
		v.detailsView = nil
		return v, v.loadBackups

	case error:
		form.err = msg
		form.processing = false
		return v, nil
	}

	return v, nil
}

func (v *BackupView) restoreBackup() tea.Cmd {
	form := v.restoreForm

	// Get selected databases
	var databases []string
	for i, selected := range form.selected {
		if selected && i < len(form.databases) {
			databases = append(databases, form.databases[i])
		}
	}

	return func() tea.Msg {
		opts := db.RestoreOptions{
			BackupID:           form.metadata.ID,
			Databases:          databases,
			DropExisting:       form.dropExist,
			CreateIfNotExists:  true,
			DisableForeignKeys: true,
		}

		if err := v.conn.RestoreBackup(opts); err != nil {
			return err
		}
		return backupRestoredMsg{}
	}
}

func (v *BackupView) updateConfirmDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "n":
			if v.detailsView != nil {
				v.mode = backupModeDetails
			} else {
				v.mode = backupModeList
			}
			v.confirmDelete = nil
			return v, nil
		case "y":
			backupID := v.confirmDelete.metadata.ID
			v.confirmDelete = nil
			return v, v.deleteBackup(backupID)
		}

	case backupDeletedMsg:
		v.mode = backupModeList
		v.detailsView = nil
		return v, v.loadBackups

	case error:
		v.err = msg
		v.mode = backupModeList
		return v, nil
	}

	return v, nil
}

func (v *BackupView) deleteBackup(id string) tea.Cmd {
	return func() tea.Msg {
		if err := db.DeleteBackup(id); err != nil {
			return err
		}
		return backupDeletedMsg{}
	}
}

// View renders the view
func (v *BackupView) View() string {
	switch v.mode {
	case backupModeCreate:
		return v.viewCreateForm()
	case backupModeDetails:
		return v.viewDetails()
	case backupModeRestore:
		return v.viewRestoreForm()
	case backupModeConfirmDelete:
		return v.viewConfirmDelete()
	}

	return v.viewList()
}

func (v *BackupView) viewList() string {
	var b strings.Builder

	if v.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", v.err)))
		b.WriteString("\n\n")
	}

	b.WriteString(v.list.View())
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Enter: Details | c: Create | r: Restore | d: Delete | R: Refresh | Esc: Back | q: Quit"))

	return b.String()
}

func (v *BackupView) viewCreateForm() string {
	var b strings.Builder
	form := v.createForm

	b.WriteString(titleStyle.Render("Create Backup"))
	b.WriteString("\n\n")

	// Databases
	if form.focused == 0 {
		b.WriteString(focusedStyle.Render("Databases:"))
	} else {
		b.WriteString(blurredStyle.Render("Databases:"))
	}
	b.WriteString(" (Space to toggle, 'a' to select all)\n")

	if len(form.databases) == 0 {
		b.WriteString(mutedStyle.Render("  Loading..."))
		b.WriteString("\n")
	} else {
		maxShow := 8
		start := 0
		if form.dbCursor >= maxShow {
			start = form.dbCursor - maxShow + 1
		}

		for i := start; i < len(form.databases) && i < start+maxShow; i++ {
			checkbox := "[ ]"
			if form.selected[i] {
				checkbox = "[x]"
			}

			if form.focused == 0 && i == form.dbCursor {
				b.WriteString(focusedStyle.Render(fmt.Sprintf("  → %s %s", checkbox, form.databases[i])))
			} else {
				b.WriteString(fmt.Sprintf("    %s %s", checkbox, form.databases[i]))
			}
			b.WriteString("\n")
		}

		if len(form.databases) > maxShow {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("    ... and %d more", len(form.databases)-maxShow)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	// Compression
	if form.focused == 1 {
		b.WriteString(focusedStyle.Render("Compression:"))
	} else {
		b.WriteString(blurredStyle.Render("Compression:"))
	}
	b.WriteString("\n")

	for i, opt := range compressionOptions {
		if form.focused == 1 && i == form.compressionIndex {
			b.WriteString(focusedStyle.Render(fmt.Sprintf("  → [%s]", opt)))
		} else if i == form.compressionIndex {
			b.WriteString(fmt.Sprintf("    [%s]", opt))
		} else {
			b.WriteString(fmt.Sprintf("     %s", opt))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	if form.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", form.err)))
		b.WriteString("\n\n")
	}

	if form.processing {
		progress := form.progress
		if progress == "" {
			progress = "Creating backup..."
		}
		b.WriteString(progress)
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("Tab: Switch | ↑↓: Navigate | Space: Toggle | a: All | Enter: Create | Esc: Cancel"))

	return b.String()
}

func (v *BackupView) viewDetails() string {
	var b strings.Builder
	m := v.detailsView.metadata

	b.WriteString(titleStyle.Render(fmt.Sprintf("Backup: %s", m.ID)))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("  Timestamp:      %s\n", m.Timestamp.Format("2006-01-02 15:04:05")))
	b.WriteString(fmt.Sprintf("  Server Type:    %s\n", m.ServerType))
	b.WriteString(fmt.Sprintf("  Server Version: %s\n", m.ServerVersion))
	b.WriteString(fmt.Sprintf("  Total Size:     %s\n", db.FormatSize(m.TotalSize)))
	if m.Compression != "" {
		b.WriteString(fmt.Sprintf("  Compression:    %s\n", m.Compression))
	}
	if m.Description != "" {
		b.WriteString(fmt.Sprintf("  Description:    %s\n", m.Description))
	}

	b.WriteString("\n")
	b.WriteString("Databases:\n")
	for _, f := range m.Files {
		b.WriteString(fmt.Sprintf("  - %s (%d tables, %d rows, %s)\n",
			f.Database, f.Tables, f.Rows, db.FormatSize(f.Size)))
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("r: Restore | d: Delete | Esc: Back"))

	return b.String()
}

func (v *BackupView) viewRestoreForm() string {
	var b strings.Builder
	form := v.restoreForm

	b.WriteString(titleStyle.Render(fmt.Sprintf("Restore Backup: %s", form.metadata.ID)))
	b.WriteString("\n\n")

	b.WriteString("Select databases to restore:\n")
	for i, dbName := range form.databases {
		checkbox := "[ ]"
		if form.selected[i] {
			checkbox = "[x]"
		}

		if i == form.dbCursor {
			b.WriteString(focusedStyle.Render(fmt.Sprintf("  → %s %s", checkbox, dbName)))
		} else {
			b.WriteString(fmt.Sprintf("    %s %s", checkbox, dbName))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	dropCheck := "[ ]"
	if form.dropExist {
		dropCheck = "[x]"
	}
	b.WriteString(fmt.Sprintf("Options: %s Drop existing databases (press 'd' to toggle)\n", dropCheck))

	b.WriteString("\n")

	if form.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", form.err)))
		b.WriteString("\n\n")
	}

	if form.processing {
		progress := form.progress
		if progress == "" {
			progress = "Restoring backup..."
		}
		b.WriteString(progress)
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("↑↓: Navigate | Space: Toggle | d: Drop existing | Enter: Restore | Esc: Cancel"))

	return b.String()
}

func (v *BackupView) viewConfirmDelete() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Confirm Delete Backup"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Are you sure you want to delete backup '%s'?\n\n", v.confirmDelete.metadata.ID))
	b.WriteString(fmt.Sprintf("  Databases: %d\n", len(v.confirmDelete.metadata.Databases)))
	b.WriteString(fmt.Sprintf("  Size:      %s\n", db.FormatSize(v.confirmDelete.metadata.TotalSize)))
	b.WriteString("\n")
	b.WriteString(errorStyle.Render("This action cannot be undone!"))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("y: Yes, delete | n/Esc: Cancel"))

	return b.String()
}
