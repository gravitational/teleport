/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/cloud/provisioning"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/msgraph"
	tslices "github.com/gravitational/teleport/lib/utils/slices"
)

// graphAppId is the pre-defined application ID of the Graph API
// Ref: [https://learn.microsoft.com/en-us/troubleshoot/entra/entra-id/governance/verify-first-party-apps-sign-in#application-ids-of-commonly-used-microsoft-applications].
const graphAppId = "00000003-0000-0000-c000-000000000000"

var requiredGraphRoleNames = []string{
	"User.ReadBasic.All",
	"Group.Read.All",
	"Directory.Read.All",
	"User.Read.All",
	"Policy.Read.All",
}

func newManagedIdAction(cred *azidentity.DefaultAzureCredential, subId string, managedId string, roleName string) (*provisioning.Action, error) {
	runnerFn := func(ctx context.Context) error {
		// Create the role
		roleDefCli, err := armauthorization.NewRoleDefinitionsClient(cred, nil)
		if err != nil {
			return trace.Wrap(fmt.Errorf("failed to create role definitions client: %w", err))
		}
		roleDefId := uuid.New().String()
		customRole := "CustomRole"
		scope := fmt.Sprintf("/subscriptions/%s", subId)
		roleDefinition := armauthorization.RoleDefinition{
			Name: &roleDefId,
			Properties: &armauthorization.RoleDefinitionProperties{
				RoleName: &roleName,
				RoleType: &customRole,
				Permissions: []*armauthorization.Permission{
					{
						Actions: tslices.ToPointers([]string{
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
		roleRes, err := roleDefCli.CreateOrUpdate(ctx, scope, roleDefId, roleDefinition, nil)
		if err != nil {
			return trace.Wrap(fmt.Errorf("failed to create custom role: %w", err))
		}

		// Assign the new role to the managed identity
		roleAssignCli, err := armauthorization.NewRoleAssignmentsClient(subId, cred, nil)
		if err != nil {
			return fmt.Errorf("failed to create role assignments client: %w", err)
		}
		assignName := uuid.New().String()
		if err != nil {
			return trace.Wrap(fmt.Errorf("failed to create role assignments client: %w", err))
		}
		roleAssignParams := armauthorization.RoleAssignmentCreateParameters{
			Properties: &armauthorization.RoleAssignmentProperties{
				PrincipalID:      &managedId,
				RoleDefinitionID: roleRes.ID,
			},
		}
		_, err = roleAssignCli.Create(ctx, scope, assignName, roleAssignParams, nil)
		if err != nil {
			return fmt.Errorf(
				"failed to assign role %s to principal %s: %w", roleName, managedId, err)
		}

		// Assign the Graph API permissions to the managed identity
		graphCli, err := msgraph.NewClient(msgraph.Config{
			TokenProvider: cred,
		})
		if err != nil {
			return trace.Wrap(fmt.Errorf("failed to create msgraph client: %w", err))
		}
		graphPrincipal, err := graphCli.GetServicePrincipalByAppId(ctx, graphAppId)
		if err != nil {
			return trace.Wrap(fmt.Errorf("failed to get the graph API service principal: %w", err))
		}
		var graphRoleIds []string
		for _, appRole := range graphPrincipal.AppRoles {
			if slices.Contains(requiredGraphRoleNames, *appRole.Value) {
				graphRoleIds = append(graphRoleIds, *appRole.ID)
			}
		}
		for _, graphRoleId := range graphRoleIds {
			_, err := graphCli.GrantAppRoleToServicePrincipal(ctx, managedId, &msgraph.AppRoleAssignment{
				AppRoleID:   &graphRoleId,
				PrincipalID: &managedId,
				ResourceID:  graphPrincipal.ID,
			})
			if err != nil {
				return trace.Wrap(fmt.Errorf("failed to assign graph API role to %s: %w", managedId, err))
			}
		}
		return nil
	}
	cfg := provisioning.ActionConfig{
		Name:    "NewSyncManagedId",
		Summary: "Creates a new Azure role and attaches it to a managed identity for the Discovery service",
		Details: strings.Join([]string{
			"The Discovery service needs to run as a credentialed Azure managed identity. This managed identity ",
			"can be system assigned (i.e. tied to the lifecycle of a virtual machine running the Discovery service), ",
			"or user-assigned (i.e. a persistent identity). The managed identity requires two types of permissions: ",
			"1) Azure resource permissions in order to fetch virtual machines, role definitions, etc, and 2) Graph ",
			"API permissions to fetch users, groups, and service principals. The command assigns both Azure resource ",
			"permissions as well as Graph API permissions to the specified managed identity.",
		}, ""),
		RunnerFn: runnerFn,
	}
	return provisioning.NewAction(cfg)
}

// ConfigureAccessGraphSyncAzure sets up the managed identity and role required for Teleport to be able to pull
// AWS resources into Teleport.
func ConfigureAccessGraphSyncAzure(ctx context.Context, params config.IntegrationConfAccessGraphAzureSync) error {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return trace.Wrap(err)
	}
	managedIdAction, err := newManagedIdAction(cred, params.SubscriptionID, params.ManagedIdentity, params.RoleName)
	if err != nil {
		return trace.Wrap(err)
	}
	opCfg := provisioning.OperationConfig{
		Name: "access-graph-azure-sync",
		Actions: []provisioning.Action{
			*managedIdAction,
		},
		AutoConfirm: params.AutoConfirm,
		Output:      os.Stdout,
	}
	return trace.Wrap(provisioning.Run(ctx, opCfg))
}
