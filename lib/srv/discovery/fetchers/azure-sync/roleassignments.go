package azure_sync

import (
	"context"
	"fmt"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (a *azureFetcher) fetchRoleAssignments(ctx context.Context) ([]*accessgraphv1alpha.AzureRoleAssignment, error) {
	// Get the Role Assignments client
	cli, err := a.CloudClients.GetAzureRoleAssignmentsClient(a.GetSubscriptionID())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// List the role definitions
	roleAssigns, err := cli.ListRoleAssignments(ctx, fmt.Sprintf("/subscriptions/%s", a.SubscriptionID))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Convert to protobuf format
	pbRoleAssigns := make([]*accessgraphv1alpha.AzureRoleAssignment, 0, len(roleAssigns))
	for _, roleAssign := range roleAssigns {
		pbRoleAssign := &accessgraphv1alpha.AzureRoleAssignment{
			Id:               *roleAssign.ID,
			SubscriptionId:   a.SubscriptionID,
			LastSyncTime:     timestamppb.Now(),
			PrincipalId:      *roleAssign.Properties.PrincipalID,
			RoleDefinitionId: *roleAssign.Properties.RoleDefinitionID,
		}
		pbRoleAssigns = append(pbRoleAssigns, pbRoleAssign)
	}
	return pbRoleAssigns, nil
}
