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

package db

import (
	"database/sql"
	"fmt"
	"time"
)

// ClusterType represents the type of cluster/replication
type ClusterType string

const (
	ClusterTypeNone            ClusterType = "none"
	ClusterTypeMariaDBGalera   ClusterType = "galera"
	ClusterTypeMariaDBReplica  ClusterType = "mariadb_replication"
	ClusterTypePostgresStream  ClusterType = "postgres_streaming"
	ClusterTypePostgresLogical ClusterType = "postgres_logical"
)

// ClusterStatus represents the overall cluster status
type ClusterStatus struct {
	Type         ClusterType
	IsPrimary    bool
	IsHealthy    bool
	NodeCount    int
	Nodes        []ClusterNode
	LocalNode    *ClusterNode
	LastChecked  time.Time
	ErrorMessage string
}

// ClusterNode represents a node in the cluster
type ClusterNode struct {
	Address          string
	Port             int
	Role             string // "primary", "replica", "standby", "donor", "synced", etc.
	State            string
	IsLocal          bool
	LagBytes         int64
	LagSeconds       float64
	SyncState        string
	LastSeen         time.Time
	ReplicationSlot  string
	SentLSN          string
	WriteLSN         string
	FlushLSN         string
	ReplayLSN        string
}

// GaleraStatus represents MariaDB Galera cluster status
type GaleraStatus struct {
	ClusterStatus   string // "Primary", "Non-Primary", "Disconnected"
	ClusterSize     int
	ClusterStateUUID string
	LocalState      string // "Synced", "Donor", "Desync", "Joining", "Disconnected"
	LocalIndex      int
	Ready           bool
	Connected       bool
	LocalSendQueue  int
	LocalRecvQueue  int
	FlowControl     bool
}

// ReplicationStatus represents master/slave replication status
type ReplicationStatus struct {
	IsMaster         bool
	IsReplica        bool
	MasterHost       string
	MasterPort       int
	ReplicaIORunning bool
	ReplicaSQLRunning bool
	SecondsBehind    *int64
	LastError        string
	LastIOError      string
	LastSQLError     string
	Position         string
	GTIDMode         bool
}

// GetClusterStatus returns the current cluster status
func (c *Connection) GetClusterStatus() (*ClusterStatus, error) {
	status := &ClusterStatus{
		Type:        ClusterTypeNone,
		LastChecked: time.Now(),
	}

	// Check if primary
	isPrimary, err := c.IsPrimary()
	if err == nil {
		status.IsPrimary = isPrimary
	}

	if c.Config.Type == DatabaseTypeMariaDB {
		// Try Galera first
		galeraStatus, err := c.GetGaleraStatus()
		if err == nil && galeraStatus.ClusterStatus != "" {
			status.Type = ClusterTypeMariaDBGalera
			status.IsHealthy = galeraStatus.Ready && galeraStatus.Connected
			status.NodeCount = galeraStatus.ClusterSize
			status.LocalNode = &ClusterNode{
				Role:  galeraStatus.LocalState,
				State: galeraStatus.ClusterStatus,
			}
			return status, nil
		}

		// Check master/slave replication
		replStatus, err := c.GetMariaDBReplicationStatus()
		if err == nil && (replStatus.IsMaster || replStatus.IsReplica) {
			status.Type = ClusterTypeMariaDBReplica
			status.IsPrimary = replStatus.IsMaster
			if replStatus.IsReplica {
				status.IsHealthy = replStatus.ReplicaIORunning && replStatus.ReplicaSQLRunning
				if replStatus.SecondsBehind != nil {
					status.LocalNode = &ClusterNode{
						Role:       "replica",
						LagSeconds: float64(*replStatus.SecondsBehind),
					}
				}
			} else {
				status.IsHealthy = true
				status.LocalNode = &ClusterNode{Role: "master"}
			}
			return status, nil
		}
	} else if c.Config.Type == DatabaseTypePostgres {
		// Check PostgreSQL streaming replication
		nodes, err := c.GetPostgresReplicaNodes()
		if err == nil {
			if len(nodes) > 0 || !isPrimary {
				status.Type = ClusterTypePostgresStream
				status.Nodes = nodes
				status.NodeCount = len(nodes) + 1 // +1 for primary
				status.IsHealthy = true

				// Check for lag issues
				for _, node := range nodes {
					if node.LagSeconds > 60 {
						status.IsHealthy = false
						break
					}
				}

				status.LocalNode = &ClusterNode{
					Role:    "primary",
					IsLocal: true,
				}
				if !isPrimary {
					status.LocalNode.Role = "standby"
				}

				return status, nil
			}
		}
	}

	return status, nil
}

// IsPrimary checks if this node is the primary/master
func (c *Connection) IsPrimary() (bool, error) {
	query := c.Driver.IsPrimaryQuery()
	if query == "" {
		return true, nil // Assume primary if no check available
	}

	if c.Config.Type == DatabaseTypePostgres {
		var isPrimary bool
		err := c.DB.QueryRow(query).Scan(&isPrimary)
		return isPrimary, err
	}

	// MariaDB - check SHOW MASTER STATUS
	rows, err := c.DB.Query(query)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	// If we can show master status and have binlog position, we're a master
	if rows.Next() {
		return true, nil
	}

	return false, nil
}

