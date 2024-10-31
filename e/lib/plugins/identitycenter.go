package plugins

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/provisioning"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/lib/services"
)

const (
	IdentityCenterDownstreamID = services.DownstreamID("identitycenter")
)

// awsICInstanceFactory creates a new instance of the AWS Identity Center Plugin.
// It implements `instanceFactory`, and so takes config information from the
// plugin manager amd returns a function that can be invoked to run the service.
func awsIdentityCenterInstanceFactory(_ context.Context, p *types.PluginV1, deps instanceDependencies) (func() error, error) {
	settings := p.Spec.GetAwsIc()
	if settings == nil {
		return nil, trace.BadParameter("plugin must have AWS IC settings")
	}

	if len(deps.staticCredentials) == 0 {
		return nil, trace.BadParameter("plugin dependencies must supply SCIM bearer token as a credential")
	}
	bearerToken := deps.staticCredentials[0].GetAPIToken()
	if bearerToken == "" {
		return nil, trace.BadParameter("plugin dependencies must supply SCIM bearer token as an API token")
	}

	svc := func() error {
		deps.logger.InfoContext(deps.lifetime, "AWS IC integration starting")
		defer deps.logger.InfoContext(deps.lifetime, "AWS IC integration stopped")

		authServer := deps.parentProcess.GetAuthServer()
		logger := deps.logger.With(teleport.ComponentKey, "PROV")

		scimClient, err := scimsdk.New(&scimsdk.Config{
			Endpoint: settings.ProvisioningSpec.BaseUrl,
			Token:    bearerToken,
			Log:      logger,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		svc, err := provisioning.NewService(provisioning.ServiceConfig{
			SCIMClient:       scimClient,
			DownstreamID:     IdentityCenterDownstreamID,
			UsersCache:       authServer.Cache,
			AccessListsCache: authServer.Cache,
			Locks:            authServer.Services,
			StateSvc:         authServer.Services,
			StateSvcCache:    authServer.Cache,
			EventsClient:     deps.client,
			Logger:           logger,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		logger.DebugContext(deps.lifetime, "Running Identity Center service...")
		if err := svc.Run(deps.lifetime); err != nil {
			logger.ErrorContext(deps.lifetime, "Identity Center Service exited with error",
				"error", err)
			return trace.Wrap(err)
		}
		return nil
	}

	return svc, nil
}
