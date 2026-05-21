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
	_ "embed"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"golang.org/x/term"

	"github.com/gravitational/teleport"
	accessgraph "github.com/gravitational/teleport/lib/accessgraph/apiclient"
	logmodels "github.com/gravitational/teleport/lib/accessgraph/apiclient/models/logs"
)

// investigateSkill is the Markdown blob `--skill` prints, written for an
// LLM agent that needs to learn the command from a single read.
//
//go:embed skills/investigate.md
var investigateSkill string

// investigateArgs holds the parsed flag values for `tctl investigate`. The
// include/exclude slices mirror the filter fields exposed by the Identity
// Security investigate UI; see filterFields for the mapping to Lucene fields.
type investigateArgs struct {
	cmd *kingpin.CmdClause

	from   time.Time
	to     time.Time
	limit  int
	order  string
	format string

	// Include filters
	includeIdentity        []string
	includeUserKind        []string
	includeEventType       []string
	includeResource        []string
	includeResourceKind    []string
	includeIP              []string
	includeStatus          []string
	includeSource          []string
	includeCountry         []string
	includeCity            []string
	includeRegion          []string
	includeAWSAccountID    []string
	includeAWSService      []string
	includeGitHubOrg       []string
	includeGitHubRepo      []string
	includeOktaOrg         []string
	includeTeleportCluster []string
	includeToken           []string
	includeUserAgent       []string

	// Exclude filters
	excludeIdentity        []string
	excludeUserKind        []string
	excludeEventType       []string
	excludeResource        []string
	excludeResourceKind    []string
	excludeIP              []string
	excludeStatus          []string
	excludeSource          []string
	excludeCountry         []string
	excludeCity            []string
	excludeRegion          []string
	excludeAWSAccountID    []string
	excludeAWSService      []string
	excludeGitHubOrg       []string
	excludeGitHubRepo      []string
	excludeOktaOrg         []string
	excludeTeleportCluster []string
	excludeToken           []string
	excludeUserAgent       []string

	// Raw query
	rawQuery string

	// Geo filters
	latitude  *float32
	longitude *float32
	radius    *float32

	// General flags
	printQuery    bool
	allFacets     bool
	showUnmatched bool
	facetsOnly    bool
	skill         bool
}

// facetTextTopN is the default number of facet values to show in text output
const facetTextTopN = 5

// filterField describes one structured filter exposed by the CLI
type filterField struct {
	flag       string
	lucene     string
	help       string
	include    *[]string
	exclude    *[]string
	enumValues []string
}

