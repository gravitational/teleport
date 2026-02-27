// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package debuglive

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Quit     key.Binding
	Tab      key.Binding
	ShiftTab key.Binding
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	Enter    key.Binding
	Filter   key.Binding
	Tab1     key.Binding
	Tab2     key.Binding
	Tab3     key.Binding
	LogLevel key.Binding
	Pause    key.Binding
	Space    key.Binding
}

func newKeyMap() *keyMap {
	return &keyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch pane"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "switch pane"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("←", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("→", "right"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		Tab1: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "logs"),
		),
		Tab2: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "metrics"),
		),
		Tab3: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "profile"),
		),
		LogLevel: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "cycle log level"),
		),
		Pause: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "pause/resume"),
		),
		Space: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.Up, k.Down, k.Tab1, k.Tab2, k.Tab3, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Tab, k.ShiftTab},
		{k.Up, k.Down, k.Enter},
		{k.Tab1, k.Tab2, k.Tab3},
		{k.Quit},
	}
}
