package azure_sync

import (
	"context"
	"slices"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

const groupType = "#microsoft.graph.group"

func (a *Fetcher) fetchPrincipals(ctx context.Context) ([]*accessgraphv1alpha.AzurePrincipal, error) {
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

	// Fetch the users, groups, and managed identities
	users, err := cli.ListUsers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	groups, err := cli.ListGroups(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	svcPrincipals, err := cli.ListServicePrincipals(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	principals := slices.Concat(users, groups, svcPrincipals)

	// Return the users as protobuf messages
	pbPrincipals := make([]*accessgraphv1alpha.AzurePrincipal, 0, len(principals))
	for _, principal := range principals {
		// Extract group membership
		memberOf := make([]string, 0)
		for _, member := range principal.MemberOf {
			if member.Type == groupType {
				memberOf = append(memberOf, member.ID)
			}
		}
		// Create the protobuf principal and append it to the list
		pbPrincipal := &accessgraphv1alpha.AzurePrincipal{
			Id:             principal.ID,
			SubscriptionId: a.GetSubscriptionID(),
			LastSyncTime:   timestamppb.Now(),
			DisplayName:    principal.Name,
			MemberOf:       memberOf,
		}
		pbPrincipals = append(pbPrincipals, pbPrincipal)
	}
	return pbPrincipals, nil
}
