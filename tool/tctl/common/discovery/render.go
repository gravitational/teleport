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
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	usertasksapi "github.com/gravitational/teleport/api/types/usertasks"
	"github.com/gravitational/teleport/lib/usertasks"
	"github.com/gravitational/trace"
)

func displayIntegrationName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "none (ambient credentials)"
	}
	return name
}

func renderTasksListText(w io.Writer, items []taskListItem, hints taskListHintsInput) error {
	style := newTextStyle(w)
	now := time.Now().UTC()

	fmt.Fprintf(w, "%s\n", style.section(fmt.Sprintf("User Tasks [%d matching filters]", len(items))))
	if len(items) == 0 {
		fmt.Fprintf(w, "%s\n", style.warning("No user tasks for the selected filters."))
		return trace.Wrap(renderNextActions(w, style, taskListNextActions(items, hints)))
	}

	for i, item := range items {
		if i > 0 {
			fmt.Fprintln(w, "")
		}
		details := []keyValue{
			{Key: "TASK", Value: item.Name},
			{Key: "STATE", Value: style.statusValue(item.State)},
			{Key: "TYPE", Value: friendlyTaskType(item.TaskType)},
			{Key: "ISSUE TYPE", Value: item.IssueType},
			{Key: "AFFECTED", Value: fmt.Sprintf("%d", item.Affected)},
			{Key: "INTEGRATION", Value: displayIntegrationName(item.Integration)},
			{Key: "LAST STATE CHANGE", Value: formatRelativeTime(item.LastStateChange, now)},
		}
		if err := style.numberedBlock(w, i, details); err != nil {
			return trace.Wrap(err)
		}
	}

	return trace.Wrap(renderNextActions(w, style, taskListNextActions(items, hints)))
}

type taskListHintsInput struct {
	State       string
	Integration string
	TaskType    string
	IssueType   string
}

func taskListNextActions(items []taskListItem, input taskListHintsInput) []nextAction {
	if len(items) == 0 {
		commands := []string{"tctl discovery tasks ls"}
		if input.State == usertasksapi.TaskStateOpen {
			commands = append(commands, "tctl discovery tasks ls --state=all")
		}
		return []nextAction{
			{
				Comment:  "Broaden task list filters",
				Commands: commands,
			},
			{
				Comment:  "Check discovery status",
				Commands: []string{"tctl discovery status"},
			},
			{
				Comment:  "List integrations",
				Commands: []string{"tctl discovery integration ls"},
			},
		}
	}

	actions := make([]nextAction, 0, 4)
	filterCommands := make([]string, 0, 3)
	if input.TaskType == "" {
		filterCommands = append(filterCommands, "tctl discovery tasks ls --task-type=discover-ec2")
	}
	if input.IssueType == "" {
		filterCommands = append(filterCommands, "tctl discovery tasks ls --issue-type=ec2-ssm-script-failure")
	}
	if input.Integration == "" && items[0].Integration != "" {
		filterCommands = append(filterCommands, fmt.Sprintf("tctl discovery tasks ls --integration=%s", items[0].Integration))
	}
	if len(filterCommands) > 0 {
		actions = append(actions, nextAction{
			Comment:  "Adjust task list filters",
			Commands: filterCommands,
		})
	}

	actions = append(actions, nextAction{
		Comment: "Inspect one task in detail",
		Commands: []string{
			fmt.Sprintf("tctl discovery tasks show %s", taskNamePrefix(items[0].Name)),
			"tctl discovery tasks show <task-id-prefix>",
		},
	})
	actions = append(actions, nextAction{
		Comment: "Use machine-readable output",
		Commands: []string{
			"tctl discovery tasks ls --format=json",
			"tctl discovery tasks ls --format=yaml",
		},
	})
	actions = append(actions, nextAction{
		Comment:  "List integrations",
		Commands: []string{"tctl discovery integration ls"},
	})

	return actions
}

