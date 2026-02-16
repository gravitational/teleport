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
	"slices"
	"strings"
	"time"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	apievents "github.com/gravitational/teleport/api/types/events"
	usertasksapi "github.com/gravitational/teleport/api/types/usertasks"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/usertasks"
	"github.com/gravitational/trace"
)

type taskFilters struct {
	State       string
	Integration string
	TaskType    string
	IssueType   string
}

func normalizeTaskState(input string) (string, error) {
	value := strings.TrimSpace(strings.ToUpper(input))
	switch value {
	case "", usertasksapi.TaskStateOpen:
		return usertasksapi.TaskStateOpen, nil
	case usertasksapi.TaskStateResolved:
		return usertasksapi.TaskStateResolved, nil
	case "ALL":
		return "", nil
	default:
		return "", trace.BadParameter("invalid state %q, valid values are: open, resolved, all", input)
	}
}

func listUserTasks(ctx context.Context, client *authclient.Client, integration, taskState string) ([]*usertasksv1.UserTask, error) {
	items, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, token string) ([]*usertasksv1.UserTask, string, error) {
		return client.UserTasksClient().ListUserTasks(ctx, int64(limit), token, &usertasksv1.ListUserTasksFilters{
			Integration: integration,
			TaskState:   taskState,
		})
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return items, nil
}

func filterUserTasks(tasks []*usertasksv1.UserTask, filters taskFilters) []*usertasksv1.UserTask {
	out := make([]*usertasksv1.UserTask, 0, len(tasks))

	for _, task := range tasks {
		spec := task.GetSpec()
		if filters.State != "" && spec.GetState() != filters.State {
			continue
		}
		if filters.Integration != "" && spec.GetIntegration() != filters.Integration {
			continue
		}
		if filters.TaskType != "" && spec.GetTaskType() != filters.TaskType {
			continue
		}
		if filters.IssueType != "" && spec.GetIssueType() != filters.IssueType {
			continue
		}
		out = append(out, task)
	}

	return out
}

func taskLastStateChange(task *usertasksv1.UserTask) time.Time {
	if ts := task.GetStatus().GetLastStateChange(); ts != nil {
		return ts.AsTime()
	}
	return time.Time{}
}

func taskNamePrefix(name string) string {
	trimmed := strings.TrimSpace(name)
	trimmed = strings.TrimSuffix(trimmed, "...")
	trimmed = strings.TrimSuffix(trimmed, "…")
	if trimmed == "" {
		return ""
	}
	if prefix, ok := shortUUIDPrefix(trimmed); ok {
		return prefix
	}
	return trimmed
}

func shortUUIDPrefix(name string) (string, bool) {
	parts := strings.Split(name, "-")
	if len(parts) != 5 {
		return "", false
	}
	expectedPartLengths := []int{8, 4, 4, 4, 12}
	for i, part := range parts {
		if len(part) != expectedPartLengths[i] || !isHexString(part) {
			return "", false
		}
	}
	return parts[0], true
}

func isHexString(input string) bool {
	for i := 0; i < len(input); i++ {
		ch := input[i]
		switch {
		case ch >= '0' && ch <= '9':
		case ch >= 'a' && ch <= 'f':
		case ch >= 'A' && ch <= 'F':
		default:
			return false
		}
	}
	return true
}

func friendlyTaskType(taskType string) string {
	switch taskType {
	case usertasksapi.TaskTypeDiscoverEC2:
		return "AWS EC2"
	case usertasksapi.TaskTypeDiscoverEKS:
		return "AWS EKS"
	case usertasksapi.TaskTypeDiscoverRDS:
		return "AWS RDS"
	default:
		if strings.TrimSpace(taskType) == "" {
			return "Unknown"
		}
		return taskType
	}
}

func findTaskByNamePrefix(tasks []*usertasksv1.UserTask, input string) (*usertasksv1.UserTask, error) {
	prefix := strings.TrimSpace(input)
	prefix = strings.TrimSuffix(prefix, "...")
	prefix = strings.TrimSuffix(prefix, "…")
	if prefix == "" {
		return nil, trace.BadParameter("task name is required")
	}

	var matches []*usertasksv1.UserTask
	for _, task := range tasks {
		name := task.GetMetadata().GetName()
		if name == prefix {
			return task, nil
		}
		if strings.HasPrefix(name, prefix) {
			matches = append(matches, task)
		}
	}

	switch len(matches) {
	case 0:
		return nil, trace.NotFound("user task %q not found", input)
	case 1:
		return matches[0], nil
	default:
		slices.SortFunc(matches, func(a, b *usertasksv1.UserTask) int {
			return cmp.Compare(a.GetMetadata().GetName(), b.GetMetadata().GetName())
		})
		maxExamples := 5
		if len(matches) < maxExamples {
			maxExamples = len(matches)
		}
		examples := make([]string, 0, maxExamples)
		for i := 0; i < maxExamples; i++ {
			examples = append(examples, matches[i].GetMetadata().GetName())
		}
		return nil, trace.BadParameter(
			"task name prefix %q is ambiguous (%d matches). Use a longer prefix. Matches: %s",
			input,
			len(matches),
			strings.Join(examples, ", "),
		)
	}
}

type taskListItem struct {
	Name            string    `json:"name" yaml:"name"`
	State           string    `json:"state" yaml:"state"`
	TaskType        string    `json:"task_type" yaml:"task_type"`
	IssueType       string    `json:"issue_type" yaml:"issue_type"`
	IssueTitle      string    `json:"issue_title,omitempty" yaml:"issue_title,omitempty"`
	Integration     string    `json:"integration" yaml:"integration"`
	Affected        int       `json:"affected" yaml:"affected"`
	LastStateChange time.Time `json:"last_state_change,omitempty" yaml:"last_state_change,omitempty"`
}

type tasksListOutput struct {
	Total int            `json:"total" yaml:"total"`
	Items []taskListItem `json:"items" yaml:"items"`
}

