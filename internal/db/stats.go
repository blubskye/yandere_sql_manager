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

// ServerStats contains comprehensive server statistics
type ServerStats struct {
	Version     string
	Uptime      time.Duration
	Databases   []DatabaseStats
	Connections ConnectionStats
	Performance PerformanceStats
	Replication *ReplicationStats // PostgreSQL only
}

// DatabaseStats contains statistics for a single database
type DatabaseStats struct {
	Name       string
	Size       int64
	TableCount int
}

// TableStats contains statistics for a single table
type TableStats struct {
	Name      string
	RowCount  int64
	DataSize  int64
	IndexSize int64
	TotalSize int64
}

// ConnectionStats contains connection information
type ConnectionStats struct {
	Active int
	Max    int
	Idle   int
}

// PerformanceStats contains performance metrics
type PerformanceStats struct {
	SlowQueries  int64
	CacheHitRate float64
}

// ReplicationStats contains PostgreSQL replication info
type ReplicationStats struct {
	IsReplica  bool
	LagBytes   int64
	LagSeconds float64
}

// GetServerStats collects all server statistics
func (c *Connection) GetServerStats() (*ServerStats, error) {
	stats := &ServerStats{}

	// Get version
	if version, err := c.GetServerVersion(); err == nil {
		stats.Version = version
	}

	// Get uptime
	if uptime, err := c.GetUptime(); err == nil {
		stats.Uptime = uptime
	}

	// Get database stats
	if dbStats, err := c.GetDatabaseStats(); err == nil {
		stats.Databases = dbStats
	}

	// Get connection stats
	if connStats, err := c.GetConnectionStats(); err == nil {
		stats.Connections = connStats
	}

	// Get performance stats
	if perfStats, err := c.GetPerformanceStats(); err == nil {
		stats.Performance = perfStats
	}

	// Get replication stats (PostgreSQL only)
	if c.Config.Type == DatabaseTypePostgres {
		if replStats, err := c.GetReplicationStats(); err == nil {
			stats.Replication = replStats
		}
	}

	return stats, nil
}

// GetUptime returns the server uptime
func (c *Connection) GetUptime() (time.Duration, error) {
	query := c.Driver.UptimeQuery()
	if query == "" {
		return 0, fmt.Errorf("uptime query not supported")
	}

	if c.Config.Type == DatabaseTypePostgres {
		var uptime time.Duration
		err := c.DB.QueryRow(query).Scan(&uptime)
		if err != nil {
			return 0, err
		}
		return uptime, nil
	}

	// MariaDB returns status value
	rows, err := c.DB.Query(query)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			continue
		}
		if name == "Uptime" {
			var seconds int64
			fmt.Sscanf(value, "%d", &seconds)
			return time.Duration(seconds) * time.Second, nil
		}
	}

	return 0, fmt.Errorf("uptime not found")
}

// GetDatabaseStats returns statistics for all databases
func (c *Connection) GetDatabaseStats() ([]DatabaseStats, error) {
	databases, err := c.ListDatabases()
	if err != nil {
		return nil, err
	}

	var stats []DatabaseStats
	for _, db := range databases {
		ds := DatabaseStats{
			Name: db.Name,
		}

		// Get size
		if size, err := c.GetDatabaseSize(db.Name); err == nil {
			ds.Size = size
		}

		// Get table count
		if count, err := c.GetTableCount(db.Name); err == nil {
			ds.TableCount = count
		}

		stats = append(stats, ds)
	}

	return stats, nil
}

// GetDatabaseSize returns the size of a database in bytes
func (c *Connection) GetDatabaseSize(database string) (int64, error) {
	query := c.Driver.DatabaseSizeQuery(database)
	if query == "" {
		return 0, fmt.Errorf("database size query not supported")
	}

	var size int64
	err := c.DB.QueryRow(query).Scan(&size)
	if err != nil {
		return 0, err
	}

	return size, nil
}

// GetTableCount returns the number of tables in a database
func (c *Connection) GetTableCount(database string) (int, error) {
	// Save current database
	origDB := c.Config.Database

	// Switch to target database
	if database != "" && database != origDB {
		if err := c.UseDatabase(database); err != nil {
			return 0, err
		}
		defer c.UseDatabase(origDB)
	}

	tables, err := c.ListTables()
	if err != nil {
		return 0, err
	}

	return len(tables), nil
}

