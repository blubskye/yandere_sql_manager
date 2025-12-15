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
	"time"

	"github.com/blubskye/yandere_sql_manager/internal/db"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Cluster view modes
type clusterMode int

const (
	clusterModeStatus clusterMode = iota
	clusterModeNodes
	clusterModeGalera
	clusterModeReplication
)

// ClusterView shows cluster and replication status
type ClusterView struct {
	conn        *db.Connection
	width       int
	height      int
	err         error
	mode        clusterMode
	loading     bool
	autoRefresh bool
	lastUpdate  time.Time

	// Status data
	clusterStatus *db.ClusterStatus
	galeraStatus  *db.GaleraStatus
	replStatus    *db.ReplicationStatus
}

// Styles for the cluster view
var (
	clusterBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF69B4")).
			Padding(0, 1)

	clusterTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FF69B4"))

	clusterHealthyStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#44FF44"))

	clusterUnhealthyStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FF4444"))

	clusterWarningStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFAA00"))

	clusterNodeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF"))
)

// NewClusterView creates a new cluster view
func NewClusterView(conn *db.Connection, width, height int) *ClusterView {
	return &ClusterView{
		conn:    conn,
		width:   width,
		height:  height,
		loading: true,
		mode:    clusterModeStatus,
	}
}

// Init initializes the view
func (v *ClusterView) Init() tea.Cmd {
	return v.loadClusterStatus
}

func (v *ClusterView) loadClusterStatus() tea.Msg {
	status, err := v.conn.GetClusterStatus()
	if err != nil {
		return err
	}
	return clusterStatusLoadedMsg{status: status}
}

func (v *ClusterView) loadGaleraStatus() tea.Msg {
	status, err := v.conn.GetGaleraStatus()
	if err != nil {
		return err
	}
	return galeraStatusLoadedMsg{status: status}
}

func (v *ClusterView) loadReplicationStatus() tea.Msg {
	status, err := v.conn.GetMariaDBReplicationStatus()
	if err != nil {
		return err
	}
	return replicationStatusLoadedMsg{status: status}
}

type clusterStatusLoadedMsg struct {
	status *db.ClusterStatus
}

type galeraStatusLoadedMsg struct {
	status *db.GaleraStatus
}

type replicationStatusLoadedMsg struct {
	status *db.ReplicationStatus
}

type clusterTickMsg struct{}

// Update handles messages
func (v *ClusterView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "1":
			v.mode = clusterModeStatus
			v.loading = true
			return v, v.loadClusterStatus
		case "2":
			v.mode = clusterModeNodes
			v.loading = true
			return v, v.loadClusterStatus
		case "3":
			if v.conn.Config.Type == db.DatabaseTypeMariaDB {
				v.mode = clusterModeGalera
				v.loading = true
				return v, v.loadGaleraStatus
			}
		case "4":
			v.mode = clusterModeReplication
			v.loading = true
			if v.conn.Config.Type == db.DatabaseTypeMariaDB {
				return v, v.loadReplicationStatus
			}
			return v, v.loadClusterStatus
		case "r":
			v.loading = true
			return v, v.getLoadCmd()
		case "a":
			v.autoRefresh = !v.autoRefresh
			if v.autoRefresh {
				return v, v.tick()
			}
			return v, nil
		case "esc", "backspace", "q":
			return v, func() tea.Msg {
				return SwitchViewMsg{View: "databases"}
			}
		}

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height

	case clusterStatusLoadedMsg:
		v.clusterStatus = msg.status
		v.loading = false
		v.lastUpdate = time.Now()
		v.err = nil
		if v.autoRefresh {
			return v, v.tick()
		}
		return v, nil

	case galeraStatusLoadedMsg:
		v.galeraStatus = msg.status
		v.loading = false
		v.lastUpdate = time.Now()
		v.err = nil
		if v.autoRefresh {
			return v, v.tick()
		}
		return v, nil

	case replicationStatusLoadedMsg:
		v.replStatus = msg.status
		v.loading = false
		v.lastUpdate = time.Now()
		v.err = nil
		if v.autoRefresh {
			return v, v.tick()
		}
		return v, nil

	case clusterTickMsg:
		if v.autoRefresh {
			return v, v.getLoadCmd()
		}
		return v, nil

	case error:
		v.err = msg
		v.loading = false
		return v, nil
	}

	return v, nil
}

