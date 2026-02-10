package auth

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"

	liblicense "github.com/gravitational/license"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport"
	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	externalauditstoragev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/externalauditstorage/v1"
	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	resourceusagepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/resourceusage/v1"
	samlidppb "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	secreportsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/secreports/v1"
	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/e/lib/accessgraph"
	"github.com/gravitational/teleport/e/lib/accesslist"
	"github.com/gravitational/teleport/e/lib/auth/machineid/workloadidentityv1"
	"github.com/gravitational/teleport/e/lib/auth/summarizer"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/summarizerv1"
	"github.com/gravitational/teleport/e/lib/devicetrust/devicetrustv1"
	dtstorage "github.com/gravitational/teleport/e/lib/devicetrust/storage"
	"github.com/gravitational/teleport/e/lib/externalauditstorage/externalauditstoragev1"
	"github.com/gravitational/teleport/e/lib/idp/saml/samlidpv1"
	"github.com/gravitational/teleport/e/lib/loginrule"
	"github.com/gravitational/teleport/e/lib/loginrule/loginrulev1"
	lrstorage "github.com/gravitational/teleport/e/lib/loginrule/storage"
	"github.com/gravitational/teleport/e/lib/okta"
	"github.com/gravitational/teleport/e/lib/okta/common/connected"
	oktaservice "github.com/gravitational/teleport/e/lib/okta/service"
	"github.com/gravitational/teleport/e/lib/plugins"
	"github.com/gravitational/teleport/e/lib/plugins/pluginsv1"
	"github.com/gravitational/teleport/e/lib/resourceusage/resourceusagev1"
	scimservice "github.com/gravitational/teleport/e/lib/scim/service"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
	"github.com/gravitational/teleport/e/lib/secreports"
	"github.com/gravitational/teleport/e/lib/secreports/limiter"
	"github.com/gravitational/teleport/e/lib/secreports/query/athena"
	"github.com/gravitational/teleport/e/lib/sigstore"
	"github.com/gravitational/teleport/entitlements"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/secreports/secreportsv1"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/teleport/lib/integrations/externalauditstorage"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/release"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	pluginName                = "auth.enterprise"
	envVarNameDisabledPlugins = "TELEPORT_UNSTABLE_DISABLE_PLUGINS"
	envVarNameBedrockRegion   = "TELEPORT_BEDROCK_REGION"
	envVarNameBedrockModel    = "TELEPORT_BEDROCK_MODEL"
)

var logger = logutils.NewPackageLogger(teleport.ComponentKey, pluginName)

type getCertFunc = func() (*tls.Certificate, error)

// License is an interface for checking if a license is disabled.
type License interface {
	GetKeyPair() *liblicense.License
	IsDisabled() bool
}

// ErrLicenseExpired is the error returned when a feature is disabled due to license expiry.
var ErrLicenseExpired = trace.AccessDenied("Teleport Enterprise license expired")

// Config is a configuration of the web plugin
type Config struct {
	Logger *slog.Logger

	// License holds the license under which the Teleport instance is running.
	License License

	HostedPlugins servicecfg.HostedPluginsConfig

	// AccessMonitoring holds the configuration for the Access Monitoring feature.
	AccessMonitoring *servicecfg.AccessMonitoringOptions

	// AccessGraph holds the configuration for the Access Graph feature.
	AccessGraph servicecfg.AccessGraphConfig

	// HTTPTransport is the transport to use for HTTP requests.
	// Useful during testing to inject custom transport to plugin
	// and test the third-party integrations.
	HTTPTransport http.RoundTripper
}

// NewPlugin creates an instance of the Enterprise Web Plugin
func NewPlugin(cfg Config) (*Plugin, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Plugin{
		Config: cfg,
		logger: logger,
	}, nil
}

