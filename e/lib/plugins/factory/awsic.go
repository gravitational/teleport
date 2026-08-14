package factory

import (
	"context"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	awssdkconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter"
	identitycentercommon "github.com/gravitational/teleport/e/lib/aws/identitycenter/common"
	icprov "github.com/gravitational/teleport/e/lib/aws/identitycenter/provisioning"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	cloudaws "github.com/gravitational/teleport/e/lib/cloud/aws"
	"github.com/gravitational/teleport/e/lib/provisioning"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/auth"
	icfilters "github.com/gravitational/teleport/lib/aws/identitycenter/filters"
	"github.com/gravitational/teleport/lib/cloud/aws/config"
	"github.com/gravitational/teleport/lib/integrations/awsoidc/credprovider"
)

// AWSIC creates a new instance of the AWS Identity Center Plugin. It implements
// [Factory] for the AWSIC integration, and so takes config information from the
// plugin manager and returns a function that can be invoked to run the service.
func AWSIC(ctx context.Context, p *types.PluginV1, deps Dependencies) (Delegate, error) {
	settings := p.Spec.GetAwsIc()
	if settings == nil {
		return nil, trace.BadParameter("plugin must have AWS IC settings")
	}

	instanceARN, err := arn.Parse(settings.Arn)
	if err != nil {
		return nil, trace.Wrap(err, "malformed IC Instance ARN")
	}

	switch len(deps.StaticCredentials) {
	case 0:
		return nil, trace.BadParameter("plugin dependencies must supply SCIM bearer token as a credential")
	case 1:
		// Exactly what we want, fall out to the happy path
	default:
		// We have more than one credential. Legal, but unexpected. Worth a warning.
		deps.Logger.WarnContext(ctx, "Multiple credentials found. Picking first available.",
			"credential_in_use", deps.StaticCredentials[0].GetName(),
			"count", len(deps.StaticCredentials))
	}

	bearerToken := deps.StaticCredentials[0].GetAPIToken()
	if bearerToken == "" {
		return nil, trace.BadParameter("plugin dependencies must supply SCIM bearer token as an API token")
	}

	svc := func(ctx context.Context) error {
		deps.Logger.InfoContext(ctx, "AWS IC integration starting")
		defer deps.Logger.InfoContext(ctx, "AWS IC integration stopped")

		// set up a context that will automatically cancel itself when this
		// plugin delegate exits. This is to make sure that everything we start
		// below gets stopped on exit, *especially* if that exit is early due to
		// error.
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		authServer := deps.ParentProcess.GetAuthServer()
		logger := deps.Logger.With(teleport.ComponentKey, eteleport.ComponentAWSIC)
		scimConfig := scimsdk.Config{
			Endpoint:        settings.ProvisioningSpec.BaseUrl,
			Token:           bearerToken,
			Log:             logger,
			IntegrationType: types.PluginTypeAWSIdentityCenter,
		}

		scimClient, err := scimsdk.New(&scimConfig)
		if err != nil {
			return trace.Wrap(err)
		}

		scimClientWithBreaker, err := icprov.NewSCIMClientWithBreaker(icprov.SCIMClientWithBreakerConfig{
			SCIMConfig: scimConfig,
			Clock:      authServer.GetClock(),
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

		userProvisioningMode := selectUserProvisioningMode(settings)

		svc, err := identitycenter.NewService(identitycenter.ServiceConfig{
			Provisioning: identitycenter.ProvisioningConfig{
				SCIMClient:            scimClientWithBreaker,
				HealthCheckSCIMClient: scimClient, // Healthcheck shouldn't be tripped by a circuit breaker so we use a separate client without breaker for it.
				StateSvc:              authServer.Services,
				StateSvcCache:         authServer.Cache,
				UsersSvcCache:         authServer.Cache,
				AccessListsSvcCache:   authServer.Cache,
				LocksSvc:              authServer.Services,
				UserProvisioningMode:  userProvisioningMode,
			},
			ICClient:                   identityCenterClient,
			UsersSvc:                   authServer.Services,
			AccessListsSvc:             authServer.Services,
			AccessRequestsSvc:          authServer.Services,
			Clock:                      authServer.GetClock(),
			EventsClient:               deps.Client,
			IdentityCenterDataSvc:      authServer.Services,
			IdentityCenterDataSvcCache: authServer.Cache,
			Log:                        logger,
			RolesSvc:                   authServer.Services,
			ImportConfig: identitycenter.ImportConfig{
				AccessListDefaultOwners: settings.AccessListDefaultOwners,
				GroupSyncFilter:         groupsFilters,
				AccountFilters:          accountFilters,
			},
			PluginStatusSink: deps.StatusSink,
			PluginsService:   deps.PluginsService,
			UserPredicate:    identitycentercommon.UserPredicateFilter(settings.UserSyncFilters),
			Emitter:          deps.ParentProcess.GetAuthServer().GetEmitter(),
			RolesSyncMode:    rolesSyncMode,
			SSORegion:        settings.Region,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		logger.DebugContext(ctx, "Running AWS IC service.")
		if err := svc.Run(ctx); err != nil {
			if err := deps.StatusSink.Emit(ctx, &types.PluginStatusV1{
				Code:         types.PluginStatusCode_OTHER_ERROR,
				ErrorMessage: err.Error(),
			}); err != nil {
				logger.ErrorContext(ctx, "Failed to emit plugin error status", "error", err)
			}
			logger.ErrorContext(ctx, "Identity Center Service exited with error",
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

// selectUserProvisioningMode determined which UserProvisioningMode to use based
// on.
func selectUserProvisioningMode(settings *types.PluginAWSICSettings) provisioning.UserProvisioningMode {
	// The plugin not having a SamlIdpServiceProviderName means that Teleport is
	// not providing SAML login for Identity Center. This in turn implies that
	// some other IdP is doing it. This is a good indicator that the IC integration
	// is running in "hybrid mode" and we should defer to the external IdP for
	// user provisioning.
	if settings.SamlIdpServiceProviderName == "" {
		return provisioning.UserProvisioningModeExternal
	}
	return provisioning.UserProvisioningModeInternal
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
		cfg, err := config.LoadDefaultConfig(ctx, awssdkconfig.WithRegion(settings.Region))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &cfg, nil
	default:
		return nil, trace.BadParameter("invalid Credentials source type: %T", source)
	}
}