func (v *ClusterView) getLoadCmd() tea.Cmd {
	switch v.mode {
	case clusterModeGalera:
		return v.loadGaleraStatus
	case clusterModeReplication:
		if v.conn.Config.Type == db.DatabaseTypeMariaDB {
			return v.loadReplicationStatus
		}
		return v.loadClusterStatus
	default:
		return v.loadClusterStatus
	}
}

func (v *ClusterView) tick() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return clusterTickMsg{}
	})
}

// View renders the view
func (v *ClusterView) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Cluster / Replication"))
	b.WriteString("\n\n")

	if v.loading && v.clusterStatus == nil && v.galeraStatus == nil && v.replStatus == nil {
		b.WriteString("Loading cluster status...\n")
		return b.String()
	}

	if v.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", v.err)))
		b.WriteString("\n\n")
	}

	// Tab bar
	b.WriteString(v.renderTabs())
	b.WriteString("\n\n")

	// Content based on mode
	switch v.mode {
	case clusterModeStatus:
		b.WriteString(v.renderStatus())
	case clusterModeNodes:
		b.WriteString(v.renderNodes())
	case clusterModeGalera:
		b.WriteString(v.renderGalera())
	case clusterModeReplication:
		b.WriteString(v.renderReplication())
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
	b.WriteString(helpStyle.Render("1-4: Switch tabs | r: Refresh | a: Auto-refresh | Esc: Back | q: Quit"))

	return b.String()
}

func (v *ClusterView) renderTabs() string {
	tabs := []string{"[1] Status", "[2] Nodes"}

	if v.conn.Config.Type == db.DatabaseTypeMariaDB {
		tabs = append(tabs, "[3] Galera")
	}

	tabs = append(tabs, "[4] Replication")

	// Highlight current tab
	var rendered []string
	for i, tab := range tabs {
		tabMode := clusterMode(i)
		if i == 3 {
			tabMode = clusterModeReplication
		}

		if tabMode == v.mode {
			rendered = append(rendered, selectedStyle.Render(tab))
		} else {
			rendered = append(rendered, mutedStyle.Render(tab))
		}
	}

	return strings.Join(rendered, "  ")
}

