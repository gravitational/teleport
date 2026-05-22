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
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/teleport"
	accessgraph "github.com/gravitational/teleport/lib/accessgraph/apiclient"
	graphmodels "github.com/gravitational/teleport/lib/accessgraph/apiclient/models/graph"
	diffmodels "github.com/gravitational/teleport/lib/accessgraph/apiclient/models/jsondiff"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/trace"
)

type accessChangesArgs struct {
	cmd    *kingpin.CmdClause
	ls     accessChangesListArgs
	get    accessChangesGetArgs
	format string
}

type accessChangesListArgs struct {
	cmd *kingpin.CmdClause

	// General filters
	search string
	source string
	kind   string
	typ    string

	// Output control
	limit int
}

type accessChangesGetArgs struct {
	cmd      *kingpin.CmdClause
	changeID string
}

func (c *AccessGraphCommand) initAccessChanges(app *kingpin.Application) {
	accessChangesCmd := app.Command("access-changes", "Monitor access path changes to crown jewels.").Hidden()
	accessChangesCmd.Flag("format", "Output format. (Values: text, json, yaml)").
		Default(teleport.Text).
		EnumVar(&c.accessChanges.format, teleport.Text, teleport.JSON, teleport.YAML)
	c.accessChanges.cmd = accessChangesCmd
	c.initAccessChangesList(c.accessChanges.cmd)
	c.initAccessChangesGet(c.accessChanges.cmd)
}

func (c *AccessGraphCommand) initAccessChangesList(parent *kingpin.CmdClause) {
	lsCmd := parent.Command("ls", "List access path changes.")

	lsCmd.Flag("search", "Search term to filter access changes.").
		StringVar(&c.accessChanges.ls.search)
	lsCmd.Flag("kind", "Filter by change kind (Examples: resource, identity, etc...).").
		StringVar(&c.accessChanges.ls.kind)
	lsCmd.Flag("type", "Filter by change type (Values: aws_s3, teleport_user, etc...).").
		StringVar(&c.accessChanges.ls.typ)
	lsCmd.Flag("source", "Filter by source of the change (Example: Teleport, Okta, etc...).").
		StringVar(&c.accessChanges.ls.source)
	lsCmd.Flag("limit", "Maximum number of changes to return (0 for unlimited).").
		Default("100").
		IntVar(&c.accessChanges.ls.limit)

	c.accessChanges.ls.cmd = lsCmd
}

func (c *AccessGraphCommand) initAccessChangesGet(parent *kingpin.CmdClause) {
	getCmd := parent.Command("get", "Get details about an access path change.")

	getCmd.Arg("change-id", "The ID of the access path change to retrieve.").Required().StringVar(&c.accessChanges.get.changeID)

	c.accessChanges.get.cmd = getCmd
}

// AccessChangesGet executes `tctl access-changes get <change-id>`.
func (c *AccessGraphCommand) AccessChangesGet(ctx context.Context, client *accessgraph.ClientWithResponses) error {
	resp, err := doRequest(client.GetCrownJewelAccessPathsWithResponse(ctx, c.accessChanges.get.changeID))
	if err != nil {
		return trace.Wrap(err)
	}
	return displayAccessChange(c.stdout, resp.JSON200, c.accessChanges.format)
}

// AccessChangesList executes `tctl access-changes ls`.
func (c *AccessGraphCommand) AccessChangesList(ctx context.Context, client *accessgraph.ClientWithResponses) error {
	params, err := constructAccessChangesListQuery(c.accessChanges)
	if err != nil {
		return trace.Wrap(err)
	}
	changes, err := fetchAccessChanges(ctx, client, params, c.accessChanges.ls.limit)
	if err != nil {
		return trace.Wrap(err)
	}
	return displayAccessChanges(c.stdout, changes, c.accessChanges.format)
}

