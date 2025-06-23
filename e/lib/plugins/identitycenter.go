package plugins

import (
	"context"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter"
	identitycentercommon "github.com/gravitational/teleport/e/lib/aws/identitycenter/common"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	cloudaws "github.com/gravitational/teleport/e/lib/cloud/aws"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/auth"
	icfilters "github.com/gravitational/teleport/lib/aws/identitycenter/filters"
	"github.com/gravitational/teleport/lib/integrations/awsoidc/credprovider"
)

// awsIdentityCenterInstanceFactory creates a new instance of the AWS Identity Center Plugin.
// It implements [instanceFactory], and so takes config information from the
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
		logger := deps.logger.With(teleport.ComponentKey, eteleport.ComponentAWSIC)

		scimClient, err := scimsdk.New(&scimsdk.Config{
			Endpoint:        settings.ProvisioningSpec.BaseUrl,
			Token:           bearerToken,
			Log:             logger,
			IntegrationType: types.PluginTypeAWSIdentityCenter,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		awsClientConfig, err := makeAWSConfig(ctx, settings, authServer, logger)
		if err != nil {
			return trace.Wrap(err)
		}

		identityCenterClient, err := icsdk.New(icsdk.Config{
			InstanceARN: instanceARN.String(),
			AWSConfig:   awsClientConfig,
			Logger:      logger.With(teleport.ComponentKey, eteleport.ComponentAWSICSDK),
		})
		if err != nil {
			return trace.Wrap(err, "creating Identity Center client")
		}

		accountFilters, err := icfilters.New(settings.AwsAccountsFilters)
		if err != nil {
			return trace.Wrap(err)
		}

		groupsFilters, err := icfilters.New(settings.GroupSyncFilters)
		if err != nil {
			return trace.Wrap(err)
		}

		rolesSyncMode, err := selectRoleSyncMode(settings.RolesSyncMode)
		if err != nil {
			return trace.Wrap(err)
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
			ICClient:                   identityCenterClient,
			UsersSvc:                   authServer.Services,
			AccessListsSvc:             authServer.Services,
			AccessRequestsSvc:          authServer.Services,
			Clock:                      authServer.GetClock(),
			EventsClient:               deps.client,
			IdentityCenterDataSvc:      authServer.Services,
			IdentityCenterDataSvcCache: authServer.Cache,
			Log:                        logger,
			RolesSvc:                   authServer.Services,
			ImportConfig: identitycenter.ImportConfig{
				AccessListDefaultOwners: settings.AccessListDefaultOwners,
				GroupSyncFilter:         groupsFilters,
				AccountFilters:          accountFilters,
			},
			PluginStatusSink: deps.statusSink,
			PluginsService:   deps.pluginsService,
			UserPredicate:    identitycentercommon.UserPredicateFilter(settings.UserSyncFilters),
			Emitter:          deps.parentProcess.GetAuthServer().GetEmitter(),
			RolesSyncMode:    rolesSyncMode,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		logger.DebugContext(deps.lifetime, "Running AWS IC service.")
		if err := svc.Run(deps.lifetime); err != nil {
			if err := deps.statusSink.Emit(ctx, &types.PluginStatusV1{
				Code:         types.PluginStatusCode_OTHER_ERROR,
				ErrorMessage: err.Error(),
			}); err != nil {
				logger.ErrorContext(ctx, "Failed to emit plugin error status", "error", err)
			}
			logger.ErrorContext(deps.lifetime, "Identity Center Service exited with error",
				"error", err)
			return trace.Wrap(err)
		}
		return nil
	}

	return svc, nil
}

// selectRoleSyncMode validates the supplied mode string and translates it into
// the appropriate RolesSyncMode value.
func selectRoleSyncMode(m string) (identitycenter.RolesSyncMode, error) {
	switch m {
	case "", types.AWSICRolesSyncModeAll:
		return identitycenter.RolesSyncModeAll, nil
	case types.AWSICRolesSyncModeNone:
		return identitycenter.RolesSyncModeNone, nil
	}
	return 0, trace.BadParameter("invalid role sync mode %q", m)
}

// makeAWSConfig generates an AWS client configuration for the integration to
// use.
func makeAWSConfig(ctx context.Context, settings *types.PluginAWSICSettings, authServer *auth.Server, logger *slog.Logger) (*aws.Config, error) {
	if err := cloudaws.ValidateAWSRegion(settings.Region); err != nil {
		return nil, trace.Wrap(err)
	}

	switch source := settings.Credentials.GetSource().(type) {
	case *types.AWSICCredentials_Oidc:
		logger.DebugContext(ctx, "Using AWS OIDC integration",
			slog.String("integration", settings.IntegrationName))
		cfg, err := cloudaws.CreateAWSConfigForIntegration(ctx, credprovider.Config{
			Region:                settings.Region,
			IntegrationName:       source.Oidc.IntegrationName,
			IntegrationGetter:     authServer.Services,
			AWSOIDCTokenGenerator: identitycenter.MakeTokenGenerator(authServer),
			Logger:                logger,
			Clock:                 authServer.GetClock(),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return cfg, nil
	case *types.AWSICCredentials_System:
		if source.System.AssumeRoleArn != "" {
			logger.DebugContext(ctx, "Using ambient system AWS credential with configured assume role ARN")
			cfg, err := cloudaws.BuildAWSConfig(ctx, settings.Region, source.System.AssumeRoleArn, nil)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &cfg, nil
		}

		// AssumeRoleARN is the preferred method and required for newer integration.
		// Below only serves to support already active integration that uses system credential
		// without role assumption.
		// TODO(sshah): DELETE in Teleport 19.
		logger.DebugContext(ctx, "Using ambient system AWS credential")
		cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(settings.Region))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &cfg, nil
	default:
		return nil, trace.BadParameter("invalid Credentials source type: %T", source)
	}
}
