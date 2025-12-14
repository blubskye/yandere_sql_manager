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

package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors - a pink/magenta theme for the yandere aesthetic
	primaryColor   = lipgloss.Color("#FF69B4") // Hot pink
	secondaryColor = lipgloss.Color("#FF1493") // Deep pink
	accentColor    = lipgloss.Color("#FFB6C1") // Light pink
	bgColor        = lipgloss.Color("#1a1a2e") // Dark blue-ish background
	textColor      = lipgloss.Color("#FFFFFF")
	mutedColor     = lipgloss.Color("#888888")
	errorColor     = lipgloss.Color("#FF4444")
	successColor   = lipgloss.Color("#44FF44")

	// Base styles
	baseStyle = lipgloss.NewStyle().
			Foreground(textColor)

	// Title style
	titleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			Padding(0, 1)

	// Subtitle style
	subtitleStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Italic(true)

	// Selected item style
	selectedStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Background(primaryColor).
			Bold(true).
			Padding(0, 1)

	// Normal item style
	itemStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Padding(0, 1)

	// Muted text style
	mutedStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Error style
	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	// Success style
	successStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	// Box style for panels
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2)

	// Active box style
	activeBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(secondaryColor).
			Padding(1, 2)

	// Help style
	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Padding(1, 0)

	// Status bar style
	statusBarStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Background(primaryColor).
			Padding(0, 1)

	// Input style
	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(primaryColor).
			Padding(0, 1)

	// Focused input style
	focusedInputStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(secondaryColor).
				Padding(0, 1)

	// Table header style
	tableHeaderStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true).
				Padding(0, 1)

	// Table cell style
	tableCellStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Padding(0, 1)

	// Logo/banner
	bannerStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)
)

// Logo returns the YSM logo
func Logo() string {
	return bannerStyle.Render(`
  ██╗   ██╗███████╗███╗   ███╗
  ╚██╗ ██╔╝██╔════╝████╗ ████║
   ╚████╔╝ ███████╗██╔████╔██║
    ╚██╔╝  ╚════██║██║╚██╔╝██║
     ██║   ███████║██║ ╚═╝ ██║
     ╚═╝   ╚══════╝╚═╝     ╚═╝
`) + subtitleStyle.Render("  Yandere SQL Manager")
}
