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
	"net/url"
	"strings"
)

// PostgresDriver implements the Driver interface for PostgreSQL
type PostgresDriver struct{}

// DSN generates a PostgreSQL connection string
func (d *PostgresDriver) DSN(cfg ConnectionConfig) string {
	host := cfg.Host
	if host == "" {
		host = "localhost"
	}
	port := cfg.Port
	if port == 0 {
		port = 5432
	}

	// Build connection string
	// Format: postgres://user:password@host:port/database?sslmode=disable
	u := url.URL{
		Scheme: "postgres",
		Host:   fmt.Sprintf("%s:%d", host, port),
		Path:   cfg.Database,
	}

	if cfg.User != "" {
		if cfg.Password != "" {
			u.User = url.UserPassword(cfg.User, cfg.Password)
		} else {
			u.User = url.User(cfg.User)
		}
	}

	// Add query parameters
	q := u.Query()
	q.Set("sslmode", "disable") // Default to disable for local dev; can be made configurable
	u.RawQuery = q.Encode()

	return u.String()
}

// DriverName returns the database/sql driver name
func (d *PostgresDriver) DriverName() string {
	return "postgres"
}

// DefaultPort returns the default PostgreSQL port
func (d *PostgresDriver) DefaultPort() int {
	return 5432
}

// QuoteIdentifier quotes an identifier with double quotes
func (d *PostgresDriver) QuoteIdentifier(name string) string {
	return "\"" + strings.ReplaceAll(name, "\"", "\"\"") + "\""
}

// ListDatabasesQuery returns the query to list all databases
func (d *PostgresDriver) ListDatabasesQuery() string {
	return "SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname"
}

// ListTablesQuery returns the query to list all tables with metadata
func (d *PostgresDriver) ListTablesQuery() string {
	return `SELECT
		t.table_name as "Name",
		'' as "Engine",
		COALESCE(s.n_live_tup, 0) as "Rows"
	FROM information_schema.tables t
	LEFT JOIN pg_stat_user_tables s ON t.table_name = s.relname
	WHERE t.table_schema = 'public'
	AND t.table_type = 'BASE TABLE'
	ORDER BY t.table_name`
}

// DescribeTableQuery returns the query to describe a table's columns
func (d *PostgresDriver) DescribeTableQuery(table string) string {
	return fmt.Sprintf(`SELECT
		column_name as "Field",
		data_type as "Type",
		CASE WHEN is_nullable = 'YES' THEN 'YES' ELSE 'NO' END as "Null",
		CASE WHEN column_default LIKE 'nextval%%' THEN 'PRI'
		     WHEN column_default IS NOT NULL THEN 'MUL'
		     ELSE '' END as "Key",
		COALESCE(column_default, 'NULL') as "Default",
		CASE WHEN column_default LIKE 'nextval%%' THEN 'auto_increment' ELSE '' END as "Extra"
	FROM information_schema.columns
	WHERE table_name = '%s' AND table_schema = 'public'
	ORDER BY ordinal_position`, table)
}

// GetCreateTableQuery returns the query to get a table's CREATE statement
// PostgreSQL doesn't have SHOW CREATE TABLE, so we build it from information_schema
func (d *PostgresDriver) GetCreateTableQuery(table string) string {
	// This returns column info; actual CREATE TABLE must be built in code
	return fmt.Sprintf(`SELECT
		column_name,
		data_type,
		character_maximum_length,
		is_nullable,
		column_default
	FROM information_schema.columns
	WHERE table_name = '%s' AND table_schema = 'public'
	ORDER BY ordinal_position`, table)
}

// TableRowCountQuery returns the query to count rows in a table
func (d *PostgresDriver) TableRowCountQuery(table string) string {
	return fmt.Sprintf("SELECT COUNT(*) FROM %s", d.QuoteIdentifier(table))
}

// CreateDatabaseQuery returns the query to create a database
func (d *PostgresDriver) CreateDatabaseQuery(name string) string {
	return fmt.Sprintf("CREATE DATABASE %s", d.QuoteIdentifier(name))
}

// DropDatabaseQuery returns the query to drop a database
func (d *PostgresDriver) DropDatabaseQuery(name string) string {
	return fmt.Sprintf("DROP DATABASE %s", d.QuoteIdentifier(name))
}

// UseDatabaseStatement returns empty string because PostgreSQL requires reconnecting
func (d *PostgresDriver) UseDatabaseStatement(name string) string {
	return "" // PostgreSQL requires reconnecting to switch databases
}

// GetVariableQuery returns the query to get a single variable
func (d *PostgresDriver) GetVariableQuery(name string) string {
	return fmt.Sprintf("SELECT setting FROM pg_settings WHERE name = '%s'", name)
}

