package service

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/utils"
)

func reconcileOAuthProxyApps(ctx context.Context, clt *authclient.Client, hostUUID, publicAddr string) error {
	for integration, err := range clientutils.Resources(ctx, clt.ListIntegrations) {
		if err != nil {
			continue
		}

		if integration.GetSubKind() != types.IntegrationSubKindOAuthProxy {
			continue
		}

		appServer, err := types.NewAppServerForOAuthProxyIntegration(
			integration.GetName(),
			hostUUID,
			utils.DefaultAppPublicAddr(integration.GetName(), publicAddr),
			integration.GetOAuthProxyIntegrationSpec().UpstreamUrl,
			integration.GetAllLabels(),
		)
		if err != nil {
			return trace.Wrap(err)
		}

		if _, err := clt.UpsertApplicationServer(ctx, appServer); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}
