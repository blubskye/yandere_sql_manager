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

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show server information",
	Long:  `Display information about the MariaDB server including version, uptime, and database sizes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		info, err := conn.GetServerInfo()
		if err != nil {
			return fmt.Errorf("failed to get server info: %w", err)
		}

		fmt.Println("MariaDB Server Information")
		fmt.Println("==========================")
		fmt.Printf("Version:     %s\n", info.Version)
		fmt.Printf("Uptime:      %s\n", info.Uptime)
		fmt.Printf("Connections: %d\n", info.Connections)
		fmt.Println()

		if len(info.DatabaseSizes) > 0 {
			fmt.Println("Database Sizes:")
			var totalSize int64
			for name, size := range info.DatabaseSizes {
				fmt.Printf("  %-30s %s\n", name, formatBytes(size))
				totalSize += size
			}
			fmt.Printf("  %-30s %s\n", "TOTAL", formatBytes(totalSize))
		}

		return nil
	},
}

func formatBytes(bytes int64) string {
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
	rootCmd.AddCommand(infoCmd)
}
