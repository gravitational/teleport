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

import (
	"context"
	"time"

	"github.com/alecthomas/kingpin/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// Command is a debug command that consumes the
// Teleport /metrics endpoint and displays diagnostic
// information an easy to consume way.
type Command struct {
	config        *servicecfg.Config
	top           *kingpin.CmdClause
	diagURL       string
	refreshPeriod time.Duration
}

// Initialize sets up the "tctl top" command.
func (c *Command) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.config = config
	c.top = app.Command("top", "Report diagnostic information.")
	c.top.Arg("diag-addr", "Diagnostic HTTP URL").Default("http://127.0.0.1:3000").StringVar(&c.diagURL)
	c.top.Arg("refresh", "Refresh period").Default("5s").DurationVar(&c.refreshPeriod)
}

// TryRun attempts to run subcommands.
func (c *Command) TryRun(ctx context.Context, cmd string, _ commonclient.InitFunc) (match bool, err error) {
	if cmd != c.top.FullCommand() {
		return false, nil
	}

	diagClient, err := roundtrip.NewClient(c.diagURL, "")
	if err != nil {
		return true, trace.Wrap(err)
	}

	p := tea.NewProgram(
		newTopModel(c.refreshPeriod, diagClient),
		tea.WithAltScreen(),
		tea.WithContext(ctx),
	)

	_, err = p.Run()
	return true, trace.Wrap(err)
}
