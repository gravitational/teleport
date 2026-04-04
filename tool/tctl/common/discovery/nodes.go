// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package discovery

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/clientutils"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/usertasks"
)

// discoveryClient abstracts the auth client methods used by discovery commands.
// It is a strict subset of authclient.Client.
type discoveryClient interface {
	// SearchEvents searches audit events.
	SearchEvents(ctx context.Context, req libevents.SearchEventsRequest) ([]apievents.AuditEvent, string, error)
	// GetResources lists resources with pagination.
	GetResources(ctx context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error)
	// UserTasksClient returns a client for managing user tasks.
	UserTasksClient() services.UserTasks
}

// runResult captures the most recent installation script result for an instance,
// sourced from audit events (e.g. SSM run events for AWS).
type runResult struct {
	ExitCode  int64     `json:"exit_code"`
	Output    string    `json:"output"`
	Time      time.Time `json:"time"`
	IsFailure bool      `json:"is_failure"`
}

// instanceInfo represents a discovered cloud instance, built by correlating
// audit events, online Teleport nodes, and user tasks on the cloud instance ID.
type instanceInfo struct {
	Region        string     `json:"region"`
	IsOnline      bool       `json:"is_online"`
	Expiry        time.Time  `json:"expiry,omitempty"`
	RunResult     *runResult `json:"run_result,omitempty"`
	UserTaskID    string     `json:"user_task_id,omitempty"`
	UserTaskIssue string     `json:"user_task_issue,omitempty"`

	AWS *awsInfo `json:"aws,omitempty"`
}

// cloudInfo provides cloud-specific identifiers for an instance.
// Used for rendering, sorting, and matching against user tasks.
type cloudInfo interface {
	cloudName() string
	cloudInstanceID() string
	cloudAccountID() string
}

// cloud returns the cloud-specific metadata for this instance, or nil
// if no cloud provider info is available.
func (inst instanceInfo) cloud() cloudInfo {
	if inst.AWS != nil {
		return inst.AWS
	}
	return nil
}

// lastTimeValue returns the most recent timestamp for sorting.
// Prefers the run result time, falls back to expiry.
func (inst instanceInfo) lastTimeValue() time.Time {
	if inst.RunResult != nil && !inst.RunResult.Time.IsZero() {
		return inst.RunResult.Time
	}
	return inst.Expiry
}

// lastTime returns the most recent run timestamp formatted as RFC3339.
// For instances with no run results, falls back to the node expiry time.
func (inst instanceInfo) lastTime() string {
	t := inst.lastTimeValue()
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// runOutput returns the run command stdout/stderr with newlines escaped for
// single-line display. Output longer than runOutputMaxLen is truncated.
func (inst instanceInfo) runOutput() string {
	if inst.RunResult == nil {
		return ""
	}
	out := strings.TrimSpace(inst.RunResult.Output)
	if out == "" {
		return ""
	}

	// runOutputMaxLen is the maximum length of the raw output string before
	// quoting. Longer values are truncated with an ellipsis.
	const runOutputMaxLen = 100

	if len(out) > runOutputMaxLen {
		out = out[:runOutputMaxLen] + "..."
	}
	return fmt.Sprintf("%q", out)
}

// details returns a combined column: user task description if present, otherwise
// the run output prefixed with its source for clarity.
// Full details are always available in JSON output.
func (inst instanceInfo) details() string {
	if inst.UserTaskIssue != "" {
		title, _ := usertasks.DescriptionForDiscoverEC2Issue(inst.UserTaskIssue)
		if title == "" {
			title = inst.UserTaskIssue
		}
		return title
	}
	out := inst.runOutput()
	if out == "" {
		return ""
	}
	return "Script output: " + out
}

// status returns a human-readable status combining online state and run result.
func (inst instanceInfo) status() string {
	if inst.RunResult == nil {
		if inst.IsOnline {
			return "Online"
		}
		return ""
	}
	if !inst.RunResult.IsFailure {
		if inst.IsOnline {
			return "Online"
		}
		return "Installed (offline)"
	}
	if inst.IsOnline {
		return fmt.Sprintf("Online, exit code=%d", inst.RunResult.ExitCode)
	}
	return fmt.Sprintf("Failed (exit code=%d)", inst.RunResult.ExitCode)
}

// filterFailures returns only instances that have a failed run result or a user task.
func filterFailures(instances []instanceInfo) []instanceInfo {
	result := make([]instanceInfo, 0, len(instances))
	for _, inst := range instances {
		if (inst.RunResult != nil && inst.RunResult.IsFailure) || inst.UserTaskID != "" {
			result = append(result, inst)
		}
	}
	return result
}

// buildNodes builds a report of discovered cloud instances by correlating three
// data sources on the cloud instance ID (e.g. AWS EC2 instance ID):
//  1. Installation script audit events (e.g. SSM runs) — provides run results.
//  2. Online Teleport nodes — provides enrollment/online status.
//  3. User tasks — provides human-readable issue descriptions.
func buildNodes(ctx context.Context, clt discoveryClient, from, to time.Time) ([]instanceInfo, error) {
	slog.DebugContext(ctx, "Fetching installation audit events")
	ssmEvents, err := getRunEvents(ctx, clt, from, to)
	if err != nil {
		return nil, trace.Wrap(err, "fetching installation audit events")
	}
	slog.DebugContext(ctx, "Fetched installation audit events", "ssm_count", len(ssmEvents))

	slog.DebugContext(ctx, "Fetching online nodes")
	nodes, err := client.GetAllResources[types.Server](ctx, clt, &proto.ListResourcesRequest{
		ResourceType: types.KindNode,
		Namespace:    apidefaults.Namespace,
	})
	if err != nil {
		return nil, trace.Wrap(err, "fetching nodes")
	}
	slog.DebugContext(ctx, "Fetched online nodes", "count", len(nodes))

	instances := correlate(ssmEvents, nodes)

	slog.DebugContext(ctx, "Fetching user tasks")
	tasks, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, token string) ([]*usertasksv1.UserTask, string, error) {
		return clt.UserTasksClient().ListUserTasks(ctx, int64(limit), token, nil)
	}))
	if err != nil {
		return nil, trace.Wrap(err, "fetching user tasks")
	}
	slog.DebugContext(ctx, "Fetched user tasks", "count", len(tasks))
	matchUserTasks(instances, tasks)

	return instances, nil
}

