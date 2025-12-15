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
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ConnectionPool manages multiple database connections
type ConnectionPool struct {
	connections map[string]*Connection
	mu          sync.RWMutex
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool() *ConnectionPool {
	return &ConnectionPool{
		connections: make(map[string]*Connection),
	}
}

// Add adds a connection to the pool with a name
func (p *ConnectionPool) Add(name string, conn *Connection) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.connections[name] = conn
}

// Get retrieves a connection by name
func (p *ConnectionPool) Get(name string) (*Connection, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	conn, ok := p.connections[name]
	return conn, ok
}

// Remove removes and closes a connection
func (p *ConnectionPool) Remove(name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if conn, ok := p.connections[name]; ok {
		delete(p.connections, name)
		return conn.Close()
	}
	return nil
}

// CloseAll closes all connections
func (p *ConnectionPool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, conn := range p.connections {
		conn.Close()
	}
	p.connections = make(map[string]*Connection)
}

// List returns all connection names
func (p *ConnectionPool) List() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	names := make([]string, 0, len(p.connections))
	for name := range p.connections {
		names = append(names, name)
	}
	return names
}

// CloneOptions configures database cloning
type CloneOptions struct {
	SourceDB     string
	TargetDB     string
	IncludeData  bool // If false, only clone structure
	DropIfExists bool // Drop target database if it exists
	OnProgress   func(table string, tableNum, totalTables int)
}

// CloneDatabase creates a copy of a database
func (c *Connection) CloneDatabase(opts CloneOptions) error {
	// Check if target exists
	if opts.DropIfExists {
		c.DB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", c.QuoteIdentifier(opts.TargetDB)))
	}

	// Create target database
	_, err := c.DB.Exec(c.Driver.CreateDatabaseQuery(opts.TargetDB))
	if err != nil {
		return fmt.Errorf("failed to create target database: %w", err)
	}

	// Switch to source database
	if err := c.UseDatabase(opts.SourceDB); err != nil {
		return err
	}

	// Get all tables
	tables, err := c.ListTables()
	if err != nil {
		return fmt.Errorf("failed to list tables: %w", err)
	}

	// Clone each table
	for i, table := range tables {
		if opts.OnProgress != nil {
			opts.OnProgress(table.Name, i+1, len(tables))
		}

		// Get CREATE TABLE statement
		createStmt, err := c.getCreateTable(table.Name)
		if err != nil {
			return fmt.Errorf("failed to get CREATE TABLE for %s: %w", table.Name, err)
		}

		// Create table in target database
		if err := c.UseDatabase(opts.TargetDB); err != nil {
			return fmt.Errorf("failed to switch to target database: %w", err)
		}

		if _, err := c.DB.Exec(createStmt); err != nil {
			return fmt.Errorf("failed to create table %s: %w", table.Name, err)
		}

		// Copy data if requested
		if opts.IncludeData {
			_, err := c.DB.Exec(fmt.Sprintf(
				"INSERT INTO %s.%s SELECT * FROM %s.%s",
				c.QuoteIdentifier(opts.TargetDB), c.QuoteIdentifier(table.Name),
				c.QuoteIdentifier(opts.SourceDB), c.QuoteIdentifier(table.Name),
			))
			if err != nil {
				return fmt.Errorf("failed to copy data for %s: %w", table.Name, err)
			}
		}

		// Switch back to source for next iteration
		c.UseDatabase(opts.SourceDB)
	}

	return nil
}

// MergeOptions configures database merging
type MergeOptions struct {
	SourceDBs       []string // Databases to merge from
	TargetDB        string   // Database to merge into
	CreateTarget    bool     // Create target if it doesn't exist
	ConflictHandler func(table string, sourceDB string) MergeConflictAction
	OnProgress      func(sourceDB, table string, sourceNum, totalSources int)
}

// MergeConflictAction defines how to handle merge conflicts
type MergeConflictAction int

const (
	MergeSkip     MergeConflictAction = iota // Skip conflicting table
	MergeReplace                             // Replace with source table
	MergeAppend                              // Append data to existing table
	MergeRename                              // Rename source table (add suffix)
)

