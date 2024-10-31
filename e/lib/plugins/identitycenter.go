package plugins

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
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
		instanceARN, err := arn.Parse(settings.Arn)
		if err != nil {
			return trace.Wrap(err, "malformed IC Instance ARN")
		}
		// TODO(tcsc): replace with real implementation
		tokenFactory := func(_ context.Context, integration string) (string, error) {
			return "", trace.NotImplemented("Integration token generator not yet implemented")
		}

		svc, err := identitycenter.NewService(identitycenter.ServiceConfig{
			Provisioning: identitycenter.ProvisioningConfig{
				SCIMClient:          scimClient,
				StateSvc:            authServer.Services,
				UsersSvcCache:       authServer.Cache,
				AccessListsSvcCache: authServer.Cache,
				LocksSvc:            authServer.Services,
			},
			AWS: identitycenter.AWSConfig{
				InstanceARN:         instanceARN,
				Region:              settings.Region,
				IntegrationName:     settings.IntegrationName,
				IntegrationsService: authServer.Services,
				TokenFactoryFn:      tokenFactory,
			},
			UsersSvc:              authServer.Services,
			AccessListsSvc:        authServer.Services,
			AccessRequestsSvc:     authServer.Services,
			Clock:                 authServer.GetClock(),
			EventsClient:          deps.client,
			IdentityCenterDataSvc: authServer.Services,
			Log:                   logger,
			RolesSvc:              authServer.Services,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		logger.DebugContext(deps.lifetime, "Running AWS IC service.")
		if err := svc.Run(deps.lifetime); err != nil {
			logger.ErrorContext(deps.lifetime, "Identity Center Service exited with error",
				"error", err)
			return trace.Wrap(err)
		}
		return nil
	}

	return svc, nil
}
