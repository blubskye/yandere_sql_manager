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
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/blubskye/yandere_sql_manager/internal/buffer"
	"github.com/blubskye/yandere_sql_manager/internal/logging"
)

// ImportOptions configures the import behavior
type ImportOptions struct {
	FilePath           string
	Database           string
	CreateDB           bool              // Create database if it doesn't exist
	RenameDB           string            // Rename database during import (empty = use original)
	BatchSize          int               // Number of statements per transaction batch (0 = auto)
	BufferSize         int               // Read buffer size in bytes (0 = default 64KB)
	OnProgress         func(bytesRead, totalBytes int64, statementsExecuted int64)
	OnError            func(err error, statement string) bool // Return true to continue, false to abort
	MaxMemory          int64             // Maximum memory for statement buffer (0 = 64MB)
	ResumeFromByte     int64             // Resume from this byte position (for interrupted imports)
	DisableForeignKeys bool              // Disable foreign key checks during import
	DisableUniqueChecks bool             // Disable unique checks during import
	SetVariables       map[string]string // Additional variables to set before import
	UseNativeTool      bool              // Use pg_restore/mysql instead of built-in import
	Jobs               int               // Number of parallel jobs for pg_restore (0 = default)
}

// ImportStats contains statistics about the import
type ImportStats struct {
	BytesRead          int64
	StatementsExecuted int64
	ErrorsEncountered  int64
	Duration           time.Duration
	Compressed         bool
	CompressionType    string
}

// ImportSQL imports a SQL file into the database with improved buffering
func (c *Connection) ImportSQL(opts ImportOptions) error {
	stats, err := c.ImportSQLWithStats(opts)
	_ = stats
	return err
}

