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

import "github.com/charmbracelet/lipgloss"

// Shared styles for all views
var (
	primaryColor   = lipgloss.Color("#FF69B4") // Hot pink
	secondaryColor = lipgloss.Color("#FF1493") // Deep pink
	accentColor    = lipgloss.Color("#FFB6C1") // Light pink
	mutedColor     = lipgloss.Color("#888888")
	errorColor     = lipgloss.Color("#FF4444")
	successColor   = lipgloss.Color("#44FF44")

	titleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Italic(true)

	focusedStyle = lipgloss.NewStyle().
			Foreground(primaryColor)

	blurredStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	mutedStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor)

	successStyle = lipgloss.NewStyle().
			Foreground(successColor)

	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	bannerStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	headerStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true)
)
