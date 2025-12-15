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
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/blubskye/yandere_sql_manager/internal/buffer"
	"github.com/blubskye/yandere_sql_manager/internal/logging"
)

// CompressionType represents supported compression formats
type CompressionType string

const (
	CompressionNone CompressionType = ""
	CompressionGzip CompressionType = "gzip"
	CompressionXZ   CompressionType = "xz"
	CompressionZstd CompressionType = "zstd"
)

// ExportOptions configures the export behavior
type ExportOptions struct {
	FilePath        string
	Database        string
	Tables          []string        // Empty = all tables
	NoData          bool            // Export structure only
	NoCreate        bool            // Export data only
	AddDropTable    bool            // Add DROP TABLE statements
	Compression     CompressionType // Compression type (auto-detected from extension if empty)
	BufferSize      int             // Write buffer size (0 = default 64KB)
	BatchSize       int             // Rows per INSERT batch (0 = default 1000)
	IncludeVars     bool            // Include SET statements for session variables
	IncludeVarsList []string        // Specific variables to include (empty = common variables)
	OnProgress      func(currentTable string, tableNum, totalTables int, rowsExported int64)
}

// ExportStats contains statistics about the export
type ExportStats struct {
	TablesExported int
	RowsExported   int64
	BytesWritten   int64
	Duration       time.Duration
	Compressed     bool
	OutputFile     string
}

// ExportSQL exports a database to a SQL file with improved buffering
func (c *Connection) ExportSQL(opts ExportOptions) error {
	stats, err := c.ExportSQLWithStats(opts)
	_ = stats
	return err
}

// ExportSQLWithStats exports a database and returns detailed statistics
func (c *Connection) ExportSQLWithStats(opts ExportOptions) (*ExportStats, error) {
	startTime := time.Now()
	stats := &ExportStats{}

	logging.Debug("Starting SQL export to: %s", opts.FilePath)
	logging.Debug("Database: %s, Tables: %v", opts.Database, opts.Tables)

	// Set defaults - use larger buffers for better performance
	if opts.BufferSize <= 0 {
		opts.BufferSize = buffer.LargeBufferSize // 8MB buffer for exports
		logging.Debug("Using buffer size: %d bytes", opts.BufferSize)
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 1000 // 1000 rows per INSERT
		logging.Debug("Using batch size: %d rows", opts.BatchSize)
	}

	if opts.Database != "" {
		if err := c.UseDatabase(opts.Database); err != nil {
			return nil, err
		}
	}

	// Detect compression from filename if not specified
	compression := opts.Compression
	if compression == "" {
		ext := strings.ToLower(filepath.Ext(opts.FilePath))
		switch ext {
		case ".xz":
			compression = CompressionXZ
		case ".zst", ".zstd":
			compression = CompressionZstd
		case ".gz", ".gzip":
			compression = CompressionGzip
		}
	}

	// Create output file
	file, err := os.Create(opts.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Set up writer chain based on compression
	var writer io.Writer
	var compressCmd *exec.Cmd

	switch compression {
	case CompressionXZ:
		stats.Compressed = true
		compressCmd = exec.Command("xz", "-c", "-6") // Level 6 is good balance
		compressCmd.Stdout = file
		stdin, err := compressCmd.StdinPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create xz pipe: %w", err)
		}
		if err := compressCmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to start xz compression (is xz installed?): %w", err)
		}
		writer = stdin
		defer func() {
			stdin.Close()
			compressCmd.Wait()
		}()

	case CompressionZstd:
		stats.Compressed = true
		compressCmd = exec.Command("zstd", "-c", "-3") // Level 3 is fast with good compression
		compressCmd.Stdout = file
		stdin, err := compressCmd.StdinPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create zstd pipe: %w", err)
		}
		if err := compressCmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to start zstd compression (is zstd installed?): %w", err)
		}
		writer = stdin
		defer func() {
			stdin.Close()
			compressCmd.Wait()
		}()

	case CompressionGzip:
		stats.Compressed = true
		gzWriter := gzip.NewWriter(file)
		defer gzWriter.Close()
		writer = gzWriter

	default:
		writer = file
	}

	// Wrap in buffered writer
	bufWriter := bufio.NewWriterSize(writer, opts.BufferSize)
	defer bufWriter.Flush()

	// Write header
	fmt.Fprintf(bufWriter, "-- YSM (Yandere SQL Manager) Database Export\n")
	fmt.Fprintf(bufWriter, "-- Database: %s\n", opts.Database)
	fmt.Fprintf(bufWriter, "-- Type: %s\n", c.Config.Type)
	fmt.Fprintf(bufWriter, "-- Generated: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(bufWriter, "-- \"I'll never let your databases go~\"\n\n")

	// Include session variables if requested
	if opts.IncludeVars {
		fmt.Fprintf(bufWriter, "-- Session Variables\n")
		varList := opts.IncludeVarsList
		if len(varList) == 0 {
			varList = c.Driver.CommonVariables()
		}
		for _, varName := range varList {
			value, err := c.GetVariable(varName)
			if err == nil && value != "" {
				if c.Config.Type == DatabaseTypePostgres {
					fmt.Fprintf(bufWriter, "SET %s = '%s';\n", varName, c.EscapeString(value))
				} else {
					fmt.Fprintf(bufWriter, "SET @saved_%s = @@%s;\n", varName, varName)
					fmt.Fprintf(bufWriter, "SET %s = '%s';\n", varName, c.EscapeString(value))
				}
			}
		}
		fmt.Fprintf(bufWriter, "\n")
	}

	// Write database-specific header
	fmt.Fprintf(bufWriter, "%s\n", c.Driver.ExportHeader())

	// Get tables to export
	tables := opts.Tables
	if len(tables) == 0 {
		tableList, err := c.ListTables()
		if err != nil {
			return nil, fmt.Errorf("failed to list tables: %w", err)
		}
		for _, t := range tableList {
			tables = append(tables, t.Name)
		}
	}

	// Export each table
	var totalRows int64
	for i, tableName := range tables {
		if opts.OnProgress != nil {
			opts.OnProgress(tableName, i+1, len(tables), totalRows)
		}

		fmt.Fprintf(bufWriter, "-- --------------------------------------------------------\n")
		fmt.Fprintf(bufWriter, "-- Table structure for table %s\n", c.QuoteIdentifier(tableName))
		fmt.Fprintf(bufWriter, "-- --------------------------------------------------------\n\n")

		// Export table structure
		if !opts.NoCreate {
			if opts.AddDropTable {
				fmt.Fprintf(bufWriter, "DROP TABLE IF EXISTS %s;\n", c.QuoteIdentifier(tableName))
			}

			createStmt, err := c.getCreateTable(tableName)
			if err != nil {
				return nil, fmt.Errorf("failed to get CREATE TABLE for %s: %w", tableName, err)
			}
			fmt.Fprintf(bufWriter, "%s;\n\n", createStmt)
		}

		// Export table data
		if !opts.NoData {
			rowCount, err := c.exportTableDataBuffered(bufWriter, tableName, opts.BatchSize)
			if err != nil {
				return nil, fmt.Errorf("failed to export data for %s: %w", tableName, err)
			}
			totalRows += rowCount
		}

		stats.TablesExported++
	}

	// Write database-specific footer
	fmt.Fprintf(bufWriter, "\n%s", c.Driver.ExportFooter())

	// Ensure everything is flushed
	bufWriter.Flush()

	stats.RowsExported = totalRows
	stats.Duration = time.Since(startTime)
	stats.OutputFile = opts.FilePath

	// Get file size
	if info, err := file.Stat(); err == nil {
		stats.BytesWritten = info.Size()
	}

	return stats, nil
}

