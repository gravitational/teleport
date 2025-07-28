package keymap

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Quit  key.Binding
	Right key.Binding
	Left  key.Binding

	// Tabs
	Common  key.Binding
	Backend key.Binding
	Cache   key.Binding
	Watcher key.Binding
	Audit   key.Binding
}

var (
	Keymap = keyMap{
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
		Common: key.NewBinding(
			key.WithKeys("1"), key.WithHelp("1", "common"),
		),
		Backend: key.NewBinding(
			key.WithKeys("2"), key.WithHelp("2", "backend"),
		),
		Cache: key.NewBinding(
			key.WithKeys("3"), key.WithHelp("3", "cache"),
		),
		Watcher: key.NewBinding(
			key.WithKeys("4"), key.WithHelp("4", "watcher"),
		),
		Audit: key.NewBinding(
			key.WithKeys("5"), key.WithHelp("5", "audit"),
		),
	}
)

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Left, k.Right, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Left, k.Right},
		{k.Quit},
	}
}
