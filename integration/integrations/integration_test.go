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

package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web/ui"
)

func TestMain(m *testing.M) {
	modules.SetInsecureTestMode(true)
	os.Exit(m.Run())
}

// TestIntegrationCRUD starts a Teleport cluster and using its Proxy Web server,
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
	roleWithFullAccess, err = authServer.UpsertRole(ctx, roleWithFullAccess)
	require.NoError(t, err)

	integrationsEndpoint, err := url.JoinPath("sites", "$site", "integrations")
	require.NoError(t, err)

	// Set up User
	username := "fullaccess"
	user, err := types.NewUser(username)
	require.NoError(t, err)

	user.AddRole(roleWithFullAccess.GetName())
	_, err = authServer.UpsertUser(ctx, user)
	require.NoError(t, err)

	userPassword := uuid.NewString()
	require.NoError(t, authServer.UpsertPassword(username, []byte(userPassword)))

	webPack := helpers.LoginWebClient(t, proxyAddr.String(), username, userPassword)

	// List integrations should return empty
	respStatusCode, respBody := webPack.DoRequest(t, http.MethodGet, integrationsEndpoint, nil)
	require.Equal(t, http.StatusOK, respStatusCode, string(respBody))

	listResp := ui.IntegrationsListResponse{}
	require.NoError(t, json.Unmarshal(respBody, &listResp))

	require.Empty(t, listResp.Items)

	// Create Integration
	createIntegrationReq := ui.Integration{
		Name:    "MyAWSAccount",
		SubKind: types.IntegrationSubKindAWSOIDC,
		AWSOIDC: &ui.IntegrationAWSOIDCSpec{
			RoleARN:        "arn:aws:iam::123456789012:role/DevTeam",
			IssuerS3Bucket: "my-bucket",
			IssuerS3Prefix: "prefix",
		},
	}

	respStatusCode, respBody = webPack.DoRequest(t, http.MethodPost, integrationsEndpoint, createIntegrationReq)
	require.Equal(t, http.StatusOK, respStatusCode, string(respBody))

	// Create Integration without S3 location
	createIntegrationWithoutS3LocationReq := ui.Integration{
		Name:    "MyAWSAccountWithoutS3",
		SubKind: types.IntegrationSubKindAWSOIDC,
		AWSOIDC: &ui.IntegrationAWSOIDCSpec{
			RoleARN: "arn:aws:iam::123456789012:role/DevTeam",
		},
	}

	respStatusCode, respBody = webPack.DoRequest(t, http.MethodPost, integrationsEndpoint, createIntegrationWithoutS3LocationReq)
	require.Equal(t, http.StatusOK, respStatusCode, string(respBody))

	// Get One Integration by name
	respStatusCode, respBody = webPack.DoRequest(t, http.MethodGet, integrationsEndpoint+"/MyAWSAccount", nil)
	require.Equal(t, http.StatusOK, respStatusCode, string(respBody))

	integrationResp := ui.Integration{}
	require.NoError(t, json.Unmarshal(respBody, &integrationResp))

	require.Equal(t, ui.Integration{
		Name:    "MyAWSAccount",
		SubKind: types.IntegrationSubKindAWSOIDC,
		AWSOIDC: &ui.IntegrationAWSOIDCSpec{
			RoleARN:        "arn:aws:iam::123456789012:role/DevTeam",
			IssuerS3Bucket: "my-bucket",
			IssuerS3Prefix: "prefix",
		},
	}, integrationResp, string(respBody))

	// Update the integration to another RoleARN
	respStatusCode, respBody = webPack.DoRequest(t, http.MethodPut, integrationsEndpoint+"/MyAWSAccount", ui.UpdateIntegrationRequest{
		AWSOIDC: &ui.IntegrationAWSOIDCSpec{
			RoleARN:        "arn:aws:iam::123456789012:role/OpsTeam",
			IssuerS3Bucket: "my-bucket",
			IssuerS3Prefix: "my-prefix",
		},
	})
	require.Equal(t, http.StatusOK, respStatusCode, string(respBody))

	integrationResp = ui.Integration{}
	require.NoError(t, json.Unmarshal(respBody, &integrationResp))

	require.Equal(t, ui.Integration{
		Name:    "MyAWSAccount",
		SubKind: types.IntegrationSubKindAWSOIDC,
		AWSOIDC: &ui.IntegrationAWSOIDCSpec{
			RoleARN:        "arn:aws:iam::123456789012:role/OpsTeam",
			IssuerS3Bucket: "my-bucket",
			IssuerS3Prefix: "my-prefix",
		},
	}, integrationResp, string(respBody))

	// Update the integration to remove the S3 Location
	respStatusCode, respBody = webPack.DoRequest(t, http.MethodPut, integrationsEndpoint+"/MyAWSAccount", ui.UpdateIntegrationRequest{
		AWSOIDC: &ui.IntegrationAWSOIDCSpec{
			RoleARN: "arn:aws:iam::123456789012:role/OpsTeam2",
		},
	})
	require.Equal(t, http.StatusOK, respStatusCode, string(respBody))

	integrationResp = ui.Integration{}
	require.NoError(t, json.Unmarshal(respBody, &integrationResp))

	require.Equal(t, ui.Integration{
		Name:    "MyAWSAccount",
		SubKind: types.IntegrationSubKindAWSOIDC,
		AWSOIDC: &ui.IntegrationAWSOIDCSpec{
			RoleARN: "arn:aws:iam::123456789012:role/OpsTeam2",
		},
	}, integrationResp, string(respBody))

	// Delete resource
	respStatusCode, respBody = webPack.DoRequest(t, http.MethodDelete, integrationsEndpoint+"/MyAWSAccount", nil)
	require.Equal(t, http.StatusOK, respStatusCode, string(respBody))

	// Add multiple integrations to test pagination
	// Tests two full pages + 1 item to prevent off by one errors.
	pageSize := 10
	totalItems := pageSize*2 + 1
	for i := 0; i < totalItems; i++ {
		createIntegrationReq := ui.Integration{
			Name:    fmt.Sprintf("AWSIntegration%d", i),
			SubKind: types.IntegrationSubKindAWSOIDC,
			AWSOIDC: &ui.IntegrationAWSOIDCSpec{
				RoleARN:        "arn:aws:iam::123456789012:role/DevTeam",
				IssuerS3Bucket: "my-bucket",
				IssuerS3Prefix: "my-prefix",
			},
		}

		respStatusCode, respBody := webPack.DoRequest(t, http.MethodPost, integrationsEndpoint, createIntegrationReq)
		require.Equal(t, http.StatusOK, respStatusCode, string(respBody))
	}

	// List integrations should return a full page
	respStatusCode, respBody = webPack.DoRequest(t, http.MethodGet, integrationsEndpoint+"?limit=10", nil)
	require.Equal(t, http.StatusOK, respStatusCode, string(respBody))

	listResp = ui.IntegrationsListResponse{}
	require.NoError(t, json.Unmarshal(respBody, &listResp))

	require.Len(t, listResp.Items, pageSize)

	// Requesting the 2nd page should return a full page
	respStatusCode, respBody = webPack.DoRequest(t, http.MethodGet, integrationsEndpoint+"?limit=10&startKey="+listResp.NextKey, nil)
	require.Equal(t, http.StatusOK, respStatusCode, string(respBody))

	listResp = ui.IntegrationsListResponse{}
	require.NoError(t, json.Unmarshal(respBody, &listResp))

	require.Len(t, listResp.Items, pageSize)

	// Requesting the 3rd page should return two items and empty StartKey
	respStatusCode, respBody = webPack.DoRequest(t, http.MethodGet, integrationsEndpoint+"?limit=10&startKey="+listResp.NextKey, nil)
	require.Equal(t, http.StatusOK, respStatusCode, string(respBody))

	listResp = ui.IntegrationsListResponse{}
	require.NoError(t, json.Unmarshal(respBody, &listResp))

	require.Len(t, listResp.Items, 2)
	require.Empty(t, listResp.NextKey)
}
