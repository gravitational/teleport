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
	"github.com/gravitational/teleport"
	accessgraph "github.com/gravitational/teleport/lib/accessgraph/apiclient"
	models "github.com/gravitational/teleport/lib/accessgraph/apiclient/models/logs"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/trace"
)

type accessRequestsArgs struct {
	cmd *kingpin.CmdClause
	ls  accessRequestsListArgs
	// Date filters
	from time.Time
	to   time.Time
	// Output format
	format string
}

type accessRequestsListArgs struct {
	cmd *kingpin.CmdClause

	// General filters
	kind      string
	state     string
	requester string
	approver  string

	// Meta filters
	unused bool

	// Output control
	limit int
}

func (c *AccessGraphCommand) initAccessRequests(app *kingpin.Application) {
	accessRequestCmd := app.Command("access-requests", "Review access requests and approvals.").Hidden()
	accessRequestCmd.Flag("from", fmt.Sprintf("Include activity at or after this time. (Examples: %s, 24h, 7d; negative durations like -1h are future-relative. Default: 30d)", time.RFC3339)).
		Default("30d").
		SetValue(timeValue{target: &c.accessRequests.from})
	accessRequestCmd.Flag("to", fmt.Sprintf("Include activity at or before this time. (Examples: %s, 24h, 7d; negative durations like -1h are future-relative. Default: now)", time.RFC3339)).
		Default("now").
		SetValue(timeValue{target: &c.accessRequests.to})
	accessRequestCmd.Flag("format", "Output format. (Values: text, json, yaml)").
		Default(teleport.YAML).
		EnumVar(&c.accessRequests.format, teleport.Text, teleport.JSON, teleport.YAML)
	c.accessRequests.cmd = accessRequestCmd

	c.initAccessRequestsList(c.accessRequests.cmd)
}

func (c *AccessGraphCommand) initAccessRequestsList(parent *kingpin.CmdClause) {
	lsCmd := parent.Command("ls", "List access requests.")
	lsCmd.Flag("kind", "Filter for a specific kind of access request. (Example: kube_cluster, database, role)").
		StringVar(&c.accessRequests.ls.kind)
	lsCmd.Flag("state", "Filter by request state. (Values: PENDING, APPROVED, DENIED, PROMOTED)").
		EnumVar(&c.accessRequests.ls.state, "PENDING", "APPROVED", "DENIED", "PROMOTED")
	lsCmd.Flag("user", "Filter by the Teleport user who created the request. (Example: alice)").
		StringVar(&c.accessRequests.ls.requester)
	lsCmd.Flag("approver", "Filter by the Teleport user who approved the request. (Example: bob)").
		StringVar(&c.accessRequests.ls.approver)
	lsCmd.Flag("limit", "Limit the number of access requests returned.").
		Default("50").
		IntVar(&c.accessRequests.ls.limit)

	lsCmd.Flag("unused", "Filter for requests that were approved but not used.").
		BoolVar(&c.accessRequests.ls.unused)
	c.accessRequests.ls.cmd = lsCmd
}

// AccessRequestsList executes `tctl access-requests ls`.
func (c *AccessGraphCommand) AccessRequestsList(ctx context.Context, client *accessgraph.ClientWithResponses) error {
	query := constructAccessRequestsListQuery(c.accessRequests)

	resp, err := doRequest(client.ExecuteLogsQueryV1WithResponse(ctx, &query))

	if err != nil {
		return trace.Wrap(err)
	}

	grouped := groupRequestsById(resp.JSON200.Data)
	var requests []AccessRequestListResult
	for id, logs := range grouped {
		requests = append(requests, parseAccessRequest(id, logs))
	}

	// Cross-reference with cert.create events to figure out which approved
	// requests actually had a cert issued for them.
	usage, err := fetchAccessRequestUsage(ctx, client, c.accessRequests.from, c.accessRequests.to)
	if err != nil {
		return trace.Wrap(err)
	}
	for i := range requests {
		if firstUse, ok := usage[requests[i].Id]; ok {
			t := firstUse
			requests[i].UsedOn = &t
		}
	}

	return displayAccessRequests(c.stdout, requests, c.accessRequests.format)
}

