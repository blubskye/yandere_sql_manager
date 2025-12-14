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

	"github.com/blubskye/yandere_sql_manager/internal/db"
	"github.com/spf13/cobra"
)

var (
	setGlobal bool
	setShow   string
	setList   bool
)

var setCmd = &cobra.Command{
	Use:   "set [variable] [value]",
	Short: "Set or show MariaDB system variables",
	Long: `Set or show MariaDB system variables.

Examples:
  # Set a session variable
  ysm set foreign_key_checks 0

  # Set a global variable (requires SUPER privilege)
  ysm set --global max_connections 200

  # Show variables matching a pattern
  ysm set --show "character%"

  # Show a specific variable
  ysm set --show foreign_key_checks

  # List common variables with current values
  ysm set --list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		// List common variables
		if setList {
			return listCommonVariables(conn)
		}

		// Show variables matching pattern
		if setShow != "" {
			return showVariables(conn, setShow)
		}

		// Set a variable
		if len(args) < 2 {
			return fmt.Errorf("usage: ysm set <variable> <value>")
		}

		varName := args[0]
		varValue := args[1]

		if err := conn.SetVariable(varName, varValue, setGlobal); err != nil {
			return err
		}

		scope := "Session"
		if setGlobal {
			scope = "Global"
		}
		fmt.Printf("%s variable '%s' set to '%s'\n", scope, varName, varValue)

		return nil
	},
}

func listCommonVariables(conn *db.Connection) error {
	variables, err := conn.GetCommonVariables()
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "VARIABLE\tVALUE")
	fmt.Fprintln(w, "--------\t-----")

	for _, v := range variables {
		fmt.Fprintf(w, "%s\t%s\n", v.Name, v.Value)
	}

	return w.Flush()
}

func showVariables(conn *db.Connection, pattern string) error {
	variables, err := conn.GetVariables(pattern)
	if err != nil {
		return err
	}

	if len(variables) == 0 {
		fmt.Printf("No variables found matching '%s'\n", pattern)
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "VARIABLE\tVALUE")
	fmt.Fprintln(w, "--------\t-----")

	for _, v := range variables {
		fmt.Fprintf(w, "%s\t%s\n", v.Name, v.Value)
	}

	return w.Flush()
}

func init() {
	setCmd.Flags().BoolVarP(&setGlobal, "global", "g", false, "Set as global variable (requires SUPER privilege)")
	setCmd.Flags().StringVarP(&setShow, "show", "s", "", "Show variables matching pattern")
	setCmd.Flags().BoolVarP(&setList, "list", "l", false, "List common variables with current values")

	rootCmd.AddCommand(setCmd)
}