// fetchAccessChanges paginates ListCrownJewelAccessPaths until limit items have been collected or the server runs out of pages; limit<=0 disables the cap.
func fetchAccessChanges(
	ctx context.Context,
	client *accessgraph.ClientWithResponses,
	params accessgraph.ListCrownJewelAccessPathsParams,
	limit int,
) ([]accessgraph.AccessPathSummaryItem, error) {
	var (
		changes []accessgraph.AccessPathSummaryItem
		cursor  *string
	)
	for {
		params.Iterator = cursor
		resp, err := doRequest(client.ListCrownJewelAccessPathsWithResponse(ctx, &params))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		changes = append(changes, resp.JSON200.Data...)
		if limit > 0 && len(changes) >= limit {
			return changes[:limit], nil
		}
		if resp.JSON200.NextCursor == nil {
			return changes, nil
		}
		// Guard against a backend that returns a non-advancing cursor, which would otherwise spin forever.
		if cursor != nil && *resp.JSON200.NextCursor == *cursor {
			slog.DebugContext(ctx, "Access Graph cursor did not advance; stopping pagination", "cursor", *cursor)
			return changes, nil
		}
		cursor = resp.JSON200.NextCursor
	}
}

type accessChangeTypeFilter struct {
	OriginType *string `json:"origin_type,omitempty"`
	Source     *string `json:"source,omitempty"`
	Kind       *string `json:"kind,omitempty"`
}

func constructAccessChangesListQuery(args accessChangesArgs) (accessgraph.ListCrownJewelAccessPathsParams, error) {
	params := accessgraph.ListCrownJewelAccessPathsParams{}

	if args.ls.kind != "" || args.ls.source != "" || args.ls.typ != "" {
		filter := accessChangeTypeFilter{}
		if args.ls.typ != "" {
			filter.OriginType = &args.ls.typ
		}
		if args.ls.source != "" {
			filter.Source = &args.ls.source
		}
		if args.ls.kind != "" {
			filter.Kind = &args.ls.kind
		}
		filterJson, err := json.Marshal([]accessChangeTypeFilter{filter})
		if err != nil {
			return params, trace.Wrap(err)
		}
		filterStr := string(filterJson)
		params.TypeFilter = &filterStr
	}

	if args.ls.search != "" {
		params.Search = &args.ls.search
	}

	return params, nil
}

func displayAccessChanges(writer io.Writer, changes []accessgraph.AccessPathSummaryItem, format string) error {
	if changes == nil {
		changes = []accessgraph.AccessPathSummaryItem{}
	}
	return writeOutput(writer, changes, format, func(w io.Writer) error {
		return displayAccessChangesText(w, changes)
	})
}

func displayAccessChangesText(writer io.Writer, changes []accessgraph.AccessPathSummaryItem) error {
	table := asciitable.MakeTable([]string{
		"Change ID",
		"Kind",
		"Name",
		"Source",
		"Origin Type",
		"Alias",
		"Created At",
	})

	for _, change := range changes {
		table.AddRow([]string{
			change.Id,
			change.AffectedNode.Kind,
			change.AffectedNode.Name,
			change.AffectedNode.Source,
			change.AffectedNode.OriginType,
			change.AffectedNode.Alias,
			change.CreatedAt.Format(time.RFC3339),
		})
	}
	_, err := fmt.Fprintln(writer, table.AsBuffer().String())
	return trace.Wrap(err)
}

func displayAccessChange(writer io.Writer, change *accessgraph.AccessPathDiff, format string) error {
	return writeOutput(writer, change, format, func(w io.Writer) error {
		return displayAccessChangeText(w, change)
	})
}

