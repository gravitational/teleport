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

		details = append(details, keyValue{Key: "DISCOVERY CONFIG", Value: cmp.Or(strings.TrimSpace(instance.GetDiscoveryConfig()), placeholderNA)})
		details = append(details, keyValue{Key: "DISCOVERY GROUP", Value: cmp.Or(strings.TrimSpace(instance.GetDiscoveryGroup()), placeholderNA)})

		syncTime := placeholderNA
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
