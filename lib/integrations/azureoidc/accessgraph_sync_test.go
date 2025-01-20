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

package azureoidc

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/msgraph"
)

type mockClientConfig struct {
	createRoleErr     bool
	assignRoleErr     bool
	fetchPrincipalErr bool
	grantAppRoleErr   bool
}

type mockAzureConfigClient struct {
	cfg mockClientConfig
}

func (c *mockAzureConfigClient) CreateRoleDefinition(ctx context.Context, scope string, roleDefinition armauthorization.RoleDefinition) (string, error) {
	if c.cfg.createRoleErr {
		return "", trace.Errorf("failed to create role definition")
	}
	return "foo", nil
}

func (c *mockAzureConfigClient) CreateRoleAssignment(ctx context.Context, scope string, roleAssignment armauthorization.RoleAssignmentCreateParameters) error {
	if c.cfg.assignRoleErr {
		return trace.Errorf("failed to assign role")
	}
	return nil
}

func (c *mockAzureConfigClient) GetServicePrincipalByAppID(ctx context.Context, appID string) (*msgraph.ServicePrincipal, error) {
	if c.cfg.fetchPrincipalErr {
		return nil, trace.Errorf("failed to fetch principal")
	}
	spID := uuid.NewString()
	appRoleValues := slices.Collect(maps.Keys(requiredGraphRoleNames))
	var roles []*msgraph.AppRole
	for _, rv := range appRoleValues {
		roleID := uuid.NewString()
		roles = append(roles, &msgraph.AppRole{
			ID:    &roleID,
			Value: &rv,
		})
	}
	return &msgraph.ServicePrincipal{
		DirectoryObject: msgraph.DirectoryObject{
			ID: &spID,
		},
		AppRoles: roles,
	}, nil
}

func (c *mockAzureConfigClient) GrantAppRoleToServicePrincipal(ctx context.Context, roleAssignment msgraph.AppRoleAssignment) error {
	if c.cfg.grantAppRoleErr {
		return fmt.Errorf("failed to grant app role")
	}
	return nil
}

func TestAccessGraphAzureConfigOutput(t *testing.T) {
	ctx := context.Background()
	for _, tt := range []struct {
		clientCfg mockClientConfig
		hasError  bool
	}{
		{
			clientCfg: mockClientConfig{},
			hasError:  false,
		},
		{
			clientCfg: mockClientConfig{
				createRoleErr: true,
			},
			hasError: true,
		},
		{
			clientCfg: mockClientConfig{
				assignRoleErr: true,
			},
			hasError: true,
		},
		{
			clientCfg: mockClientConfig{
				fetchPrincipalErr: true,
			},
			hasError: true,
		},
		{
			clientCfg: mockClientConfig{
				grantAppRoleErr: true,
			},
			hasError: true,
		},
	} {
		var buf bytes.Buffer
		req := AccessGraphAzureConfigureRequest{
			ManagedIdentity: "foo",
			RoleName:        "bar",
			SubscriptionID:  "1234567890",
			AutoConfirm:     true,
			stdout:          &buf,
		}
		clt := &mockAzureConfigClient{
			cfg: tt.clientCfg,
		}
		err := ConfigureAccessGraphSyncAzure(ctx, clt, req)
		if !tt.hasError {
			require.NoError(t, err)
		}
	}
}