func toTaskListItems(tasks []*usertasksv1.UserTask) []taskListItem {
	items := make([]taskListItem, 0, len(tasks))
	for _, task := range tasks {
		title, _ := usertasks.DescriptionForDiscoverEC2Issue(task.GetSpec().GetIssueType())
		items = append(items, taskListItem{
			Name:            task.GetMetadata().GetName(),
			State:           task.GetSpec().GetState(),
			TaskType:        task.GetSpec().GetTaskType(),
			IssueType:       task.GetSpec().GetIssueType(),
			IssueTitle:      title,
			Integration:     task.GetSpec().GetIntegration(),
			Affected:        taskAffectedCount(task),
			LastStateChange: taskLastStateChange(task),
		})
	}
	return items
}

func taskAffectedCount(task *usertasksv1.UserTask) int {
	spec := task.GetSpec()
	switch spec.GetTaskType() {
	case usertasksapi.TaskTypeDiscoverEC2:
		return len(spec.GetDiscoverEc2().GetInstances())
	case usertasksapi.TaskTypeDiscoverEKS:
		return len(spec.GetDiscoverEks().GetClusters())
	case usertasksapi.TaskTypeDiscoverRDS:
		return len(spec.GetDiscoverRds().GetDatabases())
	default:
		return 0
	}
}

type statusSummary struct {
	GeneratedAt          time.Time             `json:"generated_at" yaml:"generated_at"`
	FilteredIntegration  string                `json:"filtered_integration,omitempty" yaml:"filtered_integration,omitempty"`
	DiscoveryConfigCount int                   `json:"discovery_config_count" yaml:"discovery_config_count"`
	DiscoveryGroupCount  int                   `json:"discovery_group_count" yaml:"discovery_group_count"`
	UserTasks            []taskListItem        `json:"user_tasks" yaml:"user_tasks"`
	DiscoveryConfigs     []configStatus        `json:"discovery_configs" yaml:"discovery_configs"`
	TotalTasks           int                   `json:"total_tasks" yaml:"total_tasks"`
	OpenTasks            int                   `json:"open_tasks" yaml:"open_tasks"`
	ResolvedTasks        int                   `json:"resolved_tasks" yaml:"resolved_tasks"`
	TasksByType          map[string]int        `json:"tasks_by_type" yaml:"tasks_by_type"`
	TasksByIssue         map[string]int        `json:"tasks_by_issue" yaml:"tasks_by_issue"`
	TasksByIntegration   map[string]int        `json:"tasks_by_integration" yaml:"tasks_by_integration"`
	Integrations         []integrationListItem `json:"integrations" yaml:"integrations"`
	SSMRunStats          *auditEventStats      `json:"ssm_run_stats,omitempty" yaml:"ssm_run_stats,omitempty"`
	JoinStats            *auditEventStats      `json:"join_stats,omitempty" yaml:"join_stats,omitempty"`
}

type auditEventStats struct {
	Window          string    `json:"window" yaml:"window"`
	EffectiveWindow string    `json:"effective_window,omitempty" yaml:"effective_window,omitempty"`
	OldestEvent     time.Time `json:"oldest_event,omitempty" yaml:"oldest_event,omitempty"`
	SuggestedLimit  int       `json:"suggested_limit,omitempty" yaml:"suggested_limit,omitempty"`
	Total           int       `json:"total" yaml:"total"`
	Success         int       `json:"success" yaml:"success"`
	Failed          int       `json:"failed" yaml:"failed"`
	DistinctHosts   int       `json:"distinct_hosts" yaml:"distinct_hosts"`
	FailingHosts    int       `json:"failing_hosts" yaml:"failing_hosts"`
	LimitReached    bool      `json:"limit_reached" yaml:"limit_reached"`
}

type configStatus struct {
	Name       string    `json:"name" yaml:"name"`
	Group      string    `json:"group" yaml:"group"`
	State      string    `json:"state" yaml:"state"`
	Matchers   string    `json:"matchers" yaml:"matchers"`
	Discovered uint64    `json:"discovered" yaml:"discovered"`
	LastSync   time.Time `json:"last_sync" yaml:"last_sync"`
}

type resourcesAggregate struct {
	Found    uint64 `json:"found"`
	Enrolled uint64 `json:"enrolled"`
	Failed   uint64 `json:"failed"`
}

func makeStatusSummary(tasks []*usertasksv1.UserTask, dcs []*discoveryconfig.DiscoveryConfig, integrations []types.Integration, integration string) statusSummary {
	summary := statusSummary{
		GeneratedAt:          time.Now().UTC(),
		FilteredIntegration:  integration,
		DiscoveryConfigCount: len(dcs),
		DiscoveryGroupCount:  countDiscoveryGroups(dcs),
		UserTasks:            make([]taskListItem, 0, len(tasks)),
		DiscoveryConfigs:     make([]configStatus, 0, len(dcs)),
		TotalTasks:           len(tasks),
		TasksByType:          map[string]int{},
		TasksByIssue:         map[string]int{},
		TasksByIntegration:   map[string]int{},
	}

	for _, task := range tasks {
		switch task.GetSpec().GetState() {
		case usertasksapi.TaskStateOpen:
			summary.OpenTasks++
		case usertasksapi.TaskStateResolved:
			summary.ResolvedTasks++
		}
		summary.TasksByType[task.GetSpec().GetTaskType()]++
		summary.TasksByIssue[task.GetSpec().GetIssueType()]++
		summary.TasksByIntegration[task.GetSpec().GetIntegration()]++
	}
	summary.UserTasks = toTaskListItems(tasks)
	slices.SortFunc(summary.UserTasks, func(a, b taskListItem) int {
		if c := compareTimeDesc(a.LastStateChange, b.LastStateChange); c != 0 {
			return c
		}
		return cmp.Compare(a.Name, b.Name)
	})

	for _, dc := range dcs {
		summary.DiscoveryConfigs = append(summary.DiscoveryConfigs, configStatus{
			Name:       dc.GetName(),
			Group:      dc.GetDiscoveryGroup(),
			State:      cmp.Or(strings.TrimSpace(dc.Status.State), "UNKNOWN"),
			Matchers:   configMatchersSummary(dc),
			Discovered: dc.Status.DiscoveredResources,
			LastSync:   dc.Status.LastSyncTime.UTC(),
		})
	}
	slices.SortFunc(summary.DiscoveryConfigs, func(a, b configStatus) int {
		return cmp.Compare(a.Name, b.Name)
	})

	statsMap := buildIntegrationStatsMap(dcs)
	taskCountMap := countTasksByIntegration(tasks)
	summary.Integrations = toIntegrationListItems(integrations, statsMap, taskCountMap)

	return summary
}

