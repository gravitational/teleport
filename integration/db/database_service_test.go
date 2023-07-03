// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package db

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web/ui"
)

type listDatabaseServicesResp struct {
	Items []ui.DatabaseService `json:"items"`
}

func TestDatabaseServiceHeartbeat(t *testing.T) {
	ctx := context.Background()

	// Start Teleport Auth and Proxy services
	authProcess, proxyProcess, provisionToken := helpers.MakeTestServers(t)
	authServer := authProcess.GetAuthServer()

	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	resMatchers := []services.ResourceMatcher{
		{Labels: types.Labels{"env": []string{"stg", "qa"}}},
		{Labels: types.Labels{"aws-tag": []string{"dev"}}},
	}

	// Start Teleport Database Service
	helpers.MakeTestDatabaseServer(t, *proxyAddr, provisionToken, resMatchers, servicecfg.Database{
		Name:     "dummydb",
		Protocol: defaults.ProtocolPostgres,
		URI:      "127.0.0.1:0",
	})

	roleWithRODBService, err := types.NewRole("ro_dbservices", types.RoleSpecV6{
		Allow: types.RoleConditions{
			DatabaseServiceLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			Rules: []types.Rule{
				types.NewRule(types.KindDatabaseService, services.RO()),
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, authServer.UpsertRole(ctx, roleWithRODBService))

	// Set up User
	teleportUser := "user123"
	user, err := types.NewUser(teleportUser)
	require.NoError(t, err)

	user.AddRole(roleWithRODBService.GetName())
	require.NoError(t, authServer.UpsertUser(user))

	userPassword := uuid.NewString()
	require.NoError(t, authServer.UpsertPassword(teleportUser, []byte(userPassword)))

	webPack := helpers.LoginWebClient(t, proxyAddr.String(), teleportUser, userPassword)

	// List Database Services
	listDBServicesEndpoint := strings.Join([]string{"sites", "$site", "databaseservices"}, "/")
	respStatusCode, respBody := webPack.DoRequest(t, http.MethodGet, listDBServicesEndpoint, nil)
	require.Equal(t, http.StatusOK, respStatusCode, string(respBody))

	var listResp listDatabaseServicesResp
	require.NoError(t, json.Unmarshal(respBody, &listResp))

	require.Len(t, listResp.Items, 1)
	dbServic01 := listResp.Items[0]
	expectedDBResourceMatcher := []*types.DatabaseResourceMatcher{
		{Labels: &types.Labels{"env": []string{"stg", "qa"}}},
		{Labels: &types.Labels{"aws-tag": []string{"dev"}}},
	}

	require.Equal(t, expectedDBResourceMatcher, dbServic01.ResourceMatchers)
}
