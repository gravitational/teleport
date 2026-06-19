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
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

type nodesArgs struct {
	cmd          *kingpin.CmdClause
	last         time.Duration
	format       string
	failuresOnly bool
	cloudFilter  string
}

func (n *nodesArgs) initNodes(app *kingpin.CmdClause) {
	nodesCmd := app.Command("nodes", "Report discovered server instances and their enrollment status using Teleport audit log and cluster state.")
	nodesCmd.Alias(`
Examples:

  List discovered instances in the last hour (default):
  $ tctl discovery nodes

  Look back 24 hours and output as JSON:
  $ tctl discovery nodes --last=24h --format=json

  Look back 30 minutes:
  $ tctl discovery nodes --last=30m
`)

	nodesCmd.Flag("last", "Time window to look back for failures in Teleport audit log (e.g. 1h, 24h, 30m).").
		Default("1h").
		DurationVar(&n.last)
	nodesCmd.Flag("format", "Output format.").
		Default(teleport.Text).
		EnumVar(&n.format, teleport.Text, teleport.JSON, teleport.YAML)
	nodesCmd.Flag("failures-only", "Only show instances with enrollment failures.").
		BoolVar(&n.failuresOnly)
	nodesCmd.Flag("cloud", "Comma-separated list of cloud providers to include (allowed: aws, azure). Empty (default) returns all.").
		Default("").
		StringVar(&n.cloudFilter)

	n.cmd = nodesCmd
}

func (n *nodesArgs) run(ctx context.Context, clt discoveryClient, w io.Writer) error {
	dateTo := time.Now().UTC()
	dateFrom := dateTo.Add(-n.last)

	return n.runWithTimeRange(ctx, clt, w, dateFrom, dateTo)
}

func (n *nodesArgs) runWithTimeRange(ctx context.Context, clt discoveryClient, w io.Writer, dateFrom, dateTo time.Time) error {
	slog.DebugContext(ctx, "Resolved time range for nodes",
		"from", dateFrom,
		"to", dateTo,
		"last", n.last,
	)

	cfg, err := parseCloudProviders(n.cloudFilter)
	if err != nil {
		return trace.Wrap(err)
	}

	instances, err := buildNodes(ctx, clt, dateFrom, dateTo, cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	if n.failuresOnly {
		instances = filterFailures(instances)
	}

	switch n.format {
	case teleport.Text:
		return trace.Wrap(renderText(w, instances))
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSONArray(w, instances))
	case teleport.YAML:
		return trace.Wrap(utils.WriteYAML(w, instances))
	default:
		return trace.BadParameter("unknown format %q", n.format)
	}
}
