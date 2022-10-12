package resources

import (
	"context"

	"github.com/gravitational/trace"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	resourcesv2 "github.com/gravitational/teleport/operator/apis/resources/v2"
	"github.com/gravitational/teleport/operator/sidecar"
)

type SAMLConnectorClient struct {
	TeleportClientAccessor sidecar.ClientAccessor
}

func (r SAMLConnectorClient) Get(ctx context.Context, name string) (types.SAMLConnector, error) {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return teleportClient.GetSAMLConnector(ctx, name, false /* with secrets*/)
}

func (r SAMLConnectorClient) Create(ctx context.Context, oidc types.SAMLConnector) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return teleportClient.UpsertSAMLConnector(ctx, oidc)
}

func (r SAMLConnectorClient) Update(ctx context.Context, oidc types.SAMLConnector) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return teleportClient.UpsertSAMLConnector(ctx, oidc)
}

func (r SAMLConnectorClient) Delete(ctx context.Context, name string) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return teleportClient.DeleteSAMLConnector(ctx, name)
}

func NewSAMLConnectorReconciler(client kclient.Client, accessor sidecar.ClientAccessor) *TeleportResourceReconciler[types.SAMLConnector, *resourcesv2.TeleportSAMLConnector] {
	samlClient := &SAMLConnectorClient{
		TeleportClientAccessor: accessor,
	}

	resourceReconciler := NewTeleportResourceReconciler[types.SAMLConnector, *resourcesv2.TeleportSAMLConnector](
		client,
		samlClient,
		&resourcesv2.TeleportSAMLConnector{},
	)

	return resourceReconciler
}
