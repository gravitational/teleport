/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gravitational/trace"
)

// pagerKeyMap defines key bindings for the pager
type pagerKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	HalfUp   key.Binding
	HalfDown key.Binding
	Top      key.Binding
	Bottom   key.Binding
	Quit     key.Binding
	Help     key.Binding
}

// ShortHelp returns key bindings to be shown in the mini help view.
func (k pagerKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Help, k.Quit}
}

// FullHelp returns key bindings for the expanded help view.
func (k pagerKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown},
		{k.HalfUp, k.HalfDown, k.Top, k.Bottom},
		{k.Help, k.Quit},
	}
}

func newPagerKeyMap() pagerKeyMap {
	return pagerKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "b"),
			key.WithHelp("pgup/b", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "f", " "),
			key.WithHelp("pgdn/f/space", "page down"),
		),
		HalfUp: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "½ page up"),
		),
		HalfDown: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "½ page down"),
		),
		Top: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home/g", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end/G", "bottom"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q/esc", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
	}
}

// pagerModel is a bubbletea model for interactive paging
type pagerModel struct {
	viewport     viewport.Model
	help         help.Model
	keys         pagerKeyMap
	ready        bool
	showHelp     bool
	headerHeight int
	footerHeight int
	content      string // Store content until viewport is ready
}

func newPagerModel(content string) *pagerModel {
	return &pagerModel{
		help:         help.New(),
		keys:         newPagerKeyMap(),
		headerHeight: 0,
		footerHeight: 2, // Status line + help line
		content:      content,
	}
}

func (m *pagerModel) Init() tea.Cmd {
	return nil
}

func (m *pagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if !m.ready {
			// Initialize viewport when we know the terminal size
			m.viewport = viewport.New(msg.Width, msg.Height-m.headerHeight-m.footerHeight)
			m.viewport.YPosition = m.headerHeight

			// Enable text wrapping to fit within terminal width
			wrappedContent := wrapText(m.content, msg.Width)
			m.viewport.SetContent(wrappedContent)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - m.headerHeight - m.footerHeight

			// Re-wrap content when window is resized
			wrappedContent := wrapText(m.content, msg.Width)
			m.viewport.SetContent(wrappedContent)
		}

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.showHelp = !m.showHelp
		case key.Matches(msg, m.keys.Up):
			m.viewport.ScrollUp(1)
		case key.Matches(msg, m.keys.Down):
			m.viewport.ScrollDown(1)
		case key.Matches(msg, m.keys.PageUp):
			m.viewport.PageUp()
		case key.Matches(msg, m.keys.PageDown):
			m.viewport.PageDown()
		case key.Matches(msg, m.keys.HalfUp):
			m.viewport.HalfPageUp()
		case key.Matches(msg, m.keys.HalfDown):
			m.viewport.HalfPageDown()
		case key.Matches(msg, m.keys.Top):
			m.viewport.GotoTop()
		case key.Matches(msg, m.keys.Bottom):
			m.viewport.GotoBottom()
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *pagerModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	var sections []string

	// Main content
	sections = append(sections, m.viewport.View())

	// Footer with status and help
	footer := m.renderFooter()
	sections = append(sections, footer)

	return strings.Join(sections, "\n")
}

func (m *pagerModel) renderFooter() string {
	// Status line showing position
	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Background(lipgloss.Color("235"))

	percentage := 100
	if m.viewport.TotalLineCount() > 0 {
		percentage = int(float64(m.viewport.YOffset) / float64(m.viewport.TotalLineCount()-m.viewport.Height) * 100)
		if percentage < 0 {
			percentage = 0
		}
		if percentage > 100 {
			percentage = 100
		}
	}

	status := fmt.Sprintf(" %d%% (%d/%d) ", percentage, m.viewport.YOffset, m.viewport.TotalLineCount())
	statusLine := statusStyle.Render(status)

	// Help line
	var helpLine string
	if m.showHelp {
		helpLine = m.help.View(m.keys)
	} else {
		helpLine = m.help.ShortHelpView(m.keys.ShortHelp())
	}

	return fmt.Sprintf("%s\n%s", statusLine, helpLine)
}

// wrapText wraps long lines to fit within the specified width
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		if len(line) <= width {
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}

		// Wrap long lines
		remaining := line
		for len(remaining) > width {
			// Try to break at a space
			breakPoint := width
			if idx := strings.LastIndex(remaining[:width], " "); idx > 0 {
				breakPoint = idx
			}

			result.WriteString(remaining[:breakPoint])
			result.WriteString("\n")
			remaining = strings.TrimLeft(remaining[breakPoint:], " ")
		}

		if len(remaining) > 0 {
			result.WriteString(remaining)
			result.WriteString("\n")
		}
	}

	return result.String()
}

// runInteractivePager displays content in an interactive pager built with bubbletea
func runInteractivePager(content string, w io.Writer) error {
	model := newPagerModel(content)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithOutput(w),
	)

	if _, err := p.Run(); err != nil {
		return trace.Wrap(err, "running pager")
	}

	return nil
}
