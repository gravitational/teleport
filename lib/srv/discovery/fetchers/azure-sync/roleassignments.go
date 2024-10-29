package azure_sync

import (
	"context"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

func (a *azureFetcher) fetchRoleAssignments(ctx context.Context) ([]*accessgraphv1alpha.AzureRoleDefinition, error) {
	return nil, nil
}
