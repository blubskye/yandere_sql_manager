# YSM - Yandere SQL Manager

A TUI and CLI tool for managing MariaDB and PostgreSQL databases. *"I'll never let your databases go~"*

![YSM Screenshot](ysm_screenshot.png)

## Features

### Core Features
- **Interactive TUI** - Browse databases, tables, and data with a beautiful terminal interface
- **Multi-Database Support** - Full support for MariaDB/MySQL and PostgreSQL
- **Import/Export** - Full support for `.sql`, `.sql.gz`, `.sql.xz`, and `.sql.zst` files
- **Connection Profiles** - Save and manage multiple database connections with auto-applied settings
- **Query Editor** - Execute SQL queries directly from the TUI
- **Database Operations** - Clone, merge, copy, and diff databases

### User Management
- Create, drop, and manage database users
- Grant and revoke privileges
- View user permissions
- Support for host-based access (MariaDB) and roles (PostgreSQL)

### Backup & Restore
- Create full database backups with compression
- Restore from backup with progress tracking
- Per-database backup scheduling
- Backup retention policies
- List and manage backup history

### Database Setup Wizard
- Quick setup for common applications (WordPress, Laravel, Drupal, Nextcloud)
- Create database + user in one step
- Configurable charset and collation
- Template-based configuration

### Statistics Dashboard
- Real-time server statistics
- Database and table sizes
- Connection monitoring
- Performance metrics (cache hit rate, slow queries)
- Auto-refresh support

### Cluster Management
- MariaDB Galera Cluster support
- MariaDB Master/Slave replication monitoring
- PostgreSQL Streaming Replication support
- Cluster health checks
- Node status and lag monitoring

### System Variables
- View, edit, and manage session/global variables
- Profile-based variable presets
- Common variable quick-access

### Performance
- **Buffered I/O** - Efficient handling of large database files (auto-scaling buffers up to 32MB)
- **Batch Processing** - Optimized transaction batching for imports
- **Progress Tracking** - Real-time progress for long operations

### Customization
- **Customizable Keybindings** - Remap any key to any action via TUI settings menu
- Per-view keybinding configuration
- Instant key rebinding with visual feedback

### Debugging
- Verbose, debug, and trace logging levels
- Stack traces on errors
- File logging support

## Installation

### From Binary

