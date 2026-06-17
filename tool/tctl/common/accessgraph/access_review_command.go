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
	"log/slog"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	accessgraph "github.com/gravitational/teleport/lib/accessgraph/apiclient"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/utils"
)

// accessReviewPageSize is the per-request identity page size used to paginate
// the access endpoint. 25 balances latency against round-trips on the current
// (non-CTE) backend pagination.
const accessReviewPageSize = 25

type accessReviewArgs struct {
	cmd *kingpin.CmdClause

	query  string
	from   time.Time
	to     time.Time
	limit  int
	format string
}

func (c *AccessGraphCommand) initAccessReview(app *kingpin.Application) {
	cmd := app.Command("access-review", "Review which identities can access which resources.")

	// TODO: add structured convenience filters (e.g. --user, --acl, --role)
	// that build the query for the user. They will complement --query, which
	// stays as the power-user filter.
	cmd.Flag("query", "SQL SELECT against access_path scoping the identities to review.").
		Required().
		StringVar(&c.accessReview.query)
	cmd.Flag("from", fmt.Sprintf("Show access activity at or after this time; enables the activity columns. (Examples: %s, %s, 24h, 7d). Default: activity hidden.", time.RFC3339, time.DateOnly)).
		SetValue(timeValue{target: &c.accessReview.from})
	cmd.Flag("to", "Upper bound for access activity. Defaults to now when --from is set; requires --from.").
		SetValue(timeValue{target: &c.accessReview.to})
	cmd.Flag("limit", "Maximum number of identities to return.").
		Default("50").
		IntVar(&c.accessReview.limit)
	cmd.Flag("format", "Output format. (Values: text, json, yaml)").
		Default(teleport.Text).
		EnumVar(&c.accessReview.format, teleport.Text, teleport.JSON, teleport.YAML)

	c.accessReview.cmd = cmd
}

// AccessReview executes `tctl access-review`.
func (c *AccessGraphCommand) AccessReview(ctx context.Context, client *accessgraph.ClientWithResponses) error {
	args := &c.accessReview

	// Always bounded: no 0/unlimited sentinel.
	if args.limit < 1 {
		return trace.BadParameter("--limit must be at least 1")
	}

	from, to := args.from, args.to
	showActivity := !from.IsZero() || !to.IsZero()
	if !to.IsZero() && from.IsZero() {
		return trace.BadParameter("--to requires --from")
	}
	if !from.IsZero() && to.IsZero() {
		to = time.Now()
	}
	if showActivity {
		if err := validateTimeWindow(from, to); err != nil {
			return trace.Wrap(err)
		}
	}

	params := accessgraph.ListIdentityAccessParams{Query: args.query}
	if showActivity {
		fromUTC, toUTC := from.UTC(), to.UTC()
		params.StartTime = &fromUTC
		params.EndTime = &toUTC
	}

	resp, truncated, err := fetchIdentityAccess(ctx, client, params, args.limit)
	if err != nil {
		// A bare 404 means the endpoint isn't routed here; a 404 with a body is a
		// real API error and surfaces verbatim below.
		if trace.IsNotFound(err) {
			return trace.Wrap(err, "access-review is unavailable on this cluster. This usually means Identity Security is not enabled, or that the access-review endpoint is not yet available on this cluster.")
		}
		return trace.Wrap(err)
	}

	output := buildAccessReviewOutput(resp)
	if resp.IacError != nil && *resp.IacError != "" {
		output.Warnings = append(output.Warnings, fmt.Sprintf("activity unavailable: %s", utils.EscapeControl(*resp.IacError)))
	}
	if truncated {
		output.Warnings = append(output.Warnings, fmt.Sprintf("results truncated at %d identities; narrow --query for the full set", args.limit))
	}

	return writeOutput(c.stdout, output, args.format, func(w io.Writer) error {
		return displayAccessReviewText(w, output, from, to, showActivity)
	})
}

