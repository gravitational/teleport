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

package discovery

import (
	"context"
	"io"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
	"github.com/gravitational/trace"
)

// Command implements the "tctl discovery" CLI command group.
type Command struct {
	stdout io.Writer

	nodes   nodesArgs
	summary summaryArgs
}

// Initialize registers the "discovery" command and its subcommands with the CLI parser.
func (c *Command) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, _ *servicecfg.Config) {
	if c.stdout == nil {
		c.stdout = os.Stdout
	}

	discovery := app.Command("discovery", "Troubleshoot auto-discovery issues.")
	c.nodes.initNodes(discovery, c.stdout)
}

// TryRun attempts to run the matched subcommand.
func (c *Command) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (bool, error) {
	var commandFunc func(context.Context, discoveryClient, io.Writer) error

	switch cmd {
	case c.nodes.cmd.FullCommand():
		commandFunc = c.nodes.run
	case c.summary.cmd.FullCommand():
		commandFunc = c.summary.run
	default:
		return false, nil
	}

	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return true, trace.Wrap(err)
	}
	defer closeFn(ctx)

	return true, trace.Wrap(commandFunc(ctx, client, c.stdout))
}
