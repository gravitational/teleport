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
	roleWithRODBService, err = authServer.UpsertRole(ctx, roleWithRODBService)
	require.NoError(t, err)

	// Set up User
	teleportUser := "user123"
	user, err := types.NewUser(teleportUser)
	require.NoError(t, err)

	user.AddRole(roleWithRODBService.GetName())
	_, err = authServer.UpsertUser(ctx, user)
	require.NoError(t, err)

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