func displayAccessChangeText(writer io.Writer, change *accessgraph.AccessPathDiff) error {
	fmt.Fprintf(writer, "Change ID: %s\n\n", change.ChangeId)

	fmt.Fprintln(writer, "Affected Node:")
	nodeTable := asciitable.MakeTable([]string{"ID", "Kind", "Name", "Source", "Origin Type", "Alias"})
	nodeTable.AddRow([]string{
		change.AffectedNode.Id.String(),
		change.AffectedNode.Kind,
		change.AffectedNode.Name,
		change.AffectedNode.Source,
		change.AffectedNode.OriginType,
		change.AffectedNode.Alias,
	})
	fmt.Fprintln(writer, nodeTable.AsBuffer().String())

	if len(change.Diff) == 0 {
		_, err := fmt.Fprintln(writer, "No changes.")
		return trace.Wrap(err)
	}

	baseNodes, baseEdges := buildBaseLookups(change.Base)

	// Sort nodes before edges; clone first to avoid mutating the caller's slice.
	ops := slices.Clone(change.Diff)
	slices.SortStableFunc(ops, func(a, b diffmodels.Operation) int {
		typeA := diffEntityType(strPtrToStr(a.Path))
		typeB := diffEntityType(strPtrToStr(b.Path))
		if typeA == typeB {
			return 0
		}
		if typeA == "node" {
			return -1
		}
		return 1
	})

	fmt.Fprintf(writer, "Changes (%d):\n", len(ops))
	var rows [][]string
	for _, op := range ops {
		path := strPtrToStr(op.Path)
		entityType := diffEntityType(path)
		id := diffEntityID(path)

		var name, kind, originType string
		if op.Value != nil && *op.Value != nil {
			// Op carries a payload (add/replace) — decode into the typed entity for this path prefix.
			switch entityType {
			case "node":
				var n accessgraph.GenericNode
				if err := remarshal(*op.Value, &n); err == nil {
					name, kind, originType = nodeLabel(n), n.Kind, n.OriginType
				}
			case "edge":
				var e graphmodels.Edge
				if err := remarshal(*op.Value, &e); err == nil {
					name, kind = edgeLabel(e, baseNodes), e.EdgeType
				}
			}
		} else {
			// remove: no payload — resolve entity from the pre-change base graph.
			switch entityType {
			case "node":
				if n, ok := baseNodes[id]; ok {
					name, kind, originType = nodeLabel(n), n.Kind, n.OriginType
				}
			case "edge":
				if e, ok := baseEdges[id]; ok {
					name, kind = edgeLabel(e, baseNodes), e.EdgeType
				}
			}
		}
		if name == "" {
			name = truncateID(id)
		}
		rows = append(rows, []string{string(op.Op), entityType, name, kind, originType})
	}
	diffTable := asciitable.MakeTableWithTruncatedColumn([]string{"Operation", "Type", "Name", "Kind", "Origin Type"}, rows, "Name")
	_, err := fmt.Fprintln(writer, diffTable.AsBuffer().String())
	return trace.Wrap(err)
}

// buildBaseLookups indexes the pre-change graph by UUID so removed entities (which carry no value in the diff) can be resolved.
func buildBaseLookups(base accessgraph.GenericNodesList) (nodes map[string]accessgraph.GenericNode, edges map[string]graphmodels.Edge) {
	if base.Nodes != nil {
		nodes = make(map[string]accessgraph.GenericNode, len(*base.Nodes))
		for _, n := range *base.Nodes {
			nodes[n.Id] = n
		}
	}
	if base.Edges != nil {
		edges = make(map[string]graphmodels.Edge, len(*base.Edges))
		for _, e := range *base.Edges {
			edges[e.Id.String()] = e
		}
	}
	return
}

// remarshal round-trips src through JSON into dst (used to decode jsondiff Operation values into typed structs).
func remarshal(src, dst any) error {
	data, err := json.Marshal(src)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(json.Unmarshal(data, dst))
}

// diffEntityType returns "node" or "edge" based on a JSON Patch path (e.g. "/nodes/<id>").
func diffEntityType(path string) string {
	switch {
	case strings.HasPrefix(path, "/nodes/"):
		return "node"
	case strings.HasPrefix(path, "/edges/"):
		return "edge"
	default:
		return ""
	}
}

// diffEntityID extracts the UUID from a JSON Patch path (e.g. "/nodes/<id>" → "<id>").
func diffEntityID(path string) string {
	if idx := strings.LastIndexByte(path, '/'); idx >= 0 {
		return path[idx+1:]
	}
	return path
}

// nodeLabel returns the most descriptive label for a node: alias > name > truncated ID.
func nodeLabel(n accessgraph.GenericNode) string {
	switch {
	case n.Alias != "":
		return n.Alias
	case n.Name != "":
		return n.Name
	default:
		return truncateID(n.Id)
	}
}

// edgeLabel formats an edge as "from → to" using node labels from the base graph.
func edgeLabel(e graphmodels.Edge, nodes map[string]accessgraph.GenericNode) string {
	fromID, toID := e.From.String(), e.To.String()
	fromLabel, toLabel := truncateID(fromID), truncateID(toID)
	if n, ok := nodes[fromID]; ok {
		fromLabel = nodeLabel(n)
	}
	if n, ok := nodes[toID]; ok {
		toLabel = nodeLabel(n)
	}
	return fmt.Sprintf("%s → %s", fromLabel, toLabel)
}

// truncateID shortens a UUID to its last 8 characters for compact display.
func truncateID(id string) string {
	if len(id) > 8 {
		return fmt.Sprintf("…%s", id[len(id)-8:])
	}
	return id
}