// fetchIdentityAccess paginates the access endpoint up to maxResults identities,
// merging each page's rows and deduplicated nodes into a single response.
// truncated is true if more identities remained.
func fetchIdentityAccess(
	ctx context.Context,
	client *accessgraph.ClientWithResponses,
	params accessgraph.ListIdentityAccessParams,
	maxResults int,
) (*accessgraph.IdentityAccessResponse, bool, error) {
	pageSize := min(accessReviewPageSize, maxResults)
	params.Limit = &pageSize
	slog.DebugContext(ctx, "access review query", "query", params.Query, "max_results", maxResults, "page_size", pageSize)

	var (
		cursor    *string
		rows      []accessgraph.IdentityAccessRow
		iacErr    *string
		truncated bool
		pages     int
	)
	nodesByID := make(map[uuid.UUID]accessgraph.IdentityAccessNode)

	for {
		params.Iterator = cursor
		resp, err := doRequest(client.ListIdentityAccessWithResponse(ctx, &params))
		if err != nil {
			return nil, false, trace.Wrap(err)
		}
		if resp.JSON200 == nil {
			return nil, false, trace.Errorf("received nil json response from Access Graph API")
		}

		// Guard against a non-advancing cursor that would otherwise loop forever.
		if cursor != nil && resp.JSON200.NextCursor != nil && *resp.JSON200.NextCursor == *cursor {
			slog.DebugContext(ctx, "Access Graph cursor did not advance; stopping pagination", "cursor", *cursor)
			truncated = true
			break
		}

		pages++
		slog.DebugContext(ctx, "access review page fetched", "page", pages, "page_size", len(resp.JSON200.Data))
		for _, n := range resp.JSON200.Nodes {
			nodesByID[n.Id] = n
		}
		rows = append(rows, resp.JSON200.Data...)
		// Keep the last non-empty error; empty/nil pages don't clobber it.
		if resp.JSON200.IacError != nil && *resp.JSON200.IacError != "" {
			iacErr = resp.JSON200.IacError
		}

		if len(rows) >= maxResults {
			truncated = len(rows) > maxResults || resp.JSON200.NextCursor != nil
			rows = rows[:maxResults]
			break
		}
		if resp.JSON200.NextCursor == nil {
			break
		}
		cursor = resp.JSON200.NextCursor
	}

	nodes := make([]accessgraph.IdentityAccessNode, 0, len(nodesByID))
	for _, n := range nodesByID {
		nodes = append(nodes, n)
	}
	slog.DebugContext(ctx, "access review fetch complete", "pages", pages, "identities", len(rows), "nodes", len(nodes), "truncated", truncated)
	return &accessgraph.IdentityAccessResponse{Data: rows, Nodes: nodes, IacError: iacErr}, truncated, nil
}

// indexNodesByID maps nodes by id for resolveNode lookups.
func indexNodesByID(nodes []accessgraph.IdentityAccessNode) map[uuid.UUID]accessgraph.IdentityAccessNode {
	byID := make(map[uuid.UUID]accessgraph.IdentityAccessNode, len(nodes))
	for _, n := range nodes {
		byID[n.Id] = n
	}
	return byID
}

// --- output types -----------------------------------------------------------

