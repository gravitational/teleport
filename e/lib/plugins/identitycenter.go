package plugins

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/lib/integrations/awsoidc/credprovider"
)

// awsICInstanceFactory creates a new instance of the AWS Identity Center Plugin.
// It implements `instanceFactory`, and so takes config information from the
// plugin manager amd returns a function that can be invoked to run the service.
func awsIdentityCenterInstanceFactory(_ context.Context, p *types.PluginV1, deps instanceDependencies) (func() error, error) {
	settings := p.Spec.GetAwsIc()
	if settings == nil {
		return nil, trace.BadParameter("plugin must have AWS IC settings")
	}

	instanceARN, err := arn.Parse(settings.Arn)
	if err != nil {
		return nil, trace.Wrap(err, "malformed IC Instance ARN")
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

		// set up a context that will automatically cancel itself when this
		// plugin delegate exits. This is to make sure that everything we start
		// below gets stopped on exit, *especially* if that exit is early due to
		// error.
		ctx, cancel := context.WithCancel(deps.lifetime)
		defer cancel()

		authServer := deps.parentProcess.GetAuthServer()
		logger := deps.logger.With(teleport.ComponentKey, identitycenter.Component)

		scimClient, err := scimsdk.New(&scimsdk.Config{
			Endpoint: settings.ProvisioningSpec.BaseUrl,
			Token:    bearerToken,
			Log:      logger,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		awsConfig, err := credprovider.CreateAWSConfigForIntegration(ctx, credprovider.Config{
			Region:                settings.Region,
			IntegrationName:       settings.IntegrationName,
			IntegrationGetter:     authServer.Services,
			AWSOIDCTokenGenerator: identitycenter.MakeTokenGenerator(authServer),
			Logger:                logger,
			Clock:                 authServer.GetClock(),
		})
		if err != nil {
			return trace.Wrap(err)
		}

		identityCenterClient, err := icsdk.New(icsdk.Config{
			InstanceARN: instanceARN.String(),
			AWSConfig:   awsConfig,
			Logger:      logger,
		})
		if err != nil {
			return trace.Wrap(err, "creating Identity Center client")
		}

		svc, err := identitycenter.NewService(identitycenter.ServiceConfig{
			Provisioning: identitycenter.ProvisioningConfig{
				SCIMClient:          scimClient,
				StateSvc:            authServer.Services,
				StateSvcCache:       authServer.Cache,
				UsersSvcCache:       authServer.Cache,
				AccessListsSvcCache: authServer.Cache,
				LocksSvc:            authServer.Services,
			},
			ICClient:              identityCenterClient,
			UsersSvc:              authServer.Services,
			AccessListsSvc:        authServer.Services,
			AccessRequestsSvc:     authServer.Services,
			Clock:                 authServer.GetClock(),
			EventsClient:          deps.client,
			IdentityCenterDataSvc: authServer.Services,
			Log:                   logger,
			RolesSvc:              authServer.Services,
			ImportConfig: identitycenter.ImportConfig{
				AccessListDefaultOwners: settings.AccessListDefaultOwners,
			},
			PluginStatusSink: deps.statusSink,
			PluginsService:   deps.pluginsService,

			// TODO(tcsc): Expose ListAccountAssignments on the cache interface
			//             and replace this with a reference to authServer.Cache
			IdentityCenterDataSvcCache: authServer.Services,
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
