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
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	apievents "github.com/gravitational/teleport/api/types/events"
	usertasksapi "github.com/gravitational/teleport/api/types/usertasks"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
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
	case usertasksapi.TaskTypeDiscoverAzureVM:
		return "Azure VM"
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
	case usertasksapi.TaskTypeDiscoverAzureVM:
		return len(spec.GetDiscoverAzureVm().GetInstances())
	default:
		return 0
	}
}

type statusSummary struct {
	GeneratedAt              time.Time                     `json:"generated_at"`
	FilteredState            string                        `json:"filtered_state"`
	FilteredIntegration      string                        `json:"filtered_integration,omitempty"`
	DiscoveryConfigCount     int                           `json:"discovery_config_count"`
	DiscoveryGroupCount      int                           `json:"discovery_group_count"`
	UserTasks                []taskListItem                `json:"user_tasks"`
	DiscoveryConfigs         []configStatus                `json:"discovery_configs"`
	TotalTasks               int                           `json:"total_tasks"`
	OpenTasks                int                           `json:"open_tasks"`
	ResolvedTasks            int                           `json:"resolved_tasks"`
	FilteredTaskCount        int                           `json:"filtered_task_count"`
	TasksByType              map[string]int                `json:"tasks_by_type"`
	TasksByIssue             map[string]int                `json:"tasks_by_issue"`
	TasksByIntegration       map[string]int                `json:"tasks_by_integration"`
	IntegrationResourceStats map[string]resourcesAggregate `json:"integration_resource_stats"`
}

type configStatus struct {
	Name       string    `json:"name"`
	Group      string    `json:"group"`
	State      string    `json:"state"`
	Matchers   string    `json:"matchers"`
	Discovered uint64    `json:"discovered"`
	LastSync   time.Time `json:"last_sync"`
}

type resourcesAggregate struct {
	Found    uint64 `json:"found"`
	Enrolled uint64 `json:"enrolled"`
	Failed   uint64 `json:"failed"`
}

func makeStatusSummary(allTasks, filteredTasks []*usertasksv1.UserTask, dcs []*discoveryconfig.DiscoveryConfig, state, integration string) statusSummary {
	summary := statusSummary{
		GeneratedAt:              time.Now().UTC(),
		FilteredState:            cmp.Or(state, "ALL"),
		FilteredIntegration:      integration,
		DiscoveryConfigCount:     len(dcs),
		DiscoveryGroupCount:      countDiscoveryGroups(dcs),
		UserTasks:                make([]taskListItem, 0, len(filteredTasks)),
		DiscoveryConfigs:         make([]configStatus, 0, len(dcs)),
		TotalTasks:               len(allTasks),
		FilteredTaskCount:        len(filteredTasks),
		TasksByType:              map[string]int{},
		TasksByIssue:             map[string]int{},
		TasksByIntegration:       map[string]int{},
		IntegrationResourceStats: map[string]resourcesAggregate{},
	}

	for _, task := range allTasks {
		switch task.GetSpec().GetState() {
		case usertasksapi.TaskStateOpen:
			summary.OpenTasks++
		case usertasksapi.TaskStateResolved:
			summary.ResolvedTasks++
		}
	}

	for _, task := range filteredTasks {
		summary.TasksByType[task.GetSpec().GetTaskType()]++
		summary.TasksByIssue[task.GetSpec().GetIssueType()]++
		summary.TasksByIntegration[task.GetSpec().GetIntegration()]++
	}
	summary.UserTasks = toTaskListItems(filteredTasks)
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

	for _, dc := range dcs {
		for integrationName, integrationSummary := range dc.Status.IntegrationDiscoveredResources {
			key := integrationName
			agg := summary.IntegrationResourceStats[key]
			addDiscoveredSummary(&agg, integrationSummary.GetAwsEc2())
			addDiscoveredSummary(&agg, integrationSummary.GetAwsEks())
			addDiscoveredSummary(&agg, integrationSummary.GetAwsRds())
			addDiscoveredSummary(&agg, integrationSummary.GetAzureVms())
			summary.IntegrationResourceStats[key] = agg
		}
	}

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

type integrationStatsRow struct {
	Integration string
	Found       uint64
	Enrolled    uint64
	Failed      uint64
}

func integrationStatsRows(stats map[string]resourcesAggregate) []integrationStatsRow {
	names := mapKeys(stats)
	rows := make([]integrationStatsRow, 0, len(names))
	for _, name := range names {
		value := stats[name]
		rows = append(rows, integrationStatsRow{
			Integration: name,
			Found:       value.Found,
			Enrolled:    value.Enrolled,
			Failed:      value.Failed,
		})
	}
	slices.SortFunc(rows, func(a, b integrationStatsRow) int {
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
		return cmp.Compare(a.Integration, b.Integration)
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
	Stderr        string `json:"stderr"`

	parsedEventTime time.Time
}

type ssmRunEventFilters struct {
	FailedOnly bool
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
			Stderr:        run.StandardError,
		}
		if !run.Time.IsZero() {
			record.parsedEventTime = run.Time.UTC()
			record.EventTime = record.parsedEventTime.Format(time.RFC3339Nano)
		}

		if filters.InstanceID != "" && !strings.EqualFold(strings.TrimSpace(record.InstanceID), strings.TrimSpace(filters.InstanceID)) {
			continue
		}
		if filters.FailedOnly && !isSSMRunFailure(record) {
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
		})
	}
	return rows
}

type ssmRunsOutput struct {
	Window       string       `json:"window"`
	Query        string       `json:"query"`
	TotalRuns    int          `json:"total_runs"`
	SuccessRuns  int          `json:"success_runs"`
	FailedRuns   int          `json:"failed_runs"`
	TotalVMs     int          `json:"total_vms"`
	FailingVMs   int          `json:"failing_vms"`
	DisplayedVMs int          `json:"displayed_vms"`
	VMPage       pageInfo     `json:"vm_page"`
	VMs          []ssmVMGroup `json:"vms"`
}
