package auth

import (
	"context"
	"net/http"
	"time"

	liblicense "github.com/gravitational/license"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	samlidppb "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/e/lib/devicetrust/devicetrustv1"
	dtstorage "github.com/gravitational/teleport/e/lib/devicetrust/storage"
	"github.com/gravitational/teleport/e/lib/idp/saml"
	"github.com/gravitational/teleport/e/lib/loginrule"
	"github.com/gravitational/teleport/e/lib/loginrule/loginrulev1"
	lrstorage "github.com/gravitational/teleport/e/lib/loginrule/storage"
	"github.com/gravitational/teleport/e/lib/plugins"
	"github.com/gravitational/teleport/e/lib/plugins/pluginsv1"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/release"
	"github.com/gravitational/teleport/lib/service/servicecfg"
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
	// cloudClient is a client of the Cloud API server
	cloudClient cloudapi.TenantsServiceClient
	// authServer is the authServer passed into RegisterAuthServices on
	// startup.
	authServer *auth.GRPCServer
}

// GetName returns plugin name
func (p *Plugin) GetName() string {
	return pluginName
}

// EnableCloud enables cloud features
func (p *Plugin) EnableCloud(client cloudapi.TenantsServiceClient) {
	p.cloudClient = client
}

// RegisterProxyWebHandlers registers to proxy web handler
func (p *Plugin) RegisterProxyWebHandlers(handler interface{}) error {
	return nil
}

// RegisterAuthServices registers Auth Services (GRPC)
func (p *Plugin) RegisterAuthServices(server interface{}) error {
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

	// Register Device Trust.
	deviceStorage, err := dtstorage.New(p.authServer.GetBackend())
	if err != nil {
		return trace.Wrap(err)
	}
	deviceService, err := devicetrustv1.New(devicetrustv1.ServiceParams{
		AuthServer: p.authServer.AuthServer,
		Authorizer: p.authServer.Authorizer,
		Emitter:    p.authServer.Emitter,
		Storage:    deviceStorage,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	devicepb.RegisterDeviceTrustServiceServer(gRPCServer, deviceService)

	if err := p.registerLoginRuleService(p.authServer); err != nil {
		return trace.Wrap(err)
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
	backendService := local.NewPluginsService(server.GetBackend())
	service, err := pluginsv1.NewService(pluginsv1.ServiceConfig{
		Authorizer:        p.authServer.Authorizer,
		BackendService:    backendService,
		PluginAuthorizers: authorizers,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	pluginspb.RegisterPluginServiceServer(grpcServer, service)
	modules.GetModules().EnablePlugins()

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