func renderTaskDetailsText(w io.Writer, task *usertasksv1.UserTask, start, end int, baseCommand string) error {
	style := newTextStyle(w)
	now := time.Now().UTC()
	title, description := usertasks.DescriptionForDiscoverEC2Issue(task.GetSpec().GetIssueType())
	totalAffected := taskAffectedCount(task)

	headerRows := [][]string{
		{"Name", task.GetMetadata().GetName()},
		{"State", style.statusValue(task.GetSpec().GetState())},
		{"Task Type", friendlyTaskType(task.GetSpec().GetTaskType())},
		{"Issue Type", task.GetSpec().GetIssueType()},
		{"Issue", cmp.Or(title, task.GetSpec().GetIssueType())},
		{"Integration", displayIntegrationName(task.GetSpec().GetIntegration())},
		{"Affected resources", fmt.Sprintf("%d", totalAffected)},
		{"Last State Change", formatRelativeTime(taskLastStateChange(task), now)},
	}
	if exp := task.GetMetadata().GetExpires(); exp != nil {
		headerRows = append(headerRows, []string{"Expires", formatExpiryTime(exp.AsTime(), now)})
	}
	if err := renderTable(w, []string{"Field", "Value"}, headerRows, style.tableWidth); err != nil {
		return trace.Wrap(err)
	}

	var resourcePage pageInfo
	var err error
	switch task.GetSpec().GetTaskType() {
	case usertasksapi.TaskTypeDiscoverEC2:
		resourcePage, err = renderEC2Details(w, task, start, end)
	case usertasksapi.TaskTypeDiscoverEKS:
		resourcePage, err = renderEKSDetails(w, task, start, end)
	case usertasksapi.TaskTypeDiscoverRDS:
		resourcePage, err = renderRDSDetails(w, task, start, end)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	displayStart, displayEnd := resourceDisplayRange(resourcePage)
	if resourcePage.Total == 0 {
		fmt.Fprintf(w, "\n%s\n", style.info("Showing resources: 0-0."))
	} else if displayStart == 0 {
		fmt.Fprintf(w, "\n%s\n", style.info(fmt.Sprintf("Showing resources: 0-0 of %d.", resourcePage.Total)))
	} else {
		fmt.Fprintf(w, "\n%s\n", style.info(fmt.Sprintf("Showing resources: %d-%d of %d.", displayStart, displayEnd, resourcePage.Total)))
	}
	if description != "" {
		fmt.Fprintf(w, "\n%s\n", style.section("How to fix:"))
		fmt.Fprintf(w, "%s\n", formatHelpText(description))
	}

	return trace.Wrap(renderNextActions(w, style, taskShowNextActions(task, resourcePage, displayStart, baseCommand)))
}

func taskShowNextActions(task *usertasksv1.UserTask, resourcePage pageInfo, displayStart int, baseCommand string) []nextAction {
	actions := make([]nextAction, 0, 5)
	if resourcePage.Total > 0 && displayStart == 0 {
		actions = append(actions, nextAction{
			Comment:  "Current resource page is out of range",
			Commands: []string{withRangeFlag(baseCommand, 0, resourcePage.End)},
		})
	} else if resourcePage.HasNext {
		pageSize := resourcePage.End - resourcePage.Start
		nextEnd := resourcePage.End + pageSize
		if nextEnd > resourcePage.Total {
			nextEnd = resourcePage.Total
		}
		actions = append(actions, nextAction{
			Comment:  "Show next page of affected resources",
			Commands: []string{withRangeFlag(baseCommand, resourcePage.End, nextEnd)},
		})
	}
	if integration := task.GetSpec().GetIntegration(); integration != "" {
		actions = append(actions, nextAction{
			Comment: "See tasks for the same integration",
			Commands: []string{
				fmt.Sprintf("tctl discovery tasks ls --integration=%s", integration),
				fmt.Sprintf("tctl discovery tasks ls --integration=%s --state=resolved", integration),
			},
		})
		actions = append(actions, nextAction{
			Comment:  "Inspect this integration",
			Commands: []string{fmt.Sprintf("tctl discovery integration show %s", integration)},
		})
	}
	if task.GetSpec().GetTaskType() == usertasksapi.TaskTypeDiscoverEC2 {
		if instances := task.GetSpec().GetDiscoverEc2().GetInstances(); len(instances) > 0 {
			keys := mapKeys(instances)
			slices.Sort(keys)
			instanceID := keys[0]
			actions = append(actions, nextAction{
				Comment: "Check SSM runs for this instance",
				Commands: []string{
					fmt.Sprintf("tctl discovery ssm-runs show %s", instanceID),
					fmt.Sprintf("tctl discovery ssm-runs show %s --show-all-runs", instanceID),
					fmt.Sprintf("tctl discovery ssm-runs show %s --last=1h", instanceID),
				},
			})
		}
	}
	actions = append(actions, nextAction{
		Comment: "Use machine-readable output",
		Commands: []string{
			fmt.Sprintf("tctl discovery tasks show %s --format=json", taskNamePrefix(task.GetMetadata().GetName())),
			fmt.Sprintf("tctl discovery tasks show %s --format=yaml", taskNamePrefix(task.GetMetadata().GetName())),
		},
	})
	actions = append(actions, nextAction{
		Comment: "Check instance joins",
		Commands: []string{
			"tctl discovery joins ls",
			"tctl discovery joins ls --last=1h",
		},
	})
	actions = append(actions, nextAction{
		Comment:  "Return to discovery overview",
		Commands: []string{"tctl discovery status"},
	})
	return actions
}

func resourceDisplayRange(info pageInfo) (start, end int) {
	if info.Total == 0 || info.Start >= info.End {
		return 0, 0
	}
	return info.Start + 1, info.End
}

func renderEC2Details(w io.Writer, task *usertasksv1.UserTask, start, end int) (pageInfo, error) {
	ec2 := usertasks.EC2InstancesWithURLs(task)
	pageKeys, info, style := paginateMapKeys(w, ec2.Instances, start, end)
	now := time.Now().UTC()

	fmt.Fprintf(w, "\n%s\n", style.section("Affected EC2 instances:"))
	if len(pageKeys) == 0 {
		fmt.Fprintf(w, "%s\n", style.warning("No affected EC2 instances for the selected page."))
		return info, nil
	}

	for i, key := range pageKeys {
		instance := ec2.Instances[key]
		if i > 0 {
			fmt.Fprintln(w, "")
		}

		details := make([]keyValue, 0, 8)
		details = append(details, keyValue{Key: "INSTANCE", Value: instance.GetInstanceId()})
		if name := strings.TrimSpace(instance.GetName()); name != "" {
			details = append(details, keyValue{Key: "NAME", Value: name})
		}
		if region := strings.TrimSpace(ec2.GetRegion()); region != "" {
			details = append(details, keyValue{Key: "REGION", Value: region})
		}

		details = append(details, keyValue{Key: "DISCOVERY CONFIG", Value: cmp.Or(strings.TrimSpace(instance.GetDiscoveryConfig()), "n/a")})
		details = append(details, keyValue{Key: "DISCOVERY GROUP", Value: cmp.Or(strings.TrimSpace(instance.GetDiscoveryGroup()), "n/a")})

		syncTime := "n/a"
		if ts := instance.GetSyncTime(); ts != nil {
			abs := formatProtoTimestamp(ts)
			syncTime = fmt.Sprintf("%s (%s)", abs, formatRelativeTime(ts.AsTime(), now))
		}
		details = append(details, keyValue{Key: "SYNC TIME", Value: syncTime})

		if awsURL := strings.TrimSpace(instance.ResourceURL); awsURL != "" {
			details = append(details, keyValue{Key: "AWS URL", Value: awsURL})
		}
		if invocationURL := strings.TrimSpace(instance.GetInvocationUrl()); invocationURL != "" {
			details = append(details, keyValue{Key: "INVOCATION URL", Value: invocationURL})
		}
		if err := style.numberedBlock(w, info.Start+i, details); err != nil {
			return info, trace.Wrap(err)
		}
	}

	return info, nil
}

func renderEKSDetails(w io.Writer, task *usertasksv1.UserTask, start, end int) (pageInfo, error) {
	eks := usertasks.EKSClustersWithURLs(task)
	pageKeys, info, style := paginateMapKeys(w, eks.Clusters, start, end)

	rows := make([][]string, 0, len(pageKeys))
	for _, key := range pageKeys {
		cluster := eks.Clusters[key]
		rows = append(rows, []string{
			cluster.GetName(),
			cluster.GetDiscoveryConfig(),
			cluster.GetDiscoveryGroup(),
			formatProtoTimestamp(cluster.GetSyncTime()),
			cluster.ResourceURL,
			eksActionURL(cluster),
		})
	}

	fmt.Fprintf(w, "\n%s\n", style.section("Affected EKS clusters:"))
	return info, trace.Wrap(renderTable(w, []string{"Cluster", "DiscoveryConfig", "DiscoveryGroup", "Sync Time", "AWS URL", "Action URL"}, rows, style.tableWidth))
}

func eksActionURL(cluster *usertasks.DiscoverEKSClusterWithURLs) string {
	if cluster.OpenTeleportAgentURL != "" {
		return cluster.OpenTeleportAgentURL
	}
	if cluster.ManageAccessURL != "" {
		return cluster.ManageAccessURL
	}
	if cluster.ManageEndpointAccessURL != "" {
		return cluster.ManageEndpointAccessURL
	}
	if cluster.ManageClusterURL != "" {
		return cluster.ManageClusterURL
	}
	return ""
}

func renderRDSDetails(w io.Writer, task *usertasksv1.UserTask, start, end int) (pageInfo, error) {
	rds := usertasks.RDSDatabasesWithURLs(task)
	pageKeys, info, style := paginateMapKeys(w, rds.Databases, start, end)

	rows := make([][]string, 0, len(pageKeys))
	for _, key := range pageKeys {
		database := rds.Databases[key]
		rows = append(rows, []string{
			database.GetName(),
			database.GetEngine(),
			fmt.Sprintf("%t", database.GetIsCluster()),
			database.GetDiscoveryConfig(),
			database.GetDiscoveryGroup(),
			formatProtoTimestamp(database.GetSyncTime()),
			database.ResourceURL,
			database.ConfigurationURL,
		})
	}

	fmt.Fprintf(w, "\n%s\n", style.section("Affected RDS databases:"))
	return info, trace.Wrap(renderTable(w, []string{"Database", "Engine", "Cluster", "DiscoveryConfig", "DiscoveryGroup", "Sync Time", "AWS URL", "Config URL"}, rows, style.tableWidth))
}

func renderStatusText(w io.Writer, summary statusSummary) error {
	style := newTextStyle(w)
	now := summary.GeneratedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	fmt.Fprintf(w, "  %s\n", style.section(fmt.Sprintf("User Tasks [%d total, %d open, %d resolved]", summary.TotalTasks, summary.OpenTasks, summary.ResolvedTasks)))
	if summary.FilteredIntegration != "" {
		fmt.Fprintf(w, "  %s\n", style.info(fmt.Sprintf("Filtered by integration=%s", summary.FilteredIntegration)))
	}
	if len(summary.UserTasks) == 0 {
		fmt.Fprintf(w, "%s\n", style.warning("No user tasks for the selected filters."))
	} else {
		rows := make([][]string, 0, len(summary.UserTasks))
		for _, task := range summary.UserTasks {
			rows = append(rows, []string{
				task.Name,
				style.statusValue(task.State),
				friendlyTaskType(task.TaskType),
				task.IssueType,
				fmt.Sprintf("%d", task.Affected),
				displayIntegrationName(task.Integration),
				formatRelativeTime(task.LastStateChange, now),
			})
		}
		if err := renderTable(w, []string{"Name", "State", "TaskType", "IssueType", "Affected", "Integration", "Last State Change"}, rows, style.tableWidth); err != nil {
			return trace.Wrap(err)
		}
	}

	if len(summary.DiscoveryConfigs) > 0 {
		fmt.Fprintf(w, "\n  %s\n", style.section(fmt.Sprintf("Discovery Configs [%d total, %s]", summary.DiscoveryConfigCount, formatCountLabel(summary.DiscoveryGroupCount, "group", "groups"))))
		rows := make([][]string, 0, len(summary.DiscoveryConfigs))
		for _, cfg := range summary.DiscoveryConfigs {
			rows = append(rows, []string{
				cfg.Name,
				cfg.Group,
				style.statusValue(humanizeEnumValue(cfg.State)),
				cfg.Matchers,
				style.discoveredCount(cfg.Discovered),
				formatRelativeTime(cfg.LastSync, summary.GeneratedAt),
			})
		}
		if err := renderTable(w, []string{"Name", "Group", "State", "Matchers", "Discovered", "Last Sync"}, rows, style.tableWidth); err != nil {
			return trace.Wrap(err)
		}
	}

	if len(summary.Integrations) > 0 {
		activeCount := 0
		for _, item := range summary.Integrations {
			if item.Found > 0 || item.Enrolled > 0 || item.Failed > 0 {
				activeCount++
			}
		}
		fmt.Fprintf(w, "\n  %s\n", style.section(fmt.Sprintf("Integrations [%d total, %d active]",
			len(summary.Integrations), activeCount)))
		rows := make([][]string, 0, len(summary.Integrations))
		for _, item := range summary.Integrations {
			rows = append(rows, []string{
				item.Name,
				item.Type,
				fmt.Sprintf("%d", item.OpenTasks),
				style.discoveredCount(item.Found),
				style.pendingCount(item.AwaitingJoin),
				style.discoveredCount(item.Enrolled),
				style.failedCount(item.Failed),
			})
		}
		if err := renderTable(w, []string{"Name", "Type", "Open Tasks", "Found", "Awaiting Join", "Enrolled", "Failed"}, rows, style.tableWidth); err != nil {
			return trace.Wrap(err)
		}
	}

	if summary.SSMRunStats != nil || summary.JoinStats != nil {
		renderAuditEventStatsSection(w, style, "SSM Runs", summary.SSMRunStats, "--ssm-limit")
		renderAuditEventStatsSection(w, style, "Instance Joins", summary.JoinStats, "--join-limit")
	}

	if summary.CacheSummary != "" {
		fmt.Fprintf(w, "\n  %s %s\n", style.info("Cache:"), summary.CacheSummary)
	}

	return trace.Wrap(renderNextActions(w, style, statusNextActions(summary)))
}

func renderAuditEventStatsSection(w io.Writer, style textStyle, title string, stats *auditEventStats, limitFlag string) {
	if stats == nil {
		return
	}
	windowLabel := stats.Window
	if stats.LimitReached && stats.EffectiveWindow != "" {
		if stats.SuggestedLimit > 0 {
			windowLabel = fmt.Sprintf("requested %s, oldest returned %s; use %s=%d", stats.Window, stats.EffectiveWindow, limitFlag, stats.SuggestedLimit)
		} else {
			windowLabel = fmt.Sprintf("requested %s, oldest returned %s; increase %s", stats.Window, stats.EffectiveWindow, limitFlag)
		}
	} else if stats.LimitReached {
		windowLabel += fmt.Sprintf("; increase %s", limitFlag)
	}
	fmt.Fprintf(w, "\n  %s\n", style.section(fmt.Sprintf("%s (%s)", title, windowLabel)))

	details := []keyValue{
		{Key: "TOTAL EVENTS", Value: fmt.Sprintf("%d", stats.Total)},
		{Key: "SUCCESSFUL", Value: fmt.Sprintf("%d", stats.Success)},
		{Key: "FAILED", Value: style.failedCount(uint64(stats.Failed))},
		{Key: "DISTINCT HOSTS", Value: fmt.Sprintf("%d", stats.DistinctHosts)},
		{Key: "HOSTS WITH FAILURES", Value: style.failedCount(uint64(stats.FailingHosts))},
	}
	_ = style.indented().keyValues(w, details)
}

func statusNextActions(summary statusSummary) []nextAction {
	var actions []nextAction

	// Limit-reached warnings first — incomplete data is critical to surface.
	if summary.SSMRunStats != nil && summary.SSMRunStats.LimitReached {
		limit := suggestedOrFallbackLimit(summary.SSMRunStats.SuggestedLimit, 0)
		if limit > 0 {
			actions = append(actions, nextAction{
				Comment:  fmt.Sprintf("SSM run limit reached — rerun with --ssm-limit=%d to cover full window", limit),
				Commands: []string{fmt.Sprintf("tctl discovery status --ssm-limit=%d", limit)},
			})
		}
	}
	if summary.JoinStats != nil && summary.JoinStats.LimitReached {
		limit := suggestedOrFallbackLimit(summary.JoinStats.SuggestedLimit, 0)
		if limit > 0 {
			actions = append(actions, nextAction{
				Comment:  fmt.Sprintf("Join limit reached — rerun with --join-limit=%d to cover full window", limit),
				Commands: []string{fmt.Sprintf("tctl discovery status --join-limit=%d", limit)},
			})
		}
	}

	inspectCommands := []string{"tctl discovery tasks show <task-id-prefix>"}
	if len(summary.UserTasks) > 0 {
		inspectCommands = []string{
			fmt.Sprintf("tctl discovery tasks show %s", taskNamePrefix(summary.UserTasks[0].Name)),
			"tctl discovery tasks show <task-id-prefix>",
		}
	}
	actions = append(actions,
		nextAction{
			Comment: "List discovery tasks",
			Commands: []string{
				"tctl discovery tasks ls",
				"tctl discovery tasks ls --state=resolved",
			},
		},
		nextAction{
			Comment:  "Investigate particular open task",
			Commands: inspectCommands,
		},
		nextAction{
			Comment: "List integrations",
			Commands: []string{
				"tctl discovery integration ls",
			},
		},
		nextAction{
			Comment: "Check SSM runs (use --cluster to group similar runs)",
			Commands: []string{
				"tctl discovery ssm-runs ls",
				"tctl discovery ssm-runs ls --cluster",
				"tctl discovery ssm-runs ls --last=1h",
				"tctl discovery ssm-runs ls --from-utc=2026-02-15T08:00 --to-utc=2026-02-15T20:00",
			},
		},
		nextAction{
			Comment: "Check instance joins",
			Commands: []string{
				"tctl discovery joins ls",
				"tctl discovery joins ls --last=1h",
			},
		},
		nextAction{
			Comment:  "View unified host inventory",
			Commands: []string{"tctl discovery inventory ls"},
		},
		nextAction{
			Comment: "Use machine-readable output",
			Commands: []string{
				"tctl discovery status --format=json",
				"tctl discovery status --format=yaml",
			},
		},
	)
	return actions
}

func writeCountTable(w io.Writer, label string, counts map[string]int, style textStyle, colorKeys bool) error {
	total := 0
	for _, count := range counts {
		total += count
	}
	return renderCountRowsTable(w, label, countRows(counts), total, style, colorKeys)
}

func renderCountRowsTable(w io.Writer, label string, rows []countRow, total int, style textStyle, colorKeys bool) error {
	dataRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		share := "0.0%"
		if total > 0 {
			share = fmt.Sprintf("%.1f%%", (float64(row.Count)/float64(total))*100.0)
		}
		key := row.Key
		if colorKeys {
			key = style.statusValue(key)
		}
		dataRows = append(dataRows, []string{key, fmt.Sprintf("%d", row.Count), share})
	}
	return trace.Wrap(renderTable(w, []string{label, "Count", "Share"}, dataRows, style.tableWidth))
}

