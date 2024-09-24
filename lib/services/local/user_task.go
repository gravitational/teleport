/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package local

import (
	"context"

	"github.com/gravitational/trace"

	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/usertasks"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

type UserTasksService struct {
	service *generic.ServiceWrapper[*usertasksv1.UserTask]
}

const userTasksKey = "user_tasks"

// NewUserTasksService creates a new UserTasksService.
func NewUserTasksService(backend backend.Backend) (*UserTasksService, error) {
	service, err := generic.NewServiceWrapper(
		generic.ServiceWrapperConfig[*usertasksv1.UserTask]{
			Backend:       backend,
			ResourceKind:  types.KindUserTask,
			BackendPrefix: userTasksKey,
			MarshalFunc:   services.MarshalProtoResource[*usertasksv1.UserTask],
			UnmarshalFunc: services.UnmarshalProtoResource[*usertasksv1.UserTask],
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &UserTasksService{service: service}, nil
}

func (s *UserTasksService) ListUserTasks(ctx context.Context, pagesize int64, lastKey string) ([]*usertasksv1.UserTask, string, error) {
	r, nextToken, err := s.service.ListResources(ctx, int(pagesize), lastKey)
	return r, nextToken, trace.Wrap(err)
}

func (s *UserTasksService) GetUserTask(ctx context.Context, name string) (*usertasksv1.UserTask, error) {
	r, err := s.service.GetResource(ctx, name)
	return r, trace.Wrap(err)
}

func (s *UserTasksService) CreateUserTask(ctx context.Context, userTask *usertasksv1.UserTask) (*usertasksv1.UserTask, error) {
	if err := usertasks.ValidateUserTask(userTask); err != nil {
		return nil, trace.Wrap(err)
	}

	r, err := s.service.CreateResource(ctx, userTask)
	return r, trace.Wrap(err)
}

func (s *UserTasksService) UpdateUserTask(ctx context.Context, userTask *usertasksv1.UserTask) (*usertasksv1.UserTask, error) {
	if err := usertasks.ValidateUserTask(userTask); err != nil {
		return nil, trace.Wrap(err)
	}

	r, err := s.service.ConditionalUpdateResource(ctx, userTask)
	return r, trace.Wrap(err)
}

func (s *UserTasksService) UpsertUserTask(ctx context.Context, userTask *usertasksv1.UserTask) (*usertasksv1.UserTask, error) {
	if err := usertasks.ValidateUserTask(userTask); err != nil {
		return nil, trace.Wrap(err)
	}

	r, err := s.service.UpsertResource(ctx, userTask)
	return r, trace.Wrap(err)
}

func (s *UserTasksService) DeleteUserTask(ctx context.Context, name string) error {
	err := s.service.DeleteResource(ctx, name)
	return trace.Wrap(err)
}

func (s *UserTasksService) DeleteAllUserTasks(ctx context.Context) error {
	err := s.service.DeleteAllResources(ctx)
	return trace.Wrap(err)
}
