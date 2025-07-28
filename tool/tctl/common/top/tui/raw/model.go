// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package raw

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gravitational/teleport/tool/tctl/common/top/tui/common"
)

// Model of the raw metrics component
type Model struct {
	list list.Model
}

func New() Model {
	d := list.NewDefaultDelegate()
	d.Styles.NormalDesc = lipgloss.NewStyle().Faint(true)
	d.Styles.NormalTitle = lipgloss.NewStyle().Faint(true)
	d.Styles.SelectedTitle = lipgloss.NewStyle().Faint(false).Foreground(lipgloss.Color("4"))
	d.Styles.SelectedDesc = lipgloss.NewStyle().Faint(false)

	l := list.New([]list.Item{}, d, 0, 0)
	l.SetShowTitle(false)
	l.SetShowFilter(true)
	l.SetShowStatusBar(true)
	l.SetShowHelp(false)

	return Model{
		list: l,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
	case common.MetricsMsg:
		cmd = m.list.SetItems(convertMetricsToItems(msg))
		cmds = append(cmds, cmd)
	default:
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)

}

func (m Model) View() string {
	return m.list.View()
}

func (m Model) Focused() bool {
	return m.list.FilterState() == list.Filtering
}