// fetchAccessRequestUsage returns map[requestID]firstUseTime derived from
// cert.create events whose access_requests field includes the request id.
// One query covers all requests in the window, avoiding N per-request lookups.
func fetchAccessRequestUsage(ctx context.Context, client *accessgraph.ClientWithResponses, from, to time.Time) (map[string]time.Time, error) {
	query := "event_type:cert.create"
	order := accessgraph.Asc
	events, _, err := fetchAllLogs(ctx, client, accessgraph.ExecuteLogsQueryV1Params{
		Query:     &query,
		StartTime: &from,
		EndTime:   &to,
		Order:     &order,
	}, 0)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	usage := make(map[string]time.Time)
	for _, ev := range events {
		data, err := ev.EventData.AsTeleportAuditLog()
		if err != nil {
			continue
		}
		ids := getStringSlice(data, "access_requests")
		for _, id := range ids {
			if existing, ok := usage[id]; !ok || ev.Time.Before(existing) {
				usage[id] = ev.Time
			}
		}
	}
	return usage, nil
}

// constructAccessRequestsListQuery builds the query for listing access requests based on the provided arguments.
func constructAccessRequestsListQuery(args accessRequestsArgs) accessgraph.ExecuteLogsQueryV1Params {
	eventTypes := []string{
		"access_request.create", "access_request.review", "access_request.expire",
		"access_request.update", "access_request.delete",
	}
	queryParts := []string{dslClause("event_type", eventTypes)}
	if args.ls.kind != "" {
		queryParts = append(queryParts, dslClause("target_kind", []string{args.ls.kind}))
	}
	if args.ls.state != "" {
		queryParts = append(queryParts, dslClause("status", []string{args.ls.state}))
	}
	if args.ls.requester != "" {
		queryParts = append(queryParts, dslClause("identity", []string{args.ls.requester}))
	}
	if args.ls.approver != "" {
		queryParts = append(queryParts, dslClause("approver", []string{args.ls.approver}))
	}
	query := strings.Join(queryParts, " AND ")

	order := accessgraph.Desc
	return accessgraph.ExecuteLogsQueryV1Params{
		Query:     &query,
		StartTime: &args.from,
		EndTime:   &args.to,
		Limit:     &args.ls.limit,
		Order:     &order,
	}
}

type Review struct {
	Reviewer      string    `json:"reviewer"`
	ProposedState string    `json:"proposed_state"`
	State         string    `json:"state"`
	Reason        string    `json:"reason"`
	ReviewedAt    time.Time `json:"reviewed_at"`
}

type AccessRequestListResult struct {
	// The id for the access request
	Id string `json:"id"`
	// The Teleport user that made the request
	User string `json:"user"`
	// The role that was requested (either a standard or generated role)
	Roles []string `json:"roles"`
	// The resource (kind/name) that the request is for
	Resource string `json:"resource"`
	// The reason provided by the requester when creating the request
	Reason string `json:"reason"`
	// The current state of the request (PENDING, APPROVED, DENIED, etc)
	State string `json:"state"`
	// The time the request was created
	CreatedAt time.Time `json:"created_at"`
	// The Teleport user(s) that approved the request
	Reviews []Review `json:"reviews"`
	// Used determines wether the access was used after being approved.
	UsedOn *time.Time `json:"used_on,omitempty"`
}

func groupRequestsById(logs []models.AccessgraphStorageV1alphaEvent) map[string][]models.AccessgraphStorageV1alphaEvent {
	requestsByID := make(map[string][]models.AccessgraphStorageV1alphaEvent)
	for _, log := range logs {
		ev, err := log.EventData.AsTeleportAuditLog()
		if err != nil {
			slog.Warn("Failed to parse event data", "log_id", log.Uuid, "error", err)
			continue
		}

		requestID := ev["id"].(string)
		requestsByID[requestID] = append(requestsByID[requestID], log)
	}
	return requestsByID
}

