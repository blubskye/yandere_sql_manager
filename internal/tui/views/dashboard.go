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

package views

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/blubskye/yandere_sql_manager/internal/db"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DashboardView shows server statistics
type DashboardView struct {
	conn        *db.Connection
	width       int
	height      int
	err         error
	stats       *db.ServerStats
	loading     bool
	autoRefresh bool
	lastUpdate  time.Time
	statsMu     sync.RWMutex // Protects stats for background updates
	stopChan    chan struct{}
}

// Styles for the dashboard
var (
	dashboardBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#FF69B4")).
				Padding(0, 1)

	dashboardTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FF69B4"))

	dashboardValueStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF"))

	dashboardBarFull = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#44FF44"))

	dashboardBarEmpty = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#333333"))

	dashboardBarWarning = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFAA00"))

	dashboardBarDanger = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF4444"))
)

// NewDashboardView creates a new dashboard view
func NewDashboardView(conn *db.Connection, width, height int) *DashboardView {
	return &DashboardView{
		conn:     conn,
		width:    width,
		height:   height,
		loading:  true,
		stopChan: make(chan struct{}),
	}
}

// Init initializes the view
func (v *DashboardView) Init() tea.Cmd {
	return v.loadStats
}

func (v *DashboardView) loadStats() tea.Msg {
	stats, err := v.conn.GetServerStats()
	if err != nil {
		return err
	}
	return statsLoadedMsg{stats: stats}
}

// loadStatsBackground fetches stats in a background goroutine
func (v *DashboardView) loadStatsBackground() tea.Cmd {
	return func() tea.Msg {
		// Fetch stats in goroutine
		resultChan := make(chan statsLoadedMsg, 1)
		errChan := make(chan error, 1)

		go func() {
			stats, err := v.conn.GetServerStats()
			if err != nil {
				errChan <- err
				return
			}
			resultChan <- statsLoadedMsg{stats: stats}
		}()

		// Wait for result or stop signal
		select {
		case result := <-resultChan:
			return result
		case err := <-errChan:
			return err
		case <-v.stopChan:
			return nil
		}
	}
}

type statsLoadedMsg struct {
	stats *db.ServerStats
}

type tickMsg struct{}

// Update handles messages
func (v *DashboardView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			v.loading = true
			return v, v.loadStats
		case "a":
			v.autoRefresh = !v.autoRefresh
			if v.autoRefresh {
				return v, v.tick()
			}
			return v, nil
		case "esc", "backspace", "q":
			// Stop any background operations
			v.autoRefresh = false
			close(v.stopChan)
			v.stopChan = make(chan struct{}) // Reset for potential reuse
			return v, func() tea.Msg {
				return SwitchViewMsg{View: "databases"}
			}
		}

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height

	case statsLoadedMsg:
		v.statsMu.Lock()
		v.stats = msg.stats
		v.statsMu.Unlock()
		v.loading = false
		v.lastUpdate = time.Now()
		if v.autoRefresh {
			return v, v.tick()
		}
		return v, nil

	case tickMsg:
		if v.autoRefresh {
			v.loading = true
			return v, v.loadStatsBackground()
		}
		return v, nil

	case error:
		v.err = msg
		v.loading = false
		return v, nil
	}

	return v, nil
}

func (v *DashboardView) tick() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

// View renders the view
func (v *DashboardView) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Server Dashboard"))
	b.WriteString("\n\n")

	// Thread-safe stats access
	v.statsMu.RLock()
	stats := v.stats
	v.statsMu.RUnlock()

	if v.loading && stats == nil {
		b.WriteString("Loading statistics...\n")
		return b.String()
	}

	if v.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", v.err)))
		b.WriteString("\n\n")
	}

	if stats == nil {
		b.WriteString(helpStyle.Render("Press 'r' to refresh"))
		return b.String()
	}

	// Layout: 2 columns of boxes
	leftWidth := (v.width - 6) / 2
	rightWidth := leftWidth

	// Server Info Box
	serverInfo := v.renderServerInfo(leftWidth)

	// Connections Box
	connInfo := v.renderConnections(rightWidth)

	// Render first row
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, serverInfo, "  ", connInfo))
	b.WriteString("\n\n")

	// Storage Box
	storageInfo := v.renderStorage(leftWidth)

	// Performance Box
	perfInfo := v.renderPerformance(rightWidth)

	// Render second row
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, storageInfo, "  ", perfInfo))

	// Replication info for PostgreSQL
	if stats.Replication != nil {
		b.WriteString("\n\n")
		b.WriteString(v.renderReplication(leftWidth + rightWidth + 2))
	}

	b.WriteString("\n\n")

	// Status bar
	updateStatus := ""
	if v.loading {
		updateStatus = "Updating..."
	} else {
		updateStatus = fmt.Sprintf("Last update: %s", v.lastUpdate.Format("15:04:05"))
	}

	autoStatus := "off"
	if v.autoRefresh {
		autoStatus = "on (5s)"
	}

	b.WriteString(mutedStyle.Render(fmt.Sprintf("%s | Auto-refresh: %s", updateStatus, autoStatus)))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("r: Refresh | a: Toggle auto-refresh | Esc: Back | q: Quit"))

	return b.String()
}