// MergeDatabases merges multiple databases into one
func (c *Connection) MergeDatabases(opts MergeOptions) error {
	// Create target if needed
	if opts.CreateTarget {
		c.DB.Exec(c.Driver.CreateDatabaseQuery(opts.TargetDB))
	}

	// Get existing tables in target
	if err := c.UseDatabase(opts.TargetDB); err != nil {
		return fmt.Errorf("failed to switch to target database: %w", err)
	}
	existingTables, err := c.ListTables()
	if err != nil {
		return fmt.Errorf("failed to list target tables: %w", err)
	}
	existingTableMap := make(map[string]bool)
	for _, t := range existingTables {
		existingTableMap[t.Name] = true
	}

	// Process each source database
	for sourceNum, sourceDB := range opts.SourceDBs {
		if err := c.UseDatabase(sourceDB); err != nil {
			return fmt.Errorf("failed to switch to source database %s: %w", sourceDB, err)
		}

		tables, err := c.ListTables()
		if err != nil {
			return fmt.Errorf("failed to list tables in %s: %w", sourceDB, err)
		}

		for _, table := range tables {
			if opts.OnProgress != nil {
				opts.OnProgress(sourceDB, table.Name, sourceNum+1, len(opts.SourceDBs))
			}

			tableName := table.Name
			action := MergeAppend // Default action

			// Check for conflicts
			if existingTableMap[tableName] {
				if opts.ConflictHandler != nil {
					action = opts.ConflictHandler(tableName, sourceDB)
				}
			} else {
				action = MergeReplace // No conflict, just copy
			}

			switch action {
			case MergeSkip:
				continue

			case MergeReplace:
				// Drop existing and copy
				c.DB.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s.%s",
					c.QuoteIdentifier(opts.TargetDB), c.QuoteIdentifier(tableName)))

				createStmt, err := c.getCreateTable(tableName)
				if err != nil {
					return fmt.Errorf("failed to get CREATE TABLE for %s: %w", tableName, err)
				}

				if err := c.UseDatabase(opts.TargetDB); err != nil {
					return err
				}
				if _, err := c.DB.Exec(createStmt); err != nil {
					return fmt.Errorf("failed to create table %s: %w", tableName, err)
				}

				_, err = c.DB.Exec(fmt.Sprintf(
					"INSERT INTO %s.%s SELECT * FROM %s.%s",
					c.QuoteIdentifier(opts.TargetDB), c.QuoteIdentifier(tableName),
					c.QuoteIdentifier(sourceDB), c.QuoteIdentifier(tableName),
				))
				if err != nil {
					return fmt.Errorf("failed to copy data for %s: %w", tableName, err)
				}

				existingTableMap[tableName] = true

			case MergeAppend:
				// Just append data (assumes compatible schema)
				_, err := c.DB.Exec(fmt.Sprintf(
					"INSERT INTO %s.%s SELECT * FROM %s.%s",
					c.QuoteIdentifier(opts.TargetDB), c.QuoteIdentifier(tableName),
					c.QuoteIdentifier(sourceDB), c.QuoteIdentifier(tableName),
				))
				if err != nil {
					return fmt.Errorf("failed to append data for %s: %w", tableName, err)
				}

			case MergeRename:
				// Copy with new name
				newName := fmt.Sprintf("%s_%s", tableName, sourceDB)

				createStmt, err := c.getCreateTable(tableName)
				if err != nil {
					return fmt.Errorf("failed to get CREATE TABLE for %s: %w", tableName, err)
				}

				// Replace table name in CREATE statement
				createStmt = strings.Replace(createStmt,
					fmt.Sprintf("CREATE TABLE %s", c.QuoteIdentifier(tableName)),
					fmt.Sprintf("CREATE TABLE %s", c.QuoteIdentifier(newName)), 1)

				if err := c.UseDatabase(opts.TargetDB); err != nil {
					return err
				}
				if _, err := c.DB.Exec(createStmt); err != nil {
					return fmt.Errorf("failed to create renamed table %s: %w", newName, err)
				}

				_, err = c.DB.Exec(fmt.Sprintf(
					"INSERT INTO %s.%s SELECT * FROM %s.%s",
					c.QuoteIdentifier(opts.TargetDB), c.QuoteIdentifier(newName),
					c.QuoteIdentifier(sourceDB), c.QuoteIdentifier(tableName),
				))
				if err != nil {
					return fmt.Errorf("failed to copy data for %s: %w", newName, err)
				}

				existingTableMap[newName] = true
			}

			// Switch back to source
			c.UseDatabase(sourceDB)
		}
	}

	return nil
}

// CopyTableOptions configures table copying
type CopyTableOptions struct {
	SourceDB      string
	SourceTable   string
	TargetDB      string
	TargetTable   string // If empty, use same name as source
	IncludeData   bool
	DropIfExists  bool
	WhereClause   string // Optional WHERE clause for filtering data
	OnProgress    func(rowsCopied int64)
	BatchSize     int // Rows per batch (0 = all at once)
}