// Plugin extends OSS auth server API with enterprise features
type Plugin struct {
	Config

	// logger to be used by plugin and subsystems initialized by it.
	// Guaranteed to be non-nil after NewPlugin.
	logger *slog.Logger

	// authServer is the authServer passed into RegisterAuthServices on
	// startup.
	authServer *auth.GRPCServer
	// plugins is the plugins backend service.
	plugins services.Plugins

	// pluginCreds is the backend Plugin Static Credentials service
	pluginCreds services.PluginStaticCredentials

	// mtx protects the cloudClient field.
	mtx sync.Mutex
	// cloudClient is a client of the Cloud API server
	cloudClient cloudapi.TenantsServiceClient
	// oktaConnected is a utility that detects whether Okta is connected.
	oktaConnected *connected.OktaConnected
}

// GetName returns plugin name
func (p *Plugin) GetName() string {
	return pluginName
}

// EnableCloud enables cloud features
func (p *Plugin) EnableCloud(client cloudapi.TenantsServiceClient) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	p.cloudClient = client
}

// GetCloudClient returns cloud client
func (p *Plugin) GetCloudClient() cloudapi.TenantsServiceClient {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	return p.cloudClient
}

// newExternalAuditStorageConfigurator creates an external audit storage configurator from the backend.
// Returns a non-nil configurator that may or may not be in use (check IsUsed()).
func (p *Plugin) newExternalAuditStorageConfigurator(ctx context.Context) (*externalauditstorage.Configurator, error) {
	if !modules.GetModules().Features().Cloud {
		return nil, nil
	}
	easSvc := local.NewExternalAuditStorageService(p.authServer.GetBackend())
	integrationSvc, err := local.NewIntegrationsService(p.authServer.GetBackend())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	statusService := local.NewStatusService(p.authServer.GetBackend())
	config, err := externalauditstorage.NewConfigurator(ctx, easSvc, integrationSvc, statusService)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if config.IsUsed() {
		config.SetGenerateOIDCTokenFn(p.authServer.AuthServer.GenerateExternalAuditStorageOIDCToken)
	}
	return config, nil
}

// RegisterProxyWebHandlers registers to proxy web handler
func (p *Plugin) RegisterProxyWebHandlers(handler any) error {
	return nil
}

// PluginsService returns the plugins (i.e. integrations) service
func (p *Plugin) PluginsService() services.Plugins {
	return p.plugins
}

// PluginStaticCredentialsService returns the plugins (i.e. integrations) service
func (p *Plugin) PluginStaticCredentialsService() services.PluginStaticCredentials {
	return p.pluginCreds
}

