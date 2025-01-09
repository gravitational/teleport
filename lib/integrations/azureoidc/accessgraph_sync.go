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
	"context"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/cloud/provisioning"
	"github.com/gravitational/teleport/lib/msgraph"
	libslices "github.com/gravitational/teleport/lib/utils/slices"
)

// graphAppID is the pre-defined application ID of the Graph API
// Ref: [https://learn.microsoft.com/en-us/troubleshoot/entra/entra-id/governance/verify-first-party-apps-sign-in#application-ids-of-commonly-used-microsoft-applications].
const graphAppID = "00000003-0000-0000-c000-000000000000"

var requiredGraphRoleNames = map[string]bool{
	"User.ReadBasic.All": true,
	"Group.Read.All":     true,
	"Directory.Read.All": true,
	"User.Read.All":      true,
	"Policy.Read.All":    true,
}

type AccessGraphAzureConfigureClient interface {
	CreateRoleDefinition(ctx context.Context, scope string, roleDefinition armauthorization.RoleDefinition) (string, error)
	CreateRoleAssignment(ctx context.Context, scope string, roleAssignment armauthorization.RoleAssignmentCreateParameters) error
	GetServicePrincipalByAppID(ctx context.Context, appID string) (*msgraph.ServicePrincipal, error)
	GrantAppRoleToServicePrincipal(ctx context.Context, roleAssignment msgraph.AppRoleAssignment) error
}

type azureConfigClient struct {
	roleDefCli    *armauthorization.RoleDefinitionsClient
	roleAssignCli *armauthorization.RoleAssignmentsClient
	graphCli      *msgraph.Client
}

func NewAzureConfigClient(subscriptionID string) (AccessGraphAzureConfigureClient, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleDefCli, err := armauthorization.NewRoleDefinitionsClient(cred, nil)
	if err != nil {
		return nil, trace.BadParameter("failed to create role definitions client: %v", err)
	}
	roleAssignCli, err := armauthorization.NewRoleAssignmentsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, trace.BadParameter("failed to create role assignments client: %v", err)
	}
	graphCli, err := msgraph.NewClient(msgraph.Config{
		TokenProvider: cred,
	})
	if err != nil {
		return nil, trace.BadParameter("failed to create msgraph client: %v", err)
	}
	return &azureConfigClient{
		roleDefCli:    roleDefCli,
		roleAssignCli: roleAssignCli,
		graphCli:      graphCli,
	}, nil
}

func (c *azureConfigClient) CreateRoleDefinition(ctx context.Context, scope string, roleDefinition armauthorization.RoleDefinition) (string, error) {
	newUuid, err := uuid.NewRandom()
	if err != nil {
		return "", trace.Wrap(err)
	}
	roleDefID := newUuid.String()
	roleRes, err := c.roleDefCli.CreateOrUpdate(ctx, scope, roleDefID, roleDefinition, nil)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return *roleRes.ID, err
}

func (c *azureConfigClient) CreateRoleAssignment(ctx context.Context, scope string, roleAssignment armauthorization.RoleAssignmentCreateParameters) error {
	newUuid, err := uuid.NewRandom()
	if err != nil {
		return trace.Wrap(err)
	}
	assignID := newUuid.String()
	if _, err = c.roleAssignCli.Create(ctx, scope, assignID, roleAssignment, nil); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *azureConfigClient) GetServicePrincipalByAppID(ctx context.Context, appID string) (*msgraph.ServicePrincipal, error) {
	graphPrincipal, err := c.graphCli.GetServicePrincipalByAppId(ctx, appID)
	if err != nil {
		return nil, trace.BadParameter("failed to get the graph API service principal: %v", err)
	}
	return graphPrincipal, nil
}

