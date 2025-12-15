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
	"path/filepath"

	"github.com/blubskye/yandere_sql_manager/internal/config"
	"github.com/blubskye/yandere_sql_manager/internal/db"
	"github.com/blubskye/yandere_sql_manager/internal/logging"
	"github.com/blubskye/yandere_sql_manager/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	// Global flags
	dbType   string
	host     string
	port     int
	user     string
	password string
	socket   string
	profile  string
	database string

	// Debug flags
	verbose    bool
	debug      bool
	trace      bool
	logFile    string
	stackTrace bool

	// Flag changed tracking
	typeChanged bool
	hostChanged bool
	portChanged bool

	// Config
	cfg *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "ysm",
	Short: "Yandere SQL Manager - Database management made easy",
	Long: `YSM (Yandere SQL Manager) - "I'll never let your databases go~"

A TUI and CLI tool for managing MariaDB and PostgreSQL databases.
Run without arguments to start the interactive TUI.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize logging based on flags
		initLogging()
	},
	PreRun: func(cmd *cobra.Command, args []string) {
		typeChanged = cmd.Flag("type").Changed
		hostChanged = cmd.Flag("host").Changed
		portChanged = cmd.Flag("port").Changed
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return startTUI()
	},
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global connection flags
	rootCmd.PersistentFlags().StringVarP(&dbType, "type", "t", "mariadb", "Database type (mariadb, postgres)")
	rootCmd.PersistentFlags().StringVarP(&host, "host", "H", "localhost", "Database host")
	rootCmd.PersistentFlags().IntVarP(&port, "port", "P", 0, "Database port (default: 3306 for MariaDB, 5432 for PostgreSQL)")
	rootCmd.PersistentFlags().StringVarP(&user, "user", "u", "", "Database user")
	rootCmd.PersistentFlags().StringVarP(&password, "password", "p", "", "Database password")
	rootCmd.PersistentFlags().StringVarP(&socket, "socket", "S", "", "Unix socket path")
	rootCmd.PersistentFlags().StringVar(&profile, "profile", "", "Connection profile to use")
	rootCmd.PersistentFlags().StringVarP(&database, "database", "d", "", "Database to use")

	// Debug and logging flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output (info level)")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug output (shows caller info)")
	rootCmd.PersistentFlags().BoolVar(&trace, "trace", false, "Enable trace output (most verbose)")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "Write logs to file (in addition to stderr)")
	rootCmd.PersistentFlags().BoolVar(&stackTrace, "stack-trace", false, "Show stack traces on errors")

	// Add subcommands
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(queryCmd)
	rootCmd.AddCommand(profileCmd)
	rootCmd.AddCommand(setCmd)
	rootCmd.AddCommand(userCmd)
	rootCmd.AddCommand(backupCmd)
	rootCmd.AddCommand(dbCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(clusterCmd)
	rootCmd.AddCommand(versionCmd)
}

func initConfig() {
	var err error
	cfg, err = config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
		cfg = &config.Config{
			Profiles: make(map[string]config.Profile),
		}
	}
}

func initLogging() {
	// Set log level based on flags (most verbose wins)
	if trace {
		logging.EnableTrace()
		logging.Debug("Trace logging enabled")
	} else if debug {
		logging.EnableDebug()
		logging.Debug("Debug logging enabled")
	} else if verbose {
		logging.SetLevel(logging.LevelInfo)
		logging.Info("Verbose logging enabled")
	}

	// Enable stack traces on errors if requested
	if stackTrace {
		logging.EnableStackOnError()
		logging.Debug("Stack traces on errors enabled")
	}

	// Set up log file if specified
	if logFile != "" {
		// Expand path
		if logFile[0] == '~' {
			home, _ := os.UserHomeDir()
			logFile = filepath.Join(home, logFile[1:])
		}

		if err := logging.SetLogFile(logFile); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to open log file: %v\n", err)
		} else {
			logging.Debug("Logging to file: %s", logFile)
		}
	}
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

// getConnectionConfig returns the connection configuration from flags or profile
func getConnectionConfig() (db.ConnectionConfig, error) {
	// If profile specified, use it
	if profile != "" {
		p, err := cfg.GetProfile(profile)
		if err != nil {
			return db.ConnectionConfig{}, err
		}
		connCfg := p.ToConnectionConfig()

		// Override with any explicitly set flags
		if typeChanged {
			connCfg.Type = db.DatabaseType(dbType)
		}
		if hostChanged {
			connCfg.Host = host
		}
		if portChanged {
			connCfg.Port = port
		}
		if user != "" {
			connCfg.User = user
		}
		if password != "" {
			connCfg.Password = password
		}
		if socket != "" {
			connCfg.Socket = socket
		}
		if database != "" {
			connCfg.Database = database
		}

		return connCfg, nil
	}

	// Check for default profile
	if cfg != nil && cfg.DefaultProfile != "" && user == "" {
		p, err := cfg.GetProfile(cfg.DefaultProfile)
		if err == nil {
			return p.ToConnectionConfig(), nil
		}
	}

	// Use flags directly
	if user == "" {
		return db.ConnectionConfig{}, fmt.Errorf("no user specified. Use -u/--user or set up a profile")
	}

	// Determine database type and default port
	connType := db.DatabaseType(dbType)
	connPort := port
	if connPort == 0 {
		connPort = db.DefaultPort(connType)
	}

	return db.ConnectionConfig{
		Type:     connType,
		Host:     host,
		Port:     connPort,
		User:     user,
		Password: password,
		Socket:   socket,
		Database: database,
	}, nil
}

// promptPassword prompts for password if not provided
func promptPassword() (string, error) {
	if password != "" {
		return password, nil
	}

	fmt.Print("Enter password: ")
	pwd, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}
	return string(pwd), nil
}

// connect establishes a database connection
func connect() (*db.Connection, error) {
	connCfg, err := getConnectionConfig()
	if err != nil {
		return nil, err
	}

	// Prompt for password if not provided
	if connCfg.Password == "" {
		pwd, err := promptPassword()
		if err != nil {
			return nil, err
		}
		connCfg.Password = pwd
	}

	conn, err := db.Connect(connCfg)
	if err != nil {
		return nil, err
	}

	// Apply profile variables if using a profile
	profileName := profile
	if profileName == "" && cfg != nil {
		profileName = cfg.DefaultProfile
	}

	if profileName != "" && cfg != nil {
		p, err := cfg.GetProfile(profileName)
		if err == nil && p.Variables != nil && len(p.Variables) > 0 {
			if err := conn.ApplyVariables(p.Variables); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to apply profile variables: %v\n", err)
			}
		}
	}

	return conn, nil
}

func startTUI() error {
	// Get connection config if available
	var connCfg *db.ConnectionConfig

	if profile != "" || user != "" {
		c, err := getConnectionConfig()
		if err == nil {
			connCfg = &c
		}
	}

	return tui.Run(connCfg)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("YSM (Yandere SQL Manager) v0.2.2")
		fmt.Println("\"I'll never let your databases go~\" <3")
		fmt.Println()
		fmt.Println("Copyright (C) 2025 blubskye")
		fmt.Println("License: GNU AGPL v3.0 <https://www.gnu.org/licenses/agpl-3.0.html>")
		fmt.Println("Source:  https://github.com/blubskye/yandere_sql_manager")
	},
}
