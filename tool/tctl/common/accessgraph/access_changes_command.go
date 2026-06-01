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
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/trace"
)

type accessChangesArgs struct {
	cmd    *kingpin.CmdClause
	ls     accessChangesListArgs
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

func (c *AccessGraphCommand) initAccessChanges(app *kingpin.Application) {
	accessChangesCmd := app.Command("access-changes", "Monitor access path changes to crown jewels.")
	accessChangesCmd.Flag("format", "Output format. (Values: text, json, yaml)").
		Default(teleport.Text).
		EnumVar(&c.accessChanges.format, teleport.Text, teleport.JSON, teleport.YAML)
	c.accessChanges.cmd = accessChangesCmd
	c.initAccessChangesList(c.accessChanges.cmd)
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
	for pair := range strings.SplitSeq(s, ",") {
		key, value, ok := strings.Cut(pair, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok || key == "" || value == "" {
			return env, trace.BadParameter("invalid filter %q: want comma-separated key=value pairs", s)
		}
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
			change.Id,
			change.AffectedNode.Kind,
			change.AffectedNode.Name,
			change.AffectedNode.Source,
			change.AffectedNode.OriginType,
			change.AffectedNode.Alias,
			change.CreatedAt.Format(time.RFC3339),
		})
	}
	_, err := fmt.Fprintln(out, table.AsBuffer().String())
	return trace.Wrap(err)
}