func (c *azureConfigClient) GrantAppRoleToServicePrincipal(ctx context.Context, roleAssignment msgraph.AppRoleAssignment) error {
	_, err := c.graphCli.GrantAppRoleToServicePrincipal(ctx, *roleAssignment.PrincipalID, &roleAssignment)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// AccessGraphAzureConfigureRequest is a request to configure the required Policies to use the TAG AWS Sync.
type AccessGraphAzureConfigureRequest struct {
	// ManagedIdentity is the principal performing the discovery
	ManagedIdentity string
	// RoleName is the name of the Azure Role to create and assign to the managed identity
	RoleName string
	// SubscriptionID is the Azure subscription containing resources for sync
	SubscriptionID string
	// AutoConfirm skips user confirmation of the operation plan if true
	AutoConfirm bool
	// stdout is used to override stdout output in tests.
	stdout io.Writer
}

func roleAssignmentAction(clt AccessGraphAzureConfigureClient, subscriptionID string, managedID string, roleName string) (*provisioning.Action, error) {
	customRole := "CustomRole"
	scope := "/subscriptions/" + subscriptionID
	runnerFn := func(ctx context.Context) error {
		// Create the role
		roleDefinition := armauthorization.RoleDefinition{
			Name: &roleName,
			Properties: &armauthorization.RoleDefinitionProperties{
				RoleName: &roleName,
				RoleType: &customRole,
				Permissions: []*armauthorization.Permission{
					{
						Actions: libslices.ToPointers([]string{
							"Microsoft.Compute/virtualMachines/read",
							"Microsoft.Compute/virtualMachineScaleSets/virtualMachines/read",
							"Microsoft.Authorization/roleDefinitions/read",
							"Microsoft.Authorization/roleAssignments/read",
						}),
					},
				},
				AssignableScopes: []*string{&scope}, // Scope must be provided
			},
		}
		roleID, err := clt.CreateRoleDefinition(ctx, scope, roleDefinition)
		if err != nil {
			return trace.Errorf("failed to create custom role: %v", err)
		}

		// Assign the new role to the managed identity
		roleAssignParams := armauthorization.RoleAssignmentCreateParameters{
			Properties: &armauthorization.RoleAssignmentProperties{
				PrincipalID:      &managedID,
				RoleDefinitionID: &roleID,
			},
		}
		if err = clt.CreateRoleAssignment(ctx, scope, roleAssignParams); err != nil {
			return trace.Errorf("failed to assign role %s to principal %s: %v", roleName, managedID, err)
		}

		// Assign the Graph API permissions to the managed identity
		graphPrincipal, err := clt.GetServicePrincipalByAppID(ctx, graphAppID)
		if err != nil {
			return trace.Errorf("could not get the graph API service principal: %v", err)
		}
		rolesNotAssigned := make(map[string]bool)
		for k, v := range requiredGraphRoleNames {
			rolesNotAssigned[k] = v
		}
		for _, appRole := range graphPrincipal.AppRoles {
			if _, ok := requiredGraphRoleNames[*appRole.Value]; ok {
				roleAssignment := msgraph.AppRoleAssignment{
					AppRoleID:   appRole.ID,
					PrincipalID: &managedID,
					ResourceID:  graphPrincipal.ID,
				}
				if err = clt.GrantAppRoleToServicePrincipal(ctx, roleAssignment); err != nil {
					return trace.Errorf("failed to assign graph API role to %s: %v", managedID, err)
				}
				delete(rolesNotAssigned, *appRole.Value)
			}
		}
		if len(rolesNotAssigned) > 0 {
			return trace.Errorf("could not assign all required roles: %v", slices.Collect(maps.Keys(rolesNotAssigned)))
		}
		return nil
	}
	cfg := provisioning.ActionConfig{
		Name:    "AssignRole",
		Summary: "Creates a new Azure role and attaches it to a managed identity for the Discovery service",
		Details: strings.Join([]string{
			"The Discovery service needs to run as a credentialed Azure managed identity. This managed identity ",
			"can be system assigned (i.e. tied to the lifecycle of a virtual machine running the Discovery service), ",
			"or user-assigned (i.e. a persistent identity). The managed identity requires two types of permissions:\n\n",
			"\t1) Azure resource permissions in order to fetch virtual machines, role definitions, etc, and\n",
			"\t2) Graph API permissions to fetch users, groups, and service principals.\n\n",
			"The command assigns both Azure resource permissions as well as Graph API permissions to the specified ",
			"managed identity.",
		}, ""),
		RunnerFn: runnerFn,
	}
	return provisioning.NewAction(cfg)
}

// ConfigureAccessGraphSyncAzure sets up the managed identity and role required for Teleport to be able to pull
// AWS resources into Teleport.
func ConfigureAccessGraphSyncAzure(ctx context.Context, clt AccessGraphAzureConfigureClient, req AccessGraphAzureConfigureRequest) error {
	managedIDAction, err := roleAssignmentAction(clt, req.SubscriptionID, req.ManagedIdentity, req.RoleName)
	if err != nil {
		return trace.Wrap(err)
	}
	opCfg := provisioning.OperationConfig{
		Name: "access-graph-azure-sync",
		Actions: []provisioning.Action{
			*managedIDAction,
		},
		AutoConfirm: req.AutoConfirm,
		Output:      req.stdout,
	}
	return trace.Wrap(provisioning.Run(ctx, opCfg))
}