func configMatchersSummary(dc *discoveryconfig.DiscoveryConfig) string {
	accessGraphMatchers := 0
	if dc.Spec.AccessGraph != nil {
		accessGraphMatchers = len(dc.Spec.AccessGraph.AWS)
	}
	parts := make([]string, 0, 5)
	appendNonZero := func(label string, value int) {
		if value > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", label, value))
		}
	}
	appendNonZero("aws", len(dc.Spec.AWS))
	appendNonZero("azure", len(dc.Spec.Azure))
	appendNonZero("gcp", len(dc.Spec.GCP))
	appendNonZero("kube", len(dc.Spec.Kube))
	appendNonZero("ag", accessGraphMatchers)
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, " ")
}

func countDiscoveryGroups(dcs []*discoveryconfig.DiscoveryConfig) int {
	groups := map[string]struct{}{}
	for _, dc := range dcs {
		groups[dc.GetDiscoveryGroup()] = struct{}{}
	}
	return len(groups)
}

func addDiscoveredSummary(total *resourcesAggregate, summary *discoveryconfigv1.ResourcesDiscoveredSummary) {
	if summary == nil {
		return
	}
	total.Found += summary.GetFound()
	total.Enrolled += summary.GetEnrolled()
	total.Failed += summary.GetFailed()
}

func awaitingJoin(stats resourcesAggregate) uint64 {
	joinedOrFailed := stats.Enrolled + stats.Failed
	if joinedOrFailed >= stats.Found {
		return 0
	}
	return stats.Found - joinedOrFailed
}

type countRow struct {
	Key   string
	Count int
}

func countRows(counts map[string]int) []countRow {
	keys := mapKeys(counts)
	rows := make([]countRow, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, countRow{Key: key, Count: counts[key]})
	}
	slices.SortFunc(rows, func(a, b countRow) int {
		if a.Count != b.Count {
			if a.Count > b.Count {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.Key, b.Key)
	})
	return rows
}

type ssmRunRecord struct {
	EventTime     string `json:"event_time"`
	Code          string `json:"code"`
	InstanceID    string `json:"instance_id"`
	Status        string `json:"status"`
	ExitCode      string `json:"exit_code"`
	AccountID     string `json:"account_id"`
	Region        string `json:"region"`
	CommandID     string `json:"command_id"`
	InvocationURL string `json:"invocation_url"`
	Stdout        string `json:"stdout,omitempty"`
	Stderr        string `json:"stderr,omitempty"`

	parsedEventTime time.Time
}

type ssmRunEventFilters struct {
	InstanceID string
}

func parseSSMRunEvents(eventList []apievents.AuditEvent, filters ssmRunEventFilters) []ssmRunRecord {
	records := make([]ssmRunRecord, 0, len(eventList))
	for _, event := range eventList {
		run, ok := event.(*apievents.SSMRun)
		if !ok {
			continue
		}

		record := ssmRunRecord{
			Code:          run.Code,
			InstanceID:    run.InstanceID,
			Status:        run.Status,
			ExitCode:      fmt.Sprintf("%d", run.ExitCode),
			AccountID:     run.AccountID,
			Region:        run.Region,
			CommandID:     run.CommandID,
			InvocationURL: run.InvocationURL,
			Stdout:        run.StandardOutput,
			Stderr:        run.StandardError,
		}
		if !run.Time.IsZero() {
			record.parsedEventTime = run.Time.UTC()
			record.EventTime = record.parsedEventTime.Format(time.RFC3339Nano)
		}

		if filters.InstanceID != "" && !strings.EqualFold(strings.TrimSpace(record.InstanceID), strings.TrimSpace(filters.InstanceID)) {
			continue
		}
		records = append(records, record)
	}

	sortSSMRunRecords(records)
	return records
}

// compareTimeDesc compares two times in descending order (newest first).
// Zero times are considered equal to each other and older than any non-zero time.
func compareTimeDesc(a, b time.Time) int {
	switch {
	case a.IsZero() && b.IsZero():
		return 0
	case a.IsZero():
		return 1
	case b.IsZero():
		return -1
	case a.After(b):
		return -1
	case b.After(a):
		return 1
	default:
		return 0
	}
}

func sortSSMRunRecords(records []ssmRunRecord) {
	slices.SortFunc(records, func(a, b ssmRunRecord) int {
		if c := compareTimeDesc(a.parsedEventTime, b.parsedEventTime); c != 0 {
			return c
		}
		return cmp.Compare(b.EventTime, a.EventTime)
	})
}

func parseAuditEventTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}

	withTZLayouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
	}
	for _, layout := range withTZLayouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.UTC(), true
		}
	}

	withoutTZLayouts := []string{
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05.999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range withoutTZLayouts {
		if parsed, err := time.ParseInLocation(layout, raw, time.UTC); err == nil {
			return parsed.UTC(), true
		}
	}

	return time.Time{}, false
}

type ssmRunAnalysis struct {
	Total            int            `json:"total"`
	Success          int            `json:"success"`
	Failed           int            `json:"failed"`
	ByInstance       map[string]int `json:"by_instance"`
	FailedByInstance map[string]int `json:"failed_by_instance"`
}

func analyzeSSMRuns(records []ssmRunRecord) ssmRunAnalysis {
	analysis := ssmRunAnalysis{
		Total:            len(records),
		ByInstance:       map[string]int{},
		FailedByInstance: map[string]int{},
	}

	for _, record := range records {
		instanceID := cmp.Or(strings.TrimSpace(record.InstanceID), "unknown")
		analysis.ByInstance[instanceID]++

		if isSSMRunFailure(record) {
			analysis.Failed++
			analysis.FailedByInstance[instanceID]++
		} else {
			analysis.Success++
		}
	}

	return analysis
}