// CopyTable copies a table between databases
func (c *Connection) CopyTable(opts CopyTableOptions) error {
	if opts.TargetTable == "" {
		opts.TargetTable = opts.SourceTable
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 10000
	}

	// Get source table structure
	if err := c.UseDatabase(opts.SourceDB); err != nil {
		return err
	}

	createStmt, err := c.getCreateTable(opts.SourceTable)
	if err != nil {
		return fmt.Errorf("failed to get source table structure: %w", err)
	}

	// Modify CREATE statement if table name is different
	if opts.TargetTable != opts.SourceTable {
		createStmt = strings.Replace(createStmt,
			fmt.Sprintf("CREATE TABLE %s", c.QuoteIdentifier(opts.SourceTable)),
			fmt.Sprintf("CREATE TABLE %s", c.QuoteIdentifier(opts.TargetTable)), 1)
	}

	// Create target table
	if opts.DropIfExists {
		c.DB.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s.%s",
			c.QuoteIdentifier(opts.TargetDB), c.QuoteIdentifier(opts.TargetTable)))
	}

	if err := c.UseDatabase(opts.TargetDB); err != nil {
		return fmt.Errorf("failed to switch to target database: %w", err)
	}

	if _, err := c.DB.Exec(createStmt); err != nil {
		return fmt.Errorf("failed to create target table: %w", err)
	}

	// Copy data if requested
	if opts.IncludeData {
		query := fmt.Sprintf("SELECT * FROM %s.%s",
			c.QuoteIdentifier(opts.SourceDB), c.QuoteIdentifier(opts.SourceTable))
		if opts.WhereClause != "" {
			query += " WHERE " + opts.WhereClause
		}

		// For large tables, use batched inserts
		var rowsCopied int64
		offset := 0

		for {
			batchQuery := fmt.Sprintf("%s LIMIT %d OFFSET %d", query, opts.BatchSize, offset)
			rows, err := c.DB.Query(batchQuery)
			if err != nil {
				return fmt.Errorf("failed to query source table: %w", err)
			}

			columns, _ := rows.Columns()
			if len(columns) == 0 {
				rows.Close()
				break
			}

			var batch []string
			for rows.Next() {
				valuePtrs := make([]interface{}, len(columns))
				valueHolders := make([]interface{}, len(columns))
				for i := range valuePtrs {
					valuePtrs[i] = &valueHolders[i]
				}

				if err := rows.Scan(valuePtrs...); err != nil {
					rows.Close()
					return fmt.Errorf("failed to scan row: %w", err)
				}

				var rowValues []string
				for _, val := range valueHolders {
					rowValues = append(rowValues, c.formatValueForInsert(val))
				}
				batch = append(batch, fmt.Sprintf("(%s)", strings.Join(rowValues, ", ")))
			}
			rows.Close()

			if len(batch) == 0 {
				break
			}

			// Quote column names
			quotedColumns := make([]string, len(columns))
			for i, col := range columns {
				quotedColumns[i] = c.QuoteIdentifier(col)
			}

			insertQuery := fmt.Sprintf(
				"INSERT INTO %s.%s (%s) VALUES %s",
				c.QuoteIdentifier(opts.TargetDB), c.QuoteIdentifier(opts.TargetTable),
				strings.Join(quotedColumns, ", "),
				strings.Join(batch, ", "),
			)

			if _, err := c.DB.Exec(insertQuery); err != nil {
				return fmt.Errorf("failed to insert batch: %w", err)
			}

			rowsCopied += int64(len(batch))
			if opts.OnProgress != nil {
				opts.OnProgress(rowsCopied)
			}

			offset += opts.BatchSize

			if len(batch) < opts.BatchSize {
				break // Last batch
			}
		}
	}

	return nil
}

// formatValueForInsert formats a value for use in an INSERT statement
func (c *Connection) formatValueForInsert(val interface{}) string {
	if val == nil {
		return "NULL"
	}

	switch v := val.(type) {
	case []byte:
		s := string(v)
		// Check if it looks like binary data
		if containsBinaryData(v) {
			return fmt.Sprintf("X'%X'", v)
		}
		return fmt.Sprintf("'%s'", c.EscapeString(s))
	case string:
		return fmt.Sprintf("'%s'", c.EscapeString(v))
	case int64, int32, int, uint64, uint32, uint:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%g", v)
	case float32:
		return fmt.Sprintf("%g", v)
	case bool:
		if v {
			return "1"
		}
		return "0"
	case time.Time:
		return fmt.Sprintf("'%s'", v.Format("2006-01-02 15:04:05"))
	default:
		return fmt.Sprintf("'%s'", c.EscapeString(fmt.Sprintf("%v", v)))
	}
}

// SyncOptions configures database synchronization
type SyncOptions struct {
	SourceDB   string
	TargetDB   string
	Tables     []string // Empty = all tables
	SyncMode   SyncMode
	DryRun     bool // Just report what would change
	OnProgress func(table string, action string)
}