// ImportSQLWithStats imports a SQL file and returns detailed statistics
func (c *Connection) ImportSQLWithStats(opts ImportOptions) (*ImportStats, error) {
	startTime := time.Now()
	stats := &ImportStats{}

	logging.Debug("Starting SQL import from: %s", opts.FilePath)

	// Detect if this is a PostgreSQL dump file
	ext := strings.ToLower(filepath.Ext(opts.FilePath))
	baseName := strings.ToLower(filepath.Base(opts.FilePath))

	isPgDump := ext == ".dump" || ext == ".pgdump" ||
		strings.HasSuffix(baseName, ".dump.gz") ||
		strings.HasSuffix(baseName, ".dump.xz") ||
		strings.HasSuffix(baseName, ".dump.zst")

	// Use pg_restore for PostgreSQL dump files
	if c.Config.Type == DatabaseTypePostgres && (isPgDump || opts.UseNativeTool) {
		return c.importWithPgRestore(opts)
	}

	// Get file size to determine optimal buffer size
	fileSize, _ := buffer.GetFileSize(opts.FilePath)
	logging.Debug("File size: %d bytes", fileSize)

	// Set defaults based on file size
	if opts.BufferSize <= 0 {
		opts.BufferSize = buffer.RecommendedBufferSize(fileSize)
		logging.Debug("Using auto-detected buffer size: %d bytes", opts.BufferSize)
	}
	if opts.MaxMemory <= 0 {
		opts.MaxMemory = 64 * 1024 * 1024 // 64MB max statement size
	}
	if opts.BatchSize <= 0 {
		// Larger batches for larger files
		if fileSize > 100*1024*1024 {
			opts.BatchSize = 500 // 500 statements per transaction for large files
		} else {
			opts.BatchSize = 100 // 100 statements per transaction
		}
		logging.Debug("Using batch size: %d statements", opts.BatchSize)
	}

	// Open file and detect compression
	file, err := os.Open(opts.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file size for progress
	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file stats: %w", err)
	}
	totalBytes := stat.Size()

	// Create reader based on file extension (handle compression)
	var reader io.Reader
	ext = strings.ToLower(filepath.Ext(opts.FilePath))

	// Handle double extensions like .sql.xz
	baseName = filepath.Base(opts.FilePath)
	if strings.HasSuffix(strings.ToLower(baseName), ".sql.xz") {
		ext = ".xz"
	} else if strings.HasSuffix(strings.ToLower(baseName), ".sql.gz") {
		ext = ".gz"
	} else if strings.HasSuffix(strings.ToLower(baseName), ".sql.zst") {
		ext = ".zst"
	}

	switch ext {
	case ".xz":
		stats.Compressed = true
		stats.CompressionType = "xz"
		// Use external xz command for decompression (more efficient)
		cmd := exec.Command("xz", "-dc")
		cmd.Stdin = file
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create xz pipe: %w", err)
		}
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to start xz decompression (is xz installed?): %w", err)
		}
		defer cmd.Wait()
		reader = stdout
		totalBytes = -1 // Unknown uncompressed size

	case ".zst", ".zstd":
		stats.Compressed = true
		stats.CompressionType = "zstd"
		// Use external zstd command for decompression
		cmd := exec.Command("zstd", "-dc")
		cmd.Stdin = file
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create zstd pipe: %w", err)
		}
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to start zstd decompression (is zstd installed?): %w", err)
		}
		defer cmd.Wait()
		reader = stdout
		totalBytes = -1 // Unknown uncompressed size

	case ".gz", ".gzip":
		stats.Compressed = true
		stats.CompressionType = "gzip"
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
		totalBytes = -1 // Unknown uncompressed size

	default:
		reader = file
		// Resume support for uncompressed files
		if opts.ResumeFromByte > 0 {
			if _, err := file.Seek(opts.ResumeFromByte, 0); err != nil {
				return nil, fmt.Errorf("failed to seek to resume position: %w", err)
			}
			stats.BytesRead = opts.ResumeFromByte
		}
	}

	// Wrap in buffered reader
	bufReader := bufio.NewReaderSize(reader, opts.BufferSize)

	// Determine target database
	targetDB := opts.Database
	if opts.RenameDB != "" {
		targetDB = opts.RenameDB
	}

	// Create database if requested
	if opts.CreateDB && targetDB != "" {
		if c.Config.Type == DatabaseTypePostgres {
			// PostgreSQL: Check if database exists first
			var exists bool
			c.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", targetDB).Scan(&exists)
			if !exists {
				_, err := c.DB.Exec(c.Driver.CreateDatabaseQuery(targetDB))
				if err != nil {
					return nil, fmt.Errorf("failed to create database: %w", err)
				}
			}
		} else {
			// MariaDB: Use IF NOT EXISTS
			_, err := c.DB.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", c.QuoteIdentifier(targetDB)))
			if err != nil {
				return nil, fmt.Errorf("failed to create database: %w", err)
			}
		}
	}

	// Use target database if specified
	if targetDB != "" {
		if err := c.UseDatabase(targetDB); err != nil {
			return nil, err
		}
	}

	// Apply import-specific variable settings
	var restoreVars []string
	if opts.DisableForeignKeys {
		c.DB.Exec(c.Driver.DisableForeignKeysSQL())
		restoreVars = append(restoreVars, c.Driver.EnableForeignKeysSQL())
	}
	if opts.DisableUniqueChecks {
		c.DB.Exec(c.Driver.DisableUniqueChecksSQL())
		restoreVars = append(restoreVars, c.Driver.EnableUniqueChecksSQL())
	}
	for name, value := range opts.SetVariables {
		c.SetVariable(name, value, false)
	}
	// Defer restore of variables
	defer func() {
		for _, stmt := range restoreVars {
			c.DB.Exec(stmt)
		}
	}()

	// Process SQL statements with batched transactions
	var bytesRead atomic.Int64
	bytesRead.Store(stats.BytesRead)

	parser := newSQLParser(bufReader, opts.MaxMemory)
	var batch []string
	var statementsExecuted int64

	for {
		stmt, n, err := parser.NextStatement()
		bytesRead.Add(int64(n))

		if err == io.EOF {
			break
		}
		if err != nil {
			return stats, fmt.Errorf("failed to parse SQL: %w", err)
		}

		stmt = strings.TrimSpace(stmt)
		if stmt == "" || stmt == ";" {
			continue
		}

		// Skip statements when renaming database
		if opts.RenameDB != "" {
			upperStmt := strings.ToUpper(stmt)
			if strings.Contains(upperStmt, "CREATE DATABASE") ||
				strings.HasPrefix(upperStmt, "USE ") {
				continue
			}
		}

		batch = append(batch, stmt)

		// Execute batch
		if len(batch) >= opts.BatchSize {
			if err := c.executeBatch(batch); err != nil {
				if opts.OnError != nil && opts.OnError(err, batch[len(batch)-1]) {
					stats.ErrorsEncountered++
					batch = batch[:0]
					continue
				}
				return stats, err
			}
			statementsExecuted += int64(len(batch))
			batch = batch[:0]

			// Report progress
			if opts.OnProgress != nil {
				opts.OnProgress(bytesRead.Load(), totalBytes, statementsExecuted)
			}
		}
	}

	// Execute remaining batch
	if len(batch) > 0 {
		if err := c.executeBatch(batch); err != nil {
			if opts.OnError == nil || !opts.OnError(err, batch[len(batch)-1]) {
				return stats, err
			}
			stats.ErrorsEncountered++
		} else {
			statementsExecuted += int64(len(batch))
		}
	}

	stats.BytesRead = bytesRead.Load()
	stats.StatementsExecuted = statementsExecuted
	stats.Duration = time.Since(startTime)

	return stats, nil
}