// suggestedOrFallbackLimit returns the suggested limit if available,
// otherwise falls back to 5x the current limit.
func suggestedOrFallbackLimit(suggested, current int) int {
	if suggested > 0 {
		return suggested
	}
	return current * 5
}

func renderSSMRunsText(w io.Writer, output ssmRunsOutput, instanceIDFilter string, showAllRuns bool, baseCommand string) error {
	style := newTextStyle(w)
	now := time.Now().UTC()
	runsInWindow := fmt.Sprintf("%d total (%d failed, %d success)", output.TotalRuns, output.FailedRuns, output.SuccessRuns)
	if output.LimitReached {
		if output.SuggestedLimit > 0 {
			runsInWindow += fmt.Sprintf(" [limit: %d, use --limit=%d to cover full window]", output.FetchLimit, output.SuggestedLimit)
		} else {
			runsInWindow += fmt.Sprintf(" [limit: %d, use --limit to increase]", output.FetchLimit)
		}
	}
	summaryRows := [][]string{
		{"Query window", output.Window},
		{"SSM runs in window", runsInWindow},
		{"VM status snapshot", fmt.Sprintf("%d total VMs, %d currently failing", output.TotalVMs, output.FailingVMs)},
		{"VM rows shown", fmt.Sprintf("%d (range %d-%d)", len(output.VMs), output.VMPage.Start, output.VMPage.End)},
	}
	if output.CacheSummary != "" {
		summaryRows = append(summaryRows, []string{"Cache", output.CacheSummary})
	}
	if instanceIDFilter != "" {
		summaryRows = append(summaryRows, []string{"Filtered instance", instanceIDFilter})
	}
	if err := renderTable(w, []string{"Summary Item", "Details"}, summaryRows, style.tableWidth); err != nil {
		return trace.Wrap(err)
	}

	if len(output.VMs) == 0 {
		if instanceIDFilter != "" {
			fmt.Fprintf(w, "\n%s\n", style.warning(fmt.Sprintf("No SSM runs found for instance %s in the selected window.", instanceIDFilter)))
		} else {
			fmt.Fprintf(w, "\n%s\n", style.warning("No VMs found for the selected window."))
		}
	} else {
		sectionTitle := "VMs (sorted by most recent run):"
		if instanceIDFilter != "" {
			sectionTitle = "VM:"
		}
		fmt.Fprintf(w, "\n%s\n", style.section(sectionTitle))
		for i, vm := range output.VMs {
			if i > 0 {
				fmt.Fprintln(w, "")
			}
			details := []keyValue{
				{Key: "INSTANCE", Value: vm.InstanceID},
				{Key: "MOST RECENT", Value: formatRelativeOrTimestamp(vm.MostRecent.parsedEventTime, vm.MostRecent.EventTime, now)},
				{Key: "RESULT", Value: cmp.Or(vm.MostRecent.Status, vm.MostRecent.Code)},
				{Key: "RUNS", Value: fmt.Sprintf("%d", vm.TotalRuns)},
				{Key: "FAILED", Value: fmt.Sprintf("%d", vm.FailedRuns)},
				{Key: "REGION", Value: cmp.Or(strings.TrimSpace(vm.MostRecent.Region), "n/a")},
			}
			if err := style.numberedBlock(w, i, details); err != nil {
				return trace.Wrap(err)
			}
		}

		// In single-instance view, always show history. In multi-VM ls view,
		// show history when --show-all-runs is set or when any VM has output
		// (stdout/stderr from the install script).
		showHistory := showAllRuns || instanceIDFilter != ""
		if !showHistory {
			for _, vm := range output.VMs {
				for _, run := range vm.Runs {
					if run.Stdout != "" || run.Stderr != "" {
						showHistory = true
						break
					}
				}
				if showHistory {
					break
				}
			}
		}
		if showHistory {
			fmt.Fprintf(w, "\n%s\n", style.section("Run history:"))
			for vmIndex, vm := range output.VMs {
				prefix := fmt.Sprintf("[%d] ", vmIndex+1)
				fmt.Fprintf(w, "\n%s\n", style.info(fmt.Sprintf("%sVM: %s", prefix, vm.InstanceID)))
				rows := buildVMHistoryRows(vm, showAllRuns)
				sub := style.nested(vmIndex)
				if err := renderSSMRunHistoryRows(w, sub, rows, now); err != nil {
					return trace.Wrap(err)
				}
			}
		}
	}
	if len(output.ErrorClusters) > 0 {
		renderSSMRunClusters(w, style, "Error", output.ErrorClusters)
	}
	if len(output.SuccessClusters) > 0 {
		renderSSMRunClusters(w, style, "Success", output.SuccessClusters)
	}

	if output.VMPage.HasNext {
		fmt.Fprintf(w, "\n%s\n", style.warning(fmt.Sprintf("More VMs available: %d remaining.", output.VMPage.Remaining)))
		pageSize := output.VMPage.End - output.VMPage.Start
		nextEnd := output.VMPage.End + pageSize
		if nextEnd > output.VMPage.Total {
			nextEnd = output.VMPage.Total
		}
		fmt.Fprintf(w, "%s %s\n", style.info("Next page:"), withRangeFlag(baseCommand, output.VMPage.End, nextEnd))
	}

	actions := ssmRunsNextActions(output, instanceIDFilter, showAllRuns, baseCommand)

	return trace.Wrap(renderNextActions(w, style, actions))
}

