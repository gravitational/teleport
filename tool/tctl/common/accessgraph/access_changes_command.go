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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	accessgraph "github.com/gravitational/teleport/lib/accessgraph/apiclient"
	graphmodels "github.com/gravitational/teleport/lib/accessgraph/apiclient/models/graph"
	diffmodels "github.com/gravitational/teleport/lib/accessgraph/apiclient/models/jsondiff"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/utils"
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
	// Structured filters
	source []string
	kind   []string
	typ    []string
	// filters allows you to combine structured filters in the form of `--filters source=x,kind=y`
	filters []string

	// Output control
	limit int
}

type accessChangesGetArgs struct {
	cmd      *kingpin.CmdClause
	changeID string
}

func (c *AccessGraphCommand) initAccessChanges(app *kingpin.Application) {
	accessChangesCmd := app.Command("access-changes", "Monitor access path changes to crown jewels.")
	accessChangesCmd.Flag("format", "Output format. (Values: text, json, yaml)").
		Default(teleport.Text).
		EnumVar(&c.accessChanges.format, teleport.Text, teleport.JSON, teleport.YAML)
	c.accessChanges.cmd = accessChangesCmd
	c.initAccessChangesList(c.accessChanges.cmd)
	c.initAccessChangesGet(c.accessChanges.cmd)
}

func (c *AccessGraphCommand) initAccessChangesList(parent *kingpin.CmdClause) {
	lsCmd := parent.Command("ls", "List access path changes for your crown jewels")

	lsCmd.Flag("search", "Search term to filter access changes.").
		StringVar(&c.accessChanges.ls.search)
	lsCmd.Flag("filter", "Filter by comma-separated key=value pairs (keys: type, kind, source). Pairs within one --filter are AND'd; repeat --filter to OR. Example: --filter kind=resource,source=AWS --filter type=teleport_user.").
		StringsVar(&c.accessChanges.ls.filters)
	lsCmd.Flag("type", "Filter by origin type (repeatable; each is OR'd). Combine axes with --filter.").
		EnumsVar(&c.accessChanges.ls.typ, allowedOriginTypes...)
	lsCmd.Flag("kind", "Filter by change kind (repeatable; each is OR'd). Combine axes with --filter.").
		EnumsVar(&c.accessChanges.ls.kind, allowedKinds...)
	lsCmd.Flag("source", "Filter by source (repeatable; each is OR'd). Combine axes with --filter.").
		EnumsVar(&c.accessChanges.ls.source, allowedSources...)

	lsCmd.Flag("limit", "Maximum number of changes to return (0 for unlimited).").
		Default("100").
		IntVar(&c.accessChanges.ls.limit)

	c.accessChanges.ls.cmd = lsCmd
}

func (c *AccessGraphCommand) initAccessChangesGet(parent *kingpin.CmdClause) {
	getCmd := parent.Command("get", "Get details about an access path change")

	getCmd.Arg("change-id", "The ID of the access path change to retrieve.").Required().StringVar(&c.accessChanges.get.changeID)

	c.accessChanges.get.cmd = getCmd
}

// AccessChangesGet executes `tctl access-changes get <change-id>`.
func (c *AccessGraphCommand) AccessChangesGet(ctx context.Context, client *accessgraph.ClientWithResponses) error {
	resp, err := doRequest(client.GetCrownJewelAccessPathsWithResponse(ctx, c.accessChanges.get.changeID))
	if err != nil {
		return trace.Wrap(err)
	}
	if resp.JSON200 == nil {
		return trace.Errorf("received nil json response from Access Graph API")
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
		if resp.JSON200 == nil {
			return nil, trace.Errorf("received nil json response from Access Graph API")
		}
		// Guard against a backend that returns a non-advancing cursor, which would otherwise spin forever.
		if cursor != nil && resp.JSON200.NextCursor != nil && *resp.JSON200.NextCursor == *cursor {
			slog.DebugContext(ctx, "Access Graph cursor did not advance; stopping pagination", "cursor", *cursor)
			return changes, nil
		}
		changes = append(changes, resp.JSON200.Data...)
		if limit > 0 && len(changes) >= limit {
			return changes[:limit], nil
		}
		if resp.JSON200.NextCursor == nil {
			return changes, nil
		}
		cursor = resp.JSON200.NextCursor
	}
}

