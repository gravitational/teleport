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
		fmt.Fprintf(w, "[%d] TASK: %s\n", i+1, item.Name)
		details := []keyValue{
			{Key: "STATE", Value: style.statusValue(item.State)},
			{Key: "TYPE", Value: friendlyTaskType(item.TaskType)},
			{Key: "ISSUE TYPE", Value: item.IssueType},
			{Key: "AFFECTED", Value: fmt.Sprintf("%d", item.Affected)},
			{Key: "INTEGRATION", Value: displayIntegrationName(item.Integration)},
			{Key: "LAST STATE CHANGE", Value: formatRelativeTime(item.LastStateChange, now)},
		}
		if err := renderAlignedKeyValues(w, "    ", details); err != nil {
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
		}
	}

	actions := make([]nextAction, 0, 3)
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

	return actions
}

func renderTaskDetailsText(w io.Writer, task *usertasksv1.UserTask, page, pageSize int, baseCommand string) error {
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
		resourcePage, err = renderEC2Details(w, task, page, pageSize)
	case usertasksapi.TaskTypeDiscoverEKS:
		resourcePage, err = renderEKSDetails(w, task, page, pageSize)
	case usertasksapi.TaskTypeDiscoverRDS:
		resourcePage, err = renderRDSDetails(w, task, page, pageSize)
	case usertasksapi.TaskTypeDiscoverAzureVM:
		resourcePage, err = renderAzureVMDetails(w, task, page, pageSize)
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
			Commands: []string{withPageFlag(baseCommand, 1)},
		})
	} else if resourcePage.HasNext {
		actions = append(actions, nextAction{
			Comment:  "Show next page of affected resources",
			Commands: []string{withPageFlag(baseCommand, resourcePage.NextPage)},
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
					fmt.Sprintf("tctl discovery ssm-runs show %s --since=1h --failed", instanceID),
				},
			})
		}
	}
	actions = append(actions, nextAction{
		Comment:  "Return to discovery overview",
		Commands: []string{"tctl discovery status --state=open"},
	})
	actions = append(actions, nextAction{
		Comment: "Use machine-readable output",
		Commands: []string{
			fmt.Sprintf("tctl discovery tasks show %s --format=json", taskNamePrefix(task.GetMetadata().GetName())),
			fmt.Sprintf("tctl discovery tasks show %s --format=yaml", taskNamePrefix(task.GetMetadata().GetName())),
		},
	})
	return actions
}

func resourceDisplayRange(info pageInfo) (start, end int) {
	if info.Total == 0 || info.Start >= info.End {
		return 0, 0
	}
	return info.Start + 1, info.End
}

func renderEC2Details(w io.Writer, task *usertasksv1.UserTask, page, pageSize int) (pageInfo, error) {
	ec2 := usertasks.EC2InstancesWithURLs(task)
	pageKeys, info, style := paginateMapKeys(w, ec2.Instances, page, pageSize)
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

		fmt.Fprintf(w, "[%d] INSTANCE: %s\n", info.Start+i+1, instance.GetInstanceId())
		details := make([]keyValue, 0, 7)
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
		if err := renderAlignedKeyValues(w, "    ", details); err != nil {
			return info, trace.Wrap(err)
		}
	}

	return info, nil
}

