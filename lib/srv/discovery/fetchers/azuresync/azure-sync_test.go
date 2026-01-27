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

package azuresync

import (
	"context"
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
)

type testRoleDefCli struct {
	returnErr bool
	roleDefs  []*armauthorization.RoleDefinition
}

func (t testRoleDefCli) ListRoleDefinitions(ctx context.Context, scope string) ([]*armauthorization.RoleDefinition, error) {
	if t.returnErr {
		return nil, fmt.Errorf("error")
	}
	return t.roleDefs, nil
}

type testRoleAssignCli struct {
	returnErr   bool
	roleAssigns []*armauthorization.RoleAssignment
}

func (t testRoleAssignCli) ListRoleAssignments(ctx context.Context, scope string) ([]*armauthorization.RoleAssignment, error) {
	if t.returnErr {
		return nil, fmt.Errorf("error")
	}
	return t.roleAssigns, nil
}

type testVmCli struct {
	returnErr bool
	vms       []*armcompute.VirtualMachine
}

func (t testVmCli) ListVirtualMachines(ctx context.Context, resourceGroup string) ([]*armcompute.VirtualMachine, error) {
	if t.returnErr {
		return nil, fmt.Errorf("error")
	}
	return t.vms, nil
}

func newRoleDef(id string, name string) *armauthorization.RoleDefinition {
	roleName := "test_role_name"
	action1 := "Microsoft.Compute/virtualMachines/read"
	action2 := "Microsoft.Compute/virtualMachines/*"
	action3 := "Microsoft.Compute/*"
	return &armauthorization.RoleDefinition{
		ID:   &id,
		Name: &name,
		Properties: &armauthorization.RoleDefinitionProperties{
			Permissions: []*armauthorization.Permission{
				{
					Actions: []*string{&action1, &action2},
				},
				{
					Actions: []*string{&action3},
				},
			},
			RoleName: &roleName,
		},
	}
}

func newRoleAssign(id string, name string) *armauthorization.RoleAssignment {
	scope := "test_scope"
	principalId := "test_principal_id"
	roleDefId := "test_role_def_id"
	return &armauthorization.RoleAssignment{
		ID:   &id,
		Name: &name,
		Properties: &armauthorization.RoleAssignmentProperties{
			PrincipalID:      &principalId,
			RoleDefinitionID: &roleDefId,
			Scope:            &scope,
		},
	}
}

func newVm(id string, name string) *armcompute.VirtualMachine {
	return &armcompute.VirtualMachine{
		ID:   &id,
		Name: &name,
	}
}

type mockOIDCCredentials struct {
	integration types.Integration
}

func (m *mockOIDCCredentials) GenerateAzureOIDCToken(ctx context.Context, integration string) (string, error) {
	return "oidc-token", nil
}

func (m *mockOIDCCredentials) GetIntegration(ctx context.Context, name string) (types.Integration, error) {
	if m.integration == nil {
		return nil, trace.NotFound("integration %q not found", name)
	}
	return m.integration, nil
}

func TestNewFetcher(t *testing.T) {
	tests := []struct {
		name            string
		integrationName string
		integration     types.Integration
		wantErr         string
	}{
		{
			name: "empty",
		},
		{
			name:            "valid azure integration",
			integrationName: "azure",
			integration: &types.IntegrationV1{
				ResourceHeader: types.ResourceHeader{
					Kind:    types.KindIntegration,
					SubKind: types.IntegrationSubKindAzureOIDC,
					Version: types.V1,
					Metadata: types.Metadata{
						Name:      "azure",
						Namespace: defaults.Namespace,
					},
				},
				Spec: types.IntegrationSpecV1{
					SubKindSpec: &types.IntegrationSpecV1_AzureOIDC{
						AzureOIDC: &types.AzureOIDCIntegrationSpecV1{
							ClientID: "baz-quux",
							TenantID: "foo-bar",
						},
					},
				},
			},
		},
		{
			name:            "integration not found",
			integrationName: "azure",
			integration:     nil,
			wantErr:         `integration "azure" not found`,
		},
		{
			name:            "invalid integration type",
			integrationName: "azure",
			integration: &types.IntegrationV1{
				ResourceHeader: types.ResourceHeader{
					Kind:    types.KindIntegration,
					SubKind: types.IntegrationSubKindAWSOIDC,
					Version: types.V1,
					Metadata: types.Metadata{
						Name:      "azure",
						Namespace: defaults.Namespace,
					},
				},
				Spec: types.IntegrationSpecV1{
					SubKindSpec: &types.IntegrationSpecV1_AWSOIDC{
						AWSOIDC: &types.AWSOIDCIntegrationSpecV1{
							RoleARN: "arn:aws:iam::123456789012:role/teleport",
						},
					},
				},
			},
			wantErr: `expected "azure" to be an "azure-oidc" integration, was "aws-oidc" instead`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := NewFetcher(Config{
				SubscriptionID:      "dummy-subscription-id",
				Integration:         tt.integrationName,
				DiscoveryConfigName: "dummy-discovery-config",
				OIDCCredentials: &mockOIDCCredentials{
					integration: tt.integration,
				},
			}, t.Context())

			if tt.wantErr == "" {
				require.NoError(t, err)
				require.NotNil(t, f)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
				require.Nil(t, f)
			}
		})
	}

}

