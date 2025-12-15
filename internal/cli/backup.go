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
	"strings"
	"text/tabwriter"

	"github.com/blubskye/yandere_sql_manager/internal/db"
	"github.com/spf13/cobra"
)

var (
	backupOutputDir   string
	backupCompression string
	backupDescription string
	backupParallel    int
	restoreDropExist  bool
	restoreRename     []string
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup and restore databases",
	Long: `Create and manage database backups.

Subcommands:
  create  - Create a new backup
  list    - List all backups
  show    - Show backup details
  restore - Restore a backup
  delete  - Delete a backup`,
}

var backupCreateCmd = &cobra.Command{
	Use:   "create [databases...]",
	Short: "Create a new backup",
	Long: `Create a backup of one or more databases.

Examples:
  ysm backup create                           # Backup all databases
  ysm backup create mydb1 mydb2               # Backup specific databases
  ysm backup create --compress zstd           # Use zstd compression
  ysm backup create -o /path/to/backups       # Custom output directory
  ysm backup create --parallel 4              # Backup 4 databases in parallel
  ysm backup create --parallel -1             # Auto-detect parallelism (CPU count)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		compression := db.CompressionNone
		switch strings.ToLower(backupCompression) {
		case "gzip", "gz":
			compression = db.CompressionGzip
		case "xz":
			compression = db.CompressionXZ
		case "zstd", "zst":
			compression = db.CompressionZstd
		}

		opts := db.BackupOptions{
			OutputDir:   backupOutputDir,
			Databases:   args,
			Compression: compression,
			Description: backupDescription,
			Profile:     profile,
			Parallel:    backupParallel,
			OnProgress: func(database string, dbNum, totalDBs int) {
				fmt.Printf("Backing up %s (%d/%d)...\n", database, dbNum, totalDBs)
			},
		}

		metadata, err := conn.CreateBackup(opts)
		if err != nil {
			return err
		}

		fmt.Println()
		fmt.Printf("Backup created successfully!\n")
		fmt.Printf("  ID:        %s\n", metadata.ID)
		fmt.Printf("  Databases: %d\n", len(metadata.Databases))
		fmt.Printf("  Size:      %s\n", db.FormatSize(metadata.TotalSize))
		if metadata.Compression != "" {
			fmt.Printf("  Compressed: %s\n", metadata.Compression)
		}

		return nil
	},
}

var backupListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all backups",
	RunE: func(cmd *cobra.Command, args []string) error {
		backups, err := db.ListBackups()
		if err != nil {
			return err
		}

		if len(backups) == 0 {
			fmt.Println("No backups found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tDATE\tDATABASES\tSIZE\tCOMPRESSION")
		fmt.Fprintln(w, "--\t----\t---------\t----\t-----------")

		for _, b := range backups {
			compression := "-"
			if b.Compression != "" {
				compression = string(b.Compression)
			}
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
				b.ID,
				b.Timestamp.Format("2006-01-02 15:04"),
				len(b.Databases),
				db.FormatSize(b.TotalSize),
				compression,
			)
		}
		return w.Flush()
	},
}

var backupShowCmd = &cobra.Command{
	Use:   "show <backup-id>",
	Short: "Show backup details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		metadata, err := db.GetBackup(args[0])
		if err != nil {
			return err
		}

		fmt.Printf("Backup: %s\n", metadata.ID)
		fmt.Printf("  Timestamp:      %s\n", metadata.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Server Type:    %s\n", metadata.ServerType)
		fmt.Printf("  Server Version: %s\n", metadata.ServerVersion)
		fmt.Printf("  Total Size:     %s\n", db.FormatSize(metadata.TotalSize))
		if metadata.Compression != "" {
			fmt.Printf("  Compression:    %s\n", metadata.Compression)
		}
		if metadata.Description != "" {
			fmt.Printf("  Description:    %s\n", metadata.Description)
		}
		if metadata.Profile != "" {
			fmt.Printf("  Profile:        %s\n", metadata.Profile)
		}

		fmt.Println()
		fmt.Println("Databases:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "  NAME\tTABLES\tROWS\tSIZE")
		fmt.Fprintln(w, "  ----\t------\t----\t----")

		for _, f := range metadata.Files {
			fmt.Fprintf(w, "  %s\t%d\t%d\t%s\n",
				f.Database,
				f.Tables,
				f.Rows,
				db.FormatSize(f.Size),
			)
		}
		return w.Flush()
	},
}

var backupRestoreCmd = &cobra.Command{
	Use:   "restore <backup-id> [databases...]",
	Short: "Restore a backup",
	Long: `Restore a backup to the database server.

Examples:
  ysm backup restore 20240101-120000              # Restore all databases
  ysm backup restore 20240101-120000 mydb         # Restore specific database
  ysm backup restore 20240101-120000 --drop       # Drop existing before restore
  ysm backup restore 20240101-120000 --rename old:new  # Rename during restore`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		backupID := args[0]
		databases := args[1:]

		// Parse rename map
		renameMap := make(map[string]string)
		for _, r := range restoreRename {
			parts := strings.SplitN(r, ":", 2)
			if len(parts) == 2 {
				renameMap[parts[0]] = parts[1]
			}
		}

		// Confirm if dropping existing
		if restoreDropExist {
			fmt.Printf("WARNING: This will DROP existing databases before restoring.\n")
			fmt.Printf("Are you sure you want to continue? [y/N]: ")
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) != "y" && strings.ToLower(confirm) != "yes" {
				fmt.Println("Aborted.")
				return nil
			}
		}

		opts := db.RestoreOptions{
			BackupID:           backupID,
			Databases:          databases,
			RenameMap:          renameMap,
			DropExisting:       restoreDropExist,
			CreateIfNotExists:  true,
			DisableForeignKeys: true,
			OnProgress: func(database string, dbNum, totalDBs int, percent float64) {
				if percent > 0 {
					fmt.Printf("\rRestoring %s (%d/%d): %.0f%%", database, dbNum, totalDBs, percent)
				} else {
					fmt.Printf("Restoring %s (%d/%d)...\n", database, dbNum, totalDBs)
				}
			},
		}

		if err := conn.RestoreBackup(opts); err != nil {
			return err
		}

		fmt.Println()
		fmt.Println("Restore completed successfully!")
		return nil
	},
}

