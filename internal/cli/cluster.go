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

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Cluster and replication management",
	Long: `View and manage cluster/replication status.

Supports:
  - MariaDB Galera Cluster
  - MariaDB Master/Slave Replication
  - PostgreSQL Streaming Replication

Subcommands:
  status  - Show cluster status
  nodes   - List cluster nodes
  health  - Quick health check`,
}

var clusterStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cluster status",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		status, err := conn.GetClusterStatus()
		if err != nil {
			return err
		}

		fmt.Println("Cluster Status")
		fmt.Println("==============")
		fmt.Println()

		if status.Type == db.ClusterTypeNone {
			fmt.Println("Type: Standalone (no cluster/replication)")
			return nil
		}

		fmt.Printf("Type:      %s\n", formatClusterType(status.Type))
		fmt.Printf("Role:      %s\n", formatRole(status.IsPrimary))
		fmt.Printf("Healthy:   %s\n", formatBool(status.IsHealthy))
		fmt.Printf("Nodes:     %d\n", status.NodeCount)

		if status.LocalNode != nil {
			fmt.Println()
			fmt.Println("Local Node:")
			fmt.Printf("  Role:  %s\n", status.LocalNode.Role)
			fmt.Printf("  State: %s\n", status.LocalNode.State)
			if status.LocalNode.LagSeconds > 0 {
				fmt.Printf("  Lag:   %.2f seconds\n", status.LocalNode.LagSeconds)
			}
		}

		if status.ErrorMessage != "" {
			fmt.Println()
			fmt.Printf("Warning: %s\n", status.ErrorMessage)
		}

		return nil
	},
}

var clusterNodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "List cluster nodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		status, err := conn.GetClusterStatus()
		if err != nil {
			return err
		}

		if status.Type == db.ClusterTypeNone {
			fmt.Println("Not running in cluster/replication mode.")
			return nil
		}

		fmt.Printf("Cluster Type: %s\n\n", formatClusterType(status.Type))

		if len(status.Nodes) == 0 {
			if status.LocalNode != nil {
				fmt.Println("Local node only (no replicas connected)")
				fmt.Printf("  Role:  %s\n", status.LocalNode.Role)
				fmt.Printf("  State: %s\n", status.LocalNode.State)
			}
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ADDRESS\tROLE\tSTATE\tSYNC\tLAG")
		fmt.Fprintln(w, "-------\t----\t-----\t----\t---")

		for _, node := range status.Nodes {
			lag := "-"
			if node.LagSeconds > 0 {
				lag = fmt.Sprintf("%.1fs", node.LagSeconds)
			} else if node.LagBytes > 0 {
				lag = fmt.Sprintf("%d bytes", node.LagBytes)
			}

			sync := node.SyncState
			if sync == "" {
				sync = "-"
			}

			state := node.State
			if state == "" {
				state = "-"
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				node.Address,
				node.Role,
				state,
				sync,
				lag,
			)
		}

		return w.Flush()
	},
}

var clusterHealthCmd = &cobra.Command{
	Use:   "health",
	Short: "Quick health check",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		healthy, mode, err := conn.GetClusterHealth()
		if err != nil {
			return err
		}

		if mode == "standalone" {
			fmt.Println("Status: OK (standalone)")
			return nil
		}

		if healthy {
			fmt.Printf("Status: OK (%s)\n", mode)
		} else {
			fmt.Printf("Status: UNHEALTHY (%s)\n", mode)
			return fmt.Errorf("cluster health check failed")
		}

		return nil
	},
}