// Allowed filter values, derived from the Access Graph web filter tree
// (access-graph/web/src/routes/crownjewels/access/typeOptions.ts)
var (
	// allowedOriginTypes backs --type and the type key of --filter.
	allowedOriginTypes = []string{
		"aws_ec2", "aws_eks", "aws_rds", "aws_s3", "aws_user",
		"entra_user", "gitlab_project", "gitlab_user",
		"okta_application", "okta_user",
		"teleport_application", "teleport_bot", "teleport_database",
		"teleport_desktop", "teleport_kubernetes", "teleport_node", "teleport_user",
	}
	// allowedSources backs --source and the source key of --filter.
	allowedSources = []string{"AWS", "Entra", "Gitlab", "Okta", "TELEPORT"}
	// allowedKinds backs --kind and the kind key of --filter.
	allowedKinds = []string{"identity", "resource"}
)

type accessChangeTypeFilter struct {
	OriginType *string `json:"origin_type,omitempty"`
	Source     *string `json:"source,omitempty"`
	Kind       *string `json:"kind,omitempty"`
}

func constructAccessChangesListQuery(args accessChangesArgs) (accessgraph.ListCrownJewelAccessPathsParams, error) {
	params := accessgraph.ListCrownJewelAccessPathsParams{}

	// Assemble the OR'd envelope list. --filter envelopes come first, in the
	// order given then --type, --kind, and --source value. (Order is cosmetic
	// and to make testing easier)
	var envelopes []accessChangeTypeFilter
	for _, raw := range args.ls.filters {
		env, err := parseFilterEnvelope(raw)
		if err != nil {
			return params, trace.Wrap(err)
		}
		envelopes = append(envelopes, env)
	}
	for _, t := range args.ls.typ {
		v := t
		envelopes = append(envelopes, accessChangeTypeFilter{OriginType: &v})
	}
	for _, k := range args.ls.kind {
		v := k
		envelopes = append(envelopes, accessChangeTypeFilter{Kind: &v})
	}
	for _, s := range args.ls.source {
		v := s
		envelopes = append(envelopes, accessChangeTypeFilter{Source: &v})
	}

	if len(envelopes) > 0 {
		filterJson, err := json.Marshal(envelopes)
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

// parseFilterEnvelope turns a single --filter value ("kind=resource,source=AWS")
// into one type_filter envelope, where the comma-separated pairs are AND'd.
func parseFilterEnvelope(s string) (accessChangeTypeFilter, error) {
	var env accessChangeTypeFilter
	seenKeys := make(map[string]struct{})
	for pair := range strings.SplitSeq(s, ",") {
		key, value, ok := strings.Cut(pair, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok || key == "" || value == "" {
			return env, trace.BadParameter("invalid filter %q: want comma-separated key=value pairs", s)
		}
		if _, seen := seenKeys[key]; seen {
			return env, trace.BadParameter("duplicate filter key %q in filter %q", key, s)
		}
		seenKeys[key] = struct{}{}
		v := value
		switch key {
		case "type":
			if !slices.Contains(allowedOriginTypes, v) {
				return env, trace.BadParameter("invalid type %q: want one of %v", v, allowedOriginTypes)
			}
			env.OriginType = &v
		case "kind":
			if !slices.Contains(allowedKinds, v) {
				return env, trace.BadParameter("invalid kind %q: want one of %v", v, allowedKinds)
			}
			env.Kind = &v
		case "source":
			if !slices.Contains(allowedSources, v) {
				return env, trace.BadParameter("invalid source %q: want one of %v", v, allowedSources)
			}
			env.Source = &v
		default:
			return env, trace.BadParameter("unknown filter key %q: want type, kind, or source", key)
		}
	}

	return env, nil
}

func displayAccessChanges(out io.Writer, changes []accessgraph.AccessPathSummaryItem, format string) error {
	if changes == nil {
		changes = []accessgraph.AccessPathSummaryItem{}
	}
	return writeOutput(out, changes, format, func(w io.Writer) error {
		return displayAccessChangesText(w, changes)
	})
}

func displayAccessChangesText(out io.Writer, changes []accessgraph.AccessPathSummaryItem) error {
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
			utils.EscapeControl(change.Id),
			utils.EscapeControl(change.AffectedNode.Kind),
			utils.EscapeControl(change.AffectedNode.Name),
			utils.EscapeControl(change.AffectedNode.Source),
			utils.EscapeControl(change.AffectedNode.OriginType),
			utils.EscapeControl(change.AffectedNode.Alias),
			change.CreatedAt.Format(time.RFC3339),
		})
	}
	_, err := fmt.Fprintln(out, table.AsBuffer().String())
	return trace.Wrap(err)
}

