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

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	userHost       string
	userPassword   string
	grantDatabase  string
	grantTable     string
	grantPrivileges []string
)

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Manage database users",
	Long: `Manage database users and their privileges.

Subcommands:
  list    - List all users
  create  - Create a new user
  drop    - Drop a user
  show    - Show user privileges
  grant   - Grant privileges to a user
  revoke  - Revoke privileges from a user`,
}

var userListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all users",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		users, err := conn.ListUsers()
		if err != nil {
			return err
		}

		if len(users) == 0 {
			fmt.Println("No users found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "USER\tHOST")
		fmt.Fprintln(w, "----\t----")
		for _, u := range users {
			host := u.Host
			if host == "" {
				host = "(all)"
			}
			fmt.Fprintf(w, "%s\t%s\n", u.Username, host)
		}
		return w.Flush()
	},
}

var userCreateCmd = &cobra.Command{
	Use:   "create <username>",
	Short: "Create a new user",
	Long: `Create a new database user.

Examples:
  ysm user create myuser -p mypassword
  ysm user create myuser --host '%' -p mypassword
  ysm user create appuser -p pass123 --host localhost`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		username := args[0]

		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		// Prompt for password if not provided
		pwd := userPassword
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

		host := userHost
		if host == "" {
			host = "localhost"
		}

		if err := conn.CreateUser(username, host, pwd); err != nil {
			return err
		}

		fmt.Printf("User '%s'@'%s' created successfully.\n", username, host)
		return nil
	},
}

var userDropCmd = &cobra.Command{
	Use:   "drop <username>",
	Short: "Drop a user",
	Long: `Drop a database user.

Examples:
  ysm user drop myuser
  ysm user drop myuser --host '%'`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		username := args[0]

		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		host := userHost
		if host == "" {
			host = "localhost"
		}

		// Confirm deletion
		fmt.Printf("Are you sure you want to drop user '%s'@'%s'? [y/N]: ", username, host)
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(confirm) != "y" && strings.ToLower(confirm) != "yes" {
			fmt.Println("Aborted.")
			return nil
		}

		if err := conn.DropUser(username, host); err != nil {
			return err
		}

		fmt.Printf("User '%s'@'%s' dropped successfully.\n", username, host)
		return nil
	},
}

var userShowCmd = &cobra.Command{
	Use:   "show <username>",
	Short: "Show user privileges",
	Long: `Show privileges for a database user.

Examples:
  ysm user show myuser
  ysm user show myuser --host '%'`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		username := args[0]

		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		host := userHost
		if host == "" {
			host = "localhost"
		}

		grants, err := conn.GetUserGrants(username, host)
		if err != nil {
			return err
		}

		if len(grants) == 0 {
			fmt.Printf("No grants found for '%s'@'%s'.\n", username, host)
			return nil
		}

		fmt.Printf("Grants for '%s'@'%s':\n\n", username, host)

		// Check if we have raw grant text (MariaDB) or structured (PostgreSQL)
		if grants[0].GrantText != "" {
			// MariaDB format
			for _, g := range grants {
				fmt.Println(g.GrantText)
			}
		} else {
			// PostgreSQL format
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "DATABASE\tOBJECT\tPRIVILEGE")
			fmt.Fprintln(w, "--------\t------\t---------")
			for _, g := range grants {
				fmt.Fprintf(w, "%s\t%s\t%s\n", g.Database, g.Table, g.Privilege)
			}
			w.Flush()
		}

		return nil
	},
}

var userGrantCmd = &cobra.Command{
	Use:   "grant <username>",
	Short: "Grant privileges to a user",
	Long: `Grant privileges to a database user.

Examples:
  ysm user grant myuser -d mydb
  ysm user grant myuser -d mydb --privileges SELECT,INSERT,UPDATE
  ysm user grant myuser -d mydb -t mytable --privileges SELECT`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		username := args[0]

		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		host := userHost
		if host == "" {
			host = "localhost"
		}

		privs := grantPrivileges
		if len(privs) == 0 {
			privs = []string{"ALL PRIVILEGES"}
		}

		if err := conn.GrantPrivileges(username, host, privs, grantDatabase, grantTable); err != nil {
			return err
		}

		target := "*.*"
		if grantDatabase != "" && grantTable != "" {
			target = fmt.Sprintf("%s.%s", grantDatabase, grantTable)
		} else if grantDatabase != "" {
			target = fmt.Sprintf("%s.*", grantDatabase)
		}

		fmt.Printf("Granted %s on %s to '%s'@'%s'.\n",
			strings.Join(privs, ", "), target, username, host)
		return nil
	},
}

var userRevokeCmd = &cobra.Command{
	Use:   "revoke <username>",
	Short: "Revoke privileges from a user",
	Long: `Revoke privileges from a database user.

Examples:
  ysm user revoke myuser -d mydb
  ysm user revoke myuser -d mydb --privileges SELECT,INSERT
  ysm user revoke myuser -d mydb -t mytable --privileges ALL`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		username := args[0]

		conn, err := connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		host := userHost
		if host == "" {
			host = "localhost"
		}

		privs := grantPrivileges
		if len(privs) == 0 {
			privs = []string{"ALL PRIVILEGES"}
		}

		if err := conn.RevokePrivileges(username, host, privs, grantDatabase, grantTable); err != nil {
			return err
		}

		target := "*.*"
		if grantDatabase != "" && grantTable != "" {
			target = fmt.Sprintf("%s.%s", grantDatabase, grantTable)
		} else if grantDatabase != "" {
			target = fmt.Sprintf("%s.*", grantDatabase)
		}

		fmt.Printf("Revoked %s on %s from '%s'@'%s'.\n",
			strings.Join(privs, ", "), target, username, host)
		return nil
	},
}

func init() {
	// Common flags
	userCreateCmd.Flags().StringVar(&userHost, "host", "localhost", "Host for the user (MariaDB only)")
	userCreateCmd.Flags().StringVarP(&userPassword, "password", "p", "", "Password for the user")

	userDropCmd.Flags().StringVar(&userHost, "host", "localhost", "Host for the user (MariaDB only)")

	userShowCmd.Flags().StringVar(&userHost, "host", "localhost", "Host for the user (MariaDB only)")

	userGrantCmd.Flags().StringVar(&userHost, "host", "localhost", "Host for the user (MariaDB only)")
	userGrantCmd.Flags().StringVarP(&grantDatabase, "db", "d", "", "Database to grant access to")
	userGrantCmd.Flags().StringVarP(&grantTable, "table", "t", "", "Table to grant access to")
	userGrantCmd.Flags().StringSliceVar(&grantPrivileges, "privileges", []string{}, "Privileges to grant (comma-separated)")

	userRevokeCmd.Flags().StringVar(&userHost, "host", "localhost", "Host for the user (MariaDB only)")
	userRevokeCmd.Flags().StringVarP(&grantDatabase, "db", "d", "", "Database to revoke access from")
	userRevokeCmd.Flags().StringVarP(&grantTable, "table", "t", "", "Table to revoke access from")
	userRevokeCmd.Flags().StringSliceVar(&grantPrivileges, "privileges", []string{}, "Privileges to revoke (comma-separated)")

	userCmd.AddCommand(userListCmd)
	userCmd.AddCommand(userCreateCmd)
	userCmd.AddCommand(userDropCmd)
	userCmd.AddCommand(userShowCmd)
	userCmd.AddCommand(userGrantCmd)
	userCmd.AddCommand(userRevokeCmd)
}
