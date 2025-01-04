package azureoidc

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization"
	"github.com/google/uuid"
	"github.com/gravitational/teleport/lib/cloud/provisioning"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/msgraph"
	tslices "github.com/gravitational/teleport/lib/utils/slices"
	"github.com/gravitational/trace"
	"os"
	"slices"
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
			return trace.Wrap(fmt.Errorf("failed to create role definitions client: %v", err))
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
							"Microsoft.Compute/virtualMachines/list",
							"Microsoft.Compute/virtualMachineScaleSets/virtualMachines/read",
							"Microsoft.Compute/virtualMachineScaleSets/virtualMachines/list",
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
			return trace.Wrap(fmt.Errorf("failed to create custom role: %v", err))
		}

		// Assign the Azure role to the managed identity
		roleAssignCli, err := armauthorization.NewRoleAssignmentsClient(subId, cred, nil)
		if err != nil {
			return fmt.Errorf("failed to create role assignments client: %v", err)
		}
		assignName := uuid.New().String()
		if err != nil {
			return trace.Wrap(fmt.Errorf("failed to create role assignments client: %v", err))
		}
		roleAssignParams := armauthorization.RoleAssignmentCreateParameters{
			Properties: &armauthorization.RoleAssignmentProperties{
				PrincipalID:      &managedId,
				RoleDefinitionID: roleRes.ID,
			},
		}
		_, err = roleAssignCli.Create(ctx, scope, assignName, roleAssignParams, nil)
		if err != nil {
			return fmt.Errorf("failed to create role assignment: %v", err)
		}

		// Assign the Graph API permissions to the managed identity
		graphCli, err := msgraph.NewClient(msgraph.Config{
			TokenProvider: cred,
		})
		graphPrincipal, err := graphCli.GetServicePrincipalByAppId(ctx, graphAppId)
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
				return trace.Wrap(fmt.Errorf("failed to create role assignment: %v", err))
			}
		}

		return nil
	}
	cfg := provisioning.ActionConfig{
		Name:     "NewSyncManagedId",
		Summary:  "Creates a new Azure role and attaches it to a managed identity for the Discovery service",
		Details:  "Creates a new Azure role and attaches it to a managed identity for the Discovery service",
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
