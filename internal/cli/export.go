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
	"path/filepath"
	"strings"
	"time"

	"github.com/blubskye/yandere_sql_manager/internal/db"
	"github.com/spf13/cobra"
)

var (
	exportOutput      string
	exportNoData      bool
	exportNoCreate    bool
	exportAddDrop     bool
	exportTables      []string
	exportCompress    string
	exportBatchSize   int
	exportIncludeVars bool
)

var exportCmd = &cobra.Command{
	Use:   "export <database>",
	Short: "Export a database to a SQL file",
	Long: `Export a database to a SQL file.

Supports compression: gzip (.gz), xz (.xz), zstd (.zst)
Compression is auto-detected from output filename or can be specified with --compress.

Examples:
  ysm export mydb
  ysm export mydb -o backup.sql
  ysm export mydb -o backup.sql.zst
  ysm export mydb -o backup.sql.xz --compress=xz
  ysm export mydb --no-data
  ysm export mydb --tables users,posts
  ysm export mydb --include-vars`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dbName := args[0]

		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		// Determine output file
		output := exportOutput
		if output == "" {
			timestamp := time.Now().Format("20060102_150405")
			output = fmt.Sprintf("%s_%s.sql", dbName, timestamp)
		}

		// Make path absolute if relative
		if !filepath.IsAbs(output) {
			absPath, err := filepath.Abs(output)
			if err == nil {
				output = absPath
			}
		}

		// Determine compression
		var compression db.CompressionType
		if exportCompress != "" {
			switch strings.ToLower(exportCompress) {
			case "gzip", "gz":
				compression = db.CompressionGzip
			case "xz":
				compression = db.CompressionXZ
			case "zstd", "zst":
				compression = db.CompressionZstd
			case "none", "":
				compression = db.CompressionNone
			default:
				return fmt.Errorf("unknown compression type: %s (use: gzip, xz, zstd, none)", exportCompress)
			}
		}

		// Show compression info
		compressionName := "none"
		if compression != "" {
			compressionName = string(compression)
		} else {
			// Auto-detect from filename
			ext := strings.ToLower(filepath.Ext(output))
			switch ext {
			case ".xz":
				compressionName = "xz"
			case ".zst", ".zstd":
				compressionName = "zstd"
			case ".gz", ".gzip":
				compressionName = "gzip"
			}
		}

		fmt.Printf("Exporting database '%s' to %s\n", dbName, output)
		fmt.Printf("Compression: %s\n\n", compressionName)

		opts := db.ExportOptions{
			FilePath:     output,
			Database:     dbName,
			Tables:       exportTables,
			NoData:       exportNoData,
			NoCreate:     exportNoCreate,
			AddDropTable: exportAddDrop,
			Compression:  compression,
			BatchSize:    exportBatchSize,
			IncludeVars:  exportIncludeVars,
			OnProgress: func(currentTable string, tableNum, totalTables int, rowsExported int64) {
				fmt.Printf("\r[%d/%d] Exporting: %-40s (%d rows)", tableNum, totalTables, currentTable, rowsExported)
			},
		}

		stats, err := conn.ExportSQLWithStats(opts)
		if err != nil {
			return fmt.Errorf("export failed: %w", err)
		}

		fmt.Printf("\n\nExport completed successfully!\n")
		fmt.Printf("  Tables exported: %d\n", stats.TablesExported)
		fmt.Printf("  Rows exported: %d\n", stats.RowsExported)
		fmt.Printf("  File size: %s\n", formatSize(stats.BytesWritten))
		fmt.Printf("  Duration: %s\n", stats.Duration.Round(time.Millisecond))
		fmt.Printf("  Output: %s\n", output)

		// Calculate compression ratio if we can
		if stats.Compressed && stats.RowsExported > 0 {
			speed := float64(stats.RowsExported) / stats.Duration.Seconds()
			fmt.Printf("  Speed: %.0f rows/sec\n", speed)
		}

		return nil
	},
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func init() {
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file (default: <db>_<timestamp>.sql)")
	exportCmd.Flags().BoolVar(&exportNoData, "no-data", false, "Export structure only, no data")
	exportCmd.Flags().BoolVar(&exportNoCreate, "no-create", false, "Export data only, no CREATE statements")
	exportCmd.Flags().BoolVar(&exportAddDrop, "add-drop", true, "Add DROP TABLE statements")
	exportCmd.Flags().StringSliceVar(&exportTables, "tables", nil, "Export only specific tables (comma-separated)")
	exportCmd.Flags().StringVar(&exportCompress, "compress", "", "Compression: gzip, xz, zstd, none (auto-detect from filename)")
	exportCmd.Flags().IntVar(&exportBatchSize, "batch", 1000, "Rows per INSERT batch")
	exportCmd.Flags().BoolVar(&exportIncludeVars, "include-vars", false, "Include session variable SET statements in export")
}
