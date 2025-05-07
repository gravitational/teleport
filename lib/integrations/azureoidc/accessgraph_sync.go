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
	"errors"
	"io"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	armpolicy "github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
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

// azureUserAgent defines the user agent for the Azure SDK to better identify misbehaving clients
const azureUserAgent = "teleport"

// requiredGraphRoleNames is the list of Graph API roles required for the managed identity to fetch resources from Azure
var requiredGraphRoleNames = map[string]struct{}{
	"Group.Read.All":     {},
	"Directory.Read.All": {},
	"User.Read.All":      {},
	"Policy.Read.All":    {},
}

// requiredGraphAppRoleNames is the list of Graph API roles required for the enterprise application to fetch resources from Azure
var requiredGraphAppRoleNames = map[string]struct{}{
	"Group.Read.All": {},
}

// AccessGraphAzureConfigureClient provides an interface for granting the managed identity the necessary permissions
// to fetch Azure resources
type AccessGraphAzureConfigureClient interface {
	// CreateRoleDefinition creates an Azure role definition
	CreateRoleDefinition(ctx context.Context, scope string, roleDefinition armauthorization.RoleDefinition) (string, error)
	// CreateRoleAssignment assigns a role to an Azure principal
	CreateRoleAssignment(ctx context.Context, scope string, roleAssignment armauthorization.RoleAssignmentCreateParameters) error
	// GetServicePrincipalByAppID returns a service principal based on its application ID
	GetServicePrincipalByAppID(ctx context.Context, appID string) (*msgraph.ServicePrincipal, error)
	// GrantAppRoleToServicePrincipal grants a specific type of application role to a service principal
	GrantAppRoleToServicePrincipal(ctx context.Context, roleAssignment msgraph.AppRoleAssignment) error
}

// azureConfigClient wraps the role definition, role assignments, and Graph API clients
type azureConfigClient struct {
	roleDefCli    *armauthorization.RoleDefinitionsClient
	roleAssignCli *armauthorization.RoleAssignmentsClient
	graphCli      *msgraph.Client
}