var clusterGaleraCmd = &cobra.Command{
	Use:   "galera",
	Short: "Show Galera cluster details (MariaDB)",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		if conn.Config.Type != db.DatabaseTypeMariaDB {
			return fmt.Errorf("Galera is only available for MariaDB")
		}

		status, err := conn.GetGaleraStatus()
		if err != nil {
			return err
		}

		fmt.Println("Galera Cluster Status")
		fmt.Println("=====================")
		fmt.Println()
		fmt.Printf("Cluster Status: %s\n", status.ClusterStatus)
		fmt.Printf("Cluster Size:   %d nodes\n", status.ClusterSize)
		fmt.Printf("Cluster UUID:   %s\n", status.ClusterStateUUID)
		fmt.Println()
		fmt.Println("Local Node:")
		fmt.Printf("  State:     %s\n", status.LocalState)
		fmt.Printf("  Index:     %d\n", status.LocalIndex)
		fmt.Printf("  Ready:     %s\n", formatBool(status.Ready))
		fmt.Printf("  Connected: %s\n", formatBool(status.Connected))

		if status.FlowControl {
			fmt.Println()
			fmt.Println("WARNING: Flow control is active!")
		}

		return nil
	},
}

var clusterReplicationCmd = &cobra.Command{
	Use:   "replication",
	Short: "Show replication details",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		if conn.Config.Type == db.DatabaseTypeMariaDB {
			status, err := conn.GetMariaDBReplicationStatus()
			if err != nil {
				return err
			}

			fmt.Println("MariaDB Replication Status")
			fmt.Println("==========================")
			fmt.Println()

			if status.IsMaster {
				fmt.Println("Role: Master")
				fmt.Printf("Position: %s\n", status.Position)
			}

			if status.IsReplica {
				fmt.Println("Role: Replica")
				fmt.Printf("Master Host: %s:%d\n", status.MasterHost, status.MasterPort)
				fmt.Printf("IO Running:  %s\n", formatBool(status.ReplicaIORunning))
				fmt.Printf("SQL Running: %s\n", formatBool(status.ReplicaSQLRunning))
				if status.SecondsBehind != nil {
					fmt.Printf("Lag:         %d seconds\n", *status.SecondsBehind)
				}
				if status.LastError != "" {
					fmt.Printf("\nLast Error: %s\n", status.LastError)
				}
			}

			return nil
		}

		// PostgreSQL
		nodes, err := conn.GetPostgresReplicaNodes()
		if err != nil {
			return err
		}

		isPrimary, _ := conn.IsPrimary()

		fmt.Println("PostgreSQL Replication Status")
		fmt.Println("=============================")
		fmt.Println()

		if isPrimary {
			fmt.Println("Role: Primary")
			fmt.Printf("Replicas: %d\n", len(nodes))
		} else {
			fmt.Println("Role: Standby")
		}

		if len(nodes) > 0 {
			fmt.Println()
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ADDRESS\tSTATE\tSYNC\tSENT\tWRITE\tFLUSH\tREPLAY")
			fmt.Fprintln(w, "-------\t-----\t----\t----\t-----\t-----\t------")

			for _, node := range nodes {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					node.Address,
					node.State,
					node.SyncState,
					node.SentLSN,
					node.WriteLSN,
					node.FlushLSN,
					node.ReplayLSN,
				)
			}
			w.Flush()
		}

		return nil
	},
}

func init() {
	clusterCmd.AddCommand(clusterStatusCmd)
	clusterCmd.AddCommand(clusterNodesCmd)
	clusterCmd.AddCommand(clusterHealthCmd)
	clusterCmd.AddCommand(clusterGaleraCmd)
	clusterCmd.AddCommand(clusterReplicationCmd)
}

// Helper functions

func formatClusterType(t db.ClusterType) string {
	switch t {
	case db.ClusterTypeMariaDBGalera:
		return "MariaDB Galera Cluster"
	case db.ClusterTypeMariaDBReplica:
		return "MariaDB Master/Slave Replication"
	case db.ClusterTypePostgresStream:
		return "PostgreSQL Streaming Replication"
	case db.ClusterTypePostgresLogical:
		return "PostgreSQL Logical Replication"
	default:
		return string(t)
	}
}

func formatRole(isPrimary bool) string {
	if isPrimary {
		return "Primary/Master"
	}
	return "Replica/Standby"
}

func formatBool(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}