func (v *ClusterView) renderStatus() string {
	if v.clusterStatus == nil {
		return helpStyle.Render("Press 'r' to refresh")
	}

	var b strings.Builder
	status := v.clusterStatus

	leftWidth := (v.width - 6) / 2
	rightWidth := leftWidth

	// Overview Box
	var overview strings.Builder
	overview.WriteString(clusterTitleStyle.Render("Overview"))
	overview.WriteString("\n\n")

	if status.Type == db.ClusterTypeNone {
		overview.WriteString("Type: Standalone (no cluster/replication)\n")
		overview.WriteString(mutedStyle.Render("\nThis server is not part of a cluster\nor replication setup."))
	} else {
		overview.WriteString(fmt.Sprintf("Type:    %s\n", formatClusterType(status.Type)))
		overview.WriteString(fmt.Sprintf("Role:    %s\n", formatRole(status.IsPrimary)))
		overview.WriteString(fmt.Sprintf("Nodes:   %d\n", status.NodeCount))
		overview.WriteString("\n")
		overview.WriteString("Health:  ")
		if status.IsHealthy {
			overview.WriteString(clusterHealthyStyle.Render("Healthy"))
		} else {
			overview.WriteString(clusterUnhealthyStyle.Render("Unhealthy"))
		}
	}

	overviewBox := clusterBoxStyle.Width(leftWidth).Render(overview.String())

	// Local Node Box
	var localNode strings.Builder
	localNode.WriteString(clusterTitleStyle.Render("Local Node"))
	localNode.WriteString("\n\n")

	if status.LocalNode != nil {
		localNode.WriteString(fmt.Sprintf("Role:  %s\n", status.LocalNode.Role))
		if status.LocalNode.State != "" {
			localNode.WriteString(fmt.Sprintf("State: %s\n", status.LocalNode.State))
		}
		if status.LocalNode.LagSeconds > 0 {
			lagStyle := clusterHealthyStyle
			if status.LocalNode.LagSeconds > 60 {
				lagStyle = clusterUnhealthyStyle
			} else if status.LocalNode.LagSeconds > 10 {
				lagStyle = clusterWarningStyle
			}
			localNode.WriteString(fmt.Sprintf("Lag:   %s\n", lagStyle.Render(fmt.Sprintf("%.2fs", status.LocalNode.LagSeconds))))
		}
	} else {
		localNode.WriteString(mutedStyle.Render("No local node info available"))
	}

	localNodeBox := clusterBoxStyle.Width(rightWidth).Render(localNode.String())

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, overviewBox, "  ", localNodeBox))

	if status.ErrorMessage != "" {
		b.WriteString("\n\n")
		b.WriteString(clusterWarningStyle.Render("Warning: " + status.ErrorMessage))
	}

	return b.String()
}

func (v *ClusterView) renderNodes() string {
	if v.clusterStatus == nil {
		return helpStyle.Render("Press 'r' to refresh")
	}

	status := v.clusterStatus
	var b strings.Builder

	if status.Type == db.ClusterTypeNone {
		b.WriteString(mutedStyle.Render("Not running in cluster/replication mode."))
		return b.String()
	}

	b.WriteString(clusterTitleStyle.Render(fmt.Sprintf("Cluster Nodes (%s)", formatClusterType(status.Type))))
	b.WriteString("\n\n")

	if len(status.Nodes) == 0 {
		if status.LocalNode != nil {
			b.WriteString("Local node only (no replicas connected)\n\n")
			b.WriteString(fmt.Sprintf("  Role:  %s\n", status.LocalNode.Role))
			if status.LocalNode.State != "" {
				b.WriteString(fmt.Sprintf("  State: %s\n", status.LocalNode.State))
			}
		} else {
			b.WriteString(mutedStyle.Render("No node information available"))
		}
		return b.String()
	}

	// Header
	header := fmt.Sprintf("%-20s %-12s %-10s %-10s %-10s", "ADDRESS", "ROLE", "STATE", "SYNC", "LAG")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", 70))
	b.WriteString("\n")

	// Nodes
	for _, node := range status.Nodes {
		lag := "-"
		if node.LagSeconds > 0 {
			lag = fmt.Sprintf("%.1fs", node.LagSeconds)
		} else if node.LagBytes > 0 {
			lag = fmt.Sprintf("%d B", node.LagBytes)
		}

		sync := node.SyncState
		if sync == "" {
			sync = "-"
		}

		state := node.State
		if state == "" {
			state = "-"
		}

		address := node.Address
		if len(address) > 20 {
			address = address[:17] + "..."
		}

		row := fmt.Sprintf("%-20s %-12s %-10s %-10s %-10s",
			address, node.Role, state, sync, lag)
		b.WriteString(clusterNodeStyle.Render(row))
		b.WriteString("\n")
	}

	return b.String()
}

