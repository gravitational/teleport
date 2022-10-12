package resources

import (
	"context"

	resourcesv3 "github.com/gravitational/teleport/operator/apis/resources/v3"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/operator/sidecar"
	"github.com/gravitational/trace"
)

type OIDCConnectorClient struct {
	TeleportClientAccessor sidecar.ClientAccessor
}

func (r OIDCConnectorClient) Get(ctx context.Context, name string) (types.OIDCConnector, error) {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return teleportClient.GetOIDCConnector(ctx, name, false /* with secrets*/)
}

func (r OIDCConnectorClient) Create(ctx context.Context, oidc types.OIDCConnector) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return teleportClient.UpsertOIDCConnector(ctx, oidc)
}

func (r OIDCConnectorClient) Update(ctx context.Context, oidc types.OIDCConnector) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return teleportClient.UpsertOIDCConnector(ctx, oidc)
}

func (r OIDCConnectorClient) Delete(ctx context.Context, name string) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return teleportClient.DeleteOIDCConnector(ctx, name)
}

func NewOIDCConnectorReconciler(client kclient.Client, accessor sidecar.ClientAccessor) *TeleportResourceReconciler[types.OIDCConnector, *resourcesv3.TeleportOIDCConnector] {
	oidcClient := &OIDCConnectorClient{
		TeleportClientAccessor: accessor,
	}

	resourceReconciler := NewTeleportResourceReconciler[types.OIDCConnector, *resourcesv3.TeleportOIDCConnector](
		client,
		oidcClient,
		&resourcesv3.TeleportOIDCConnector{},
	)

	return resourceReconciler
}
