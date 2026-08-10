/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/dustin/go-humanize"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/tool/common"
)

type beamsLSCommand struct {
	*kingpin.CmdClause
	all    bool
	format string

	// These helper functions can be overridden.
	fetchFn        func(context.Context, *client.TeleportClient, bool) ([]*beamsv1.Beam, error)
	proxyAddrFn    func(*CLIConf) (string, error)
	humanizeTimeFn func(time.Time) string
}

func newBeamsLSCommand(parent *kingpin.CmdClause) *beamsLSCommand {
	cmd := &beamsLSCommand{
		CmdClause: parent.Command("ls", "List beam instances.").Alias("list"),
	}
	cmd.fetchFn = cmd.fetch
	cmd.proxyAddrFn = cmd.proxyAddr
	cmd.humanizeTimeFn = humanize.Time

	cmd.Flag("all", "List all beams. By default, filters to show only beams belonging to the current user.").BoolVar(&cmd.all)
	cmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).
		Short('f').
		Default(teleport.Text).
		EnumVar(&cmd.format, defaults.DefaultFormats...)
	return cmd
}

func (c *beamsLSCommand) run(cf *CLIConf) error {
	ctx := cf.Context

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	beams, err := c.fetchFn(ctx, tc, c.all)
	if err != nil {
		return trace.Wrap(err)
	}

	proxyAddr, err := c.proxyAddrFn(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(c.print(cf, beams, proxyAddr))
}

func (c *beamsLSCommand) fetch(ctx context.Context, tc *client.TeleportClient, all bool) ([]*beamsv1.Beam, error) {
	var beams []*beamsv1.Beam
	err := client.RetryWithRelogin(ctx, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		rootClient, err := clusterClient.ConnectToRootCluster(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer rootClient.Close()

		filters := &beamsv1.ListBeamsRequest_Filters{}
		if !all {
			user, err := rootClient.GetCurrentUser(ctx)
			if err != nil {
				return trace.Wrap(err)
			}
			filters.Users = []string{user.GetName()}
		}

		beams, err = stream.Collect(
			clientutils.Resources(ctx, func(ctx context.Context, pageSize int, pageToken string) ([]*beamsv1.Beam, string, error) {
				rsp, err := rootClient.
					BeamServiceClient().
					ListBeams(ctx, &beamsv1.ListBeamsRequest{
						PageSize:  int32(pageSize),
						PageToken: pageToken,
						Filters:   filters,
					})
				if err != nil {
					return nil, "", trace.Wrap(err)
				}
				return rsp.GetBeams(), rsp.GetNextPageToken(), err
			}),
		)
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return beams, nil
}

func (c *beamsLSCommand) proxyAddr(cf *CLIConf) (string, error) {
	_, proxyAddr, err := fetchProxyVersion(cf)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return proxyAddr, nil
}

func (c *beamsLSCommand) print(cf *CLIConf, beams []*beamsv1.Beam, proxyAddr string) error {
	// Convert to script/agent friendly representation.
	formatted := make([]formattedBeam, len(beams))
	for idx, beam := range beams {
		formatted[idx] = formatBeam(beam, proxyAddr)
	}
	switch strings.ToLower(c.format) {
	case teleport.JSON:
		return trace.Wrap(common.PrintJSONIndent(cf.Stdout(), formatted))
	case teleport.YAML:
		return trace.Wrap(common.PrintYAML(cf.Stdout(), formatted))
	default:
		return trace.Wrap(c.printTable(cf.Stdout(), formatted))
	}
}

func (c *beamsLSCommand) printTable(w io.Writer, beams []formattedBeam) error {
	headings := []string{"ID", "URL", "Expires"}

	// We only show the owner column when the user passed the `--all` flag,
	// otherwise there's no point as we're just showing them their own beams.
	if c.all {
		headings = append(headings, "Owner")
	}

	table := asciitable.MakeTable(headings)
	for _, beam := range beams {
		row := []string{
			beam.ID,
			beam.URL,
			c.humanizeTimeFn(beam.Expires),
		}
		if c.all {
			row = append(row, beam.Owner)
		}
		table.AddRow(row)
	}
	return trace.Wrap(table.WriteTo(w))
}
