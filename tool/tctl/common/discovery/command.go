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
	"log/slog"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// Command implements the "tctl discovery" CLI command group.
type Command struct {
	config *servicecfg.Config
	clock  clockwork.Clock
	stdout io.Writer

	inventoryCmd *kingpin.CmdClause

	inventoryLast   string
	inventoryFormat string
}

// output returns the configured stdout writer, defaulting to os.Stdout.
func (c *Command) output() io.Writer {
	if c.stdout != nil {
		return c.stdout
	}
	return os.Stdout
}

// Initialize registers the "discovery" command and its subcommands with the CLI parser.
func (c *Command) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.config = config
	if c.clock == nil {
		c.clock = clockwork.NewRealClock()
	}

	discovery := app.Command("discovery", "Troubleshoot Discovery auto-enrollment issues.")
	c.inventoryCmd = discovery.Command("inventory", "Report discovered instances and their SSM enrollment status.")

	c.inventoryCmd.Alias(`
Examples:

  List discovered instances in the last hour (default):
  $ tctl discovery inventory

  Look back 24 hours and output as JSON:
  $ tctl discovery inventory --last=24h --format=json

  Look back 30 minutes:
  $ tctl discovery inventory --last=30m
`)

	c.inventoryCmd.Flag("last", "Time window to look back for failures (e.g. 1h, 24h, 30m).").
		Default("1h").
		StringVar(&c.inventoryLast)
	c.inventoryCmd.Flag("format", "Output format.").
		Default(teleport.Text).
		EnumVar(&c.inventoryFormat, teleport.Text, teleport.JSON)
}

// TryRun attempts to run the matched subcommand.
func (c *Command) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	if cmd != c.inventoryCmd.FullCommand() {
		return false, nil
	}

	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return true, trace.Wrap(err)
	}
	defer closeFn(ctx)

	return true, trace.Wrap(c.runInventory(ctx, client))
}

// runInventory fetches all discovered instances and renders the output.
func (c *Command) runInventory(ctx context.Context, clt discoveryClient) error {
	from, to, err := resolveTimeRange(c.clock, c.inventoryLast)
	if err != nil {
		return trace.Wrap(err)
	}
	slog.DebugContext(ctx, "Resolved time range for inventory",
		"from", from,
		"to", to,
		"last", c.inventoryLast,
	)

	instances, err := buildInventory(ctx, clt, from, to)
	if err != nil {
		return trace.Wrap(err)
	}
	slog.DebugContext(ctx, "Built instance inventory",
		"total_instances", len(instances),
		"format", c.inventoryFormat,
	)

	w := c.output()
	switch c.inventoryFormat {
	case teleport.JSON:
		return trace.Wrap(renderJSON(w, instances))
	default:
		return trace.Wrap(renderText(w, instances))
	}
}