// GetGaleraStatus returns Galera cluster status for MariaDB
func (c *Connection) GetGaleraStatus() (*GaleraStatus, error) {
	status := &GaleraStatus{}

	// Query wsrep variables
	wsrepVars := map[string]*string{
		"wsrep_cluster_status":     &status.ClusterStatus,
		"wsrep_local_state_comment": &status.LocalState,
		"wsrep_cluster_state_uuid": &status.ClusterStateUUID,
	}

	for varName, dest := range wsrepVars {
		var name, value string
		err := c.DB.QueryRow("SHOW STATUS LIKE ?", varName).Scan(&name, &value)
		if err == nil {
			*dest = value
		}
	}

	// Get numeric values
	var name, value string
	if err := c.DB.QueryRow("SHOW STATUS LIKE 'wsrep_cluster_size'").Scan(&name, &value); err == nil {
		fmt.Sscanf(value, "%d", &status.ClusterSize)
	}

	if err := c.DB.QueryRow("SHOW STATUS LIKE 'wsrep_local_index'").Scan(&name, &value); err == nil {
		fmt.Sscanf(value, "%d", &status.LocalIndex)
	}

	if err := c.DB.QueryRow("SHOW STATUS LIKE 'wsrep_ready'").Scan(&name, &value); err == nil {
		status.Ready = value == "ON"
	}

	if err := c.DB.QueryRow("SHOW STATUS LIKE 'wsrep_connected'").Scan(&name, &value); err == nil {
		status.Connected = value == "ON"
	}

	// If we got cluster status, Galera is active
	if status.ClusterStatus == "" {
		return nil, fmt.Errorf("Galera cluster not configured")
	}

	return status, nil
}

// GetMariaDBReplicationStatus returns master/slave replication status
func (c *Connection) GetMariaDBReplicationStatus() (*ReplicationStatus, error) {
	status := &ReplicationStatus{}

	// Check if this is a master
	masterRows, err := c.DB.Query("SHOW MASTER STATUS")
	if err == nil {
		defer masterRows.Close()
		if masterRows.Next() {
			status.IsMaster = true
			// Get position info
			cols, _ := masterRows.Columns()
			values := make([]interface{}, len(cols))
			valuePtrs := make([]interface{}, len(cols))
			for i := range values {
				valuePtrs[i] = &values[i]
			}
			masterRows.Scan(valuePtrs...)

			if len(values) > 1 {
				if pos, ok := values[1].(int64); ok {
					status.Position = fmt.Sprintf("%d", pos)
				}
			}
		}
	}

	// Check if this is a replica
	slaveRows, err := c.DB.Query("SHOW SLAVE STATUS")
	if err == nil {
		defer slaveRows.Close()
		if slaveRows.Next() {
			status.IsReplica = true

			// Parse slave status - this has many columns
			cols, _ := slaveRows.Columns()
			values := make([]interface{}, len(cols))
			valuePtrs := make([]interface{}, len(cols))
			for i := range values {
				valuePtrs[i] = &values[i]
			}
			slaveRows.Scan(valuePtrs...)

			// Find specific fields by column name
			for i, col := range cols {
				switch col {
				case "Master_Host":
					if v, ok := values[i].([]byte); ok {
						status.MasterHost = string(v)
					}
				case "Master_Port":
					if v, ok := values[i].(int64); ok {
						status.MasterPort = int(v)
					}
				case "Slave_IO_Running":
					if v, ok := values[i].([]byte); ok {
						status.ReplicaIORunning = string(v) == "Yes"
					}
				case "Slave_SQL_Running":
					if v, ok := values[i].([]byte); ok {
						status.ReplicaSQLRunning = string(v) == "Yes"
					}
				case "Seconds_Behind_Master":
					if v, ok := values[i].(int64); ok {
						status.SecondsBehind = &v
					}
				case "Last_Error":
					if v, ok := values[i].([]byte); ok {
						status.LastError = string(v)
					}
				}
			}
		}
	}

	if !status.IsMaster && !status.IsReplica {
		return nil, fmt.Errorf("no replication configured")
	}

	return status, nil
}

// GetPostgresReplicaNodes returns streaming replication replica nodes
func (c *Connection) GetPostgresReplicaNodes() ([]ClusterNode, error) {
	query := c.Driver.ClusterNodesQuery()
	if query == "" {
		return nil, fmt.Errorf("cluster nodes query not supported")
	}

	rows, err := c.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []ClusterNode
	for rows.Next() {
		var node ClusterNode
		var addr, state sql.NullString
		var sentLSN, writeLSN, flushLSN, replayLSN, syncState sql.NullString

		err := rows.Scan(&addr, &state, &sentLSN, &writeLSN, &flushLSN, &replayLSN, &syncState)
		if err != nil {
			continue
		}

		node.Address = addr.String
		node.State = state.String
		node.Role = "replica"
		node.SentLSN = sentLSN.String
		node.WriteLSN = writeLSN.String
		node.FlushLSN = flushLSN.String
		node.ReplayLSN = replayLSN.String
		node.SyncState = syncState.String

		nodes = append(nodes, node)
	}

	return nodes, rows.Err()
}

// GetClusterHealth returns a simple health check for the cluster
func (c *Connection) GetClusterHealth() (bool, string, error) {
	status, err := c.GetClusterStatus()
	if err != nil {
		return false, "", err
	}

	if status.Type == ClusterTypeNone {
		return true, "standalone", nil
	}

	if status.IsHealthy {
		return true, string(status.Type), nil
	}

	return false, status.ErrorMessage, nil
}