func renderSSMRunHistoryRows(w io.Writer, style textStyle, rows []ssmRunHistoryRow, now time.Time) error {
	for i, row := range rows {
		if i > 0 {
			fmt.Fprintln(w, "")
		}

		details := []keyValue{
			{Key: "TIMESTAMP", Value: formatHistoryTimestamp(row.Timestamp, now)},
			{Key: "RESULT", Value: row.Result},
			{Key: "COMMAND", Value: cmp.Or(strings.TrimSpace(row.CommandID), "n/a")},
			{Key: "EXIT", Value: cmp.Or(strings.TrimSpace(row.ExitCode), "n/a")},
		}
		if err := style.numberedBlock(w, i, details); err != nil {
			return trace.Wrap(err)
		}
		renderQuotedOutput(w, row.Output, style.nested(i).indent, details)
	}
	return nil
}

func renderQuotedOutput(w io.Writer, output string, indent string, details []keyValue) {
	output = strings.TrimSpace(output)
	if output == "" {
		return
	}
	maxKeyWidth := len("OUTPUT")
	for _, kv := range details {
		maxKeyWidth = max(maxKeyWidth, len(kv.Key))
	}
	fmt.Fprintf(w, "%s%-*s:\n", indent, maxKeyWidth, "OUTPUT")
	for _, line := range strings.Split(output, "\n") {
		line = strings.ReplaceAll(line, "\r", "")
		fmt.Fprintf(w, "%s> %s\n", indent, line)
	}
}

