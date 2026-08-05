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
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport"
	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	beamsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	clientiprestrictionv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clientiprestriction/v1"
	devicetrustpublicv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/public/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	externalauditstoragev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/externalauditstorage/v1"
	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	resourceusagepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/resourceusage/v1"
	samlidppb "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	secreportsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/secreports/v1"
	sessionsearchv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/sessionsearch/v1"
	subcapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	workloadclusterv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadcluster/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	beamservicev1 "github.com/gravitational/teleport/e/api/beamservice/v1"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/e/lib/accessgraph"
	"github.com/gravitational/teleport/e/lib/accesslist"
	clientiprestrictionv1 "github.com/gravitational/teleport/e/lib/auth/clientiprestriction/clientiprestrictionv1"
	"github.com/gravitational/teleport/e/lib/auth/machineid/workloadidentityv1"
	"github.com/gravitational/teleport/e/lib/auth/summarizer"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/sessionsearchv1"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/summarizerv1"
	"github.com/gravitational/teleport/e/lib/auth/workloadcluster/workloadclusterv1"
	beamsv1 "github.com/gravitational/teleport/e/lib/beams/v1"
	beamscompute "github.com/gravitational/teleport/e/lib/beams/v1/compute"
	"github.com/gravitational/teleport/e/lib/devicetrust/devicetrustpublicv1"
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
	"github.com/gravitational/teleport/e/lib/plugins/pluginsv1"
	"github.com/gravitational/teleport/e/lib/resourceusage/resourceusagev1"
	scimservice "github.com/gravitational/teleport/e/lib/scim/service"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
	"github.com/gravitational/teleport/e/lib/secreports"
	"github.com/gravitational/teleport/e/lib/secreports/limiter"
	"github.com/gravitational/teleport/e/lib/secreports/query/athena"
	"github.com/gravitational/teleport/e/lib/sigstore"
	"github.com/gravitational/teleport/e/lib/subca/subcav1"
	"github.com/gravitational/teleport/entitlements"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
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
	pluginName                         = "auth.enterprise"
	envVarNameDisabledPlugins          = "TELEPORT_UNSTABLE_DISABLE_PLUGINS"
	envVarNameBedrockRegion            = "TELEPORT_BEDROCK_REGION"
	envVarNameBedrockModel             = "TELEPORT_BEDROCK_MODEL"
	envVarNameBeamServiceAddress       = "TELEPORT_BEAM_SERVICE_ADDRESS"
	envVarNameBeamServiceAddressSuffix = "TELEPORT_BEAM_SERVICE_ADDRESS_SUFFIX"
	envVarNameValidBeamRegions         = "TELEPORT_BEAM_SERVICE_VALID_REGIONS"
	beamServiceAddressPrefix           = "teleport-beam-"
)

var logger = logutils.NewPackageLogger(teleport.ComponentKey, pluginName)

// regionalBeamComputeClientProvider routes beam compute requests to regional
// orchestrators. Addresses are built as:
//
//	teleport-beam-<region><addressSuffix>
//
// Region validation happens in the Beam service before it asks the provider
// for a client.
type regionalBeamComputeClientProvider struct {
	addressSuffix string
	creds         credentials.TransportCredentials

	mu    sync.Mutex
	conns map[string]*grpc.ClientConn
}

func newRegionalBeamComputeClientProvider(addressSuffix string, creds credentials.TransportCredentials) *regionalBeamComputeClientProvider {
	return &regionalBeamComputeClientProvider{
		addressSuffix: addressSuffix,
		creds:         creds,
		conns:         make(map[string]*grpc.ClientConn),
	}
}

// ClientForRegion returns a cached gRPC client for the regional Beam compute
// service selected by region. The region is validated before it is
// interpolated into the service address.
func (p *regionalBeamComputeClientProvider) ClientForRegion(region string) (beamservicev1.BeamsOrchestratorServiceClient, error) {
	if region == "" {
		return nil, trace.BadParameter("beam region is required")
	}
	if err := beamsv1.ValidateBeamRegionSyntax(region); err != nil {
		return nil, trace.Wrap(err)
	}
	addr := beamServiceAddressPrefix + region + p.addressSuffix

	p.mu.Lock()
	defer p.mu.Unlock()
	if conn := p.conns[addr]; conn != nil {
		return beamservicev1.NewBeamsOrchestratorServiceClient(conn), nil
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(p.creds))
	if err != nil {
		return nil, trace.Wrap(err, "create regional beams compute service client")
	}
	p.conns[addr] = conn
	return beamservicev1.NewBeamsOrchestratorServiceClient(conn), nil
}

