package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

func (rc *ResourceCommand) getKubeServer(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	servers, err := client.GetKubernetesServers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if rc.ref.Name == "" {
		return collections.NewKubeServerCollection(servers), nil
	}
	altNameFn := func(r types.KubeServer) string {
		return r.GetHostname()
	}
	servers = filterByNameOrDiscoveredName(servers, rc.ref.Name, altNameFn)
	if len(servers) == 0 {
		return nil, trace.NotFound("Kubernetes server %q not found", rc.ref.Name)
	}
	return collections.NewKubeServerCollection(servers), nil
}

func (rc *ResourceCommand) getKubeCluster(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	clusters, err := client.GetKubernetesClusters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if rc.ref.Name == "" {
		return collections.NewKubeClusterCollection(clusters), nil
	}
	clusters = filterByNameOrDiscoveredName(clusters, rc.ref.Name)
	if len(clusters) == 0 {
		return nil, trace.NotFound("Kubernetes cluster %q not found", rc.ref.Name)
	}
	return collections.NewKubeClusterCollection(clusters), nil
}

func (rc *ResourceCommand) createKubeCluster(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	cluster, err := services.UnmarshalKubeCluster(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := client.CreateKubernetesCluster(ctx, cluster); err != nil {
		if trace.IsAlreadyExists(err) {
			if !rc.force {
				return trace.AlreadyExists("Kubernetes cluster %q already exists", cluster.GetName())
			}
			if err := client.UpdateKubernetesCluster(ctx, cluster); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("Kubernetes cluster %q has been updated\n", cluster.GetName())
			return nil
		}
		return trace.Wrap(err)
	}
	fmt.Printf("Kubernetes cluster %q has been created\n", cluster.GetName())
	return nil
}