func isSSMRunFailure(record ssmRunRecord) bool {
	if strings.EqualFold(strings.TrimSpace(record.Code), "TDS00W") {
		return true
	}
	status := strings.TrimSpace(record.Status)
	if status == "" {
		return false
	}
	return !strings.EqualFold(status, "Success")
}

type ssmVMGroup struct {
	InstanceID         string         `json:"instance_id"`
	MostRecent         ssmRunRecord   `json:"most_recent"`
	MostRecentFailed   bool           `json:"most_recent_failed"`
	TotalRuns          int            `json:"total_runs"`
	FailedRuns         int            `json:"failed_runs"`
	SuccessRuns        int            `json:"success_runs"`
	StatusByMostRecent map[string]int `json:"status_by_most_recent,omitempty"`
	Runs               []ssmRunRecord `json:"runs"`
}

func groupSSMRunsByVM(records []ssmRunRecord) []ssmVMGroup {
	byVM := map[string][]ssmRunRecord{}
	for _, record := range records {
		instanceID := cmp.Or(strings.TrimSpace(record.InstanceID), "unknown")
		byVM[instanceID] = append(byVM[instanceID], record)
	}

	groups := make([]ssmVMGroup, 0, len(byVM))
	for instanceID, vmRuns := range byVM {
		sortSSMRunRecords(vmRuns)

		group := ssmVMGroup{
			InstanceID:       instanceID,
			MostRecent:       vmRuns[0],
			MostRecentFailed: isSSMRunFailure(vmRuns[0]),
			TotalRuns:        len(vmRuns),
			Runs:             vmRuns,
		}
		for _, run := range vmRuns {
			if isSSMRunFailure(run) {
				group.FailedRuns++
			} else {
				group.SuccessRuns++
			}
		}
		groups = append(groups, group)
	}

	slices.SortFunc(groups, func(a, b ssmVMGroup) int {
		if c := compareTimeDesc(a.MostRecent.parsedEventTime, b.MostRecent.parsedEventTime); c != 0 {
			return c
		}
		return cmp.Compare(a.InstanceID, b.InstanceID)
	})

	return groups
}