Download the latest release from the [Releases](https://github.com/blubskye/yandere_sql_manager/releases) page.

```bash
# Linux (amd64)
tar -xzf ysm-linux-amd64.tar.gz
sudo ./install.sh

# Or manual install
sudo cp ysm /usr/local/bin/
sudo cp ysm.1 /usr/local/share/man/man1/
```

### From Source

```bash
git clone https://github.com/blubskye/yandere_sql_manager.git
cd yandere_sql_manager
make build
sudo make install
```

### Dependencies

- Go 1.21+ (for building from source)
- MariaDB/MySQL or PostgreSQL server
- Optional: `xz`, `zstd` for compression support

## Usage

### TUI Mode

```bash
# Launch interactive TUI
ysm

# Connect with specific credentials
ysm -H localhost -P 3306 -u root -p mypassword

# Use a saved profile
ysm --profile local

# Connect to PostgreSQL
ysm -t postgres -H localhost -P 5432 -u postgres
```

**TUI Key Bindings (Default):**
| Key | Action |
|-----|--------|
| `Enter` | Select database/table |
| `/` | Filter list |
| `n` | New database (setup wizard) |
| `d` | Statistics dashboard |
| `c` | Cluster status |
| `u` | User management |
| `b` | Backup management |
| `i` | Import SQL file |
| `e` | Export database |
| `s` | Open SQL query editor |
| `v` | System variables |
| `r` | Refresh |
| `?` | Keybindings settings |
| `Esc` | Go back |
| `q` | Quit |

**Connection Screen Key Bindings:**
| Key | Action |
|-----|--------|
| `Tab` | Next field |
| `Enter` | Connect (or select type/profile) |
| `Ctrl+S` | Save current connection as profile |
| `Ctrl+P` | Load saved profile |
| `←/→` | Change database type |
| `Esc` | Quit |

**Note:** All keybindings are fully customizable! Press `?` in any view to open the keybindings settings. You can remap any key to any action and changes are saved automatically to `~/.config/ysm/keybindings.yaml`~

### CLI Commands

#### Import

```bash
# Basic import
ysm import backup.sql -d mydb

# Import compressed file
ysm import backup.sql.zst -d mydb

# Create database if it doesn't exist
ysm import backup.sql -d mydb --create

# Rename database during import
ysm import backup.sql -d olddb --rename newdb

# Disable foreign key checks during import
ysm import backup.sql -d mydb --no-fk-checks

# PostgreSQL native format import (.dump files use pg_restore)
ysm import backup.dump -d mydb --create

# PostgreSQL parallel restore (faster for large databases)
ysm import backup.dump -d mydb --jobs=4

# Force native tool (psql/pg_restore for PostgreSQL)
ysm import backup.sql -d mydb --native
```

#### Export

```bash
# Basic export
ysm export mydb

# Export with compression
ysm export mydb -o backup.sql.zst

# Export structure only (no data)
ysm export mydb --no-data

# Export specific tables
ysm export mydb --tables users,posts

# PostgreSQL custom format (smaller, faster restore with pg_restore)
ysm export mydb -o backup.dump --format=custom

# PostgreSQL tar format
ysm export mydb -o backup.tar --format=tar

# PostgreSQL directory format (for parallel restore)
ysm export mydb -o backup_dir --format=dir

# Use native tools (pg_dump/mysqldump)
ysm export mydb -o backup.sql --native
```

#### Backup & Restore

```bash
# Create backup of all databases
ysm backup create

# Backup specific databases with compression
ysm backup create mydb1 mydb2 --compress zstd

# List all backups
ysm backup list

# Show backup details
ysm backup show 20250101-120000

# Restore a backup
ysm backup restore 20250101-120000

# Restore specific databases
ysm backup restore 20250101-120000 --databases mydb1

# Delete a backup
ysm backup delete 20250101-120000
```

#### User Management

```bash
# List all users
ysm user list

# Create a new user
ysm user create myuser -p mypassword

# Create user with specific host (MariaDB)
ysm user create myuser -p mypassword -H 192.168.1.%

# Show user privileges
ysm user show myuser

# Grant privileges
ysm user grant myuser -d mydb --privileges SELECT,INSERT,UPDATE

# Revoke privileges
ysm user revoke myuser -d mydb --privileges ALL

# Drop user
ysm user drop myuser
```

#### Database Management

```bash
# Create database
ysm db create mydb

# Create with charset/collation
ysm db create mydb --charset utf8mb4 --collation utf8mb4_unicode_ci

# Setup database for an app (creates db + user)
ysm db setup --template wordpress --name wp_site --user wp_user

# List available templates
ysm db templates

# Drop database
ysm db drop mydb
```

#### Statistics

```bash
# Show server summary
ysm stats summary

# Show database sizes
ysm stats databases

# Show table sizes
ysm stats tables mydb

# Show connection info
ysm stats connections

# Show performance metrics
ysm stats performance
```

#### Cluster Management

```bash
# Show cluster status
ysm cluster status

# List cluster nodes
ysm cluster nodes

# Quick health check
ysm cluster health

# Galera-specific status (MariaDB)
ysm cluster galera

# Replication details
ysm cluster replication
```

#### System Variables

```bash
# Set a session variable
ysm set foreign_key_checks 0

# Set a global variable
ysm set --global max_connections 200

# Show variables matching a pattern
ysm set --show "character%"

# List common variables
ysm set --list
```

#### Connection Profiles

```bash
# Add a new profile
ysm profile add local -H localhost -P 3306 -u root -p mypassword

# Add PostgreSQL profile
ysm profile add pglocal -t postgres -H localhost -P 5432 -u postgres

# Set default profile
ysm profile use local

# Set profile variables
ysm profile set-var local foreign_key_checks 0
```

### Debug Flags

```bash
# Verbose output (info level)
ysm -v import backup.sql -d mydb

# Debug output (shows file:line)
ysm --debug import backup.sql -d mydb

# Trace output (most verbose)
ysm --trace import backup.sql -d mydb

# Show stack traces on errors
ysm --stack-trace import backup.sql -d mydb

# Log to file
ysm --log-file /var/log/ysm.log import backup.sql -d mydb
```

## Compression Support

YSM supports multiple compression formats for import/export:

| Format | Extension | Notes |
|--------|-----------|-------|
| gzip | `.gz` | Built-in support |
| xz | `.xz` | Built-in support |
| zstd | `.zst` | Built-in support |

Compression is auto-detected from file extension.

## PostgreSQL Native Formats

For PostgreSQL, YSM supports native dump formats using `pg_dump` and `pg_restore`:

| Format | Extension | Notes |
|--------|-----------|-------|
| Plain SQL | `.sql` | Default, compatible with any PostgreSQL version |
| Custom | `.dump`, `.pgdump` | Compressed, fast restore with `pg_restore` |
| Tar | `.tar` | Archive format, supports parallel restore |
| Directory | `<dir>/` | Directory of files, best for parallel restore |

**Benefits of custom format (.dump):**
- Built-in compression (smaller file sizes)
- Selective restore (restore specific tables/schemas)
- Parallel restore with `--jobs=N` flag
- Auto-detected from extension or use `--format=custom`

**Example workflow:**
```bash
# Export PostgreSQL database in custom format
ysm export mydb -o backup.dump --format=custom

# Restore with parallel processing
ysm import backup.dump -d mydb --create --jobs=4
```

## Configuration

Configuration is stored in `~/.config/ysm/config.yaml`:

```yaml
default_profile: local
profiles:
  local:
    type: mariadb
    host: localhost
    port: 3306
    user: root
    password: mypassword
    database: mydb
    variables:
      foreign_key_checks: "0"
  postgres:
    type: postgres
    host: localhost
    port: 5432
    user: postgres
    password: secret
```

### Backup Storage

Backups are stored in `~/.local/share/ysm/backups/` (or `$XDG_DATA_HOME/ysm/backups/`).

### Backup Schedules

Schedules are stored in `~/.config/ysm/schedules.json`:

```json
{
  "schedules": {
    "mydb": {
      "enabled": true,
      "interval": "daily",
      "compression": "zstd",
      "retain_count": 7
    }
  }
}
```

### Keybindings

Keybindings are fully customizable and stored in `~/.config/ysm/keybindings.yaml`:

```yaml
# Global keybindings (work in all views)
global:
  quit: q
  back: esc
  select: enter
  filter: /
  refresh: r
  up: up
  down: down

# View-specific keybindings
databases:
  new_database: n
  dashboard: d
  cluster: c
  users: u
  backup: b
  import: i
  export: e
  query: s
  variables: v
  settings: "?"
```

**Available keys:** `a-z`, `0-9`, `enter`, `esc`, `tab`, `space`, `backspace`, `delete`, `up`, `down`, `left`, `right`, `home`, `end`, `pgup`, `pgdown`, `f1-f12`, `ctrl+<key>`, `shift+<key>`, `alt+<key>`

To customize keybindings:
1. Press `?` in the TUI to open keybindings settings
2. Navigate with arrow keys, press Enter to rebind
3. Press the new key you want to assign
4. Changes are saved automatically

To reset all keybindings to defaults, delete `~/.config/ysm/keybindings.yaml` and restart YSM~

## Man Page

After installation, view the man page:

```bash
man ysm
```

## License

This project is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0).

See [LICENSE](LICENSE) for details.

## Source Code

https://github.com/blubskye/yandere_sql_manager