// Close closes all cached regional Beam compute service connections.
func (p *regionalBeamComputeClientProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var errs []error
	for addr, conn := range p.conns {
		if err := conn.Close(); err != nil {
			errs = append(errs, err)
			continue
		}
		delete(p.conns, addr)
	}
	return trace.NewAggregate(errs...)
}

// beamServiceAddrConfig resolves Beam compute service environment variables.
// When both the legacy single address and regional address suffix are set, the
// suffix is used for regional requests and the single address is retained for
// legacy clients that do not send a region.
func beamServiceAddrConfig() (addr, addrSuffix string, validRegions []string, err error) {
	addr = os.Getenv(envVarNameBeamServiceAddress)
	addrSuffix = os.Getenv(envVarNameBeamServiceAddressSuffix)
	validRegions = splitStringList(os.Getenv(envVarNameValidBeamRegions))

	if addrSuffix == "" {
		return addr, "", validRegions, nil
	}
	if !strings.HasPrefix(addrSuffix, ".") &&
		!strings.HasPrefix(addrSuffix, ":") {
		return "", "", nil, trace.BadParameter("%s must start with '.' or ':'", envVarNameBeamServiceAddressSuffix)
	}
	if len(validRegions) == 0 {
		return "", "", nil, trace.BadParameter("%s is required when using %s", envVarNameValidBeamRegions, envVarNameBeamServiceAddressSuffix)
	}
	return addr, addrSuffix, validRegions, nil
}

func splitStringList(s string) []string {
	if s == "" {
		return nil
	}
	var vs []string
	for v := range strings.SplitSeq(s, ",") {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		vs = append(vs, v)
	}
	return vs
}

type getCertFunc = func() (*tls.Certificate, error)

// License provides access to the license key pair.
type License interface {
	GetKeyPair() *liblicense.License
}

// LicenseChecker reports whether an enterprise license is currently disabled.
// It is implemented by [*emodules.EnterpriseModules] from e/tool/modules.
type LicenseChecker interface {
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

	// Modules defines build time constraints and licensed features.
	Modules modules.Modules

	// LicenseChecker is a service capable of checking if the license is valid.
	// EnterpriseModules implements this interface, but it is kept as a different
	// entry in the config for ease of testing.
	LicenseChecker LicenseChecker
	// Beams holds configuration for the Beams feature.
	Beams BeamsConfig
}

// BeamsConfig holds configuration for the Beams feature.
type BeamsConfig struct {
	// ComputeServiceClient overrides the client that will be used to communicate
	// with the compute service, primarily in tests.
	ComputeServiceClient beamservicev1.BeamsOrchestratorServiceClient
}

