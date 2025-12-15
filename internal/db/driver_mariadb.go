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
	"fmt"
	"strings"
)

// MariaDBDriver implements the Driver interface for MariaDB/MySQL
type MariaDBDriver struct{}

// DSN generates a MariaDB/MySQL connection string
func (d *MariaDBDriver) DSN(cfg ConnectionConfig) string {
	// Use socket if provided
	if cfg.Socket != "" {
		dsn := fmt.Sprintf("%s:%s@unix(%s)/%s?parseTime=true&multiStatements=true",
			cfg.User, cfg.Password, cfg.Socket, cfg.Database)
		return dsn
	}

	// TCP connection
	host := cfg.Host
	if host == "" {
		host = "localhost"
	}
	port := cfg.Port
	if port == 0 {
		port = 3306
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&multiStatements=true",
		cfg.User, cfg.Password, host, port, cfg.Database)
	return dsn
}

// DriverName returns the database/sql driver name
func (d *MariaDBDriver) DriverName() string {
	return "mysql"
}

// DefaultPort returns the default MariaDB port
func (d *MariaDBDriver) DefaultPort() int {
	return 3306
}

// QuoteIdentifier quotes an identifier with backticks
func (d *MariaDBDriver) QuoteIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// ListDatabasesQuery returns the query to list all databases
func (d *MariaDBDriver) ListDatabasesQuery() string {
	return "SHOW DATABASES"
}

// ListTablesQuery returns the query to list all tables with metadata
func (d *MariaDBDriver) ListTablesQuery() string {
	return "SHOW TABLE STATUS"
}

// DescribeTableQuery returns the query to describe a table's columns
func (d *MariaDBDriver) DescribeTableQuery(table string) string {
	return fmt.Sprintf("DESCRIBE %s", d.QuoteIdentifier(table))
}

// GetCreateTableQuery returns the query to get a table's CREATE statement
func (d *MariaDBDriver) GetCreateTableQuery(table string) string {
	return fmt.Sprintf("SHOW CREATE TABLE %s", d.QuoteIdentifier(table))
}

// TableRowCountQuery returns the query to count rows in a table
func (d *MariaDBDriver) TableRowCountQuery(table string) string {
	return fmt.Sprintf("SELECT COUNT(*) FROM %s", d.QuoteIdentifier(table))
}

// CreateDatabaseQuery returns the query to create a database
func (d *MariaDBDriver) CreateDatabaseQuery(name string) string {
	return fmt.Sprintf("CREATE DATABASE %s", d.QuoteIdentifier(name))
}

// DropDatabaseQuery returns the query to drop a database
func (d *MariaDBDriver) DropDatabaseQuery(name string) string {
	return fmt.Sprintf("DROP DATABASE %s", d.QuoteIdentifier(name))
}

// UseDatabaseStatement returns the statement to switch databases
func (d *MariaDBDriver) UseDatabaseStatement(name string) string {
	return fmt.Sprintf("USE %s", d.QuoteIdentifier(name))
}

// GetVariableQuery returns the query to get a single variable
func (d *MariaDBDriver) GetVariableQuery(name string) string {
	return fmt.Sprintf("SHOW VARIABLES LIKE '%s'", name)
}

// GetVariablesLikeQuery returns the query to get variables matching a pattern
func (d *MariaDBDriver) GetVariablesLikeQuery(pattern string) string {
	return fmt.Sprintf("SHOW VARIABLES LIKE '%s'", pattern)
}

// GetGlobalVariablesLikeQuery returns the query to get global variables matching a pattern
func (d *MariaDBDriver) GetGlobalVariablesLikeQuery(pattern string) string {
	return fmt.Sprintf("SHOW GLOBAL VARIABLES LIKE '%s'", pattern)
}

// SetVariableQuery returns the query to set a variable
func (d *MariaDBDriver) SetVariableQuery(name, value string, global bool) string {
	scope := "SESSION"
	if global {
		scope = "GLOBAL"
	}
	return fmt.Sprintf("SET %s %s = ?", scope, name)
}

// CommonVariables returns the list of common MariaDB variables
func (d *MariaDBDriver) CommonVariables() []string {
	return []string{
		"foreign_key_checks",
		"unique_checks",
		"autocommit",
		"sql_mode",
		"wait_timeout",
		"max_allowed_packet",
		"character_set_client",
		"character_set_results",
		"character_set_connection",
		"collation_connection",
		"time_zone",
		"tx_isolation",
		"sql_safe_updates",
		"sql_select_limit",
	}
}

