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
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"

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
	APIError  string    `json:"api_error"`
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
	Expiry        time.Time  `json:"expiry,omitzero"`
	RunResult     *runResult `json:"run_result,omitempty"`
	UserTaskID    string     `json:"user_task_id,omitempty"`
	UserTaskIssue string     `json:"user_task_issue,omitempty"`

	AWS   *awsInfo   `json:"aws,omitempty"`
	Azure *azureInfo `json:"azure,omitempty"`
}

// cloudInfo provides cloud-specific identifiers for an instance.
// Used for rendering, sorting, and matching against user tasks.
type cloudInfo interface {
	cloudName() string
	cloudAccountID() string
	instanceText() string
}

// cloud returns the cloud-specific metadata for this instance, or nil
// if no cloud provider info is available.
func (inst instanceInfo) cloud() cloudInfo {
	if inst.AWS != nil {
		return inst.AWS
	}
	if inst.Azure != nil {
		return inst.Azure
	}
	return nil
}

// accountID returns the cloud account ID for this instance, or "" if no
// cloud provider info is available.
func (inst instanceInfo) accountID() string {
	if ci := inst.cloud(); ci != nil {
		return ci.cloudAccountID()
	}
	return ""
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

func trimEscape(out string) string {
	out = strings.TrimSpace(out)
	if out == "" {
		return ""
	}

	// maxLen is the maximum length of the raw output string before
	// quoting. Longer values are truncated with an ellipsis.
	const maxLen = 100
	if len(out) > maxLen {
		out = out[:maxLen] + "..."
	}
	return fmt.Sprintf("%q", out)
}

// details returns a column combining available information: the user task title
// (if any), followed by the script output or API error (if any).
// Full, non-combined details are always available in JSON output.
func (inst instanceInfo) details() string {
	title := inst.userTaskTitle()
	var extra string
	if inst.RunResult != nil {
		if out := trimEscape(inst.RunResult.Output); out != "" {
			extra = "Script output: " + out
		} else if apiErr := trimEscape(inst.RunResult.APIError); apiErr != "" {
			extra = "API error: " + apiErr
		}
	}
	switch {
	case title != "" && extra != "":
		return title + ". " + extra
	case title != "":
		return title
	default:
		return extra
	}
}

// userTaskTitle returns the human-readable title for this instance's user task
// issue, or "" if there is no task. Falls back to the raw issue string when
// the cloud is unknown or the issue type has no registered description.
func (inst instanceInfo) userTaskTitle() string {
	if inst.UserTaskIssue == "" {
		return ""
	}
	var title string
	if ci := inst.cloud(); ci != nil {
		switch ci.cloudName() {
		case cloudAWS:
			title, _ = usertasks.DescriptionForDiscoverEC2Issue(inst.UserTaskIssue)
		case cloudAzure:
			title, _ = usertasks.DescriptionForDiscoverAzureVMIssue(inst.UserTaskIssue)
		}
	}
	return cmp.Or(title, inst.UserTaskIssue)
}

// status returns a short human-readable status combining online state and run result.
func (inst instanceInfo) status() string {
	var runResultStatus string
	if inst.RunResult != nil && inst.RunResult.IsFailure {
		if inst.RunResult.APIError != "" {
			runResultStatus = "API error" // details column will have more information.
		} else {
			runResultStatus = fmt.Sprintf("exit code=%d", inst.RunResult.ExitCode)
		}
	}

	// machine is online
	if inst.IsOnline {
		if runResultStatus == "" {
			return "Online"
		}
		// most recent installation attempt failed, yet the machine is online anyway.
		// odd, but it can happen in some configurations.
		return "Online, " + runResultStatus
	}

	// offline machine, no run result... why are we even here?
	if inst.RunResult == nil {
		return "Unknown"
	}

	// installation worked, but machine failed to join or went down for some other reason.
	if !inst.RunResult.IsFailure {
		return "Installed (offline)"
	}

	// plain failure; include the reason.
	return fmt.Sprintf("Failed (%s)", runResultStatus)
}

func (inst instanceInfo) failed() bool {
	if inst.RunResult != nil && inst.RunResult.IsFailure {
		return true
	}
	if inst.UserTaskID != "" {
		return true
	}
	return false
}

// filterFailures returns only instances that have failed in some way.
func filterFailures(instances []instanceInfo) []instanceInfo {
	return slices.DeleteFunc(slices.Clone(instances), func(inst instanceInfo) bool {
		return !inst.failed()
	})
}

type cloudProviderConfig struct {
	aws, azure bool
}

func parseCloudProviders(value string) (cloudProviderConfig, error) {
	const (
		cloudProviderAWS   = "aws"
		cloudProviderAzure = "azure"
	)

	if value == "" {
		return cloudProviderConfig{
			aws:   true,
			azure: true,
		}, nil
	}

	cfg := cloudProviderConfig{}
	parts := strings.Split(value, ",")
	for _, part := range parts {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case cloudProviderAWS:
			cfg.aws = true
		case cloudProviderAzure:
			cfg.azure = true
		case "":
			return cloudProviderConfig{}, trace.BadParameter("empty cloud provider in --cloud (allowed: aws, azure)")
		default:
			return cloudProviderConfig{}, trace.BadParameter("unknown cloud provider %q (allowed: aws, azure)", part)
		}
	}
	return cfg, nil
}