// GetVariablesLikeQuery returns the query to get variables matching a pattern
func (d *PostgresDriver) GetVariablesLikeQuery(pattern string) string {
	// Convert SQL LIKE pattern (%) to work with pg_settings
	return fmt.Sprintf("SELECT name, setting FROM pg_settings WHERE name LIKE '%s' ORDER BY name", pattern)
}

// GetGlobalVariablesLikeQuery returns the query to get global variables matching a pattern
// PostgreSQL doesn't distinguish session/global the same way, so this is the same
func (d *PostgresDriver) GetGlobalVariablesLikeQuery(pattern string) string {
	return d.GetVariablesLikeQuery(pattern)
}

// SetVariableQuery returns the query to set a variable
func (d *PostgresDriver) SetVariableQuery(name, value string, global bool) string {
	// PostgreSQL uses SET for session variables
	// Global variables require ALTER SYSTEM (and reload) - not supported in session
	if global {
		return fmt.Sprintf("ALTER SYSTEM SET %s = '%s'", name, value)
	}
	return fmt.Sprintf("SET %s = '%s'", name, value)
}

// CommonVariables returns the list of common PostgreSQL variables
func (d *PostgresDriver) CommonVariables() []string {
	return []string{
		"work_mem",
		"maintenance_work_mem",
		"shared_buffers",
		"effective_cache_size",
		"random_page_cost",
		"statement_timeout",
		"lock_timeout",
		"idle_in_transaction_session_timeout",
		"timezone",
		"client_encoding",
		"search_path",
		"default_transaction_isolation",
		"log_statement",
		"log_min_duration_statement",
	}
}

// ExportHeader returns the SQL header for exports
func (d *PostgresDriver) ExportHeader() string {
	return `SET session_replication_role = 'replica';
SET client_encoding = 'UTF8';
BEGIN;
SET timezone = '+00:00';
`
}

// ExportFooter returns the SQL footer for exports
func (d *PostgresDriver) ExportFooter() string {
	return `COMMIT;
SET session_replication_role = 'origin';
`
}

// DisableForeignKeysSQL returns the SQL to disable foreign key checks
// PostgreSQL uses session_replication_role to disable triggers/FK checks
func (d *PostgresDriver) DisableForeignKeysSQL() string {
	return "SET session_replication_role = 'replica'"
}

// EnableForeignKeysSQL returns the SQL to enable foreign key checks
func (d *PostgresDriver) EnableForeignKeysSQL() string {
	return "SET session_replication_role = 'origin'"
}

// DisableUniqueChecksSQL returns the SQL to disable unique checks
// PostgreSQL doesn't have a direct equivalent; using deferred constraints
func (d *PostgresDriver) DisableUniqueChecksSQL() string {
	return "SET CONSTRAINTS ALL DEFERRED"
}

// EnableUniqueChecksSQL returns the SQL to enable unique checks
func (d *PostgresDriver) EnableUniqueChecksSQL() string {
	return "SET CONSTRAINTS ALL IMMEDIATE"
}

// ServerVersionQuery returns the query to get server version
func (d *PostgresDriver) ServerVersionQuery() string {
	return "SELECT version()"
}

// UptimeQuery returns the query to get server uptime in seconds
func (d *PostgresDriver) UptimeQuery() string {
	return "SELECT EXTRACT(EPOCH FROM (now() - pg_postmaster_start_time()))::integer AS uptime"
}

// ConnectionCountQuery returns the query to get connection count
func (d *PostgresDriver) ConnectionCountQuery() string {
	return "SELECT count(*) FROM pg_stat_activity"
}

// EscapeString escapes a string for safe use in SQL
// PostgreSQL uses standard SQL escaping (double single quotes)
func (d *PostgresDriver) EscapeString(s string) string {
	// PostgreSQL standard: escape single quotes by doubling them
	return strings.ReplaceAll(s, "'", "''")
}

// User Management

// ListUsersQuery returns the query to list all users (roles)
func (d *PostgresDriver) ListUsersQuery() string {
	return `SELECT usename AS "User", '' AS "Host" FROM pg_user ORDER BY usename`
}

// CreateUserQuery returns the query to create a user
// PostgreSQL doesn't use host; roles are server-wide
func (d *PostgresDriver) CreateUserQuery(username, host, password string) string {
	return fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'",
		d.QuoteIdentifier(username), d.EscapeString(password))
}

// DropUserQuery returns the query to drop a user
func (d *PostgresDriver) DropUserQuery(username, host string) string {
	return fmt.Sprintf("DROP USER IF EXISTS %s", d.QuoteIdentifier(username))
}

