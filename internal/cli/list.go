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
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list [databases|tables]",
	Short: "List databases or tables",
	Long: `List databases on the server or tables in a database.

Examples:
  ysm list databases
  ysm list tables -d mydb
  ysm list tables mydb`,
}

var listDatabasesCmd = &cobra.Command{
	Use:     "databases",
	Aliases: []string{"dbs", "db"},
	Short:   "List all databases",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		databases, err := conn.ListDatabases()
		if err != nil {
			return fmt.Errorf("failed to list databases: %w", err)
		}

		fmt.Printf("Databases (%d):\n", len(databases))
		for _, db := range databases {
			fmt.Printf("  %s\n", db.Name)
		}

		return nil
	},
}

var listTablesCmd = &cobra.Command{
	Use:     "tables [database]",
	Aliases: []string{"tbl", "t"},
	Short:   "List tables in a database",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		// Get database from args or flag
		dbName := database
		if len(args) > 0 {
			dbName = args[0]
		}

		if dbName == "" {
			return fmt.Errorf("no database specified. Use -d/--database or provide as argument")
		}

		if err := conn.UseDatabase(dbName); err != nil {
			return err
		}

		tables, err := conn.ListTables()
		if err != nil {
			return fmt.Errorf("failed to list tables: %w", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "TABLE\tENGINE\tROWS\n")
		fmt.Fprintf(w, "-----\t------\t----\n")
		for _, t := range tables {
			fmt.Fprintf(w, "%s\t%s\t%d\n", t.Name, t.Engine, t.Rows)
		}
		w.Flush()

		return nil
	},
}

var describeCmd = &cobra.Command{
	Use:     "describe [table]",
	Aliases: []string{"desc"},
	Short:   "Describe a table structure",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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

		tableName := args[0]
		columns, err := conn.DescribeTable(tableName)
		if err != nil {
			return fmt.Errorf("failed to describe table: %w", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "FIELD\tTYPE\tNULL\tKEY\tDEFAULT\tEXTRA\n")
		fmt.Fprintf(w, "-----\t----\t----\t---\t-------\t-----\n")
		for _, col := range columns {
			def := "NULL"
			if col.Default != nil {
				def = *col.Default
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				col.Field, col.Type, col.Null, col.Key, def, col.Extra)
		}
		w.Flush()

		return nil
	},
}

func init() {
	listCmd.AddCommand(listDatabasesCmd)
	listCmd.AddCommand(listTablesCmd)
	listCmd.AddCommand(describeCmd)
}