// buildNodes combines information about cloud instances from three sources,
// matching on cloud instance ID:
//  1. Installation audit events.
//  2. Online Teleport nodes.
//  3. User tasks.
func buildNodes(ctx context.Context, clt discoveryClient, from, to time.Time, cfg cloudProviderConfig) ([]instanceInfo, error) {
	slog.DebugContext(ctx, "Fetching installation audit events")
	ssmEvents, azureEvents, err := getRunEvents(ctx, clt, from, to, cfg)
	if err != nil {
		return nil, trace.Wrap(err, "fetching installation audit events")
	}
	slog.DebugContext(ctx, "Fetched installation audit events", "ssm_count", len(ssmEvents), "azure_count", len(azureEvents))

	slog.DebugContext(ctx, "Fetching online nodes")
	nodes, err := client.GetAllResources[types.Server](ctx, clt, &proto.ListResourcesRequest{
		ResourceType: types.KindNode,
		Namespace:    apidefaults.Namespace,
	})
	if err != nil {
		return nil, trace.Wrap(err, "fetching nodes")
	}
	slog.DebugContext(ctx, "Fetched online nodes", "count", len(nodes))

	slog.DebugContext(ctx, "Fetching user tasks")
	tasks, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, token string) ([]*usertasksv1.UserTask, string, error) {
		return clt.UserTasksClient().ListUserTasks(ctx, int64(limit), token, nil)
	}))
	if err != nil {
		return nil, trace.Wrap(err, "fetching user tasks")
	}
	slog.DebugContext(ctx, "Fetched user tasks", "count", len(tasks))

	var instances []instanceInfo
	if cfg.aws {
		awsInstances := cloudNodes(
			mergeInstances(correlateSSMEvents(ssmEvents), correlateAWSNodes(nodes)),
			tasks, awsTaskInstanceKeys)
		instances = append(instances, awsInstances...)
	}
	if cfg.azure {
		azureInstances := cloudNodes(
			mergeInstances(correlateAzureRunEvents(azureEvents), correlateAzureNodes(nodes)),
			tasks, azureTaskInstanceKeys)
		instances = append(instances, azureInstances...)
	}

	return instances, nil
}

// cloudNodes finalizes one cloud's pipeline: flatten the instance map to a
// slice, populate user-task fields, and sort.
func cloudNodes(
	instances map[string]instanceInfo,
	tasks []*usertasksv1.UserTask,
	taskInstanceKeys func(*usertasksv1.UserTask) []string,
) []instanceInfo {
	taskMap := make(map[string]*usertasksv1.UserTask)
	for _, task := range tasks {
		for _, instanceKey := range taskInstanceKeys(task) {
			taskMap[instanceKey] = task
		}
	}

	result := make([]instanceInfo, 0, len(instances))
	for instanceKey, info := range instances {
		if task, match := taskMap[instanceKey]; match {
			info.UserTaskID = task.GetMetadata().GetName()
			info.UserTaskIssue = task.GetSpec().GetIssueType()
		}
		// collect updated copy
		result = append(result, info)
	}
	slices.SortFunc(result, sortInstances)

	return result
}

// sortInstances orders instances by cloud account, region, then descending time.
func sortInstances(a, b instanceInfo) int {
	return cmp.Or(
		cmp.Compare(a.accountID(), b.accountID()),
		cmp.Compare(a.Region, b.Region),
		// Descending time: newer entries first.
		b.lastTimeValue().Compare(a.lastTimeValue()),
	)
}

// getRunEvents fetches installation script audit events in descending time order
// (most recent first). Returns SSM run events (AWS) and Azure run events; will
// include GCP equivalents when available.
func getRunEvents(ctx context.Context, clt discoveryClient, from, to time.Time, cfg cloudProviderConfig) ([]*apievents.SSMRun, []*apievents.AzureRun, error) {
	var eventTypes []string
	if cfg.aws {
		eventTypes = append(eventTypes, libevents.SSMRunEvent)
	}
	if cfg.azure {
		eventTypes = append(eventTypes, libevents.AzureRunEvent)
	}

	events, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, token string) ([]apievents.AuditEvent, string, error) {
		return clt.SearchEvents(ctx, libevents.SearchEventsRequest{
			From:       from,
			To:         to,
			EventTypes: eventTypes,
			Order:      types.EventOrderDescending,
			Limit:      limit,
			StartKey:   token,
		})
	}))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Type-assert to concrete event types. GCP run command events would be
	// extracted here as well.
	var ssmRuns []*apievents.SSMRun
	var azureRuns []*apievents.AzureRun
	for _, ev := range events {
		switch run := ev.(type) {
		case *apievents.SSMRun:
			ssmRuns = append(ssmRuns, run)
		case *apievents.AzureRun:
			azureRuns = append(azureRuns, run)
		}
	}
	return ssmRuns, azureRuns, nil
}

// mergeInstances merges two instanceInfo maps with bias toward fstMap.
func mergeInstances(fstMap, sndMap map[string]instanceInfo) map[string]instanceInfo {
	out := make(map[string]instanceInfo)
	maps.Copy(out, fstMap)
	for k, sndValue := range sndMap {
		out[k] = mergeInstanceInfo(out[k], sndValue)
	}
	return out
}

// mergeInstanceInfo merges two instance info structs with bias toward fst.
func mergeInstanceInfo(fst, snd instanceInfo) instanceInfo {
	return instanceInfo{
		Region:        cmp.Or(fst.Region, snd.Region),
		IsOnline:      cmp.Or(fst.IsOnline, snd.IsOnline),
		Expiry:        cmp.Or(fst.Expiry, snd.Expiry),
		RunResult:     cmp.Or(fst.RunResult, snd.RunResult),
		UserTaskID:    cmp.Or(fst.UserTaskID, snd.UserTaskID),
		UserTaskIssue: cmp.Or(fst.UserTaskIssue, snd.UserTaskIssue),
		AWS:           cmp.Or(fst.AWS, snd.AWS),
		Azure:         cmp.Or(fst.Azure, snd.Azure),
	}
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
