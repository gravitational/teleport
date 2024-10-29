package azure_sync

import (
	"context"
	"fmt"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (a *azureFetcher) fetchRoleDefinitions(ctx context.Context) ([]*accessgraphv1alpha.AzureRoleDefinition, error) {
	// Get the Role Definitions client
	cli, err := a.CloudClients.GetAzureRoleDefinitionsClient(a.GetSubscriptionID())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	roleDefs, err := cli.ListRoleDefinitions(ctx, fmt.Sprintf("/subscriptions/%s", a.SubscriptionID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pbRoleDefs := make([]*accessgraphv1alpha.AzureRoleDefinition, 0, len(roleDefs))
	for _, roleDef := range roleDefs {
		pbPerms := make([]*accessgraphv1alpha.AzurePermission, len(roleDef.Properties.Permissions))
		for _, perm := range roleDef.Properties.Permissions {
			pbPerm := accessgraphv1alpha.AzurePermission{
				Actions: ptrsToList(perm.Actions),
			}
			pbPerms = append(pbPerms, &pbPerm)
		}
		pbRoleDef := &accessgraphv1alpha.AzureRoleDefinition{
			Id:             *roleDef.ID,
			SubscriptionId: a.SubscriptionID,
			LastSyncTime:   timestamppb.Now(),
			Permissions:    pbPerms,
		}
		pbRoleDefs = append(pbRoleDefs, pbRoleDef)
	}
	return pbRoleDefs, nil
}
