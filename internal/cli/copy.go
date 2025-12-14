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
	copyNoData       bool
	copyDropTarget   bool
	copyWhere        string
	copyTargetName   string
)

var copyCmd = &cobra.Command{
	Use:   "copy <source-db>.<table> <target-db>",
	Short: "Copy a table between databases",
	Long: `Copy a table from one database to another.

Examples:
  ysm copy mydb.users otherdb
  ysm copy mydb.users otherdb --name=users_backup
  ysm copy mydb.orders otherdb --where="status='completed'"
  ysm copy mydb.logs archive --no-data`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Parse source
		parts := strings.SplitN(args[0], ".", 2)
		if len(parts) != 2 {
			return fmt.Errorf("source must be in format: database.table")
		}
		sourceDB := parts[0]
		sourceTable := parts[1]
		targetDB := args[1]

		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		targetTable := copyTargetName
		if targetTable == "" {
			targetTable = sourceTable
		}

		fmt.Printf("Copying %s.%s to %s.%s...\n", sourceDB, sourceTable, targetDB, targetTable)

		opts := db.CopyTableOptions{
			SourceDB:     sourceDB,
			SourceTable:  sourceTable,
			TargetDB:     targetDB,
			TargetTable:  targetTable,
			IncludeData:  !copyNoData,
			DropIfExists: copyDropTarget,
			WhereClause:  copyWhere,
			OnProgress: func(rowsCopied int64) {
				fmt.Printf("\rRows copied: %d", rowsCopied)
			},
		}

		if err := conn.CopyTable(opts); err != nil {
			return fmt.Errorf("copy failed: %w", err)
		}

		fmt.Println("\nCopy completed successfully!")
		return nil
	},
}

func init() {
	copyCmd.Flags().BoolVar(&copyNoData, "no-data", false, "Copy structure only, no data")
	copyCmd.Flags().BoolVar(&copyDropTarget, "drop-target", false, "Drop target table if it exists")
	copyCmd.Flags().StringVar(&copyWhere, "where", "", "WHERE clause to filter data")
	copyCmd.Flags().StringVar(&copyTargetName, "name", "", "Target table name (default: same as source)")

	rootCmd.AddCommand(copyCmd)
}
