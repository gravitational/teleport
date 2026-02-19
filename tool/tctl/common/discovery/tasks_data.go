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
	"slices"
	"strings"
	"time"

	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	usertasksapi "github.com/gravitational/teleport/api/types/usertasks"
	"github.com/gravitational/teleport/api/utils/clientutils"

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

func listUserTasks(ctx context.Context, client discoveryClient, integration, taskState string) ([]*usertasksv1.UserTask, error) {
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
