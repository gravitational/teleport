package azureoidc

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"
	"github.com/google/uuid"
	"github.com/gravitational/teleport/lib/cloud/provisioning"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/msgraph"
	"github.com/gravitational/teleport/lib/utils/slices"
	"github.com/gravitational/trace"
	"log/slog"
	"os"
)

func newManagedIdAction(cred *azidentity.DefaultAzureCredential, subId string, managedId string, roleName string) (*provisioning.Action, error) {
	runnerFn := func(ctx context.Context) error {
		// Create the managed identity
		userIdCli, err := armmsi.NewUserAssignedIdentitiesClient(subId, cred, nil)
		if err != nil {
			return trace.Wrap(fmt.Errorf("could not create managed identity client: %v", err))
		}
		id := armmsi.Identity{}
		userIdCli.Get(ctx)
		mgdIdRes, err := userIdCli.CreateOrUpdate(ctx, "", name, id, nil)
		if err != nil {
			return trace.Wrap(fmt.Errorf("could not create managed identity: %v", err))
		}
		slog.InfoContext(ctx, fmt.Sprintf(
			"Managed identity created, Name: %s, ID: %s", *mgdIdRes.Name, *mgdIdRes.ID))

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
						Actions: slices.ToPointers([]string{
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
		graphCli.GetServicePrincipalByAppId()

		return nil
	}
	cfg := provisioning.ActionConfig{
		Name:     "NewSyncManagedId",
		Summary:  "Creates a new Azure managed ID for the discovery service to use",
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
	managedIdAction, err := newManagedIdAction(cred, params.SubscriptionID, params.ManagedIdentity)
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