// filterFields enumerates every structured filter the CLI exposes.
func (a *investigateArgs) filterFields() []filterField {
	return []filterField{
		{flag: "aws-account-id", lucene: "aws_account_id", help: "Filter by AWS account ID (repeatable).", include: &a.includeAWSAccountID, exclude: &a.excludeAWSAccountID},
		{flag: "aws-service", lucene: "aws_service", help: "Filter by AWS service name (repeatable).", include: &a.includeAWSService, exclude: &a.excludeAWSService},
		{flag: "city", lucene: "city", help: "Filter by city of origin (repeatable).", include: &a.includeCity, exclude: &a.excludeCity},
		{flag: "country", lucene: "country", help: "Filter by country of origin (repeatable).", include: &a.includeCountry, exclude: &a.excludeCountry},
		{flag: "event-type", lucene: "event_type", help: "Filter by event type, e.g. session.start (repeatable).", include: &a.includeEventType, exclude: &a.excludeEventType},
		{flag: "github-org", lucene: "github_organization", help: "Filter by GitHub organization (repeatable).", include: &a.includeGitHubOrg, exclude: &a.excludeGitHubOrg},
		{flag: "github-repo", lucene: "github_repo", help: "Filter by GitHub repository (repeatable).", include: &a.includeGitHubRepo, exclude: &a.excludeGitHubRepo},
		// We are using `--user` instead of `--identity` since the latter is already registered at the top level in tctl.
		{flag: "user", lucene: "identity_id", help: "Filter by user (email for users, ID for bots; repeatable).", include: &a.includeIdentity, exclude: &a.excludeIdentity},
		{flag: "user-kind", lucene: "identity_kind", help: "Filter by user kind, e.g. user, system (repeatable).", include: &a.includeUserKind, exclude: &a.excludeUserKind},
		{flag: "ip", lucene: "ip", help: "Filter by source IP address (repeatable).", include: &a.includeIP, exclude: &a.excludeIP},
		{flag: "okta-org", lucene: "okta_org", help: "Filter by Okta organization (repeatable).", include: &a.includeOktaOrg, exclude: &a.excludeOktaOrg},
		{flag: "region", lucene: "region", help: "Filter by region (e.g. us-east-1 or a US state code; repeatable).", include: &a.includeRegion, exclude: &a.excludeRegion},
		// Lucene names match the Athena column names directly so facet responses (which carry the Athena names) line up with our flags
		{flag: "resource", lucene: "target_resource", help: "Filter by target resource (repeatable).", include: &a.includeResource, exclude: &a.excludeResource},
		{flag: "resource-kind", lucene: "target_kind", help: "Filter by resource kind, e.g. ssh, kube, session_recording (repeatable).", include: &a.includeResourceKind, exclude: &a.excludeResourceKind},
		{flag: "source", lucene: "event_source", help: "Filter by event source (repeatable).", include: &a.includeSource, exclude: &a.excludeSource, enumValues: []string{"aws", "github", "okta", "teleport"}},
		{flag: "status", lucene: "status", help: "Filter by event status (repeatable).", include: &a.includeStatus, exclude: &a.excludeStatus, enumValues: []string{"success", "error"}},
		{flag: "teleport-cluster", lucene: "teleport_cluster", help: "Filter by Teleport cluster name (repeatable).", include: &a.includeTeleportCluster, exclude: &a.excludeTeleportCluster},
		{flag: "token", lucene: "token", help: "Filter by token identifier (repeatable).", include: &a.includeToken, exclude: &a.excludeToken},
		{flag: "user-agent", lucene: "user_agent", help: "Filter by user agent string (repeatable). Not populated on every Teleport event — use deliberately.", include: &a.includeUserAgent, exclude: &a.excludeUserAgent},
	}
}

