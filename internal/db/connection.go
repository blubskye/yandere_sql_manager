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
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

// Connection holds the database connection and configuration
type Connection struct {
	DB     *sql.DB
	Config ConnectionConfig
}

// ConnectionConfig holds the connection parameters
type ConnectionConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	Socket   string // Unix socket path (optional)
}

// DSN returns the data source name for the connection
func (c *ConnectionConfig) DSN() string {
	if c.Socket != "" {
		// Unix socket connection
		if c.Database != "" {
			return fmt.Sprintf("%s:%s@unix(%s)/%s?parseTime=true&multiStatements=true",
				c.User, c.Password, c.Socket, c.Database)
		}
		return fmt.Sprintf("%s:%s@unix(%s)/?parseTime=true&multiStatements=true",
			c.User, c.Password, c.Socket)
	}

	// TCP connection
	if c.Database != "" {
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&multiStatements=true",
			c.User, c.Password, c.Host, c.Port, c.Database)
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/?parseTime=true&multiStatements=true",
		c.User, c.Password, c.Host, c.Port)
}

// Connect establishes a connection to the MariaDB server
func Connect(cfg ConnectionConfig) (*Connection, error) {
	db, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Connection{
		DB:     db,
		Config: cfg,
	}, nil
}

// Close closes the database connection
func (c *Connection) Close() error {
	if c.DB != nil {
		return c.DB.Close()
	}
	return nil
}

// UseDatabase switches to a different database
func (c *Connection) UseDatabase(name string) error {
	_, err := c.DB.Exec("USE " + name)
	if err != nil {
		return fmt.Errorf("failed to use database %s: %w", name, err)
	}
	c.Config.Database = name
	return nil
}

// DefaultPort returns the default MariaDB port
func DefaultPort() int {
	return 3306
}