func TestPoll(t *testing.T) {
	roleDefs := []*armauthorization.RoleDefinition{
		newRoleDef("id1", "name1"),
	}
	roleAssigns := []*armauthorization.RoleAssignment{
		newRoleAssign("id1", "name1"),
	}
	vms := []*armcompute.VirtualMachine{
		newVm("id1", "name2"),
	}
	roleDefClient := testRoleDefCli{}
	roleAssignClient := testRoleAssignCli{}
	vmClient := testVmCli{}
	fetcher := Fetcher{
		Config:           Config{SubscriptionID: "1234567890"},
		lastResult:       &Resources{},
		roleDefClient:    &roleDefClient,
		roleAssignClient: &roleAssignClient,
		vmClient:         &vmClient,
	}
	ctx := context.Background()
	allFeats := Features{
		RoleDefinitions: true,
		RoleAssignments: true,
		VirtualMachines: true,
	}
	noVmsFeats := allFeats
	noVmsFeats.VirtualMachines = false

	tests := []struct {
		name        string
		returnErr   bool
		roleDefs    []*armauthorization.RoleDefinition
		roleAssigns []*armauthorization.RoleAssignment
		vms         []*armcompute.VirtualMachine
		feats       Features
	}{
		// Process no results from clients
		{
			name:        "WithoutResults",
			returnErr:   false,
			roleDefs:    []*armauthorization.RoleDefinition{},
			roleAssigns: []*armauthorization.RoleAssignment{},
			vms:         []*armcompute.VirtualMachine{},
			feats:       allFeats,
		},
		// Process test results from clients
		{
			name:        "WithResults",
			returnErr:   false,
			roleDefs:    roleDefs,
			roleAssigns: roleAssigns,
			vms:         vms,
			feats:       allFeats,
		},
		// Handle errors from clients
		{
			name:        "PollErrors",
			returnErr:   true,
			roleDefs:    roleDefs,
			roleAssigns: roleAssigns,
			vms:         vms,
			feats:       allFeats,
		},
		// Handle VM features being disabled
		{
			name:        "NoVmsFeats",
			returnErr:   false,
			roleDefs:    roleDefs,
			roleAssigns: roleAssigns,
			vms:         vms,
			feats:       noVmsFeats,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the test data
			roleDefClient.returnErr = tt.returnErr
			roleDefClient.roleDefs = tt.roleDefs
			roleAssignClient.returnErr = tt.returnErr
			roleAssignClient.roleAssigns = tt.roleAssigns
			vmClient.returnErr = tt.returnErr
			vmClient.vms = tt.vms

			// Poll for resources
			resources, err := fetcher.Poll(ctx, tt.feats)

			// Require no error unless otherwise specified
			if tt.returnErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify the results, based on the features set
			require.NotNil(t, resources)
			require.Equal(t, tt.feats.RoleDefinitions == false || len(tt.roleDefs) == 0, len(resources.RoleDefinitions) == 0)
			for idx, resource := range resources.RoleDefinitions {
				roleDef := tt.roleDefs[idx]
				require.Equal(t, *roleDef.ID, resource.Id)
				require.Equal(t, fetcher.SubscriptionID, resource.SubscriptionId)
				require.Equal(t, *roleDef.Properties.RoleName, resource.Name)
				require.Len(t, roleDef.Properties.Permissions, len(resource.Permissions))
			}
			require.Equal(t, tt.feats.RoleAssignments == false || len(tt.roleAssigns) == 0, len(resources.RoleAssignments) == 0)
			for idx, resource := range resources.RoleAssignments {
				roleAssign := tt.roleAssigns[idx]
				require.Equal(t, *roleAssign.ID, resource.Id)
				require.Equal(t, fetcher.SubscriptionID, resource.SubscriptionId)
				require.Equal(t, *roleAssign.Properties.PrincipalID, resource.PrincipalId)
				require.Equal(t, *roleAssign.Properties.RoleDefinitionID, resource.RoleDefinitionId)
				require.Equal(t, *roleAssign.Properties.Scope, resource.Scope)
			}
			require.Equal(t, tt.feats.VirtualMachines == false || len(tt.vms) == 0, len(resources.VirtualMachines) == 0)
			for idx, resource := range resources.VirtualMachines {
				vm := tt.vms[idx]
				require.Equal(t, *vm.ID, resource.Id)
				require.Equal(t, fetcher.SubscriptionID, resource.SubscriptionId)
				require.Equal(t, *vm.Name, resource.Name)
			}
		})
	}
}