// initInvestigate registers `tctl investigate` and all its flags.
func (c *AccessGraphCommand) initInvestigate(app *kingpin.Application) {
	cmd := app.Command("investigate", "Investigate identity and resource activity.")

	cmd.Flag("from", fmt.Sprintf("Include activity at or after this time. (Examples: %s, %s, 24h, 7d; negative durations like -1h are future-relative. Default: 1d)", time.RFC3339, time.DateOnly)).
		Default("1d").
		SetValue(timeValue{target: &c.investigate.from})
	cmd.Flag("to", fmt.Sprintf("Include activity at or before this time. (Examples: %s, %s, 24h, 7d; negative durations like -1h are future-relative. Default: now)", time.RFC3339, time.DateOnly)).
		Default("now").
		SetValue(timeValue{target: &c.investigate.to})

	cmd.Flag("limit", "Maximum number of events to return (0 for unlimited).").
		Default("100").
		IntVar(&c.investigate.limit)
	cmd.Flag("order", "Result order by timestamp. (Values: asc, desc)").
		Default(string(accessgraph.Desc)).
		EnumVar(&c.investigate.order, string(accessgraph.Asc), string(accessgraph.Desc))
	cmd.Flag("format", "Output format. (Values: text, json, yaml)").
		Default(teleport.Text).
		EnumVar(&c.investigate.format, teleport.Text, teleport.JSON, teleport.YAML)
	cmd.Flag("all-facets", fmt.Sprintf("Show every facet value in text output. Without this flag, each facet is truncated to the top %d values by count. Has no effect on JSON/YAML output.", facetTextTopN)).
		BoolVar(&c.investigate.allFacets)
	cmd.Flag("show-unmatched", "Include facet values that exist in the time window but did not match the current filter (the backend reports these with count=-1). Useful for discovering filters to broaden.").
		BoolVar(&c.investigate.showUnmatched)
	cmd.Flag("facets-only", "Skip fetching events; return only the facet summary. Useful for narrowing a query before pulling logs. Implies --all-facets in text output.").
		BoolVar(&c.investigate.facetsOnly)
	cmd.Flag("skill", "Print a Markdown skill describing how an LLM agent should use this command, then exit. All other flags are ignored.").
		BoolVar(&c.investigate.skill)

	for _, f := range c.investigate.filterFields() {
		include := cmd.Flag(f.flag, f.help)
		exclude := cmd.Flag("exclude-"+f.flag, "Exclude "+f.lucene+" values (repeatable).")
		if len(f.enumValues) > 0 {
			include.EnumsVar(f.include, f.enumValues...)
			exclude.EnumsVar(f.exclude, f.enumValues...)
		} else {
			include.StringsVar(f.include)
			exclude.StringsVar(f.exclude)
		}
	}

	cmd.Flag("query", `Raw Lucene query. Mutually exclusive with structured filter flags. Example: --query 'identity_id:"alice@example.com" AND NOT status:"error"'`).
		StringVar(&c.investigate.rawQuery)
	cmd.Flag("print-query", "Print the constructed query and exit without contacting the backend.").
		BoolVar(&c.investigate.printQuery)

	cmd.Flag("latitude", "Center latitude for geo-filtered search (decimal degrees). Requires --longitude and --radius.").
		SetValue(optionalFloat32{target: &c.investigate.latitude})
	cmd.Flag("longitude", "Center longitude for geo-filtered search (decimal degrees). Requires --latitude and --radius.").
		SetValue(optionalFloat32{target: &c.investigate.longitude})
	cmd.Flag("radius", "Radius in kilometers around the geo center. Requires --latitude and --longitude.").
		SetValue(optionalFloat32{target: &c.investigate.radius})

	c.investigate.cmd = cmd
}

// Investigate executes `tctl investigate`.
func (c *AccessGraphCommand) Investigate(ctx context.Context, client *accessgraph.ClientWithResponses) error {
	args := &c.investigate

	// --skill short-circuits everything: print the agent skill and exit
	// without touching the backend or any other flag.
	if args.skill {
		_, err := io.WriteString(c.stdout, investigateSkill)
		return trace.Wrap(err)
	}

	// Check raw-vs-structured conflict before --print-query: it shapes the query.
	if err := args.validateRawQueryExclusive(); err != nil {
		return trace.Wrap(err)
	}

	query := args.buildQuery()

	if args.printQuery {
		_, err := fmt.Fprintln(c.stdout, query)
		return trace.Wrap(err)
	}

	if err := validateTimeWindow(args.from, args.to); err != nil {
		return trace.Wrap(err)
	}
	if err := args.validateGeo(); err != nil {
		return trace.Wrap(err)
	}

	// Normalize to UTC before sending to the backend
	// non-UTC time shifts the stats window relative to the logs window
	fromUTC := args.from.UTC()
	toUTC := args.to.UTC()

	order := accessgraph.ExecuteLogsQueryV1ParamsOrder(args.order)
	params := accessgraph.ExecuteLogsQueryV1Params{
		StartTime: &fromUTC,
		EndTime:   &toUTC,
		Order:     &order,
		Latitude:  args.latitude,
		Longitude: args.longitude,
		Radius:    args.radius,
	}
	if query != "" {
		params.Query = &query
	}
	if args.limit > 0 {
		params.Limit = &args.limit
	}

	// Facets and events have no inter-dependency, so fetch them in parallel.
	var (
		facets    []logsFacet
		total     int64
		events    []logmodels.AccessgraphStorageV1alphaEvent
		truncated bool
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		facets, total, err = fetchLogsFacets(gctx, client, accessgraph.ExecuteLogsStatsQueryV1Params{
			StartTime: &fromUTC,
			EndTime:   &toUTC,
			Query:     params.Query,
		}, args.facetNames())
		return trace.Wrap(err)
	})
	if !args.facetsOnly {
		g.Go(func() error {
			var err error
			events, truncated, err = fetchAllLogs(gctx, client, params, args.limit)
			return trace.Wrap(err)
		})
	}
	if err := g.Wait(); err != nil {
		return trace.Wrap(err)
	}

	if !args.showUnmatched {
		facets = stripUnmatchedFacets(facets)
	}

	// Always emit a non-nil slice so JSON renders "data": [] rather than null
	if events == nil {
		events = []logmodels.AccessgraphStorageV1alphaEvent{}
	}

	output := investigateOutput{
		Total:  total,
		Facets: facets,
		Data:   events,
	}

	// --facets-only implies --all-facets in text output
	allFacets := args.allFacets || args.facetsOnly
	// Geo applies to events only; stats has no geo params.
	geoActive := args.latitude != nil || args.longitude != nil || args.radius != nil

	return writeOutput(c.stdout, output, args.format, func(w io.Writer) error {
		return displayInvestigateText(w, output, args.from, args.to, args.limit, truncated, allFacets, args.facetsOnly, geoActive)
	})
}

