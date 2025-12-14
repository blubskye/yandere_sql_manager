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

	"github.com/spf13/cobra"
)

var queryCmd = &cobra.Command{
	Use:   "query <sql>",
	Short: "Execute a SQL query",
	Long: `Execute a SQL query and display results.

Examples:
  ysm query "SELECT * FROM users LIMIT 10" -d mydb
  ysm query "SHOW DATABASES"
  ysm query "INSERT INTO users (name) VALUES ('test')" -d mydb`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sql := strings.Join(args, " ")

		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		if database != "" {
			if err := conn.UseDatabase(database); err != nil {
				return err
			}
		}

		// Determine if this is a SELECT/SHOW query
		upperSQL := strings.ToUpper(strings.TrimSpace(sql))
		isQuery := strings.HasPrefix(upperSQL, "SELECT") ||
			strings.HasPrefix(upperSQL, "SHOW") ||
			strings.HasPrefix(upperSQL, "DESCRIBE") ||
			strings.HasPrefix(upperSQL, "EXPLAIN")

		if isQuery {
			result, err := conn.Query(sql)
			if err != nil {
				return fmt.Errorf("query failed: %w", err)
			}

			if len(result.Columns) == 0 {
				fmt.Println("No results")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

			// Print header
			fmt.Fprintln(w, strings.Join(result.Columns, "\t"))

			// Print separator
			sep := make([]string, len(result.Columns))
			for i, col := range result.Columns {
				sep[i] = strings.Repeat("-", len(col))
			}
			fmt.Fprintln(w, strings.Join(sep, "\t"))

			// Print rows
			for _, row := range result.Rows {
				fmt.Fprintln(w, strings.Join(row, "\t"))
			}

			w.Flush()

			fmt.Printf("\n%d row(s) returned\n", len(result.Rows))
		} else {
			affected, err := conn.Execute(sql)
			if err != nil {
				return fmt.Errorf("execution failed: %w", err)
			}

			fmt.Printf("Query OK, %d row(s) affected\n", affected)
		}

		return nil
	},
}
