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

package db

import (
	"fmt"
	"strings"
)

// Variable represents a MariaDB system variable
type Variable struct {
	Name  string
	Value string
	Scope string // GLOBAL, SESSION, or BOTH
}

// CommonVariables lists frequently used system variables
var CommonVariables = []string{
	"foreign_key_checks",
	"unique_checks",
	"autocommit",
	"sql_mode",
	"wait_timeout",
	"max_allowed_packet",
	"character_set_client",
	"character_set_results",
	"character_set_connection",
	"collation_connection",
	"time_zone",
	"tx_isolation",
	"sql_safe_updates",
	"sql_select_limit",
}

// GetVariable retrieves a single system variable value
func (c *Connection) GetVariable(name string) (string, error) {
	var varName, value string
	query := fmt.Sprintf("SHOW VARIABLES LIKE '%s'", name)
	err := c.DB.QueryRow(query).Scan(&varName, &value)
	if err != nil {
		return "", fmt.Errorf("failed to get variable '%s': %w", name, err)
	}
	return value, nil
}

// GetVariables retrieves variables matching a pattern
func (c *Connection) GetVariables(pattern string) ([]Variable, error) {
	if pattern == "" {
		pattern = "%"
	}

	query := fmt.Sprintf("SHOW VARIABLES LIKE '%s'", pattern)
	rows, err := c.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get variables: %w", err)
	}
	defer rows.Close()

	var variables []Variable
	for rows.Next() {
		var v Variable
		if err := rows.Scan(&v.Name, &v.Value); err != nil {
			return nil, fmt.Errorf("failed to scan variable: %w", err)
		}
		v.Scope = "SESSION"
		variables = append(variables, v)
	}

	return variables, rows.Err()
}

// GetGlobalVariables retrieves global variables matching a pattern
func (c *Connection) GetGlobalVariables(pattern string) ([]Variable, error) {
	if pattern == "" {
		pattern = "%"
	}

	query := fmt.Sprintf("SHOW GLOBAL VARIABLES LIKE '%s'", pattern)
	rows, err := c.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get global variables: %w", err)
	}
	defer rows.Close()

	var variables []Variable
	for rows.Next() {
		var v Variable
		if err := rows.Scan(&v.Name, &v.Value); err != nil {
			return nil, fmt.Errorf("failed to scan variable: %w", err)
		}
		v.Scope = "GLOBAL"
		variables = append(variables, v)
	}

	return variables, rows.Err()
}

// GetCommonVariables retrieves the common variables with their current values
func (c *Connection) GetCommonVariables() ([]Variable, error) {
	var variables []Variable
	for _, name := range CommonVariables {
		value, err := c.GetVariable(name)
		if err != nil {
			// Variable might not exist, skip it
			continue
		}
		variables = append(variables, Variable{
			Name:  name,
			Value: value,
			Scope: "SESSION",
		})
	}
	return variables, nil
}

// SetVariable sets a system variable
func (c *Connection) SetVariable(name, value string, global bool) error {
	// Sanitize the variable name (only alphanumeric and underscores allowed)
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return fmt.Errorf("invalid variable name: %s", name)
		}
	}

	var query string
	if global {
		query = fmt.Sprintf("SET GLOBAL %s = ?", name)
	} else {
		query = fmt.Sprintf("SET SESSION %s = ?", name)
	}

	_, err := c.DB.Exec(query, value)
	if err != nil {
		scope := "session"
		if global {
			scope = "global"
		}
		return fmt.Errorf("failed to set %s variable '%s': %w", scope, name, err)
	}

	return nil
}

// ApplyVariables applies a map of variables to the current session
func (c *Connection) ApplyVariables(vars map[string]string) error {
	var errors []string
	for name, value := range vars {
		if err := c.SetVariable(name, value, false); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to set some variables: %s", strings.Join(errors, "; "))
	}

	return nil
}

// GetVariableInfo returns detailed information about a variable
func (c *Connection) GetVariableInfo(name string) (*Variable, error) {
	// Get session value
	sessionVal, sessionErr := c.GetVariable(name)

	// Get global value
	var globalVal string
	query := fmt.Sprintf("SHOW GLOBAL VARIABLES LIKE '%s'", name)
	globalErr := c.DB.QueryRow(query).Scan(new(string), &globalVal)

	if sessionErr != nil && globalErr != nil {
		return nil, fmt.Errorf("variable '%s' not found", name)
	}

	v := &Variable{
		Name: name,
	}

	if sessionErr == nil {
		v.Value = sessionVal
		v.Scope = "SESSION"
	}

	if globalErr == nil && sessionErr != nil {
		v.Value = globalVal
		v.Scope = "GLOBAL"
	}

	if sessionErr == nil && globalErr == nil {
		if sessionVal == globalVal {
			v.Scope = "BOTH"
		} else {
			v.Scope = "SESSION"
		}
	}

	return v, nil
}
