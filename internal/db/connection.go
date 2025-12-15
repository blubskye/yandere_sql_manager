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
	_ "github.com/lib/pq"
)

// Connection holds the database connection and configuration
type Connection struct {
	DB     *sql.DB
	Config ConnectionConfig
	Driver Driver
}

// ConnectionConfig holds the connection parameters
type ConnectionConfig struct {
	Type     DatabaseType // Database type (mariadb, postgres)
	Host     string
	Port     int
	User     string
	Password string
	Database string
	Socket   string // Unix socket path (optional, MariaDB only)
}

// Connect establishes a connection to the database server
func Connect(cfg ConnectionConfig) (*Connection, error) {
	// Default to MariaDB for backward compatibility
	if cfg.Type == "" {
		cfg.Type = DatabaseTypeMariaDB
	}

	// Get the appropriate driver
	driver, err := GetDriver(cfg.Type)
	if err != nil {
		return nil, err
	}

	// Set default port if not specified
	if cfg.Port == 0 {
		cfg.Port = driver.DefaultPort()
	}

	// Open connection using driver-specific DSN
	db, err := sql.Open(driver.DriverName(), driver.DSN(cfg))
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
		Driver: driver,
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
	stmt := c.Driver.UseDatabaseStatement(name)

	if stmt == "" {
		// PostgreSQL requires reconnecting to switch databases
		return c.reconnectToDatabase(name)
	}

	// MariaDB can use USE statement
	_, err := c.DB.Exec(stmt)
	if err != nil {
		return fmt.Errorf("failed to use database %s: %w", name, err)
	}
	c.Config.Database = name
	return nil
}

// reconnectToDatabase closes and reopens connection with new database (for PostgreSQL)
func (c *Connection) reconnectToDatabase(name string) error {
	// Close existing connection
	if err := c.DB.Close(); err != nil {
		return fmt.Errorf("failed to close existing connection: %w", err)
	}

	// Update config and reconnect
	newCfg := c.Config
	newCfg.Database = name

	db, err := sql.Open(c.Driver.DriverName(), c.Driver.DSN(newCfg))
	if err != nil {
		return fmt.Errorf("failed to reconnect to database %s: %w", name, err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping database %s: %w", name, err)
	}

	c.DB = db
	c.Config.Database = name
	return nil
}

// DefaultPort returns the default port for the given database type
func DefaultPort(dbType DatabaseType) int {
	driver, err := GetDriver(dbType)
	if err != nil {
		return 3306 // Fallback to MariaDB default
	}
	return driver.DefaultPort()
}

// QuoteIdentifier quotes an identifier using the connection's driver
func (c *Connection) QuoteIdentifier(name string) string {
	return c.Driver.QuoteIdentifier(name)
}

// EscapeString escapes a string using the connection's driver
func (c *Connection) EscapeString(s string) string {
	return c.Driver.EscapeString(s)
}
