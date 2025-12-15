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

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// KeyAction represents an action that can be triggered by a keybinding
type KeyAction string

const (
	// Navigation actions
	ActionSelect      KeyAction = "select"
	ActionBack        KeyAction = "back"
	ActionQuit        KeyAction = "quit"
	ActionFilter      KeyAction = "filter"
	ActionRefresh     KeyAction = "refresh"
	ActionUp          KeyAction = "up"
	ActionDown        KeyAction = "down"
	ActionPageUp      KeyAction = "page_up"
	ActionPageDown    KeyAction = "page_down"
	ActionTop         KeyAction = "top"
	ActionBottom      KeyAction = "bottom"

	// View switching actions
	ActionNewDatabase KeyAction = "new_database"
	ActionDashboard   KeyAction = "dashboard"
	ActionCluster     KeyAction = "cluster"
	ActionUsers       KeyAction = "users"
	ActionBackup      KeyAction = "backup"
	ActionImport      KeyAction = "import"
	ActionExport      KeyAction = "export"
	ActionQuery       KeyAction = "query"
	ActionVariables   KeyAction = "variables"
	ActionSettings    KeyAction = "settings"

	// Editing actions
	ActionEdit        KeyAction = "edit"
	ActionDelete      KeyAction = "delete"
	ActionCreate      KeyAction = "create"
	ActionSave        KeyAction = "save"
	ActionCancel      KeyAction = "cancel"

	// Toggle actions
	ActionToggleGlobal KeyAction = "toggle_global"
	ActionToggleAutoRefresh KeyAction = "toggle_auto_refresh"
	ActionClearFilter  KeyAction = "clear_filter"

	// Tab navigation
	ActionNextTab     KeyAction = "next_tab"
	ActionPrevTab     KeyAction = "prev_tab"
	ActionTab1        KeyAction = "tab1"
	ActionTab2        KeyAction = "tab2"
	ActionTab3        KeyAction = "tab3"
	ActionTab4        KeyAction = "tab4"
)

// KeyBinding represents a single keybinding
type KeyBinding struct {
	Key         string    `yaml:"key"`
	Action      KeyAction `yaml:"action"`
	Description string    `yaml:"description,omitempty"`
}

// KeyBindings holds all keybindings configuration
type KeyBindings struct {
	// Global keybindings (work in all views)
	Global map[KeyAction]string `yaml:"global"`

	// View-specific keybindings
	Databases map[KeyAction]string `yaml:"databases"`
	Tables    map[KeyAction]string `yaml:"tables"`
	Browser   map[KeyAction]string `yaml:"browser"`
	Query     map[KeyAction]string `yaml:"query"`
	Settings  map[KeyAction]string `yaml:"settings"`
	Users     map[KeyAction]string `yaml:"users"`
	Backup    map[KeyAction]string `yaml:"backup"`
	Dashboard map[KeyAction]string `yaml:"dashboard"`
	Cluster   map[KeyAction]string `yaml:"cluster"`
}

// DefaultKeyBindings returns the default keybindings
func DefaultKeyBindings() *KeyBindings {
	return &KeyBindings{
		Global: map[KeyAction]string{
			ActionQuit:     "q",
			ActionBack:     "esc",
			ActionSelect:   "enter",
			ActionFilter:   "/",
			ActionRefresh:  "r",
			ActionUp:       "up",
			ActionDown:     "down",
			ActionPageUp:   "pgup",
			ActionPageDown: "pgdown",
			ActionTop:      "home",
			ActionBottom:   "end",
		},
		Databases: map[KeyAction]string{
			ActionNewDatabase: "n",
			ActionDashboard:   "d",
			ActionCluster:     "c",
			ActionUsers:       "u",
			ActionBackup:      "b",
			ActionImport:      "i",
			ActionExport:      "e",
			ActionQuery:       "s",
			ActionVariables:   "v",
			ActionSettings:    "?",
		},
		Tables: map[KeyAction]string{
			ActionQuery:  "s",
			ActionImport: "i",
			ActionExport: "e",
		},
		Browser: map[KeyAction]string{
			ActionEdit:   "e",
			ActionDelete: "d",
		},
		Query: map[KeyAction]string{
			ActionSave:   "ctrl+s",
			ActionCancel: "esc",
		},
		Settings: map[KeyAction]string{
			ActionToggleGlobal: "g",
			ActionClearFilter:  "c",
			ActionEdit:         "enter",
		},
		Users: map[KeyAction]string{
			ActionCreate: "n",
			ActionDelete: "d",
			ActionEdit:   "e",
		},
		Backup: map[KeyAction]string{
			ActionCreate: "n",
			ActionDelete: "d",
		},
		Dashboard: map[KeyAction]string{
			ActionToggleAutoRefresh: "a",
			ActionNextTab:           "tab",
			ActionPrevTab:           "shift+tab",
		},
		Cluster: map[KeyAction]string{
			ActionToggleAutoRefresh: "a",
			ActionTab1:              "1",
			ActionTab2:              "2",
			ActionTab3:              "3",
			ActionTab4:              "4",
		},
	}
}