func getReview(eventType string, data map[string]any) (*Review, bool) {
	if eventType == "access_request.review" {
		time, err := time.Parse(time.RFC3339, data["time"].(string))
		if err != nil {
			slog.Warn("Failed to parse review time", "error", err)
		}

		reason, ok := data["reason"].(string)
		if !ok {
			reason = ""
		}

		return &Review{
			Reviewer:      data["reviewer"].(string),
			ProposedState: data["proposed_state"].(string),
			State:         data["state"].(string),
			Reason:        reason,
			ReviewedAt:    time,
		}, true
	}
	return nil, false
}

func getReason(eventType string, data map[string]any) string {
	if eventType == "access_request.create" {
		if reason, ok := data["reason"].(string); ok && reason != "" {
			return reason
		}
	}
	return ""
}

func mergeCompactStrings(dst []string, src []string) []string {
	if len(src) == 0 {
		return dst
	}

	merged := append(dst, src...)
	slices.Sort(merged)
	return slices.Compact(merged)
}

func parseAccessRequest(id string, logs []models.AccessgraphStorageV1alphaEvent) AccessRequestListResult {
	result := AccessRequestListResult{Id: id}
	var resourceNames []string

	for _, log := range logs {
		ev, err := log.EventData.AsTeleportAuditLog()
		if err != nil {
			slog.Warn("Failed to parse event data", "log_id", log.Uuid, "error", err)
			continue
		}

		review, ok := getReview(log.EventType, ev)
		if ok {
			result.Reviews = append(result.Reviews, *review)
		}

		if result.User == "" {
			result.User = log.Identity.Name
		}

		if resourceNamesFromEvent := getStringSlice(ev, "resource_names"); len(resourceNamesFromEvent) > 0 {
			resourceNames = mergeCompactStrings(resourceNames, resourceNamesFromEvent)
			result.Resource = strings.Join(resourceNames, ",")
		} else if result.Resource == "" {
			result.Resource = log.Target.Resource
		}

		if roles := getStringSlice(ev, "roles"); len(roles) > 0 {
			result.Roles = mergeCompactStrings(result.Roles, roles)
		}

		if result.Reason == "" {
			result.Reason = getReason(log.EventType, ev)
		}

		if state, ok := ev["state"].(string); ok && state != "" && result.State == "" {
			result.State = state
		}
		// We want to grab the `earliest` created_at, so we keep updating the CreatedAt field
		if createdAt, ok := ev["time"].(string); ok && createdAt != "" {
			parsedTime, err := time.Parse(time.RFC3339, createdAt)
			if err != nil {
				slog.Warn("Failed to parse created_at time", "error", err)
			} else {
				result.CreatedAt = parsedTime
			}
		}
	}
	return result
}

func displayAccessRequests(out io.Writer, reqs []AccessRequestListResult, format string) error {
	if reqs == nil {
		reqs = []AccessRequestListResult{}
	}
	return writeOutput(out, reqs, format, func(w io.Writer) error {
		return displayAccessRequestsText(w, reqs)
	})
}

func formatReviews(reviews []Review) string {
	var formatted []string
	for _, r := range reviews {
		formatted = append(formatted, fmt.Sprintf("%s %s at %s (Reason: %s, State: %s)", r.ProposedState, r.Reviewer, r.ReviewedAt.Format(time.RFC3339), r.Reason, r.State))
	}
	return strings.Join(formatted, "\n ")
}

func displayAccessRequestsText(out io.Writer, reqs []AccessRequestListResult) error {
	table := asciitable.MakeTable([]string{
		"ID",
		"User",
		"Resource",
		"Role",
		"State",
		"Reason",
		"Reviews",
		"Created At",
		"Used On",
	})

	for _, req := range reqs {
		used := "never"
		if req.UsedOn != nil {
			used = req.UsedOn.Format(time.RFC3339)
		}
		table.AddRow([]string{
			req.Id,
			req.User,
			req.Resource,
			strings.Join(req.Roles, ","),
			req.State,
			req.Reason,
			formatReviews(req.Reviews),
			req.CreatedAt.Format(time.RFC3339),
			used,
		})
	}
	_, err := fmt.Fprintln(out, table.AsBuffer().String())
	return trace.Wrap(err)
}
