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
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// SetupWizardView provides a step-by-step wizard for setting up app databases
type SetupWizardView struct {
	conn      *db.Connection
	width     int
	height    int
	err       error
	success   bool

	step      wizardStep
	templates []db.AppTemplate

	// User selections
	templateIndex int
	dbName        textinput.Model
	username      textinput.Model
	password      textinput.Model
	confirmPass   textinput.Model
	hostIndex     int
	charsetIndex  int
	collationIndex int

	// Available options
	charsets   []string
	collations []string

	// Processing state
	processing bool
}

type wizardStep int

const (
	wizardStepTemplate wizardStep = iota
	wizardStepDBName
	wizardStepUsername
	wizardStepPassword
	wizardStepConfirm
	wizardStepAdvanced
	wizardStepReview
	wizardStepComplete
)

var defaultHosts2 = []string{"localhost", "%", "127.0.0.1"}

// NewSetupWizardView creates a new setup wizard view
func NewSetupWizardView(conn *db.Connection, width, height int) *SetupWizardView {
	v := &SetupWizardView{
		conn:      conn,
		width:     width,
		height:    height,
		templates: db.DefaultTemplates(),
		charsets:  db.CommonCharsets(),
	}

	// Initialize text inputs
	v.dbName = textinput.New()
	v.dbName.Placeholder = "myapp_db"
	v.dbName.Focus()
	v.dbName.PromptStyle = focusedStyle
	v.dbName.TextStyle = focusedStyle

	v.username = textinput.New()
	v.username.Placeholder = "myapp_user"

	v.password = textinput.New()
	v.password.Placeholder = "password"
	v.password.EchoMode = textinput.EchoPassword
	v.password.EchoCharacter = '•'

	v.confirmPass = textinput.New()
	v.confirmPass.Placeholder = "confirm password"
	v.confirmPass.EchoMode = textinput.EchoPassword
	v.confirmPass.EchoCharacter = '•'

	// Set default collations for default charset
	v.collations = db.CommonCollationsForCharset(v.charsets[0])

	return v
}

// Init initializes the view
func (v *SetupWizardView) Init() tea.Cmd {
	return textinput.Blink
}

type setupCompleteMsg struct{}

// Update handles messages
func (v *SetupWizardView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if v.processing {
			return v, nil
		}

		switch msg.String() {
		case "esc":
			if v.step == wizardStepTemplate || v.success {
				return v, func() tea.Msg {
					return SwitchViewMsg{View: "databases"}
				}
			}
			v.prevStep()
			return v, nil

		case "enter":
			return v.handleEnter()

		case "up", "k":
			return v.handleUp()

		case "down", "j":
			return v.handleDown()

		case "left":
			return v.handleLeft()

		case "right":
			return v.handleRight()

		case "tab":
			if v.step == wizardStepAdvanced {
				// Cycle through advanced options
				return v, nil
			}
		}

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height

	case setupCompleteMsg:
		v.processing = false
		v.success = true
		v.step = wizardStepComplete
		return v, nil

	case error:
		v.err = msg
		v.processing = false
		return v, nil
	}

	// Update text inputs based on current step
	var cmd tea.Cmd
	switch v.step {
	case wizardStepDBName:
		v.dbName, cmd = v.dbName.Update(msg)
	case wizardStepUsername:
		v.username, cmd = v.username.Update(msg)
	case wizardStepPassword:
		v.password, cmd = v.password.Update(msg)
	case wizardStepConfirm:
		v.confirmPass, cmd = v.confirmPass.Update(msg)
	}

	return v, cmd
}

