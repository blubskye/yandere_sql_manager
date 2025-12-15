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
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UsersView shows the list of database users and allows management
type UsersView struct {
	conn   *db.Connection
	list   list.Model
	users  []db.User
	width  int
	height int
	err    error

	// Sub-views/modes
	mode        usersMode
	createForm  *userCreateForm
	grantForm   *userGrantForm
	grantsView  *userGrantsView
	confirmDrop *confirmDropView
}

type usersMode int

const (
	usersModeList usersMode = iota
	usersModeCreate
	usersModeGrants
	usersModeGrant
	usersModeRevoke
	usersModeConfirmDrop
)

type userItem struct {
	user db.User
}

func (i userItem) Title() string {
	if i.user.Host != "" {
		return fmt.Sprintf("%s@%s", i.user.Username, i.user.Host)
	}
	return i.user.Username
}
func (i userItem) Description() string { return "" }
func (i userItem) FilterValue() string { return i.user.Username }

// User create form
type userCreateForm struct {
	inputs     []textinput.Model
	focused    int
	hostIndex  int // For MariaDB host selection
	isMariaDB  bool
	err        error
	processing bool
}

const (
	createInputUsername = iota
	createInputPassword
	createInputConfirm
)

var defaultHosts = []string{"localhost", "%", "127.0.0.1"}

// User grants view
type userGrantsView struct {
	user   db.User
	grants []db.Grant
	err    error
}

// User grant form
type userGrantForm struct {
	user        db.User
	databases   []string
	dbIndex     int
	privIndex   int
	privileges  []string
	selected    map[int]bool
	isRevoke    bool
	focused     int // 0 = database, 1 = privileges
	err         error
	processing  bool
}

// Confirm drop view
type confirmDropView struct {
	user      db.User
	confirmed bool
}

// NewUsersView creates a new users view
func NewUsersView(conn *db.Connection, width, height int) *UsersView {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#FF69B4")).
		Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("#FFB6C1")).
		Background(lipgloss.Color("#FF69B4"))

	l := list.New([]list.Item{}, delegate, width, height-4)
	l.Title = "Database Users"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle

	return &UsersView{
		conn:   conn,
		list:   l,
		width:  width,
		height: height,
		mode:   usersModeList,
	}
}

// Init initializes the view
func (v *UsersView) Init() tea.Cmd {
	return v.loadUsers
}

func (v *UsersView) loadUsers() tea.Msg {
	users, err := v.conn.ListUsers()
	if err != nil {
		return err
	}
	return usersLoadedMsg{users: users}
}

type usersLoadedMsg struct {
	users []db.User
}

type userCreatedMsg struct{}
type userDroppedMsg struct{}
type grantsLoadedMsg struct {
	grants []db.Grant
}
type privilegesChangedMsg struct{}
type databasesLoadedMsg struct {
	databases []string
}

// Update handles messages
func (v *UsersView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v.mode {
	case usersModeCreate:
		return v.updateCreateForm(msg)
	case usersModeGrants:
		return v.updateGrantsView(msg)
	case usersModeGrant, usersModeRevoke:
		return v.updateGrantForm(msg)
	case usersModeConfirmDrop:
		return v.updateConfirmDrop(msg)
	}

	return v.updateList(msg)
}