// ShowUserGrantsQuery returns the query to show user grants
func (d *PostgresDriver) ShowUserGrantsQuery(username, host string) string {
	return fmt.Sprintf(`SELECT
		COALESCE(table_catalog, datname, '*') AS database,
		COALESCE(table_schema || '.' || table_name, '*') AS object,
		COALESCE(privilege_type, 'CONNECT') AS privilege
	FROM (
		SELECT table_catalog, table_schema, table_name, privilege_type
		FROM information_schema.role_table_grants
		WHERE grantee = '%s'
		UNION ALL
		SELECT datname::text, NULL, NULL, 'CONNECT'
		FROM pg_database d
		WHERE has_database_privilege('%s', d.oid, 'CONNECT')
		AND datistemplate = false
	) grants
	ORDER BY database, object`, d.EscapeString(username), d.EscapeString(username))
}

// GrantPrivilegesQuery returns the query to grant privileges
func (d *PostgresDriver) GrantPrivilegesQuery(privs []string, database, table, username, host string) string {
	// Map common MySQL privileges to PostgreSQL
	pgPrivs := d.mapPrivileges(privs)

	if database != "" && table != "" {
		return fmt.Sprintf("GRANT %s ON TABLE %s.%s TO %s",
			strings.Join(pgPrivs, ", "),
			d.QuoteIdentifier(database), d.QuoteIdentifier(table),
			d.QuoteIdentifier(username))
	} else if database != "" {
		// Grant on all tables in schema + connect privilege
		return fmt.Sprintf("GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO %s; GRANT CONNECT ON DATABASE %s TO %s",
			d.QuoteIdentifier(username),
			d.QuoteIdentifier(database), d.QuoteIdentifier(username))
	}
	return fmt.Sprintf("GRANT %s TO %s",
		strings.Join(pgPrivs, ", "), d.QuoteIdentifier(username))
}

// RevokePrivilegesQuery returns the query to revoke privileges
func (d *PostgresDriver) RevokePrivilegesQuery(privs []string, database, table, username, host string) string {
	pgPrivs := d.mapPrivileges(privs)

	if database != "" && table != "" {
		return fmt.Sprintf("REVOKE %s ON TABLE %s.%s FROM %s",
			strings.Join(pgPrivs, ", "),
			d.QuoteIdentifier(database), d.QuoteIdentifier(table),
			d.QuoteIdentifier(username))
	} else if database != "" {
		return fmt.Sprintf("REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM %s; REVOKE CONNECT ON DATABASE %s FROM %s",
			d.QuoteIdentifier(username),
			d.QuoteIdentifier(database), d.QuoteIdentifier(username))
	}
	return fmt.Sprintf("REVOKE %s FROM %s",
		strings.Join(pgPrivs, ", "), d.QuoteIdentifier(username))
}

// FlushPrivilegesQuery returns empty string as PostgreSQL doesn't need this
func (d *PostgresDriver) FlushPrivilegesQuery() string {
	return "" // PostgreSQL applies privilege changes immediately
}

// mapPrivileges maps MySQL-style privileges to PostgreSQL equivalents
func (d *PostgresDriver) mapPrivileges(privs []string) []string {
	result := make([]string, 0, len(privs))
	for _, p := range privs {
		upper := strings.ToUpper(strings.TrimSpace(p))
		switch upper {
		case "ALL", "ALL PRIVILEGES":
			result = append(result, "ALL PRIVILEGES")
		case "SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER":
			result = append(result, upper)
		case "CREATE", "DROP":
			result = append(result, upper)
		case "INDEX":
			// PostgreSQL doesn't have INDEX privilege, skip or use CREATE
			result = append(result, "CREATE")
		case "ALTER":
			// PostgreSQL uses different mechanism
			result = append(result, "ALL PRIVILEGES")
		default:
			result = append(result, upper)
		}
	}
	return result
}

// Enhanced Database Creation

// CreateDatabaseWithOptionsQuery returns the query to create a database with options
func (d *PostgresDriver) CreateDatabaseWithOptionsQuery(name, charset, collation string) string {
	query := fmt.Sprintf("CREATE DATABASE %s", d.QuoteIdentifier(name))
	if charset != "" {
		query += fmt.Sprintf(" ENCODING '%s'", charset)
	}
	if collation != "" {
		query += fmt.Sprintf(" LC_COLLATE '%s' LC_CTYPE '%s'", collation, collation)
	}
	return query
}

