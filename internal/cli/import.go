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

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blubskye/yandere_sql_manager/internal/db"
	"github.com/spf13/cobra"
)

var (
	importCreateDB       bool
	importRename         string
	importBatchSize      int
	importContinue       bool
	importNoFKChecks     bool
	importNoUniqueChecks bool
	importUseNative      bool
	importJobs           int
)

var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import a SQL file into a database",
	Long: `Import a SQL file into a database.

Supports compressed files: .sql.gz, .sql.xz, .sql.zst
PostgreSQL formats: .dump, .pgdump (uses pg_restore)
Compression is auto-detected from file extension.

Examples:
  ysm import backup.sql -d mydb
  ysm import backup.sql.zst -d mydb
  ysm import backup.sql.xz -d mydb --create
  ysm import backup.sql -d olddb --rename newdb
  ysm import large_backup.sql -d mydb --batch=500
  ysm import backup.sql -d mydb --no-fk-checks

PostgreSQL native formats:
  ysm import backup.dump -d mydb --create
  ysm import backup.dump -d mydb --jobs=4
  ysm import backup.sql -d mydb --native`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", filePath)
		}

		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		// Determine target database
		targetDB := database
		if importRename != "" {
			targetDB = importRename
		}

		if targetDB == "" {
			// Try to infer from filename (strip compression extensions)
			base := filepath.Base(filePath)
			// Remove compression extensions
			for _, ext := range []string{".gz", ".xz", ".zst", ".zstd", ".gzip"} {
				base = strings.TrimSuffix(base, ext)
			}
			ext := filepath.Ext(base)
			if ext != "" {
				targetDB = base[:len(base)-len(ext)]
			} else {
				targetDB = base
			}
			fmt.Printf("No database specified, using: %s\n", targetDB)
		}

		// Detect compression
		compression := "none"
		lowerPath := strings.ToLower(filePath)
		if strings.HasSuffix(lowerPath, ".xz") {
			compression = "xz"
		} else if strings.HasSuffix(lowerPath, ".zst") || strings.HasSuffix(lowerPath, ".zstd") {
			compression = "zstd"
		} else if strings.HasSuffix(lowerPath, ".gz") || strings.HasSuffix(lowerPath, ".gzip") {
			compression = "gzip"
		}

		fmt.Printf("Importing %s into database '%s'...\n", filePath, targetDB)
		if compression != "none" {
			fmt.Printf("Compression: %s\n", compression)
		}

		startTime := time.Now()
		var lastProgress time.Time

		opts := db.ImportOptions{
			FilePath:            filePath,
			Database:            database,
			CreateDB:            importCreateDB || database == "",
			RenameDB:            importRename,
			BatchSize:           importBatchSize,
			DisableForeignKeys:  importNoFKChecks,
			DisableUniqueChecks: importNoUniqueChecks,
			UseNativeTool:       importUseNative,
			Jobs:                importJobs,
			OnProgress: func(bytesRead, totalBytes int64, stmts int64) {
				now := time.Now()
				if now.Sub(lastProgress) < 100*time.Millisecond {
					return // Rate limit progress updates
				}
				lastProgress = now

				if totalBytes > 0 {
					pct := float64(bytesRead) / float64(totalBytes) * 100
					elapsed := time.Since(startTime)
					speed := float64(bytesRead) / elapsed.Seconds() / 1024 / 1024
					fmt.Printf("\rProgress: %.1f%% | %d statements | %.1f MB/s", pct, stmts, speed)
				} else {
					// Compressed file - unknown total size
					fmt.Printf("\rStatements: %d", stmts)
				}
			},
			OnError: func(err error, stmt string) bool {
				if importContinue {
					fmt.Printf("\nWarning: %v\n", err)
					return true // Continue on error
				}
				return false // Stop on error
			},
		}

		stats, err := conn.ImportSQLWithStats(opts)
		if err != nil {
			return fmt.Errorf("import failed: %w", err)
		}

		fmt.Printf("\n\nImport completed successfully!\n")
		fmt.Printf("  Statements executed: %d\n", stats.StatementsExecuted)
		fmt.Printf("  Duration: %s\n", stats.Duration.Round(time.Millisecond))
		if stats.ErrorsEncountered > 0 {
			fmt.Printf("  Errors (skipped): %d\n", stats.ErrorsEncountered)
		}

		return nil
	},
}

func init() {
	importCmd.Flags().BoolVar(&importCreateDB, "create", false, "Create database if it doesn't exist")
	importCmd.Flags().StringVar(&importRename, "rename", "", "Rename database during import")
	importCmd.Flags().IntVar(&importBatchSize, "batch", 100, "Statements per transaction batch")
	importCmd.Flags().BoolVar(&importContinue, "continue", false, "Continue on errors")
	importCmd.Flags().BoolVar(&importNoFKChecks, "no-fk-checks", false, "Disable foreign key checks during import")
	importCmd.Flags().BoolVar(&importNoUniqueChecks, "no-unique-checks", false, "Disable unique checks during import")
	importCmd.Flags().BoolVar(&importUseNative, "native", false, "Use native tools (pg_restore/psql for PostgreSQL)")
	importCmd.Flags().IntVar(&importJobs, "jobs", 0, "Number of parallel jobs for pg_restore (PostgreSQL only)")
}