// KeyBindingsPath returns the keybindings file path
func KeyBindingsPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "keybindings.yaml"), nil
}

// LoadKeyBindings loads keybindings from disk
func LoadKeyBindings() (*KeyBindings, error) {
	path, err := KeyBindingsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default keybindings if file doesn't exist
			return DefaultKeyBindings(), nil
		}
		return nil, fmt.Errorf("failed to read keybindings file: %w", err)
	}

	kb := DefaultKeyBindings() // Start with defaults
	if err := yaml.Unmarshal(data, kb); err != nil {
		return nil, fmt.Errorf("failed to parse keybindings file: %w", err)
	}

	return kb, nil
}

// Save saves keybindings to disk
func (kb *KeyBindings) Save() error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	path, err := KeyBindingsPath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(kb)
	if err != nil {
		return fmt.Errorf("failed to marshal keybindings: %w", err)
	}

	// Add header comment
	header := `# YSM Keybindings Configuration
# Customize your keybindings here~ <3
#
# Available keys: a-z, 0-9, enter, esc, tab, space, backspace, delete
#                 up, down, left, right, home, end, pgup, pgdown
#                 f1-f12, ctrl+<key>, shift+<key>, alt+<key>
#
# To reset to defaults, delete this file and restart YSM~

`
	content := header + string(data)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write keybindings file: %w", err)
	}

	return nil
}

// GetKey returns the key for an action in a specific view
// Falls back to global keybindings if not found in view-specific bindings
func (kb *KeyBindings) GetKey(view string, action KeyAction) string {
	var viewBindings map[KeyAction]string

	switch view {
	case "databases":
		viewBindings = kb.Databases
	case "tables":
		viewBindings = kb.Tables
	case "browser":
		viewBindings = kb.Browser
	case "query":
		viewBindings = kb.Query
	case "settings":
		viewBindings = kb.Settings
	case "users":
		viewBindings = kb.Users
	case "backup":
		viewBindings = kb.Backup
	case "dashboard":
		viewBindings = kb.Dashboard
	case "cluster":
		viewBindings = kb.Cluster
	}

	// Check view-specific binding first
	if viewBindings != nil {
		if key, ok := viewBindings[action]; ok {
			return key
		}
	}

	// Fall back to global binding
	if key, ok := kb.Global[action]; ok {
		return key
	}

	return ""
}

// SetKey sets a keybinding for an action
func (kb *KeyBindings) SetKey(view string, action KeyAction, key string) error {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	switch view {
	case "global":
		if kb.Global == nil {
			kb.Global = make(map[KeyAction]string)
		}
		kb.Global[action] = key
	case "databases":
		if kb.Databases == nil {
			kb.Databases = make(map[KeyAction]string)
		}
		kb.Databases[action] = key
	case "tables":
		if kb.Tables == nil {
			kb.Tables = make(map[KeyAction]string)
		}
		kb.Tables[action] = key
	case "browser":
		if kb.Browser == nil {
			kb.Browser = make(map[KeyAction]string)
		}
		kb.Browser[action] = key
	case "query":
		if kb.Query == nil {
			kb.Query = make(map[KeyAction]string)
		}
		kb.Query[action] = key
	case "settings":
		if kb.Settings == nil {
			kb.Settings = make(map[KeyAction]string)
		}
		kb.Settings[action] = key
	case "users":
		if kb.Users == nil {
			kb.Users = make(map[KeyAction]string)
		}
		kb.Users[action] = key
	case "backup":
		if kb.Backup == nil {
			kb.Backup = make(map[KeyAction]string)
		}
		kb.Backup[action] = key
	case "dashboard":
		if kb.Dashboard == nil {
			kb.Dashboard = make(map[KeyAction]string)
		}
		kb.Dashboard[action] = key
	case "cluster":
		if kb.Cluster == nil {
			kb.Cluster = make(map[KeyAction]string)
		}
		kb.Cluster[action] = key
	default:
		return fmt.Errorf("unknown view: %s", view)
	}

	return nil
}

