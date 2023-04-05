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

package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web/ui"
)

// TestIntegraitionCRUD starts a Teleport cluster and using its Proxy Web server,
// tests the CRUD operations over the Integration resource.
func TestIntegrationCRUD(t *testing.T) {
	ctx := context.Background()

	// Start Teleport Auth and Proxy services
	authProcess, proxyProcess, _ := helpers.MakeTestServers(t)
	authServer := authProcess.GetAuthServer()
	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	roleWithFullAccess, err := types.NewRole("fullaccess", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Namespaces: []string{apidefaults.Namespace},
			Rules: []types.Rule{
				types.NewRule(types.KindIntegration, services.RW()),
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, authServer.UpsertRole(ctx, roleWithFullAccess))

	integrationsEndpoint := strings.Join([]string{"sites", "$site", "integrations"}, "/")

	// Set up User
	username := "fullaccess"
	user, err := types.NewUser(username)
	require.NoError(t, err)

	user.AddRole(roleWithFullAccess.GetName())
	require.NoError(t, authServer.UpsertUser(user))

	userPassword := uuid.NewString()
	require.NoError(t, authServer.UpsertPassword(username, []byte(userPassword)))

	webPack := helpers.LoginWebClient(t, proxyAddr.String(), username, userPassword)

	// List integrations should return empty
	resp, err := webPack.DoRequest(http.MethodGet, integrationsEndpoint, nil)
	require.NoError(t, err)

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, string(respBody))

	listResp := ui.IntegrationsListResponse{}
	require.NoError(t, json.Unmarshal(respBody, &listResp))

	require.Empty(t, listResp.Items)

	// Create Integration
	createIntegrationReq := ui.Integration{
		Name:    "MyAWSAccount",
		SubKind: types.IntegrationSubKindAWSOIDC,
		AWSOIDC: &ui.IntegrationAWSOIDCSpec{
			RoleARN: "arn:aws:iam::123456789012:role/DevTeam",
		},
	}

	resp, err = webPack.DoRequest(http.MethodPost, integrationsEndpoint, createIntegrationReq)
	require.NoError(t, err)

	respBody, err = io.ReadAll(resp.Body)
	require.NoError(t, err)

	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, string(respBody))

	// Get One Integration by name
	resp, err = webPack.DoRequest(http.MethodGet, integrationsEndpoint+"/MyAWSAccount", nil)
	require.NoError(t, err)

	respBody, err = io.ReadAll(resp.Body)
	require.NoError(t, err)

	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, string(respBody))

	integrationResp := ui.Integration{}
	require.NoError(t, json.Unmarshal(respBody, &integrationResp))

	require.Equal(t, ui.Integration{
		Name:    "MyAWSAccount",
		SubKind: types.IntegrationSubKindAWSOIDC,
		AWSOIDC: &ui.IntegrationAWSOIDCSpec{
			RoleARN: "arn:aws:iam::123456789012:role/DevTeam",
		},
	}, integrationResp, string(respBody))

	// Update the integration to another RoleARN
	resp, err = webPack.DoRequest(http.MethodPut, integrationsEndpoint+"/MyAWSAccount", ui.UpdateIntegrationRequest{
		AWSOIDC: &ui.IntegrationAWSOIDCSpec{
			RoleARN: "arn:aws:iam::123456789012:role/OpsTeam",
		},
	})
	require.NoError(t, err)

	respBody, err = io.ReadAll(resp.Body)
	require.NoError(t, err)

	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, string(respBody))

	integrationResp = ui.Integration{}
	require.NoError(t, json.Unmarshal(respBody, &integrationResp))

	require.Equal(t, ui.Integration{
		Name:    "MyAWSAccount",
		SubKind: types.IntegrationSubKindAWSOIDC,
		AWSOIDC: &ui.IntegrationAWSOIDCSpec{
			RoleARN: "arn:aws:iam::123456789012:role/OpsTeam",
		},
	}, integrationResp, string(respBody))

	// Delete resource
	resp, err = webPack.DoRequest(http.MethodDelete, integrationsEndpoint+"/MyAWSAccount", nil)
	require.NoError(t, err)

	respBody, err = io.ReadAll(resp.Body)
	require.NoError(t, err)

	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, string(respBody))

	// Add multiple integrations to test pagination
	// Tests two full pages + 1 item to prevent off by one errors.
	pageSize := 10
	totalItems := pageSize*2 + 1
	for i := 0; i < totalItems; i++ {
		createIntegrationReq := ui.Integration{
			Name:    fmt.Sprintf("AWSIntegration%d", i),
			SubKind: types.IntegrationSubKindAWSOIDC,
			AWSOIDC: &ui.IntegrationAWSOIDCSpec{
				RoleARN: "arn:aws:iam::123456789012:role/DevTeam",
			},
		}

		resp, err = webPack.DoRequest(http.MethodPost, integrationsEndpoint, createIntegrationReq)
		require.NoError(t, err)

		respBody, err = io.ReadAll(resp.Body)
		require.NoError(t, err)

		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode, string(respBody))
	}

	// List integrations should return a full page
	resp, err = webPack.DoRequest(http.MethodGet, integrationsEndpoint+"?limit=10", nil)
	require.NoError(t, err)

	respBody, err = io.ReadAll(resp.Body)
	require.NoError(t, err)

	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, string(respBody))

	listResp = ui.IntegrationsListResponse{}
	require.NoError(t, json.Unmarshal(respBody, &listResp))

	require.Len(t, listResp.Items, pageSize)

	// Requesting the 2nd page should return a full page
	resp, err = webPack.DoRequest(http.MethodGet, integrationsEndpoint+"?limit=10&startKey="+listResp.NextKey, nil)
	require.NoError(t, err)

	respBody, err = io.ReadAll(resp.Body)
	require.NoError(t, err)

	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, string(respBody))

	listResp = ui.IntegrationsListResponse{}
	require.NoError(t, json.Unmarshal(respBody, &listResp))

	require.Len(t, listResp.Items, pageSize)

	// Requesting the 3rd page should return a single item and empty StartKey
	resp, err = webPack.DoRequest(http.MethodGet, integrationsEndpoint+"?limit=10&startKey="+listResp.NextKey, nil)
	require.NoError(t, err)

	respBody, err = io.ReadAll(resp.Body)
	require.NoError(t, err)

	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, string(respBody))

	listResp = ui.IntegrationsListResponse{}
	require.NoError(t, json.Unmarshal(respBody, &listResp))

	require.Len(t, listResp.Items, 1)
	require.Empty(t, listResp.NextKey)
}