// RegisterAuthServices registers Auth Services (GRPC)
func (p *Plugin) RegisterAuthServices(ctx context.Context, server any, getClientCert getCertFunc) error {
	var ok bool
	p.authServer, ok = server.(*auth.GRPCServer)
	if !ok {
		return trace.BadParameter("unsupported auth server type %T", server)
	}

	gRPCServer, err := p.authServer.GetServer()
	if err != nil {
		return trace.BadParameter("missing proto server")
	}

	keypair := p.Config.License.GetKeyPair()
	if keypair != nil {
		p.authServer.AuthServer.SetLicense(keypair)
	}

	// Register Cloud APIs.
	cloudapi.RegisterTenantsServiceServer(gRPCServer, &cloudWithRoles{
		plugin: p,
	})

	deviceService, err := registerDeviceTrustService(p.logger, gRPCServer, p.authServer)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := p.registerLoginRuleService(p.authServer); err != nil {
		return trace.Wrap(err)
	}

	if modules.GetModules().Features().Cloud && modules.GetModules().Features().GetEntitlement(entitlements.ExternalAuditStorage).Enabled {
		if err := p.registerExternalAuditStorageService(ctx); err != nil {
			return trace.Wrap(err)
		}
	}

	// Create a SAMLService and register it with the auth.Server
	sas, err := NewSAMLAuthService(&SAMLAuthServiceConfig{
		Auth:    p.authServer.AuthServer,
		Emitter: p.authServer.Emitter,
		License: p.Config.License,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	p.authServer.AuthServer.SetSAMLService(sas)

	// Create a OIDCService and register it with the auth.Server
	oas, err := NewOIDCAuthService(&OIDCAuthServiceConfig{
		Auth:    p.authServer.AuthServer,
		Emitter: p.authServer.Emitter,
		License: p.Config.License,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	p.authServer.AuthServer.SetOIDCService(oas)

	// Create the ReleaseClient
	if keypair != nil {
		releaseTLSConfig, err := liblicense.MakeTLSConfig(*keypair)
		if err != nil {
			return trace.Wrap(err)
		}

		releaseClient, err := release.NewClient(
			release.ClientConfig{
				TLSConfig:         releaseTLSConfig,
				ReleaseServerAddr: release.GetServerAddr(),
			},
		)
		if err != nil {
			return trace.Wrap(err)
		}
		p.authServer.AuthServer.SetReleaseService(*releaseClient)
	}

	p.plugins = local.NewPluginsService(p.authServer.GetBackend())
	p.pluginCreds, err = local.NewPluginStaticCredentialsService(p.authServer.GetBackend())
	if err != nil {
		return trace.Wrap(err)
	}

	pluginService, err := p.registerPluginsService()
	if err != nil {
		return trace.Wrap(err)
	}

	samlIdPService, err := samlidpv1.NewSAMLIdPService(samlidpv1.SAMLIdPServiceConfig{
		Client:           p.authServer.AuthServer,
		KeyStore:         p.authServer.AuthServer.GetKeyStore(),
		Authorizer:       p.authServer.Authorizer,
		MFAAuthenticator: p.authServer.AuthServer,
		Logger:           logger,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	samlidppb.RegisterSAMLIdPServiceServer(gRPCServer, samlIdPService)

	// register the start hour getter so that auth can use it during periodic MaintenanceWindow
	// resource sync.
	p.authServer.AuthServer.SetUpgradeWindowStartHourGetter(p.getAccountUpgradeWindowStartHour)

	p.oktaConnected, err = connected.New(connected.Config{
		Logger:          logger,
		ConnectedGetter: p.authServer.AuthServer,
		Plugins:         p.plugins,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	clusterName, err := p.authServer.AuthServer.GetClusterName(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Register the Okta user assignment creator login hook.
	uac, err := okta.NewUserAssignmentCreator(okta.UserAssignmentCreatorConfig{
		Logger:               logger,
		ClusterName:          clusterName.GetClusterName(),
		AccessPoint:          p.authServer.AuthServer,
		OktaConnected:        p.oktaConnected,
		UnifiedResourceCache: p.authServer.AuthServer.UnifiedResourceCache,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	accessListSvc, err := accesslist.NewService(
		ctx,
		accesslist.ServiceConfig{
			Logger:            logger,
			Authorizer:        p.authServer.Authorizer,
			AccessLists:       p.authServer.AuthServer,
			LockGetter:        p.authServer.AuthServer,
			AccessListReviews: p.authServer.AuthServer,
			Plugins:           p.plugins,
			Emitter:           p.authServer.Emitter,
			UsageEvents:       p.authServer.AuthServer,
			UsageReporter:     p.authServer.AuthServer,
			Clock:             p.authServer.AuthServer.GetClock(),
			Cache:             p.authServer.AuthServer.Cache,
			AuthServer:        p.authServer.AuthServer,
			Backend:           p.authServer.GetBackend(),
		})
	if err != nil {
		return trace.Wrap(err)
	}
	accesslistv1.RegisterAccessListServiceServer(gRPCServer, accessListSvc)

	go accessListSvc.ReportCompliance(ctx)

	if err := p.registerAccessGraphService(ctx, p.authServer.AuthServer, gRPCServer, getClientCert); err != nil {
		return trace.Wrap(err)
	}

	if err := p.initAndRegisterSecurityReport(ctx, gRPCServer); err != nil {
		return trace.Wrap(err)
	}

	p.authServer.AuthServer.RegisterLoginHook(uac.OnLogin)

	if modules.GetModules().Features().GetEntitlement(entitlements.Policy).Enabled {
		p.logger.InfoContext(ctx, "Session summarizer enabled")

		oidcClient := AWSOIDCClient{
			IntegrationGetter: p.authServer.AuthServer.Cache,
			cache:             p.authServer.AuthServer.Cache,
			keyStoreManager:   p.authServer.AuthServer.GetKeyStore(),
			serverID:          p.authServer.AuthServer.ServerID,
			clock:             p.authServer.AuthServer.GetClock(),
		}
		cfgCache, err := awsconfig.NewCache(
			awsconfig.WithDefaults(
				awsconfig.WithOIDCIntegrationClient(&oidcClient),
			),
		)
		if err != nil {
			return trace.Wrap(err)
		}

		sessionSummarizer, err := summarizer.NewSessionSummarizer(summarizer.SummarizerConfig{
			Backend:                          p.authServer.AuthServer,
			Streamer:                         p.authServer.AuthServer,
			SummaryUploader:                  p.authServer.AuthServer,
			Clock:                            p.authServer.AuthServer.GetClock(),
			EnableBedrockWithoutRestrictions: !modules.GetModules().Features().Cloud,
			Encrypter:                        p.authServer.AuthServer.EncryptedIO,
			AWSConfigCache:                   cfgCache,
			EnvBedrockRegion:                 os.Getenv(envVarNameBedrockRegion),
			EnvBedrockModelID:                os.Getenv(envVarNameBedrockModel),
			UsageReporter:                    p.authServer.AuthServer.UsageReporter,
			Emitter:                          p.authServer.AuthServer.GetEmitter(),
		})
		if err != nil {
			return trace.Wrap(err)
		}
		p.authServer.AuthServer.SetSummarizerService(sessionSummarizer)

		summarizerService, err := summarizerv1.NewService(summarizerv1.ServiceConfig{
			Authorizer:                       p.authServer.Authorizer,
			Backend:                          p.authServer.AuthServer,
			SummaryDownloader:                p.authServer.AuthServer,
			Decrypter:                        p.authServer.AuthServer.EncryptedIO,
			Emitter:                          p.authServer.Emitter,
			AWSConfigCache:                   cfgCache,
			EnableBedrockWithoutRestrictions: !modules.GetModules().Features().Cloud,
			UsageReporter:                    p.authServer.AuthServer.UsageReporter,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		summarizerv1pb.RegisterSummarizerServiceServer(gRPCServer, summarizerService)
	}

	if err := p.registerResourceUsageService(p.authServer, resourceusagev1.ServiceConfig{
		GetDevicesUsageFunc: deviceService.GetResourceDevicesUsage,
	}); err != nil {
		return trace.Wrap(err)
	}

	err = p.registerSCIMService(ctx, gRPCServer, &common.Config{
		IdentityService:     p.authServer.AuthServer,
		Authorizer:          p.authServer.Authorizer,
		UsersService:        p.authServer.AuthServer,
		RolesService:        p.authServer.AuthServer,
		PluginsService:      p.plugins,
		CredentialsService:  p.pluginCreds,
		LocksService:        p.authServer.AuthServer.Services,
		AccessListsService:  p.authServer.AuthServer.AccessListsInternal,
		AccessListGetter:    p.authServer.AuthServer.AccessListsInternal,
		CertAuthorityGetter: p.authServer.AuthServer,
		JWTSignerGetter:     p.authServer.AuthServer.GetKeyStore(),
		Logger:              logger,
		AssignmentService:   p.authServer.AuthServer.Services.Okta,
		HTTPClient: &http.Client{
			Transport: p.HTTPTransport,
		},
		ClusterName: clusterName.GetClusterName(),
		Clock:       p.authServer.AuthServer.GetClock(),
		Semaphore:   p.authServer.AuthServer,
	})
	if err != nil {
		return trace.Wrap(err, "registering SCIM service")
	}

	oktaSvc, err := oktaservice.NewService(oktaservice.ServiceConfig{
		Backend:       p.authServer.GetBackend(),
		Authorizer:    p.authServer.Authorizer,
		JWTSigner:     p.authServer.AuthServer.GetKeyStore(),
		RoundTripper:  p.Config.HTTPTransport,
		AuthCache:     p.authServer.AuthServer.Cache,
		AuthService:   p.authServer.AuthServer,
		PluginService: pluginService,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	oktapb.RegisterOktaServiceServer(gRPCServer, oktaSvc)

	{
		srv, err := workloadidentityv1.NewX509OverridesService(workloadidentityv1.X509OverridesServiceConfig{
			Authorizer: p.authServer.Authorizer,
			Storage:    p.authServer.AuthServer.Services,
			Emitter:    p.authServer.Emitter,
			// deliberately bypass the cache when getting CAs
			CAGetter: p.authServer.AuthServer.Services,
			KeyStore: p.authServer.AuthServer.GetKeyStore(),

			ClusterName: clusterName.GetClusterName(),
		})
		if err != nil {
			return trace.Wrap(err, "creating workload identity X509 overrides service")
		}
		workloadidentityv1pb.RegisterX509OverridesServiceServer(gRPCServer, srv)
	}
	{
		srv, err := sigstore.NewPolicyResourceService(sigstore.PolicyResourceServiceConfig{
			Backend:    p.authServer.AuthServer,
			Authorizer: p.authServer.Authorizer,
			Emitter:    p.authServer.Emitter,
			Logger:     logger,
		})
		if err != nil {
			return trace.Wrap(err, "creating Sigstore policy service")
		}
		workloadidentityv1pb.RegisterSigstorePolicyResourceServiceServer(gRPCServer, srv)

		eval, err := sigstore.NewPolicyEvaluator(sigstore.PolicyEvaluatorConfig{
			Store:  p.authServer.AuthServer,
			Logger: logger,
		})
		if err != nil {
			return trace.Wrap(err, "creating Sigstore policy evaluator")
		}
		p.authServer.AuthServer.SetSigstorePolicyEvaluator(eval)
	}

	return nil
}

// AWSOIDCClient generates AWS OIDC tokens for the auth server.
type AWSOIDCClient struct {
	awsconfig.IntegrationGetter
	cache           awsoidc.Cache
	keyStoreManager awsoidc.KeyStoreManager
	serverID        string
	clock           clockwork.Clock
}

// GenerateAWSOIDCToken generates AWS OIDC tokens for the auth server.
func (c *AWSOIDCClient) GenerateAWSOIDCToken(ctx context.Context, integration string) (string, error) {
	token, err := awsoidc.GenerateAWSOIDCToken(ctx, c.cache, c.keyStoreManager, awsoidc.GenerateAWSOIDCTokenRequest{
		Integration: integration,
		Username:    c.serverID,
		Subject:     types.IntegrationAWSOIDCSubjectAuth,
		Clock:       c.clock,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return token, nil
}

func (p *Plugin) registerSCIMService(ctx context.Context, registrar grpc.ServiceRegistrar, cfg *common.Config) error {
	logger.InfoContext(ctx, "Registering SCIM service")

	if !p.Config.HostedPlugins.Enabled {
		// TODO(tcsc): handle dynamic updates to config/license features
		logger.InfoContext(ctx, "Hosted plugins disabled, not registering SCIM service")
		return nil
	}

	logger.DebugContext(ctx, "Creating SCIM service")
	scimService, err := scimservice.NewService(cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	logger.DebugContext(ctx, "Registering SCIM service with GRPC server")
	scimpb.RegisterSCIMServiceServer(registrar, scimService)

	logger.DebugContext(ctx, "Overriding default SCIM implementation")
	p.authServer.AuthServer.SetSCIMService(scimService)

	return nil
}

// registerAccessGraphService registers gRPC AccessGraphService in Auth.
// Proxy services call Auth rather than TAG directly, since only Auth holds the Cloud license
// which is required to authenticate to the external TAG service.
func (p *Plugin) registerAccessGraphService(ctx context.Context, authServer *auth.Server, service grpc.ServiceRegistrar, getClientCert getCertFunc) error {
	if !p.Config.AccessGraph.Enabled {
		return nil
	}

	logger.InfoContext(ctx, "Access Graph Enabled")

	// TODO(justinas): remove Access Graph relay gRPC service from auth altogether,
	// see https://github.com/gravitational/access-graph/issues/362
	agConn, err := accessgraph.NewAccessGraphClient(
		ctx,
		accessgraph.ServiceClientConfig{
			Addr:     p.Config.AccessGraph.Addr,
			CA:       p.Config.AccessGraph.CA,
			Insecure: p.Config.AccessGraph.Insecure,
		},
		getClientCert,
	)
	if err != nil {
		return trace.Wrap(err)
	}
	agClient := accessgraphv1.NewAccessGraphServiceClient(agConn)
	clusterName, err := authServer.GetDomainName()
	if err != nil {
		return trace.Wrap(err, "failed to get cluster name")
	}

	storage, err := local.NewAccessGraphSecretsService(p.authServer.GetBackend())
	if err != nil {
		return trace.Wrap(err, "failed to create access graph secrets service")
	}

	accessGraphService, err := accessgraph.NewService(accessgraph.ServiceConfig{
		Client:                agClient,
		Authorizer:            p.authServer.Authorizer,
		Logger:                p.logger,
		ClusterName:           clusterName,
		Storage:               storage,
		AuthPreferenceGetter:  p.authServer.AuthServer.GetReadOnlyAuthPreference,
		DeviceAssertionServer: p.authServer.AuthServer.GetDeviceAssertionServer(),
		UsageReporter:         p.authServer.AuthServer.UsageReporter,
		Modules:               modules.GetModules(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	accessgraphv1.RegisterAccessGraphServiceServer(service, accessGraphService)
	accessgraphsecretsv1pb.RegisterSecretsScannerServiceServer(service, accessGraphService)

	p.authServer.AuthServer.SetAccessGraphSecretService(storage)

	return nil
}

func (p *Plugin) initAndRegisterSecurityReport(ctx context.Context, serviceGRPC grpc.ServiceRegistrar) error {
	if p.AccessMonitoring == nil || !p.AccessMonitoring.Enabled {
		secreportsv1pb.RegisterSecReportsServiceServer(serviceGRPC, secreportsv1.NotImplementedService{})
		return nil
	}
	logger.InfoContext(ctx, "Access Monitoring Enabled")
	auditConf, err := p.authServer.AuthServer.GetClusterAuditConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	athenaURI, ok := athena.GetAthenaURI(auditConf.AuditEventsURIs())
	if !ok {
		logger.WarnContext(ctx, "Access Monitoring Enabled but Athena backend is not configured")
		secreportsv1pb.RegisterSecReportsServiceServer(serviceGRPC, secreportsv1.NotImplementedService{})
		return nil
	}

	features := modules.GetModules().Features()
	if !features.GetEntitlement(entitlements.AccessMonitoring).Enabled {
		logger.WarnContext(ctx, "Access Monitoring specified in config, but the subscription does not include Access Monitoring, Access Monitoring will not be enabled")
		secreportsv1pb.RegisterSecReportsServiceServer(serviceGRPC, secreportsv1.NotImplementedService{})
		return nil
	}

	modules.GetModules().EnableAccessMonitoring()

	storage, err := local.NewSecReportsService(p.authServer.GetBackend(), p.authServer.AuthServer.GetClock())
	if err != nil {
		return trace.Wrap(err)
	}
	limiter, err := limiter.NewLimiter(limiter.Config{
		Store:      storage,
		Semaphore:  p.authServer.AuthServer,
		Logger:     logger,
		Clock:      p.authServer.AuthServer.GetClock(),
		TotalLimit: p.AccessMonitoring.DataLimit,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	go limiter.UpdateLimiterBasedOnCloudProduct(ctx, p)

	// Initialize external audit storage configurator if available
	externalAuditStorage, err := p.newExternalAuditStorageConfigurator(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	secReportsSvc, err := secreports.NewService(secreports.ServiceConfig{
		Limiter:              limiter,
		AccessMonitoring:     p.AccessMonitoring,
		AthenaURL:            athenaURI,
		Authorizer:           p.authServer.Authorizer,
		Backend:              p.authServer.GetBackend(),
		Clock:                p.authServer.AuthServer.GetClock(),
		Emitter:              p.authServer.Emitter,
		LimiterStorage:       storage,
		Logger:               logger.With(teleport.ComponentKey, "mon"),
		ProcessContext:       ctx,
		Region:               auditConf.Region(),
		Semaphore:            p.authServer.AuthServer,
		Storage:              storage,
		ExternalAuditStorage: externalAuditStorage,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if err := secReportsSvc.Init(ctx); err != nil {
		return trace.Wrap(err)
	}

	secreportsv1pb.RegisterSecReportsServiceServer(serviceGRPC, secReportsSvc)
	return nil
}

func registerDeviceTrustService(logger *slog.Logger, s *grpc.Server, authGRPC *auth.GRPCServer) (*devicetrustv1.Service, error) {
	authServer := authGRPC.AuthServer
	deviceStorage, err := dtstorage.New(dtstorage.Params{
		Logger:       logger,
		Backend:      authGRPC.GetBackend(),
		UsersService: authServer.Services,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	deviceService, err := devicetrustv1.New(devicetrustv1.ServiceParams{
		Logger:              logger,
		AuthServer:          authServer,
		Authorizer:          authGRPC.Authorizer,
		CachedAccessService: authServer.Cache,
		CachedUsersService:  authServer.Cache,
		Emitter:             authGRPC.Emitter,
		Storage:             deviceStorage,
		Modules:             modules.GetModules(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	devicepb.RegisterDeviceTrustServiceServer(s, deviceService)

	// Wire DeviceWebToken creation into auth.Server.
	authServer.SetCreateDeviceWebTokenFunc(deviceService.CreateDeviceWebToken)
	authServer.SetDeviceAssertionServer(deviceService.CreateAssertCeremony)
	authServer.SetDevicesGetter(deviceStorage)
	return deviceService, nil
}

func (p *Plugin) registerLoginRuleService(server *auth.GRPCServer) error {
	storage := lrstorage.New(server.GetBackend())

	evaluator := loginrule.NewEvaluator(storage)
	server.AuthServer.SetLoginRuleEvaluator(evaluator)

	grpcServer, err := server.GetServer()
	if err != nil {
		return trace.Wrap(err)
	}
	service, err := loginrulev1.NewService(&loginrulev1.ServiceConfig{
		Storage:    storage,
		Authorizer: p.authServer.Authorizer,
		Emitter:    server.Emitter,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	loginrulepb.RegisterLoginRuleServiceServer(grpcServer, service)

	return nil
}

func (p *Plugin) registerExternalAuditStorageService(ctx context.Context) error {
	externalAuditStorage := local.NewExternalAuditStorageService(p.authServer.GetBackend())

	integrationsSvc, err := local.NewIntegrationsService(p.authServer.GetBackend())
	if err != nil {
		return trace.Wrap(err)
	}

	externalauditSvc, err := externalauditstoragev1.NewService(&externalauditstoragev1.ServiceConfig{
		Authorizer:               p.authServer.Authorizer,
		ExternalAuditStorage:     externalAuditStorage,
		ClusterAuditConfigGetter: p.authServer.AuthServer,
		IntegrationSvc:           integrationsSvc,
		OIDCTokenFn:              p.authServer.AuthServer.GenerateExternalAuditStorageOIDCToken,
		Emitter:                  p.authServer.Emitter,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	grpcServer, err := p.authServer.GetServer()
	if err != nil {
		return trace.Wrap(err)
	}

	externalauditstoragev1pb.RegisterExternalAuditStorageServiceServer(grpcServer, externalauditSvc)

	return nil
}

func (p *Plugin) registerPluginsService() (*pluginsv1.Service, error) {
	cfg := p.Config.HostedPlugins
	if !cfg.Enabled {
		return nil, nil
	}

	grpcServer, err := p.authServer.GetServer()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authorizers := plugins.NewAuthorizerSetFromConfig(p.HostedPlugins.OAuthProviders, p.Logger)
	service, err := pluginsv1.NewService(pluginsv1.ServiceConfig{
		Authorizer:                     p.authServer.Authorizer,
		AuthServer:                     p.authServer.AuthServer,
		DisabledPlugins:                getDisabledPlugins(),
		PluginService:                  p.plugins,
		PluginStaticCredentialsService: p.pluginCreds,
		PluginAuthorizers:              authorizers,
		Logger:                         p.logger,
		KeyStoreManager:                p.authServer.AuthServer.GetKeyStore(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pluginspb.RegisterPluginServiceServer(grpcServer, service)
	modules.GetModules().EnablePlugins()

	return service, nil
}

func (p *Plugin) registerResourceUsageService(server *auth.GRPCServer, cfg resourceusagev1.ServiceConfig) error {
	grpcServer, err := server.GetServer()
	if err != nil {
		return trace.Wrap(err)
	}

	cfg.Authorizer = p.authServer.Authorizer
	cfg.AuditLog = p.authServer.AuditLog
	service, err := resourceusagev1.New(cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	resourceusagepb.RegisterResourceUsageServiceServer(grpcServer, service)
	return nil
}

// RegisterAuthWebHandlers plugs in new handlers into OSS auth server router
func (p *Plugin) RegisterAuthWebHandlers(handler any) error {
	apiServer, ok := handler.(*auth.APIServer)
	if !ok {
		return trace.BadParameter("unsupported auth web handler type %T", handler)
	}

	apiServer.POST("/:version/saml/requests/validate", apiServer.WithAuth(validateSAMLResponseWeb))
	apiServer.POST("/:version/oidc/requests/validate", apiServer.WithAuth(validateOIDCAuthCallbackWeb))

	return nil
}

// getUpgradeWindowStartHour is passed to the oss auth server to let it pull the start
// hour when trying to generate a 'MaintenanceWindow' resource.
func (p *Plugin) getAccountUpgradeWindowStartHour(ctx context.Context) (int64, error) {
	clt := p.GetCloudClient()
	if clt == nil {
		return 0, trace.Errorf("cannot get cloud upgrade window start hour, cloud API client not yet registered")
	}
	rsp, err := clt.GetAccountUpgradeWindowStartHour(ctx, &cloudapi.EmptyRequest{})
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return rsp.GetUpgradeWindowStartHour(), nil
}

func getDisabledPlugins() []types.PluginType {
	disabledPluginsRaw := os.Getenv(envVarNameDisabledPlugins)
	var disabledPlugins []types.PluginType
	for disabledPluginRaw := range strings.SplitSeq(disabledPluginsRaw, ",") {
		disabledPlugin := strings.TrimSpace(disabledPluginRaw)
		if disabledPlugin == "" {
			continue
		}
		disabledPlugins = append(disabledPlugins, types.PluginType(disabledPlugin))
	}
	return disabledPlugins
}
