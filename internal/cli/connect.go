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

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Test database connection",
	Long:  `Test connection to a MariaDB server without starting the TUI.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connect()
		if err != nil {
			return fmt.Errorf("connection failed: %w", err)
		}
		defer conn.Close()

		fmt.Println("Connection successful!")

		// Get server info
		var version string
		err = conn.DB.QueryRow("SELECT VERSION()").Scan(&version)
		if err == nil {
			fmt.Printf("Server version: %s\n", version)
		}

		// List databases
		databases, err := conn.ListDatabases()
		if err == nil {
			fmt.Printf("Databases: %d\n", len(databases))
		}

		return nil
	},
}