func (v *ClusterView) renderGalera() string {
	if v.conn.Config.Type != db.DatabaseTypeMariaDB {
		return mutedStyle.Render("Galera is only available for MariaDB")
	}

	if v.galeraStatus == nil {
		return helpStyle.Render("Press 'r' to refresh")
	}

	status := v.galeraStatus
	var b strings.Builder

	leftWidth := (v.width - 6) / 2
	rightWidth := leftWidth

	// Cluster Info Box
	var cluster strings.Builder
	cluster.WriteString(clusterTitleStyle.Render("Galera Cluster"))
	cluster.WriteString("\n\n")
	cluster.WriteString(fmt.Sprintf("Status:      %s\n", status.ClusterStatus))
	cluster.WriteString(fmt.Sprintf("Size:        %d nodes\n", status.ClusterSize))
	cluster.WriteString(fmt.Sprintf("Cluster ID:  %s\n", truncateUUID(status.ClusterStateUUID)))

	clusterBox := clusterBoxStyle.Width(leftWidth).Render(cluster.String())

	// Local Node Box
	var local strings.Builder
	local.WriteString(clusterTitleStyle.Render("Local Node"))
	local.WriteString("\n\n")
	local.WriteString(fmt.Sprintf("State:     %s\n", status.LocalState))
	local.WriteString(fmt.Sprintf("Index:     %d\n", status.LocalIndex))
	local.WriteString("Ready:     ")
	if status.Ready {
		local.WriteString(clusterHealthyStyle.Render("Yes"))
	} else {
		local.WriteString(clusterUnhealthyStyle.Render("No"))
	}
	local.WriteString("\nConnected: ")
	if status.Connected {
		local.WriteString(clusterHealthyStyle.Render("Yes"))
	} else {
		local.WriteString(clusterUnhealthyStyle.Render("No"))
	}

	localBox := clusterBoxStyle.Width(rightWidth).Render(local.String())

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, clusterBox, "  ", localBox))

	if status.FlowControl {
		b.WriteString("\n\n")
		b.WriteString(clusterWarningStyle.Render("WARNING: Flow control is active!"))
	}

	return b.String()
}

