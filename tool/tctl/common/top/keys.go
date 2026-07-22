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

package top

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Quit key.Binding

	// Movement
	Right  key.Binding
	Left   key.Binding
	Up     key.Binding
	Down   key.Binding
	Filter key.Binding

	// Tabs
	Common  key.Binding
	Backend key.Binding
	Cache   key.Binding
	Watcher key.Binding
	Audit   key.Binding
	Raw     key.Binding
}

// newDefaultKeymap returns default keybinds used by top
func newDefaultKeymap() *keyMap {
	return &keyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "esc", "shift+tab", "h"),
			key.WithHelp("←", "previous"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "tab", "l"),
			key.WithHelp("→", "next"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓", "down"),
		),
		Common: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "common"),
		),
		Backend: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "backend"),
		),
		Cache: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "cache"),
		),
		Watcher: key.NewBinding(
			key.WithKeys("4"),
			key.WithHelp("4", "watcher"),
		),
		Audit: key.NewBinding(
			key.WithKeys("5"),
			key.WithHelp("5", "audit"),
		),
		Raw: key.NewBinding(
			key.WithKeys("6"),
			key.WithHelp("6", "raw"),
		),

		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Left, k.Right, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Left, k.Right},
		{k.Quit},
	}
}
