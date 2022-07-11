package clusters

import (
	"context"

	"github.com/gravitational/teleport/lib/teleterm/gateway"

	"github.com/gravitational/trace"
)

type GatewayCreator struct {
	clusterResolver ClusterResolver
}

func NewGatewayCreator(clusterResolver ClusterResolver) GatewayCreator {
	return GatewayCreator{
		clusterResolver: clusterResolver,
	}
}

func (g GatewayCreator) CreateGateway(ctx context.Context, params CreateGatewayParams) (*gateway.Gateway, error) {
	cluster, err := g.clusterResolver.ResolveCluster(params.TargetURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	gateway, err := cluster.CreateGateway(ctx, params)
	return gateway, trace.Wrap(err)
}

type ClusterResolver interface {
	ResolveCluster(string) (*Cluster, error)
}