// IsKey checks if a key matches an action in a view
func (kb *KeyBindings) IsKey(view string, key string, action KeyAction) bool {
	boundKey := kb.GetKey(view, action)
	return strings.EqualFold(key, boundKey)
}

// GetActionDescription returns a human-readable description for an action
func GetActionDescription(action KeyAction) string {
	descriptions := map[KeyAction]string{
		ActionSelect:            "Select item",
		ActionBack:              "Go back",
		ActionQuit:              "Quit YSM",
		ActionFilter:            "Filter list",
		ActionRefresh:           "Refresh data",
		ActionUp:                "Move up",
		ActionDown:              "Move down",
		ActionPageUp:            "Page up",
		ActionPageDown:          "Page down",
		ActionTop:               "Go to top",
		ActionBottom:            "Go to bottom",
		ActionNewDatabase:       "New database (wizard)",
		ActionDashboard:         "Statistics dashboard",
		ActionCluster:           "Cluster status",
		ActionUsers:             "User management",
		ActionBackup:            "Backup management",
		ActionImport:            "Import SQL file",
		ActionExport:            "Export database",
		ActionQuery:             "SQL query editor",
		ActionVariables:         "System variables",
		ActionSettings:          "Settings & keybindings",
		ActionEdit:              "Edit item",
		ActionDelete:            "Delete item",
		ActionCreate:            "Create new",
		ActionSave:              "Save changes",
		ActionCancel:            "Cancel",
		ActionToggleGlobal:      "Toggle global/session",
		ActionToggleAutoRefresh: "Toggle auto-refresh",
		ActionClearFilter:       "Clear filter",
		ActionNextTab:           "Next tab",
		ActionPrevTab:           "Previous tab",
		ActionTab1:              "Tab 1",
		ActionTab2:              "Tab 2",
		ActionTab3:              "Tab 3",
		ActionTab4:              "Tab 4",
	}

	if desc, ok := descriptions[action]; ok {
		return desc
	}
	return string(action)
}

// AllActions returns all available actions grouped by category
func AllActions() map[string][]KeyAction {
	return map[string][]KeyAction{
		"Navigation": {
			ActionSelect,
			ActionBack,
			ActionQuit,
			ActionFilter,
			ActionRefresh,
			ActionUp,
			ActionDown,
			ActionPageUp,
			ActionPageDown,
			ActionTop,
			ActionBottom,
		},
		"Views": {
			ActionNewDatabase,
			ActionDashboard,
			ActionCluster,
			ActionUsers,
			ActionBackup,
			ActionImport,
			ActionExport,
			ActionQuery,
			ActionVariables,
			ActionSettings,
		},
		"Editing": {
			ActionEdit,
			ActionDelete,
			ActionCreate,
			ActionSave,
			ActionCancel,
		},
		"Toggles": {
			ActionToggleGlobal,
			ActionToggleAutoRefresh,
			ActionClearFilter,
		},
		"Tabs": {
			ActionNextTab,
			ActionPrevTab,
			ActionTab1,
			ActionTab2,
			ActionTab3,
			ActionTab4,
		},
	}
}

// ViewNames returns all view names
func ViewNames() []string {
	return []string{
		"global",
		"databases",
		"tables",
		"browser",
		"query",
		"settings",
		"users",
		"backup",
		"dashboard",
		"cluster",
	}
}
