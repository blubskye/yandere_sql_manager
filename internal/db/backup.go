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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blubskye/yandere_sql_manager/internal/logging"
)

// BackupMetadata contains information about a backup
type BackupMetadata struct {
	ID            string          `json:"id"`
	Timestamp     time.Time       `json:"timestamp"`
	Databases     []string        `json:"databases"`
	Files         []BackupFile    `json:"files"`
	TotalSize     int64           `json:"total_size"`
	Compression   CompressionType `json:"compression"`
	ServerVersion string          `json:"server_version"`
	ServerType    DatabaseType    `json:"server_type"`
	Profile       string          `json:"profile,omitempty"`
	Description   string          `json:"description,omitempty"`
}

// BackupFile represents a single backup file
type BackupFile struct {
	Database string `json:"database"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	Tables   int    `json:"tables"`
	Rows     int64  `json:"rows"`
}

// BackupOptions configures backup creation
type BackupOptions struct {
	OutputDir     string          // Directory to store backups
	Databases     []string        // Databases to backup (empty = all)
	Compression   CompressionType // Compression type
	Description   string          // Optional description
	Profile       string          // Optional profile name
	Parallel      int             // Number of parallel workers (0 = sequential, -1 = auto)
	OnProgress    func(database string, dbNum, totalDBs int)
}

// RestoreOptions configures backup restoration
type RestoreOptions struct {
	BackupID           string            // Backup ID to restore
	BackupPath         string            // Or direct path to backup file
	Databases          []string          // Specific databases to restore (empty = all)
	RenameMap          map[string]string // Rename databases during restore (original -> new)
	DropExisting       bool              // Drop existing databases before restore
	CreateIfNotExists  bool              // Create databases if they don't exist
	DisableForeignKeys bool              // Disable FK checks during restore
	OnProgress         func(database string, dbNum, totalDBs int, percent float64)
}

// GetBackupsDir returns the default backups directory
func GetBackupsDir() (string, error) {
	// Use XDG_DATA_HOME or fallback to ~/.local/share
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		dataHome = filepath.Join(home, ".local", "share")
	}

	backupsDir := filepath.Join(dataHome, "ysm", "backups")
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backups directory: %w", err)
	}

	return backupsDir, nil
}

// CreateBackup creates a backup of one or more databases
func (c *Connection) CreateBackup(opts BackupOptions) (*BackupMetadata, error) {
	logging.Debug("Starting backup creation")
	logging.Debug("Compression: %s, Databases: %v", opts.Compression, opts.Databases)

	// Set up output directory
	outputDir := opts.OutputDir
	if outputDir == "" {
		var err error
		outputDir, err = GetBackupsDir()
		if err != nil {
			return nil, err
		}
	}
	logging.Debug("Output directory: %s", outputDir)

	// Get list of databases to backup
	databases := opts.Databases
	if len(databases) == 0 {
		dbList, err := c.ListDatabases()
		if err != nil {
			return nil, fmt.Errorf("failed to list databases: %w", err)
		}
		for _, db := range dbList {
			// Skip system databases
			if isSystemDatabase(db.Name, c.Config.Type) {
				continue
			}
			databases = append(databases, db.Name)
		}
	}

	if len(databases) == 0 {
		return nil, fmt.Errorf("no databases to backup")
	}

	// Get server version
	serverVersion := ""
	if v, err := c.GetServerVersion(); err == nil {
		serverVersion = v
	}

	// Create backup metadata
	backupID := generateBackupID()
	backupDir := filepath.Join(outputDir, backupID)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	metadata := &BackupMetadata{
		ID:            backupID,
		Timestamp:     time.Now(),
		Databases:     databases,
		Files:         []BackupFile{},
		Compression:   opts.Compression,
		ServerVersion: serverVersion,
		ServerType:    c.Config.Type,
		Profile:       opts.Profile,
		Description:   opts.Description,
	}

	// Determine file extension
	ext := ".sql"
	switch opts.Compression {
	case CompressionGzip:
		ext = ".sql.gz"
	case CompressionXZ:
		ext = ".sql.xz"
	case CompressionZstd:
		ext = ".sql.zst"
	}

	// Determine parallelism
	parallelWorkers := opts.Parallel
	if parallelWorkers < 0 {
		parallelWorkers = runtime.NumCPU()
	}
	if parallelWorkers > len(databases) {
		parallelWorkers = len(databases)
	}

	var totalSize int64

	if parallelWorkers > 1 {
		// Parallel backup
		logging.Info("Starting parallel backup of %d databases with %d workers", len(databases), parallelWorkers)

		type backupResult struct {
			index    int
			database string
			file     BackupFile
			err      error
		}

		resultsChan := make(chan backupResult, len(databases))
		var wg sync.WaitGroup
		sem := make(chan struct{}, parallelWorkers) // Semaphore for limiting concurrency
		var completed atomic.Int64

		// Launch backup goroutines
		for i, dbName := range databases {
			wg.Add(1)
			go func(idx int, db string) {
				defer wg.Done()
				sem <- struct{}{}        // Acquire semaphore
				defer func() { <-sem }() // Release semaphore

				filename := fmt.Sprintf("%s%s", db, ext)
				filePath := filepath.Join(backupDir, filename)

				exportOpts := ExportOptions{
					FilePath:     filePath,
					Database:     db,
					AddDropTable: true,
					Compression:  opts.Compression,
				}

				stats, err := c.ExportSQLWithStats(exportOpts)
				if err != nil {
					resultsChan <- backupResult{
						index:    idx,
						database: db,
						err:      fmt.Errorf("failed to backup database %s: %w", db, err),
					}
					return
				}

				// Get file size
				fileInfo, err := os.Stat(filePath)
				if err != nil {
					resultsChan <- backupResult{
						index:    idx,
						database: db,
						err:      fmt.Errorf("failed to get file info for %s: %w", filename, err),
					}
					return
				}

				comp := completed.Add(1)
				if opts.OnProgress != nil {
					opts.OnProgress(db, int(comp), len(databases))
				}

				resultsChan <- backupResult{
					index:    idx,
					database: db,
					file: BackupFile{
						Database: db,
						Filename: filename,
						Size:     fileInfo.Size(),
						Tables:   stats.TablesExported,
						Rows:     stats.RowsExported,
					},
				}
			}(i, dbName)
		}

		// Wait for all goroutines and close results channel
		go func() {
			wg.Wait()
			close(resultsChan)
		}()

		// Collect results
		results := make([]backupResult, len(databases))
		var firstError error
		for result := range resultsChan {
			results[result.index] = result
			if result.err != nil && firstError == nil {
				firstError = result.err
			}
		}

		// Check for errors
		if firstError != nil {
			os.RemoveAll(backupDir)
			return nil, firstError
		}

		// Build metadata from ordered results
		for _, result := range results {
			metadata.Files = append(metadata.Files, result.file)
			totalSize += result.file.Size
		}

		logging.Info("Parallel backup completed: %d databases backed up", len(databases))

	} else {
		// Sequential backup (original logic)
		for i, dbName := range databases {
			if opts.OnProgress != nil {
				opts.OnProgress(dbName, i+1, len(databases))
			}

			filename := fmt.Sprintf("%s%s", dbName, ext)
			filePath := filepath.Join(backupDir, filename)

			exportOpts := ExportOptions{
				FilePath:     filePath,
				Database:     dbName,
				AddDropTable: true,
				Compression:  opts.Compression,
			}

			stats, err := c.ExportSQLWithStats(exportOpts)
			if err != nil {
				// Clean up partial backup on error
				os.RemoveAll(backupDir)
				return nil, fmt.Errorf("failed to backup database %s: %w", dbName, err)
			}

			// Get file size
			fileInfo, err := os.Stat(filePath)
			if err != nil {
				os.RemoveAll(backupDir)
				return nil, fmt.Errorf("failed to get file info for %s: %w", filename, err)
			}

			metadata.Files = append(metadata.Files, BackupFile{
				Database: dbName,
				Filename: filename,
				Size:     fileInfo.Size(),
				Tables:   stats.TablesExported,
				Rows:     stats.RowsExported,
			})

			totalSize += fileInfo.Size()
		}
	}

	metadata.TotalSize = totalSize

	// Save metadata
	metadataPath := filepath.Join(backupDir, "metadata.json")
	metadataData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		os.RemoveAll(backupDir)
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metadataPath, metadataData, 0644); err != nil {
		os.RemoveAll(backupDir)
		return nil, fmt.Errorf("failed to write metadata: %w", err)
	}

	return metadata, nil
}

// RestoreBackup restores a backup
func (c *Connection) RestoreBackup(opts RestoreOptions) error {
	logging.Debug("Starting backup restore")
	logging.Debug("BackupID: %s, BackupPath: %s", opts.BackupID, opts.BackupPath)

	// Find backup
	var backupDir string
	var metadata *BackupMetadata

	if opts.BackupID != "" {
		backupsDir, err := GetBackupsDir()
		if err != nil {
			return err
		}
		backupDir = filepath.Join(backupsDir, opts.BackupID)
	} else if opts.BackupPath != "" {
		backupDir = opts.BackupPath
	} else {
		return fmt.Errorf("backup ID or path is required")
	}
	logging.Debug("Backup directory: %s", backupDir)

	// Load metadata
	metadataPath := filepath.Join(backupDir, "metadata.json")
	metadataData, err := os.ReadFile(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to read backup metadata: %w", err)
	}

	metadata = &BackupMetadata{}
	if err := json.Unmarshal(metadataData, metadata); err != nil {
		return fmt.Errorf("failed to parse backup metadata: %w", err)
	}

	// Determine which databases to restore
	databasesToRestore := opts.Databases
	if len(databasesToRestore) == 0 {
		databasesToRestore = metadata.Databases
	}

	// Restore each database
	for i, dbName := range databasesToRestore {
		// Find corresponding backup file
		var backupFile *BackupFile
		for _, f := range metadata.Files {
			if f.Database == dbName {
				backupFile = &f
				break
			}
		}

		if backupFile == nil {
			return fmt.Errorf("database %s not found in backup", dbName)
		}

		// Determine target database name
		targetDB := dbName
		if rename, ok := opts.RenameMap[dbName]; ok {
			targetDB = rename
		}

		if opts.OnProgress != nil {
			opts.OnProgress(dbName, i+1, len(databasesToRestore), 0)
		}

		// Drop existing if requested
		if opts.DropExisting {
			// Check if database exists
			databases, err := c.ListDatabases()
			if err == nil {
				for _, d := range databases {
					if d.Name == targetDB {
						if _, err := c.DB.Exec(c.Driver.DropDatabaseQuery(targetDB)); err != nil {
							return fmt.Errorf("failed to drop existing database %s: %w", targetDB, err)
						}
						break
					}
				}
			}
		}

		// Import the backup
		filePath := filepath.Join(backupDir, backupFile.Filename)
		importOpts := ImportOptions{
			FilePath:           filePath,
			Database:           targetDB,
			CreateDB:           opts.CreateIfNotExists,
			DisableForeignKeys: opts.DisableForeignKeys,
			OnProgress: func(bytesRead, totalBytes int64, _ int64) {
				if opts.OnProgress != nil && totalBytes > 0 {
					percent := float64(bytesRead) / float64(totalBytes) * 100
					opts.OnProgress(dbName, i+1, len(databasesToRestore), percent)
				}
			},
		}

		if err := c.ImportSQL(importOpts); err != nil {
			return fmt.Errorf("failed to restore database %s: %w", dbName, err)
		}
	}

	return nil
}

// ListBackups returns all available backups
func ListBackups() ([]BackupMetadata, error) {
	backupsDir, err := GetBackupsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupMetadata{}, nil
		}
		return nil, fmt.Errorf("failed to read backups directory: %w", err)
	}

	var backups []BackupMetadata
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		metadataPath := filepath.Join(backupsDir, entry.Name(), "metadata.json")
		metadataData, err := os.ReadFile(metadataPath)
		if err != nil {
			continue // Skip invalid backup directories
		}

		var metadata BackupMetadata
		if err := json.Unmarshal(metadataData, &metadata); err != nil {
			continue
		}

		backups = append(backups, metadata)
	}

	// Sort by timestamp, newest first
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})

	return backups, nil
}

// GetBackup returns metadata for a specific backup
func GetBackup(id string) (*BackupMetadata, error) {
	backupsDir, err := GetBackupsDir()
	if err != nil {
		return nil, err
	}

	metadataPath := filepath.Join(backupsDir, id, "metadata.json")
	metadataData, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("backup not found: %w", err)
	}

	var metadata BackupMetadata
	if err := json.Unmarshal(metadataData, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse backup metadata: %w", err)
	}

	return &metadata, nil
}

// DeleteBackup removes a backup
func DeleteBackup(id string) error {
	backupsDir, err := GetBackupsDir()
	if err != nil {
		return err
	}

	backupDir := filepath.Join(backupsDir, id)
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return fmt.Errorf("backup not found: %s", id)
	}

	if err := os.RemoveAll(backupDir); err != nil {
		return fmt.Errorf("failed to delete backup: %w", err)
	}

	return nil
}

// GetServerVersion returns the database server version
func (c *Connection) GetServerVersion() (string, error) {
	var version string
	err := c.DB.QueryRow(c.Driver.ServerVersionQuery()).Scan(&version)
	if err != nil {
		return "", err
	}
	return version, nil
}

// Helper functions

func generateBackupID() string {
	return time.Now().Format("20060102-150405")
}

func isSystemDatabase(name string, dbType DatabaseType) bool {
	name = strings.ToLower(name)

	if dbType == DatabaseTypePostgres {
		return name == "postgres" || name == "template0" || name == "template1"
	}

	// MariaDB/MySQL system databases
	systemDBs := []string{
		"information_schema",
		"mysql",
		"performance_schema",
		"sys",
	}

	for _, sysDB := range systemDBs {
		if name == sysDB {
			return true
		}
	}

	return false
}

// FormatSize formats bytes into human-readable size
func FormatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