// ExportHeader returns the SQL header for exports
func (d *MariaDBDriver) ExportHeader() string {
	return `SET FOREIGN_KEY_CHECKS=0;
SET SQL_MODE = "NO_AUTO_VALUE_ON_ZERO";
SET AUTOCOMMIT = 0;
START TRANSACTION;
SET time_zone = "+00:00";
`
}

// ExportFooter returns the SQL footer for exports
func (d *MariaDBDriver) ExportFooter() string {
	return `COMMIT;
SET FOREIGN_KEY_CHECKS=1;
`
}

// DisableForeignKeysSQL returns the SQL to disable foreign key checks
func (d *MariaDBDriver) DisableForeignKeysSQL() string {
	return "SET foreign_key_checks = 0"
}

// EnableForeignKeysSQL returns the SQL to enable foreign key checks
func (d *MariaDBDriver) EnableForeignKeysSQL() string {
	return "SET foreign_key_checks = 1"
}

// DisableUniqueChecksSQL returns the SQL to disable unique checks
func (d *MariaDBDriver) DisableUniqueChecksSQL() string {
	return "SET unique_checks = 0"
}

// EnableUniqueChecksSQL returns the SQL to enable unique checks
func (d *MariaDBDriver) EnableUniqueChecksSQL() string {
	return "SET unique_checks = 1"
}

// ServerVersionQuery returns the query to get server version
func (d *MariaDBDriver) ServerVersionQuery() string {
	return "SELECT VERSION()"
}

// UptimeQuery returns the query to get server uptime
func (d *MariaDBDriver) UptimeQuery() string {
	return "SHOW STATUS LIKE 'Uptime'"
}

// ConnectionCountQuery returns the query to get connection count
func (d *MariaDBDriver) ConnectionCountQuery() string {
	return "SHOW STATUS LIKE 'Threads_connected'"
}

// EscapeString escapes a string for safe use in SQL
func (d *MariaDBDriver) EscapeString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 10)

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\':
			b.WriteString("\\\\")
		case '\'':
			b.WriteString("\\'")
		case '"':
			b.WriteString("\\\"")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		case 0:
			b.WriteString("\\0")
		case 26: // Ctrl+Z
			b.WriteString("\\Z")
		default:
			b.WriteByte(c)
		}
	}

	return b.String()
}

// User Management

// ListUsersQuery returns the query to list all users
func (d *MariaDBDriver) ListUsersQuery() string {
	return "SELECT User, Host FROM mysql.user ORDER BY User, Host"
}

// CreateUserQuery returns the query to create a user
func (d *MariaDBDriver) CreateUserQuery(username, host, password string) string {
	return fmt.Sprintf("CREATE USER '%s'@'%s' IDENTIFIED BY '%s'",
		d.EscapeString(username), d.EscapeString(host), d.EscapeString(password))
}

// DropUserQuery returns the query to drop a user
func (d *MariaDBDriver) DropUserQuery(username, host string) string {
	return fmt.Sprintf("DROP USER '%s'@'%s'",
		d.EscapeString(username), d.EscapeString(host))
}

// ShowUserGrantsQuery returns the query to show user grants
func (d *MariaDBDriver) ShowUserGrantsQuery(username, host string) string {
	return fmt.Sprintf("SHOW GRANTS FOR '%s'@'%s'",
		d.EscapeString(username), d.EscapeString(host))
}

// GrantPrivilegesQuery returns the query to grant privileges
func (d *MariaDBDriver) GrantPrivilegesQuery(privs []string, database, table, username, host string) string {
	target := "*.*"
	if database != "" && table != "" {
		target = fmt.Sprintf("%s.%s", d.QuoteIdentifier(database), d.QuoteIdentifier(table))
	} else if database != "" {
		target = fmt.Sprintf("%s.*", d.QuoteIdentifier(database))
	}
	return fmt.Sprintf("GRANT %s ON %s TO '%s'@'%s'",
		strings.Join(privs, ", "), target,
		d.EscapeString(username), d.EscapeString(host))
}

// RevokePrivilegesQuery returns the query to revoke privileges
func (d *MariaDBDriver) RevokePrivilegesQuery(privs []string, database, table, username, host string) string {
	target := "*.*"
	if database != "" && table != "" {
		target = fmt.Sprintf("%s.%s", d.QuoteIdentifier(database), d.QuoteIdentifier(table))
	} else if database != "" {
		target = fmt.Sprintf("%s.*", d.QuoteIdentifier(database))
	}
	return fmt.Sprintf("REVOKE %s ON %s FROM '%s'@'%s'",
		strings.Join(privs, ", "), target,
		d.EscapeString(username), d.EscapeString(host))
}

