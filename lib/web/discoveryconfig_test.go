/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web/ui"
)

func TestDiscoveryConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	env := newWebPack(t, 1)
	clusterName := env.server.ClusterName()

	username := uuid.NewString()
	roleRWDiscoveryConfig, err := types.NewRole(
		services.RoleNameForUser(username), types.RoleSpecV6{
			Allow: types.RoleConditions{Rules: []types.Rule{{
				Resources: []string{types.KindDiscoveryConfig},
				Verbs:     services.RW(),
			}}},
		})
	require.NoError(t, err)
	pack := env.proxies[0].authPack(t, username, []types.Role{roleRWDiscoveryConfig})

	getAllEndpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "discoveryconfig")
	t.Run("Get All should return an empty list", func(t *testing.T) {
		resp, err := pack.clt.Get(ctx, getAllEndpoint, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		var listResponse ui.DiscoveryConfigsListResponse
		err = json.Unmarshal(resp.Bytes(), &listResponse)
		require.NoError(t, err)
		require.Empty(t, listResponse.NextKey)
		require.Empty(t, listResponse.Items)
	})

	createEndpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "discoveryconfig")
	t.Run("Create without a name must fai", func(t *testing.T) {
		resp, err := pack.clt.PostJSON(ctx, createEndpoint, ui.DiscoveryConfig{
			DiscoveryGroup: "dg01",
		})
		require.ErrorContains(t, err, "missing discovery config name")
		require.Equal(t, http.StatusBadRequest, resp.Code())
	})

	t.Run("Create without a group must fail", func(t *testing.T) {
		resp, err := pack.clt.PostJSON(ctx, createEndpoint, ui.DiscoveryConfig{
			Name: "dc01",
		})
		require.ErrorContains(t, err, "missing discovery group")
		require.Equal(t, http.StatusBadRequest, resp.Code())
	})

	t.Run("Get One must return not found when it doesn't exist", func(t *testing.T) {
		getDC02Endpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "discoveryconfig", "dc02")
		resp, err := pack.clt.Get(ctx, getDC02Endpoint, nil)
		require.ErrorContains(t, err, "doesn't exist")
		require.Equal(t, http.StatusNotFound, resp.Code())
	})

	t.Run("Create valid", func(t *testing.T) {
		resp, err := pack.clt.PostJSON(ctx, createEndpoint, ui.DiscoveryConfig{
			Name:           "dc01",
			DiscoveryGroup: "dg01",
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		t.Run("Create fails when name already exists", func(t *testing.T) {
			resp, err := pack.clt.PostJSON(ctx, createEndpoint, ui.DiscoveryConfig{
				Name:           "dc01",
				DiscoveryGroup: "dg01",
			})
			require.ErrorContains(t, err, "already exists")
			require.Equal(t, http.StatusConflict, resp.Code())
		})

		getDC01Endpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "discoveryconfig", "dc01")
		t.Run("Get one", func(t *testing.T) {
			resp, err := pack.clt.Get(ctx, getDC01Endpoint, nil)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.Code())

			var discoveryConfigResp ui.DiscoveryConfig
			err = json.Unmarshal(resp.Bytes(), &discoveryConfigResp)
			require.NoError(t, err)
			require.Equal(t, "dg01", discoveryConfigResp.DiscoveryGroup)
			require.Equal(t, "dc01", discoveryConfigResp.Name)
		})

		t.Run("Read status after an update", func(t *testing.T) {
			client, err := env.server.NewClient(auth.TestIdentity{
				I: authz.BuiltinRole{
					Role:     types.RoleDiscovery,
					Username: "disc",
				},
			})
			require.NoError(t, err)
			status := discoveryconfig.Status{
				State:               discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING.String(),
				DiscoveredResources: 1,
				LastSyncTime:        env.clock.Now().UTC(),
			}
			_, err = client.DiscoveryConfigClient().UpdateDiscoveryConfigStatus(ctx, "dc01", status)
			require.NoError(t, err)
			resp, err := pack.clt.Get(ctx, getDC01Endpoint, nil)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.Code())

			var discoveryConfigResp ui.DiscoveryConfig
			err = json.Unmarshal(resp.Bytes(), &discoveryConfigResp)
			require.NoError(t, err)
			assert.Equal(t, "dg01", discoveryConfigResp.DiscoveryGroup)
			assert.Equal(t, "dc01", discoveryConfigResp.Name)
			assert.Equal(t, status, discoveryConfigResp.Status)
		})

		t.Run("Update discovery config", func(t *testing.T) {
			updateDC01Endpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "discoveryconfig", "dc01")
			resp, err = pack.clt.PutJSON(ctx, updateDC01Endpoint, ui.UpdateDiscoveryConfigRequest{
				DiscoveryGroup: "dgAA",
			})
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.Code())

			resp, err = pack.clt.Get(ctx, getDC01Endpoint, nil)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.Code())

			var discoveryConfigResp ui.DiscoveryConfig
			err = json.Unmarshal(resp.Bytes(), &discoveryConfigResp)
			require.NoError(t, err)
			require.Equal(t, "dgAA", discoveryConfigResp.DiscoveryGroup)
			require.Equal(t, "dc01", discoveryConfigResp.Name)
		})

		t.Run("Delete discovery config", func(t *testing.T) {
			deleteDC01Endpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "discoveryconfig", "dc01")
			resp, err = pack.clt.Delete(ctx, deleteDC01Endpoint)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.Code())

			t.Run("Get All should return an empty list", func(t *testing.T) {
				resp, err := pack.clt.Get(ctx, getAllEndpoint, nil)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.Code())

				var listResponse ui.DiscoveryConfigsListResponse
				err = json.Unmarshal(resp.Bytes(), &listResponse)
				require.NoError(t, err)
				require.Empty(t, listResponse.NextKey)
				require.Empty(t, listResponse.Items)
			})
		})
	})

	t.Run("Update must fail when discovery group is not present", func(t *testing.T) {
		updateDC01Endpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "discoveryconfig", "dc01")
		resp, err := pack.clt.PutJSON(ctx, updateDC01Endpoint, ui.UpdateDiscoveryConfigRequest{
			DiscoveryGroup: "",
		})
		require.ErrorContains(t, err, "missing discovery group")
		require.Equal(t, http.StatusBadRequest, resp.Code())
	})

	t.Run("Update must return not found when it doesn't exist", func(t *testing.T) {
		updateDC02Endpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "discoveryconfig", "dc02")
		resp, err := pack.clt.PutJSON(ctx, updateDC02Endpoint, ui.UpdateDiscoveryConfigRequest{
			DiscoveryGroup: "dg01",
		})
		require.ErrorContains(t, err, "doesn't exist")
		require.Equal(t, http.StatusNotFound, resp.Code())
	})

	t.Run("Create multiple and then list all of them", func(t *testing.T) {
		listTestCount := 54
		for i := 0; i < listTestCount; i++ {
			resp, err := pack.clt.PostJSON(ctx, createEndpoint, ui.DiscoveryConfig{
				Name:           fmt.Sprintf("dc-%d", i),
				DiscoveryGroup: "dg01",
			})
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.Code())
		}
		uniqDC := make(map[string]struct{}, listTestCount)
		iterationsCount := listTestCount / 5
		startKey := ""
		for {
			// Add a small limit page to test iteration.
			resp, err := pack.clt.Get(ctx, getAllEndpoint, url.Values{
				"limit":    []string{"5"},
				"startKey": []string{startKey},
			})
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.Code())

			var listResponse ui.DiscoveryConfigsListResponse
			err = json.Unmarshal(resp.Bytes(), &listResponse)
			require.NoError(t, err)
			for _, item := range listResponse.Items {
				uniqDC[item.Name] = struct{}{}
			}
			if listResponse.NextKey == "" {
				break
			}
			iterationsCount--
			require.NotEmpty(t, listResponse.NextKey)
			startKey = listResponse.NextKey
		}
		require.Len(t, uniqDC, listTestCount)
		require.Zero(t, iterationsCount, "invalid number of iterations")
	})

	t.Run("Create valid access graph", func(t *testing.T) {
		resp, err := pack.clt.PostJSON(ctx, createEndpoint, ui.DiscoveryConfig{
			Name:           "dc01",
			DiscoveryGroup: "dg01",
			AccessGraph: &types.AccessGraphSync{
				AWS: []*types.AccessGraphAWSSync{
					{
						Regions:     []string{"us-west-2"},
						Integration: "integrationrole",
					},
				},
			},
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		t.Run("Create fails when name already exists", func(t *testing.T) {
			resp, err := pack.clt.PostJSON(ctx, createEndpoint, ui.DiscoveryConfig{
				Name:           "dc01",
				DiscoveryGroup: "dg01",
				AccessGraph: &types.AccessGraphSync{
					AWS: []*types.AccessGraphAWSSync{
						{
							Regions:     []string{"us-west-2"},
							Integration: "integrationrole",
						},
					},
				},
			})
			require.ErrorContains(t, err, "already exists")
			require.Equal(t, http.StatusConflict, resp.Code())
		})

		getDC01Endpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "discoveryconfig", "dc01")
		t.Run("Get one", func(t *testing.T) {
			resp, err := pack.clt.Get(ctx, getDC01Endpoint, nil)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.Code())

			var discoveryConfigResp ui.DiscoveryConfig
			err = json.Unmarshal(resp.Bytes(), &discoveryConfigResp)
			require.NoError(t, err)
			require.Equal(t, "dg01", discoveryConfigResp.DiscoveryGroup)
			require.Equal(t, "dc01", discoveryConfigResp.Name)
			require.NotNil(t, discoveryConfigResp.AccessGraph)
			expected := &types.AccessGraphSync{
				AWS: []*types.AccessGraphAWSSync{
					{
						Regions:     []string{"us-west-2"},
						Integration: "integrationrole",
					},
				},
			}
			require.Equal(t, expected, discoveryConfigResp.AccessGraph)
		})
	})
}