func (v *DashboardView) renderServerInfo(width int) string {
	var content strings.Builder

	content.WriteString(dashboardTitleStyle.Render("Server Info"))
	content.WriteString("\n\n")
	content.WriteString(fmt.Sprintf("Version:   %s\n", v.stats.Version))
	content.WriteString(fmt.Sprintf("Uptime:    %s\n", db.FormatUptime(v.stats.Uptime)))
	content.WriteString(fmt.Sprintf("Databases: %d\n", len(v.stats.Databases)))

	// Calculate total size
	var totalSize int64
	for _, d := range v.stats.Databases {
		totalSize += d.Size
	}
	content.WriteString(fmt.Sprintf("Total Size: %s", db.FormatSize(totalSize)))

	return dashboardBoxStyle.Width(width).Render(content.String())
}

func (v *DashboardView) renderConnections(width int) string {
	var content strings.Builder

	content.WriteString(dashboardTitleStyle.Render("Connections"))
	content.WriteString("\n\n")

	conn := v.stats.Connections
	content.WriteString(fmt.Sprintf("Active: %d / %d\n", conn.Active, conn.Max))

	if conn.Max > 0 {
		usage := float64(conn.Active) / float64(conn.Max) * 100
		content.WriteString("\n")
		content.WriteString(v.renderBar(usage, width-4))
		content.WriteString(fmt.Sprintf(" %.1f%%", usage))
	}

	return dashboardBoxStyle.Width(width).Render(content.String())
}

func (v *DashboardView) renderStorage(width int) string {
	var content strings.Builder

	content.WriteString(dashboardTitleStyle.Render("Storage"))
	content.WriteString("\n\n")

	// Show top 5 databases by size
	maxShow := 5
	if len(v.stats.Databases) < maxShow {
		maxShow = len(v.stats.Databases)
	}

	// Find max size for bar scaling
	var maxSize int64
	for _, d := range v.stats.Databases {
		if d.Size > maxSize {
			maxSize = d.Size
		}
	}

	for i := 0; i < maxShow; i++ {
		d := v.stats.Databases[i]

		// Truncate name if too long
		name := d.Name
		maxNameLen := 15
		if len(name) > maxNameLen {
			name = name[:maxNameLen-2] + ".."
		}

		// Calculate percentage of max
		pct := float64(0)
		if maxSize > 0 {
			pct = float64(d.Size) / float64(maxSize) * 100
		}

		barWidth := width - 30
		if barWidth < 10 {
			barWidth = 10
		}

		bar := v.renderBarSimple(pct, barWidth)
		content.WriteString(fmt.Sprintf("%-15s %s %s\n", name, bar, db.FormatSize(d.Size)))
	}

	if len(v.stats.Databases) > maxShow {
		content.WriteString(mutedStyle.Render(fmt.Sprintf("\n... and %d more", len(v.stats.Databases)-maxShow)))
	}

	return dashboardBoxStyle.Width(width).Render(content.String())
}

func (v *DashboardView) renderPerformance(width int) string {
	var content strings.Builder

	content.WriteString(dashboardTitleStyle.Render("Performance"))
	content.WriteString("\n\n")

	perf := v.stats.Performance
	content.WriteString(fmt.Sprintf("Slow Queries: %d\n", perf.SlowQueries))

	if perf.CacheHitRate > 0 {
		content.WriteString(fmt.Sprintf("\nCache Hit Rate:\n"))
		content.WriteString(v.renderBar(perf.CacheHitRate, width-4))
		content.WriteString(fmt.Sprintf(" %.1f%%", perf.CacheHitRate))
	} else {
		content.WriteString("\n")
		content.WriteString(mutedStyle.Render("Cache stats unavailable"))
	}

	return dashboardBoxStyle.Width(width).Render(content.String())
}

func (v *DashboardView) renderReplication(width int) string {
	var content strings.Builder

	content.WriteString(dashboardTitleStyle.Render("Replication"))
	content.WriteString("\n\n")

	repl := v.stats.Replication
	if repl.IsReplica {
		content.WriteString("Status: Replica\n")
		content.WriteString(fmt.Sprintf("Lag (bytes): %d\n", repl.LagBytes))
		content.WriteString(fmt.Sprintf("Lag (time):  %.2fs", repl.LagSeconds))
	} else {
		content.WriteString("Status: Primary")
	}

	return dashboardBoxStyle.Width(width).Render(content.String())
}

func (v *DashboardView) renderBar(percent float64, width int) string {
	if width < 5 {
		width = 5
	}

	filled := int(percent / 100 * float64(width))
	if filled > width {
		filled = width
	}

	var bar strings.Builder

	// Determine color based on percentage
	barStyle := dashboardBarFull
	if percent >= 90 {
		barStyle = dashboardBarDanger
	} else if percent >= 70 {
		barStyle = dashboardBarWarning
	}

	for i := 0; i < width; i++ {
		if i < filled {
			bar.WriteString(barStyle.Render("█"))
		} else {
			bar.WriteString(dashboardBarEmpty.Render("░"))
		}
	}

	return bar.String()
}

func (v *DashboardView) renderBarSimple(percent float64, width int) string {
	if width < 5 {
		width = 5
	}

	filled := int(percent / 100 * float64(width))
	if filled > width {
		filled = width
	}

	var bar strings.Builder

	for i := 0; i < width; i++ {
		if i < filled {
			bar.WriteString(dashboardBarFull.Render("█"))
		} else {
			bar.WriteString(dashboardBarEmpty.Render("░"))
		}
	}

	return bar.String()
}
