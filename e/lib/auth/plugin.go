package auth

import (
	"context"
	"net/http"
	"sync"
	"time"

	liblicense "github.com/gravitational/license"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	externalauditstoragev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/externalauditstorage/v1"
	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	resourceusagepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/resourceusage/v1"
	samlidppb "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	secreportsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/secreports/v1"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/e/lib/accessgraph"
	"github.com/gravitational/teleport/e/lib/accesslist"
	"github.com/gravitational/teleport/e/lib/devicetrust/devicetrustv1"
	dtstorage "github.com/gravitational/teleport/e/lib/devicetrust/storage"
	"github.com/gravitational/teleport/e/lib/externalauditstorage/externalauditstoragev1"
	"github.com/gravitational/teleport/e/lib/idp/saml"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/e/lib/loginrule"
	"github.com/gravitational/teleport/e/lib/loginrule/loginrulev1"
	lrstorage "github.com/gravitational/teleport/e/lib/loginrule/storage"
	"github.com/gravitational/teleport/e/lib/okta"
	"github.com/gravitational/teleport/e/lib/plugins"
	"github.com/gravitational/teleport/e/lib/plugins/pluginsv1"
	"github.com/gravitational/teleport/e/lib/resourceusage/resourceusagev1"
	"github.com/gravitational/teleport/e/lib/secreports"
	"github.com/gravitational/teleport/e/lib/secreports/limiter"
	"github.com/gravitational/teleport/e/lib/secreports/query/athena"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/release"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

const (
	pluginName = "auth.enterprise"
)

var log = logrus.WithField(trace.Component, pluginName)

// License is an interface for checking if a license is disabled.
type License interface {
	GetKeyPair() *liblicense.License
	IsDisabled() bool
}

// ErrLicenseExpired is the error returned when a feature is disabled due to license expiry.
var ErrLicenseExpired = trace.AccessDenied("Teleport Enterprise license expired")

// Config is a configuration of the web plugin
type Config struct {
	// License holds the license under which the Teleport instance is running.
	License License

	HostedPlugins servicecfg.HostedPluginsConfig

	// AccessMonitoring holds the configuration for the Access Monitoring feature.
	AccessMonitoring *servicecfg.AccessMonitoringOptions

	// AccessGraph holds the configuration for the Access Graph feature.
	AccessGraph servicecfg.AccessGraphConfig
}

// CheckAndSetDefaults checks and sets the defaults
func (c *Config) CheckAndSetDefaults() error {
	return nil
}

// NewPlugin creates an instance of the Enterprise Web Plugin
func NewPlugin(cfg Config) (*Plugin, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Plugin{
		Config: cfg,
	}, nil
}

// Plugin extends OSS auth server API with enterprise features
type Plugin struct {
	Config
	// authServer is the authServer passed into RegisterAuthServices on
	// startup.
	authServer *auth.GRPCServer
	// plugins is the plugins backend service.
	plugins services.Plugins
	// mtx protects the cloudClient field.
	mtx sync.Mutex
	// cloudClient is a client of the Cloud API server
	cloudClient cloudapi.TenantsServiceClient
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

// RegisterProxyWebHandlers registers to proxy web handler
func (p *Plugin) RegisterProxyWebHandlers(handler interface{}) error {
	return nil
}

// RegisterAuthServices registers Auth Services (GRPC)
func (p *Plugin) RegisterAuthServices(ctx context.Context, server interface{}) error {
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

	deviceService, err := registerDeviceTrustService(gRPCServer, p.authServer)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := p.registerLoginRuleService(p.authServer); err != nil {
		return trace.Wrap(err)
	}

	if modules.GetModules().Features().Cloud && !modules.GetModules().Features().IsTeam() {
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

	// Create plugins service
	if err := p.registerPluginsService(p.authServer); err != nil {
		return trace.Wrap(err)
	}

	signingService, err := saml.NewSigningService(&saml.SigningServiceConfig{
		Client:     p.authServer.AuthServer,
		KeyStore:   p.authServer.AuthServer.GetKeyStore(),
		Authorizer: p.authServer.Authorizer,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	samlidppb.RegisterSAMLIdPServiceServer(gRPCServer, signingService)

	// register the start hour getter so that auth can use it during periodic MaintenanceWindow
	// resource sync.
	p.authServer.AuthServer.SetUpgradeWindowStartHourGetter(p.getAccountUpgradeWindowStartHour)

	clusterName, err := p.authServer.AuthServer.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}

	// Register the Okta user assignment creator login hook.
	uac, err := okta.NewUserAssignmentCreator(okta.UserAssignmentCreatorConfig{
		Log:         log,
		ClusterName: clusterName.GetClusterName(),
		AccessPoint: p.authServer.AuthServer,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	accessListStorage, err := local.NewAccessListService(p.authServer.GetBackend(), p.authServer.AuthServer.GetClock())
	if err != nil {
		return trace.Wrap(err)
	}

	accessListSvc, err := accesslist.NewService(accesslist.ServiceConfig{
		Authorizer:          p.authServer.Authorizer,
		AccessLists:         accessListStorage,
		LockGetter:          p.authServer.AuthServer,
		AccessListReviews:   accessListStorage,
		Emitter:             p.authServer.Emitter,
		UsageEvents:         p.authServer.AuthServer,
		Clock:               p.authServer.AuthServer.GetClock(),
		CachedUsersServices: p.authServer.AuthServer.Cache,
		AuthServer:          p.authServer.AuthServer,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	accesslistv1.RegisterAccessListServiceServer(gRPCServer, accessListSvc)

	if err := p.registerAccessGraphService(ctx, gRPCServer); err != nil {
		return trace.Wrap(err)
	}

	if err := p.initAndRegisterSecurityReport(ctx, gRPCServer); err != nil {
		return trace.Wrap(err)
	}

	p.authServer.AuthServer.RegisterLoginHook(uac.OnLogin)

	if err := p.registerResourceUsageService(p.authServer, resourceusagev1.ServiceConfig{
		GetDevicesUsageFunc: deviceService.GetResourceDevicesUsage,
	}); err != nil {
		return trace.Wrap(err)
	}

	userMonitor, err := NewUserMonitor(ctx, UserMonitorConfig{
		Log:        log,
		AuthServer: p.authServer.AuthServer,
		Events:     p.authServer.AuthServer.Services,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	userMonitor.Start(ctx)

	return nil
}

// registerAccessGraphService registers gRPC AccessGraphService in Auth.
// Proxy services call Auth rather than TAG directly, since only Auth holds the Cloud license
// which is required to authenticate to the external TAG service.
func (p *Plugin) registerAccessGraphService(ctx context.Context, service grpc.ServiceRegistrar) error {
	if !p.Config.AccessGraph.Enabled {
		return nil
	}

	log.Info("Access Graph Enabled.")

	license, ok := p.Config.License.(*licensefile.LicenseFile)
	if !ok {
		return trace.BadParameter("invalid license type %T", p.Config.License)
	}

	agConn, err := accessgraph.NewAccessGraphClient(
		ctx,
		accessgraph.ServiceClientConfig{
			Addr:     p.Config.AccessGraph.Addr,
			CA:       p.Config.AccessGraph.CA,
			License:  license,
			Insecure: p.Config.AccessGraph.Insecure,
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}
	agClient := accessgraphv1.NewAccessGraphServiceClient(agConn)

	accessGraphService, err := accessgraph.NewService(accessgraph.ServiceConfig{
		Client:     agClient,
		Authorizer: p.authServer.Authorizer,
		Logger:     log,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	accessgraphv1.RegisterAccessGraphServiceServer(service, accessGraphService)

	return nil
}

func (p *Plugin) initAndRegisterSecurityReport(ctx context.Context, serviceGRPC grpc.ServiceRegistrar) error {
	if p.AccessMonitoring == nil || !p.AccessMonitoring.Enabled {
		return nil
	}
	log.Infof("Access Monitoring Enabled.")
	auditConf, err := p.authServer.AuthServer.GetClusterAuditConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	athenaURI, ok := athena.GetAthenaURI(auditConf.AuditEventsURIs())
	if !ok {
		log.Warn("Access Monitoring Enabled but Athena backend is not configured.")
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
		Log:        log,
		Clock:      p.authServer.AuthServer.GetClock(),
		TotalLimit: p.AccessMonitoring.DataLimit,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	go limiter.UpdateLimiterBasedOnCloudProduct(ctx, p)

	secReportsSvc, err := secreports.NewService(secreports.ServiceConfig{
		Limiter:          limiter,
		AccessMonitoring: p.AccessMonitoring,
		AthenaURL:        athenaURI,
		Authorizer:       p.authServer.Authorizer,
		Backend:          p.authServer.GetBackend(),
		Clock:            p.authServer.AuthServer.GetClock(),
		Emitter:          p.authServer.Emitter,
		LimiterStorage:   storage,
		Logger:           log.WithField(trace.Component, "mon"),
		ProcessContext:   ctx,
		Region:           auditConf.Region(),
		Semaphore:        p.authServer.AuthServer,
		Storage:          storage,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if err := secReportsSvc.Init(ctx); err != nil {
		return trace.Wrap(err)
	}

	secreportsv1.RegisterSecReportsServiceServer(serviceGRPC, secReportsSvc)
	return nil
}

func registerDeviceTrustService(s *grpc.Server, authGRPC *auth.GRPCServer) (*devicetrustv1.Service, error) {
	authServer := authGRPC.AuthServer
	deviceStorage, err := dtstorage.New(dtstorage.Params{
		Backend:      authGRPC.GetBackend(),
		UsersService: authServer.Services,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	deviceService, err := devicetrustv1.New(devicetrustv1.ServiceParams{
		AuthServer:         authServer,
		Authorizer:         authGRPC.Authorizer,
		CachedUsersService: authServer.Cache,
		Emitter:            authGRPC.Emitter,
		Storage:            deviceStorage,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	devicepb.RegisterDeviceTrustServiceServer(s, deviceService)
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

func (p *Plugin) registerPluginsService(server *auth.GRPCServer) error {
	cfg := p.Config.HostedPlugins
	if !cfg.Enabled {
		return nil
	}

	grpcServer, err := server.GetServer()
	if err != nil {
		return trace.Wrap(err)
	}

	authorizers := plugins.NewAuthorizerSetFromConfig(p.HostedPlugins.OAuthProviders)
	p.plugins = local.NewPluginsService(server.GetBackend())
	pluginStaticCredentialsService, err := local.NewPluginStaticCredentialsService(server.GetBackend())
	if err != nil {
		return trace.Wrap(err)
	}
	service, err := pluginsv1.NewService(pluginsv1.ServiceConfig{
		Authorizer:                     p.authServer.Authorizer,
		PluginService:                  p.plugins,
		PluginStaticCredentialsService: pluginStaticCredentialsService,
		PluginAuthorizers:              authorizers,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	pluginspb.RegisterPluginServiceServer(grpcServer, service)
	modules.GetModules().EnablePlugins()

	return nil
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
func (p *Plugin) RegisterAuthWebHandlers(handler interface{}) error {
	apiServer, ok := handler.(*auth.APIServer)
	if !ok {
		return trace.BadParameter("unsupported auth web handler type %T", handler)
	}

	apiServer.GET("/:version/license/status", httplib.MakeHandler(p.getLicenseCheckResult))
	apiServer.POST("/:version/saml/requests/validate", apiServer.WithAuth(validateSAMLResponseWeb))
	apiServer.POST("/:version/oidc/requests/validate", apiServer.WithAuth(validateOIDCAuthCallbackWeb))

	return nil
}

// TODO(espadolini): delete once we're sure that the proxy doesn't use this
func (p *Plugin) getLicenseCheckResult(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	return map[string]any{
		"kind":    "heartbeat",
		"version": "v2",
		"metadata": map[string]any{
			"name":    "heartbeat",
			"created": time.Now().UTC(),
		},
		"spec": map[string]any{},
	}, nil
}

// getUpgradeWindowStartHour is passed to the oss auth server to let it pull the start
// hour when trying to generate a 'MaintenanceWindow' resource.
func (p *Plugin) getAccountUpgradeWindowStartHour(ctx context.Context) (int64, error) {
	rsp, err := p.cloudClient.GetAccountUpgradeWindowStartHour(ctx, &cloudapi.EmptyRequest{})
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return rsp.GetUpgradeWindowStartHour(), nil
}
