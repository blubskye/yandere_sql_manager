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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// BackupSchedule represents a backup schedule for a database
type BackupSchedule struct {
	ID          string          `json:"id"`
	Database    string          `json:"database"`
	Enabled     bool            `json:"enabled"`
	Interval    string          `json:"interval"` // "hourly", "daily", "weekly", "monthly"
	Compression CompressionType `json:"compression"`
	RetainCount int             `json:"retain_count"` // Number of backups to keep (0 = unlimited)
	LastRun     time.Time       `json:"last_run,omitempty"`
	NextRun     time.Time       `json:"next_run,omitempty"`
	Profile     string          `json:"profile,omitempty"` // Connection profile to use
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// ScheduleConfig holds all backup schedules
type ScheduleConfig struct {
	Schedules []BackupSchedule `json:"schedules"`
}

// GetSchedulesPath returns the path to the schedules config file
func GetSchedulesPath() (string, error) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		configHome = filepath.Join(home, ".config")
	}

	configDir := filepath.Join(configHome, "ysm")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	return filepath.Join(configDir, "schedules.json"), nil
}

// LoadSchedules loads all backup schedules
func LoadSchedules() (*ScheduleConfig, error) {
	path, err := GetSchedulesPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ScheduleConfig{Schedules: []BackupSchedule{}}, nil
		}
		return nil, fmt.Errorf("failed to read schedules: %w", err)
	}

	var config ScheduleConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse schedules: %w", err)
	}

	return &config, nil
}

// SaveSchedules saves all backup schedules
func SaveSchedules(config *ScheduleConfig) error {
	path, err := GetSchedulesPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schedules: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write schedules: %w", err)
	}

	return nil
}

// GetSchedule returns the schedule for a specific database
func GetSchedule(database string) (*BackupSchedule, error) {
	config, err := LoadSchedules()
	if err != nil {
		return nil, err
	}

	for _, s := range config.Schedules {
		if s.Database == database {
			return &s, nil
		}
	}

	return nil, fmt.Errorf("no schedule found for database: %s", database)
}

// SetSchedule creates or updates a schedule for a database
func SetSchedule(schedule BackupSchedule) error {
	config, err := LoadSchedules()
	if err != nil {
		return err
	}

	schedule.UpdatedAt = time.Now()

	// Find and update existing or add new
	found := false
	for i, s := range config.Schedules {
		if s.Database == schedule.Database {
			schedule.ID = s.ID
			schedule.CreatedAt = s.CreatedAt
			config.Schedules[i] = schedule
			found = true
			break
		}
	}

	if !found {
		schedule.ID = generateScheduleID()
		schedule.CreatedAt = time.Now()
		config.Schedules = append(config.Schedules, schedule)
	}

	// Calculate next run time
	updateNextRun(&schedule)

	return SaveSchedules(config)
}

// DeleteSchedule removes a schedule for a database
func DeleteSchedule(database string) error {
	config, err := LoadSchedules()
	if err != nil {
		return err
	}

	newSchedules := make([]BackupSchedule, 0, len(config.Schedules))
	for _, s := range config.Schedules {
		if s.Database != database {
			newSchedules = append(newSchedules, s)
		}
	}

	config.Schedules = newSchedules
	return SaveSchedules(config)
}

// ToggleSchedule enables or disables a schedule
func ToggleSchedule(database string, enabled bool) error {
	config, err := LoadSchedules()
	if err != nil {
		return err
	}

	for i, s := range config.Schedules {
		if s.Database == database {
			config.Schedules[i].Enabled = enabled
			config.Schedules[i].UpdatedAt = time.Now()
			if enabled {
				updateNextRun(&config.Schedules[i])
			}
			return SaveSchedules(config)
		}
	}

	return fmt.Errorf("no schedule found for database: %s", database)
}

// ListSchedules returns all schedules
func ListSchedules() ([]BackupSchedule, error) {
	config, err := LoadSchedules()
	if err != nil {
		return nil, err
	}
	return config.Schedules, nil
}

// GetDueSchedules returns schedules that are due for backup
func GetDueSchedules() ([]BackupSchedule, error) {
	config, err := LoadSchedules()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var due []BackupSchedule

	for _, s := range config.Schedules {
		if s.Enabled && !s.NextRun.IsZero() && s.NextRun.Before(now) {
			due = append(due, s)
		}
	}

	return due, nil
}

// MarkScheduleRun updates a schedule after a backup run
func MarkScheduleRun(database string, success bool) error {
	config, err := LoadSchedules()
	if err != nil {
		return err
	}

	for i, s := range config.Schedules {
		if s.Database == database {
			config.Schedules[i].LastRun = time.Now()
			config.Schedules[i].UpdatedAt = time.Now()
			updateNextRun(&config.Schedules[i])
			return SaveSchedules(config)
		}
	}

	return nil
}

// Helper functions

func generateScheduleID() string {
	return fmt.Sprintf("sched_%d", time.Now().UnixNano())
}

func updateNextRun(schedule *BackupSchedule) {
	if !schedule.Enabled {
		schedule.NextRun = time.Time{}
		return
	}

	now := time.Now()
	var next time.Time

	switch schedule.Interval {
	case "hourly":
		next = now.Add(time.Hour)
	case "daily":
		next = now.Add(24 * time.Hour)
	case "weekly":
		next = now.Add(7 * 24 * time.Hour)
	case "monthly":
		next = now.AddDate(0, 1, 0)
	default:
		next = now.Add(24 * time.Hour) // Default to daily
	}

	schedule.NextRun = next
}

// IntervalOptions returns available backup interval options
func IntervalOptions() []string {
	return []string{"hourly", "daily", "weekly", "monthly"}
}

// CleanupOldBackups removes old backups based on retention policy
func CleanupOldBackups(database string, retainCount int) error {
	if retainCount <= 0 {
		return nil // Keep all
	}

	backups, err := ListBackups()
	if err != nil {
		return err
	}

	// Filter backups for this database
	var dbBackups []BackupMetadata
	for _, b := range backups {
		for _, db := range b.Databases {
			if db == database {
				dbBackups = append(dbBackups, b)
				break
			}
		}
	}

	// Delete old backups (they're already sorted newest first)
	if len(dbBackups) > retainCount {
		for _, b := range dbBackups[retainCount:] {
			if err := DeleteBackup(b.ID); err != nil {
				return fmt.Errorf("failed to delete old backup %s: %w", b.ID, err)
			}
		}
	}

	return nil
}
