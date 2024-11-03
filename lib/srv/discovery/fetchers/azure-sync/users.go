package azure_sync

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"
	"slices"
)

func (a *azureFetcher) fetchUsers(ctx context.Context) ([]*accessgraphv1alpha.AzureUser, error) {
	// Get the VM client
	cred, err := a.CloudClients.GetAzureCredential()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	scopes := []string{"https://graph.microsoft.com/.default"}
	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: scopes})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cli := NewGraphClient(token)

	// Fetch the users
	users, err := cli.ListUsers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	groups, err := cli.ListGroups(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identities := slices.Concat(users, groups)

	// Return the users as protobuf messages
	pbUsers := make([]*accessgraphv1alpha.AzureUser, 0, len(identities))
	for _, identity := range identities {
		pbUser := &accessgraphv1alpha.AzureUser{
			Id:             identity.ID,
			SubscriptionId: a.GetSubscriptionID(),
			LastSyncTime:   timestamppb.Now(),
			DisplayName:    identity.Name,
		}
		pbUsers = append(pbUsers, pbUser)
	}
	return pbUsers, nil
}
