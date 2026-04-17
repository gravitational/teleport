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
	"slices"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	accessgraph "github.com/gravitational/teleport/lib/accessgraph/apiclient"
	models "github.com/gravitational/teleport/lib/accessgraph/apiclient/models/graph"
	logmodels "github.com/gravitational/teleport/lib/accessgraph/apiclient/models/logs"
	"github.com/gravitational/teleport/lib/asciitable"
	utilslices "github.com/gravitational/teleport/lib/utils/slices"
	"github.com/gravitational/trace"
	"github.com/oapi-codegen/runtime/types"
)

// accessReviewResourceArgs holds arguments for `tctl access review resource`.
type accessReviewResourceArgs struct {
	cmd  *kingpin.CmdClause
	name string
}

// accessReviewACLArgs holds arguments for `tctl access review acl`.
type accessReviewACLArgs struct {
	cmd  *kingpin.CmdClause
	name string
}

// accessReviewRoleArgs holds arguments for `tctl access review role`.
type accessReviewRoleArgs struct {
	cmd  *kingpin.CmdClause
	name string
}

// initAccessReview registers `tctl access review` and its subcommands.
func (c *AccessGraphCommand) initAccessReview(parent *kingpin.CmdClause) {
	reviewCmd := parent.Command("review", "Review which users accessed a resource, ACL, or role over a given period.")
	registerTimeRangeFlags(reviewCmd, &c.access.review.from, &c.access.review.to, "30d")
	reviewCmd.Flag("unused", "Show only users who had no access in the given period.").
		BoolVar(&c.access.review.unused)
	reviewCmd.Flag("detailed", "Show per-resource access breakdown for each user.").
		BoolVar(&c.access.review.detailed)
	reviewCmd.Flag("user", "Limit results to specific users (repeatable, e.g. --user alice --user bob).").
		StringsVar(&c.access.review.users)

	c.access.review.cmd = reviewCmd

	c.initAccessReviewResource(reviewCmd)
	c.initAccessReviewACL(reviewCmd)
	c.initAccessReviewRole(reviewCmd)
}

func (c *AccessGraphCommand) initAccessReviewResource(parent *kingpin.CmdClause) {
	cmd := parent.Command("resource", "Review users who accessed a resource.")
	cmd.Arg("name", "Name or ID of the resource to review.").Required().StringVar(&c.access.review.resource.name)
	c.access.review.resource.cmd = cmd
}

func (c *AccessGraphCommand) initAccessReviewACL(parent *kingpin.CmdClause) {
	cmd := parent.Command("acl", "Review users who accessed resources governed by an ACL.")
	cmd.Arg("name", "Name or ID of the ACL to review.").Required().StringVar(&c.access.review.acl.name)
	c.access.review.acl.cmd = cmd
}

func (c *AccessGraphCommand) initAccessReviewRole(parent *kingpin.CmdClause) {
	cmd := parent.Command("role", "Review users who accessed a role.")
	cmd.Arg("name", "Name of the role to review.").Required().StringVar(&c.access.review.role.name)
	c.access.review.role.cmd = cmd
}

// --- output types -----------------------------------------------------------

// ReviewedResource is a resolved access graph node shown in review output.
type ReviewedResource struct {
	ID      string `json:"id" yaml:"id"`
	Name    string `json:"name" yaml:"name"`
	Alias   string `json:"alias,omitempty" yaml:"alias,omitempty"`
	SubKind string `json:"sub_kind" yaml:"sub_kind"`
}

// ResourceAccessEntry holds per-resource activity for a single identity (--detailed).
type ResourceAccessEntry struct {
	ResourceName string     `json:"resource_name" yaml:"resource_name"`
	AccessCount  int        `json:"access_count" yaml:"access_count"`
	LastAccess   *time.Time `json:"last_access" yaml:"last_access"`
}

