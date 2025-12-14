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

	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff <db1> <db2>",
	Short: "Compare schemas between two databases",
	Long: `Compare table structures between two databases and show differences.

Examples:
  ysm diff production staging
  ysm diff mydb mydb_backup`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		db1 := args[0]
		db2 := args[1]

		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		fmt.Printf("Comparing schemas: %s vs %s\n\n", db1, db2)

		result, err := conn.CompareSchemas(db1, db2)
		if err != nil {
			return fmt.Errorf("comparison failed: %w", err)
		}

		// Tables only in first database
		if len(result.OnlyInFirst) > 0 {
			fmt.Printf("Tables only in %s:\n", db1)
			for _, t := range result.OnlyInFirst {
				fmt.Printf("  + %s\n", t)
			}
			fmt.Println()
		}

		// Tables only in second database
		if len(result.OnlyInSecond) > 0 {
			fmt.Printf("Tables only in %s:\n", db2)
			for _, t := range result.OnlyInSecond {
				fmt.Printf("  + %s\n", t)
			}
			fmt.Println()
		}

		// Different tables
		if len(result.Different) > 0 {
			fmt.Println("Tables with different schemas:")
			for _, d := range result.Different {
				fmt.Printf("  ~ %s\n", d.TableName)
			}
			fmt.Println()
		}

		// Identical tables
		if len(result.Identical) > 0 {
			fmt.Printf("Identical tables: %d\n", len(result.Identical))
		}

		// Summary
		fmt.Println("\nSummary:")
		fmt.Printf("  Only in %s: %d\n", db1, len(result.OnlyInFirst))
		fmt.Printf("  Only in %s: %d\n", db2, len(result.OnlyInSecond))
		fmt.Printf("  Different: %d\n", len(result.Different))
		fmt.Printf("  Identical: %d\n", len(result.Identical))

		return nil
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
}