func renderSSMRunClusters(w io.Writer, style textStyle, kind string, clusters []ssmRunCluster) {
	fmt.Fprintf(w, "\n%s\n", style.section(fmt.Sprintf("%s Clusters (%d groups):", kind, len(clusters))))
	for _, c := range clusters {
		totalRuns := 0
		for _, inst := range c.Instances {
			totalRuns += len(inst.Times)
		}
		fmt.Fprintf(w, "\n  %s %d runs across %d instances\n",
			style.info(fmt.Sprintf("Cluster %d:", c.ID+1)),
			totalRuns, len(c.Instances))

		fmt.Fprintf(w, "  %s\n", style.warning("Pattern:"))
		for _, line := range strings.Split(c.Template, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				fmt.Fprintf(w, "    > %s\n", line)
			}
		}
	}
}

func renderIntegrationListText(w io.Writer, items []integrationListItem) error {
	style := newTextStyle(w)

	fmt.Fprintf(w, "%s\n", style.section(fmt.Sprintf("Integrations [%d]", len(items))))
	if len(items) == 0 {
		fmt.Fprintf(w, "%s\n", style.warning("No integrations found."))
		return trace.Wrap(renderNextActions(w, style, integrationListNextActions(items)))
	}

	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			item.Name,
			item.Type,
			style.discoveredCount(item.Found),
			style.discoveredCount(item.Enrolled),
			style.failedCount(item.Failed),
			style.pendingCount(item.AwaitingJoin),
			fmt.Sprintf("%d", item.OpenTasks),
		})
	}
	if err := renderTable(w, []string{"Name", "Type", "Found", "Enrolled", "Failed", "Awaiting Join", "Open Tasks"}, rows, style.tableWidth); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(renderNextActions(w, style, integrationListNextActions(items)))
}

func integrationListNextActions(items []integrationListItem) []nextAction {
	if len(items) == 0 {
		return []nextAction{
			{
				Comment:  "Check discovery status",
				Commands: []string{"tctl discovery status"},
			},
		}
	}

	actions := make([]nextAction, 0, 4)
	actions = append(actions, nextAction{
		Comment: "Inspect an integration",
		Commands: []string{
			fmt.Sprintf("tctl discovery integration show %s", items[0].Name),
			"tctl discovery integration show <name>",
		},
	})
	actions = append(actions, nextAction{
		Comment: "List discovery tasks",
		Commands: []string{
			"tctl discovery tasks ls",
		},
	})
	actions = append(actions, nextAction{
		Comment:  "Check discovery status",
		Commands: []string{"tctl discovery status"},
	})
	actions = append(actions, nextAction{
		Comment: "Use machine-readable output",
		Commands: []string{
			"tctl discovery integration ls --format=json",
			"tctl discovery integration ls --format=yaml",
		},
	})
	return actions
}

func renderIntegrationShowText(w io.Writer, detail integrationDetail) error {
	style := newTextStyle(w)
	now := time.Now().UTC()

	headerKVs := []keyValue{
		{Key: "NAME", Value: detail.Name},
		{Key: "TYPE", Value: detail.Type},
	}
	credKeys := mapKeys(detail.Credentials)
	slices.Sort(credKeys)
	for _, k := range credKeys {
		headerKVs = append(headerKVs, keyValue{Key: strings.ToUpper(k), Value: detail.Credentials[k]})
	}
	if err := style.keyValues(w, headerKVs); err != nil {
		return trace.Wrap(err)
	}

	if len(detail.ResourceTypeStats) > 0 {
		fmt.Fprintf(w, "\n%s\n", style.section("Resource Stats by Type"))
		rows := make([][]string, 0, len(detail.ResourceTypeStats))
		for _, rs := range detail.ResourceTypeStats {
			rows = append(rows, []string{
				rs.ResourceType,
				style.discoveredCount(rs.Found),
				style.discoveredCount(rs.Enrolled),
				style.failedCount(rs.Failed),
			})
		}
		if err := renderTable(w, []string{"Resource Type", "Found", "Enrolled", "Failed"}, rows, style.tableWidth); err != nil {
			return trace.Wrap(err)
		}
	}

	if len(detail.DiscoveryConfigs) > 0 {
		fmt.Fprintf(w, "\n%s\n", style.section("Discovery Configs"))
		rows := make([][]string, 0, len(detail.DiscoveryConfigs))
		for _, cfg := range detail.DiscoveryConfigs {
			rows = append(rows, []string{
				cfg.Name,
				style.statusValue(humanizeEnumValue(cfg.State)),
				cfg.Matchers,
				formatRelativeTime(cfg.LastSync, now),
			})
		}
		if err := renderTable(w, []string{"Name", "State", "Matchers", "Last Sync"}, rows, style.tableWidth); err != nil {
			return trace.Wrap(err)
		}
	}

	fmt.Fprintf(w, "\n%s\n", style.section("Open Tasks"))
	if len(detail.OpenTasks) == 0 {
		fmt.Fprintf(w, "%s\n", style.info("No open tasks for this integration."))
	} else {
		rows := make([][]string, 0, len(detail.OpenTasks))
		for _, task := range detail.OpenTasks {
			rows = append(rows, []string{
				task.Name,
				task.IssueType,
				fmt.Sprintf("%d", task.Affected),
			})
		}
		if err := renderTable(w, []string{"Task Name", "Issue Type", "Affected"}, rows, style.tableWidth); err != nil {
			return trace.Wrap(err)
		}
	}

	return trace.Wrap(renderNextActions(w, style, integrationShowNextActions(detail)))
}

func integrationShowNextActions(detail integrationDetail) []nextAction {
	actions := make([]nextAction, 0, 6)
	if len(detail.OpenTasks) > 0 {
		actions = append(actions, nextAction{
			Comment: "Inspect a task",
			Commands: []string{
				fmt.Sprintf("tctl discovery tasks show %s", taskNamePrefix(detail.OpenTasks[0].Name)),
				"tctl discovery tasks show <task-id-prefix>",
			},
		})
	}
	actions = append(actions, nextAction{
		Comment: "List tasks for this integration",
		Commands: []string{
			fmt.Sprintf("tctl discovery tasks ls --integration=%s", detail.Name),
			fmt.Sprintf("tctl discovery tasks ls --integration=%s --state=resolved", detail.Name),
		},
	})
	actions = append(actions, nextAction{
		Comment: "Use machine-readable output",
		Commands: []string{
			fmt.Sprintf("tctl discovery integration show %s --format=json", detail.Name),
			fmt.Sprintf("tctl discovery integration show %s --format=yaml", detail.Name),
		},
	})
	actions = append(actions, nextAction{
		Comment: "Check SSM runs",
		Commands: []string{
			"tctl discovery ssm-runs ls",
			"tctl discovery ssm-runs ls --last=1h",
			"tctl discovery ssm-runs ls --from-utc=2026-02-15T08:00 --to-utc=2026-02-15T20:00",
		},
	})
	actions = append(actions, nextAction{
		Comment: "Check instance joins",
		Commands: []string{
			"tctl discovery joins ls",
			"tctl discovery joins ls --last=1h",
		},
	})
	actions = append(actions, nextAction{
		Comment:  "Return to integration list",
		Commands: []string{"tctl discovery integration ls"},
	})
	actions = append(actions, nextAction{
		Comment:  "Check discovery status",
		Commands: []string{"tctl discovery status"},
	})
	return actions
}