// accessReviewOutput is the materialized review payload. Node references in the
// raw API response are resolved to full Node values here.
type accessReviewOutput struct {
	Identities []identityAccess `json:"identities" yaml:"identities"`
	Warnings   []string         `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

type identityAccess struct {
	Identity  node             `json:"identity" yaml:"identity"`
	Resources []resourceAccess `json:"resources" yaml:"resources"`
}

type resourceAccess struct {
	Resource      node          `json:"resource" yaml:"resource"`
	Level         string        `json:"level" yaml:"level"`
	Temporary     bool          `json:"temporary,omitempty" yaml:"temporary,omitempty"`
	GrantorCounts grantorCounts `json:"grantor_counts" yaml:"grantor_counts"`
	Grantors      []grantor     `json:"grantors" yaml:"grantors"`
	Activity      *activity     `json:"activity,omitempty" yaml:"activity,omitempty"`
}

type grantor struct {
	Node  node   `json:"node" yaml:"node"`
	Level string `json:"level" yaml:"level"`
}

type node struct {
	ID        string `json:"id" yaml:"id"`
	Name      string `json:"name" yaml:"name"`
	Alias     string `json:"alias,omitempty" yaml:"alias,omitempty"`
	Kind      string `json:"kind" yaml:"kind"`
	SubKind   string `json:"sub_kind,omitempty" yaml:"sub_kind,omitempty"`
	Source    string `json:"source,omitempty" yaml:"source,omitempty"`
	Origin    string `json:"origin,omitempty" yaml:"origin,omitempty"`
	Temporary bool   `json:"temporary,omitempty" yaml:"temporary,omitempty"`
}

type grantorCounts struct {
	Standing    int `json:"standing" yaml:"standing"`
	Impersonate int `json:"impersonate" yaml:"impersonate"`
	Request     int `json:"request" yaml:"request"`
}

type activity struct {
	Count      int64      `json:"count" yaml:"count"`
	LastAccess *time.Time `json:"last_access,omitempty" yaml:"last_access,omitempty"`
}

// --- restructuring ----------------------------------------------------------

// buildAccessReviewOutput resolves the identity-centric API response into the
// materialized output, looking up every identity/resource/grantor id against
// the response node list.
func buildAccessReviewOutput(resp *accessgraph.IdentityAccessResponse) accessReviewOutput {
	nodesByID := indexNodesByID(resp.Nodes)

	out := accessReviewOutput{Identities: make([]identityAccess, 0, len(resp.Data))}
	for _, row := range resp.Data {
		ia := identityAccess{
			Identity:  resolveNode(nodesByID, row.Identity),
			Resources: make([]resourceAccess, 0, len(row.Resources)),
		}
		for _, r := range row.Resources {
			info := r.AccessInfo
			ra := resourceAccess{
				Resource: resolveNode(nodesByID, r.Resource),
				Level:    string(info.Level),
				GrantorCounts: grantorCounts{
					Standing:    info.GrantorCounts.Standing,
					Impersonate: info.GrantorCounts.Impersonate,
					Request:     info.GrantorCounts.Request,
				},
				Grantors: make([]grantor, 0, len(info.Grantors)),
			}
			if info.Temporary != nil {
				ra.Temporary = *info.Temporary
			}
			for _, g := range info.Grantors {
				grantor := grantor{Node: resolveNode(nodesByID, g.Id), Level: string(g.Level)}
				ra.Grantors = append(ra.Grantors, grantor)
			}
			if info.Activity != nil {
				ra.Activity = &activity{Count: info.Activity.Count, LastAccess: info.Activity.LastAccess}
			}
			ia.Resources = append(ia.Resources, ra)
		}
		out.Identities = append(out.Identities, ia)
	}
	return out
}

// resolveNode looks up id in nodesByID. A referenced id missing from the node
// list yields a Node carrying only the id rather than failing the review.
func resolveNode(nodesByID map[uuid.UUID]accessgraph.IdentityAccessNode, id uuid.UUID) node {
	n, ok := nodesByID[id]
	if !ok {
		return node{ID: id.String()}
	}
	node := node{
		ID:      n.Id.String(),
		Name:    n.Name,
		Kind:    string(n.Kind),
		Alias:   strPtrToStr(n.Alias),
		SubKind: strPtrToStr(n.SubKind),
		Source:  strPtrToStr(n.Source),
		Origin:  strPtrToStr(n.Origin),
	}
	if n.Temporary != nil {
		node.Temporary = *n.Temporary
	}
	return node
}

// primaryGrantor returns the grantor that backs the access at its resolved
// level (the first match), falling back to the first grantor of any level.
func primaryGrantor(ra resourceAccess) (grantor, bool) {
	for _, g := range ra.Grantors {
		if g.Level == ra.Level {
			return g, true
		}
	}
	if len(ra.Grantors) > 0 {
		return ra.Grantors[0], true
	}
	return grantor{}, false
}

func grantorSummary(p grantorCounts) string {
	var parts []string
	if p.Standing > 0 {
		parts = append(parts, fmt.Sprintf("%d standing", p.Standing))
	}
	if p.Impersonate > 0 {
		parts = append(parts, fmt.Sprintf("%d impersonate", p.Impersonate))
	}
	if p.Request > 0 {
		parts = append(parts, fmt.Sprintf("%d request", p.Request))
	}
	return strings.Join(parts, ", ")
}

// --- display ----------------------------------------------------------------

func displayAccessReviewText(out io.Writer, output accessReviewOutput, from, to time.Time, showActivity bool) error {
	if showActivity {
		if _, err := fmt.Fprintf(out, "Period: %s → %s\n\n", from.Format(time.RFC3339), to.Format(time.RFC3339)); err != nil {
			return trace.Wrap(err)
		}
	}

	if len(output.Identities) == 0 {
		if _, err := fmt.Fprintln(out, "No access found."); err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(writeWarnings(out, output.Warnings))
	}

	if err := renderAccessReviewSummary(out, output, showActivity); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(writeWarnings(out, output.Warnings))
}

// renderAccessReviewSummary shows one row per (identity, resource) with the
// resolved level, the primary grantor, and the grantor path counts.
func renderAccessReviewSummary(out io.Writer, output accessReviewOutput, showActivity bool) error {
	headers := []string{"Identity", "Kind", "Resource", "Resource Kind", "Access Level", "Granted By", "Grantor Counts"}
	if showActivity {
		headers = append(headers, "Accesses", "Last Access")
	}

	var rows [][]string
	for _, ia := range output.Identities {
		if len(ia.Resources) == 0 {
			row := []string{cellName(ia.Identity), cellKind(ia.Identity), "", "", "", "", ""}
			if showActivity {
				row = append(row, "", "")
			}
			rows = append(rows, row)
			continue
		}
		for i, ra := range ia.Resources {
			identity, kind := "", ""
			if i == 0 {
				identity, kind = cellName(ia.Identity), cellKind(ia.Identity)
			}

			level := utils.EscapeControl(ra.Level)
			if ra.Temporary {
				level += "*"
			}
			granted := ""
			if g, ok := primaryGrantor(ra); ok {
				granted = grantorName(g)
			}
			row := []string{identity, kind, cellName(ra.Resource), cellKind(ra.Resource), level, granted, grantorSummary(ra.GrantorCounts)}
			if showActivity {
				accesses, last := "0", "never"
				if ra.Activity != nil {
					accesses = fmt.Sprintf("%d", ra.Activity.Count)
					if ra.Activity.LastAccess != nil {
						last = ra.Activity.LastAccess.Format(time.RFC3339)
					}
				}
				row = append(row, accesses, last)
			}
			rows = append(rows, row)
		}
	}

	return writeAccessTable(out, headers, rows)
}

const resourceColumn = "Resource"

const resourceColumnFloor = 16

func writeAccessTable(out io.Writer, headers []string, rows [][]string) error {
	table := buildAccessTable(headers, rows, terminalWidth(out))
	_, err := fmt.Fprintln(out, table.String())
	return trace.Wrap(err)
}

// buildAccessTable caps only the Resource column to the space the others leave,
// so wide columns are never clipped just to fit the table width. The cap ignores
// the "..." asciitable appends on truncation, so a clipped Resource can overflow
// width by a few columns; the terminal wraps it.
func buildAccessTable(headers []string, rows [][]string, width int) asciitable.Table {
	used := 0
	for i, h := range headers {
		if h == resourceColumn {
			continue
		}
		colWidth := len(h)
		for _, row := range rows {
			if i < len(row) && len(row[i]) > colWidth {
				colWidth = len(row[i])
			}
		}
		used += colWidth + 1 // +1 for tabwriter's column padding
	}

	t := asciitable.MakeTable([]string{})
	for _, h := range headers {
		col := asciitable.Column{Title: h}
		if h == resourceColumn {
			col.MaxCellLength = max(width-used, resourceColumnFloor)
		}
		t.AddColumn(col)
	}
	for _, row := range rows {
		t.AddRow(row)
	}
	return t
}

func writeWarnings(out io.Writer, warnings []string) error {
	for _, w := range warnings {
		if _, err := fmt.Fprintln(out, warningStyle.Render("Warning: "+w)); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// cellName renders a node's display label (alias, else name, else id), escaped.
func cellName(n node) string {
	name := n.Name
	if n.Alias != "" {
		name = n.Alias
	}
	if name == "" {
		name = n.ID
	}
	return utils.EscapeControl(name)
}

// cellKind renders a node's sub-kind (e.g. user, bot, ssh).
func cellKind(n node) string {
	return utils.EscapeControl(n.SubKind)
}

// grantorName renders a grantor's display label, marking temporary grantors.
func grantorName(g grantor) string {
	name := cellName(g.Node)
	if g.Node.Temporary {
		name += "*"
	}
	return name
}