// FlushPrivilegesQuery returns the query to flush privileges
func (d *MariaDBDriver) FlushPrivilegesQuery() string {
	return "FLUSH PRIVILEGES"
}

// Enhanced Database Creation

// CreateDatabaseWithOptionsQuery returns the query to create a database with options
func (d *MariaDBDriver) CreateDatabaseWithOptionsQuery(name, charset, collation string) string {
	query := fmt.Sprintf("CREATE DATABASE %s", d.QuoteIdentifier(name))
	if charset != "" {
		query += fmt.Sprintf(" CHARACTER SET %s", charset)
	}
	if collation != "" {
		query += fmt.Sprintf(" COLLATE %s", collation)
	}
	return query
}

// GetCharsetsQuery returns the query to list available charsets
func (d *MariaDBDriver) GetCharsetsQuery() string {
	return "SHOW CHARACTER SET"
}

// GetCollationsQuery returns the query to list collations
func (d *MariaDBDriver) GetCollationsQuery(charset string) string {
	if charset != "" {
		return fmt.Sprintf("SHOW COLLATION WHERE Charset = '%s'", charset)
	}
	return "SHOW COLLATION"
}

// Statistics

// DatabaseSizeQuery returns the query to get database size
func (d *MariaDBDriver) DatabaseSizeQuery(database string) string {
	return fmt.Sprintf(`SELECT COALESCE(SUM(data_length + index_length), 0)
		FROM information_schema.tables
		WHERE table_schema = '%s'`, d.EscapeString(database))
}

// TableSizesQuery returns the query to get table sizes
func (d *MariaDBDriver) TableSizesQuery() string {
	return `SELECT
		table_name AS name,
		COALESCE(table_rows, 0) AS row_count,
		COALESCE(data_length, 0) AS data_size,
		COALESCE(index_length, 0) AS index_size,
		COALESCE(data_length + index_length, 0) AS total_size
	FROM information_schema.tables
	WHERE table_schema = DATABASE()
	ORDER BY total_size DESC`
}

// IndexSizesQuery returns the query to get index sizes
func (d *MariaDBDriver) IndexSizesQuery() string {
	return `SELECT
		table_name,
		index_name,
		COALESCE(stat_value * @@innodb_page_size, 0) AS size
	FROM mysql.innodb_index_stats
	WHERE database_name = DATABASE() AND stat_name = 'size'
	ORDER BY size DESC`
}

// ActiveConnectionsQuery returns the query to get active connections
func (d *MariaDBDriver) ActiveConnectionsQuery() string {
	return "SHOW STATUS LIKE 'Threads_connected'"
}

// ConnectionLimitQuery returns the query to get max connections
func (d *MariaDBDriver) ConnectionLimitQuery() string {
	return "SHOW VARIABLES LIKE 'max_connections'"
}

// SlowQueriesCountQuery returns the query to get slow query count
func (d *MariaDBDriver) SlowQueriesCountQuery() string {
	return "SHOW STATUS LIKE 'Slow_queries'"
}

// CacheHitRateQuery returns the query to get buffer pool cache hit rate
func (d *MariaDBDriver) CacheHitRateQuery() string {
	return `SELECT
		(1 - (
			(SELECT Variable_value FROM information_schema.global_status WHERE Variable_name = 'Innodb_buffer_pool_reads') /
			NULLIF((SELECT Variable_value FROM information_schema.global_status WHERE Variable_name = 'Innodb_buffer_pool_read_requests'), 0)
		)) * 100 AS cache_hit_rate`
}

// ReplicationLagQuery returns empty string as MariaDB replication is handled differently
func (d *MariaDBDriver) ReplicationLagQuery() string {
	return "" // Not applicable in the same way as PostgreSQL
}

// Cluster/Replication

// ClusterStatusQuery returns the query to check Galera cluster status
func (d *MariaDBDriver) ClusterStatusQuery() string {
	return "SHOW STATUS LIKE 'wsrep_cluster_status'"
}

// ClusterNodesQuery returns the query to list Galera cluster nodes
func (d *MariaDBDriver) ClusterNodesQuery() string {
	return `SELECT
		VARIABLE_VALUE as cluster_size FROM information_schema.global_status
		WHERE VARIABLE_NAME = 'wsrep_cluster_size'`
}

// ReplicationStatusQuery returns the query for replication status
func (d *MariaDBDriver) ReplicationStatusQuery() string {
	return "SHOW SLAVE STATUS"
}

// IsPrimaryQuery returns the query to check if this is the primary/master
func (d *MariaDBDriver) IsPrimaryQuery() string {
	return "SHOW MASTER STATUS"
}
