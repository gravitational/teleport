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
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// Command implements the "tctl discovery" CLI command group.
type Command struct {
	nodesCmd *kingpin.CmdClause

	nodesLast         time.Duration
	nodesFormat       string
	nodesFailuresOnly bool
	nodesCloudFilter  string
}

// Initialize registers the "discovery" command and its subcommands with the CLI parser.
func (c *Command) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, _ *servicecfg.Config) {
	discovery := app.Command("discovery", "Troubleshoot auto-discovery issues.")
	c.nodesCmd = discovery.Command("nodes", "Report discovered server instances and their enrollment status using Teleport audit log and cluster state.")
	c.nodesCmd.Alias(`
Examples:

  List discovered instances in the last hour (default):
  $ tctl discovery nodes

  Look back 24 hours and output as JSON:
  $ tctl discovery nodes --last=24h --format=json

  Look back 30 minutes:
  $ tctl discovery nodes --last=30m
`)

	c.nodesCmd.Flag("last", "Time window to look back for failures in Teleport audit log (e.g. 1h, 24h, 30m).").
		Default("1h").
		DurationVar(&c.nodesLast)
	c.nodesCmd.Flag("format", "Output format.").
		Default(teleport.Text).
		EnumVar(&c.nodesFormat, teleport.Text, teleport.JSON)
	c.nodesCmd.Flag("failures-only", "Only show instances with enrollment failures.").
		BoolVar(&c.nodesFailuresOnly)
	c.nodesCmd.Flag("cloud", "Comma-separated list of cloud providers to include (allowed: aws, azure). Empty (default) returns all.").
		Default("").
		StringVar(&c.nodesCloudFilter)
}

// TryRun attempts to run the matched subcommand.
func (c *Command) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	if cmd != c.nodesCmd.FullCommand() {
		return false, nil
	}

	dateTo := time.Now().UTC()
	dateFrom := dateTo.Add(-c.nodesLast)

	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return true, trace.Wrap(err)
	}
	defer closeFn(ctx)

	return true, trace.Wrap(c.runNodes(ctx, client, os.Stdout, dateFrom, dateTo))
}

// runNodes fetches all discovered instances and renders the output.
func (c *Command) runNodes(ctx context.Context, clt discoveryClient, w io.Writer, dateFrom, dateTo time.Time) error {
	slog.DebugContext(ctx, "Resolved time range for nodes",
		"from", dateFrom,
		"to", dateTo,
		"last", c.nodesLast,
	)

	cfg, err := parseCloudProviders(c.nodesCloudFilter)
	if err != nil {
		return trace.Wrap(err)
	}

	instances, err := buildNodes(ctx, clt, dateFrom, dateTo, cfg)
	if err != nil {
		return trace.Wrap(err)
	}
	if c.nodesFailuresOnly {
		total := len(instances)
		instances = filterFailures(instances)
		slog.DebugContext(ctx, "Filtered to failures only",
			"total_instances", total,
			"failed_instances", len(instances),
		)
	}
	slog.DebugContext(ctx, "Built nodes report",
		"total_instances", len(instances),
		"format", c.nodesFormat,
	)

	switch c.nodesFormat {
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSONArray(w, instances))
	default:
		return trace.Wrap(renderText(w, instances))
	}
}