func renderEKSDetails(w io.Writer, task *usertasksv1.UserTask, page, pageSize int) (pageInfo, error) {
	eks := usertasks.EKSClustersWithURLs(task)
	pageKeys, info, style := paginateMapKeys(w, eks.Clusters, page, pageSize)

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

func renderRDSDetails(w io.Writer, task *usertasksv1.UserTask, page, pageSize int) (pageInfo, error) {
	rds := usertasks.RDSDatabasesWithURLs(task)
	pageKeys, info, style := paginateMapKeys(w, rds.Databases, page, pageSize)

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

func renderAzureVMDetails(w io.Writer, task *usertasksv1.UserTask, page, pageSize int) (pageInfo, error) {
	instances := task.GetSpec().GetDiscoverAzureVm().GetInstances()
	pageKeys, info, style := paginateMapKeys(w, instances, page, pageSize)
	now := time.Now().UTC()

	fmt.Fprintf(w, "\n%s\n", style.section("Affected Azure VMs:"))
	if len(pageKeys) == 0 {
		fmt.Fprintf(w, "%s\n", style.warning("No affected Azure VMs for the selected page."))
		return info, nil
	}

	for i, key := range pageKeys {
		vm := instances[key]
		if i > 0 {
			fmt.Fprintln(w, "")
		}

		fmt.Fprintf(w, "[%d] VM ID: %s\n", info.Start+i+1, vm.GetVmId())
		syncTime := "n/a"
		if ts := vm.GetSyncTime(); ts != nil {
			abs := formatTime(ts.AsTime())
			syncTime = fmt.Sprintf("%s (%s)", abs, formatRelativeTime(ts.AsTime(), now))
		}
		details := []keyValue{
			{Key: "NAME", Value: vm.GetName()},
			{Key: "RESOURCE ID", Value: vm.GetResourceId()},
			{Key: "DISCOVERY CONFIG", Value: vm.GetDiscoveryConfig()},
			{Key: "DISCOVERY GROUP", Value: vm.GetDiscoveryGroup()},
			{Key: "SYNC TIME", Value: syncTime},
		}
		if err := renderAlignedKeyValues(w, "    ", details); err != nil {
			return info, trace.Wrap(err)
		}
	}

	return info, nil
}

func renderStatusText(w io.Writer, summary statusSummary) error {
	style := newTextStyle(w)
	now := summary.GeneratedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	fmt.Fprintf(w, "  %s\n", style.section(fmt.Sprintf("User Tasks [%d total, %d open, %d resolved]", summary.TotalTasks, summary.OpenTasks, summary.ResolvedTasks)))
	filterDetails := fmt.Sprintf("Showing %d tasks matching state=%s", summary.FilteredTaskCount, summary.FilteredState)
	if summary.FilteredIntegration != "" {
		filterDetails += fmt.Sprintf(", integration=%s", summary.FilteredIntegration)
	}
	fmt.Fprintf(w, "  %s\n", style.info(filterDetails))
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

	if len(summary.IntegrationResourceStats) > 0 {
		fmt.Fprintf(w, "\n  %s\n", style.section(fmt.Sprintf("Integration Resource Status [%d total]", len(summary.IntegrationResourceStats))))
		integrationRows := integrationStatsRows(summary.IntegrationResourceStats)
		rows := make([][]string, 0, len(integrationRows))
		for _, stats := range integrationRows {
			awaitingJoin := awaitingJoin(resourcesAggregate{
				Found:    stats.Found,
				Enrolled: stats.Enrolled,
				Failed:   stats.Failed,
			})
			rows = append(rows, []string{
				displayIntegrationName(stats.Integration),
				style.discoveredCount(stats.Found),
				style.pendingCount(awaitingJoin),
				style.discoveredCount(stats.Enrolled),
				style.failedCount(stats.Failed),
			})
		}
		if err := renderTable(w, []string{"Integration", "Found", "Awaiting Join", "Enrolled", "Failed"}, rows, 0); err != nil {
			return trace.Wrap(err)
		}
	}

	return trace.Wrap(renderNextActions(w, style, statusNextActions(summary)))
}

func statusNextActions(summary statusSummary) []nextAction {
	inspectCommands := []string{"tctl discovery tasks show <task-id-prefix>"}
	if len(summary.UserTasks) > 0 {
		inspectCommands = []string{
			fmt.Sprintf("tctl discovery tasks show %s", taskNamePrefix(summary.UserTasks[0].Name)),
			"tctl discovery tasks show <task-id-prefix>",
		}
	}
	return []nextAction{
		{
			Comment: "List discovery tasks",
			Commands: []string{
				"tctl discovery tasks ls",
				"tctl discovery tasks ls --state=resolved",
			},
		},
		{
			Comment:  "Investigate particular open task",
			Commands: inspectCommands,
		},
		{
			Comment: "Check SSM runs",
			Commands: []string{
				"tctl discovery ssm-runs ls",
				"tctl discovery ssm-runs ls --since=1h",
				"tctl discovery ssm-runs ls --since=1h --failed",
			},
		},
		{
			Comment:  "Use machine-readable output",
			Commands: []string{"tctl discovery status --format=json"},
		},
	}
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

func renderSSMRunsText(w io.Writer, output ssmRunsOutput, instanceIDFilter string, showAllRuns bool, baseCommand string) error {
	style := newTextStyle(w)
	now := time.Now().UTC()
	rowsLabel := "Failing VM rows shown"
	if instanceIDFilter != "" {
		rowsLabel = "VM rows shown"
	}
	summaryRows := [][]string{
		{"Query window", output.Window},
		{"SSM runs in window", fmt.Sprintf("%d total (%d failed, %d success)", output.TotalRuns, output.FailedRuns, output.SuccessRuns)},
		{"VM status snapshot", fmt.Sprintf("%d total VMs, %d currently failing", output.TotalVMs, output.FailingVMs)},
		{rowsLabel, fmt.Sprintf("%d (page %d, page-size %d)", output.DisplayedVMs, output.VMPage.Page, output.VMPage.PageSize)},
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
			fmt.Fprintf(w, "\n%s\n", style.warning("No failing VMs found for the selected window."))
		}
	} else {
		sectionTitle := "Failing VMs (most recent run failed):"
		if instanceIDFilter != "" {
			sectionTitle = "VM:"
		}
		fmt.Fprintf(w, "\n%s\n", style.section(sectionTitle))
		for i, vm := range output.VMs {
			if i > 0 {
				fmt.Fprintln(w, "")
			}
			if instanceIDFilter != "" && len(output.VMs) == 1 {
				fmt.Fprintf(w, "INSTANCE: %s\n", vm.InstanceID)
			} else {
				fmt.Fprintf(w, "[%d] INSTANCE: %s\n", i+1, vm.InstanceID)
			}

			details := []keyValue{
				{Key: "MOST RECENT", Value: formatRelativeOrTimestamp(vm.MostRecent.parsedEventTime, vm.MostRecent.EventTime, now)},
				{Key: "RESULT", Value: style.statusValue(cmp.Or(vm.MostRecent.Status, vm.MostRecent.Code))},
				{Key: "RUNS", Value: fmt.Sprintf("%d", vm.TotalRuns)},
				{Key: "FAILED", Value: style.failedCount(uint64(vm.FailedRuns))},
				{Key: "REGION", Value: cmp.Or(strings.TrimSpace(vm.MostRecent.Region), "n/a")},
			}
			if err := renderAlignedKeyValues(w, "    ", details); err != nil {
				return trace.Wrap(err)
			}
		}

		fmt.Fprintf(w, "\n%s\n", style.section("Run history:"))
		if instanceIDFilter != "" && len(output.VMs) == 1 {
			rows := buildVMHistoryRows(output.VMs[0], showAllRuns)
			if err := renderSSMRunHistoryRows(w, style, rows, now, true); err != nil {
				return trace.Wrap(err)
			}
		} else {
			for vmIndex, vm := range output.VMs {
				fmt.Fprintf(w, "\n%s\n", style.info(fmt.Sprintf("[%d] VM: %s", vmIndex+1, vm.InstanceID)))
				rows := buildVMHistoryRows(vm, showAllRuns)
				if err := renderSSMRunHistoryRows(w, style, rows, now, false); err != nil {
					return trace.Wrap(err)
				}
			}
		}
	}
	if output.VMPage.HasNext {
		fmt.Fprintf(w, "\n%s\n", style.warning(fmt.Sprintf("More failing VMs available: %d remaining.", output.VMPage.Remaining)))
		fmt.Fprintf(w, "%s %s\n", style.info("Next page:"), withPageFlag(baseCommand, output.VMPage.NextPage))
	}

	actions := ssmRunsNextActions(output, instanceIDFilter, showAllRuns)

	return trace.Wrap(renderNextActions(w, style, actions))
}

func renderSSMRunHistoryRows(w io.Writer, style textStyle, rows []ssmRunHistoryRow, now time.Time, singleInstance bool) error {
	for i, row := range rows {
		if i > 0 {
			fmt.Fprintln(w, "")
		}
		if singleInstance {
			fmt.Fprintf(w, "\n[%d] TIMESTAMP: %s\n", i+1, formatHistoryTimestamp(row.Timestamp, now))
			details := []keyValue{
				{Key: "RESULT", Value: style.statusValue(row.Result)},
				{Key: "COMMAND", Value: cmp.Or(strings.TrimSpace(row.CommandID), "n/a")},
				{Key: "EXIT", Value: cmp.Or(strings.TrimSpace(row.ExitCode), "n/a")},
			}
			if err := renderAlignedKeyValues(w, "    ", details); err != nil {
				return trace.Wrap(err)
			}
			continue
		}

		if len(rows) == 1 {
			fmt.Fprintf(w, "  RUN:\n")
		} else {
			fmt.Fprintf(w, "  RUN %d:\n", i+1)
		}
		details := []keyValue{
			{Key: "TIMESTAMP", Value: formatHistoryTimestamp(row.Timestamp, now)},
			{Key: "RESULT", Value: style.statusValue(row.Result)},
			{Key: "COMMAND", Value: cmp.Or(strings.TrimSpace(row.CommandID), "n/a")},
			{Key: "EXIT", Value: cmp.Or(strings.TrimSpace(row.ExitCode), "n/a")},
		}
		if err := renderAlignedKeyValues(w, "    ", details); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func ssmRunsNextActions(output ssmRunsOutput, instanceIDFilter string, showAllRuns bool) []nextAction {
	if instanceIDFilter != "" {
		// Single-instance view: suggest going back to list view and checking tasks.
		actions := []nextAction{
			{
				Comment: "Return to SSM overview",
				Commands: []string{
					"tctl discovery ssm-runs ls",
					fmt.Sprintf("tctl discovery ssm-runs ls --since=%s", output.Window),
				},
			},
		}
		if !showAllRuns {
			actions = append(actions, nextAction{
				Comment:  "Show full run history for this instance",
				Commands: []string{fmt.Sprintf("tctl discovery ssm-runs show %s --since=%s --show-all-runs", instanceIDFilter, output.Window)},
			})
		}
		actions = append(actions, nextAction{
			Comment:  "Inspect the discovery tasks themselves",
			Commands: []string{"tctl discovery tasks ls --task-type=discover-ec2 --state=open"},
		})
		actions = append(actions, nextAction{
			Comment:  "Use machine-readable output",
			Commands: []string{fmt.Sprintf("tctl discovery ssm-runs show %s --since=%s --format=json", instanceIDFilter, output.Window)},
		})
		return actions
	}

	// List view: suggest drilling into a specific instance.
	instanceCommand := "tctl discovery ssm-runs show <instance-id> --show-all-runs"
	if len(output.VMs) > 0 && output.VMs[0].InstanceID != "unknown" {
		instanceCommand = fmt.Sprintf("tctl discovery ssm-runs show %s --show-all-runs", output.VMs[0].InstanceID)
	}

	return []nextAction{
		{
			Comment: "Start with SSM overview",
			Commands: []string{
				"tctl discovery ssm-runs ls",
				fmt.Sprintf("tctl discovery ssm-runs ls --since=%s", output.Window),
				fmt.Sprintf("tctl discovery ssm-runs ls --since=%s --failed", output.Window),
			},
		},
		{
			Comment:  "View all runs for a specific failing instance",
			Commands: []string{instanceCommand},
		},
		{
			Comment:  "Inspect the discovery tasks themselves",
			Commands: []string{"tctl discovery tasks ls --task-type=discover-ec2 --state=open"},
		},
		{
			Comment: "Use machine-readable output",
			Commands: []string{
				fmt.Sprintf("tctl discovery ssm-runs ls --since=%s --format=json", output.Window),
			},
		},
	}
}