// executeBatch executes a batch of statements in a transaction
func (c *Connection) executeBatch(statements []string) error {
	ctx := context.Background()
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute statement: %w\nSQL: %s", err, truncateSQL(stmt))
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// sqlParser handles streaming SQL parsing with minimal memory usage
type sqlParser struct {
	reader    *bufio.Reader
	buffer    strings.Builder
	maxSize   int64
	inString  bool
	stringCh  byte
	escaped   bool
}

func newSQLParser(r *bufio.Reader, maxSize int64) *sqlParser {
	return &sqlParser{
		reader:  r,
		maxSize: maxSize,
	}
}

// NextStatement returns the next complete SQL statement
func (p *sqlParser) NextStatement() (string, int, error) {
	p.buffer.Reset()
	bytesRead := 0

	for {
		b, err := p.reader.ReadByte()
		if err != nil {
			if err == io.EOF && p.buffer.Len() > 0 {
				return p.buffer.String(), bytesRead, nil
			}
			return "", bytesRead, err
		}
		bytesRead++

		// Check max size
		if int64(p.buffer.Len()) > p.maxSize {
			return "", bytesRead, fmt.Errorf("statement exceeds maximum size of %d bytes", p.maxSize)
		}

		// Handle escape sequences
		if p.escaped {
			p.buffer.WriteByte(b)
			p.escaped = false
			continue
		}

		if b == '\\' && p.inString {
			p.buffer.WriteByte(b)
			p.escaped = true
			continue
		}

		// Handle string literals
		if p.inString {
			p.buffer.WriteByte(b)
			if b == p.stringCh {
				p.inString = false
			}
			continue
		}

		// Check for string start
		if b == '\'' || b == '"' || b == '`' {
			p.inString = true
			p.stringCh = b
			p.buffer.WriteByte(b)
			continue
		}

		// Check for comments
		if b == '-' {
			next, err := p.reader.Peek(1)
			if err == nil && len(next) > 0 && next[0] == '-' {
				// Skip until end of line
				for {
					c, err := p.reader.ReadByte()
					bytesRead++
					if err != nil || c == '\n' {
						break
					}
				}
				continue
			}
		}

		if b == '#' {
			// Skip until end of line
			for {
				c, err := p.reader.ReadByte()
				bytesRead++
				if err != nil || c == '\n' {
					break
				}
			}
			continue
		}

		// Check for block comments
		if b == '/' {
			next, err := p.reader.Peek(1)
			if err == nil && len(next) > 0 && next[0] == '*' {
				p.reader.ReadByte() // consume *
				bytesRead++
				// Skip until */
				for {
					c, err := p.reader.ReadByte()
					bytesRead++
					if err != nil {
						break
					}
					if c == '*' {
						next, _ := p.reader.Peek(1)
						if len(next) > 0 && next[0] == '/' {
							p.reader.ReadByte()
							bytesRead++
							break
						}
					}
				}
				continue
			}
		}

		p.buffer.WriteByte(b)

		// Check for statement terminator
		if b == ';' {
			return p.buffer.String(), bytesRead, nil
		}
	}
}

func truncateSQL(sql string) string {
	if len(sql) > 200 {
		return sql[:200] + "..."
	}
	return sql
}

// ImportSQLWithCallback imports SQL and reports progress via callback
func (c *Connection) ImportSQLWithCallback(filePath, database string, progress func(percent float64)) error {
	return c.ImportSQL(ImportOptions{
		FilePath: filePath,
		Database: database,
		CreateDB: true,
		OnProgress: func(bytesRead, totalBytes int64, _ int64) {
			if totalBytes > 0 && progress != nil {
				progress(float64(bytesRead) / float64(totalBytes) * 100)
			}
		},
	})
}

// importWithPgRestore imports a PostgreSQL dump using pg_restore
func (c *Connection) importWithPgRestore(opts ImportOptions) (*ImportStats, error) {
	startTime := time.Now()

	logging.Debug("Using pg_restore for import: %s", opts.FilePath)

	// Determine target database
	targetDB := opts.Database
	if opts.RenameDB != "" {
		targetDB = opts.RenameDB
	}
	if targetDB == "" {
		targetDB = c.Config.Database
	}

	// Create database if requested
	if opts.CreateDB && targetDB != "" {
		var exists bool
		c.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", targetDB).Scan(&exists)
		if !exists {
			logging.Debug("Creating database: %s", targetDB)
			_, err := c.DB.Exec(c.Driver.CreateDatabaseQuery(targetDB))
			if err != nil {
				return nil, fmt.Errorf("failed to create database: %w", err)
			}
		}
	}

	// Check if this is a plain SQL file or a custom format dump
	ext := strings.ToLower(filepath.Ext(opts.FilePath))
	baseName := strings.ToLower(filepath.Base(opts.FilePath))

	// Detect file format by checking magic bytes
	isCustomFormat := false
	if file, err := os.Open(opts.FilePath); err == nil {
		header := make([]byte, 5)
		if n, _ := file.Read(header); n >= 5 {
			// PostgreSQL custom format starts with "PGDMP"
			if string(header) == "PGDMP" {
				isCustomFormat = true
			}
		}
		file.Close()
	}

	// Also check file extension
	if ext == ".dump" || ext == ".pgdump" ||
		strings.HasSuffix(baseName, ".dump.gz") ||
		strings.HasSuffix(baseName, ".dump.xz") ||
		strings.HasSuffix(baseName, ".dump.zst") {
		isCustomFormat = true
	}

	if isCustomFormat {
		// Use pg_restore for custom format
		return c.runPgRestore(opts, targetDB, startTime)
	}

	// For plain SQL files, use psql
	return c.runPsql(opts, targetDB, startTime)
}

// runPgRestore runs pg_restore for custom format dumps
func (c *Connection) runPgRestore(opts ImportOptions, targetDB string, startTime time.Time) (*ImportStats, error) {
	stats := &ImportStats{}

	args := []string{
		"-h", c.Config.Host,
		"-p", fmt.Sprintf("%d", c.Config.Port),
		"-U", c.Config.User,
		"-d", targetDB,
	}

	// Add options
	if opts.DisableForeignKeys {
		args = append(args, "--disable-triggers")
	}

	// Add parallel jobs
	if opts.Jobs > 0 {
		args = append(args, "-j", fmt.Sprintf("%d", opts.Jobs))
	}

	// Clean/drop objects before restore
	if opts.CreateDB {
		args = append(args, "--clean", "--if-exists")
	}

	// Add the file to restore
	args = append(args, opts.FilePath)

	cmd := exec.Command("pg_restore", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", c.Config.Password))

	logging.Debug("Running: pg_restore %v", args)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// pg_restore returns non-zero for warnings too, check if critical
		if !strings.Contains(string(output), "errors ignored") {
			return nil, fmt.Errorf("pg_restore failed: %w\nOutput: %s", err, string(output))
		}
		logging.Warn("pg_restore completed with warnings: %s", string(output))
	}

	// Get file size
	if info, err := os.Stat(opts.FilePath); err == nil {
		stats.BytesRead = info.Size()
	}

	stats.Duration = time.Since(startTime)
	logging.Info("pg_restore import completed")

	return stats, nil
}

// runPsql runs psql for plain SQL imports
func (c *Connection) runPsql(opts ImportOptions, targetDB string, startTime time.Time) (*ImportStats, error) {
	stats := &ImportStats{}

	args := []string{
		"-h", c.Config.Host,
		"-p", fmt.Sprintf("%d", c.Config.Port),
		"-U", c.Config.User,
		"-d", targetDB,
		"-f", opts.FilePath,
	}

	// Handle compressed files
	ext := strings.ToLower(filepath.Ext(opts.FilePath))
	baseName := strings.ToLower(filepath.Base(opts.FilePath))

	var cmd *exec.Cmd

	if strings.HasSuffix(baseName, ".sql.gz") || ext == ".gz" {
		// Pipe through gunzip
		gzipCmd := exec.Command("gunzip", "-c", opts.FilePath)
		psqlCmd := exec.Command("psql",
			"-h", c.Config.Host,
			"-p", fmt.Sprintf("%d", c.Config.Port),
			"-U", c.Config.User,
			"-d", targetDB,
		)
		psqlCmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", c.Config.Password))

		pipe, err := gzipCmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create pipe: %w", err)
		}
		psqlCmd.Stdin = pipe

		if err := gzipCmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to start gunzip: %w", err)
		}

		output, err := psqlCmd.CombinedOutput()
		gzipCmd.Wait()

		if err != nil {
			return nil, fmt.Errorf("psql failed: %w\nOutput: %s", err, string(output))
		}
	} else if strings.HasSuffix(baseName, ".sql.xz") || ext == ".xz" {
		// Pipe through xz
		xzCmd := exec.Command("xz", "-dc", opts.FilePath)
		psqlCmd := exec.Command("psql",
			"-h", c.Config.Host,
			"-p", fmt.Sprintf("%d", c.Config.Port),
			"-U", c.Config.User,
			"-d", targetDB,
		)
		psqlCmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", c.Config.Password))

		pipe, err := xzCmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create pipe: %w", err)
		}
		psqlCmd.Stdin = pipe

		if err := xzCmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to start xz: %w", err)
		}

		output, err := psqlCmd.CombinedOutput()
		xzCmd.Wait()

		if err != nil {
			return nil, fmt.Errorf("psql failed: %w\nOutput: %s", err, string(output))
		}
	} else if strings.HasSuffix(baseName, ".sql.zst") || ext == ".zst" {
		// Pipe through zstd
		zstdCmd := exec.Command("zstd", "-dc", opts.FilePath)
		psqlCmd := exec.Command("psql",
			"-h", c.Config.Host,
			"-p", fmt.Sprintf("%d", c.Config.Port),
			"-U", c.Config.User,
			"-d", targetDB,
		)
		psqlCmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", c.Config.Password))

		pipe, err := zstdCmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create pipe: %w", err)
		}
		psqlCmd.Stdin = pipe

		if err := zstdCmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to start zstd: %w", err)
		}

		output, err := psqlCmd.CombinedOutput()
		zstdCmd.Wait()

		if err != nil {
			return nil, fmt.Errorf("psql failed: %w\nOutput: %s", err, string(output))
		}
	} else {
		// Plain SQL file
		cmd = exec.Command("psql", args...)
		cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", c.Config.Password))

		logging.Debug("Running: psql %v", args)

		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("psql failed: %w\nOutput: %s", err, string(output))
		}
	}

	// Get file size
	if info, err := os.Stat(opts.FilePath); err == nil {
		stats.BytesRead = info.Size()
	}

	stats.Duration = time.Since(startTime)
	logging.Info("psql import completed")

	return stats, nil
}
