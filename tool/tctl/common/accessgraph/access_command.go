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

package accessgraph

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/teleport"
	accessgraph "github.com/gravitational/teleport/lib/accessgraph/apiclient"
	models "github.com/gravitational/teleport/lib/accessgraph/apiclient/models/graph"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/trace"
)

type accessArgs struct {
	cmd    *kingpin.CmdClause
	query  accessQueryArgs
	review accessReviewArgs

	// Output format
	format string
}

type accessQueryArgs struct {
	cmd   *kingpin.CmdClause
	query string
}

type accessReviewArgs struct {
	cmd      *kingpin.CmdClause
	resource accessReviewResourceArgs
	acl      accessReviewACLArgs
	role     accessReviewRoleArgs
	identity accessReviewIdentityArgs

	// Shared filters across all review subcommands.
	from     time.Time
	to       time.Time
	unused   bool
	detailed bool
	users    []string
}

func (c *AccessGraphCommand) initAccess(app *kingpin.Application) {
	accessCmd := app.Command("access", "Analyze who has access to what.").Hidden()
	accessCmd.Flag("format", "Output format. (Values: text, json, yaml)").
		Default(teleport.YAML).
		EnumVar(&c.access.format, teleport.Text, teleport.JSON, teleport.YAML)
	c.access.cmd = accessCmd

	c.initAccessQuery(c.access.cmd)
	c.initAccessReview(c.access.cmd)
}

func (c *AccessGraphCommand) initAccessQuery(parent *kingpin.CmdClause) {
	c.access.query.cmd = parent.Command("query", "Run a query against Access Graph.")
	c.access.query.cmd.Arg("query", "The query to execute.").Required().StringVar(&c.access.query.query)
}

// AccessQuery executes `tctl access query`, running a custom query against the
// Access Graph and returning every matched node. Resource nodes are listed
// first so that the most actionable results are visible at the top.
func (c *AccessGraphCommand) AccessQuery(ctx context.Context, client *accessgraph.ClientWithResponses) error {
	resp, err := doRequest(client.ExecuteQueryV1WithResponse(ctx, &accessgraph.ExecuteQueryV1Params{
		Query: c.access.query.query,
	}))
	if err != nil {
		return trace.Wrap(err)
	}

	var resources, other []*models.Node
	if resp.JSON200 != nil && resp.JSON200.Nodes != nil {
		for i := range *resp.JSON200.Nodes {
			n := &(*resp.JSON200.Nodes)[i]
			if n.Kind == "resource" {
				resources = append(resources, n)
			} else {
				other = append(other, n)
			}
		}
	}
	nodes := append(resources, other...)
	return writeOutput(c.stdout, nodes, c.access.format, func(w io.Writer) error {
		return displayNodesText(w, nodes)
	})
}

// displayNodesText renders a mixed list of nodes as a single table with Kind
// and Alias columns so resources and non-resources can be shown together.
func displayNodesText(out io.Writer, nodes []*models.Node) error {
	if len(nodes) == 0 {
		_, err := fmt.Fprintln(out, "No nodes found.")
		return trace.Wrap(err)
	}

	table := asciitable.MakeTable([]string{
		"Node Id",
		"Name",
		"Alias",
		"Kind",
		"Subkind",
		"Source",
		"Origin",
		"Origin Type",
	})

	for _, n := range nodes {
		alias, source, origin, originType := extractCommonProps(n)
		table.AddRow([]string{
			n.Id.String(),
			n.Name,
			alias,
			string(n.Kind),
			n.SubKind,
			source,
			origin,
			originType,
		})
	}
	_, err := fmt.Fprintln(out, table.AsBuffer().String())
	return trace.Wrap(err)
}

// extractCommonProps pulls out the common display fields (alias, source,
// origin, origin type) from either resource or identity properties, returning
// empty strings for fields that don't apply to the node's kind.
func extractCommonProps(n *models.Node) (alias, source, origin, originType string) {
	if props, err := n.Properties.AsResourceProperties(); err == nil {
		return strPtrToStr(props.Alias), strPtrToStr(props.Source), strPtrToStr(props.Origin), props.OriginType
	}
	if props, err := n.Properties.AsIdentityProperties(); err == nil {
		return "", strPtrToStr(props.Source), strPtrToStr(props.Origin), props.OriginType
	}
	return "", "", "", ""
}