func (v *UsersView) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if item, ok := v.list.SelectedItem().(userItem); ok {
				return v, v.loadGrants(item.user)
			}
		case "c":
			if !v.list.SettingFilter() {
				v.initCreateForm()
				v.mode = usersModeCreate
				return v, textinput.Blink
			}
		case "d":
			if !v.list.SettingFilter() {
				if item, ok := v.list.SelectedItem().(userItem); ok {
					v.confirmDrop = &confirmDropView{user: item.user}
					v.mode = usersModeConfirmDrop
					return v, nil
				}
			}
		case "g":
			if !v.list.SettingFilter() {
				if item, ok := v.list.SelectedItem().(userItem); ok {
					return v, v.initGrantForm(item.user, false)
				}
			}
		case "r":
			if !v.list.SettingFilter() {
				if item, ok := v.list.SelectedItem().(userItem); ok {
					return v, v.initGrantForm(item.user, true)
				}
			}
		case "R":
			if !v.list.SettingFilter() {
				return v, v.loadUsers
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

	case usersLoadedMsg:
		v.users = msg.users
		items := make([]list.Item, len(msg.users))
		for i, u := range msg.users {
			items[i] = userItem{user: u}
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

func (v *UsersView) initCreateForm() {
	isMariaDB := v.conn.Config.Type == db.DatabaseTypeMariaDB

	form := &userCreateForm{
		inputs:    make([]textinput.Model, 3),
		isMariaDB: isMariaDB,
	}

	// Username
	form.inputs[createInputUsername] = textinput.New()
	form.inputs[createInputUsername].Placeholder = "username"
	form.inputs[createInputUsername].Focus()
	form.inputs[createInputUsername].PromptStyle = focusedStyle
	form.inputs[createInputUsername].TextStyle = focusedStyle

	// Password
	form.inputs[createInputPassword] = textinput.New()
	form.inputs[createInputPassword].Placeholder = "password"
	form.inputs[createInputPassword].EchoMode = textinput.EchoPassword
	form.inputs[createInputPassword].EchoCharacter = '•'

	// Confirm password
	form.inputs[createInputConfirm] = textinput.New()
	form.inputs[createInputConfirm].Placeholder = "confirm password"
	form.inputs[createInputConfirm].EchoMode = textinput.EchoPassword
	form.inputs[createInputConfirm].EchoCharacter = '•'

	v.createForm = form
}

func (v *UsersView) updateCreateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	form := v.createForm

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			v.mode = usersModeList
			v.createForm = nil
			return v, nil

		case "tab", "down":
			form.nextInput()
			return v, nil

		case "shift+tab", "up":
			form.prevInput()
			return v, nil

		case "left", "right":
			// Host selection for MariaDB
			if form.isMariaDB && form.focused == 3 {
				if msg.String() == "left" {
					form.hostIndex--
					if form.hostIndex < 0 {
						form.hostIndex = len(defaultHosts) - 1
					}
				} else {
					form.hostIndex++
					if form.hostIndex >= len(defaultHosts) {
						form.hostIndex = 0
					}
				}
				return v, nil
			}

		case "enter":
			// Validate and create
			username := form.inputs[createInputUsername].Value()
			password := form.inputs[createInputPassword].Value()
			confirm := form.inputs[createInputConfirm].Value()

			if username == "" {
				form.err = fmt.Errorf("username is required")
				return v, nil
			}
			if password == "" {
				form.err = fmt.Errorf("password is required")
				return v, nil
			}
			if password != confirm {
				form.err = fmt.Errorf("passwords do not match")
				return v, nil
			}

			host := "localhost"
			if form.isMariaDB {
				host = defaultHosts[form.hostIndex]
			}

			form.processing = true
			return v, v.createUser(username, host, password)
		}

	case userCreatedMsg:
		v.mode = usersModeList
		v.createForm = nil
		return v, v.loadUsers

	case error:
		form.err = msg
		form.processing = false
		return v, nil
	}

	// Update text inputs
	cmds := make([]tea.Cmd, len(form.inputs))
	for i := range form.inputs {
		form.inputs[i], cmds[i] = form.inputs[i].Update(msg)
	}
	return v, tea.Batch(cmds...)
}

func (f *userCreateForm) nextInput() {
	// Blur current
	if f.focused < len(f.inputs) {
		f.inputs[f.focused].Blur()
		f.inputs[f.focused].PromptStyle = blurredStyle
		f.inputs[f.focused].TextStyle = blurredStyle
	}

	maxFocus := len(f.inputs)
	if f.isMariaDB {
		maxFocus++ // Add host selector
	}

	f.focused++
	if f.focused >= maxFocus {
		f.focused = 0
	}

	// Focus new
	if f.focused < len(f.inputs) {
		f.inputs[f.focused].Focus()
		f.inputs[f.focused].PromptStyle = focusedStyle
		f.inputs[f.focused].TextStyle = focusedStyle
	}
}

func (f *userCreateForm) prevInput() {
	// Blur current
	if f.focused < len(f.inputs) {
		f.inputs[f.focused].Blur()
		f.inputs[f.focused].PromptStyle = blurredStyle
		f.inputs[f.focused].TextStyle = blurredStyle
	}

	maxFocus := len(f.inputs)
	if f.isMariaDB {
		maxFocus++
	}

	f.focused--
	if f.focused < 0 {
		f.focused = maxFocus - 1
	}

	// Focus new
	if f.focused < len(f.inputs) {
		f.inputs[f.focused].Focus()
		f.inputs[f.focused].PromptStyle = focusedStyle
		f.inputs[f.focused].TextStyle = focusedStyle
	}
}

