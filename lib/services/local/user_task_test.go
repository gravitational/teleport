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

package local_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/mailgun/holster/v3/clock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types/usertasks"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestCreateUserTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getUserTasksService(t)

	obj, err := usertasks.NewUserTask("obj", &usertasksv1.UserTaskSpec{
		Integration: "my-integration",
		TaskType:    "discover-ec2",
		IssueType:   "ssm_agent_not_running",
		DiscoverEc2: &usertasksv1.DiscoverEC2{},
	})
	require.NoError(t, err)

	// first attempt should succeed
	objOut, err := service.CreateUserTask(ctx, obj)
	require.NoError(t, err)
	require.Equal(t, obj, objOut)

	// second attempt should fail, object already exists
	_, err = service.CreateUserTask(ctx, obj)
	require.Error(t, err)
}

func TestUpsertUserTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getUserTasksService(t)
	obj, err := usertasks.NewUserTask("obj", &usertasksv1.UserTaskSpec{
		Integration: "my-integration",
		TaskType:    "discover-ec2",
		IssueType:   "ssm_agent_not_running",
		DiscoverEc2: &usertasksv1.DiscoverEC2{},
	})
	require.NoError(t, err)
	// the first attempt should succeed
	objOut, err := service.UpsertUserTask(ctx, obj)
	require.NoError(t, err)
	require.Equal(t, obj, objOut)

	// the second attempt should also succeed
	objOut, err = service.UpsertUserTask(ctx, obj)
	require.NoError(t, err)
	require.Equal(t, obj, objOut)
}

func TestGetUserTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getUserTasksService(t)
	prepopulateUserTask(t, service, 1)

	tests := []struct {
		name    string
		key     string
		wantErr bool
		wantObj *usertasksv1.UserTask
	}{
		{
			name:    "object does not exist",
			key:     "dummy",
			wantErr: true,
			wantObj: nil,
		},
		{
			name:    "success",
			key:     getUserTaskObject(t, 0).GetMetadata().GetName(),
			wantErr: false,
			wantObj: getUserTaskObject(t, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fetch a specific object.
			obj, err := service.GetUserTask(ctx, tt.key)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			cmpOpts := []cmp.Option{
				protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
				protocmp.Transform(),
			}
			require.Equal(t, "", cmp.Diff(tt.wantObj, obj, cmpOpts...))
		})
	}
}

func TestUpdateUserTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getUserTasksService(t)
	prepopulateUserTask(t, service, 1)

	expiry := timestamppb.New(clock.Now().Add(30 * time.Minute))

	// Fetch the object from the backend so the revision is populated.
	obj, err := service.GetUserTask(ctx, getUserTaskObject(t, 0).GetMetadata().GetName())
	require.NoError(t, err)
	// update the expiry time
	obj.Metadata.Expires = expiry

	objUpdated, err := service.UpdateUserTask(ctx, obj)
	require.NoError(t, err)
	require.Equal(t, expiry, objUpdated.Metadata.Expires)

	objFresh, err := service.GetUserTask(ctx, obj.Metadata.Name)
	require.NoError(t, err)
	require.Equal(t, expiry, objFresh.Metadata.Expires)
}

func TestUpdateUserTaskMissingRevision(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getUserTasksService(t)
	prepopulateUserTask(t, service, 1)

	expiry := timestamppb.New(clock.Now().Add(30 * time.Minute))

	obj := getUserTaskObject(t, 0)
	obj.Metadata.Expires = expiry

	// Update should be rejected as the revision is missing.
	_, err := service.UpdateUserTask(ctx, obj)
	require.Error(t, err)
}

func TestDeleteUserTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getUserTasksService(t)
	prepopulateUserTask(t, service, 1)

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "object does not exist",
			key:     "dummy",
			wantErr: true,
		},
		{
			name:    "success",
			key:     getUserTaskObject(t, 0).GetMetadata().GetName(),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fetch a specific object.
			err := service.DeleteUserTask(ctx, tt.key)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestListUserTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	counts := []int{0, 1, 5, 10}
	for _, count := range counts {
		t.Run(fmt.Sprintf("count=%v", count), func(t *testing.T) {
			service := getUserTasksService(t)
			prepopulateUserTask(t, service, count)

			t.Run("one page", func(t *testing.T) {
				// Fetch all objects.
				elements, nextToken, err := service.ListUserTasks(ctx, 200, "")
				require.NoError(t, err)
				require.Empty(t, nextToken)
				require.Len(t, elements, count)

				for i := 0; i < count; i++ {
					cmpOpts := []cmp.Option{
						protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
						protocmp.Transform(),
					}
					require.Equal(t, "", cmp.Diff(getUserTaskObject(t, i), elements[i], cmpOpts...))
				}
			})

			t.Run("paginated", func(t *testing.T) {
				// Fetch a paginated list of objects
				elements := make([]*usertasksv1.UserTask, 0)
				nextToken := ""
				for {
					out, token, err := service.ListUserTasks(ctx, 2, nextToken)
					require.NoError(t, err)
					nextToken = token

					elements = append(elements, out...)
					if nextToken == "" {
						break
					}
				}

				for i := 0; i < count; i++ {
					cmpOpts := []cmp.Option{
						protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
						protocmp.Transform(),
					}
					require.Equal(t, "", cmp.Diff(getUserTaskObject(t, i), elements[i], cmpOpts...))
				}
			})
		})
	}
}

func getUserTasksService(t *testing.T) services.UserTasks {
	backend, err := memory.New(memory.Config{
		Context: context.Background(),
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service, err := local.NewUserTasksService(backend)
	require.NoError(t, err)
	return service
}

func getUserTaskObject(t *testing.T, index int) *usertasksv1.UserTask {
	name := fmt.Sprintf("obj%v", index)
	obj, err := usertasks.NewUserTask(name, &usertasksv1.UserTaskSpec{
		Integration: "my-integration",
		TaskType:    "discover-ec2",
		IssueType:   "ssm_agent_not_running",
		DiscoverEc2: &usertasksv1.DiscoverEC2{},
	})
	require.NoError(t, err)
	require.NoError(t, err)

	return obj
}

func prepopulateUserTask(t *testing.T, service services.UserTasks, count int) {
	for i := 0; i < count; i++ {
		_, err := service.CreateUserTask(context.Background(), getUserTaskObject(t, i))
		require.NoError(t, err)
	}
}