func (v *SetupWizardView) handleEnter() (tea.Model, tea.Cmd) {
	switch v.step {
	case wizardStepTemplate:
		v.step = wizardStepDBName
		v.dbName.Focus()
		return v, textinput.Blink

	case wizardStepDBName:
		if v.dbName.Value() == "" {
			v.err = fmt.Errorf("database name is required")
			return v, nil
		}
		v.err = nil
		v.dbName.Blur()
		v.step = wizardStepUsername
		v.username.Focus()
		// Default username based on db name
		if v.username.Value() == "" {
			v.username.SetValue(v.dbName.Value() + "_user")
		}
		return v, textinput.Blink

	case wizardStepUsername:
		if v.username.Value() == "" {
			v.err = fmt.Errorf("username is required")
			return v, nil
		}
		v.err = nil
		v.username.Blur()
		v.step = wizardStepPassword
		v.password.Focus()
		return v, textinput.Blink

	case wizardStepPassword:
		if v.password.Value() == "" {
			v.err = fmt.Errorf("password is required")
			return v, nil
		}
		v.err = nil
		v.password.Blur()
		v.step = wizardStepConfirm
		v.confirmPass.Focus()
		return v, textinput.Blink

	case wizardStepConfirm:
		if v.confirmPass.Value() != v.password.Value() {
			v.err = fmt.Errorf("passwords do not match")
			return v, nil
		}
		v.err = nil
		v.confirmPass.Blur()
		v.step = wizardStepAdvanced
		return v, nil

	case wizardStepAdvanced:
		v.step = wizardStepReview
		return v, nil

	case wizardStepReview:
		v.processing = true
		return v, v.runSetup()

	case wizardStepComplete:
		return v, func() tea.Msg {
			return SwitchViewMsg{View: "databases"}
		}
	}

	return v, nil
}

func (v *SetupWizardView) handleUp() (tea.Model, tea.Cmd) {
	switch v.step {
	case wizardStepTemplate:
		v.templateIndex--
		if v.templateIndex < 0 {
			v.templateIndex = len(v.templates) - 1
		}
	case wizardStepAdvanced:
		// Handle in specific sub-handler
	}
	return v, nil
}

func (v *SetupWizardView) handleDown() (tea.Model, tea.Cmd) {
	switch v.step {
	case wizardStepTemplate:
		v.templateIndex++
		if v.templateIndex >= len(v.templates) {
			v.templateIndex = 0
		}
	case wizardStepAdvanced:
		// Handle in specific sub-handler
	}
	return v, nil
}

func (v *SetupWizardView) handleLeft() (tea.Model, tea.Cmd) {
	switch v.step {
	case wizardStepAdvanced:
		// Cycle through options
		v.hostIndex--
		if v.hostIndex < 0 {
			v.hostIndex = len(defaultHosts2) - 1
		}
	}
	return v, nil
}

func (v *SetupWizardView) handleRight() (tea.Model, tea.Cmd) {
	switch v.step {
	case wizardStepAdvanced:
		// Cycle through options
		v.hostIndex++
		if v.hostIndex >= len(defaultHosts2) {
			v.hostIndex = 0
		}
	}
	return v, nil
}

func (v *SetupWizardView) prevStep() {
	switch v.step {
	case wizardStepDBName:
		v.step = wizardStepTemplate
		v.dbName.Blur()
	case wizardStepUsername:
		v.step = wizardStepDBName
		v.username.Blur()
		v.dbName.Focus()
	case wizardStepPassword:
		v.step = wizardStepUsername
		v.password.Blur()
		v.username.Focus()
	case wizardStepConfirm:
		v.step = wizardStepPassword
		v.confirmPass.Blur()
		v.password.Focus()
	case wizardStepAdvanced:
		v.step = wizardStepConfirm
		v.confirmPass.Focus()
	case wizardStepReview:
		v.step = wizardStepAdvanced
	}
	v.err = nil
}