func (v *UsersView) createUser(username, host, password string) tea.Cmd {
	return func() tea.Msg {
		if err := v.conn.CreateUser(username, host, password); err != nil {
			return err
		}
		return userCreatedMsg{}
	}
}

func (v *UsersView) loadGrants(user db.User) tea.Cmd {
	return func() tea.Msg {
		grants, err := v.conn.GetUserGrants(user.Username, user.Host)
		if err != nil {
			return err
		}
		return grantsLoadedMsg{grants: grants}
	}
}

func (v *UsersView) updateGrantsView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace", "q":
			v.mode = usersModeList
			v.grantsView = nil
			return v, nil
		case "g":
			return v, v.initGrantForm(v.grantsView.user, false)
		case "r":
			return v, v.initGrantForm(v.grantsView.user, true)
		}

	case grantsLoadedMsg:
		if item, ok := v.list.SelectedItem().(userItem); ok {
			v.grantsView = &userGrantsView{
				user:   item.user,
				grants: msg.grants,
			}
			v.mode = usersModeGrants
		}
		return v, nil

	case error:
		if v.grantsView != nil {
			v.grantsView.err = msg
		}
		return v, nil
	}

	return v, nil
}

func (v *UsersView) initGrantForm(user db.User, isRevoke bool) tea.Cmd {
	v.grantForm = &userGrantForm{
		user:       user,
		privileges: db.CommonPrivileges(),
		selected:   make(map[int]bool),
		isRevoke:   isRevoke,
	}

	if isRevoke {
		v.mode = usersModeRevoke
	} else {
		v.mode = usersModeGrant
	}

	// Load databases
	return func() tea.Msg {
		databases, err := v.conn.ListDatabases()
		if err != nil {
			return err
		}
		names := make([]string, len(databases)+1)
		names[0] = "*" // All databases
		for i, d := range databases {
			names[i+1] = d.Name
		}
		return databasesLoadedMsg{databases: names}
	}
}

func (v *UsersView) updateGrantForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	form := v.grantForm

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if v.grantsView != nil {
				v.mode = usersModeGrants
			} else {
				v.mode = usersModeList
			}
			v.grantForm = nil
			return v, nil

		case "tab":
			form.focused = (form.focused + 1) % 2
			return v, nil

		case "up", "k":
			if form.focused == 0 {
				form.dbIndex--
				if form.dbIndex < 0 {
					form.dbIndex = len(form.databases) - 1
				}
			} else {
				form.privIndex--
				if form.privIndex < 0 {
					form.privIndex = len(form.privileges) - 1
				}
			}
			return v, nil

		case "down", "j":
			if form.focused == 0 {
				form.dbIndex++
				if form.dbIndex >= len(form.databases) {
					form.dbIndex = 0
				}
			} else {
				form.privIndex++
				if form.privIndex >= len(form.privileges) {
					form.privIndex = 0
				}
			}
			return v, nil

		case " ":
			// Toggle privilege selection
			if form.focused == 1 {
				form.selected[form.privIndex] = !form.selected[form.privIndex]
			}
			return v, nil

		case "enter":
			// Execute grant/revoke
			database := ""
			if form.dbIndex > 0 {
				database = form.databases[form.dbIndex]
			}

			var privs []string
			for i, selected := range form.selected {
				if selected {
					privs = append(privs, form.privileges[i])
				}
			}
			if len(privs) == 0 {
				privs = []string{"ALL PRIVILEGES"}
			}

			form.processing = true
			if form.isRevoke {
				return v, v.revokePrivileges(form.user, privs, database)
			}
			return v, v.grantPrivileges(form.user, privs, database)
		}

	case databasesLoadedMsg:
		form.databases = msg.databases
		return v, nil

	case privilegesChangedMsg:
		v.grantForm = nil
		if v.grantsView != nil {
			v.mode = usersModeGrants
			return v, v.loadGrants(v.grantsView.user)
		}
		v.mode = usersModeList
		return v, v.loadUsers

	case error:
		form.err = msg
		form.processing = false
		return v, nil
	}

	return v, nil
}

