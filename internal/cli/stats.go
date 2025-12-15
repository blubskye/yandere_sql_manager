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

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show database statistics",
	Long: `Display various statistics about the database server.

Subcommands:
  summary     - Show overall statistics
  databases   - Show database sizes
  tables      - Show table sizes
  connections - Show connection info
  performance - Show performance metrics`,
}

var statsSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Show overall statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		stats, err := conn.GetServerStats()
		if err != nil {
			return err
		}

		fmt.Println("Server Statistics")
		fmt.Println("=================")
		fmt.Println()

		fmt.Printf("Version:     %s\n", stats.Version)
		fmt.Printf("Uptime:      %s\n", db.FormatUptime(stats.Uptime))
		fmt.Printf("Databases:   %d\n", len(stats.Databases))
		fmt.Println()

		// Calculate total size
		var totalSize int64
		for _, d := range stats.Databases {
			totalSize += d.Size
		}
		fmt.Printf("Total Size:  %s\n", db.FormatSize(totalSize))
		fmt.Println()

		fmt.Println("Connections")
		fmt.Println("-----------")
		fmt.Printf("  Active: %d / %d\n", stats.Connections.Active, stats.Connections.Max)
		if stats.Connections.Max > 0 {
			usage := float64(stats.Connections.Active) / float64(stats.Connections.Max) * 100
			fmt.Printf("  Usage:  %.1f%%\n", usage)
		}
		fmt.Println()

		fmt.Println("Performance")
		fmt.Println("-----------")
		fmt.Printf("  Slow Queries:   %d\n", stats.Performance.SlowQueries)
		if stats.Performance.CacheHitRate > 0 {
			fmt.Printf("  Cache Hit Rate: %.2f%%\n", stats.Performance.CacheHitRate)
		}

		if stats.Replication != nil {
			fmt.Println()
			fmt.Println("Replication")
			fmt.Println("-----------")
			if stats.Replication.IsReplica {
				fmt.Printf("  Status:      Replica\n")
				fmt.Printf("  Lag (bytes): %d\n", stats.Replication.LagBytes)
				fmt.Printf("  Lag (secs):  %.2f\n", stats.Replication.LagSeconds)
			} else {
				fmt.Printf("  Status:      Primary\n")
			}
		}

		return nil
	},
}

var statsDatabasesCmd = &cobra.Command{
	Use:   "databases",
	Short: "Show database sizes",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		dbStats, err := conn.GetDatabaseStats()
		if err != nil {
			return err
		}

		if len(dbStats) == 0 {
			fmt.Println("No databases found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "DATABASE\tTABLES\tSIZE")
		fmt.Fprintln(w, "--------\t------\t----")

		var totalSize int64
		for _, ds := range dbStats {
			fmt.Fprintf(w, "%s\t%d\t%s\n",
				ds.Name,
				ds.TableCount,
				db.FormatSize(ds.Size),
			)
			totalSize += ds.Size
		}

		fmt.Fprintln(w, "--------\t------\t----")
		fmt.Fprintf(w, "TOTAL\t%d\t%s\n", len(dbStats), db.FormatSize(totalSize))

		return w.Flush()
	},
}

var statsTablesCmd = &cobra.Command{
	Use:   "tables [database]",
	Short: "Show table sizes",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		// Switch to specified database if provided
		if len(args) > 0 {
			if err := conn.UseDatabase(args[0]); err != nil {
				return err
			}
		}

		tableStats, err := conn.GetTableStats()
		if err != nil {
			return err
		}

		if len(tableStats) == 0 {
			fmt.Println("No tables found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "TABLE\tROWS\tDATA\tINDEX\tTOTAL")
		fmt.Fprintln(w, "-----\t----\t----\t-----\t-----")

		var totalRows, totalData, totalIndex, totalSize int64
		for _, ts := range tableStats {
			fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\n",
				ts.Name,
				ts.RowCount,
				db.FormatSize(ts.DataSize),
				db.FormatSize(ts.IndexSize),
				db.FormatSize(ts.TotalSize),
			)
			totalRows += ts.RowCount
			totalData += ts.DataSize
			totalIndex += ts.IndexSize
			totalSize += ts.TotalSize
		}

		fmt.Fprintln(w, "-----\t----\t----\t-----\t-----")
		fmt.Fprintf(w, "TOTAL\t%d\t%s\t%s\t%s\n",
			totalRows,
			db.FormatSize(totalData),
			db.FormatSize(totalIndex),
			db.FormatSize(totalSize),
		)

		return w.Flush()
	},
}

var statsConnectionsCmd = &cobra.Command{
	Use:   "connections",
	Short: "Show connection info",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		connStats, err := conn.GetConnectionStats()
		if err != nil {
			return err
		}

		fmt.Println("Connection Statistics")
		fmt.Println("=====================")
		fmt.Println()
		fmt.Printf("Active Connections: %d\n", connStats.Active)
		fmt.Printf("Max Connections:    %d\n", connStats.Max)

		if connStats.Max > 0 {
			usage := float64(connStats.Active) / float64(connStats.Max) * 100
			fmt.Printf("Connection Usage:   %.1f%%\n", usage)

			// Visual bar
			barWidth := 40
			filled := int(usage / 100 * float64(barWidth))
			bar := ""
			for i := 0; i < barWidth; i++ {
				if i < filled {
					bar += "█"
				} else {
					bar += "░"
				}
			}
			fmt.Printf("\n[%s] %.1f%%\n", bar, usage)
		}

		return nil
	},
}

var statsPerformanceCmd = &cobra.Command{
	Use:   "performance",
	Short: "Show performance metrics",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		perfStats, err := conn.GetPerformanceStats()
		if err != nil {
			return err
		}

		fmt.Println("Performance Statistics")
		fmt.Println("======================")
		fmt.Println()
		fmt.Printf("Slow Queries: %d\n", perfStats.SlowQueries)

		if perfStats.CacheHitRate > 0 {
			fmt.Printf("Cache Hit Rate: %.2f%%\n", perfStats.CacheHitRate)

			// Visual bar
			barWidth := 40
			filled := int(perfStats.CacheHitRate / 100 * float64(barWidth))
			bar := ""
			for i := 0; i < barWidth; i++ {
				if i < filled {
					bar += "█"
				} else {
					bar += "░"
				}
			}
			fmt.Printf("\nBuffer Pool: [%s] %.1f%%\n", bar, perfStats.CacheHitRate)
		}

		return nil
	},
}

func init() {
	statsCmd.AddCommand(statsSummaryCmd)
	statsCmd.AddCommand(statsDatabasesCmd)
	statsCmd.AddCommand(statsTablesCmd)
	statsCmd.AddCommand(statsConnectionsCmd)
	statsCmd.AddCommand(statsPerformanceCmd)
}
