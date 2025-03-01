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

package services

import (
	"context"

	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
)

// UserTasks is the interface for managing user tasks resources.
type UserTasks interface {
	// CreateUserTask creates a new user tasks resource.
	CreateUserTask(context.Context, *usertasksv1.UserTask) (*usertasksv1.UserTask, error)
	// UpsertUserTask creates or updates the user tasks resource.
	UpsertUserTask(context.Context, *usertasksv1.UserTask) (*usertasksv1.UserTask, error)
	// GetUserTask returns the user tasks resource by name.
	GetUserTask(ctx context.Context, name string) (*usertasksv1.UserTask, error)
	// ListUserTasks returns the user tasks resources.
	ListUserTasks(ctx context.Context, pageSize int64, nextToken string, filters *usertasksv1.ListUserTasksFilters) ([]*usertasksv1.UserTask, string, error)
	// UpdateUserTask updates the user tasks resource.
	UpdateUserTask(context.Context, *usertasksv1.UserTask) (*usertasksv1.UserTask, error)
	// DeleteUserTask deletes the user tasks resource by name.
	DeleteUserTask(context.Context, string) error
	// DeleteAllUserTasks deletes all user tasks.
	DeleteAllUserTasks(context.Context) error
}

// MarshalUserTask marshals the UserTask object into a JSON byte array.
func MarshalUserTask(object *usertasksv1.UserTask, opts ...MarshalOption) ([]byte, error) {
	return MarshalProtoResource(object, opts...)
}

// UnmarshalUserTask unmarshals the UserTask object from a JSON byte array.
func UnmarshalUserTask(data []byte, opts ...MarshalOption) (*usertasksv1.UserTask, error) {
	return UnmarshalProtoResource[*usertasksv1.UserTask](data, opts...)
}