func (v *UsersView) grantPrivileges(user db.User, privs []string, database string) tea.Cmd {
	return func() tea.Msg {
		if err := v.conn.GrantPrivileges(user.Username, user.Host, privs, database, ""); err != nil {
			return err
		}
		return privilegesChangedMsg{}
	}
}

func (v *UsersView) revokePrivileges(user db.User, privs []string, database string) tea.Cmd {
	return func() tea.Msg {
		if err := v.conn.RevokePrivileges(user.Username, user.Host, privs, database, ""); err != nil {
			return err
		}
		return privilegesChangedMsg{}
	}
}

func (v *UsersView) updateConfirmDrop(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "n":
			v.mode = usersModeList
			v.confirmDrop = nil
			return v, nil
		case "y":
			user := v.confirmDrop.user
			v.confirmDrop = nil
			return v, v.dropUser(user)
		}

	case userDroppedMsg:
		v.mode = usersModeList
		return v, v.loadUsers

	case error:
		v.err = msg
		v.mode = usersModeList
		return v, nil
	}

	return v, nil
}

func (v *UsersView) dropUser(user db.User) tea.Cmd {
	return func() tea.Msg {
		if err := v.conn.DropUser(user.Username, user.Host); err != nil {
			return err
		}
		return userDroppedMsg{}
	}
}

// View renders the view
func (v *UsersView) View() string {
	switch v.mode {
	case usersModeCreate:
		return v.viewCreateForm()
	case usersModeGrants:
		return v.viewGrants()
	case usersModeGrant, usersModeRevoke:
		return v.viewGrantForm()
	case usersModeConfirmDrop:
		return v.viewConfirmDrop()
	}

	return v.viewList()
}

func (v *UsersView) viewList() string {
	var b strings.Builder

	if v.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", v.err)))
		b.WriteString("\n\n")
	}

	b.WriteString(v.list.View())
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Enter: Show grants | c: Create | d: Drop | g: Grant | r: Revoke | R: Refresh | Esc: Back | q: Quit"))

	return b.String()
}

func (v *UsersView) viewCreateForm() string {
	var b strings.Builder
	form := v.createForm

	b.WriteString(titleStyle.Render("Create User"))
	b.WriteString("\n\n")

	// Username
	if form.focused == createInputUsername {
		b.WriteString(focusedStyle.Render("Username:"))
	} else {
		b.WriteString(blurredStyle.Render("Username:"))
	}
	b.WriteString("\n")
	b.WriteString(form.inputs[createInputUsername].View())
	b.WriteString("\n\n")

	// Password
	if form.focused == createInputPassword {
		b.WriteString(focusedStyle.Render("Password:"))
	} else {
		b.WriteString(blurredStyle.Render("Password:"))
	}
	b.WriteString("\n")
	b.WriteString(form.inputs[createInputPassword].View())
	b.WriteString("\n\n")

	// Confirm
	if form.focused == createInputConfirm {
		b.WriteString(focusedStyle.Render("Confirm Password:"))
	} else {
		b.WriteString(blurredStyle.Render("Confirm Password:"))
	}
	b.WriteString("\n")
	b.WriteString(form.inputs[createInputConfirm].View())
	b.WriteString("\n\n")

	// Host (MariaDB only)
	if form.isMariaDB {
		if form.focused == 3 {
			b.WriteString(focusedStyle.Render("Host:"))
		} else {
			b.WriteString(blurredStyle.Render("Host:"))
		}
		b.WriteString("\n")
		hostDisplay := fmt.Sprintf("[ %s ]", defaultHosts[form.hostIndex])
		if form.focused == 3 {
			b.WriteString(focusedStyle.Render(hostDisplay))
			b.WriteString(mutedStyle.Render("  ←/→ to change"))
		} else {
			b.WriteString(blurredStyle.Render(hostDisplay))
		}
		b.WriteString("\n\n")
	}

	if form.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", form.err)))
		b.WriteString("\n\n")
	}

	if form.processing {
		b.WriteString("Creating user...\n\n")
	}

	b.WriteString(helpStyle.Render("Enter: Create | Tab: Next | Esc: Cancel"))

	return b.String()
}

