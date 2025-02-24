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

package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/usertasks"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web/ui"
)

func TestUserTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	env := newWebPack(t, 1)
	clusterName := env.server.ClusterName()

	userWithRW := uuid.NewString()
	roleRWUserTask, err := types.NewRole(
		services.RoleNameForUser(userWithRW), types.RoleSpecV6{
			Allow: types.RoleConditions{Rules: []types.Rule{{
				Resources: []string{types.KindUserTask},
				Verbs:     services.RW(),
			}}},
		})
	require.NoError(t, err)
	pack := env.proxies[0].authPack(t, userWithRW, []types.Role{roleRWUserTask})
	adminClient, err := env.server.NewClient(auth.TestAdmin())
	require.NoError(t, err)

	getAllEndpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "usertask")
	singleItemEndpoint := func(name string) string {
		return pack.clt.Endpoint("webapi", "sites", clusterName, "usertask", name)
	}
	updateStateEndpoint := func(name string) string {
		return pack.clt.Endpoint("webapi", "sites", clusterName, "usertask", name, "state")
	}

	issueTypes := []string{
		usertasks.AutoDiscoverEC2IssueSSMInvocationFailure,
		usertasks.AutoDiscoverEC2IssueSSMScriptFailure,
		usertasks.AutoDiscoverEC2IssueSSMInstanceConnectionLost,
		usertasks.AutoDiscoverEC2IssueSSMInstanceNotRegistered,
		usertasks.AutoDiscoverEC2IssueSSMInstanceUnsupportedOS,
	}
	var userTaskForTest *usertasksv1.UserTask
	for _, issueType := range issueTypes {
		userTask, err := usertasks.NewDiscoverEC2UserTask(&usertasksv1.UserTaskSpec{
			Integration: "my-integration",
			TaskType:    usertasks.TaskTypeDiscoverEC2,
			IssueType:   issueType,
			State:       usertasks.TaskStateOpen,
			DiscoverEc2: &usertasksv1.DiscoverEC2{
				AccountId: "123456789012",
				Region:    "us-east-1",
				Instances: map[string]*usertasksv1.DiscoverEC2Instance{
					"i-123": {
						InstanceId:      "i-123",
						DiscoveryConfig: "dc01",
						DiscoveryGroup:  "dg01",
					},
				},
			},
		})
		require.NoError(t, err)

		_, err = adminClient.UserTasksServiceClient().CreateUserTask(ctx, userTask)
		require.NoError(t, err)
		userTaskForTest = userTask
	}

	t.Run("Get One must return not found when it doesn't exist", func(t *testing.T) {
		resp, err := pack.clt.Get(ctx, singleItemEndpoint("invalid"), nil)
		require.ErrorContains(t, err, "doesn't exist")
		require.Equal(t, http.StatusNotFound, resp.Code())
	})

	t.Run("List by integration", func(t *testing.T) {
		startKey := ""
		var listedTasks []ui.UserTask
		for {
			// Add a small limit page to test iteration.
			resp, err := pack.clt.Get(ctx, getAllEndpoint, url.Values{
				"startKey":    []string{startKey},
				"integration": []string{"my-integration"},
			})
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.Code())

			var listResponse ui.UserTasksListResponse
			err = json.Unmarshal(resp.Bytes(), &listResponse)
			require.NoError(t, err)
			require.NotEmpty(t, listResponse.Items)
			listedTasks = append(listedTasks, listResponse.Items...)

			if listResponse.NextKey == "" {
				break
			}

			startKey = listResponse.NextKey
		}
		require.Len(t, listedTasks, len(issueTypes))
	})

	t.Run("task state moves from OPEN to RESOLVED", func(t *testing.T) {
		userTaskName := userTaskForTest.GetMetadata().GetName()

		// Task starts in OPEN state.
		resp, err := pack.clt.Get(ctx, singleItemEndpoint(userTaskName), nil)
		require.NoError(t, err)
		var userTaskDetailResp ui.UserTaskDetail
		err = json.Unmarshal(resp.Bytes(), &userTaskDetailResp)
		require.NoError(t, err)
		require.Equal(t, "OPEN", userTaskDetailResp.State)
		require.NotEmpty(t, userTaskDetailResp.DiscoverEC2)
		lastStateChangeT0 := userTaskDetailResp.LastStateChange

		// Mark it as resolved.
		_, err = pack.clt.PutJSON(ctx, updateStateEndpoint(userTaskName), ui.UpdateUserTaskStateRequest{
			State: "RESOLVED",
		})
		require.NoError(t, err)

		// Fetch the task again.
		resp, err = pack.clt.Get(ctx, singleItemEndpoint(userTaskName), nil)
		require.NoError(t, err)
		err = json.Unmarshal(resp.Bytes(), &userTaskDetailResp)
		require.NoError(t, err)
		require.NotEmpty(t, userTaskDetailResp.DiscoverEC2)
		require.Equal(t, "RESOLVED", userTaskDetailResp.State)
		// Its last changed state should be updated.
		lastStateChangeT1 := userTaskDetailResp.LastStateChange
		require.True(t, lastStateChangeT1.After(lastStateChangeT0), "last state change was not updated after changing the UserTask state")

		t.Run("List open tasks by integration must not return the resolved task", func(t *testing.T) {
			startKey := ""
			var listedTasks []ui.UserTask
			for {
				// Add a small limit page to test iteration.
				resp, err := pack.clt.Get(ctx, getAllEndpoint, url.Values{
					"startKey":    []string{startKey},
					"integration": []string{"my-integration"},
					"state":       []string{"OPEN"},
					"limit":       []string{"2"},
				})
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.Code())

				var listResponse ui.UserTasksListResponse
				err = json.Unmarshal(resp.Bytes(), &listResponse)
				require.NoError(t, err)
				require.NotEmpty(t, listResponse.Items)
				listedTasks = append(listedTasks, listResponse.Items...)

				if listResponse.NextKey == "" {
					break
				}

				startKey = listResponse.NextKey
			}
			expectedOpenTasks := len(issueTypes) - 1
			require.Len(t, listedTasks, expectedOpenTasks)
		})
		t.Run("List resolved tasks by integration must return the resolved task", func(t *testing.T) {
			startKey := ""
			var listedTasks []ui.UserTask
			for {
				// Add a small limit page to test iteration.
				resp, err := pack.clt.Get(ctx, getAllEndpoint, url.Values{
					"startKey":    []string{startKey},
					"integration": []string{"my-integration"},
					"state":       []string{"RESOLVED"},
					"limit":       []string{"2"},
				})
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.Code())

				var listResponse ui.UserTasksListResponse
				err = json.Unmarshal(resp.Bytes(), &listResponse)
				require.NoError(t, err)
				require.NotEmpty(t, listResponse.Items)
				listedTasks = append(listedTasks, listResponse.Items...)

				if listResponse.NextKey == "" {
					break
				}

				startKey = listResponse.NextKey
			}
			require.Len(t, listedTasks, 1)
		})
	})
}