func (c *Connection) getCreateTable(tableName string) (string, error) {
	if c.Config.Type == DatabaseTypePostgres {
		// PostgreSQL: Build CREATE TABLE from information_schema
		return c.buildCreateTablePostgres(tableName)
	}

	// MariaDB: Use SHOW CREATE TABLE
	var name, createStmt string
	err := c.DB.QueryRow(c.Driver.GetCreateTableQuery(tableName)).Scan(&name, &createStmt)
	if err != nil {
		return "", err
	}
	return createStmt, nil
}

// buildCreateTablePostgres builds a CREATE TABLE statement from information_schema
func (c *Connection) buildCreateTablePostgres(tableName string) (string, error) {
	// Get columns
	rows, err := c.DB.Query(`
		SELECT column_name, data_type, character_maximum_length,
		       is_nullable, column_default, udt_name
		FROM information_schema.columns
		WHERE table_name = $1 AND table_schema = 'public'
		ORDER BY ordinal_position`, tableName)
	if err != nil {
		return "", fmt.Errorf("failed to get columns: %w", err)
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var colName, dataType, isNullable string
		var charMaxLen *int64
		var colDefault, udtName *string

		if err := rows.Scan(&colName, &dataType, &charMaxLen, &isNullable, &colDefault, &udtName); err != nil {
			return "", err
		}

		colDef := fmt.Sprintf("  %s ", c.QuoteIdentifier(colName))

		// Build type with length if applicable
		if charMaxLen != nil && *charMaxLen > 0 {
			colDef += fmt.Sprintf("%s(%d)", dataType, *charMaxLen)
		} else if udtName != nil && *udtName != "" && dataType == "USER-DEFINED" {
			colDef += *udtName
		} else {
			colDef += dataType
		}

		// Add NOT NULL if applicable
		if isNullable == "NO" {
			colDef += " NOT NULL"
		}

		// Add default if applicable
		if colDefault != nil && *colDefault != "" {
			// Skip nextval defaults (serial columns)
			if !strings.HasPrefix(*colDefault, "nextval(") {
				colDef += fmt.Sprintf(" DEFAULT %s", *colDefault)
			}
		}

		columns = append(columns, colDef)
	}

	if len(columns) == 0 {
		return "", fmt.Errorf("no columns found for table %s", tableName)
	}

	// Get primary key
	pkRows, err := c.DB.Query(`
		SELECT a.attname
		FROM pg_index i
		JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
		WHERE i.indrelid = $1::regclass AND i.indisprimary`, tableName)
	if err == nil {
		defer pkRows.Close()
		var pkCols []string
		for pkRows.Next() {
			var colName string
			pkRows.Scan(&colName)
			pkCols = append(pkCols, c.QuoteIdentifier(colName))
		}
		if len(pkCols) > 0 {
			columns = append(columns, fmt.Sprintf("  PRIMARY KEY (%s)", strings.Join(pkCols, ", ")))
		}
	}

	createStmt := fmt.Sprintf("CREATE TABLE %s (\n%s\n)",
		c.QuoteIdentifier(tableName),
		strings.Join(columns, ",\n"))

	return createStmt, nil
}