// SyncMode defines how synchronization works
type SyncMode int

const (
	SyncStructureOnly SyncMode = iota // Only sync table structures
	SyncDataOnly                      // Only sync data (tables must exist)
	SyncFull                          // Sync both structure and data
)

// SyncResult contains synchronization results
type SyncResult struct {
	TablesCreated  []string
	TablesModified []string
	TablesSkipped  []string
	RowsInserted   int64
	RowsUpdated    int64
	RowsDeleted    int64
}

// CompareSchemas compares schemas between two databases
func (c *Connection) CompareSchemas(db1, db2 string) (*SchemaComparison, error) {
	result := &SchemaComparison{
		OnlyInFirst:  make([]string, 0),
		OnlyInSecond: make([]string, 0),
		Different:    make([]TableDiff, 0),
		Identical:    make([]string, 0),
	}

	// Get tables from both databases
	if err := c.UseDatabase(db1); err != nil {
		return nil, err
	}
	tables1, err := c.ListTables()
	if err != nil {
		return nil, err
	}
	tableMap1 := make(map[string]string)
	for _, t := range tables1 {
		create, _ := c.getCreateTable(t.Name)
		tableMap1[t.Name] = create
	}

	if err := c.UseDatabase(db2); err != nil {
		return nil, err
	}
	tables2, err := c.ListTables()
	if err != nil {
		return nil, err
	}
	tableMap2 := make(map[string]string)
	for _, t := range tables2 {
		create, _ := c.getCreateTable(t.Name)
		tableMap2[t.Name] = create
	}

	// Compare
	for name, create1 := range tableMap1 {
		if create2, ok := tableMap2[name]; ok {
			if create1 == create2 {
				result.Identical = append(result.Identical, name)
			} else {
				result.Different = append(result.Different, TableDiff{
					TableName:    name,
					FirstSchema:  create1,
					SecondSchema: create2,
				})
			}
		} else {
			result.OnlyInFirst = append(result.OnlyInFirst, name)
		}
	}

	for name := range tableMap2 {
		if _, ok := tableMap1[name]; !ok {
			result.OnlyInSecond = append(result.OnlyInSecond, name)
		}
	}

	return result, nil
}

// SchemaComparison holds the result of comparing two database schemas
type SchemaComparison struct {
	OnlyInFirst  []string
	OnlyInSecond []string
	Different    []TableDiff
	Identical    []string
}

// TableDiff represents differences in a table between databases
type TableDiff struct {
	TableName    string
	FirstSchema  string
	SecondSchema string
}

// HealthCheck performs a health check on the connection
func (c *Connection) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return c.DB.PingContext(ctx)
}

// GetServerInfo returns server information
func (c *Connection) GetServerInfo() (*ServerInfo, error) {
	info := &ServerInfo{}

	// Get version
	c.DB.QueryRow(c.Driver.ServerVersionQuery()).Scan(&info.Version)

	// Get uptime - handle differently based on database type
	if c.Config.Type == DatabaseTypePostgres {
		var uptime int64
		c.DB.QueryRow(c.Driver.UptimeQuery()).Scan(&uptime)
		info.Uptime = time.Duration(uptime) * time.Second
	} else {
		var varName string
		var uptime int64
		c.DB.QueryRow(c.Driver.UptimeQuery()).Scan(&varName, &uptime)
		info.Uptime = time.Duration(uptime) * time.Second
	}

	// Get connection count
	if c.Config.Type == DatabaseTypePostgres {
		c.DB.QueryRow(c.Driver.ConnectionCountQuery()).Scan(&info.Connections)
	} else {
		var varName string
		c.DB.QueryRow(c.Driver.ConnectionCountQuery()).Scan(&varName, &info.Connections)
	}

	// Get database size - different queries for different DBs
	info.DatabaseSizes = make(map[string]int64)

	if c.Config.Type == DatabaseTypePostgres {
		rows, err := c.DB.Query(`
			SELECT datname, pg_database_size(datname)
			FROM pg_database
			WHERE datistemplate = false
		`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var name string
				var size int64
				rows.Scan(&name, &size)
				info.DatabaseSizes[name] = size
			}
		}
	} else {
		rows, err := c.DB.Query(`
			SELECT table_schema, SUM(data_length + index_length)
			FROM information_schema.tables
			GROUP BY table_schema
		`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var name string
				var size int64
				rows.Scan(&name, &size)
				info.DatabaseSizes[name] = size
			}
		}
	}

	return info, nil
}

// ServerInfo contains database server information
type ServerInfo struct {
	Version       string
	Uptime        time.Duration
	Connections   int
	DatabaseSizes map[string]int64
}
