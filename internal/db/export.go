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
	FilePath     string
	Database     string
	Tables       []string        // Empty = all tables
	NoData       bool            // Export structure only
	NoCreate     bool            // Export data only
	AddDropTable bool            // Add DROP TABLE statements
	Compression  CompressionType // Compression type (auto-detected from extension if empty)
	BufferSize   int             // Write buffer size (0 = default 64KB)
	BatchSize    int             // Rows per INSERT batch (0 = default 1000)
	OnProgress   func(currentTable string, tableNum, totalTables int, rowsExported int64)
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

	// Set defaults
	if opts.BufferSize <= 0 {
		opts.BufferSize = 64 * 1024 // 64KB buffer
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 1000 // 1000 rows per INSERT
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
	fmt.Fprintf(bufWriter, "-- Generated: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(bufWriter, "-- \"I'll never let your databases go~\"\n\n")

	fmt.Fprintf(bufWriter, "SET FOREIGN_KEY_CHECKS=0;\n")
	fmt.Fprintf(bufWriter, "SET SQL_MODE = \"NO_AUTO_VALUE_ON_ZERO\";\n")
	fmt.Fprintf(bufWriter, "SET AUTOCOMMIT = 0;\n")
	fmt.Fprintf(bufWriter, "START TRANSACTION;\n")
	fmt.Fprintf(bufWriter, "SET time_zone = \"+00:00\";\n\n")

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
		fmt.Fprintf(bufWriter, "-- Table structure for table `%s`\n", tableName)
		fmt.Fprintf(bufWriter, "-- --------------------------------------------------------\n\n")

		// Export table structure
		if !opts.NoCreate {
			if opts.AddDropTable {
				fmt.Fprintf(bufWriter, "DROP TABLE IF EXISTS `%s`;\n", tableName)
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

	fmt.Fprintf(bufWriter, "\nCOMMIT;\n")
	fmt.Fprintf(bufWriter, "SET FOREIGN_KEY_CHECKS=1;\n")

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
	var name, createStmt string
	err := c.DB.QueryRow(fmt.Sprintf("SHOW CREATE TABLE `%s`", tableName)).Scan(&name, &createStmt)
	if err != nil {
		return "", err
	}
	return createStmt, nil
}

// exportTableDataBuffered exports table data with batched INSERTs
func (c *Connection) exportTableDataBuffered(writer *bufio.Writer, tableName string, batchSize int) (int64, error) {
	rows, err := c.DB.Query(fmt.Sprintf("SELECT * FROM `%s`", tableName))
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

	// Write table comment
	fmt.Fprintf(writer, "-- Dumping data for table `%s`\n\n", tableName)

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
			rowValues = append(rowValues, formatValue(val))
		}

		values = append(values, fmt.Sprintf("(%s)", strings.Join(rowValues, ", ")))
		rowCount++

		// Write batch
		if len(values) >= batchSize {
			fmt.Fprintf(writer, "INSERT INTO `%s` (`%s`) VALUES\n%s;\n\n",
				tableName,
				strings.Join(columns, "`, `"),
				strings.Join(values, ",\n"))
			values = values[:0]
		}
	}

	// Write remaining rows
	if len(values) > 0 {
		fmt.Fprintf(writer, "INSERT INTO `%s` (`%s`) VALUES\n%s;\n\n",
			tableName,
			strings.Join(columns, "`, `"),
			strings.Join(values, ",\n"))
	}

	return rowCount, rows.Err()
}

func formatValue(val interface{}) string {
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
		return fmt.Sprintf("'%s'", escapeString(s))
	case string:
		return fmt.Sprintf("'%s'", escapeString(v))
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
		return fmt.Sprintf("'%s'", escapeString(fmt.Sprintf("%v", v)))
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

func escapeString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 10)

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\':
			b.WriteString("\\\\")
		case '\'':
			b.WriteString("\\'")
		case '"':
			b.WriteString("\\\"")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		case 0:
			b.WriteString("\\0")
		case 26: // Ctrl+Z
			b.WriteString("\\Z")
		default:
			b.WriteByte(c)
		}
	}

	return b.String()
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
