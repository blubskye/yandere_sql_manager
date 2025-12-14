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

	"github.com/blubskye/yandere_sql_manager/internal/db"
	"github.com/spf13/cobra"
)

var (
	cloneNoData      bool
	cloneDropTarget  bool
)

var cloneCmd = &cobra.Command{
	Use:   "clone <source-db> <target-db>",
	Short: "Clone a database",
	Long: `Create a copy of a database with all tables and optionally data.

Examples:
  ysm clone mydb mydb_copy
  ysm clone mydb mydb_backup --no-data
  ysm clone production staging --drop-target`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		sourceDB := args[0]
		targetDB := args[1]

		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		fmt.Printf("Cloning database '%s' to '%s'...\n", sourceDB, targetDB)

		opts := db.CloneOptions{
			SourceDB:     sourceDB,
			TargetDB:     targetDB,
			IncludeData:  !cloneNoData,
			DropIfExists: cloneDropTarget,
			OnProgress: func(table string, tableNum, totalTables int) {
				fmt.Printf("\rCloning table %d/%d: %s", tableNum, totalTables, table)
			},
		}

		if err := conn.CloneDatabase(opts); err != nil {
			return fmt.Errorf("clone failed: %w", err)
		}

		fmt.Println("\nClone completed successfully!")
		return nil
	},
}

func init() {
	cloneCmd.Flags().BoolVar(&cloneNoData, "no-data", false, "Clone structure only, no data")
	cloneCmd.Flags().BoolVar(&cloneDropTarget, "drop-target", false, "Drop target database if it exists")

	rootCmd.AddCommand(cloneCmd)
}