func (v *UsersView) viewGrants() string {
	var b strings.Builder
	gv := v.grantsView

	userDisplay := gv.user.Username
	if gv.user.Host != "" {
		userDisplay = fmt.Sprintf("%s@%s", gv.user.Username, gv.user.Host)
	}

	b.WriteString(titleStyle.Render(fmt.Sprintf("Grants for %s", userDisplay)))
	b.WriteString("\n\n")

	if gv.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", gv.err)))
		b.WriteString("\n\n")
	}

	if len(gv.grants) == 0 {
		b.WriteString(mutedStyle.Render("No grants found."))
		b.WriteString("\n")
	} else {
		for _, g := range gv.grants {
			if g.GrantText != "" {
				// MariaDB raw grant
				b.WriteString("  ")
				b.WriteString(g.GrantText)
				b.WriteString("\n")
			} else {
				// PostgreSQL structured
				b.WriteString(fmt.Sprintf("  %s on %s.%s\n", g.Privilege, g.Database, g.Table))
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("g: Grant | r: Revoke | Esc: Back"))

	return b.String()
}

func (v *UsersView) viewGrantForm() string {
	var b strings.Builder
	form := v.grantForm

	action := "Grant"
	if form.isRevoke {
		action = "Revoke"
	}

	userDisplay := form.user.Username
	if form.user.Host != "" {
		userDisplay = fmt.Sprintf("%s@%s", form.user.Username, form.user.Host)
	}

	b.WriteString(titleStyle.Render(fmt.Sprintf("%s Privileges - %s", action, userDisplay)))
	b.WriteString("\n\n")

	// Database selector
	if form.focused == 0 {
		b.WriteString(focusedStyle.Render("Database:"))
	} else {
		b.WriteString(blurredStyle.Render("Database:"))
	}
	b.WriteString("\n")

	if len(form.databases) > 0 {
		dbDisplay := form.databases[form.dbIndex]
		if dbDisplay == "*" {
			dbDisplay = "* (all databases)"
		}
		if form.focused == 0 {
			b.WriteString(focusedStyle.Render(fmt.Sprintf("  → %s", dbDisplay)))
		} else {
			b.WriteString(fmt.Sprintf("  %s", dbDisplay))
		}
	} else {
		b.WriteString(mutedStyle.Render("  Loading..."))
	}
	b.WriteString("\n\n")

	// Privileges selector
	if form.focused == 1 {
		b.WriteString(focusedStyle.Render("Privileges:"))
	} else {
		b.WriteString(blurredStyle.Render("Privileges:"))
	}
	b.WriteString("\n")

	// Show privileges with selection
	maxShow := 8
	start := 0
	if form.privIndex >= maxShow {
		start = form.privIndex - maxShow + 1
	}

	for i := start; i < len(form.privileges) && i < start+maxShow; i++ {
		priv := form.privileges[i]
		checkbox := "[ ]"
		if form.selected[i] {
			checkbox = "[x]"
		}

		if form.focused == 1 && i == form.privIndex {
			b.WriteString(focusedStyle.Render(fmt.Sprintf("  → %s %s", checkbox, priv)))
		} else {
			b.WriteString(fmt.Sprintf("    %s %s", checkbox, priv))
		}
		b.WriteString("\n")
	}

	if len(form.privileges) > maxShow {
		b.WriteString(mutedStyle.Render(fmt.Sprintf("    ... and %d more", len(form.privileges)-maxShow)))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	if form.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", form.err)))
		b.WriteString("\n\n")
	}

	if form.processing {
		b.WriteString(fmt.Sprintf("%sing privileges...\n\n", action))
	}

	b.WriteString(helpStyle.Render("Tab: Switch | ↑↓: Navigate | Space: Toggle | Enter: Execute | Esc: Cancel"))

	return b.String()
}

func (v *UsersView) viewConfirmDrop() string {
	var b strings.Builder

	userDisplay := v.confirmDrop.user.Username
	if v.confirmDrop.user.Host != "" {
		userDisplay = fmt.Sprintf("%s@%s", v.confirmDrop.user.Username, v.confirmDrop.user.Host)
	}

	b.WriteString(titleStyle.Render("Confirm Drop User"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Are you sure you want to drop user '%s'?\n\n", userDisplay))
	b.WriteString(errorStyle.Render("This action cannot be undone!"))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("y: Yes, drop user | n/Esc: Cancel"))

	return b.String()
}
