package entraid

import (
	"context"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/msgraph"
	"github.com/gravitational/teleport/lib/msgraph/models"
)

// getGraphEndpoint returns the Graph API endpoint from the given provider, or
// returns the default endpoint in the case provider is nil or has no specified
// endpoint.
func getGraphEndpoint(provider *types.EntraIDGroupsProvider) string {
	graphEndpoint := types.MSGraphDefaultEndpoint
	if provider == nil {
		return graphEndpoint
	}
	if provider.GraphEndpoint != "" {
		graphEndpoint = provider.GraphEndpoint
	}
	return graphEndpoint
}

// getEntraGroupType returns the Entra group type from the given provider, or
// "security-groups" in the case provider is nil or has no specified group type.
func getEntraGroupType(provider *types.EntraIDGroupsProvider) string {
	groupType := types.EntraIDSecurityGroups
	if provider == nil {
		return groupType
	}
	if provider.GroupType != "" {
		groupType = provider.GroupType
	}
	return groupType
}

// getEntraGroups returns the Entra ID groups the provided user is a direct or nested member of.
func getEntraGroups(ctx context.Context, graphClient *msgraph.Client, groupType string, userID string) ([]string, error) {
	var groups []string
	if err := graphClient.IterateUsersTransitiveMemberOf(ctx, userID, groupType, func(group *models.Group) bool {
		if group == nil {
			return false
		}
		if group.ID == nil {
			return false
		}
		groups = append(groups, *group.ID)
		return true
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return groups, nil
}

// newGraphClient creates a MS Graph client.
func newGraphClient(
	tokenProvider azcore.TokenCredential,
	graphEndpoint string,
	httpClient *http.Client,
) (*msgraph.Client, error) {
	return msgraph.NewClient(msgraph.Config{
		TokenProvider: tokenProvider,
		GraphEndpoint: graphEndpoint,
		HTTPClient:    httpClient,
	})
}
