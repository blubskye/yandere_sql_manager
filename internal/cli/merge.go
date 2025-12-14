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
	"strings"

	"github.com/blubskye/yandere_sql_manager/internal/db"
	"github.com/spf13/cobra"
)

var (
	mergeConflict string
	mergeCreate   bool
)

var mergeCmd = &cobra.Command{
	Use:   "merge <target-db> <source-db> [source-db...]",
	Short: "Merge multiple databases into one",
	Long: `Merge tables from multiple source databases into a target database.

Conflict handling options:
  skip    - Skip tables that already exist in target
  replace - Replace existing tables with source tables
  append  - Append data to existing tables (requires matching schema)
  rename  - Rename conflicting tables (add source db name as suffix)

Examples:
  ysm merge combined db1 db2 db3
  ysm merge combined db1 db2 --conflict=append
  ysm merge newdb db1 db2 --create`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDB := args[0]
		sourceDBs := args[1:]

		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		// Parse conflict action
		var conflictAction db.MergeConflictAction
		switch strings.ToLower(mergeConflict) {
		case "skip":
			conflictAction = db.MergeSkip
		case "replace":
			conflictAction = db.MergeReplace
		case "append":
			conflictAction = db.MergeAppend
		case "rename":
			conflictAction = db.MergeRename
		default:
			return fmt.Errorf("invalid conflict option: %s (use: skip, replace, append, rename)", mergeConflict)
		}

		fmt.Printf("Merging %d databases into '%s'...\n", len(sourceDBs), targetDB)
		fmt.Printf("Sources: %s\n", strings.Join(sourceDBs, ", "))
		fmt.Printf("Conflict handling: %s\n\n", mergeConflict)

		opts := db.MergeOptions{
			SourceDBs:    sourceDBs,
			TargetDB:     targetDB,
			CreateTarget: mergeCreate,
			ConflictHandler: func(table, sourceDB string) db.MergeConflictAction {
				fmt.Printf("  Conflict: table '%s' from '%s' - %s\n", table, sourceDB, mergeConflict)
				return conflictAction
			},
			OnProgress: func(sourceDB, table string, sourceNum, totalSources int) {
				fmt.Printf("\r[%d/%d] %s: %s", sourceNum, totalSources, sourceDB, table)
			},
		}

		if err := conn.MergeDatabases(opts); err != nil {
			return fmt.Errorf("merge failed: %w", err)
		}

		fmt.Println("\nMerge completed successfully!")
		return nil
	},
}

func init() {
	mergeCmd.Flags().StringVar(&mergeConflict, "conflict", "skip", "Conflict handling: skip, replace, append, rename")
	mergeCmd.Flags().BoolVar(&mergeCreate, "create", false, "Create target database if it doesn't exist")

	rootCmd.AddCommand(mergeCmd)
}
