package auth

import (
	"net/http"

	"github.com/gravitational/reporting/types"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/e/lib/devicetrust/devicetrustv1"
	dtstorage "github.com/gravitational/teleport/e/lib/devicetrust/storage"
	"github.com/gravitational/teleport/e/lib/pro/enforcer"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/httplib"
)

const (
	pluginName = "auth.enterprise"
)

// Config is a configuration of the web plugin
type Config struct {
	// Log is the logger
	Log logrus.FieldLogger

	// GetBackend fetches the backend for the running Teleport process.
	// A func is used, instead of a plain field, so the Plugin may be created
	// before the actual Teleport process.
	GetBackend func() backend.Backend
}

// CheckAndSetDefaults checks and sets the defaults
func (c *Config) CheckAndSetDefaults() error {
	if c.Log == nil {
		c.Log = logrus.WithField(trace.Component, pluginName)
	}

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
	// enforcer is a service that heartbeats back to the control plane,
	// here it provides access to the heartbeat results for the handler
	enforcer *enforcer.Enforcer
	// cloudClient is a client of the Cloud API server
	cloudClient cloudapi.TenantsServiceClient
	// authorizer authorizes identity and returns auth context
	authorizer auth.Authorizer
	// emitter is events emitter, used to submit discrete events.
	emitter apievents.Emitter
}

// GetName returns plugin name
func (p *Plugin) GetName() string {
	return pluginName
}

// EnableEnforcer enables enforcer (teleport pro)
func (p *Plugin) EnableEnforcer(enforcer *enforcer.Enforcer) {
	p.enforcer = enforcer
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
	authServer, ok := server.(*auth.GRPCServer)
	if !ok {
		return trace.BadParameter("unsupported auth server type %T", server)
	}
	p.authorizer = authServer.Authorizer
	p.emitter = authServer.Emitter

	gRPCServer, err := authServer.GetServer()
	if err != nil {
		return trace.BadParameter("missing proto server")
	}

	// Register Cloud APIs.
	cloudapi.RegisterTenantsServiceServer(gRPCServer, &cloudWithRoles{
		plugin: p,
	})

	// Register Device Trust.
	deviceStorage, err := dtstorage.New(p.GetBackend)
	if err != nil {
		return trace.Wrap(err)
	}
	deviceService, err := devicetrustv1.New(devicetrustv1.ServiceParams{
		Authorizer: p.authorizer,
		Emitter:    p.emitter,
		Storage:    deviceStorage,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	devicepb.RegisterDeviceTrustServiceServer(gRPCServer, deviceService)

	// Create a SAMLService and register it with the auth.Server
	sas, err := NewSAMLAuthService(&SAMLAuthServiceConfig{
		Auth:    authServer.AuthServer,
		Emitter: authServer.Emitter,
		Log:     p.Log,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	authServer.AuthServer.SetSAMLService(sas)

	return nil
}

// RegisterAuthWebHandlers plugs in new handlers into OSS auth server router
func (p *Plugin) RegisterAuthWebHandlers(handler interface{}) error {
	apiServer, ok := handler.(*auth.APIServer)
	if !ok {
		return trace.BadParameter("unsupported auth web handler type %T", handler)
	}

	apiServer.GET("/:version/license/status", httplib.MakeHandler(p.getLicenseCheckResult))

	return nil
}

func (p *Plugin) getLicenseCheckResult(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	if p.enforcer == nil {
		return types.NewHeartbeat(), nil
	}
	return p.enforcer.GetLicenseCheckResult(r.Context())
}