func selectFailingVMGroups(groups []ssmVMGroup, limit int) []ssmVMGroup {
	out := make([]ssmVMGroup, 0, len(groups))
	for _, group := range groups {
		if !group.MostRecentFailed {
			continue
		}
		out = append(out, group)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

type ssmRunHistoryRow struct {
	Timestamp string `json:"timestamp"`
	Result    string `json:"result"`
	CommandID string `json:"command_id"`
	ExitCode  string `json:"exit_code"`
	Output    string `json:"output,omitempty"`
}

func buildVMHistoryRows(group ssmVMGroup, showAll bool) []ssmRunHistoryRow {
	runs := group.Runs
	if !showAll && len(runs) > 1 {
		runs = runs[:1]
	}

	rows := make([]ssmRunHistoryRow, 0, len(runs))
	for _, run := range runs {
		timestamp := cmp.Or(formatMaybeParsedTime(run.parsedEventTime), run.EventTime)
		rows = append(rows, ssmRunHistoryRow{
			Timestamp: timestamp,
			Result:    cmp.Or(run.Status, run.Code),
			CommandID: run.CommandID,
			ExitCode:  run.ExitCode,
			Output:    combineOutput(run.Stdout, run.Stderr),
		})
	}
	return rows
}

type ssmRunsOutput struct {
	Window       string       `json:"window"`
	Query        string       `json:"query"`
	FetchLimit   int          `json:"fetch_limit"`
	LimitReached bool         `json:"limit_reached"`
	TotalRuns    int          `json:"total_runs"`
	SuccessRuns  int          `json:"success_runs"`
	FailedRuns   int          `json:"failed_runs"`
	TotalVMs     int          `json:"total_vms"`
	FailingVMs   int          `json:"failing_vms"`
	DisplayedVMs int          `json:"displayed_vms"`
	VMPage       pageInfo     `json:"vm_page"`
	VMs          []ssmVMGroup `json:"vms"`
}

// Integration types and functions.

type integrationListItem struct {
	Name         string `json:"name" yaml:"name"`
	Type         string `json:"type" yaml:"type"`
	Found        uint64 `json:"found" yaml:"found"`
	Enrolled     uint64 `json:"enrolled" yaml:"enrolled"`
	Failed       uint64 `json:"failed" yaml:"failed"`
	AwaitingJoin uint64 `json:"awaiting_join" yaml:"awaiting_join"`
	OpenTasks    int    `json:"open_tasks" yaml:"open_tasks"`
}

type integrationListOutput struct {
	Total int                   `json:"total" yaml:"total"`
	Items []integrationListItem `json:"items" yaml:"items"`
}

type resourceTypeStatsRow struct {
	ResourceType string `json:"resource_type" yaml:"resource_type"`
	Found        uint64 `json:"found" yaml:"found"`
	Enrolled     uint64 `json:"enrolled" yaml:"enrolled"`
	Failed       uint64 `json:"failed" yaml:"failed"`
}

type integrationDetail struct {
	Name              string                `json:"name" yaml:"name"`
	Type              string                `json:"type" yaml:"type"`
	Credentials       map[string]string     `json:"credentials" yaml:"credentials"`
	ResourceTypeStats []resourceTypeStatsRow `json:"resource_type_stats" yaml:"resource_type_stats"`
	DiscoveryConfigs  []configStatus        `json:"discovery_configs" yaml:"discovery_configs"`
	OpenTasks         []taskListItem        `json:"open_tasks" yaml:"open_tasks"`
}

func listIntegrations(ctx context.Context, client *authclient.Client) ([]types.Integration, error) {
	items, err := stream.Collect(clientutils.Resources(ctx, client.ListIntegrations))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return items, nil
}

func friendlyIntegrationType(subKind string) string {
	switch subKind {
	case types.IntegrationSubKindAWSOIDC:
		return "AWS OIDC"
	case types.IntegrationSubKindAzureOIDC:
		return "Azure OIDC"
	case types.IntegrationSubKindGitHub:
		return "GitHub"
	case types.IntegrationSubKindAWSRolesAnywhere:
		return "AWS Roles Anywhere"
	default:
		if strings.TrimSpace(subKind) == "" {
			return "Unknown"
		}
		return subKind
	}
}

func integrationCredentialDetails(ig types.Integration) map[string]string {
	creds := map[string]string{}
	switch ig.GetSubKind() {
	case types.IntegrationSubKindAWSOIDC:
		if spec := ig.GetAWSOIDCIntegrationSpec(); spec != nil {
			creds["Role ARN"] = spec.RoleARN
		}
	case types.IntegrationSubKindAzureOIDC:
		if spec := ig.GetAzureOIDCIntegrationSpec(); spec != nil {
			creds["Tenant ID"] = spec.TenantID
			creds["Client ID"] = spec.ClientID
		}
	case types.IntegrationSubKindGitHub:
		if spec := ig.GetGitHubIntegrationSpec(); spec != nil {
			creds["Organization"] = spec.Organization
		}
	case types.IntegrationSubKindAWSRolesAnywhere:
		if spec := ig.GetAWSRolesAnywhereIntegrationSpec(); spec != nil {
			if spec.ProfileSyncConfig != nil {
				creds["Role ARN"] = spec.ProfileSyncConfig.RoleARN
			}
		}
	}
	return creds
}

func buildIntegrationStatsMap(dcs []*discoveryconfig.DiscoveryConfig) map[string]resourcesAggregate {
	statsMap := map[string]resourcesAggregate{}
	for _, dc := range dcs {
		for integrationName, integrationSummary := range dc.Status.IntegrationDiscoveredResources {
			agg := statsMap[integrationName]
			addDiscoveredSummary(&agg, integrationSummary.GetAwsEc2())
			addDiscoveredSummary(&agg, integrationSummary.GetAwsEks())
			addDiscoveredSummary(&agg, integrationSummary.GetAwsRds())
			addDiscoveredSummary(&agg, integrationSummary.GetAzureVms())
			statsMap[integrationName] = agg
		}
	}
	return statsMap
}

func countTasksByIntegration(tasks []*usertasksv1.UserTask) map[string]int {
	counts := map[string]int{}
	for _, task := range tasks {
		counts[task.GetSpec().GetIntegration()]++
	}
	return counts
}

func toIntegrationListItems(integrations []types.Integration, statsMap map[string]resourcesAggregate, taskCountMap map[string]int) []integrationListItem {
	items := make([]integrationListItem, 0, len(integrations))
	for _, ig := range integrations {
		name := ig.GetName()
		stats := statsMap[name]
		items = append(items, integrationListItem{
			Name:         name,
			Type:         friendlyIntegrationType(ig.GetSubKind()),
			Found:        stats.Found,
			Enrolled:     stats.Enrolled,
			Failed:       stats.Failed,
			AwaitingJoin: awaitingJoin(stats),
			OpenTasks:    taskCountMap[name],
		})
	}
	slices.SortFunc(items, func(a, b integrationListItem) int {
		if a.Failed != b.Failed {
			if a.Failed > b.Failed {
				return -1
			}
			return 1
		}
		if a.Found != b.Found {
			if a.Found > b.Found {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.Name, b.Name)
	})
	return items
}

func perResourceTypeStats(dcs []*discoveryconfig.DiscoveryConfig, integrationName string) []resourceTypeStatsRow {
	type key struct{ name string }
	statsMap := map[key]resourceTypeStatsRow{}
	addRow := func(resourceType string, summary *discoveryconfigv1.ResourcesDiscoveredSummary) {
		if summary == nil {
			return
		}
		if summary.GetFound() == 0 && summary.GetEnrolled() == 0 && summary.GetFailed() == 0 {
			return
		}
		k := key{name: resourceType}
		row := statsMap[k]
		row.ResourceType = resourceType
		row.Found += summary.GetFound()
		row.Enrolled += summary.GetEnrolled()
		row.Failed += summary.GetFailed()
		statsMap[k] = row
	}
	for _, dc := range dcs {
		integrationSummary, ok := dc.Status.IntegrationDiscoveredResources[integrationName]
		if !ok {
			continue
		}
		addRow("EC2", integrationSummary.GetAwsEc2())
		addRow("EKS", integrationSummary.GetAwsEks())
		addRow("RDS", integrationSummary.GetAwsRds())
		addRow("Azure VM", integrationSummary.GetAzureVms())
	}
	rows := make([]resourceTypeStatsRow, 0, len(statsMap))
	for _, row := range statsMap {
		rows = append(rows, row)
	}
	slices.SortFunc(rows, func(a, b resourceTypeStatsRow) int {
		return cmp.Compare(a.ResourceType, b.ResourceType)
	})
	return rows
}

func associatedDiscoveryConfigs(dcs []*discoveryconfig.DiscoveryConfig, integrationName string) []configStatus {
	configs := make([]configStatus, 0)
	for _, dc := range dcs {
		if _, ok := dc.Status.IntegrationDiscoveredResources[integrationName]; !ok {
			continue
		}
		configs = append(configs, configStatus{
			Name:       dc.GetName(),
			Group:      dc.GetDiscoveryGroup(),
			State:      cmp.Or(strings.TrimSpace(dc.Status.State), "UNKNOWN"),
			Matchers:   configMatchersSummary(dc),
			Discovered: dc.Status.DiscoveredResources,
			LastSync:   dc.Status.LastSyncTime.UTC(),
		})
	}
	slices.SortFunc(configs, func(a, b configStatus) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return configs
}

func buildIntegrationDetail(ig types.Integration, dcs []*discoveryconfig.DiscoveryConfig, tasks []*usertasksv1.UserTask) integrationDetail {
	name := ig.GetName()
	return integrationDetail{
		Name:              name,
		Type:              friendlyIntegrationType(ig.GetSubKind()),
		Credentials:       integrationCredentialDetails(ig),
		ResourceTypeStats: perResourceTypeStats(dcs, name),
		DiscoveryConfigs:  associatedDiscoveryConfigs(dcs, name),
		OpenTasks:         toTaskListItems(tasks),
	}
}

// Instance join types and functions.

type joinRecord struct {
	EventTime  string `json:"event_time"`
	Code       string `json:"code"`
	HostID     string `json:"host_id"`
	NodeName   string `json:"node_name"`
	Role       string `json:"role"`
	Method     string `json:"method"`
	TokenName  string `json:"token_name,omitempty"`
	InstanceID string `json:"instance_id,omitempty"`
	AccountID  string `json:"account_id,omitempty"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`

	parsedEventTime time.Time
}

type joinEventFilters struct {
	HostID string
}

type joinGroup struct {
	HostID           string       `json:"host_id"`
	NodeName         string       `json:"node_name"`
	MostRecent       joinRecord   `json:"most_recent"`
	MostRecentFailed bool         `json:"most_recent_failed"`
	TotalJoins       int          `json:"total_joins"`
	FailedJoins      int          `json:"failed_joins"`
	SuccessJoins     int          `json:"success_joins"`
	Joins            []joinRecord `json:"joins"`
}

type joinAnalysis struct {
	Total        int            `json:"total"`
	Success      int            `json:"success"`
	Failed       int            `json:"failed"`
	ByHost       map[string]int `json:"by_host"`
	FailedByHost map[string]int `json:"failed_by_host"`
}

type joinsOutput struct {
	Window         string      `json:"window"`
	Query          string      `json:"query"`
	FetchLimit     int         `json:"fetch_limit"`
	LimitReached   bool        `json:"limit_reached"`
	TotalJoins     int         `json:"total_joins"`
	SuccessJoins   int         `json:"success_joins"`
	FailedJoins    int         `json:"failed_joins"`
	TotalHosts     int         `json:"total_hosts"`
	FailingHosts   int         `json:"failing_hosts"`
	DisplayedHosts int         `json:"displayed_hosts"`
	HostPage       pageInfo    `json:"host_page"`
	Hosts          []joinGroup `json:"hosts"`
}

type joinHistoryRow struct {
	Timestamp  string `json:"timestamp"`
	Result     string `json:"result"`
	Method     string `json:"method"`
	Role       string `json:"role"`
	TokenName  string `json:"token_name,omitempty"`
	InstanceID string `json:"instance_id,omitempty"`
	AccountID  string `json:"account_id,omitempty"`
}

// getJoinAttributeString safely extracts a string value from an InstanceJoin's
// Attributes protobuf Struct field.
func getJoinAttributeString(join *apievents.InstanceJoin, key string) string {
	if join.Attributes == nil {
		return ""
	}
	v, ok := join.Attributes.Fields[key]
	if !ok || v == nil {
		return ""
	}
	return v.GetStringValue()
}

// extractEC2InstanceID extracts the EC2 instance ID from an IAM role ARN.
// For example: "arn:aws:sts::123456:assumed-role/role-name/i-030a87f439b67b43a"
// returns "i-030a87f439b67b43a".
func extractEC2InstanceID(arn string) string {
	// The instance ID is the last segment after the final "/".
	lastSlash := strings.LastIndex(arn, "/")
	if lastSlash < 0 || lastSlash+1 >= len(arn) {
		return ""
	}
	candidate := arn[lastSlash+1:]
	if strings.HasPrefix(candidate, "i-") {
		return candidate
	}
	return ""
}

func parseInstanceJoinEvents(eventList []apievents.AuditEvent, filters joinEventFilters) []joinRecord {
	records := make([]joinRecord, 0, len(eventList))
	for _, event := range eventList {
		join, ok := event.(*apievents.InstanceJoin)
		if !ok {
			continue
		}

		record := joinRecord{
			Code:       join.Code,
			HostID:     join.HostID,
			NodeName:   join.NodeName,
			Role:       join.Role,
			Method:     join.Method,
			TokenName:  sanitizeTokenName(join.TokenName, join.Method),
			InstanceID: extractEC2InstanceID(getJoinAttributeString(join, "Arn")),
			AccountID:  getJoinAttributeString(join, "Account"),
			Success:    join.Status.Success,
			Error:      join.Status.Error,
		}
		if !join.Time.IsZero() {
			record.parsedEventTime = join.Time.UTC()
			record.EventTime = record.parsedEventTime.Format(time.RFC3339Nano)
		}

		if filters.HostID != "" {
			// Compare against the group key so "show" works with the
			// same identifiers displayed by "ls" (e.g. "unknown (10.0.0.1)").
			if !strings.EqualFold(joinGroupKey(record), strings.TrimSpace(filters.HostID)) {
				continue
			}
		}
		records = append(records, record)
	}

	sortJoinRecords(records)
	return records
}

func sortJoinRecords(records []joinRecord) {
	slices.SortFunc(records, func(a, b joinRecord) int {
		if c := compareTimeDesc(a.parsedEventTime, b.parsedEventTime); c != 0 {
			return c
		}
		return cmp.Compare(b.EventTime, a.EventTime)
	})
}

func isJoinFailure(record joinRecord) bool {
	return record.Code == libevents.InstanceJoinFailureCode || !record.Success
}

// sanitizeTokenName masks the token name when the join method is "token"
// since that's a secret value.
func sanitizeTokenName(tokenName, method string) string {
	if strings.EqualFold(method, "token") && tokenName != "" {
		return "********"
	}
	return tokenName
}

// joinGroupKey returns a stable key for grouping join records by host.
// When HostID is empty (failed joins before identification), returns "unknown".
func joinGroupKey(record joinRecord) string {
	hostID := strings.TrimSpace(record.HostID)
	if hostID != "" {
		return hostID
	}
	return "unknown"
}

func isUnknownHost(hostID string) bool {
	return hostID == "unknown"
}

func analyzeInstanceJoins(records []joinRecord) joinAnalysis {
	analysis := joinAnalysis{
		Total:        len(records),
		ByHost:       map[string]int{},
		FailedByHost: map[string]int{},
	}

	for _, record := range records {
		key := joinGroupKey(record)
		analysis.ByHost[key]++

		if isJoinFailure(record) {
			analysis.Failed++
			analysis.FailedByHost[key]++
		} else {
			analysis.Success++
		}
	}

	return analysis
}

func groupJoinsByHost(records []joinRecord) []joinGroup {
	byHost := map[string][]joinRecord{}
	for _, record := range records {
		key := joinGroupKey(record)
		byHost[key] = append(byHost[key], record)
	}

	groups := make([]joinGroup, 0, len(byHost))
	for hostID, hostJoins := range byHost {
		sortJoinRecords(hostJoins)

		group := joinGroup{
			HostID:           hostID,
			NodeName:         hostJoins[0].NodeName,
			MostRecent:       hostJoins[0],
			MostRecentFailed: isJoinFailure(hostJoins[0]),
			TotalJoins:       len(hostJoins),
			Joins:            hostJoins,
		}
		for _, join := range hostJoins {
			if isJoinFailure(join) {
				group.FailedJoins++
			} else {
				group.SuccessJoins++
			}
		}
		groups = append(groups, group)
	}

	slices.SortFunc(groups, func(a, b joinGroup) int {
		if c := compareTimeDesc(a.MostRecent.parsedEventTime, b.MostRecent.parsedEventTime); c != 0 {
			return c
		}
		return cmp.Compare(a.HostID, b.HostID)
	})

	return groups
}

func selectFailingJoinGroups(groups []joinGroup, limit int) []joinGroup {
	out := make([]joinGroup, 0, len(groups))
	for _, group := range groups {
		if !group.MostRecentFailed {
			continue
		}
		out = append(out, group)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func filterOutUnknownJoinGroups(groups []joinGroup) []joinGroup {
	out := make([]joinGroup, 0, len(groups))
	for _, group := range groups {
		if isUnknownHost(group.HostID) {
			continue
		}
		out = append(out, group)
	}
	return out
}

// inventoryHostState represents where a host is in the discovery pipeline.
type inventoryHostState string

const (
	inventoryStateOnline       inventoryHostState = "Online"
	inventoryStateOffline      inventoryHostState = "Offline"
	inventoryStateJoinFailed   inventoryHostState = "Join Failed"
	inventoryStateSSMFailed    inventoryHostState = "SSM Failed"
	inventoryStateSSMAttempted inventoryHostState = "SSM Attempted"
	inventoryStateJoinedOnly   inventoryHostState = "Joined Only"
)

type inventoryHost struct {
	// DisplayID is the preferred unified identifier: instance ID when
	// available, otherwise the Teleport node UUID.
	DisplayID  string             `json:"display_id"`
	HostID     string             `json:"host_id"`
	InstanceID string             `json:"instance_id,omitempty"`
	AccountID  string             `json:"account_id,omitempty"`
	NodeName   string             `json:"node_name"`
	State      inventoryHostState `json:"state"`
	Method     string             `json:"method,omitempty"`

	LastSSMRun time.Time `json:"last_ssm_run,omitempty"`
	LastJoin   time.Time `json:"last_join,omitempty"`
	LastSeen   time.Time `json:"last_seen,omitempty"`
	IsOnline   bool      `json:"is_online"`

	SSMRuns    int `json:"ssm_runs"`
	SSMSuccess int `json:"ssm_success"`
	SSMFailed  int `json:"ssm_failed"`
	Joins      int `json:"joins"`
	JoinSuccess int `json:"join_success"`
	JoinFailed  int `json:"join_failed"`

	SSMRecords  []ssmRunRecord `json:"ssm_records,omitempty"`
	JoinRecords []joinRecord   `json:"join_records,omitempty"`

	// mostRecentActivity is the latest timestamp across all sources, used for sorting.
	mostRecentActivity time.Time
}

type inventoryOutput struct {
	Window         string          `json:"window"`
	TotalHosts     int             `json:"total_hosts"`
	OnlineHosts    int             `json:"online_hosts"`
	OfflineHosts   int             `json:"offline_hosts"`
	FailedHosts    int             `json:"failed_hosts"`
	DisplayedHosts int             `json:"displayed_hosts"`
	HostPage       pageInfo        `json:"host_page"`
	Hosts          []inventoryHost `json:"hosts"`
}

func buildInventoryHosts(
	nodes []types.Server,
	ssmRecords []ssmRunRecord,
	joinRecords []joinRecord,
) []inventoryHost {
	type hostData struct {
		nodeName   string
		instanceID string
		accountID  string
		isOnline   bool
		lastSeen   time.Time
		method     string
		ssmRuns    []ssmRunRecord
		joinRecs   []joinRecord
	}

	hosts := make(map[string]*hostData)

	getOrCreate := func(id string) *hostData {
		if h, ok := hosts[id]; ok {
			return h
		}
		h := &hostData{}
		hosts[id] = h
		return h
	}

	// 1. Nodes (currently online). Extract AWS instance/account IDs from
	// labels (teleport.dev/instance-id, teleport.dev/account-id) set during
	// EC2 discovery so that joined nodes show consistent AWS-native names.
	for _, node := range nodes {
		id := node.GetName()
		if id == "" {
			continue
		}
		h := getOrCreate(id)
		h.isOnline = true
		h.lastSeen = node.Expiry()
		if h.nodeName == "" {
			h.nodeName = node.GetHostname()
		}
		if awsID := node.GetAWSInstanceID(); awsID != "" && h.instanceID == "" {
			h.instanceID = awsID
		}
		if awsAcct := node.GetAWSAccountID(); awsAcct != "" && h.accountID == "" {
			h.accountID = awsAcct
		}
	}

	// 2. SSM runs — keyed by EC2 instance ID (e.g. i-030a87f439b67b43a).
	for _, rec := range ssmRecords {
		id := rec.InstanceID
		if id == "" {
			continue
		}
		h := getOrCreate(id)
		h.ssmRuns = append(h.ssmRuns, rec)
		if h.instanceID == "" {
			h.instanceID = id
		}
		if h.accountID == "" {
			h.accountID = rec.AccountID
		}
	}

	// 3. Join events — use InstanceID (from ARN) when available for
	// correlation with SSM runs; otherwise fall back to HostID.
	for _, rec := range joinRecords {
		var id string
		if rec.InstanceID != "" {
			id = rec.InstanceID
		} else {
			id = joinGroupKey(rec)
		}
		if id == "" {
			continue
		}
		h := getOrCreate(id)
		h.joinRecs = append(h.joinRecs, rec)
		if h.nodeName == "" && rec.NodeName != "" {
			h.nodeName = rec.NodeName
		}
		if h.method == "" && rec.Method != "" {
			h.method = rec.Method
		}
		if h.instanceID == "" && rec.InstanceID != "" {
			h.instanceID = rec.InstanceID
		}
		if h.accountID == "" && rec.AccountID != "" {
			h.accountID = rec.AccountID
		}
	}

	// 4. Merge duplicate entries. An online node (keyed by UUID) may have
	// its AWS instance ID from labels, while SSM runs and join events are
	// keyed by that same instance ID. Merge instance-ID entries into the
	// corresponding UUID entry so the host appears once.
	//
	// Build a reverse map: instanceID → UUID key for nodes that have one.
	instanceToUUID := make(map[string]string)
	for key, data := range hosts {
		if data.instanceID != "" && data.isOnline {
			instanceToUUID[data.instanceID] = key
		}
	}
	for instanceID, data := range hosts {
		if !strings.HasPrefix(instanceID, "i-") {
			continue
		}
		// Find a UUID-keyed node entry to merge into. First check
		// the reverse map (node labels), then fall back to join records.
		targetKey := ""
		if uuidKey, ok := instanceToUUID[instanceID]; ok && uuidKey != instanceID {
			targetKey = uuidKey
		} else {
			for _, rec := range data.joinRecs {
				hostID := strings.TrimSpace(rec.HostID)
				if hostID == "" || hostID == instanceID {
					continue
				}
				if _, ok := hosts[hostID]; ok {
					targetKey = hostID
					break
				}
			}
		}
		if targetKey == "" {
			continue
		}
		nodeEntry := hosts[targetKey]
		nodeEntry.ssmRuns = append(nodeEntry.ssmRuns, data.ssmRuns...)
		nodeEntry.joinRecs = append(nodeEntry.joinRecs, data.joinRecs...)
		if nodeEntry.instanceID == "" {
			nodeEntry.instanceID = data.instanceID
		}
		if nodeEntry.accountID == "" {
			nodeEntry.accountID = data.accountID
		}
		if nodeEntry.method == "" && data.method != "" {
			nodeEntry.method = data.method
		}
		if nodeEntry.nodeName == "" && data.nodeName != "" {
			nodeEntry.nodeName = data.nodeName
		}
		delete(hosts, instanceID)
	}

	// Build output
	result := make([]inventoryHost, 0, len(hosts))
	for hostID, data := range hosts {
		displayID := cmp.Or(data.instanceID, hostID)
		ih := inventoryHost{
			DisplayID:   displayID,
			HostID:      hostID,
			InstanceID:  data.instanceID,
			AccountID:   data.accountID,
			NodeName:    data.nodeName,
			IsOnline:    data.isOnline,
			LastSeen:    data.lastSeen,
			Method:      data.method,
			SSMRecords:  data.ssmRuns,
			JoinRecords: data.joinRecs,
		}

		// SSM stats
		ih.SSMRuns = len(data.ssmRuns)
		for _, r := range data.ssmRuns {
			if isSSMRunFailure(r) {
				ih.SSMFailed++
			} else {
				ih.SSMSuccess++
			}
		}
		if len(data.ssmRuns) > 0 {
			ih.LastSSMRun = data.ssmRuns[0].parsedEventTime // already sorted desc
		}

		// Join stats
		ih.Joins = len(data.joinRecs)
		hasSuccessfulJoin := false
		for _, r := range data.joinRecs {
			if isJoinFailure(r) {
				ih.JoinFailed++
			} else {
				ih.JoinSuccess++
				hasSuccessfulJoin = true
			}
		}
		if len(data.joinRecs) > 0 {
			ih.LastJoin = data.joinRecs[0].parsedEventTime
			if ih.Method == "" {
				ih.Method = data.joinRecs[0].Method
			}
		}

		// Derive state
		ih.State = deriveInventoryState(data.isOnline, hasSuccessfulJoin, data.joinRecs, data.ssmRuns)

		// Most recent activity for sorting
		ih.mostRecentActivity = maxTime(ih.LastSeen, ih.LastSSMRun, ih.LastJoin)

		result = append(result, ih)
	}

	slices.SortFunc(result, func(a, b inventoryHost) int {
		if c := compareTimeDesc(a.mostRecentActivity, b.mostRecentActivity); c != 0 {
			return c
		}
		return cmp.Compare(a.HostID, b.HostID)
	})

	return result
}

func deriveInventoryState(isOnline, hasSuccessfulJoin bool, joinRecs []joinRecord, ssmRuns []ssmRunRecord) inventoryHostState {
	if isOnline {
		return inventoryStateOnline
	}
	if len(joinRecs) > 0 {
		if hasSuccessfulJoin {
			return inventoryStateOffline
		}
		return inventoryStateJoinFailed
	}
	if len(ssmRuns) > 0 {
		if isSSMRunFailure(ssmRuns[0]) {
			return inventoryStateSSMFailed
		}
		return inventoryStateSSMAttempted
	}
	return inventoryStateJoinedOnly
}

func maxTime(times ...time.Time) time.Time {
	var best time.Time
	for _, t := range times {
		if t.After(best) {
			best = t
		}
	}
	return best
}

func buildJoinHistoryRows(group joinGroup, showAll bool) []joinHistoryRow {
	joins := group.Joins
	if !showAll && len(joins) > 1 {
		joins = joins[:1]
	}

	rows := make([]joinHistoryRow, 0, len(joins))
	for _, join := range joins {
		timestamp := cmp.Or(formatMaybeParsedTime(join.parsedEventTime), join.EventTime)
		result := "success"
		if isJoinFailure(join) {
			result = cmp.Or(join.Error, "failed")
		}
		rows = append(rows, joinHistoryRow{
			Timestamp:  timestamp,
			Result:     result,
			Method:     join.Method,
			Role:       join.Role,
			TokenName:  join.TokenName,
			InstanceID: join.InstanceID,
			AccountID:  join.AccountID,
		})
	}
	return rows
}
