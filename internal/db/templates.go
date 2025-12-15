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

import "fmt"

// AppTemplate defines a preset for common applications
type AppTemplate struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Charset     string   `json:"charset"`
	Collation   string   `json:"collation"`
	Privileges  []string `json:"privileges"`
}

// DefaultTemplates returns the built-in application templates
func DefaultTemplates() []AppTemplate {
	return []AppTemplate{
		{
			Name:        "default",
			Description: "Standard database with default settings",
			Charset:     "utf8mb4",
			Collation:   "utf8mb4_unicode_ci",
			Privileges:  []string{"ALL PRIVILEGES"},
		},
		{
			Name:        "wordpress",
			Description: "WordPress CMS",
			Charset:     "utf8mb4",
			Collation:   "utf8mb4_unicode_ci",
			Privileges:  []string{"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "ALTER", "INDEX"},
		},
		{
			Name:        "laravel",
			Description: "Laravel PHP Framework",
			Charset:     "utf8mb4",
			Collation:   "utf8mb4_unicode_ci",
			Privileges:  []string{"ALL PRIVILEGES"},
		},
		{
			Name:        "drupal",
			Description: "Drupal CMS",
			Charset:     "utf8mb4",
			Collation:   "utf8mb4_unicode_ci",
			Privileges:  []string{"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "INDEX", "ALTER", "CREATE TEMPORARY TABLES", "LOCK TABLES"},
		},
		{
			Name:        "nextcloud",
			Description: "Nextcloud file sharing",
			Charset:     "utf8mb4",
			Collation:   "utf8mb4_bin",
			Privileges:  []string{"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "INDEX", "ALTER", "CREATE TEMPORARY TABLES", "LOCK TABLES"},
		},
		{
			Name:        "matomo",
			Description: "Matomo analytics (Piwik)",
			Charset:     "utf8mb4",
			Collation:   "utf8mb4_unicode_ci",
			Privileges:  []string{"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "ALTER", "INDEX", "CREATE TEMPORARY TABLES", "LOCK TABLES"},
		},
		{
			Name:        "magento",
			Description: "Magento e-commerce",
			Charset:     "utf8mb4",
			Collation:   "utf8mb4_unicode_ci",
			Privileges:  []string{"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "INDEX", "ALTER", "CREATE TEMPORARY TABLES", "LOCK TABLES", "TRIGGER"},
		},
		{
			Name:        "joomla",
			Description: "Joomla CMS",
			Charset:     "utf8mb4",
			Collation:   "utf8mb4_unicode_ci",
			Privileges:  []string{"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "INDEX", "ALTER"},
		},
		{
			Name:        "prestashop",
			Description: "PrestaShop e-commerce",
			Charset:     "utf8mb4",
			Collation:   "utf8mb4_unicode_ci",
			Privileges:  []string{"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "ALTER", "INDEX", "LOCK TABLES"},
		},
		{
			Name:        "mediawiki",
			Description: "MediaWiki",
			Charset:     "binary",
			Collation:   "",
			Privileges:  []string{"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "INDEX", "ALTER", "CREATE TEMPORARY TABLES", "LOCK TABLES"},
		},
		{
			Name:        "readonly",
			Description: "Read-only access for reporting",
			Charset:     "utf8mb4",
			Collation:   "utf8mb4_unicode_ci",
			Privileges:  []string{"SELECT"},
		},
	}
}

// GetTemplate returns a template by name
func GetTemplate(name string) (*AppTemplate, error) {
	templates := DefaultTemplates()
	for _, t := range templates {
		if t.Name == name {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("template not found: %s", name)
}

// GetCharsets returns available character sets for MariaDB
func (c *Connection) GetCharsets() ([]string, error) {
	query := c.Driver.GetCharsetsQuery()
	rows, err := c.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get charsets: %w", err)
	}
	defer rows.Close()

	var charsets []string
	for rows.Next() {
		var charset string
		var maxLen, description, defaultCollation interface{}

		// MariaDB returns: Charset, Description, Default collation, Maxlen
		if err := rows.Scan(&charset, &description, &defaultCollation, &maxLen); err != nil {
			// Try with just charset
			rows.Scan(&charset)
		}
		charsets = append(charsets, charset)
	}

	return charsets, rows.Err()
}

// GetCollations returns available collations for a character set
func (c *Connection) GetCollations(charset string) ([]string, error) {
	query := c.Driver.GetCollationsQuery(charset)
	rows, err := c.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get collations: %w", err)
	}
	defer rows.Close()

	var collations []string
	for rows.Next() {
		var collation string
		var other interface{}

		// MariaDB returns multiple columns
		cols, _ := rows.Columns()
		if len(cols) > 1 {
			// Scan just the first column (Collation name)
			scanArgs := make([]interface{}, len(cols))
			scanArgs[0] = &collation
			for i := 1; i < len(cols); i++ {
				scanArgs[i] = &other
			}
			rows.Scan(scanArgs...)
		} else {
			rows.Scan(&collation)
		}
		collations = append(collations, collation)
	}

	return collations, rows.Err()
}

// CreateDatabaseWithOptions creates a database with specific charset and collation
func (c *Connection) CreateDatabaseWithOptions(name, charset, collation string) error {
	query := c.Driver.CreateDatabaseWithOptionsQuery(name, charset, collation)
	_, err := c.DB.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}
	return nil
}

// SetupAppDatabase creates a database and user for an application
func (c *Connection) SetupAppDatabase(template *AppTemplate, dbName, username, password, host string) error {
	if host == "" {
		host = "localhost"
	}

	// Create the database with template settings
	if err := c.CreateDatabaseWithOptions(dbName, template.Charset, template.Collation); err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	// Create the user
	if err := c.CreateUser(username, host, password); err != nil {
		// Try to clean up database if user creation fails
		c.DB.Exec(c.Driver.DropDatabaseQuery(dbName))
		return fmt.Errorf("failed to create user: %w", err)
	}

	// Grant privileges
	if err := c.GrantPrivileges(username, host, template.Privileges, dbName, ""); err != nil {
		// Try to clean up
		c.DropUser(username, host)
		c.DB.Exec(c.Driver.DropDatabaseQuery(dbName))
		return fmt.Errorf("failed to grant privileges: %w", err)
	}

	return nil
}

// CommonCharsets returns commonly used character sets
func CommonCharsets() []string {
	return []string{
		"utf8mb4",
		"utf8",
		"latin1",
		"ascii",
		"binary",
	}
}

// CommonCollationsForCharset returns common collations for a charset
func CommonCollationsForCharset(charset string) []string {
	switch charset {
	case "utf8mb4":
		return []string{
			"utf8mb4_unicode_ci",
			"utf8mb4_general_ci",
			"utf8mb4_bin",
			"utf8mb4_unicode_520_ci",
		}
	case "utf8":
		return []string{
			"utf8_unicode_ci",
			"utf8_general_ci",
			"utf8_bin",
		}
	case "latin1":
		return []string{
			"latin1_swedish_ci",
			"latin1_general_ci",
			"latin1_bin",
		}
	case "ascii":
		return []string{
			"ascii_general_ci",
			"ascii_bin",
		}
	case "binary":
		return []string{"binary"}
	default:
		return []string{}
	}
}
