package resources

import (
	"context"

	resourcesv3 "github.com/gravitational/teleport/operator/apis/resources/v3"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/operator/sidecar"
	"github.com/gravitational/trace"
)

type GithubConnectorClient struct {
	TeleportClientAccessor sidecar.ClientAccessor
}

func (r GithubConnectorClient) Get(ctx context.Context, name string) (types.GithubConnector, error) {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return teleportClient.GetGithubConnector(ctx, name, false /* with secrets*/)
}

func (r GithubConnectorClient) Create(ctx context.Context, oidc types.GithubConnector) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return teleportClient.UpsertGithubConnector(ctx, oidc)
}

func (r GithubConnectorClient) Update(ctx context.Context, oidc types.GithubConnector) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return teleportClient.UpsertGithubConnector(ctx, oidc)
}

func (r GithubConnectorClient) Delete(ctx context.Context, name string) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return teleportClient.DeleteGithubConnector(ctx, name)
}

func NewGithubConnectorReconciler(client kclient.Client, accessor sidecar.ClientAccessor) *TeleportResourceReconciler[types.GithubConnector, *resourcesv3.TeleportGithubConnector] {
	oidcClient := &GithubConnectorClient{
		TeleportClientAccessor: accessor,
	}

	resourceReconciler := NewTeleportResourceReconciler[types.GithubConnector, *resourcesv3.TeleportGithubConnector](
		client,
		oidcClient,
		&resourcesv3.TeleportGithubConnector{},
	)

	return resourceReconciler
}