func (v *ClusterView) renderReplication() string {
	var b strings.Builder

	if v.conn.Config.Type == db.DatabaseTypeMariaDB {
		return v.renderMariaDBReplication()
	}

	// PostgreSQL - use cluster status for replication info
	if v.clusterStatus == nil {
		return helpStyle.Render("Press 'r' to refresh")
	}

	status := v.clusterStatus

	b.WriteString(clusterTitleStyle.Render("PostgreSQL Streaming Replication"))
	b.WriteString("\n\n")

	// Role
	b.WriteString("Role: ")
	if status.IsPrimary {
		b.WriteString(clusterHealthyStyle.Render("Primary"))
		b.WriteString(fmt.Sprintf("\nReplicas: %d", len(status.Nodes)))
	} else {
		b.WriteString(clusterWarningStyle.Render("Standby"))
	}
	b.WriteString("\n\n")

	if len(status.Nodes) > 0 {
		// Header
		header := fmt.Sprintf("%-18s %-10s %-8s %-12s %-12s %-12s %-12s", "ADDRESS", "STATE", "SYNC", "SENT", "WRITE", "FLUSH", "REPLAY")
		b.WriteString(headerStyle.Render(header))
		b.WriteString("\n")
		b.WriteString(strings.Repeat("─", 90))
		b.WriteString("\n")

		for _, node := range status.Nodes {
			address := node.Address
			if len(address) > 18 {
				address = address[:15] + "..."
			}

			sentLSN := truncateLSN(node.SentLSN)
			writeLSN := truncateLSN(node.WriteLSN)
			flushLSN := truncateLSN(node.FlushLSN)
			replayLSN := truncateLSN(node.ReplayLSN)

			row := fmt.Sprintf("%-18s %-10s %-8s %-12s %-12s %-12s %-12s",
				address, node.State, node.SyncState, sentLSN, writeLSN, flushLSN, replayLSN)
			b.WriteString(clusterNodeStyle.Render(row))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (v *ClusterView) renderMariaDBReplication() string {
	if v.replStatus == nil {
		return helpStyle.Render("Press 'r' to refresh")
	}

	status := v.replStatus
	var b strings.Builder

	b.WriteString(clusterTitleStyle.Render("MariaDB Replication"))
	b.WriteString("\n\n")

	leftWidth := (v.width - 6) / 2
	rightWidth := leftWidth

	if status.IsMaster {
		// Master info
		var master strings.Builder
		master.WriteString(clusterTitleStyle.Render("Master Status"))
		master.WriteString("\n\n")
		master.WriteString("Role: ")
		master.WriteString(clusterHealthyStyle.Render("Master"))
		master.WriteString("\n")
		if status.Position != "" {
			master.WriteString(fmt.Sprintf("Position: %s\n", status.Position))
		}
		if status.GTIDMode {
			master.WriteString("GTID Mode: Enabled\n")
		}

		b.WriteString(clusterBoxStyle.Width(leftWidth).Render(master.String()))
	}

	if status.IsReplica {
		// Replica info
		var replica strings.Builder
		replica.WriteString(clusterTitleStyle.Render("Replica Status"))
		replica.WriteString("\n\n")
		replica.WriteString("Role: ")
		replica.WriteString(clusterWarningStyle.Render("Replica"))
		replica.WriteString("\n")
		replica.WriteString(fmt.Sprintf("Master Host: %s:%d\n", status.MasterHost, status.MasterPort))
		replica.WriteString("\n")

		// IO Thread
		replica.WriteString("IO Running:  ")
		if status.ReplicaIORunning {
			replica.WriteString(clusterHealthyStyle.Render("Yes"))
		} else {
			replica.WriteString(clusterUnhealthyStyle.Render("No"))
		}
		replica.WriteString("\n")

		// SQL Thread
		replica.WriteString("SQL Running: ")
		if status.ReplicaSQLRunning {
			replica.WriteString(clusterHealthyStyle.Render("Yes"))
		} else {
			replica.WriteString(clusterUnhealthyStyle.Render("No"))
		}
		replica.WriteString("\n")

		// Lag
		if status.SecondsBehind != nil {
			lag := *status.SecondsBehind
			lagStyle := clusterHealthyStyle
			if lag > 60 {
				lagStyle = clusterUnhealthyStyle
			} else if lag > 10 {
				lagStyle = clusterWarningStyle
			}
			replica.WriteString(fmt.Sprintf("\nLag: %s\n", lagStyle.Render(fmt.Sprintf("%d seconds", lag))))
		}

		replicaBox := clusterBoxStyle.Width(rightWidth).Render(replica.String())

		if status.IsMaster {
			// Both master and replica (chained replication)
			b.WriteString("  ")
		}
		b.WriteString(replicaBox)

		if status.LastError != "" {
			b.WriteString("\n\n")
			b.WriteString(clusterUnhealthyStyle.Render("Last Error: " + status.LastError))
		}
	}

	if !status.IsMaster && !status.IsReplica {
		b.WriteString(mutedStyle.Render("No replication configured on this server."))
	}

	return b.String()
}

// Helper functions

func formatClusterType(t db.ClusterType) string {
	switch t {
	case db.ClusterTypeMariaDBGalera:
		return "MariaDB Galera Cluster"
	case db.ClusterTypeMariaDBReplica:
		return "MariaDB Master/Slave Replication"
	case db.ClusterTypePostgresStream:
		return "PostgreSQL Streaming Replication"
	case db.ClusterTypePostgresLogical:
		return "PostgreSQL Logical Replication"
	default:
		return string(t)
	}
}

func formatRole(isPrimary bool) string {
	if isPrimary {
		return "Primary/Master"
	}
	return "Replica/Standby"
}

func truncateUUID(uuid string) string {
	if len(uuid) > 20 {
		return uuid[:8] + "..."
	}
	return uuid
}

func truncateLSN(lsn string) string {
	if len(lsn) > 12 {
		return lsn[:12]
	}
	return lsn
}
