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

	"github.com/blubskye/yandere_sql_manager/internal/config"
	"github.com/spf13/cobra"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage connection profiles",
	Long: `Manage saved connection profiles.

Profiles are stored in ~/.config/ysm/config.yaml`,
}

var profileListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(cfg.Profiles) == 0 {
			fmt.Println("No profiles configured.")
			fmt.Println("Use 'ysm profile add <name>' to create one.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tHOST\tPORT\tUSER\tDATABASE\tDEFAULT")
		fmt.Fprintln(w, "----\t----\t----\t----\t--------\t-------")

		for name, p := range cfg.Profiles {
			isDefault := ""
			if name == cfg.DefaultProfile {
				isDefault = "*"
			}
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\n",
				name, p.Host, p.Port, p.User, p.Database, isDefault)
		}
		w.Flush()

		return nil
	},
}

var profileAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new profile",
	Long: `Add a new connection profile.

Examples:
  ysm profile add local -H localhost -u root
  ysm profile add production -H db.example.com -u admin -P 3307`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Check if profile already exists
		if _, exists := cfg.Profiles[name]; exists {
			fmt.Printf("Profile '%s' already exists. Use 'ysm profile update' to modify.\n", name)
			return nil
		}

		p := config.Profile{
			Host:     host,
			Port:     port,
			User:     user,
			Password: password,
			Socket:   socket,
			Database: database,
		}

		// Validate required fields
		if p.User == "" {
			return fmt.Errorf("user is required. Use -u/--user")
		}

		cfg.AddProfile(name, p)

		// Set as default if it's the first profile
		if len(cfg.Profiles) == 1 {
			cfg.DefaultProfile = name
		}

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("Profile '%s' added successfully.\n", name)
		if cfg.DefaultProfile == name {
			fmt.Println("Set as default profile.")
		}

		return nil
	},
}

var profileRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm", "delete"},
	Short:   "Remove a profile",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if err := cfg.RemoveProfile(name); err != nil {
			return err
		}

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("Profile '%s' removed.\n", name)
		return nil
	},
}

var profileUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set the default profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if err := cfg.SetDefault(name); err != nil {
			return err
		}

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("Default profile set to '%s'.\n", name)
		return nil
	},
}

var profileShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show profile details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		p, err := cfg.GetProfile(name)
		if err != nil {
			return err
		}

		fmt.Printf("Profile: %s\n", name)
		fmt.Printf("  Host:     %s\n", p.Host)
		fmt.Printf("  Port:     %d\n", p.Port)
		fmt.Printf("  User:     %s\n", p.User)
		fmt.Printf("  Database: %s\n", p.Database)
		if p.Socket != "" {
			fmt.Printf("  Socket:   %s\n", p.Socket)
		}
		if p.Password != "" {
			fmt.Printf("  Password: ****\n")
		}
		if name == cfg.DefaultProfile {
			fmt.Println("  (default)")
		}

		return nil
	},
}

func init() {
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileAddCmd)
	profileCmd.AddCommand(profileRemoveCmd)
	profileCmd.AddCommand(profileUseCmd)
	profileCmd.AddCommand(profileShowCmd)
}
