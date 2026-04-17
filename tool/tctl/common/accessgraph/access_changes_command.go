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
	"slices"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	accessgraph "github.com/gravitational/teleport/lib/accessgraph/apiclient"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/trace"
)

type accessChangesArgs struct {
	cmd *kingpin.CmdClause
	ls  accessChangesListArgs
	get accessChangeGetArgs

	// Output format
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

type accessChangeGetArgs struct {
	cmd      *kingpin.CmdClause
	changeId string
}

func (c *AccessGraphCommand) initAccessChanges(app *kingpin.Application) {
	accessChangesCmd := app.Command("access-changes", "Monitor access path changes to crown jewels.").Hidden()
	registerFormatFlag(accessChangesCmd, &c.accessChanges.format, teleport.YAML)
	c.accessChanges.cmd = accessChangesCmd
	c.initAccessChangesList(c.accessChanges.cmd)
	c.initAccessChangeGet(c.accessChanges.cmd)
}

// Access path resource filter
type AccessChangeTypeFilter struct {
	// OriginType is the origin type of the node.
	OriginType *string `json:"origin_type"`
	// Source is the source of the node.
	Source *string `json:"source,omitempty"`
	// Kind is the kind of the node.
	Kind *string `json:"kind,omitempty"`
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
	lsCmd.Flag("limit", "Max rows to display (0 for unlimited)").
		Default("100").
		IntVar(&c.accessChanges.ls.limit)

	c.accessChanges.ls.cmd = lsCmd
}

func (c *AccessGraphCommand) initAccessChangeGet(parent *kingpin.CmdClause) {
	getCmd := parent.Command("get", "Get details about an access path change.")

	getCmd.Arg("change-id", "The Id of the access path change to retrieve.").Required().StringVar(&c.accessChanges.get.changeId)

	c.accessChanges.get.cmd = getCmd
}

// AccessChangeGet executes `tctl access-changes get <change-id>`.
func (c *AccessGraphCommand) AccessChangeGet(ctx context.Context, args accessGraphServices) error {
	resp, err := doRequest(args.accessGraph.GetCrownJewelAccessPathsWithResponse(ctx, c.accessChanges.get.changeId))

	if err != nil {
		return trace.Wrap(err)
	}

	return displayAccessChange(c.stdout, resp.JSON200, c.accessChanges.format)
}

// AccessChangesList executes `tctl access-changes ls`.
func (c *AccessGraphCommand) AccessChangesList(ctx context.Context, args accessGraphServices) error {
	query, err := constructAccessChangesListQuery(c.accessChanges)
	if err != nil {
		return trace.Wrap(err)
	}

	resp, err := doRequest(args.accessGraph.ListCrownJewelAccessPathsWithResponse(ctx, query))
	if err != nil {
		return trace.Wrap(err)
	}
	return displayAccessChanges(c.stdout, resp.JSON200.Data, c.accessChanges.format)
}

func constructAccessChangesListQuery(args accessChangesArgs) (*accessgraph.ListCrownJewelAccessPathsParams, error) {
	params := accessgraph.ListCrownJewelAccessPathsParams{}

	// A limit of 0 means no limit; omit the field so the API treats it as unlimited.
	if args.ls.limit != 0 {
		params.Limit = &args.ls.limit
	}

	if args.ls.kind != "" || args.ls.source != "" || args.ls.typ != "" {
		filterJson, err := json.Marshal([]AccessChangeTypeFilter{
			{
				OriginType: &args.ls.typ,
				Source:     &args.ls.source,
				Kind:       &args.ls.kind,
			},
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		filterStr := string(filterJson)
		params.TypeFilter = &filterStr
	}

	if args.ls.search != "" {
		params.Search = &args.ls.search
	}

	return &params, nil
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
		"Change Id",
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

	// Build base lookup maps so removed entities (which carry no value in the diff)
	// can be resolved to their names from the pre-change graph.
	baseNodes, baseEdges, err := buildBaseLookups(change.Base)
	if err != nil {
		return trace.Wrap(err)
	}

	// Marshal diff to maps for safe field access regardless of Operation struct layout.
	data, err := json.Marshal(change.Diff)
	if err != nil {
		return trace.Wrap(err)
	}
	var ops []map[string]any
	if err := json.Unmarshal(data, &ops); err != nil {
		return trace.Wrap(err)
	}

	// Sort so node changes appear before edge changes.
	slices.SortStableFunc(ops, func(a, b map[string]any) int {
		pathA, _ := a["path"].(string)
		pathB, _ := b["path"].(string)
		typeA, typeB := diffEntityType(pathA), diffEntityType(pathB)
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
		operation, _ := op["op"].(string)
		path, _ := op["path"].(string)
		entityType := diffEntityType(path)
		id := diffEntityID(path)

		var name, kind, originType string
		if value, ok := op["value"].(map[string]any); ok {
			// add/replace: entity data is in the diff value.
			name, kind, originType = entityDisplayFields(value, baseNodes)
		} else {
			// remove: resolve entity from the pre-change base graph.
			switch entityType {
			case "node":
				if node, ok := baseNodes[id]; ok {
					name = nodeLabel(node)
					kind, _ = node["kind"].(string)
					originType, _ = node["origin_type"].(string)
				}
			case "edge":
				if edge, ok := baseEdges[id]; ok {
					name = edgeLabel(edge, baseNodes)
					kind, _ = edge["edge_type"].(string)
				}
			}
			if name == "" {
				name = truncateID(id)
			}
		}
		rows = append(rows, []string{operation, entityType, name, kind, originType})
	}
	diffTable := asciitable.MakeTableWithTruncatedColumn([]string{"Operation", "Type", "Name", "Kind", "Origin Type"}, rows, "Name")
	_, err = fmt.Fprintln(writer, diffTable.AsBuffer().String())
	return trace.Wrap(err)
}

// buildBaseLookups marshals the base graph and returns node and edge maps keyed by UUID.
func buildBaseLookups(base any) (nodes, edges map[string]map[string]any, err error) {
	data, err := json.Marshal(base)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	var g map[string][]map[string]any
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return buildIDMap(g["nodes"]), buildIDMap(g["edges"]), nil
}

// buildIDMap indexes a slice of entity maps by their "id" field.
func buildIDMap(items []map[string]any) map[string]map[string]any {
	m := make(map[string]map[string]any, len(items))
	for _, item := range items {
		if id, ok := item["id"].(string); ok {
			m[id] = item
		}
	}
	return m
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

// entityDisplayFields extracts (name, kind, originType) from a diff operation value.
// Handles both node values (alias/name, kind, origin_type) and edge values (from → to).
func entityDisplayFields(value map[string]any, baseNodes map[string]map[string]any) (name, kind, originType string) {
	if value == nil {
		return
	}
	kind, _ = value["kind"].(string)
	originType, _ = value["origin_type"].(string)
	if _, isEdge := value["from"]; isEdge {
		name = edgeLabel(value, baseNodes)
	} else {
		name = nodeLabel(value)
	}
	return
}

// nodeLabel returns the most descriptive label for a node: alias > name > truncated ID.
func nodeLabel(node map[string]any) string {
	if alias, _ := node["alias"].(string); alias != "" {
		return alias
	}
	if name, _ := node["name"].(string); name != "" {
		return name
	}
	id, _ := node["id"].(string)
	return truncateID(id)
}

// edgeLabel formats an edge as "from → to" using node labels from the base graph.
func edgeLabel(edge map[string]any, nodes map[string]map[string]any) string {
	fromID, _ := edge["from"].(string)
	toID, _ := edge["to"].(string)
	fromLabel, toLabel := truncateID(fromID), truncateID(toID)
	if node, ok := nodes[fromID]; ok {
		fromLabel = nodeLabel(node)
	}
	if node, ok := nodes[toID]; ok {
		toLabel = nodeLabel(node)
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
