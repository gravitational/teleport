/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
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

	// Get All should return an empty list
	getAllEndpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "discoveryconfig")
	resp, err := pack.clt.Get(ctx, getAllEndpoint, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	var listResponse ui.DiscoveryConfigsListResponse
	err = json.Unmarshal(resp.Bytes(), &listResponse)
	require.NoError(t, err)
	require.Empty(t, listResponse.NextKey)
	require.Empty(t, listResponse.Items)

	// Create without a name must fail.
	createEndpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "discoveryconfig")
	resp, err = pack.clt.PostJSON(ctx, createEndpoint, ui.DiscoveryConfig{
		DiscoveryGroup: "dg01",
	})
	require.ErrorContains(t, err, "missing discovery config name")
	require.Equal(t, http.StatusBadRequest, resp.Code())

	// Create without a group must fail.reateEndpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "discoveryconfig")
	resp, err = pack.clt.PostJSON(ctx, createEndpoint, ui.DiscoveryConfig{
		Name: "dc01",
	})
	require.ErrorContains(t, err, "missing discovery group")
	require.Equal(t, http.StatusBadRequest, resp.Code())

	// Create valid.
	resp, err = pack.clt.PostJSON(ctx, createEndpoint, ui.DiscoveryConfig{
		Name:           "dc01",
		DiscoveryGroup: "dg01",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	// Create invalid when name already exists.
	resp, err = pack.clt.PostJSON(ctx, createEndpoint, ui.DiscoveryConfig{
		Name:           "dc01",
		DiscoveryGroup: "dg01",
	})
	require.ErrorContains(t, err, "already exists")
	require.Equal(t, http.StatusConflict, resp.Code())

	// Get One.
	getDC01Endpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "discoveryconfig", "dc01")
	resp, err = pack.clt.Get(ctx, getDC01Endpoint, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	var discoveryConfigResp ui.DiscoveryConfig
	err = json.Unmarshal(resp.Bytes(), &discoveryConfigResp)
	require.NoError(t, err)
	require.Equal(t, "dg01", discoveryConfigResp.DiscoveryGroup)
	require.Equal(t, "dc01", discoveryConfigResp.Name)

	// Get One must return not found when it doesn't exist.
	getDC02Endpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "discoveryconfig", "dc02")
	resp, err = pack.clt.Get(ctx, getDC02Endpoint, nil)
	require.ErrorContains(t, err, "doesn't exist")
	require.Equal(t, http.StatusNotFound, resp.Code())

	// Update discovery config.
	updateDC01Endpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "discoveryconfig", "dc01")
	resp, err = pack.clt.PutJSON(ctx, updateDC01Endpoint, ui.UpdateDiscoveryConfigRequest{
		DiscoveryGroup: "dgAA",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	resp, err = pack.clt.Get(ctx, getDC01Endpoint, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	err = json.Unmarshal(resp.Bytes(), &discoveryConfigResp)
	require.NoError(t, err)
	require.Equal(t, "dgAA", discoveryConfigResp.DiscoveryGroup)
	require.Equal(t, "dc01", discoveryConfigResp.Name)

	// Update must fail when discovery group is not present.
	updateDC01Endpoint = pack.clt.Endpoint("webapi", "sites", clusterName, "discoveryconfig", "dc01")
	resp, err = pack.clt.PutJSON(ctx, updateDC01Endpoint, ui.UpdateDiscoveryConfigRequest{
		DiscoveryGroup: "",
	})
	require.ErrorContains(t, err, "missing discovery group")
	require.Equal(t, http.StatusBadRequest, resp.Code())

	// Update must return not found when it doesn't exist.
	updateDC02Endpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "discoveryconfig", "dc02")
	resp, err = pack.clt.PutJSON(ctx, updateDC02Endpoint, ui.UpdateDiscoveryConfigRequest{
		DiscoveryGroup: "dg01",
	})
	require.ErrorContains(t, err, "doesn't exist")
	require.Equal(t, http.StatusNotFound, resp.Code())

	// Delete discovery config.
	deleteDC01Endpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "discoveryconfig", "dc01")
	resp, err = pack.clt.Delete(ctx, deleteDC01Endpoint)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	// Get All should return an empty list.
	getAllEndpoint = pack.clt.Endpoint("webapi", "sites", clusterName, "discoveryconfig")
	resp, err = pack.clt.Get(ctx, getAllEndpoint, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	err = json.Unmarshal(resp.Bytes(), &listResponse)
	require.NoError(t, err)
	require.Empty(t, listResponse.NextKey)
	require.Empty(t, listResponse.Items)

	// Create multiple and then list all of them.
	listTestCount := 54
	for i := 0; i < listTestCount; i++ {
		resp, err = pack.clt.PostJSON(ctx, createEndpoint, ui.DiscoveryConfig{
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
		resp, err = pack.clt.Get(ctx, getAllEndpoint, url.Values{
			"limit":    []string{"5"},
			"startKey": []string{startKey},
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

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
	require.Equal(t, listTestCount, len(uniqDC))
	require.Zero(t, iterationsCount, "invalid number of iterations")
}
