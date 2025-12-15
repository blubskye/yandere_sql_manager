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

// User represents a database user
type User struct {
	Username string
	Host     string // Empty for PostgreSQL
}

// Grant represents a user privilege
type Grant struct {
	Privilege string
	Database  string
	Table     string
	GrantText string // Raw grant statement (MariaDB)
}

// ListUsers returns all database users
func (c *Connection) ListUsers() ([]User, error) {
	query := c.Driver.ListUsersQuery()
	rows, err := c.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.Username, &u.Host); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, u)
	}

	return users, rows.Err()
}

// CreateUser creates a new database user
func (c *Connection) CreateUser(username, host, password string) error {
	if host == "" {
		host = "localhost"
	}

	query := c.Driver.CreateUserQuery(username, host, password)
	_, err := c.DB.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create user '%s'@'%s': %w", username, host, err)
	}

	// Flush privileges for MariaDB
	flushQuery := c.Driver.FlushPrivilegesQuery()
	if flushQuery != "" {
		c.DB.Exec(flushQuery)
	}

	return nil
}

// DropUser deletes a database user
func (c *Connection) DropUser(username, host string) error {
	if host == "" {
		host = "localhost"
	}

	query := c.Driver.DropUserQuery(username, host)
	_, err := c.DB.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to drop user '%s'@'%s': %w", username, host, err)
	}

	// Flush privileges for MariaDB
	flushQuery := c.Driver.FlushPrivilegesQuery()
	if flushQuery != "" {
		c.DB.Exec(flushQuery)
	}

	return nil
}

// GetUserGrants returns the grants for a user
func (c *Connection) GetUserGrants(username, host string) ([]Grant, error) {
	if host == "" {
		host = "localhost"
	}

	query := c.Driver.ShowUserGrantsQuery(username, host)
	rows, err := c.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get grants for '%s'@'%s': %w", username, host, err)
	}
	defer rows.Close()

	var grants []Grant

	// Get column count to determine format
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		if len(cols) == 1 {
			// MariaDB: returns GRANT statement as single column
			var grantText string
			if err := rows.Scan(&grantText); err != nil {
				return nil, fmt.Errorf("failed to scan grant: %w", err)
			}
			grants = append(grants, Grant{
				GrantText: grantText,
			})
		} else {
			// PostgreSQL: returns database, object, privilege columns
			var g Grant
			if err := rows.Scan(&g.Database, &g.Table, &g.Privilege); err != nil {
				return nil, fmt.Errorf("failed to scan grant: %w", err)
			}
			grants = append(grants, g)
		}
	}

	return grants, rows.Err()
}

// GrantPrivileges grants privileges to a user
func (c *Connection) GrantPrivileges(username, host string, privileges []string, database, table string) error {
	if host == "" {
		host = "localhost"
	}

	if len(privileges) == 0 {
		privileges = []string{"ALL PRIVILEGES"}
	}

	query := c.Driver.GrantPrivilegesQuery(privileges, database, table, username, host)

	// Handle multiple statements (PostgreSQL may return semicolon-separated)
	statements := strings.Split(query, ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		_, err := c.DB.Exec(stmt)
		if err != nil {
			return fmt.Errorf("failed to grant privileges: %w", err)
		}
	}

	// Flush privileges for MariaDB
	flushQuery := c.Driver.FlushPrivilegesQuery()
	if flushQuery != "" {
		c.DB.Exec(flushQuery)
	}

	return nil
}

// RevokePrivileges revokes privileges from a user
func (c *Connection) RevokePrivileges(username, host string, privileges []string, database, table string) error {
	if host == "" {
		host = "localhost"
	}

	if len(privileges) == 0 {
		privileges = []string{"ALL PRIVILEGES"}
	}

	query := c.Driver.RevokePrivilegesQuery(privileges, database, table, username, host)

	// Handle multiple statements (PostgreSQL may return semicolon-separated)
	statements := strings.Split(query, ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		_, err := c.DB.Exec(stmt)
		if err != nil {
			return fmt.Errorf("failed to revoke privileges: %w", err)
		}
	}

	// Flush privileges for MariaDB
	flushQuery := c.Driver.FlushPrivilegesQuery()
	if flushQuery != "" {
		c.DB.Exec(flushQuery)
	}

	return nil
}

// CreateUserWithDBAccess creates a user and grants access to a specific database
// This is a convenience function for the app setup wizard
func (c *Connection) CreateUserWithDBAccess(username, host, password, database string) error {
	if host == "" {
		host = "localhost"
	}

	// Create the user
	if err := c.CreateUser(username, host, password); err != nil {
		return err
	}

	// Grant privileges on the database
	if err := c.GrantPrivileges(username, host, []string{"ALL PRIVILEGES"}, database, ""); err != nil {
		// Try to clean up the user if grant fails
		c.DropUser(username, host)
		return fmt.Errorf("failed to grant privileges: %w", err)
	}

	return nil
}

// UserExists checks if a user exists
func (c *Connection) UserExists(username, host string) (bool, error) {
	users, err := c.ListUsers()
	if err != nil {
		return false, err
	}

	for _, u := range users {
		if u.Username == username {
			// For PostgreSQL, host is empty so just check username
			if c.Config.Type == DatabaseTypePostgres {
				return true, nil
			}
			// For MariaDB, check both username and host
			if u.Host == host || host == "" {
				return true, nil
			}
		}
	}

	return false, nil
}

// CommonPrivileges returns a list of common privilege options
func CommonPrivileges() []string {
	return []string{
		"ALL PRIVILEGES",
		"SELECT",
		"INSERT",
		"UPDATE",
		"DELETE",
		"CREATE",
		"DROP",
		"INDEX",
		"ALTER",
		"REFERENCES",
		"CREATE TEMPORARY TABLES",
		"LOCK TABLES",
		"EXECUTE",
		"CREATE VIEW",
		"SHOW VIEW",
		"CREATE ROUTINE",
		"ALTER ROUTINE",
		"EVENT",
		"TRIGGER",
	}
}