var backupDeleteCmd = &cobra.Command{
	Use:   "delete <backup-id>",
	Short: "Delete a backup",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		backupID := args[0]

		// Confirm deletion
		fmt.Printf("Are you sure you want to delete backup '%s'? [y/N]: ", backupID)
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(confirm) != "y" && strings.ToLower(confirm) != "yes" {
			fmt.Println("Aborted.")
			return nil
		}

		if err := db.DeleteBackup(backupID); err != nil {
			return err
		}

		fmt.Printf("Backup '%s' deleted successfully.\n", backupID)
		return nil
	},
}

func init() {
	// Create flags
	backupCreateCmd.Flags().StringVarP(&backupOutputDir, "output", "o", "", "Output directory for backups")
	backupCreateCmd.Flags().StringVarP(&backupCompression, "compress", "c", "", "Compression type (gzip, xz, zstd)")
	backupCreateCmd.Flags().StringVar(&backupDescription, "description", "", "Backup description")
	backupCreateCmd.Flags().IntVar(&backupParallel, "parallel", 0, "Number of parallel workers (0=sequential, -1=auto)")

	// Restore flags
	backupRestoreCmd.Flags().BoolVar(&restoreDropExist, "drop", false, "Drop existing databases before restore")
	backupRestoreCmd.Flags().StringArrayVar(&restoreRename, "rename", []string{}, "Rename database during restore (format: old:new)")

	backupCmd.AddCommand(backupCreateCmd)
	backupCmd.AddCommand(backupListCmd)
	backupCmd.AddCommand(backupShowCmd)
	backupCmd.AddCommand(backupRestoreCmd)
	backupCmd.AddCommand(backupDeleteCmd)
}