// stripUnmatchedFacets drops facet values with count=-1 (not in filtered response)
func stripUnmatchedFacets(facets []logsFacet) []logsFacet {
	out := make([]logsFacet, 0, len(facets))
	for _, f := range facets {
		var kept []logsFacetValue
		for _, v := range f.Values {
			if v.Count < 0 {
				continue
			}
			kept = append(kept, v)
		}
		if len(kept) == 0 {
			continue
		}
		out = append(out, logsFacet{Name: f.Name, Values: kept})
	}
	return out
}

// investigateOutput is the top-level shape returned by Investigate,
type investigateOutput struct {
	// Total number of events matching the filter, derived from the event_type facet.
	Total int64 `json:"total" yaml:"total"`
	// Facets is the list of filters that matched the query (unlimited), to be used for further filtering
	Facets []logsFacet `json:"facets" yaml:"facets"`
	// Data is the list of events matching the query, subject to --limit truncation
	Data []logmodels.AccessgraphStorageV1alphaEvent `json:"data" yaml:"data"`
}

// logsFacet is one column of the stats response that we render as a facet.
type logsFacet struct {
	Name   string           `json:"name" yaml:"name"`
	Values []logsFacetValue `json:"values" yaml:"values"`
}

// logsFacetValue is one bucket in a facet.
type logsFacetValue struct {
	Value string `json:"value" yaml:"value"`
	Count int64  `json:"count" yaml:"count"`
}

// facetNames returns a map from Lucene field name to flag name, so we can rename to match the CLI flags
func (a *investigateArgs) facetNames() map[string]string {
	fields := a.filterFields()
	names := make(map[string]string, len(fields))
	for _, f := range fields {
		names[f.lucene] = f.flag
	}
	return names
}

