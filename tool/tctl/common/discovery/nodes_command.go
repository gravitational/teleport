package discovery

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

type nodesArgs struct {
	cmd          *kingpin.CmdClause
	last         time.Duration
	format       string
	failuresOnly bool
	cloudFilter  string
}

func (c *nodesArgs) initNodes(app *kingpin.CmdClause, stdout io.Writer) {
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
		DurationVar(&c.last)
	nodesCmd.Flag("format", "Output format.").
		Default(teleport.Text).
		EnumVar(&c.format, teleport.Text, teleport.JSON, teleport.YAML)
	nodesCmd.Flag("failures-only", "Only show instances with enrollment failures.").
		BoolVar(&c.failuresOnly)
	nodesCmd.Flag("cloud", "Comma-separated list of cloud providers to include (allowed: aws, azure). Empty (default) returns all.").
		Default("").
		StringVar(&c.cloudFilter)

	c.cmd = nodesCmd
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