// IdentityAccessResult is one row in the review output.
type IdentityAccessResult struct {
	IdentityName     string                `json:"identity_name" yaml:"identity_name"`
	IdentityKind     string                `json:"identity_kind" yaml:"identity_kind"`
	Source           string                `json:"source" yaml:"source"`
	AccessCount      int                   `json:"access_count" yaml:"access_count"`
	LastAccess       *time.Time            `json:"last_access" yaml:"last_access"`
	ResourceActivity []ResourceAccessEntry `json:"resource_activity,omitempty" yaml:"resource_activity,omitempty"`
	// AlternativeAccessResources lists subject resources (by label) the identity
	// can reach via standing access paths that bypass the subjects being
	// reviewed. Populated only for acl/role review.
	AlternativeAccessResources []string `json:"alternative_access_resources,omitempty" yaml:"alternative_access_resources,omitempty"`
}

// ReviewOutput is the unified response payload for all review subcommands.
// SubjectType is "resource", "acl", or "role". Subjects are the top-level
// nodes matched by the search (e.g. one or more ACL nodes). Resources are the
// actual resource nodes in scope — equal to Subjects for resource review,
// the governed resources for acl/role review.
type ReviewOutput struct {
	SubjectType string                 `json:"subject_type" yaml:"subject_type"`
	Subjects    []ReviewedResource     `json:"subjects" yaml:"subjects"`
	Resources   []ReviewedResource     `json:"resources" yaml:"resources"`
	Identities  []IdentityAccessResult `json:"identities" yaml:"identities"`
	Warnings    []string               `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

// --- command handlers -------------------------------------------------------

// AccessReviewResource executes `tctl access review resource <name>`.
func (c *AccessGraphCommand) AccessReviewResource(ctx context.Context, args accessGraphServices) error {
	search := c.access.review.resource.name
	subjects, err := resolveSubjectNodes(ctx, args, fmt.Sprintf(
		"SELECT * FROM nodes WHERE (name ILIKE '%%%s%%' OR properties->>'alias' ILIKE '%%%s%%') AND kind = 'resource'",
		search, search,
	))
	if err != nil {
		return trace.Wrap(err)
	}
	if len(subjects) == 0 {
		return trace.NotFound("resource %q not found in access graph", search)
	}

	// Single graph covering all subjects.
	pathNodes, pathEdges, err := queryAccessPath(ctx, args, subjects)
	if err != nil {
		return trace.Wrap(err)
	}
	g := newTraversalGraph(pathNodes, pathEdges)

	// For resources, the subjects ARE the resources; identities come from
	// standing-access traversal from each subject.
	resourceMap := make(map[string]*models.Node)
	identityMap := make(map[string]*models.Node)
	for i := range subjects {
		subject := &subjects[i]
		resourceMap[subject.Id.String()] = subject
		for _, id := range g.GetIdentityNodesWithAccess(*subject) {
			identityMap[id.Id.String()] = id
		}
	}
	return c.runReview(ctx, args, "resource", subjects, resourceMap, identityMap)
}

// AccessReviewACL executes `tctl access review acl <name>`.
func (c *AccessGraphCommand) AccessReviewACL(ctx context.Context, args accessGraphServices) error {
	search := c.access.review.acl.name
	subjects, err := resolveSubjectNodes(ctx, args, fmt.Sprintf(
		"SELECT * FROM nodes WHERE (name ILIKE '%%%s%%' OR properties->>'alias' ILIKE '%%%s%%') AND kind = 'identity_group' AND sub_kind = 'access_list'",
		search, search,
	))
	if err != nil {
		return trace.Wrap(err)
	}
	if len(subjects) == 0 {
		return trace.NotFound("ACL %q not found in access graph", search)
	}
	return c.reviewIdentityGroup(ctx, args, "acl", subjects)
}

// AccessReviewRole executes `tctl access review role <name>`.
func (c *AccessGraphCommand) AccessReviewRole(ctx context.Context, args accessGraphServices) error {
	search := c.access.review.role.name
	subjects, err := resolveSubjectNodes(ctx, args, fmt.Sprintf(
		"SELECT * FROM nodes WHERE (name ILIKE '%%%s%%' OR properties->>'alias' ILIKE '%%%s%%') AND kind = 'identity_group' AND sub_kind = 'role'",
		search, search,
	))
	if err != nil {
		return trace.Wrap(err)
	}
	if len(subjects) == 0 {
		return trace.NotFound("role %q not found in access graph", search)
	}
	return c.reviewIdentityGroup(ctx, args, "role", subjects)
}

// reviewIdentityGroup implements the review flow for identity-group subjects
// (ACLs and roles). Subjects grant access to governed resources via outgoing
// edges; members/grantees are found via incoming traversal. A single combined
// access-path query is used so cross-subject edges (e.g. owner_of between
// related nodes) remain intact.
func (c *AccessGraphCommand) reviewIdentityGroup(
	ctx context.Context,
	args accessGraphServices,
	subjectType string,
	subjects []models.Node,
) error {
	for _, s := range subjects {
		slog.DebugContext(ctx, "resolved subject", "type", subjectType, "id", s.Id, "name", s.Name, "sub_kind", s.SubKind)
	}

	pathNodes, pathEdges, err := queryAccessPath(ctx, args, subjects)
	if err != nil {
		return trace.Wrap(err)
	}
	g := newTraversalGraph(pathNodes, pathEdges)

	// For each subject:
	//   - resources: outgoing traversal, skipping denied/reviewer/temporary paths.
	//   - identities: GetIdentityNodesWithAccess skips owner_of + blocked paths.
	resourceMap := make(map[string]*models.Node)
	identityMap := make(map[string]*models.Node)
	for i := range subjects {
		subject := &subjects[i]
		g.visitOutgoing(subject.Id, func(edge *EdgeWithTarget) bool {
			if isBlocking(edge) {
				return false
			}
			node := edge.Target
			if node.Kind == "resource" {
				resourceMap[node.Id.String()] = node
			}
			return true
		})
		for _, id := range g.GetIdentityNodesWithAccess(*subject) {
			identityMap[id.Id.String()] = id
		}
	}
	return c.runReview(ctx, args, subjectType, subjects, resourceMap, identityMap)
}

// --- core review logic ------------------------------------------------------

// resolveSubjectNodes executes query against the graph API and returns all
// matched nodes. Multiple matches are returned without error — callers decide
// how to handle them.
func resolveSubjectNodes(ctx context.Context, args accessGraphServices, query string) ([]models.Node, error) {
	slog.DebugContext(ctx, "resolving subject nodes", "query", query)
	rsp, err := doRequest(args.accessGraph.ExecuteQueryV1WithResponse(ctx, &accessgraph.ExecuteQueryV1Params{
		Query: query,
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if rsp.JSON200 == nil || rsp.JSON200.Nodes == nil {
		return nil, nil
	}
	return *rsp.JSON200.Nodes, nil
}

// queryAccessPath fetches a single access-path subgraph covering all provided
// subject nodes (OR'd by id). Running one combined query instead of one per
// subject keeps cross-subject edges (e.g. owner_of between related ACL nodes)
// intact in the resulting graph.
func queryAccessPath(ctx context.Context, args accessGraphServices, subjects []models.Node) (*[]models.Node, *[]models.Edge, error) {
	if len(subjects) == 0 {
		empty := []models.Node{}
		emptyEdges := []models.Edge{}
		return &empty, &emptyEdges, nil
	}
	t := time.Now()

	ids := utilslices.Map(subjects, func(n models.Node) string { return n.Id.String() })
	conditions := utilslices.Map(ids, func(id string) string { return fmt.Sprintf("id = '%s'", id) })
	query := fmt.Sprintf("SELECT * FROM access_path WHERE %s", strings.Join(conditions, " OR "))
	slog.DebugContext(ctx, "access_path query", "query", query)

	rsp, err := doRequest(args.accessGraph.ExecuteQueryV1WithResponse(ctx, &accessgraph.ExecuteQueryV1Params{
		Query: query,
	}))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	slog.DebugContext(ctx, "resolved access path",
		"node_ids", ids,
		"elapsed", time.Since(t),
		"nodes", len(*rsp.JSON200.Nodes),
		"edges", len(*rsp.JSON200.Edges),
	)
	return rsp.JSON200.Nodes, rsp.JSON200.Edges, nil
}

// runReview is the shared implementation for all review subcommands.
// Callers are responsible for computing resourceMap and identityMap using the
// traversal strategy appropriate for their subject type:
//   - resource: traverse from each resource node (GetIdentityNodesWithAccess)
//   - acl/role: traverse from the subject node itself to get members only
//
// subjectType ("resource", "acl", "role") controls display headers only.
func (c *AccessGraphCommand) runReview(
	ctx context.Context,
	args accessGraphServices,
	subjectType string,
	subjects []models.Node,
	resourceMap map[string]*models.Node,
	identityMap map[string]*models.Node,
) error {
	cmdStart := time.Now()
	slog.DebugContext(ctx, "starting review",
		"subject_type", subjectType,
		"subject_count", len(subjects),
		"resources", len(resourceMap),
		"identities", len(identityMap),
	)

	resourceNodes := make([]*models.Node, 0, len(resourceMap))
	for _, r := range resourceMap {
		resourceNodes = append(resourceNodes, r)
	}
	if len(resourceNodes) == 0 {
		fmt.Fprintf(c.stdout, "No resources found for the given %s.\n", subjectType)
		return nil
	}

	identities := make([]*models.Node, 0, len(identityMap))
	for _, id := range identityMap {
		identities = append(identities, id)
	}
	slog.DebugContext(ctx, "resolved identities with access",
		"resources", len(resourceNodes),
		"identities", len(identities),
	)
	if len(identities) == 0 {
		switch subjectType {
		case "resource":
			fmt.Fprintln(c.stdout, "No identities have standing access to this resource.")
		default:
			fmt.Fprintf(c.stdout, "The %s grants access to %d resource(s) but has no standing members.\n", subjectType, len(resourceNodes))
		}
		return nil
	}

	// Apply --user filter.
	var (
		warnings []string
		err      error
	)
	identities, warnings, err = applyUserFilter(identities, c.access.review.users)
	if err != nil {
		return trace.Wrap(err)
	}

	// Fetch activity. Always build per-resource detail so the summary view can
	// show "N/M resources accessed" even when --detailed is not set.
	t := time.Now()
	activity, err := fetchResourceActivity(ctx, args, resourceNodes, identities, c.access.review.from, c.access.review.to)
	if err != nil {
		return trace.Wrap(err)
	}
	slog.DebugContext(ctx, "fetched activity logs", "elapsed", time.Since(t), "events", len(activity))

	// For acl/role, compute per-identity alternative standing access paths so
	// we can flag identities whose access doesn't require the subject.
	var altAccess map[string]map[string]bool
	if subjectType != "resource" {
		altAccess, err = computeAlternativeAccess(ctx, args, subjects, resourceNodes)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	results := buildIdentityResults(identities, activity, altAccess)

	// Apply --unused filter. A user is "unused" if they didn't access anything
	// in the window, or if every resource they accessed has an alternative
	// standing access path (i.e. they didn't need the subject).
	if c.access.review.unused {
		results = slices.DeleteFunc(results, func(r IdentityAccessResult) bool {
			return r.usedSubject()
		})
	}

	// Sort by most recent access descending; never-accessed sink to the bottom.
	slices.SortFunc(results, compareByLastAccess)

	slog.DebugContext(ctx, "command complete", "total_elapsed", time.Since(cmdStart))

	out := ReviewOutput{
		SubjectType: subjectType,
		Subjects:    utilslices.Map(subjects, func(n models.Node) ReviewedResource { return nodeToReviewedResource(&n) }),
		Resources:   utilslices.Map(resourceNodes, func(n *models.Node) ReviewedResource { return nodeToReviewedResource(n) }),
		Identities:  results,
		Warnings:    warnings,
	}
	return displayReviewOutput(c.stdout, out, c.access.review.from, c.access.review.to, c.access.review.detailed, c.access.format)
}

// --- helpers ----------------------------------------------------------------

// nodeToReviewedResource converts a graph node to a ReviewedResource, resolving
// the alias from the node's properties when available.
func nodeToReviewedResource(n *models.Node) ReviewedResource {
	r := ReviewedResource{
		ID:      n.Id.String(),
		Name:    n.Name,
		SubKind: n.SubKind,
	}
	if props, err := n.Properties.AsResourceProperties(); err == nil && props.Alias != nil {
		r.Alias = *props.Alias
	}
	return r
}

// applyUserFilter limits identities to those named in users. Returns the
// filtered list and any warnings for users that were not found. Returns an
// error only when every requested user was absent.
func applyUserFilter(identities []*models.Node, users []string) ([]*models.Node, []string, error) {
	if len(users) == 0 {
		return identities, nil, nil
	}

	wanted := make(map[string]struct{}, len(users))
	for _, u := range users {
		wanted[u] = struct{}{}
	}

	var filtered []*models.Node
	for _, id := range identities {
		if _, ok := wanted[id.Name]; ok {
			filtered = append(filtered, id)
			delete(wanted, id.Name)
		}
	}

	if len(filtered) == 0 {
		return nil, nil, trace.NotFound(
			"none of the specified users (%s) have standing access",
			strings.Join(users, ", "),
		)
	}

	var warnings []string
	for missing := range wanted {
		warnings = append(warnings, fmt.Sprintf("user %q does not have standing access", missing))
	}
	slices.Sort(warnings)
	return filtered, warnings, nil
}

// fetchResourceActivity queries the logs for session-start activity by the
// given identities within the time window, scoped to the provided resources.
//
// Both identity and resource filters are applied server-side; event types are
// narrowed to the union of types relevant to each resource's sub-kind.
func fetchResourceActivity(
	ctx context.Context,
	args accessGraphServices,
	resources []*models.Node,
	identities []*models.Node,
	from, to time.Time,
) ([]logmodels.AccessgraphStorageV1alphaEvent, error) {
	query := buildSessionActivityQuery(identities, resources)
	slog.DebugContext(ctx, "logs query", "query", query)
	order := accessgraph.Desc
	return fetchAllLogs(ctx, args.accessGraph, accessgraph.ExecuteLogsQueryV1Params{
		Query:     &query,
		StartTime: &from,
		EndTime:   &to,
		Order:     &order,
	})
}

// buildSessionActivityQuery builds a logs DSL query that matches session-start
// events for the given identities and resources. Resource names and event types
// are OR'd so a single query covers all provided resources.
func buildSessionActivityQuery(identities []*models.Node, resources []*models.Node) string {
	identityNames := utilslices.Map(identities, func(id *models.Node) string { return id.Name })
	// Use alias when available as it's the human-readable identifier in events.
	resourceNames := utilslices.Map(resources, func(r *models.Node) string {
		if props, err := r.Properties.AsResourceProperties(); err == nil && props.Alias != nil && *props.Alias != "" {
			return *props.Alias
		}
		return r.Name
	})

	// Union of event types across all resource sub-kinds.
	eventTypeSet := make(map[string]struct{})
	for _, r := range resources {
		for _, et := range sessionEventTypesForSubKind(r.SubKind) {
			eventTypeSet[et] = struct{}{}
		}
	}
	eventTypes := make([]string, 0, len(eventTypeSet))
	for et := range eventTypeSet {
		eventTypes = append(eventTypes, et)
	}
	slices.Sort(eventTypes)

	return fmt.Sprintf("%s AND (%s AND %s)",
		dslClause("identity_id", quoteAll(identityNames)),
		dslClause("resource", quoteAll(resourceNames)),
		dslClause("event_type", quoteAll(eventTypes)),
	)
}

// sessionEventTypesForSubKind returns the access-graph logs event_type values
// that represent session access for the given resource sub-kind. Falls back to
// all four session-start types when the sub-kind is unrecognized.
func sessionEventTypesForSubKind(subKind string) []string {
	switch subKind {
	case "ssh":
		return []string{"session.start"}
	case "kubernetes":
		return []string{"session.start"}
	case "db":
		return []string{"db.session.start"}
	case "app":
		return []string{"app.session.start"}
	case "desktop":
		return []string{"windows.desktop.session.start"}
	default:
		return []string{
			"session.start",
			"db.session.start",
			"app.session.start",
			"windows.desktop.session.start",
		}
	}
}

// buildIdentityResults cross-references standing-access identities with event
// logs, producing one IdentityAccessResult per identity. Per-resource detail
// is always computed so the summary view can show "N/M resources accessed".
// altAccess, if non-nil, provides per-identity alternative access info keyed
// as altAccess[identityName][resourceLabel].
func buildIdentityResults(
	identities []*models.Node,
	events []logmodels.AccessgraphStorageV1alphaEvent,
	altAccess map[string]map[string]bool,
) []IdentityAccessResult {
	type perResource struct {
		count      int
		lastAccess time.Time
	}
	type activityEntry struct {
		count      int
		lastAccess time.Time
		byResource map[string]*perResource
	}

	byIdentity := make(map[string]*activityEntry)
	for _, ev := range events {
		name := ev.Identity.Name
		if name == "" {
			continue
		}
		entry := byIdentity[name]
		if entry == nil {
			entry = &activityEntry{byResource: make(map[string]*perResource)}
			byIdentity[name] = entry
		}
		entry.count++
		if ev.Time.After(entry.lastAccess) {
			entry.lastAccess = ev.Time
		}
		res := entry.byResource[ev.Target.Resource]
		if res == nil {
			res = &perResource{}
			entry.byResource[ev.Target.Resource] = res
		}
		res.count++
		if ev.Time.After(res.lastAccess) {
			res.lastAccess = ev.Time
		}
	}

	results := make([]IdentityAccessResult, 0, len(identities))
	for _, identity := range identities {
		props, err := identity.Properties.AsIdentityProperties()
		source := ""
		if err == nil && props.Source != nil {
			source = *props.Source
		}

		r := IdentityAccessResult{
			IdentityName: identity.Name,
			IdentityKind: identity.SubKind,
			Source:       source,
		}
		if entry := byIdentity[identity.Name]; entry != nil {
			r.AccessCount = entry.count
			t := entry.lastAccess
			r.LastAccess = &t

			for resName, ra := range entry.byResource {
				t := ra.lastAccess
				r.ResourceActivity = append(r.ResourceActivity, ResourceAccessEntry{
					ResourceName: resName,
					AccessCount:  ra.count,
					LastAccess:   &t,
				})
			}
			slices.SortFunc(r.ResourceActivity, func(a, b ResourceAccessEntry) int {
				return compareByLastAccessEntry(a.LastAccess, b.LastAccess)
			})
		}

		if identityAlt := altAccess[identity.Name]; len(identityAlt) > 0 {
			for label := range identityAlt {
				r.AlternativeAccessResources = append(r.AlternativeAccessResources, label)
			}
			slices.Sort(r.AlternativeAccessResources)
		}

		results = append(results, r)
	}
	return results
}

// resourceLabel returns the label used in activity events for a resource node:
// alias when present, otherwise name. Keeps activity and alt-access keys in sync.
func resourceLabel(n *models.Node) string {
	if props, err := n.Properties.AsResourceProperties(); err == nil && props.Alias != nil && *props.Alias != "" {
		return *props.Alias
	}
	return n.Name
}

// computeAlternativeAccess determines, for each identity with access via the
// subjects, which of the governed resources they could still reach without
// the subjects in the picture. Returns map[identityName][resourceLabel]=true.
func computeAlternativeAccess(
	ctx context.Context,
	args accessGraphServices,
	subjects []models.Node,
	resources []*models.Node,
) (map[string]map[string]bool, error) {
	if len(resources) == 0 || len(subjects) == 0 {
		return nil, nil
	}
	resourceNodes := utilslices.Map(resources, func(n *models.Node) models.Node { return *n })
	pathNodes, pathEdges, err := queryAccessPath(ctx, args, resourceNodes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	g := newTraversalGraph(pathNodes, pathEdges)

	excluded := make(map[types.UUID]bool, len(subjects))
	for _, s := range subjects {
		excluded[s.Id] = true
	}

	alt := make(map[string]map[string]bool)
	for _, r := range resources {
		label := resourceLabel(r)
		for _, id := range g.GetIdentityNodesWithAccessExcluding(*r, excluded) {
			if alt[id.Name] == nil {
				alt[id.Name] = make(map[string]bool)
			}
			alt[id.Name][label] = true
		}
	}
	slog.DebugContext(ctx, "computed alternative access",
		"resources", len(resources),
		"identities_with_alt", len(alt),
	)
	return alt, nil
}

// usedSubject reports whether the identity actually needed the subject being
// reviewed. It returns true when they accessed at least one governed resource
// that they could NOT have reached via an alternative standing access path.
// Used as the inverse for --unused.
func (r IdentityAccessResult) usedSubject() bool {
	if r.AccessCount == 0 {
		return false
	}
	altSet := make(map[string]bool, len(r.AlternativeAccessResources))
	for _, label := range r.AlternativeAccessResources {
		altSet[label] = true
	}
	for _, ra := range r.ResourceActivity {
		if ra.AccessCount > 0 && !altSet[ra.ResourceName] {
			return true
		}
	}
	return false
}

func compareByLastAccess(a, b IdentityAccessResult) int {
	return compareByLastAccessEntry(a.LastAccess, b.LastAccess)
}

func compareByLastAccessEntry(a, b *time.Time) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return 1
	case b == nil:
		return -1
	default:
		return b.Compare(*a)
	}
}

// --- display ----------------------------------------------------------------

func displayReviewOutput(out io.Writer, output ReviewOutput, from, to time.Time, detailed bool, format string) error {
	return writeOutput(out, output, format, func(w io.Writer) error {
		return displayReviewText(w, output, from, to, detailed)
	})
}

func displayReviewText(out io.Writer, output ReviewOutput, from, to time.Time, detailed bool) error {
	subjectLabel := strings.ToUpper(output.SubjectType[:1]) + output.SubjectType[1:]

	if len(output.Subjects) == 1 {
		s := output.Subjects[0]
		label := s.Name
		if s.Alias != "" {
			label = s.Alias
		}
		fmt.Fprintf(out, "%s: %s [%s] (node_id: %s)\n", subjectLabel, label, s.SubKind, s.ID)
	} else {
		fmt.Fprintf(out, "%ss (%d):\n", subjectLabel, len(output.Subjects))
		for _, s := range output.Subjects {
			label := s.Name
			if s.Alias != "" {
				label = s.Alias
			}
			fmt.Fprintf(out, "  • %s [%s] (node_id: %s)\n", label, s.SubKind, s.ID)
		}
	}

	// For acl/role review, list the governed resources separately.
	if output.SubjectType != "resource" {
		fmt.Fprintf(out, "Resources (%d):\n", len(output.Resources))
		for _, r := range output.Resources {
			label := r.Name
			if r.Alias != "" {
				label = r.Alias
			}
			fmt.Fprintf(out, "  • %s [%s] (node_id: %s)\n", label, r.SubKind, r.ID)
		}
	}

	fmt.Fprintf(out, "Period: %s → %s\n\n", from.Format(time.RFC3339), to.Format(time.RFC3339))

	showAltAccess := output.SubjectType != "resource"
	if len(output.Identities) == 0 {
		fmt.Fprintln(out, "No results.")
	} else if detailed {
		if err := displayReviewDetailed(out, output.Identities, output.Resources, showAltAccess); err != nil {
			return trace.Wrap(err)
		}
	} else {
		if err := displayReviewSummary(out, output.Identities, len(output.Resources), showAltAccess); err != nil {
			return trace.Wrap(err)
		}
	}

	if len(output.Warnings) > 0 {
		fmt.Fprintln(out)
		for _, w := range output.Warnings {
			fmt.Fprintf(out, "Warning: %s\n", w)
		}
	}
	return nil
}

// displayReviewSummary shows one row per identity. For a single-resource review
// the columns are Access Count + Last Access; for multi-resource it also shows
// how many distinct resources each identity accessed. When showAltAccess is
// true, an additional column reports how many governed resources the identity
// can reach without needing the subject.
func displayReviewSummary(out io.Writer, results []IdentityAccessResult, totalResources int, showAltAccess bool) error {
	var headers []string
	if totalResources == 1 {
		headers = []string{"Identity", "Kind", "Source", "Access Count"}
	} else {
		headers = []string{"Identity", "Kind", "Source", "Resources Accessed", "Total Accesses"}
	}
	if showAltAccess {
		headers = append(headers, "Alt Access")
	}
	headers = append(headers, "Last Access")
	table := asciitable.MakeTable(headers)

	for _, r := range results {
		var row []string
		if totalResources == 1 {
			row = []string{r.IdentityName, r.IdentityKind, r.Source, fmt.Sprintf("%d", r.AccessCount)}
		} else {
			accessed := 0
			for _, ra := range r.ResourceActivity {
				if ra.AccessCount > 0 {
					accessed++
				}
			}
			row = []string{r.IdentityName, r.IdentityKind, r.Source, fmt.Sprintf("%d/%d", accessed, totalResources), fmt.Sprintf("%d", r.AccessCount)}
		}
		if showAltAccess {
			row = append(row, fmt.Sprintf("%d/%d", len(r.AlternativeAccessResources), totalResources))
		}
		row = append(row, formatLastAccess(r.LastAccess))
		table.AddRow(row)
	}
	_, err := fmt.Fprintln(out, table.AsBuffer().String())
	return trace.Wrap(err)
}

// displayReviewDetailed shows one row per identity/resource pair. Resources
// with no activity are shown with count 0 and "never". When showAltAccess is
// true, a per-row "Alt Access" column shows whether the identity can reach
// that specific resource without needing the subject.
func displayReviewDetailed(out io.Writer, results []IdentityAccessResult, resources []ReviewedResource, showAltAccess bool) error {
	resourceLabels := utilslices.Map(resources, func(r ReviewedResource) string {
		if r.Alias != "" {
			return r.Alias
		}
		return r.Name
	})

	headers := []string{"Identity", "Kind", "Source", "Resource", "Access Count"}
	if showAltAccess {
		headers = append(headers, "Alt Access")
	}
	headers = append(headers, "Last Access")
	table := asciitable.MakeTable(headers)

	for _, r := range results {
		activityByResource := make(map[string]ResourceAccessEntry, len(r.ResourceActivity))
		for _, ra := range r.ResourceActivity {
			activityByResource[ra.ResourceName] = ra
		}
		altSet := make(map[string]bool, len(r.AlternativeAccessResources))
		for _, label := range r.AlternativeAccessResources {
			altSet[label] = true
		}
		for i, label := range resourceLabels {
			identity, kind, source := "", "", ""
			if i == 0 {
				identity, kind, source = r.IdentityName, r.IdentityKind, r.Source
			}
			var row []string
			var lastAccess string
			if ra, ok := activityByResource[label]; ok {
				row = []string{identity, kind, source, label, fmt.Sprintf("%d", ra.AccessCount)}
				lastAccess = formatLastAccess(ra.LastAccess)
			} else {
				row = []string{identity, kind, source, label, "0"}
				lastAccess = "never"
			}
			if showAltAccess {
				if altSet[label] {
					row = append(row, "yes")
				} else {
					row = append(row, "no")
				}
			}
			row = append(row, lastAccess)
			table.AddRow(row)
		}
	}
	_, err := fmt.Fprintln(out, table.AsBuffer().String())
	return trace.Wrap(err)
}

func formatLastAccess(t *time.Time) string {
	if t == nil {
		return "never"
	}
	return t.Format(time.RFC3339)
}