// fetchLogsFacets calls ExecuteLogsStatsQueryV1 and transforms the response into a list of logsFacets
func fetchLogsFacets(ctx context.Context, client *accessgraph.ClientWithResponses, params accessgraph.ExecuteLogsStatsQueryV1Params, facetNames map[string]string) ([]logsFacet, int64, error) {
	resp, err := doRequest(client.ExecuteLogsStatsQueryV1WithResponse(ctx, &params))
	if err != nil {
		return nil, 0, trace.Wrap(err)
	}
	var total int64
	byFlag := make(map[string][]logsFacetValue, len(resp.JSON200.Data))
	for _, column := range resp.JSON200.Data {
		if len(column.Values) == 0 {
			continue
		}
		if column.ColumnName == "event_type" {
			for _, v := range column.Values {
				if v.Count > 0 {
					total += v.Count
				}
			}
		}
		flag, ok := facetNames[column.ColumnName]
		if !ok {
			continue
		}
		values := make([]logsFacetValue, len(column.Values))
		for i, v := range column.Values {
			values[i] = logsFacetValue{Value: v.Value, Count: v.Count}
		}
		sort.SliceStable(values, func(i, j int) bool {
			return values[i].Count > values[j].Count
		})
		byFlag[flag] = values
	}
	flags := make([]string, 0, len(byFlag))
	for flag := range byFlag {
		flags = append(flags, flag)
	}
	sort.Strings(flags)
	facets := make([]logsFacet, 0, len(flags))
	for _, flag := range flags {
		facets = append(facets, logsFacet{Name: flag, Values: byFlag[flag]})
	}
	return facets, total, nil
}

// buildQuery returns either the raw --query value or a query assembled from structured filters
func (a *investigateArgs) buildQuery() string {
	if a.rawQuery != "" {
		return a.rawQuery
	}
	var parts []string
	for _, f := range a.filterFields() {
		if clause := dslClause(f.lucene, *f.include); clause != "" {
			parts = append(parts, clause)
		}
		if clause := dslClause(f.lucene, *f.exclude); clause != "" {
			parts = append(parts, "NOT "+clause)
		}
	}
	return strings.Join(parts, " AND ")
}

// validateRawQueryExclusive rejects combinations of --query with any structured filter
func (a *investigateArgs) validateRawQueryExclusive() error {
	if a.rawQuery == "" {
		return nil
	}
	var offenders []string
	for _, f := range a.filterFields() {
		if len(*f.include) > 0 {
			offenders = append(offenders, "--"+f.flag)
		}
		if len(*f.exclude) > 0 {
			offenders = append(offenders, "--exclude-"+f.flag)
		}
	}
	if len(offenders) == 0 {
		return nil
	}
	sort.Strings(offenders)
	return trace.BadParameter("--query is mutually exclusive with structured filter flags; remove: %s", strings.Join(offenders, ", "))
}

// validateGeo rejects partial geo specs; AG needs all three to apply a geo filter.
func (a *investigateArgs) validateGeo() error {
	set := 0
	if a.latitude != nil {
		set++
	}
	if a.longitude != nil {
		set++
	}
	if a.radius != nil {
		set++
	}
	if set == 0 || set == 3 {
		return nil
	}
	var missing []string
	if a.latitude == nil {
		missing = append(missing, "--latitude")
	}
	if a.longitude == nil {
		missing = append(missing, "--longitude")
	}
	if a.radius == nil {
		missing = append(missing, "--radius")
	}
	return trace.BadParameter("geo filter requires all of --latitude, --longitude, and --radius; missing: %s", strings.Join(missing, ", "))
}

