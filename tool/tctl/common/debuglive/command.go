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

import (
	"context"

	"github.com/alecthomas/kingpin/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
)

// Command implements the `tctl debug live` TUI command.
type Command struct {
	config *servicecfg.Config
	live   *kingpin.CmdClause
}

// Initialize registers the command with the kingpin CLI parser.
// It expects the parent "debug" command to be passed so that
// `live` becomes a subcommand of `debug`.
func (c *Command) Initialize(debug *kingpin.CmdClause, config *servicecfg.Config) {
	c.config = config
	c.live = debug.Command("live", "Interactive TUI for browsing and debugging cluster instances.")
}

// FullCommand returns the full command string for matching in TryRun.
func (c *Command) FullCommand() string {
	return c.live.FullCommand()
}

// Run starts the TUI.
func (c *Command) Run(ctx context.Context, clientFunc commonclient.InitFunc) error {
	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer closeFn(ctx)

	m := newModel(client)

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithContext(ctx),
	)

	// Store the program reference so the model can use p.Send from goroutines.
	go p.Send(programReadyMsg{p: p})

	_, err = p.Run()
	return trace.Wrap(err)
}