// NewPlugin creates an instance of the Enterprise Web Plugin
func NewPlugin(cfg Config) (*Plugin, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// If LicenseChecker is not set, fall back to License if it also implements
	// LicenseChecker. This allows callers that only set License (e.g. tests
	// using ValidLicense{}) to continue working without explicitly setting both.
	if cfg.LicenseChecker == nil {
		if lc, ok := cfg.License.(LicenseChecker); ok {
			cfg.LicenseChecker = lc
		}
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
	if !p.Config.Modules.Features().Cloud {
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

	// Register Cloud APIs.
	cloudapi.RegisterTenantsServiceServer(gRPCServer, &cloudWithRoles{
		plugin: p,
	})

	err = registerDeviceTrustService(p.logger, gRPCServer, p.authServer, p.Config.Modules)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := p.registerLoginRuleService(p.authServer); err != nil {
		return trace.Wrap(err)
	}

	if p.Config.Modules.Features().Cloud && p.Config.Modules.Features().GetEntitlement(entitlements.ExternalAuditStorage).Enabled {
		if err := p.registerExternalAuditStorageService(ctx); err != nil {
			return trace.Wrap(err)
		}
	}

	// Create a SAMLService and register it with the auth.Server
	sas, err := NewSAMLAuthService(&SAMLAuthServiceConfig{
		Auth:           p.authServer.AuthServer,
		Emitter:        p.authServer.Emitter,
		LicenseChecker: p.Config.LicenseChecker,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	p.authServer.AuthServer.SetSAMLService(sas)

	// Create a OIDCService and register it with the auth.Server
	oas, err := NewOIDCAuthService(&OIDCAuthServiceConfig{
		Auth:           p.authServer.AuthServer,
		Emitter:        p.authServer.Emitter,
		LicenseChecker: p.Config.LicenseChecker,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	p.authServer.AuthServer.SetOIDCService(oas)

	if p.Config.Modules.Features().Cloud {
		workloadclusterServiceServer, err := workloadclusterv1.NewService(workloadclusterv1.ServiceConfig{
			Authorizer:        p.authServer.Authorizer,
			Emitter:           p.authServer.Emitter,
			CloudClientGetter: p,
			Modules:           p.Config.Modules,
			Logger:            p.logger,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		workloadclusterv1pb.RegisterWorkloadClusterServiceServer(gRPCServer, workloadclusterServiceServer)
	}

	if p.Config.Modules.Features().Cloud {
		cirServiceServer, err := clientiprestrictionv1.NewService(clientiprestrictionv1.ServiceConfig{
			Authorizer:        p.authServer.Authorizer,
			Emitter:           p.authServer.Emitter,
			CloudClientGetter: p,
			Modules:           p.Config.Modules,
			Logger:            p.logger,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		clientiprestrictionv1pb.RegisterClientIPRestrictionServiceServer(gRPCServer, cirServiceServer)
	}

	keypair := p.Config.License.GetKeyPair()
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
			Authorizer:        p.authServer.ScopedAuthorizer,
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
			Modules:           p.Config.Modules,
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

	var accessGraphClientGetter func() (accessgraphv1.SessionRecordingServiceClient, error)
	if p.Config.AccessGraph.Enabled {
		accessGraphConn, err := accessgraph.NewAccessGraphClient(
			ctx,
			accessgraph.ServiceClientConfig{
				Addr:        p.Config.AccessGraph.Addr,
				CA:          p.Config.AccessGraph.CA,
				Insecure:    p.Config.AccessGraph.Insecure,
				LazyConnect: true,
			},
			getClientCert,
		)
		if err != nil {
			return trace.Wrap(err)
		}
		client := accessgraphv1.NewSessionRecordingServiceClient(accessGraphConn)
		accessGraphClientGetter = func() (accessgraphv1.SessionRecordingServiceClient, error) {
			return client, nil
		}
	} else {
		accessGraphClientGetter = func() (accessgraphv1.SessionRecordingServiceClient, error) {
			return nil, trace.NotFound("access graph is not enabled")
		}
	}
	availabilityCache, err := summarizer.NewAvailabilityCache(
		accessGraphClientGetter,
		p.authServer.AuthServer.GetClock(),
		0, // use default TTL
	)
	if err != nil {
		return trace.Wrap(err)
	}

	isSessionSummariesLicensed := func() bool {
		return p.Config.Modules.Features().GetEntitlement(entitlements.SessionSummaries).Enabled
	}

	sessionSummarizer, err := summarizer.NewSessionSummarizer(summarizer.SummarizerConfig{
		Cache:                            p.authServer.AuthServer.Cache,
		Streamer:                         p.authServer.AuthServer,
		SummaryUploader:                  p.authServer.AuthServer,
		Clock:                            p.authServer.AuthServer.GetClock(),
		EnableBedrockWithoutRestrictions: !p.Config.Modules.Features().Cloud,
		Encrypter:                        p.authServer.AuthServer.EncryptedIO,
		AWSConfigCache:                   cfgCache,
		EnvBedrockRegion:                 os.Getenv(envVarNameBedrockRegion),
		EnvBedrockModelID:                os.Getenv(envVarNameBedrockModel),
		AvailabilityCache:                availabilityCache,
		UsageReporter:                    p.authServer.AuthServer.UsageReporter,
		Emitter:                          p.authServer.AuthServer.GetEmitter(),
		AccessGraphClientGetter:          accessGraphClientGetter,
		IsLicensed:                       isSessionSummariesLicensed,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	p.authServer.AuthServer.SetSummarizerService(sessionSummarizer)

	summarizerService, err := summarizerv1.NewService(summarizerv1.ServiceConfig{
		Authorizer:                       p.authServer.Authorizer,
		Backend:                          p.authServer.AuthServer.Services,
		Cache:                            p.authServer.AuthServer.Cache,
		SummaryDownloader:                p.authServer.AuthServer,
		Decrypter:                        p.authServer.AuthServer.EncryptedIO,
		Emitter:                          p.authServer.Emitter,
		AWSConfigCache:                   cfgCache,
		EnableBedrockWithoutRestrictions: !p.Config.Modules.Features().Cloud,
		UsageReporter:                    p.authServer.AuthServer.UsageReporter,
		IsLicensed:                       isSessionSummariesLicensed,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	summarizerv1pb.RegisterSummarizerServiceServer(gRPCServer, summarizerService)

	sessionSearchService, err := sessionsearchv1.NewService(
		sessionsearchv1.ServiceConfig{
			Authorizer:              p.authServer.Authorizer,
			Cache:                   p.authServer.AuthServer.Cache,
			AWSConfigCache:          cfgCache,
			EnvBedrockRegion:        os.Getenv(envVarNameBedrockRegion),
			AccessGraphClientGetter: accessGraphClientGetter,
			AvailabilityCache:       availabilityCache,
			IsLicensed:              isSessionSummariesLicensed,
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}
	sessionsearchv1pb.RegisterSessionSearchServiceServer(gRPCServer, sessionSearchService)

	if err := p.registerResourceUsageService(p.authServer, resourceusagev1.ServiceConfig{
		Modules: p.Config.Modules,
	}); err != nil {
		return trace.Wrap(err)
	}

	err = p.registerSCIMService(ctx, gRPCServer, &common.Config{
		Authorizer:  p.authServer.Authorizer,
		AccessPoint: p.scimAccessPoint(),
		Backend:     p.authServer.AuthServer.Services,
		Logger:      logger,
		HTTPClient: &http.Client{
			Transport: p.HTTPTransport,
		},
		ClusterName: clusterName.GetClusterName(),
		Clock:       p.authServer.AuthServer.GetClock(),
		Modules:     p.Config.Modules,
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
		Modules:       p.Config.Modules,
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

	if err := registerSubCAService(ctx, gRPCServer, p.authServer); err != nil {
		return trace.Wrap(err, "register Sub CA service")
	}

	if err := p.registerBeamsService(ctx, gRPCServer); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// registerBeamsService registers the Beams gRPC service and starts its garbage
// collector. It is a no-op when the cluster is not entitled to Beams or no
// compute service client is available.
func (p *Plugin) registerBeamsService(ctx context.Context, grpcServer *grpc.Server) error {
	if !p.Config.Modules.Features().GetEntitlement(entitlements.Beams).Enabled {
		return nil
	}

	client := p.Config.Beams.ComputeServiceClient
	addr, addrSuffix, validRegions, err := beamServiceAddrConfig()
	if err != nil {
		return trace.Wrap(err)
	}
	if client == nil && addr == "" && addrSuffix == "" {
		return nil
	}

	clusterName, err := p.authServer.AuthServer.GetDomainName()
	if err != nil {
		return trace.Wrap(err, "getting cluster name")
	}

	var provider beamsv1.ComputeServiceClientProvider
	var defaultRegion string
	if client == nil {
		creds, err := beamscompute.TransportCredentials(ctx, beamscompute.TransportCredentialsConfig{
			ClusterName:          clusterName,
			AuthPreferenceGetter: p.authServer.AuthServer,
			CertAuthorityGetter:  p.authServer.AuthServer,
			Keystore:             p.authServer.AuthServer.GetKeyStore(),
			Logger:               logger.With(teleport.ComponentKey, "beams-transport-credentials"),
			Emitter:              p.authServer.Emitter,
		})
		if err != nil {
			return trace.Wrap(err, "creating beams transport credentials")
		}
		if addrSuffix != "" {
			regionalProvider := newRegionalBeamComputeClientProvider(addrSuffix, creds)
			// This context is only terminated when the server is closed
			context.AfterFunc(ctx, func() {
				if err := regionalProvider.Close(); err != nil {
					logger.WarnContext(context.Background(), "Failed to close regional Beam compute service clients", "error", err)
				}
			})
			provider = regionalProvider
			defaultRegion = validRegions[0]
		}
		if addr != "" {
			// For backward-compatibility with single-region configurations.
			conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(creds))
			if err != nil {
				return trace.Wrap(err, "create beams compute service client")
			}
			// This context is only terminated when the server is closed
			context.AfterFunc(ctx, func() {
				if err := conn.Close(); err != nil {
					logger.WarnContext(context.Background(), "Failed to close Beam compute service client", "error", err)
				}
			})
			client = beamservicev1.NewBeamsOrchestratorServiceClient(conn)
		}
	}

	srv, err := beamsv1.NewBeamService(beamsv1.BeamsServiceConfig{
		ClusterName:                  clusterName,
		AuthPreferenceGetter:         p.authServer.AuthServer,
		BeamReader:                   p.authServer.AuthServer,
		StorageBackend:               p.authServer.GetBackend(),
		AppWriter:                    p.authServer.AuthServer,
		BeamWriter:                   p.authServer.AuthServer,
		DelegationSessionWriter:      p.authServer.AuthServer,
		ProvisionTokenWriter:         p.authServer.AuthServer,
		UserWriter:                   p.authServer.AuthServer,
		RoleWriter:                   p.authServer.AuthServer,
		NodeWriter:                   p.authServer.AuthServer,
		WorkloadIdentityWriter:       p.authServer.AuthServer,
		ComputeServiceClient:         client,
		ComputeServiceClientProvider: provider,
		Authorizer:                   p.authServer.Authorizer,
		UsageReporter:                p.authServer.AuthServer,
		ValidRegions:                 validRegions,
		DefaultRegion:                defaultRegion,
		Logger:                       logger.With(teleport.ComponentKey, "beams-service"),
	})
	if err != nil {
		return trace.Wrap(err, "creating beams service")
	}
	beamsv1pb.RegisterBeamServiceServer(grpcServer, srv)

	gc, err := beamsv1.NewGarbageCollector(beamsv1.GarbageCollectorConfig{
		Cache:         p.authServer.AuthServer,
		Backend:       p.authServer.AuthServer.Services,
		BeamService:   srv,
		Semaphores:    p.authServer.AuthServer,
		HostID:        p.authServer.AuthServer.ServerID,
		UsageReporter: p.authServer.AuthServer,
		Logger:        logger.With(teleport.ComponentTeleport, "beams-garbage-collector"),
	})
	if err != nil {
		return trace.Wrap(err, "creating beams garbage collector")
	}
	go gc.Run(ctx)

	beamsConfigSrv, err := beamsv1.NewBeamsConfigService(beamsv1.BeamsConfigServiceConfig{
		Authorizer: p.authServer.Authorizer,
		Cache:      p.authServer.AuthServer.Cache,
		Backend:    p.authServer.AuthServer.Services.BeamsConfigService,
		Emitter:    p.authServer.Emitter,
		Logger:     logger.With(teleport.ComponentKey, "beams-config-service"),
	})
	if err != nil {
		return trace.Wrap(err, "creating beams config service")
	}
	beamsv1pb.RegisterBeamsConfigServiceServer(grpcServer, beamsConfigSrv)

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

// scimAccessPoint composes several distinct implementations into
// a single [common.AccessPoint] interface value.
func (p *Plugin) scimAccessPoint() common.AccessPoint {
	return &struct {
		common.Users
		common.Roles
		common.Identity
		common.Plugins
		common.Credentials
		common.JWTSigner
		types.Semaphores
		common.AccessLists
		services.AuthorityGetter
		common.Locks
	}{
		Users:           p.authServer.AuthServer,
		Roles:           p.authServer.AuthServer,
		Identity:        p.authServer.AuthServer,
		Plugins:         p.plugins,
		Credentials:     p.pluginCreds,
		JWTSigner:       p.authServer.AuthServer.GetKeyStore(),
		Semaphores:      p.authServer.AuthServer,
		AccessLists:     p.authServer.AuthServer,
		AuthorityGetter: p.authServer.AuthServer,
		Locks:           p.authServer.AuthServer,
	}
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
	agClient := accessgraphv1alpha.NewAccessGraphServiceClient(agConn)
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
		Modules:               p.Config.Modules,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	accessgraphv1alpha.RegisterAccessGraphServiceServer(service, accessGraphService)
	accessgraphsecretsv1pb.RegisterSecretsScannerServiceServer(service, accessGraphService)

	p.authServer.AuthServer.SetAccessGraphSecretService(storage)

	return nil
}

func (p *Plugin) initAndRegisterSecurityReport(ctx context.Context, serviceGRPC grpc.ServiceRegistrar) error {
	if p.AccessMonitoring == nil || !p.AccessMonitoring.Enabled {
		secreportsv1pb.RegisterSecReportsServiceServer(serviceGRPC, secreportsv1.NotImplementedService{
			CustomError: &trace.AccessDeniedError{
				Message: "Security Reports are unavailable because Access Monitoring is not enabled in Auth service configuration",
			},
		})
		return nil
	}

	var disabledReasons []string

	auditConf, err := p.authServer.AuthServer.GetClusterAuditConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	athenaURI, ok := athena.GetAthenaURI(auditConf.AuditEventsURIs())
	if !ok {
		disabledReasons = append(disabledReasons, "the Athena audit backend is not configured")
	}

	features := p.Config.Modules.Features()
	if !features.GetEntitlement(entitlements.AccessMonitoring).Enabled {
		disabledReasons = append(disabledReasons, "the subscription does not include Access Monitoring")
	}

	if len(disabledReasons) > 0 {
		logger.WarnContext(ctx, "Access Monitoring is disabled", "reasons", disabledReasons)
		reasonsStr := strings.Join(disabledReasons, ", ")
		secreportsv1pb.RegisterSecReportsServiceServer(serviceGRPC, secreportsv1.NotImplementedService{
			CustomError: &trace.AccessDeniedError{
				Message: "Security Reports are unavailable for the following reason(s): " + reasonsStr,
			},
		})
		return nil
	}
	logger.InfoContext(ctx, "Access Monitoring enabled")

	p.Config.Modules.EnableAccessMonitoring()

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
		Modules:    p.Config.Modules,
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
		Modules:              p.Config.Modules,
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

func registerDeviceTrustService(logger *slog.Logger, s *grpc.Server, authGRPC *auth.GRPCServer, m modules.Modules) error {
	authServer := authGRPC.AuthServer
	deviceStorage, err := dtstorage.New(dtstorage.Params{
		Logger:       logger,
		Backend:      authGRPC.GetBackend(),
		UsersService: authServer.Services,
		Modules:      m,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	deviceService, err := devicetrustv1.New(devicetrustv1.ServiceParams{
		Logger:              logger,
		AuthServer:          authServer,
		Authorizer:          authGRPC.Authorizer,
		CachedAccessService: authServer.Cache,
		CachedUsersService:  authServer.Cache,
		Emitter:             authGRPC.Emitter,
		Storage:             deviceStorage,
		EnrollPairing:       authServer.Services,
		Modules:             m,
	})
	if err != nil {
		return trace.Wrap(err, "setting up Device Trust service")
	}
	publicDeviceService, err := devicetrustpublicv1.New(devicetrustpublicv1.ServiceParams{
		Logger:        logger,
		EnrollPairing: authServer.Services,
		Storage:       deviceStorage,
		Authorizer:    authGRPC.Authorizer,
		CachedUsers:   authServer.Cache,
		Emitter:       authGRPC.Emitter,
		Modules:       m,
	})
	if err != nil {
		return trace.Wrap(err, "setting up public Device Trust service")
	}

	devicepb.RegisterDeviceTrustServiceServer(s, deviceService)
	devicetrustpublicv1pb.RegisterDeviceTrustServiceServer(s, publicDeviceService)

	// Wire DeviceWebToken creation into auth.Server.
	authServer.SetCreateDeviceWebTokenFunc(deviceService.CreateDeviceWebToken)
	authServer.SetDeviceAssertionServer(deviceService.CreateAssertCeremony)
	authServer.SetDevicesGetter(deviceStorage)
	return nil
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

	service, err := pluginsv1.NewService(pluginsv1.ServiceConfig{
		Authorizer:                     p.authServer.Authorizer,
		AuthServer:                     p.authServer.AuthServer,
		DisabledPlugins:                getDisabledPlugins(),
		PluginService:                  p.plugins,
		PluginStaticCredentialsService: p.pluginCreds,
		Logger:                         p.logger,
		KeyStoreManager:                p.authServer.AuthServer.GetKeyStore(),
		Modules:                        p.Config.Modules,
		HostedPluginConfig:             cfg,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pluginspb.RegisterPluginServiceServer(grpcServer, service)

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

func registerSubCAService(
	processContext context.Context,
	server grpc.ServiceRegistrar,
	authGRPC *auth.GRPCServer,
) error {
	authServer := authGRPC.AuthServer
	subCAService, err := subcav1.New(subcav1.ServiceParams{
		Clock:                   authServer.GetClock(),
		Logger:                  logger,
		CachedClusterNameGetter: authServer.Cache,
		CachedSubCA:             authServer.Cache,
		SubCA:                   authServer.Services,
		PendingCSR:              authServer.Services,
		Trust:                   authServer.Services,
		WatcherContext:          processContext,
		WatcherSource:           authServer.Cache,
		KeystoreManager:         authServer.GetKeyStore(),
		Authorizer:              authGRPC.Authorizer,
		Emitter:                 authGRPC.Emitter,
	})
	if err != nil {
		return trace.Wrap(err, "create Sub CA RPC service")
	}
	subcapb.RegisterSubCAServiceServer(server, subCAService)

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