func ssmRunsNextActions(output ssmRunsOutput, instanceIDFilter string, showAllRuns bool, baseCommand string) []nextAction {
	if instanceIDFilter != "" {
		// Single-instance view: suggest going back to list view and checking tasks.
		actions := []nextAction{
			{
				Comment: "Return to SSM overview (use --cluster to group similar runs)",
				Commands: []string{
					"tctl discovery ssm-runs ls",
					"tctl discovery ssm-runs ls --cluster",
					"tctl discovery ssm-runs ls --last=1h",
					"tctl discovery ssm-runs ls --from-utc=2026-02-15T08:00 --to-utc=2026-02-15T20:00",
				},
			},
		}
		if !showAllRuns {
			actions = append(actions, nextAction{
				Comment:  "Show full run history for this instance",
				Commands: []string{fmt.Sprintf("tctl discovery ssm-runs show %s --last=1h --show-all-runs", instanceIDFilter)},
			})
		}
		actions = append(actions, nextAction{
			Comment:  "Use machine-readable output",
			Commands: []string{fmt.Sprintf("tctl discovery ssm-runs show %s --last=1h --format=json", instanceIDFilter)},
		})
		actions = append(actions, nextAction{
			Comment:  "Inspect the discovery tasks themselves",
			Commands: []string{"tctl discovery tasks ls --task-type=discover-ec2 --state=open"},
		})
		actions = append(actions, nextAction{
			Comment:  "List integrations",
			Commands: []string{"tctl discovery integration ls"},
		})
		return actions
	}

	// List view: suggest drilling into a specific instance.
	instanceCommand := "tctl discovery ssm-runs show <instance-id> --show-all-runs"
	if len(output.VMs) > 0 && output.VMs[0].InstanceID != "unknown" {
		instanceCommand = fmt.Sprintf("tctl discovery ssm-runs show %s --show-all-runs", output.VMs[0].InstanceID)
	}

	actions := []nextAction{}
	if output.LimitReached {
		limit := suggestedOrFallbackLimit(output.SuggestedLimit, output.FetchLimit)
		actions = append(actions, nextAction{
			Comment:  fmt.Sprintf("Rerun with --limit=%d to cover full window", limit),
			Commands: []string{fmt.Sprintf("%s --limit=%d", baseCommand, limit)},
		})
	}
	if output.ErrorClusters == nil && output.SuccessClusters == nil {
		actions = append(actions, nextAction{
			Comment: "Group similar runs into clusters",
			Commands: []string{
				"tctl discovery ssm-runs ls --cluster",
				"tctl discovery ssm-runs ls --cluster --format=json",
			},
		})
	}
	actions = append(actions,
		nextAction{
			Comment:  "View all runs for a specific failing instance",
			Commands: []string{instanceCommand},
		},
		nextAction{
			Comment: "Adjust SSM time window",
			Commands: []string{
				"tctl discovery ssm-runs ls --last=1h",
				"tctl discovery ssm-runs ls --from-utc=2026-02-15T08:00 --to-utc=2026-02-15T20:00",
			},
		},
	)
	actions = append(actions,
		nextAction{
			Comment: "Use machine-readable output",
			Commands: []string{
				"tctl discovery ssm-runs ls --last=1h --format=json",
			},
		},
		nextAction{
			Comment:  "Inspect the discovery tasks themselves",
			Commands: []string{"tctl discovery tasks ls --task-type=discover-ec2 --state=open"},
		},
		nextAction{
			Comment:  "List integrations",
			Commands: []string{"tctl discovery integration ls"},
		},
		nextAction{
			Comment: "Check instance joins",
			Commands: []string{
				"tctl discovery joins ls",
				"tctl discovery joins ls --last=1h",
			},
		},
		nextAction{
			Comment:  "View unified host inventory",
			Commands: []string{"tctl discovery inventory ls"},
		},
	)
	return actions
}

func renderJoinsText(w io.Writer, output joinsOutput, hostIDFilter string, showAllJoins bool, baseCommand string) error {
	style := newTextStyle(w)
	now := time.Now().UTC()
	joinsInWindow := fmt.Sprintf("%d total (%d failed, %d success)", output.TotalJoins, output.FailedJoins, output.SuccessJoins)
	if output.LimitReached {
		if output.SuggestedLimit > 0 {
			joinsInWindow += fmt.Sprintf(" [limit: %d, use --limit=%d to cover full window]", output.FetchLimit, output.SuggestedLimit)
		} else {
			joinsInWindow += fmt.Sprintf(" [limit: %d, use --limit to increase]", output.FetchLimit)
		}
	}
	summaryRows := [][]string{
		{"Query window", output.Window},
		{"Joins in window", joinsInWindow},
		{"Host snapshot", fmt.Sprintf("%d total hosts, %d with failed joins", output.TotalHosts, output.FailingHosts)},
		{"Host rows shown", fmt.Sprintf("%d (range %d-%d)", len(output.Hosts), output.HostPage.Start, output.HostPage.End)},
	}
	if output.CacheSummary != "" {
		summaryRows = append(summaryRows, []string{"Cache", output.CacheSummary})
	}
	if hostIDFilter != "" {
		summaryRows = append(summaryRows, []string{"Filtered host", hostIDFilter})
	}
	if err := renderTable(w, []string{"Summary Item", "Details"}, summaryRows, style.tableWidth); err != nil {
		return trace.Wrap(err)
	}

	if len(output.Hosts) == 0 {
		if hostIDFilter != "" {
			fmt.Fprintf(w, "\n%s\n", style.warning(fmt.Sprintf("No join events found for host %s in the selected window.", hostIDFilter)))
		} else {
			fmt.Fprintf(w, "\n%s\n", style.warning("No hosts found for the selected window."))
		}
	} else {
		sectionTitle := "Hosts (sorted by most recent join):"
		if hostIDFilter != "" {
			sectionTitle = "Host:"
		}
		fmt.Fprintf(w, "\n%s\n", style.section(sectionTitle))
		for i, host := range output.Hosts {
			if i > 0 {
				fmt.Fprintln(w, "")
			}
			result := "success"
			if host.MostRecentFailed {
				result = cmp.Or(host.MostRecent.Error, "failed")
			}
			details := []keyValue{
				{Key: "HOST", Value: host.HostID},
				{Key: "NODE NAME", Value: cmp.Or(strings.TrimSpace(host.NodeName), "n/a")},
				{Key: "INSTANCE ID", Value: cmp.Or(strings.TrimSpace(host.MostRecent.InstanceID), "n/a")},
				{Key: "ACCOUNT ID", Value: cmp.Or(strings.TrimSpace(host.MostRecent.AccountID), "n/a")},
				{Key: "TOKEN", Value: cmp.Or(strings.TrimSpace(host.MostRecent.TokenName), "n/a")},
				{Key: "MOST RECENT", Value: formatRelativeOrTimestamp(host.MostRecent.parsedEventTime, host.MostRecent.EventTime, now)},
				{Key: "RESULT", Value: result},
				{Key: "METHOD", Value: cmp.Or(strings.TrimSpace(host.MostRecent.Method), "n/a")},
				{Key: "ROLE", Value: cmp.Or(strings.TrimSpace(host.MostRecent.Role), "n/a")},
				{Key: "JOINS", Value: fmt.Sprintf("%d", host.TotalJoins)},
				{Key: "FAILED", Value: fmt.Sprintf("%d", host.FailedJoins)},
			}
			if err := style.numberedBlock(w, i, details); err != nil {
				return trace.Wrap(err)
			}
		}

		// Show history in single-host detail view or when --show-all-joins is set.
		if hostIDFilter != "" || showAllJoins {
			fmt.Fprintf(w, "\n%s\n", style.section("Join history:"))
			for hostIndex, host := range output.Hosts {
				prefix := fmt.Sprintf("[%d] ", hostIndex+1)
				fmt.Fprintf(w, "\n%s\n", style.info(fmt.Sprintf("%sHOST: %s", prefix, host.HostID)))
				rows := buildJoinHistoryRows(host, showAllJoins)
				sub := style.nested(hostIndex)
				if err := renderJoinHistoryRows(w, sub, rows, now); err != nil {
					return trace.Wrap(err)
				}
			}
		}
	}
	if output.HostPage.HasNext {
		fmt.Fprintf(w, "\n%s\n", style.warning(fmt.Sprintf("More hosts available: %d remaining.", output.HostPage.Remaining)))
		pageSize := output.HostPage.End - output.HostPage.Start
		nextEnd := min(output.HostPage.End+pageSize, output.HostPage.Total)
		fmt.Fprintf(w, "%s %s\n", style.info("Next page:"), withRangeFlag(baseCommand, output.HostPage.End, nextEnd))
	}

	actions := joinsNextActions(output, hostIDFilter, showAllJoins, baseCommand)
	return trace.Wrap(renderNextActions(w, style, actions))
}

