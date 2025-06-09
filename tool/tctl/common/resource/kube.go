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

var kubeServer = resource{
	getHandler:    getKubeServer,
	deleteHandler: deleteKubeServer,
}

func getKubeServer(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	servers, err := client.GetKubernetesServers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if ref.Name == "" {
		return collections.NewKubeServerCollection(servers), nil
	}
	altNameFn := func(r types.KubeServer) string {
		return r.GetHostname()
	}
	servers = filterByNameOrDiscoveredName(servers, ref.Name, altNameFn)
	if len(servers) == 0 {
		return nil, trace.NotFound("Kubernetes server %q not found", ref.Name)
	}
	return collections.NewKubeServerCollection(servers), nil
}

func deleteKubeServer(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	servers, err := client.GetKubernetesServers(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	resDesc := "Kubernetes server"
	servers = filterByNameOrDiscoveredName(servers, ref.Name)
	name, err := getOneResourceNameToDelete(servers, ref, resDesc)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, s := range servers {
		err := client.DeleteKubernetesServer(ctx, s.GetHostID(), name)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	fmt.Printf("%s %q has been deleted\n", resDesc, name)
	return nil
}

var kubeCluster = resource{
	getHandler:    getKubeCluster,
	createHandler: createKubeCluster,
	deleteHandler: deleteKubeCluster,
}

func getKubeCluster(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	clusters, err := client.GetKubernetesClusters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if ref.Name == "" {
		return collections.NewKubeClusterCollection(clusters), nil
	}
	clusters = filterByNameOrDiscoveredName(clusters, ref.Name)
	if len(clusters) == 0 {
		return nil, trace.NotFound("Kubernetes cluster %q not found", ref.Name)
	}
	return collections.NewKubeClusterCollection(clusters), nil
}

func createKubeCluster(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	cluster, err := services.UnmarshalKubeCluster(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := client.CreateKubernetesCluster(ctx, cluster); err != nil {
		if trace.IsAlreadyExists(err) {
			if !opts.force {
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

func deleteKubeCluster(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	clusters, err := client.GetKubernetesClusters(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	resDesc := "Kubernetes cluster"
	clusters = filterByNameOrDiscoveredName(clusters, ref.Name)
	name, err := getOneResourceNameToDelete(clusters, ref, resDesc)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := client.DeleteKubernetesCluster(ctx, name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("%s %q has been deleted\n", resDesc, name)
	return nil
}