func (v *SetupWizardView) runSetup() tea.Cmd {
	template := v.templates[v.templateIndex]
	dbName := v.dbName.Value()
	username := v.username.Value()
	password := v.password.Value()
	host := defaultHosts2[v.hostIndex]

	// Apply advanced settings if changed
	if v.charsetIndex > 0 && v.charsetIndex < len(v.charsets) {
		template.Charset = v.charsets[v.charsetIndex]
	}
	if v.collationIndex > 0 && v.collationIndex < len(v.collations) {
		template.Collation = v.collations[v.collationIndex]
	}

	return func() tea.Msg {
		if err := v.conn.SetupAppDatabase(&template, dbName, username, password, host); err != nil {
			return err
		}
		return setupCompleteMsg{}
	}
}

// View renders the view
func (v *SetupWizardView) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("App Database Setup Wizard"))
	b.WriteString("\n\n")

	// Progress indicator
	steps := []string{"Template", "Database", "User", "Password", "Confirm", "Options", "Review"}
	currentStep := int(v.step)
	if currentStep >= len(steps) {
		currentStep = len(steps) - 1
	}

	for i, s := range steps {
		if i == currentStep {
			b.WriteString(focusedStyle.Render(fmt.Sprintf("[%s]", s)))
		} else if i < currentStep {
			b.WriteString(successStyle.Render(fmt.Sprintf("[✓ %s]", s)))
		} else {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("[ %s ]", s)))
		}
		if i < len(steps)-1 {
			b.WriteString(" → ")
		}
	}
	b.WriteString("\n\n")

	// Render current step
	switch v.step {
	case wizardStepTemplate:
		b.WriteString(v.viewTemplateStep())
	case wizardStepDBName:
		b.WriteString(v.viewDBNameStep())
	case wizardStepUsername:
		b.WriteString(v.viewUsernameStep())
	case wizardStepPassword:
		b.WriteString(v.viewPasswordStep())
	case wizardStepConfirm:
		b.WriteString(v.viewConfirmStep())
	case wizardStepAdvanced:
		b.WriteString(v.viewAdvancedStep())
	case wizardStepReview:
		b.WriteString(v.viewReviewStep())
	case wizardStepComplete:
		b.WriteString(v.viewCompleteStep())
	}

	// Error display
	if v.err != nil {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", v.err)))
		b.WriteString("\n")
	}

	// Help
	b.WriteString("\n")
	if v.step == wizardStepComplete {
		b.WriteString(helpStyle.Render("Enter: Return to databases | Esc: Return to databases"))
	} else if v.step == wizardStepTemplate {
		b.WriteString(helpStyle.Render("↑↓: Select template | Enter: Next | Esc: Cancel"))
	} else {
		b.WriteString(helpStyle.Render("Enter: Next | Esc: Back"))
	}

	return b.String()
}

func (v *SetupWizardView) viewTemplateStep() string {
	var b strings.Builder

	b.WriteString("Select an application template:\n\n")

	maxShow := 8
	start := 0
	if v.templateIndex >= maxShow {
		start = v.templateIndex - maxShow + 1
	}

	for i := start; i < len(v.templates) && i < start+maxShow; i++ {
		t := v.templates[i]
		if i == v.templateIndex {
			b.WriteString(focusedStyle.Render(fmt.Sprintf("  → %s", t.Name)))
			b.WriteString(mutedStyle.Render(fmt.Sprintf(" - %s", t.Description)))
		} else {
			b.WriteString(fmt.Sprintf("    %s", t.Name))
			b.WriteString(mutedStyle.Render(fmt.Sprintf(" - %s", t.Description)))
		}
		b.WriteString("\n")
	}

	if len(v.templates) > maxShow {
		b.WriteString(mutedStyle.Render(fmt.Sprintf("\n    ... and %d more", len(v.templates)-maxShow)))
	}

	// Show selected template details
	b.WriteString("\n\nTemplate details:\n")
	t := v.templates[v.templateIndex]
	b.WriteString(fmt.Sprintf("  Charset:   %s\n", t.Charset))
	if t.Collation != "" {
		b.WriteString(fmt.Sprintf("  Collation: %s\n", t.Collation))
	}
	privStr := strings.Join(t.Privileges, ", ")
	if len(privStr) > 60 {
		privStr = privStr[:60] + "..."
	}
	b.WriteString(fmt.Sprintf("  Privileges: %s\n", privStr))

	return b.String()
}

