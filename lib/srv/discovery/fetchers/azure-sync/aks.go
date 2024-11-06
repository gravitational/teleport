package azure_sync

import (
	"context"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (a *azureFetcher) fetchClusters(ctx context.Context) ([]*accessgraphv1alpha.AzureAKSCluster, error) {
	// Get the client
	cli, err := a.CloudClients.GetAzureKubernetesClient(a.GetSubscriptionID())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get the clusters
	clusters, err := cli.ListAll(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pbClusters := make([]*accessgraphv1alpha.AzureAKSCluster, 0, len(clusters))
	for _, cluster := range clusters {
		pbCluster := &accessgraphv1alpha.AzureAKSCluster{
			Id:             cluster.ID,
			SubscriptionId: cluster.SubscriptionID,
			LastSyncTime:   timestamppb.Now(),
			Name:           cluster.Name,
		}
		pbClusters = append(pbClusters, pbCluster)
	}
	return pbClusters, nil
}
