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
	"github.com/gravitational/teleport/tool/tctl/common/top/tui/common"
)

// Model of the raw metrics component
type Model struct {
	list list.Model
}

func New() Model {
	l := list.New([]list.Item{}, itemDelegate{}, 120, 30)
	l.SetShowTitle(true)
	l.SetShowFilter(true)
	l.SetShowStatusBar(true)
	l.SetShowHelp(true)

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
	case common.MetricsMsg:
		cmd = m.list.SetItems(convertMetricsToListItems(msg))
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