func (v *SetupWizardView) viewDBNameStep() string {
	var b strings.Builder

	b.WriteString("Enter the database name:\n\n")
	b.WriteString(focusedStyle.Render("Database Name:"))
	b.WriteString("\n")
	b.WriteString(v.dbName.View())
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("This will be the name of the new database."))

	return b.String()
}

func (v *SetupWizardView) viewUsernameStep() string {
	var b strings.Builder

	b.WriteString("Enter the username for this database:\n\n")
	b.WriteString(focusedStyle.Render("Username:"))
	b.WriteString("\n")
	b.WriteString(v.username.View())
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("This user will have access to the database."))

	return b.String()
}

func (v *SetupWizardView) viewPasswordStep() string {
	var b strings.Builder

	b.WriteString("Enter the password for the user:\n\n")
	b.WriteString(focusedStyle.Render("Password:"))
	b.WriteString("\n")
	b.WriteString(v.password.View())
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("Choose a secure password."))

	return b.String()
}

func (v *SetupWizardView) viewConfirmStep() string {
	var b strings.Builder

	b.WriteString("Confirm the password:\n\n")
	b.WriteString(focusedStyle.Render("Confirm Password:"))
	b.WriteString("\n")
	b.WriteString(v.confirmPass.View())

	return b.String()
}

func (v *SetupWizardView) viewAdvancedStep() string {
	var b strings.Builder

	b.WriteString("Advanced options (optional):\n\n")

	// Host selection (MariaDB)
	isMariaDB := v.conn.Config.Type == db.DatabaseTypeMariaDB
	if isMariaDB {
		b.WriteString(focusedStyle.Render("Host:"))
		b.WriteString(" (←/→ to change)\n")
		b.WriteString(fmt.Sprintf("  [ %s ]\n\n", defaultHosts2[v.hostIndex]))
	}

	b.WriteString(mutedStyle.Render("Press Enter to continue with default settings,\nor use arrow keys to modify options."))

	return b.String()
}

func (v *SetupWizardView) viewReviewStep() string {
	var b strings.Builder

	t := v.templates[v.templateIndex]

	b.WriteString("Review your settings:\n\n")
	b.WriteString(fmt.Sprintf("  Template:  %s\n", t.Name))
	b.WriteString(fmt.Sprintf("  Database:  %s\n", v.dbName.Value()))
	b.WriteString(fmt.Sprintf("  Username:  %s@%s\n", v.username.Value(), defaultHosts2[v.hostIndex]))
	b.WriteString(fmt.Sprintf("  Charset:   %s\n", t.Charset))
	if t.Collation != "" {
		b.WriteString(fmt.Sprintf("  Collation: %s\n", t.Collation))
	}

	b.WriteString("\n")

	if v.processing {
		b.WriteString("Setting up database...")
	} else {
		b.WriteString(focusedStyle.Render("Press Enter to create the database and user."))
	}

	return b.String()
}

func (v *SetupWizardView) viewCompleteStep() string {
	var b strings.Builder

	t := v.templates[v.templateIndex]

	b.WriteString(successStyle.Render("Setup completed successfully!"))
	b.WriteString("\n\n")
	b.WriteString("Your new database is ready:\n\n")
	b.WriteString(fmt.Sprintf("  Database: %s\n", v.dbName.Value()))
	b.WriteString(fmt.Sprintf("  Username: %s\n", v.username.Value()))
	b.WriteString(fmt.Sprintf("  Host:     %s\n", v.conn.Config.Host))
	b.WriteString(fmt.Sprintf("  Port:     %d\n", v.conn.Config.Port))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Configured for: %s\n", t.Description))

	return b.String()
}