// GetTableStats returns statistics for tables in the current database
func (c *Connection) GetTableStats() ([]TableStats, error) {
	query := c.Driver.TableSizesQuery()
	if query == "" {
		return nil, fmt.Errorf("table sizes query not supported")
	}

	rows, err := c.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []TableStats
	for rows.Next() {
		var ts TableStats
		err := rows.Scan(&ts.Name, &ts.RowCount, &ts.DataSize, &ts.IndexSize, &ts.TotalSize)
		if err != nil {
			// Try simpler scan
			var name string
			var rowCount, dataSize, indexSize, totalSize sql.NullInt64
			if err := rows.Scan(&name, &rowCount, &dataSize, &indexSize, &totalSize); err != nil {
				continue
			}
			ts.Name = name
			ts.RowCount = rowCount.Int64
			ts.DataSize = dataSize.Int64
			ts.IndexSize = indexSize.Int64
			ts.TotalSize = totalSize.Int64
		}
		stats = append(stats, ts)
	}

	return stats, rows.Err()
}

// GetConnectionStats returns connection statistics
func (c *Connection) GetConnectionStats() (ConnectionStats, error) {
	stats := ConnectionStats{}

	// Get active connections
	activeQuery := c.Driver.ActiveConnectionsQuery()
	if activeQuery != "" {
		if c.Config.Type == DatabaseTypePostgres {
			var active int
			c.DB.QueryRow(activeQuery).Scan(&active)
			stats.Active = active
		} else {
			rows, err := c.DB.Query(activeQuery)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var name, value string
					rows.Scan(&name, &value)
					if name == "Threads_connected" {
						fmt.Sscanf(value, "%d", &stats.Active)
					}
				}
			}
		}
	}

	// Get max connections
	maxQuery := c.Driver.ConnectionLimitQuery()
	if maxQuery != "" {
		if c.Config.Type == DatabaseTypePostgres {
			var max int
			c.DB.QueryRow(maxQuery).Scan(&max)
			stats.Max = max
		} else {
			rows, err := c.DB.Query(maxQuery)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var name, value string
					rows.Scan(&name, &value)
					if name == "max_connections" {
						fmt.Sscanf(value, "%d", &stats.Max)
					}
				}
			}
		}
	}

	return stats, nil
}

// GetPerformanceStats returns performance metrics
func (c *Connection) GetPerformanceStats() (PerformanceStats, error) {
	stats := PerformanceStats{}

	// Get slow queries count
	slowQuery := c.Driver.SlowQueriesCountQuery()
	if slowQuery != "" {
		if c.Config.Type == DatabaseTypePostgres {
			// PostgreSQL doesn't have slow_queries by default
			stats.SlowQueries = 0
		} else {
			rows, err := c.DB.Query(slowQuery)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var name, value string
					rows.Scan(&name, &value)
					if name == "Slow_queries" {
						fmt.Sscanf(value, "%d", &stats.SlowQueries)
					}
				}
			}
		}
	}

	// Get cache hit rate
	cacheQuery := c.Driver.CacheHitRateQuery()
	if cacheQuery != "" {
		var rate sql.NullFloat64
		err := c.DB.QueryRow(cacheQuery).Scan(&rate)
		if err == nil && rate.Valid {
			stats.CacheHitRate = rate.Float64
		}
	}

	return stats, nil
}

// GetReplicationStats returns PostgreSQL replication stats
func (c *Connection) GetReplicationStats() (*ReplicationStats, error) {
	if c.Config.Type != DatabaseTypePostgres {
		return nil, fmt.Errorf("replication stats only available for PostgreSQL")
	}

	query := c.Driver.ReplicationLagQuery()
	if query == "" {
		return nil, fmt.Errorf("replication query not supported")
	}

	stats := &ReplicationStats{}

	var isReplica bool
	var lagBytes sql.NullInt64
	var lagSeconds sql.NullFloat64

	err := c.DB.QueryRow(query).Scan(&isReplica, &lagBytes, &lagSeconds)
	if err != nil {
		return nil, err
	}

	stats.IsReplica = isReplica
	stats.LagBytes = lagBytes.Int64
	stats.LagSeconds = lagSeconds.Float64

	return stats, nil
}

// FormatUptime formats duration as human-readable uptime
func FormatUptime(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