// displayInvestigateText renders the period header, the facet panel and the events table
func displayInvestigateText(out io.Writer, output investigateOutput, from, to time.Time, limit int, truncated, allFacets, facetsOnly, geoActive bool) error {
	if _, err := fmt.Fprintf(out, "Period: %s → %s\n", from.Format(time.RFC3339), to.Format(time.RFC3339)); err != nil {
		return trace.Wrap(err)
	}
	// "~" prefix because the total is derived from the stats endpoint, which
	// can drift from the logs count
	matches := fmt.Sprintf("Matches: ~%d", output.Total)
	if !facetsOnly {
		matches += fmt.Sprintf(" (showing %d)", len(output.Data))
	}
	if _, err := fmt.Fprintf(out, "%s\n", matches); err != nil {
		return trace.Wrap(err)
	}
	if geoActive {
		if _, err := fmt.Fprintln(out, "Note: geo filter applies to events only; total and facet counts cover the window+query without geo."); err != nil {
			return trace.Wrap(err)
		}
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return trace.Wrap(err)
	}
	if err := displayFacetsText(out, output.Facets, allFacets); err != nil {
		return trace.Wrap(err)
	}
	if facetsOnly {
		return nil
	}
	if err := displayEventsText(out, output.Data); err != nil {
		return trace.Wrap(err)
	}
	if truncated && limit > 0 {
		if _, err := fmt.Fprintf(out, "Results truncated at %d; re-run with --limit <larger> for more.\n", limit); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// displayFacetsText prints one block per non-empty facet column in the form
//
//	name (top N of M): value (count), value (count),
//	                   value (count), ...
//
// Values are truncated to facetTextTopN unless allFacets is set. Unmatched
// values (count == -1) are filtered upstream in Investigate, so by the time
// they reach this function the caller has already chosen what to render.
// When values are wrapped to a second line, the continuation lines align
// under the first value.
func displayFacetsText(out io.Writer, facets []logsFacet, allFacets bool) error {
	if len(facets) == 0 {
		if _, err := fmt.Fprintln(out, "Facets: none"); err != nil {
			return trace.Wrap(err)
		}
		_, err := fmt.Fprintln(out)
		return trace.Wrap(err)
	}

	width := facetWrapWidth(out)
	if _, err := fmt.Fprintln(out, "Facets:"); err != nil {
		return trace.Wrap(err)
	}

	// First pass: build (header, values) for each facet, and find the
	// longest header so we can align every facet's values to the same column
	// on emit.
	type row struct {
		header string
		parts  []string
	}
	rows := make([]row, 0, len(facets))
	maxHeader := 0
	for _, f := range facets {
		values := f.Values
		total := len(values)
		if total == 0 {
			continue
		}
		truncated := !allFacets && total > facetTextTopN
		if truncated {
			values = values[:facetTextTopN]
		}

		header := fmt.Sprintf("%s (%d)", f.Name, total)
		if truncated {
			header = fmt.Sprintf("%s (top %d of %d)", f.Name, facetTextTopN, total)
		}

		parts := make([]string, len(values))
		for i, v := range values {
			parts[i] = fmt.Sprintf("%s (%d)", v.Value, v.Count)
		}
		rows = append(rows, row{header: header, parts: parts})
		if len(header) > maxHeader {
			maxHeader = len(header)
		}
	}

	for _, r := range rows {
		padding := strings.Repeat(" ", maxHeader-len(r.header))
		prefix := "  " + r.header + ":" + padding + "  "
		if err := writeWrappedList(out, prefix, r.parts, width); err != nil {
			return trace.Wrap(err)
		}
	}
	_, err := fmt.Fprintln(out)
	return trace.Wrap(err)
}

// facetWrapWidth returns the column count to wrap facet values at.
func facetWrapWidth(out io.Writer) int {
	f, ok := out.(*os.File)
	if !ok {
		return 80
	}
	width, _, err := term.GetSize(int(f.Fd()))
	if err != nil || width <= 0 {
		return 80
	}
	return width
}

// writeWrappedList prints "prefix" followed by items joined with ", ",
// wrapping at width with a hanging indent equal to the prefix length so
// continuation lines align under the first item.
func writeWrappedList(out io.Writer, prefix string, items []string, width int) error {
	indent := strings.Repeat(" ", len(prefix))
	var line strings.Builder
	line.WriteString(prefix)
	line.WriteString(items[0])
	for _, item := range items[1:] {
		// "+2" accounts for the ", " separator we'd add before this item.
		if line.Len()+2+len(item) > width {
			if _, err := fmt.Fprintln(out, line.String()+","); err != nil {
				return trace.Wrap(err)
			}
			line.Reset()
			line.WriteString(indent)
			line.WriteString(item)
			continue
		}
		line.WriteString(", ")
		line.WriteString(item)
	}
	_, err := fmt.Fprintln(out, line.String())
	return trace.Wrap(err)
}

