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

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/blubskye/yandere_sql_manager/internal/db"
	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	Profiles       map[string]Profile `yaml:"profiles"`
	DefaultProfile string             `yaml:"default_profile"`
}

// Profile holds connection settings for a database
type Profile struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password,omitempty"`
	Socket   string `yaml:"socket,omitempty"`
	Database string `yaml:"database,omitempty"`
}

// ToConnectionConfig converts a Profile to db.ConnectionConfig
func (p *Profile) ToConnectionConfig() db.ConnectionConfig {
	port := p.Port
	if port == 0 {
		port = db.DefaultPort()
	}
	return db.ConnectionConfig{
		Host:     p.Host,
		Port:     port,
		User:     p.User,
		Password: p.Password,
		Socket:   p.Socket,
		Database: p.Database,
	}
}

// ConfigDir returns the configuration directory path
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// Use XDG_CONFIG_HOME if set, otherwise ~/.config
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}

	return filepath.Join(configHome, "ysm"), nil
}

// ConfigPath returns the configuration file path
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Load loads the configuration from disk
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty config if file doesn't exist
			return &Config{
				Profiles: make(map[string]Profile),
			}, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]Profile)
	}

	return &cfg, nil
}

// Save saves the configuration to disk
func (c *Config) Save() error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	path, err := ConfigPath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetProfile returns a profile by name
func (c *Config) GetProfile(name string) (*Profile, error) {
	if name == "" {
		name = c.DefaultProfile
	}

	if name == "" {
		return nil, fmt.Errorf("no profile specified and no default profile set")
	}

	profile, ok := c.Profiles[name]
	if !ok {
		return nil, fmt.Errorf("profile '%s' not found", name)
	}

	return &profile, nil
}

// AddProfile adds or updates a profile
func (c *Config) AddProfile(name string, profile Profile) {
	c.Profiles[name] = profile
}

// RemoveProfile removes a profile
func (c *Config) RemoveProfile(name string) error {
	if _, ok := c.Profiles[name]; !ok {
		return fmt.Errorf("profile '%s' not found", name)
	}

	delete(c.Profiles, name)

	// Clear default if it was the removed profile
	if c.DefaultProfile == name {
		c.DefaultProfile = ""
	}

	return nil
}

// SetDefault sets the default profile
func (c *Config) SetDefault(name string) error {
	if name != "" {
		if _, ok := c.Profiles[name]; !ok {
			return fmt.Errorf("profile '%s' not found", name)
		}
	}

	c.DefaultProfile = name
	return nil
}

// ListProfiles returns all profile names
func (c *Config) ListProfiles() []string {
	names := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		names = append(names, name)
	}
	return names
}