func displayAccessChange(out io.Writer, change *accessgraph.AccessPathDiff, format string) error {
	return writeOutput(out, change, format, func(w io.Writer) error {
		return displayAccessChangeText(w, change)
	})
}

func displayAccessChangeText(out io.Writer, change *accessgraph.AccessPathDiff) error {
	// UUID fields need no escaping; string fields below are escaped.
	fmt.Fprintf(out, "Change ID: %s\n\n", change.ChangeId)

	fmt.Fprintln(out, "Affected Node:")
	nodeTable := asciitable.MakeTable([]string{"ID", "Kind", "Name", "Source", "Origin Type", "Alias"})
	nodeTable.AddRow([]string{
		change.AffectedNode.Id.String(),
		utils.EscapeControl(change.AffectedNode.Kind),
		utils.EscapeControl(change.AffectedNode.Name),
		utils.EscapeControl(change.AffectedNode.Source),
		utils.EscapeControl(change.AffectedNode.OriginType),
		utils.EscapeControl(change.AffectedNode.Alias),
	})
	fmt.Fprintln(out, nodeTable.AsBuffer().String())

	if len(change.Diff) == 0 {
		_, err := fmt.Fprintln(out, "No changes.")
		return trace.Wrap(err)
	}

	// Fold diff additions into the base lookups so an edge pointing at a
	// newly-added node resolves to its name, not a raw UUID.
	baseNodes, baseEdges := buildBaseLookups(change.Base)
	baseNodes, baseEdges = addDiffAdditionsToLookups(change.Diff, baseNodes, baseEdges)

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

	fmt.Fprintf(out, "Changes (%d):\n", len(ops))
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
			name = id
		}
		rows = append(rows, []string{utils.EscapeControl(string(op.Op)), entityType, utils.EscapeControl(name), utils.EscapeControl(kind), utils.EscapeControl(originType)})
	}
	diffTable := asciitable.MakeTableWithTruncatedColumn([]string{"Operation", "Type", "Name", "Kind", "Origin Type"}, rows, "Name")
	_, err := fmt.Fprintln(out, diffTable.AsBuffer().String())
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

// addDiffAdditionsToLookups folds nodes and edges from "add" operations into the
// base lookups so entities added by the diff resolve like pre-change ones. The
// maps may arrive nil. Edge payloads carry their UUID only in the path, so edges
// are keyed by the path id.
func addDiffAdditionsToLookups(
	ops []diffmodels.Operation,
	nodes map[string]accessgraph.GenericNode,
	edges map[string]graphmodels.Edge,
) (map[string]accessgraph.GenericNode, map[string]graphmodels.Edge) {
	for _, op := range ops {
		if op.Op != diffmodels.OperationOpAdd || op.Value == nil || *op.Value == nil {
			continue
		}
		path := strPtrToStr(op.Path)
		switch diffEntityType(path) {
		case "node":
			var n accessgraph.GenericNode
			if err := remarshal(*op.Value, &n); err == nil {
				if nodes == nil {
					nodes = make(map[string]accessgraph.GenericNode)
				}
				nodes[n.Id] = n
			}
		case "edge":
			var e graphmodels.Edge
			if err := remarshal(*op.Value, &e); err == nil {
				if edges == nil {
					edges = make(map[string]graphmodels.Edge)
				}
				edges[diffEntityID(path)] = e
			}
		}
	}
	return nodes, edges
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

// nodeLabel returns the most descriptive label for a node: alias > name > ID.
func nodeLabel(n accessgraph.GenericNode) string {
	switch {
	case n.Alias != "":
		return n.Alias
	case n.Name != "":
		return n.Name
	default:
		return n.Id
	}
}

// edgeLabel formats an edge as "from → to" using node labels from the base graph.
func edgeLabel(e graphmodels.Edge, nodes map[string]accessgraph.GenericNode) string {
	fromID, toID := e.From.String(), e.To.String()
	fromLabel, toLabel := fromID, toID
	if n, ok := nodes[fromID]; ok {
		fromLabel = nodeLabel(n)
	}
	if n, ok := nodes[toID]; ok {
		toLabel = nodeLabel(n)
	}
	return fmt.Sprintf("%s → %s", fromLabel, toLabel)
}
