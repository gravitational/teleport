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
	"io"
	"slices"

	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"

	"github.com/gravitational/trace"
)

func (c *Command) runTasksList(ctx context.Context, client discoveryClient) error {
	state, err := normalizeTaskState(c.tasksListState)
	if err != nil {
		return trace.Wrap(err)
	}

	tasks, err := listUserTasks(ctx, client, c.tasksListIntegration, state)
	if err != nil {
		return trace.Wrap(err)
	}
	tasks = filterUserTasks(tasks, taskFilters{
		State:       state,
		TaskType:    c.tasksListTaskType,
		IssueType:   c.tasksListIssueType,
		Integration: c.tasksListIntegration,
	})

	slices.SortFunc(tasks, func(a, b *usertasksv1.UserTask) int {
		if c := compareTimeDesc(taskLastStateChange(a), taskLastStateChange(b)); c != 0 {
			return c
		}
		return cmp.Compare(a.GetMetadata().GetName(), b.GetMetadata().GetName())
	})

	items := toTaskListItems(tasks)
	listOutput := tasksListOutput{
		Total: len(items),
		Items: items,
	}
	return trace.Wrap(writeOutputByFormat(c.output(), c.tasksListFormat, listOutput, func(w io.Writer) error {
		return renderTasksListText(w, items, taskListHintsInput{
			State:       state,
			Integration: c.tasksListIntegration,
			TaskType:    c.tasksListTaskType,
			IssueType:   c.tasksListIssueType,
		})
	}))
}

func (c *Command) runTaskShow(ctx context.Context, client discoveryClient) error {
	start, end, err := parseRange(c.tasksShowRange)
	if err != nil {
		return trace.Wrap(err)
	}

	tasks, err := listUserTasks(ctx, client, "", "")
	if err != nil {
		return trace.Wrap(err)
	}
	task, err := findTaskByNamePrefix(tasks, c.tasksShowName)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(writeOutputByFormat(c.output(), c.tasksShowFormat, task, func(w io.Writer) error {
		return renderTaskDetailsText(w, task, start, end, buildTaskShowCommand(c))
	}))
}