func renderJoinHistoryRows(w io.Writer, style textStyle, rows []joinHistoryRow, now time.Time) error {
	for i, row := range rows {
		if i > 0 {
			fmt.Fprintln(w, "")
		}

		details := []keyValue{
			{Key: "TIMESTAMP", Value: formatHistoryTimestamp(row.Timestamp, now)},
			{Key: "RESULT", Value: row.Result},
			{Key: "METHOD", Value: cmp.Or(strings.TrimSpace(row.Method), "n/a")},
			{Key: "ROLE", Value: cmp.Or(strings.TrimSpace(row.Role), "n/a")},
			{Key: "INSTANCE ID", Value: cmp.Or(strings.TrimSpace(row.InstanceID), "n/a")},
			{Key: "ACCOUNT ID", Value: cmp.Or(strings.TrimSpace(row.AccountID), "n/a")},
			{Key: "TOKEN", Value: cmp.Or(strings.TrimSpace(row.TokenName), "n/a")},
		}
		if err := style.numberedBlock(w, i, details); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func renderInventoryText(w io.Writer, output inventoryOutput, hostIDFilter string, showAll bool, baseCommand string) error {
	style := newTextStyle(w)
	now := time.Now().UTC()

	// Summary
	windowDesc := output.Window + " (audit events); nodes: current"
	if output.LimitReached {
		if output.SuggestedLimit > 0 {
			windowDesc += fmt.Sprintf(" [limit: %d/type, use --limit=%d to cover full window]", output.FetchLimit, output.SuggestedLimit)
		} else {
			windowDesc += fmt.Sprintf(" [limit: %d/type, use --limit to increase]", output.FetchLimit)
		}
	}
	var summaryRows [][]string
	if hostIDFilter != "" {
		summaryRows = [][]string{
			{"Query window", windowDesc},
		}
	} else {
		summaryRows = [][]string{
			{"Query window", windowDesc},
			{"Hosts", fmt.Sprintf("%d total, %s online, %s offline, %s failed",
				output.TotalHosts,
				style.good(strconv.Itoa(output.OnlineHosts)),
				style.pendingCount(uint64(output.OfflineHosts)),
				style.failedCount(uint64(output.FailedHosts)))},
			{"Displayed", fmt.Sprintf("%d (range %d-%d)", len(output.Hosts), output.HostPage.Start, output.HostPage.End)},
		}
	}
	if output.CacheSummary != "" {
		summaryRows = append(summaryRows, []string{"Cache", output.CacheSummary})
	}
	if err := renderTable(w, []string{"Summary Item", "Details"}, summaryRows, style.tableWidth); err != nil {
		return trace.Wrap(err)
	}

	if len(output.Hosts) == 0 {
		if hostIDFilter != "" {
			fmt.Fprintf(w, "\n%s\n", style.warning(fmt.Sprintf("No data found for host %s in the selected window.", hostIDFilter)))
		} else {
			fmt.Fprintf(w, "\n%s\n", style.warning("No hosts found for the selected window."))
		}
		return nil
	}

	if hostIDFilter != "" {
		fmt.Fprintf(w, "\n%s\n", style.section("Host:"))
	} else {
		fmt.Fprintf(w, "\n%s\n", style.section("Hosts (sorted by most recent activity):"))
	}

	for i, host := range output.Hosts {
		if i > 0 {
			fmt.Fprintln(w, "")
		}
		details := []keyValue{
			{Key: "HOST", Value: host.DisplayID},
			{Key: "STATE", Value: style.inventoryStateValue(host.State)},
			{Key: "NODE NAME", Value: cmp.Or(host.NodeName, "-")},
			{Key: "NODE UUID", Value: host.HostID},
			{Key: "INSTANCE ID", Value: cmp.Or(host.InstanceID, "-")},
			{Key: "ACCOUNT ID", Value: cmp.Or(host.AccountID, "-")},
			{Key: "REGION", Value: cmp.Or(host.Region, "-")},
			{Key: "METHOD", Value: cmp.Or(host.Method, "-")},
			{Key: "LAST SSM RUN", Value: inventoryTimeValue(host.LastSSMRun, now)},
			{Key: "LAST JOIN", Value: inventoryTimeValue(host.LastJoin, now)},
			{Key: "LAST SEEN", Value: inventoryLastSeenValue(host, now)},
			{Key: "SSM RUNS", Value: fmt.Sprintf("%d (%d success, %d failed)", host.SSMRuns, host.SSMSuccess, host.SSMFailed)},
			{Key: "JOINS", Value: fmt.Sprintf("%d (%d success, %d failed)", host.Joins, host.JoinSuccess, host.JoinFailed)},
		}
		if err := style.numberedBlock(w, i+output.HostPage.Start, details); err != nil {
			return trace.Wrap(err)
		}

		// In show view, render the timeline
		if hostIDFilter != "" {
			if err := renderInventoryTimeline(w, style, host, now, showAll); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	// Pagination
	if output.HostPage.HasNext {
		fmt.Fprintf(w, "\n%s\n", style.info(fmt.Sprintf("  ... %d more hosts available", output.HostPage.Remaining)))
	}

	// Next actions
	actions := inventoryNextActions(output, hostIDFilter, showAll, baseCommand)
	return trace.Wrap(renderNextActions(w, style, actions))
}

func inventoryTimeValue(t time.Time, now time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return formatRelativeTime(t, now)
}

func inventoryLastSeenValue(host inventoryHost, now time.Time) string {
	if host.IsOnline {
		return "now (online)"
	}
	if host.LastSeen.IsZero() {
		return "never"
	}
	return formatRelativeTime(host.LastSeen, now)
}

func (s textStyle) inventoryStateValue(state inventoryHostState) string {
	switch state {
	case inventoryStateOnline, inventoryStateJoinedOnly:
		return s.good(string(state))
	case inventoryStateJoinFailed, inventoryStateSSMFailed:
		return s.bad(string(state))
	case inventoryStateOffline, inventoryStateSSMAttempted:
		return s.warning(string(state))
	default:
		return string(state)
	}
}

type timelineEntry struct {
	Time   time.Time
	Kind   string // "JOIN" or "SSM RUN"
	Status string
	Detail string
	Error  string // join error message (displayed in red)
	Output string // SSM run stdout/stderr (displayed with > quotes)
}

func buildTimeline(host inventoryHost) []timelineEntry {
	entries := make([]timelineEntry, 0, len(host.SSMRecords)+len(host.JoinRecords))

	for _, r := range host.JoinRecords {
		errMsg := ""
		if isJoinFailure(r) {
			errMsg = r.Error
		}
		detail := strings.TrimSpace(r.Method)
		if r.Role != "" {
			detail += "  " + r.Role + " role"
		}
		entries = append(entries, timelineEntry{
			Time:   r.parsedEventTime,
			Kind:   "JOIN",
			Status: r.Code,
			Detail: detail,
			Error:  errMsg,
		})
	}

	for _, r := range host.SSMRecords {
		output := ""
		if isSSMRunFailure(r) {
			output = combineOutput(r.Stdout, r.Stderr)
		}
		var detailParts []string
		if r.ExitCode != "" {
			detailParts = append(detailParts, "exit="+r.ExitCode)
		}
		if r.CommandID != "" {
			detailParts = append(detailParts, r.CommandID)
		}
		detail := strings.Join(detailParts, "  ")
		entries = append(entries, timelineEntry{
			Time:   r.parsedEventTime,
			Kind:   "SSM RUN",
			Status: cmp.Or(r.Status, r.Code),
			Detail: detail,
			Output: output,
		})
	}

	slices.SortFunc(entries, func(a, b timelineEntry) int {
		return compareTimeDesc(a.Time, b.Time)
	})

	return entries
}

func renderInventoryTimeline(w io.Writer, style textStyle, host inventoryHost, now time.Time, showAll bool) error {
	entries := buildTimeline(host)
	if len(entries) == 0 {
		return nil
	}

	displayEntries := entries
	if !showAll && len(displayEntries) > 10 {
		displayEntries = displayEntries[:10]
	}

	fmt.Fprintf(w, "\n%s\n", style.section("Timeline:"))
	for i, e := range displayEntries {
		relative := formatRelativeTime(e.Time, now)
		status := e.Status
		fmt.Fprintf(w, "  #%-3d %-12s %-7s  %s", i+1, relative, e.Kind, status)
		if e.Detail != "" {
			fmt.Fprintf(w, "  %s", e.Detail)
		}
		fmt.Fprintln(w)
		if e.Error != "" {
			for _, line := range strings.Split(e.Error, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					fmt.Fprintf(w, "       %s\n", style.bad(line))
				}
			}
		}
		if e.Output != "" {
			for _, line := range strings.Split(e.Output, "\n") {
				line = strings.ReplaceAll(line, "\r", "")
				fmt.Fprintf(w, "       > %s\n", line)
			}
		}
	}

	if !showAll && len(entries) > 10 {
		fmt.Fprintf(w, "\n  %s\n", style.info(fmt.Sprintf("... %d more events, use --show-all-events to see all", len(entries)-10)))
	}

	return nil
}

func inventoryNextActions(output inventoryOutput, hostIDFilter string, showAll bool, baseCommand string) []nextAction {
	var actions []nextAction

	if output.LimitReached {
		limit := suggestedOrFallbackLimit(output.SuggestedLimit, output.FetchLimit)
		actions = append(actions, nextAction{
			Comment:  "Fetch more events to cover full search window",
			Commands: []string{fmt.Sprintf("%s --limit=%d", baseCommand, limit)},
		})
	}

	if hostIDFilter == "" {
		// ls view
		if output.HostPage.HasNext {
			actions = append(actions, nextAction{
				Comment:  "Next page",
				Commands: []string{withRangeFlag(baseCommand, output.HostPage.End, output.HostPage.End+25)},
			})
		}
		if output.FailedHosts > 0 {
			actions = append(actions, nextAction{
				Comment:  "Show only failing hosts",
				Commands: []string{"tctl discovery inventory ls --state=failed"},
			})
		}
		if len(output.Hosts) > 0 {
			actions = append(actions, nextAction{
				Comment:  "Drill into a specific host",
				Commands: []string{fmt.Sprintf("tctl discovery inventory show %s", output.Hosts[0].DisplayID)},
			})
		}
	} else {
		// show view
		if !showAll {
			actions = append(actions, nextAction{
				Comment:  "Show full event timeline",
				Commands: []string{baseCommand + " --show-all-events"},
			})
		}
		actions = append(actions, nextAction{
			Comment:  "Return to host list",
			Commands: []string{"tctl discovery inventory ls"},
		})
		actions = append(actions, nextAction{
			Comment:  "Check SSM runs for this host",
			Commands: []string{fmt.Sprintf("tctl discovery ssm-runs show %s", hostIDFilter)},
		})
		actions = append(actions, nextAction{
			Comment:  "Check join events for this host",
			Commands: []string{fmt.Sprintf("tctl discovery joins show %s", hostIDFilter)},
		})
	}

	return actions
}

func joinsNextActions(output joinsOutput, hostIDFilter string, showAllJoins bool, baseCommand string) []nextAction {
	if hostIDFilter != "" {
		actions := []nextAction{
			{
				Comment: "Return to joins overview",
				Commands: []string{
					"tctl discovery joins ls",
					"tctl discovery joins ls --last=1h",
					"tctl discovery joins ls --from-utc=2026-02-15T08:00 --to-utc=2026-02-15T20:00",
				},
			},
		}
		if !showAllJoins {
			actions = append(actions, nextAction{
				Comment:  "Show full join history for this host",
				Commands: []string{fmt.Sprintf("tctl discovery joins show %s --show-all-joins", shellQuoteArg(hostIDFilter))},
			})
		}
		actions = append(actions,
			nextAction{
				Comment: "Use machine-readable output",
				Commands: []string{
					fmt.Sprintf("tctl discovery joins show %s --format=json", shellQuoteArg(hostIDFilter)),
				},
			},
			nextAction{
				Comment: "Check SSM runs",
				Commands: []string{
					"tctl discovery ssm-runs ls",
					"tctl discovery ssm-runs ls --last=1h",
					"tctl discovery ssm-runs ls --from-utc=2026-02-15T08:00 --to-utc=2026-02-15T20:00",
				},
			},
			nextAction{
				Comment:  "Inspect the discovery tasks",
				Commands: []string{"tctl discovery tasks ls"},
			},
			nextAction{
				Comment:  "List integrations",
				Commands: []string{"tctl discovery integration ls"},
			},
		)
		return actions
	}

	// List view.
	hostCommand := "tctl discovery joins show <host-id> --show-all-joins"
	if len(output.Hosts) > 0 {
		hostCommand = fmt.Sprintf("tctl discovery joins show %s --show-all-joins", shellQuoteArg(output.Hosts[0].HostID))
	}

	var actions []nextAction
	if output.LimitReached {
		limit := suggestedOrFallbackLimit(output.SuggestedLimit, output.FetchLimit)
		actions = append(actions, nextAction{
			Comment:  "Fetch more events to cover full search window",
			Commands: []string{fmt.Sprintf("%s --limit=%d", baseCommand, limit)},
		})
	}
	actions = append(actions,
		nextAction{
			Comment: "Adjust joins time window",
			Commands: []string{
				"tctl discovery joins ls --last=1h",
				"tctl discovery joins ls --from-utc=2026-02-15T08:00 --to-utc=2026-02-15T20:00",
			},
		},
		nextAction{
			Comment:  "View all joins for a specific host",
			Commands: []string{hostCommand},
		},
	)
	actions = append(actions,
		nextAction{
			Comment:  "Hide hosts with unknown/empty host ID",
			Commands: []string{"tctl discovery joins ls --hide-unknown"},
		},
		nextAction{
			Comment: "Use machine-readable output",
			Commands: []string{
				"tctl discovery joins ls --last=1h --format=json",
			},
		},
		nextAction{
			Comment: "Check SSM runs (use --cluster to group similar runs)",
			Commands: []string{
				"tctl discovery ssm-runs ls",
				"tctl discovery ssm-runs ls --cluster",
				"tctl discovery ssm-runs ls --last=1h",
				"tctl discovery ssm-runs ls --from-utc=2026-02-15T08:00 --to-utc=2026-02-15T20:00",
			},
		},
		nextAction{
			Comment:  "Inspect the discovery tasks",
			Commands: []string{"tctl discovery tasks ls"},
		},
		nextAction{
			Comment:  "List integrations",
			Commands: []string{"tctl discovery integration ls"},
		},
		nextAction{
			Comment:  "View unified host inventory",
			Commands: []string{"tctl discovery inventory ls"},
		},
	)
	return actions
}
