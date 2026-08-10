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

package web

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func TestListUnifiedInstances(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	env := newWebPack(t, 1, func(cfg *WebPackOptions) {
		cfg.enableAuthCache = true
	})
	clusterName := env.server.ClusterName()

	// Create mock instances
	instances := []*types.InstanceV1{
		{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{Name: "instance-1"},
			},
			Spec: types.InstanceSpecV1{
				Hostname: "host-1",
				Version:  "18.1.0",
				Services: []types.SystemRole{types.RoleNode},
			},
		},
		{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{Name: "instance-2"},
			},
			Spec: types.InstanceSpecV1{
				Hostname: "host-2",
				Version:  "18.2.0",
				Services: []types.SystemRole{types.RoleProxy, types.RoleAuth},
			},
		},
		{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{Name: "instance-3"},
			},
			Spec: types.InstanceSpecV1{
				Hostname: "host-3",
				Version:  "18.3.0",
				Services: []types.SystemRole{types.RoleDatabase},
			},
		},
		{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{Name: "instance-4"},
			},
			Spec: types.InstanceSpecV1{
				Hostname: "host-4",
				Version:  "18.4.0",
				Services: []types.SystemRole{types.RoleNode},
			},
		},
		{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{Name: "instance-5"},
			},
			Spec: types.InstanceSpecV1{
				Hostname: "host-5",
				Version:  "18.5.0",
				Services: []types.SystemRole{types.RoleProxy},
			},
		},
		{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{Name: "instance-6"},
			},
			Spec: types.InstanceSpecV1{
				Hostname: "host-6",
				Version:  "18.6.0",
				Services: []types.SystemRole{types.RoleAuth},
			},
		},
	}
	for _, instance := range instances {
		require.NoError(t, env.server.Auth().UpsertInstance(ctx, instance))
	}

	// Create mock bot instances
	bots := []*machineidv1.BotInstance{
		{
			Metadata: &headerv1.Metadata{Name: "bot-1"},
			Spec: &machineidv1.BotInstanceSpec{
				BotName:    "bot-1",
				InstanceId: "bot-1",
			},
			Status: &machineidv1.BotInstanceStatus{
				LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{
					{
						RecordedAt: timestamppb.Now(),
						Version:    "18.7.0",
					},
				},
			},
		},
		{
			Metadata: &headerv1.Metadata{Name: "bot-2"},
			Spec: &machineidv1.BotInstanceSpec{
				BotName:    "bot-2",
				InstanceId: "bot-2",
			},
			Status: &machineidv1.BotInstanceStatus{
				LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{
					{
						RecordedAt: timestamppb.Now(),
						Version:    "18.8.0",
					},
				},
			},
		},
		{
			Metadata: &headerv1.Metadata{Name: "bot-3"},
			Spec: &machineidv1.BotInstanceSpec{
				BotName:    "bot-3",
				InstanceId: "bot-3",
			},
			Status: &machineidv1.BotInstanceStatus{
				LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{
					{
						RecordedAt: timestamppb.Now(),
						Version:    "18.9.0",
					},
				},
			},
		},
	}
	for _, bot := range bots {
		_, err := env.server.Auth().BotInstance.CreateBotInstance(ctx, bot)
		require.NoError(t, err)
	}

	// Create user with required permissions
	username := "test-user"
	role, err := types.NewRole(services.RoleNameForUser(username), types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindInstance, []string{types.VerbRead, types.VerbList}),
				types.NewRule(types.KindBotInstance, []string{types.VerbRead, types.VerbList}),
			},
		},
	})
	require.NoError(t, err)
	pack := env.proxies[0].authPack(t, username, []types.Role{role})

	endpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "instances")

	// The initial requests are in the eventually to ensure that the inventory
	// cache is healthy, has processed all put events, and that all the instances
	// are present.
	var listResp listUnifiedInstancesResponse
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		// Test listing with no params
		resp, err := pack.clt.Get(ctx, endpoint, url.Values{})
		require.NoError(t, err)

		require.NoError(t, json.Unmarshal(resp.Bytes(), &listResp))
		require.Len(t, listResp.Instances, 9, "should have 9 items (6 instances + 3 bots)")
	}, 10*time.Second, 100*time.Millisecond, "inventory cache failed to become healthy and populated")

	// Test pagination
	// Get page of 4
	resp, err := pack.clt.Get(ctx, endpoint, url.Values{
		"limit": []string{"4"},
	})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(resp.Bytes(), &listResp))
	require.Len(t, listResp.Instances, 4, "should have 4 items")
	require.NotEmpty(t, listResp.StartKey, "should have a startKey in the response")

	// Get page of 5
	resp, err = pack.clt.Get(ctx, endpoint, url.Values{
		"limit":    []string{"5"},
		"startKey": []string{listResp.StartKey},
	})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(resp.Bytes(), &listResp))
	require.Len(t, listResp.Instances, 5, "should have 5 items in second page")
	// First item on the second page should be instance-2
	require.Equal(t, "instance-2", listResp.Instances[0].ID)
	require.Empty(t, listResp.StartKey, "should not have a startKey in the response")

	// Test sorting by version descending
	resp, err = pack.clt.Get(ctx, endpoint, url.Values{
		"sort": []string{"version:desc"},
	})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(resp.Bytes(), &listResp))
	require.Len(t, listResp.Instances, 9)
	require.Equal(t, "bot-3", listResp.Instances[0].ID)      // 18.9.0
	require.Equal(t, "bot-2", listResp.Instances[1].ID)      // 18.8.0
	require.Equal(t, "instance-4", listResp.Instances[5].ID) // 18.4.0
	require.Equal(t, "instance-1", listResp.Instances[8].ID) // 18.1.0

	// Test searching
	resp, err = pack.clt.Get(ctx, endpoint, url.Values{
		"search": []string{"host-1"},
	})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(resp.Bytes(), &listResp))
	require.Len(t, listResp.Instances, 1, "should find 1 item matching 'host-1' search")
	require.Equal(t, "instance-1", listResp.Instances[0].ID)

	// Test a search with no matches
	resp, err = pack.clt.Get(ctx, endpoint, url.Values{
		"search": []string{"nonexistentinstance"},
	})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(resp.Bytes(), &listResp))
	require.Empty(t, listResp.Instances, "should find no items matching the search")
}
