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
	"strings"
	"text/tabwriter"

	"github.com/blubskye/yandere_sql_manager/internal/db"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	dbCharset    string
	dbCollation  string
	dbTemplate   string
	dbUsername   string
	dbPassword   string
	dbHostFlag   string
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database management commands",
	Long: `Create and manage databases with advanced options.

Subcommands:
  create    - Create a new database
  drop      - Drop a database
  setup     - Setup database and user for an application
  templates - List available application templates`,
}

var dbCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new database",
	Long: `Create a new database with optional charset and collation.

Examples:
  ysm db create mydb
  ysm db create mydb --charset utf8mb4 --collation utf8mb4_unicode_ci`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		name := args[0]

		// Use appropriate defaults based on database type
		charset := dbCharset
		collation := dbCollation

		if conn.Config.Type == db.DatabaseTypePostgres {
			// PostgreSQL defaults
			if charset == "" {
				charset = "UTF8"
			}
			// PostgreSQL collation is optional - empty means use system default
			// Don't set a default collation for PostgreSQL as it depends on system locale
		} else {
			// MariaDB/MySQL defaults
			if charset == "" {
				charset = "utf8mb4"
			}
			if collation == "" {
				collation = "utf8mb4_unicode_ci"
			}
		}

		if err := conn.CreateDatabaseWithOptions(name, charset, collation); err != nil {
			return err
		}

		fmt.Printf("Database '%s' created successfully.\n", name)
		fmt.Printf("  Charset:   %s\n", charset)
		if collation != "" {
			fmt.Printf("  Collation: %s\n", collation)
		}

		return nil
	},
}

var dbDropCmd = &cobra.Command{
	Use:   "drop <name>",
	Short: "Drop a database",
	Long: `Drop a database. This action cannot be undone.

Examples:
  ysm db drop mydb`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		name := args[0]

		// Confirm deletion
		fmt.Printf("WARNING: This will permanently delete database '%s' and all its data.\n", name)
		fmt.Printf("Are you sure you want to continue? [y/N]: ")
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(confirm) != "y" && strings.ToLower(confirm) != "yes" {
			fmt.Println("Aborted.")
			return nil
		}

		if err := conn.DropDatabase(name); err != nil {
			return err
		}

		fmt.Printf("Database '%s' dropped successfully.\n", name)
		return nil
	},
}

var dbSetupCmd = &cobra.Command{
	Use:   "setup <database-name>",
	Short: "Setup database and user for an application",
	Long: `Create a database and user pair optimized for a specific application.

Examples:
  ysm db setup myblog --template wordpress --user bloguser
  ysm db setup myapp --template laravel --user appuser -p secretpass`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		dbName := args[0]

		// Get template
		templateName := dbTemplate
		if templateName == "" {
			templateName = "default"
		}

		template, err := db.GetTemplate(templateName)
		if err != nil {
			return err
		}

		// Get username
		username := dbUsername
		if username == "" {
			username = dbName + "_user"
		}

		// Get or prompt for password
		pwd := dbPassword
		if pwd == "" {
			fmt.Print("Enter password for new user: ")
			pwdBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				return fmt.Errorf("failed to read password: %w", err)
			}
			pwd = string(pwdBytes)

			// Confirm password
			fmt.Print("Confirm password: ")
			confirmBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				return fmt.Errorf("failed to read password: %w", err)
			}
			if pwd != string(confirmBytes) {
				return fmt.Errorf("passwords do not match")
			}
		}

		if pwd == "" {
			return fmt.Errorf("password is required")
		}

		// Get host
		host := dbHostFlag
		if host == "" {
			host = "localhost"
		}

		// Override charset/collation if specified
		if dbCharset != "" {
			template.Charset = dbCharset
		}
		if dbCollation != "" {
			template.Collation = dbCollation
		}

		fmt.Printf("Setting up database for %s...\n", template.Description)
		fmt.Printf("  Database: %s\n", dbName)
		fmt.Printf("  User:     %s@%s\n", username, host)
		fmt.Printf("  Charset:  %s\n", template.Charset)
		if template.Collation != "" {
			fmt.Printf("  Collation: %s\n", template.Collation)
		}
		fmt.Println()

		if err := conn.SetupAppDatabase(template, dbName, username, pwd, host); err != nil {
			return err
		}

		fmt.Println("Setup completed successfully!")
		fmt.Println()
		fmt.Println("Connection details:")
		fmt.Printf("  Host:     %s\n", conn.Config.Host)
		fmt.Printf("  Port:     %d\n", conn.Config.Port)
		fmt.Printf("  Database: %s\n", dbName)
		fmt.Printf("  User:     %s\n", username)

		return nil
	},
}

var dbTemplatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "List available application templates",
	RunE: func(cmd *cobra.Command, args []string) error {
		templates := db.DefaultTemplates()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tDESCRIPTION\tCHARSET\tCOLLATION")
		fmt.Fprintln(w, "----\t-----------\t-------\t---------")

		for _, t := range templates {
			collation := t.Collation
			if collation == "" {
				collation = "(default)"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				t.Name,
				t.Description,
				t.Charset,
				collation,
			)
		}

		return w.Flush()
	},
}

var dbCharsetsCmd = &cobra.Command{
	Use:   "charsets",
	Short: "List available character sets",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		charsets, err := conn.GetCharsets()
		if err != nil {
			// Fall back to common charsets
			charsets = db.CommonCharsets()
		}

		fmt.Println("Available character sets:")
		for _, cs := range charsets {
			fmt.Printf("  %s\n", cs)
		}

		return nil
	},
}

var dbCollationsCmd = &cobra.Command{
	Use:   "collations [charset]",
	Short: "List available collations",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		charset := ""
		if len(args) > 0 {
			charset = args[0]
		}

		collations, err := conn.GetCollations(charset)
		if err != nil {
			// Fall back to common collations
			if charset != "" {
				collations = db.CommonCollationsForCharset(charset)
			}
		}

		if charset != "" {
			fmt.Printf("Available collations for %s:\n", charset)
		} else {
			fmt.Println("Available collations:")
		}

		for _, c := range collations {
			fmt.Printf("  %s\n", c)
		}

		return nil
	},
}

func init() {
	// Create flags
	dbCreateCmd.Flags().StringVar(&dbCharset, "charset", "", "Character set (default: utf8mb4)")
	dbCreateCmd.Flags().StringVar(&dbCollation, "collation", "", "Collation (default: utf8mb4_unicode_ci)")

	// Setup flags
	dbSetupCmd.Flags().StringVarP(&dbTemplate, "template", "t", "", "Application template (default: default)")
	dbSetupCmd.Flags().StringVarP(&dbUsername, "user", "U", "", "Username for the database (default: <dbname>_user)")
	dbSetupCmd.Flags().StringVarP(&dbPassword, "password", "p", "", "Password for the user")
	dbSetupCmd.Flags().StringVar(&dbHostFlag, "host", "localhost", "Host for the user (MariaDB only)")
	dbSetupCmd.Flags().StringVar(&dbCharset, "charset", "", "Override template charset")
	dbSetupCmd.Flags().StringVar(&dbCollation, "collation", "", "Override template collation")

	dbCmd.AddCommand(dbCreateCmd)
	dbCmd.AddCommand(dbDropCmd)
	dbCmd.AddCommand(dbSetupCmd)
	dbCmd.AddCommand(dbTemplatesCmd)
	dbCmd.AddCommand(dbCharsetsCmd)
	dbCmd.AddCommand(dbCollationsCmd)
}
