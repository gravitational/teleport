/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package resources

import (
	"context"
	"fmt"
	"io"

	"github.com/gravitational/trace"

	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/common"
)

// userTasksCollection is a collection of UserTask resources that
// can be written to an io.Writer in a human-readable format.
type userTasksCollection []*usertasksv1.UserTask

func (c userTasksCollection) Resources() []types.Resource {
	out := make([]types.Resource, 0, len(c))
	for _, item := range c {
		out = append(out, types.ProtoResource153ToLegacy(item))
	}
	return out
}

func (c userTasksCollection) WriteText(w io.Writer, verbose bool) error {
	headers := []string{"Name", "Labels", "TaskType", "IssueType", "Integration"}
	var rows [][]string
	for _, item := range c {
		labels := common.FormatLabels(item.GetMetadata().GetLabels(), verbose)
		rows = append(rows, []string{item.Metadata.GetName(), labels, item.Spec.TaskType, item.Spec.IssueType, item.Spec.GetIntegration()})
	}
	t := asciitable.MakeTable(headers, rows...)
	t.SortRowsBy([]int{0}, true)
	return trace.Wrap(t.WriteTo(w))
}

func userTasksHandler() Handler {
	return Handler{
		getHandler:    getUserTask,
		createHandler: createUserTask,
		updateHandler: updateUserTask,
		deleteHandler: deleteUserTask,
		singleton:     false,
		mfaRequired:   false,
		description:   "Task requiring user intervention, e.g. fixing an Integration.",
	}
}

func createUserTask(ctx context.Context, clt *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	resource, err := services.UnmarshalProtoResource[*usertasksv1.UserTask](
		raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if opts.Force {
		if _, err := clt.UserTasksServiceClient().UpsertUserTask(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("user_task %q has been updated\n", resource.GetMetadata().GetName())
		return nil
	}

	if _, err := clt.UserTasksServiceClient().CreateUserTask(ctx, resource); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("user_task %q has been created\n", resource.GetMetadata().GetName())

	return nil
}

func updateUserTask(ctx context.Context, clt *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	resource, err := services.UnmarshalProtoResource[*usertasksv1.UserTask](
		raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := clt.UserTasksClient().UpdateUserTask(ctx, resource); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("user_task %q has been updated\n", resource.GetMetadata().GetName())
	return nil
}

func getUserTask(ctx context.Context, clt *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		resource, err := clt.UserTasksClient().GetUserTask(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return userTasksCollection{resource}, nil
	}

	items, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, token string) ([]*usertasksv1.UserTask, string, error) {
		return clt.UserTasksClient().ListUserTasks(ctx, int64(limit), token, &usertasksv1.ListUserTasksFilters{})
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return userTasksCollection(items), nil
}

func deleteUserTask(ctx context.Context, clt *authclient.Client, ref services.Ref) error {
	if err := clt.UserTasksClient().DeleteUserTask(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("user_task %q has been deleted\n", ref.Name)
	return nil
}
