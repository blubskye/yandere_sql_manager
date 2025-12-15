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

import "fmt"

// DatabaseType represents supported database types
type DatabaseType string

const (
	DatabaseTypeMariaDB  DatabaseType = "mariadb"
	DatabaseTypePostgres DatabaseType = "postgres"
)

// Driver interface defines database-specific operations
type Driver interface {
	// Connection
	DSN(cfg ConnectionConfig) string
	DriverName() string
	DefaultPort() int

	// Identifiers
	QuoteIdentifier(name string) string

	// Schema queries
	ListDatabasesQuery() string
	ListTablesQuery() string
	DescribeTableQuery(table string) string
	GetCreateTableQuery(table string) string
	TableRowCountQuery(table string) string

	// Database operations
	CreateDatabaseQuery(name string) string
	DropDatabaseQuery(name string) string
	UseDatabaseStatement(name string) string // empty string means reconnect required

	// Variables/Settings
	GetVariableQuery(name string) string
	GetVariablesLikeQuery(pattern string) string
	GetGlobalVariablesLikeQuery(pattern string) string
	SetVariableQuery(name, value string, global bool) string
	CommonVariables() []string

	// Import/Export helpers
	ExportHeader() string
	ExportFooter() string
	DisableForeignKeysSQL() string
	EnableForeignKeysSQL() string
	DisableUniqueChecksSQL() string
	EnableUniqueChecksSQL() string

	// Server info
	ServerVersionQuery() string
	UptimeQuery() string
	ConnectionCountQuery() string

	// Data type handling
	EscapeString(s string) string

	// User management
	ListUsersQuery() string
	CreateUserQuery(username, host, password string) string
	DropUserQuery(username, host string) string
	ShowUserGrantsQuery(username, host string) string
	GrantPrivilegesQuery(privs []string, database, table, username, host string) string
	RevokePrivilegesQuery(privs []string, database, table, username, host string) string
	FlushPrivilegesQuery() string

	// Enhanced database creation
	CreateDatabaseWithOptionsQuery(name, charset, collation string) string
	GetCharsetsQuery() string
	GetCollationsQuery(charset string) string

	// Statistics
	DatabaseSizeQuery(database string) string
	TableSizesQuery() string
	IndexSizesQuery() string
	ActiveConnectionsQuery() string
	ConnectionLimitQuery() string
	SlowQueriesCountQuery() string
	CacheHitRateQuery() string
	ReplicationLagQuery() string

	// Cluster/Replication
	ClusterStatusQuery() string
	ClusterNodesQuery() string
	ReplicationStatusQuery() string
	IsPrimaryQuery() string
}

// GetDriver returns the appropriate driver for the given database type
func GetDriver(dbType DatabaseType) (Driver, error) {
	switch dbType {
	case DatabaseTypeMariaDB, "mysql", "":
		return &MariaDBDriver{}, nil
	case DatabaseTypePostgres, "postgresql":
		return &PostgresDriver{}, nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}

// ValidDatabaseTypes returns all valid database type values
func ValidDatabaseTypes() []DatabaseType {
	return []DatabaseType{DatabaseTypeMariaDB, DatabaseTypePostgres}
}