// matchUserTasks populates user task fields on instances whose cloud instance
// ID appears in a user task's instance list (e.g. DiscoverEC2.Instances map).
func matchUserTasks(instances []instanceInfo, tasks []*usertasksv1.UserTask) {
	// Build lookup: cloud name -> instance ID -> user task.
	taskByInstance := map[string]map[string]*usertasksv1.UserTask{
		cloudAWS: make(map[string]*usertasksv1.UserTask),
	}

	for _, task := range tasks {
		spec := task.GetSpec()
		if spec == nil {
			continue
		}
		if ec2 := spec.GetDiscoverEc2(); ec2 != nil {
			for instanceID := range ec2.GetInstances() {
				taskByInstance[cloudAWS][instanceID] = task
			}
		}
	}

	for i := range instances {
		ci := instances[i].cloud()
		if ci == nil {
			continue
		}
		if task, ok := taskByInstance[ci.cloudName()][ci.cloudInstanceID()]; ok {
			instances[i].UserTaskID = task.GetMetadata().GetName()
			instances[i].UserTaskIssue = task.GetSpec().GetIssueType()
		}
	}
}

// getRunEvents fetches installation script audit events in descending time order
// (most recent first). Currently returns SSM run events; will include Azure/GCP
// equivalents when available.
func getRunEvents(ctx context.Context, clt discoveryClient, from, to time.Time) ([]*apievents.SSMRun, error) {
	events, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, token string) ([]apievents.AuditEvent, string, error) {
		return clt.SearchEvents(ctx, libevents.SearchEventsRequest{
			From:       from,
			To:         to,
			EventTypes: []string{libevents.SSMRunEvent},
			Order:      types.EventOrderDescending,
			Limit:      limit,
			StartKey:   token,
		})
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Type-assert to concrete event types. Currently only SSM runs exist;
	// Azure/GCP run command events would be extracted here as well.
	var ssmRuns []*apievents.SSMRun
	for _, ev := range events {
		switch run := ev.(type) {
		case *apievents.SSMRun:
			ssmRuns = append(ssmRuns, run)
		}
	}
	return ssmRuns, nil
}

// correlate joins installation audit events and online nodes into a unified
// instance list, keyed by cloud instance ID. Events are processed first to
// seed the map with run results, then nodes enrich existing entries with
// online status or create new entries for instances with no audit events.
// The result is sorted by account, region, then descending time.
func correlate(ssmEvents []*apievents.SSMRun, nodes []types.Server) []instanceInfo {
	// Key: cloud instance ID (e.g. AWS EC2 instance ID like "i-abc123").
	instances := make(map[string]*instanceInfo)

	correlateSSMEvents(instances, ssmEvents)
	correlateNodes(instances, nodes)

	// Convert map to sorted slice.
	result := make([]instanceInfo, 0, len(instances))
	for _, info := range instances {
		result = append(result, *info)
	}
	slices.SortFunc(result, func(a, b instanceInfo) int {
		var aAccount, bAccount string
		if ci := a.cloud(); ci != nil {
			aAccount = ci.cloudAccountID()
		}
		if ci := b.cloud(); ci != nil {
			bAccount = ci.cloudAccountID()
		}
		if c := cmp.Compare(aAccount, bAccount); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Region, b.Region); c != 0 {
			return c
		}
		// Descending time: newer entries first.
		return b.lastTimeValue().Compare(a.lastTimeValue())
	})
	return result
}

// combineOutput joins stdout and stderr with a newline separator.
func combineOutput(stdout, stderr string) string {
	stdout = strings.TrimSpace(stdout)
	stderr = strings.TrimSpace(stderr)
	if stdout != "" && stderr != "" {
		return stdout + "\n" + stderr
	}
	if stdout != "" {
		return stdout
	}
	return stderr
}

// resolveTimeRange parses a --last duration string into a (from, to) pair.
func resolveTimeRange(clock clockwork.Clock, last string) (from, to time.Time, err error) {
	if clock == nil {
		clock = clockwork.NewRealClock()
	}
	now := clock.Now().UTC()
	d, err := types.ParseDuration(strings.TrimSpace(last))
	if err != nil {
		return time.Time{}, time.Time{}, trace.BadParameter("invalid --last value %q, expected a duration like 1h, 24h, or 30m", last)
	}
	return now.Add(-d.Duration()), now, nil
}
