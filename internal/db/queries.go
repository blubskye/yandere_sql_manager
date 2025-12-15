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
)

// Database represents a database with its metadata
type Database struct {
	Name string
}

// Table represents a table with its metadata
type Table struct {
	Name   string
	Engine string
	Rows   int64
}

// Column represents a table column
type Column struct {
	Field   string
	Type    string
	Null    string
	Key     string
	Default *string
	Extra   string
}

// QueryResult holds the result of a query
type QueryResult struct {
	Columns []string
	Rows    [][]string
}

// ListDatabases returns all databases on the server
func (c *Connection) ListDatabases() ([]Database, error) {
	rows, err := c.DB.Query(c.Driver.ListDatabasesQuery())
	if err != nil {
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}
	defer rows.Close()

	var databases []Database
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan database: %w", err)
		}
		databases = append(databases, Database{Name: name})
	}

	return databases, rows.Err()
}

// ListTables returns all tables in the current database
func (c *Connection) ListTables() ([]Table, error) {
	rows, err := c.DB.Query(c.Driver.ListTablesQuery())
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var tables []Table
	for rows.Next() {
		// Create a slice of interface{} to hold all columns
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan table: %w", err)
		}

		table := Table{}
		for i, col := range columns {
			val := values[i]
			switch col {
			case "Name":
				if v, ok := val.([]byte); ok {
					table.Name = string(v)
				} else if v, ok := val.(string); ok {
					table.Name = v
				}
			case "Engine":
				if v, ok := val.([]byte); ok {
					table.Engine = string(v)
				} else if v, ok := val.(string); ok {
					table.Engine = v
				}
			case "Rows":
				switch v := val.(type) {
				case int64:
					table.Rows = v
				case float64:
					table.Rows = int64(v)
				case []byte:
					fmt.Sscanf(string(v), "%d", &table.Rows)
				}
			}
		}
		tables = append(tables, table)
	}

	return tables, rows.Err()
}

// DescribeTable returns the columns of a table
func (c *Connection) DescribeTable(tableName string) ([]Column, error) {
	rows, err := c.DB.Query(c.Driver.DescribeTableQuery(tableName))
	if err != nil {
		return nil, fmt.Errorf("failed to describe table: %w", err)
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var col Column
		if err := rows.Scan(&col.Field, &col.Type, &col.Null, &col.Key, &col.Default, &col.Extra); err != nil {
			return nil, fmt.Errorf("failed to scan column: %w", err)
		}
		columns = append(columns, col)
	}

	return columns, rows.Err()
}

// Query executes a SQL query and returns the results
func (c *Connection) Query(sql string) (*QueryResult, error) {
	rows, err := c.DB.Query(sql)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	result := &QueryResult{
		Columns: columns,
	}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		row := make([]string, len(columns))
		for i, val := range values {
			if val == nil {
				row[i] = "NULL"
			} else {
				switch v := val.(type) {
				case []byte:
					row[i] = string(v)
				default:
					row[i] = fmt.Sprintf("%v", v)
				}
			}
		}
		result.Rows = append(result.Rows, row)
	}

	return result, rows.Err()
}

// Execute runs a SQL statement that doesn't return rows
func (c *Connection) Execute(sql string) (int64, error) {
	result, err := c.DB.Exec(sql)
	if err != nil {
		return 0, fmt.Errorf("execution failed: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, nil // Some statements don't support RowsAffected
	}

	return affected, nil
}

// GetTableData returns rows from a table with pagination
func (c *Connection) GetTableData(tableName string, limit, offset int) (*QueryResult, error) {
	query := fmt.Sprintf("SELECT * FROM %s LIMIT %d OFFSET %d", c.QuoteIdentifier(tableName), limit, offset)
	return c.Query(query)
}

// CountTableRows returns the number of rows in a table
func (c *Connection) CountTableRows(tableName string) (int64, error) {
	var count int64
	err := c.DB.QueryRow(c.Driver.TableRowCountQuery(tableName)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count rows: %w", err)
	}
	return count, nil
}

// CreateDatabase creates a new database
func (c *Connection) CreateDatabase(name string) error {
	_, err := c.DB.Exec(c.Driver.CreateDatabaseQuery(name))
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}
	return nil
}

// DropDatabase deletes a database
func (c *Connection) DropDatabase(name string) error {
	_, err := c.DB.Exec(c.Driver.DropDatabaseQuery(name))
	if err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}
	return nil
}
