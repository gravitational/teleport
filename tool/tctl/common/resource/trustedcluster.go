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

func (rc *ResourceCommand) getTrustedCluster(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name == "" {
		trustedClusters, err := client.GetTrustedClusters(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewTrustedClusterCollection(trustedClusters), nil
	}
	trustedCluster, err := client.GetTrustedCluster(ctx, rc.ref.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewTrustedClusterCollection([]types.TrustedCluster{trustedCluster}), nil
}

// createTrustedCluster implements `tctl create cluster.yaml` command
func (rc *ResourceCommand) createTrustedCluster(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	tc, err := services.UnmarshalTrustedCluster(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	// check if such cluster already exists:
	name := tc.GetName()
	_, err = client.GetTrustedCluster(ctx, name)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	exists := (err == nil)
	if !rc.force && exists {
		return trace.AlreadyExists("trusted cluster %q already exists", name)
	}

	//nolint:staticcheck // SA1019. UpsertTrustedCluster is deprecated but will
	// continue being supported for tctl clients.
	// TODO(bernardjkim) consider using UpsertTrustedClusterV2 in VX.0.0
	out, err := client.UpsertTrustedCluster(ctx, tc)
	if err != nil {
		// If force is used and UpsertTrustedCluster returns trace.AlreadyExists,
		// this means the user tried to upsert a cluster whose exact match already
		// exists in the backend, nothing needs to occur other than happy message
		// that the trusted cluster has been created.
		if rc.force && trace.IsAlreadyExists(err) {
			out = tc
		} else {
			return trace.Wrap(err)
		}
	}
	if out.GetName() != tc.GetName() {
		fmt.Printf("WARNING: trusted cluster %q resource has been renamed to match remote cluster name %q\n", name, out.GetName())
	}
	fmt.Printf("trusted cluster %q has been %v\n", out.GetName(), UpsertVerb(exists, rc.force))
	return nil
}

func (rc *ResourceCommand) getRemoteCluster(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name == "" {
		remoteClusters, err := client.GetRemoteClusters(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewRemoteClusterCollection(remoteClusters), nil
	}
	remoteCluster, err := client.GetRemoteCluster(ctx, rc.ref.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewRemoteClusterCollection([]types.RemoteCluster{remoteCluster}), nil
}
