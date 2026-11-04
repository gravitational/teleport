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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/itertools/stream"
	toolcommon "github.com/gravitational/teleport/tool/common"
)

type phaseLSCommand struct {
	*kingpin.CmdClause
	all    bool
	failed bool
	format string
}

func newPhaseLSCommand(parent *kingpin.CmdClause) *phaseLSCommand {
	cmd := &phaseLSCommand{CmdClause: parent.Command("ls", "List Phase images.").Alias("list")}
	registerPhaseLSFlags(cmd)
	return cmd
}

func newPhaseFailedLSCommand(parent *kingpin.CmdClause) *phaseLSCommand {
	cmd := &phaseLSCommand{
		CmdClause: parent.Command("ls-failed", "List failed Phase build snapshots."),
		failed:    true,
	}
	registerPhaseLSFlags(cmd)
	return cmd
}

func registerPhaseLSFlags(cmd *phaseLSCommand) {
	cmd.Flag("all", "List all authorized Phases instead of only your own.").BoolVar(&cmd.all)
	cmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).Short('f').Default(teleport.Text).EnumVar(&cmd.format, defaults.DefaultFormats...)
}

func (c *phaseLSCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	var phases []*beamsv1.Phase
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()
		rootClient, err := clusterClient.ConnectToRootCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer rootClient.Close()
		var users []string
		if !c.all {
			user, err := rootClient.GetCurrentUser(cf.Context)
			if err != nil {
				return trace.Wrap(err)
			}
			users = []string{user.GetName()}
			owners := user.GetTraits()[types.PhaseOwnerLabel]
			if len(owners) == 1 && owners[0] != "" {
				users = owners
			}
		}
		phases, err = stream.Collect(clientutils.Resources(cf.Context, func(ctx context.Context, pageSize int, pageToken string) ([]*beamsv1.Phase, string, error) {
			resp, err := rootClient.PhaseServiceClient().ListPhases(ctx, beamsv1.ListPhasesRequest_builder{
				PageSize:  int32(pageSize),
				PageToken: pageToken,
				Users:     users,
			}.Build())
			if err != nil {
				return nil, "", trace.Wrap(err)
			}
			return resp.GetPhases(), resp.GetNextPageToken(), nil
		}))
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return c.print(cf.Stdout(), phases)
}

func (c *phaseLSCommand) print(w io.Writer, phases []*beamsv1.Phase) error {
	return c.printInLocation(w, phases, time.Local)
}

func (c *phaseLSCommand) printInLocation(w io.Writer, phases []*beamsv1.Phase, loc *time.Location) error {
	if c.failed {
		failures := make([]formattedPhaseFailure, 0)
		for _, phase := range phases {
			for _, failed := range formatPhase(phase).FailedBuilds {
				failures = append(failures, formattedPhaseFailure{
					Phase:                phase.GetMetadata().GetName(),
					formattedFailedBuild: failed,
				})
			}
		}
		switch strings.ToLower(c.format) {
		case teleport.JSON:
			return trace.Wrap(toolcommon.PrintJSONIndent(w, failures))
		case teleport.YAML:
			return trace.Wrap(toolcommon.PrintYAML(w, failures))
		default:
			return printPhaseFailureTable(w, failures, loc)
		}
	}
	formatted := make([]formattedPhase, len(phases))
	for i, phase := range phases {
		formatted[i] = formatPhase(phase)
		formatted[i].FailedBuilds = nil
	}
	switch strings.ToLower(c.format) {
	case teleport.JSON:
		return trace.Wrap(toolcommon.PrintJSONIndent(w, formatted))
	case teleport.YAML:
		return trace.Wrap(toolcommon.PrintYAML(w, formatted))
	default:
		return trace.Wrap(printPhaseTableInLocation(w, formatted, c.all, loc))
	}
}

func printPhaseTableInLocation(w io.Writer, phases []formattedPhase, all bool, loc *time.Location) error {
	table := asciitable.MakeTable([]string{"Name", "Upstream", "Strategy", "Status", "Region"})
	table.AddColumn(asciitable.Column{
		Title:         "Roles",
		MaxCellLength: 30,
		FootnoteLabel: "[+]",
	})
	table.AddFootnote("[+]",
		"Columns are truncated, use '--format=json' or '--format=yaml' to view the full list")
	table.AddColumn(asciitable.Column{Title: "Digest"})
	table.AddColumn(asciitable.Column{Title: "Last Published"})
	if all {
		table.AddColumn(asciitable.Column{Title: "Owner"})
	}
	for _, phase := range phases {
		publishedAt := "-"
		if phase.PublishedAt != nil {
			publishedAt = phase.PublishedAt.In(loc).Format("2006-01-02 15:04")
		}
		roles := "-"
		if len(phase.Roles) != 0 {
			roles = strings.Join(phase.Roles, ", ")
		}
		row := []string{
			phase.Name,
			phase.Parent,
			phase.Strategy,
			phase.State,
			phase.Region,
			roles,
			shortPhaseDigest(phase.ImageDigest),
			publishedAt,
		}
		if all {
			row = append(row, phase.Owner)
		}
		table.AddRow(row)
	}
	return trace.Wrap(table.WriteTo(w))
}

type formattedPhaseFailure struct {
	Phase string `json:"phase"`
	formattedFailedBuild
}

func printPhaseFailureTable(w io.Writer, failures []formattedPhaseFailure, loc *time.Location) error {
	table := asciitable.MakeTable([]string{"Phase", "Failed Snapshot", "Observed", "Estimated Expiration"})
	for _, failed := range failures {
		table.AddRow([]string{
			failed.Phase,
			shortPhaseDigest(failed.Digest),
			failed.Timestamp.In(loc).Format("2006-01-02 15:04"),
			failed.Expiration.In(loc).Format("2006-01-02 15:04"),
		})
	}
	return trace.Wrap(table.WriteTo(w))
}

// shortPhaseDigest shortens table values. Structured output retains the full digest.
func shortPhaseDigest(digest string) string {
	const prefix = "sha256:"
	if strings.HasPrefix(digest, prefix) && len(digest) > len(prefix)+10 {
		return digest[:len(prefix)+10]
	}
	return digest
}