// GetCharsetsQuery returns the query to list available encodings
func (d *PostgresDriver) GetCharsetsQuery() string {
	return "SELECT pg_encoding_to_char(encid) AS charset FROM (SELECT generate_series(0, 40) AS encid) e WHERE pg_encoding_to_char(encid) != ''"
}

// GetCollationsQuery returns the query to list collations
func (d *PostgresDriver) GetCollationsQuery(charset string) string {
	return "SELECT collname FROM pg_collation WHERE collencoding = -1 OR collencoding = pg_char_to_encoding(current_setting('server_encoding')) ORDER BY collname"
}

// Statistics

// DatabaseSizeQuery returns the query to get database size
func (d *PostgresDriver) DatabaseSizeQuery(database string) string {
	return fmt.Sprintf("SELECT pg_database_size('%s')", d.EscapeString(database))
}

// TableSizesQuery returns the query to get table sizes
func (d *PostgresDriver) TableSizesQuery() string {
	return `SELECT
		relname AS name,
		COALESCE(n_live_tup, 0) AS row_count,
		pg_table_size(relid) AS data_size,
		pg_indexes_size(relid) AS index_size,
		pg_total_relation_size(relid) AS total_size
	FROM pg_stat_user_tables
	ORDER BY total_size DESC`
}

// IndexSizesQuery returns the query to get index sizes
func (d *PostgresDriver) IndexSizesQuery() string {
	return `SELECT
		t.relname AS table_name,
		i.relname AS index_name,
		pg_relation_size(i.oid) AS size
	FROM pg_index idx
	JOIN pg_class i ON i.oid = idx.indexrelid
	JOIN pg_class t ON t.oid = idx.indrelid
	JOIN pg_namespace n ON n.oid = t.relnamespace
	WHERE n.nspname = 'public'
	ORDER BY size DESC`
}

// ActiveConnectionsQuery returns the query to get active connections
func (d *PostgresDriver) ActiveConnectionsQuery() string {
	return "SELECT count(*) FROM pg_stat_activity WHERE state = 'active'"
}

// ConnectionLimitQuery returns the query to get max connections
func (d *PostgresDriver) ConnectionLimitQuery() string {
	return "SHOW max_connections"
}

// SlowQueriesCountQuery returns the query to get slow query count
// Requires pg_stat_statements extension
func (d *PostgresDriver) SlowQueriesCountQuery() string {
	return `SELECT COALESCE(
		(SELECT count(*) FROM pg_stat_statements WHERE mean_time > 1000),
		0
	) AS slow_queries`
}

// CacheHitRateQuery returns the query to get buffer cache hit rate
func (d *PostgresDriver) CacheHitRateQuery() string {
	return `SELECT
		COALESCE(
			ROUND(sum(heap_blks_hit) * 100.0 / NULLIF(sum(heap_blks_hit) + sum(heap_blks_read), 0), 2),
			0
		) AS cache_hit_rate
	FROM pg_statio_user_tables`
}

// ReplicationLagQuery returns the query to get replication lag
func (d *PostgresDriver) ReplicationLagQuery() string {
	return `SELECT
		pg_is_in_recovery() AS is_replica,
		CASE WHEN pg_is_in_recovery()
			THEN COALESCE(pg_wal_lsn_diff(pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn()), 0)
			ELSE 0
		END AS lag_bytes,
		CASE WHEN pg_is_in_recovery()
			THEN COALESCE(EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp())), 0)
			ELSE 0
		END AS lag_seconds`
}

// Cluster/Replication

// ClusterStatusQuery returns the query to check PostgreSQL cluster status
func (d *PostgresDriver) ClusterStatusQuery() string {
	return `SELECT
		pg_is_in_recovery() AS is_replica,
		(SELECT count(*) FROM pg_stat_replication) AS replica_count`
}

// ClusterNodesQuery returns the query to list replication nodes
func (d *PostgresDriver) ClusterNodesQuery() string {
	return `SELECT
		client_addr AS node_address,
		state AS replication_state,
		sent_lsn,
		write_lsn,
		flush_lsn,
		replay_lsn,
		sync_state
	FROM pg_stat_replication`
}

// ReplicationStatusQuery returns the query for detailed replication status
func (d *PostgresDriver) ReplicationStatusQuery() string {
	return `SELECT
		pg_is_in_recovery() AS is_standby,
		pg_last_wal_receive_lsn() AS receive_lsn,
		pg_last_wal_replay_lsn() AS replay_lsn,
		pg_last_xact_replay_timestamp() AS last_replay_time`
}

// IsPrimaryQuery returns the query to check if this is the primary
func (d *PostgresDriver) IsPrimaryQuery() string {
	return "SELECT NOT pg_is_in_recovery() AS is_primary"
}