// NewAzureConfigClient returns a new config client for granting the principal the necessary permissions
// to fetch Azure resources
func NewAzureConfigClient(subscriptionID string) (AccessGraphAzureConfigureClient, error) {
	telemetryOpts := policy.TelemetryOptions{
		ApplicationID: azureUserAgent,
	}
	opts := &armpolicy.ClientOptions{
		ClientOptions: policy.ClientOptions{
			Telemetry: telemetryOpts,
		},
	}
	cred, err := azidentity.NewDefaultAzureCredential(&azidentity.DefaultAzureCredentialOptions{
		ClientOptions: azcore.ClientOptions{
			Telemetry: telemetryOpts,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleDefCli, err := armauthorization.NewRoleDefinitionsClient(cred, opts)
	if err != nil {
		return nil, trace.BadParameter("failed to create role definitions client: %v", err)
	}
	roleAssignCli, err := armauthorization.NewRoleAssignmentsClient(subscriptionID, cred, opts)
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

// CreateRoleDefinition creates an Azure role definition
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

// CreateRoleAssignment assigns a role to an Azure principal
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

// GetServicePrincipalByAppID returns a service principal based on its application ID
func (c *azureConfigClient) GetServicePrincipalByAppID(ctx context.Context, appID string) (*msgraph.ServicePrincipal, error) {
	graphPrincipal, err := c.graphCli.GetServicePrincipalByAppId(ctx, appID)
	if err != nil {
		return nil, trace.BadParameter("failed to get the graph API service principal: %v", err)
	}
	return graphPrincipal, nil
}

// GrantAppRoleToServicePrincipal grants a specific type of application role to a service principal
func (c *azureConfigClient) GrantAppRoleToServicePrincipal(ctx context.Context, roleAssignment msgraph.AppRoleAssignment) error {
	_, err := c.graphCli.GrantAppRoleToServicePrincipal(ctx, *roleAssignment.PrincipalID, &roleAssignment)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// AccessGraphAzureConfigureRequest is a request to assign the right permissions/roles to the Azure principal used
// for fetching resources from Azure.
type AccessGraphAzureConfigureRequest struct {
	// CreateEnterpriseApp indicates whether to create a new enterprise application for this integration
	CreateEnterpriseApp bool
	// ProxyPublicAddr is the public address of the Teleport Proxy, used when creating a new enterprise application
	ProxyPublicAddr string
	// AuthConnectorName is the name of the auth connector when creating a new enterprise application
	AuthConnectorName string
	// PrincipalID is the principal performing the discovery
	PrincipalID string
	// RoleName is the name of the Azure Role to create and assign to the managed identity
	RoleName string
	// SubscriptionID is the Azure subscription containing resources for sync
	SubscriptionID string
	// AutoConfirm skips user confirmation of the operation plan if true
	AutoConfirm bool
	// stdout is used to override stdout output in tests.
	stdout io.Writer
}

// roleAssignmentAction assigns both the Azure role and Graph API roles to the managed identity
func configureAzureSyncAction(clt AccessGraphAzureConfigureClient, subscriptionID string, createApp bool, proxyPublicAddr string, authConnectorName string, principalID string, roleName string) (*provisioning.Action, error) {
	customRole := "CustomRole"
	scope := "/subscriptions/" + subscriptionID
	runnerFn := func(ctx context.Context) error {
		var requiredRoles map[string]struct{}
		if createApp {
			appID, _, err := SetupEnterpriseApp(
				ctx, "Teleport Access Graph Sync", proxyPublicAddr, authConnectorName, false)
			if err != nil {
				return trace.Wrap(err)
			}
			principal, err := clt.GetServicePrincipalByAppID(ctx, appID)
			if err != nil {
				return trace.Wrap(err)
			}
			principalID = *principal.ID
			requiredRoles = requiredGraphAppRoleNames
		} else {
			requiredRoles = requiredGraphRoleNames
		}
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
			var respErr *azcore.ResponseError
			if errors.As(err, &respErr) {
				if respErr.StatusCode == http.StatusConflict {
					return trace.Errorf("role name already exists, delete this role or choose another name: %s", *roleDefinition.Name)
				}
				return trace.Errorf("failed to create custom role: %v", respErr)
			}
			return trace.Errorf("failed to create custom role: %v", err)
		}

		// Assign the new role to the managed identity
		roleAssignParams := armauthorization.RoleAssignmentCreateParameters{
			Properties: &armauthorization.RoleAssignmentProperties{
				PrincipalID:      &principalID,
				RoleDefinitionID: &roleID,
			},
		}
		if err = clt.CreateRoleAssignment(ctx, scope, roleAssignParams); err != nil {
			return trace.Errorf("failed to assign role %s to principal %s: %v", roleName, principalID, err)
		}

		// Assign the Graph API permissions to the managed identity
		graphPrincipal, err := clt.GetServicePrincipalByAppID(ctx, graphAppID)
		if err != nil {
			return trace.Errorf("could not get the graph API service principal: %v", err)
		}
		rolesNotAssigned := make(map[string]struct{})
		for k, v := range requiredRoles {
			rolesNotAssigned[k] = v
		}
		for _, appRole := range graphPrincipal.AppRoles {
			if _, ok := requiredRoles[*appRole.Value]; ok {
				roleAssignment := msgraph.AppRoleAssignment{
					AppRoleID:   appRole.ID,
					PrincipalID: &principalID,
					ResourceID:  graphPrincipal.ID,
				}
				if err = clt.GrantAppRoleToServicePrincipal(ctx, roleAssignment); err != nil {
					return trace.Errorf("failed to assign graph API role to %s: %v", principalID, err)
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
			"The Discovery Service needs to run as a credentialed Azure principal. This principal can be a managed",
			"identity, either system or user assigned. The principal can also be an enterprise application configured",
			"for SSO.\n\n",
			"The principal requires two types of permissions:\n\n",
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
// Azure resources into Teleport.
func ConfigureAccessGraphSyncAzure(
	ctx context.Context,
	clt AccessGraphAzureConfigureClient,
	req AccessGraphAzureConfigureRequest,
) error {
	azureSyncAction, err := configureAzureSyncAction(
		clt, req.SubscriptionID, req.CreateEnterpriseApp, req.ProxyPublicAddr, req.AuthConnectorName, req.PrincipalID,
		req.RoleName)
	if err != nil {
		return trace.Wrap(err)
	}
	opCfg := provisioning.OperationConfig{
		Name:        "access-graph-azure-sync",
		Actions:     []provisioning.Action{*azureSyncAction},
		AutoConfirm: req.AutoConfirm,
		Output:      req.stdout,
	}
	return trace.Wrap(provisioning.Run(ctx, opCfg))
}
