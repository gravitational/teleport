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
	accessgraph "github.com/gravitational/access-graph/api/client"
	models "github.com/gravitational/access-graph/api/client/models/graph"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

type accessArgs struct {
	cmd    *kingpin.CmdClause
	whoCan accessWhoCanArgs
	query  accessQueryArgs
	review accessReviewArgs

	// Output format
	format string
}

type accessWhoCanArgs struct {
	cmd      *kingpin.CmdClause
	resource string
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

	// Shared filters across all review subcommands.
	from     time.Time
	to       time.Time
	unused   bool
	detailed bool
	users    []string
}

func (c *AccessGraphCommand) initAccess(app *kingpin.Application) {
	accessCmd := app.Command("access", "Analyze who has access to what.").Hidden()

	accessCmd.Flag("format", "Output format (text, json, yaml)").Default(teleport.YAML).EnumVar(&c.access.format, teleport.Text, teleport.JSON, teleport.YAML)

	c.access.cmd = accessCmd
	c.initAccessWhoCan(c.access.cmd)
	c.initAccessQuery(c.access.cmd)
	c.initAccessReview(c.access.cmd)
}

func (c *AccessGraphCommand) initAccessWhoCan(parent *kingpin.CmdClause) {
	c.access.whoCan.cmd = parent.Command("who-can", "Show which identities have access to a resource.")
	c.access.whoCan.cmd.Arg("resource", "The resource to inspect.").Required().StringVar(&c.access.whoCan.resource)
}

func (c *AccessGraphCommand) initAccessQuery(parent *kingpin.CmdClause) {
	c.access.query.cmd = parent.Command("query", "Run a query against Access Graph.")
	c.access.query.cmd.Arg("query", "The query to execute.").Required().StringVar(&c.access.query.query)
}

// AccessWhoCan executes `tctl access who-can`, which tells you who has standing access to a given resource
func (c *AccessGraphCommand) AccessWhoCan(ctx context.Context, args accessGraphServices) error {
	search := c.access.whoCan.resource
	resourceQuery := fmt.Sprintf("SELECT * from nodes WHERE (name ILIKE '%%%s%%' OR properties->>'alias' ILIKE '%%%s%%')", search, search)
	resourceRsp, err := doRequest(args.accessGraph.ExecuteQueryV1WithResponse(ctx, &accessgraph.ExecuteQueryV1Params{
		Query: resourceQuery,
	}))

	if err != nil {
		return trace.Wrap(err)
	}

	if resourceRsp.JSON200 == nil || resourceRsp.JSON200.Nodes == nil || len(*resourceRsp.JSON200.Nodes) == 0 {
		return trace.NotFound("resource %q not found in access graph", c.access.whoCan.resource)
	}

	if len(*resourceRsp.JSON200.Nodes) > 1 {
		fmt.Fprintln(c.stdout, "Multiple resources found matching query:")
		resources := make([]*models.Node, 0)
		for _, resource := range *resourceRsp.JSON200.Nodes {
			resources = append(resources, &resource)
		}
		err := displayResources(c.stdout, resources, c.access.format)
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.BadParameter("multiple resources found matching %q, please refine your query", c.access.whoCan.resource)
	}

	resource := (*resourceRsp.JSON200.Nodes)[0]
	query := fmt.Sprintf("SELECT * from access_path WHERE id = '%s'", resource.Id)

	resp, err := doRequest(args.accessGraph.ExecuteQueryV1WithResponse(ctx, &accessgraph.ExecuteQueryV1Params{
		Query: query,
	}))

	if err != nil {
		return trace.Wrap(err)
	}

	graph := newTraversalGraph(resp.JSON200.Nodes, resp.JSON200.Edges)
	identitiesWithAccess := graph.GetIdentityNodesWithAccess(
		resource,
	)
	return displayWhoCanResult(c.stdout, identitiesWithAccess, c.access.format)
}

// AccessQuery executes `tctl access query`, which allows you to run a custom query against the Access Graph and return
// the resource nodes
func (c *AccessGraphCommand) AccessQuery(ctx context.Context, args accessGraphServices) error {
	resp, err := doRequest(args.accessGraph.ExecuteQueryV1WithResponse(ctx, &accessgraph.ExecuteQueryV1Params{
		Query: c.access.query.query,
	}))

	if err != nil {
		return trace.Wrap(err)
	}

	resources := make([]*models.Node, 0)
	for _, node := range *resp.JSON200.Nodes {
		if node.Kind == "resource" {
			resources = append(resources, &node)
		}
	}
	return displayResources(c.stdout, resources, c.access.format)
}

func displayWhoCanResult(out io.Writer, reqs []*models.Node, format string) error {
	switch format {
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSONArray(out, reqs))
	case teleport.Text:
		return trace.Wrap(displayWhoCanResultText(out, reqs))
	default:
		return trace.Wrap(utils.WriteYAML(out, reqs))
	}
}

func displayWhoCanResultText(out io.Writer, identies []*models.Node) error {
	if len(identies) == 0 {
		_, err := fmt.Fprintln(out, "No identities have access to this resource.")
		return trace.Wrap(err)
	}

	table := asciitable.MakeTable([]string{
		"Identity ID",
		"Name",
		"Kind",
		"Subkind",
		"Source",
		"Origin",
		"Origin Type",
	})

	for _, identity := range identies {
		props, err := identity.Properties.AsIdentityProperties()
		if err != nil {
			return trace.Wrap(err)
		}

		table.AddRow([]string{
			identity.Id.String(),
			identity.Name,
			string(identity.Kind),
			identity.SubKind,
			*props.Source,
			*props.Origin,
			props.OriginType,
		})
	}
	_, err := fmt.Fprintln(out, table.AsBuffer().String())
	return trace.Wrap(err)
}

func displayResources(out io.Writer, resources []*models.Node, format string) error {
	switch format {
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSONArray(out, resources))
	case teleport.Text:
		return trace.Wrap(displayResourcesText(out, resources))
	default:
		return trace.Wrap(utils.WriteYAML(out, resources))
	}
}

func strPtrToStr(str *string) string {
	if str == nil {
		return ""
	}
	return *str
}

func displayResourcesText(out io.Writer, resources []*models.Node) error {
	if len(resources) == 0 {
		_, err := fmt.Fprintln(out, "No resources found.")
		return trace.Wrap(err)
	}

	table := asciitable.MakeTable([]string{
		"Resource Id",
		"Name",
		"Alias",
		"Kind",
		"Subkind",
		"Source",
		"Origin",
		"Origin Type",
	})

	for _, resource := range resources {
		props, err := resource.Properties.AsResourceProperties()
		if err != nil {
			return trace.Wrap(err)
		}

		table.AddRow([]string{
			resource.Id.String(),
			resource.Name,
			strPtrToStr(props.Alias),
			string(resource.Kind),
			resource.SubKind,
			strPtrToStr(props.Source),
			strPtrToStr(props.Origin),
			props.OriginType,
		})
	}
	_, err := fmt.Fprintln(out, table.AsBuffer().String())
	return trace.Wrap(err)
}