// exportTableDataBuffered exports table data with batched INSERTs
func (c *Connection) exportTableDataBuffered(writer *bufio.Writer, tableName string, batchSize int) (int64, error) {
	rows, err := c.DB.Query(fmt.Sprintf("SELECT * FROM %s", c.QuoteIdentifier(tableName)))
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return 0, err
	}

	if len(columns) == 0 {
		return 0, nil
	}

	var rowCount int64
	var values []string

	// Quote column names for the INSERT statement
	quotedColumns := make([]string, len(columns))
	for i, col := range columns {
		quotedColumns[i] = c.QuoteIdentifier(col)
	}

	// Write table comment
	fmt.Fprintf(writer, "-- Dumping data for table %s\n\n", c.QuoteIdentifier(tableName))

	for rows.Next() {
		// Create slice to hold column values
		valuePtrs := make([]interface{}, len(columns))
		valueHolders := make([]interface{}, len(columns))
		for i := range valuePtrs {
			valuePtrs[i] = &valueHolders[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return rowCount, err
		}

		// Format values
		var rowValues []string
		for _, val := range valueHolders {
			rowValues = append(rowValues, c.formatValueForExport(val))
		}

		values = append(values, fmt.Sprintf("(%s)", strings.Join(rowValues, ", ")))
		rowCount++

		// Write batch
		if len(values) >= batchSize {
			fmt.Fprintf(writer, "INSERT INTO %s (%s) VALUES\n%s;\n\n",
				c.QuoteIdentifier(tableName),
				strings.Join(quotedColumns, ", "),
				strings.Join(values, ",\n"))
			values = values[:0]
		}
	}

	// Write remaining rows
	if len(values) > 0 {
		fmt.Fprintf(writer, "INSERT INTO %s (%s) VALUES\n%s;\n\n",
			c.QuoteIdentifier(tableName),
			strings.Join(quotedColumns, ", "),
			strings.Join(values, ",\n"))
	}

	return rowCount, rows.Err()
}

// formatValueForExport formats a value for use in an export SQL file
func (c *Connection) formatValueForExport(val interface{}) string {
	if val == nil {
		return "NULL"
	}

	switch v := val.(type) {
	case []byte:
		s := string(v)
		// Check if it looks like binary data
		if containsBinaryData(v) {
			if c.Config.Type == DatabaseTypePostgres {
				return fmt.Sprintf("'\\x%X'", v)
			}
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
		if c.Config.Type == DatabaseTypePostgres {
			if v {
				return "true"
			}
			return "false"
		}
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

func containsBinaryData(data []byte) bool {
	for _, b := range data {
		if b < 32 && b != '\n' && b != '\r' && b != '\t' {
			return true
		}
	}
	return false
}

// ExportSQLWithCallback exports database and reports progress via callback
func (c *Connection) ExportSQLWithCallback(filePath, database string, progress func(tableName string, percent float64)) error {
	return c.ExportSQL(ExportOptions{
		FilePath:     filePath,
		Database:     database,
		AddDropTable: true,
		OnProgress: func(currentTable string, tableNum, totalTables int, _ int64) {
			if progress != nil && totalTables > 0 {
				progress(currentTable, float64(tableNum)/float64(totalTables)*100)
			}
		},
	})
}
