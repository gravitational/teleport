package web

import (
	"net/http"

	"github.com/gravitational/teleport/e/api/cloud"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/web"

	"github.com/gravitational/trace"

	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
)

const (
	pluginName = "web.enterprise"
)

// Config is a configuration of the web plugin
type Config struct {
	// Log is the logger
	Log logrus.FieldLogger
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
	h *web.Handler
}

// GetName returns plugin name
func (p *Plugin) GetName() string {
	return pluginName
}

// RegisterAuthServices registers GRPC services
func (p *Plugin) RegisterAuthServices(grpcServer interface{}) error {
	return nil
}

// RegisterAuthWebHandlers plugs in new handlers into OSS auth server router
func (p *Plugin) RegisterAuthWebHandlers(srv interface{}) error {
	return nil
}

// RegisterProxyWebHandlers registers to proxy web handler
func (p *Plugin) RegisterProxyWebHandlers(handler interface{}) error {
	h, ok := handler.(*web.Handler)
	if !ok {
		return trace.BadParameter("unsupported handler type %T", handler)
	}

	p.h = h
	h.GET("/enterprise/authconnectors", h.WithAuth(p.getAuthConnectorsHandle))
	h.PUT("/enterprise/saml", h.WithAuth(p.upsertSAMLConnectorHandle))
	h.POST("/enterprise/saml", h.WithAuth(p.upsertSAMLConnectorHandle))
	h.DELETE("/enterprise/saml/:name", h.WithAuth(p.deleteSAMLConnectorHandle))

	h.PUT("/enterprise/oidc", h.WithAuth(p.upsertOIDCConnectorHandle))
	h.POST("/enterprise/oidc", h.WithAuth(p.upsertOIDCConnectorHandle))
	h.DELETE("/enterprise/oidc/:name", h.WithAuth(p.deleteOIDCConnectorHandle))

	h.GET("/enterprise/license/status", httplib.MakeHandler(p.getLicenseCheckStatusHandle))

	h.POST("/enterprise/nodes/token", h.WithAuth(p.createScriptJoinTokenHandle))

	h.POST("/enterprise/accessrequest", h.WithAuth(p.createAccessRequestHandle))
	h.PUT("/enterprise/accessrequest", h.WithAuth(p.reviewAccessRequestHandle))
	h.DELETE("/enterprise/accessrequest/:requestId", h.WithAuth(p.deleteAccessRequestHandle))
	h.GET("/enterprise/accessrequest/:requestId", h.WithAuth(p.getAccessRequestHandle))
	h.GET("/enterprise/accessrequest", h.WithAuth(p.getAccessRequestsHandle))

	h.GET("/scripts/:token/install-node.sh", httplib.MakeHandler(p.getNodeJoinScriptHandle))
	h.GET("/scripts/:token/install-app.sh", httplib.MakeHandler(p.getAppJoinScriptHandle))

	if p.h.ClusterFeatures.GetCloud() {
		h.DELETE("/enterprise/cloud/card", p.withCloudAuth(p.removeCardHandle))
		h.POST("/enterprise/cloud/card", p.withCloudAuth(p.addCardHandle))
		h.PUT("/enterprise/cloud/card", p.withCloudAuth(p.updateCardHandle))
		h.GET("/enterprise/cloud/billing", p.withCloudAuth(p.getBillingInformationHandle))
		h.GET("/enterprise/cloud/cycles", p.withCloudAuth(p.listBillingCyclesHandle))
		h.GET("/enterprise/cloud/invoices", p.withCloudAuth(p.listInvoicesHandle))
		h.PUT("/enterprise/cloud/account", p.withCloudAuth(p.updateAccountHandle))

		// Recovery related endpoints.
		h.POST("/enterprise/cloud/recovery/start", p.withCloud(p.startAccountRecoveryHandle))
		h.POST("/enterprise/cloud/recovery/verify", p.withCloud(p.verifyAccountRecoveryHandle))
		h.POST("/enterprise/cloud/recovery/newcredentials", p.withCloud(p.completeAccountRecoveryHandle))
		h.GET("/enterprise/cloud/recovery/token/:token", p.withCloud(p.getAccountRecoveryTokenHandle))
		h.POST("/enterprise/cloud/recovery/codes", p.withCloud(p.createAccountRecoveryCodesHandle))
		h.GET("/enterprise/cloud/recovery/codes", h.WithAuth(p.getAccountRecoveryCodesMetadataHandle))
	}

	return nil
}

// CloudHandler is a authenticated handler that is used to provide an initialized instance of the cloud client API
type CloudHandler func(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error)

type cloudPublicHandler func(w http.ResponseWriter, r *http.Request, params httprouter.Params, client cloud.Client) (interface{}, error)

// withCloudAuth authenticates and request and initializes an instance of the cloud client API
func (p *Plugin) withCloudAuth(fn CloudHandler) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
		ctx, err := p.h.AuthenticateRequest(w, r, true)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		cloudClient, err := cloud.NewClientFromConnection(ctx.GetClientConnection())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return fn(w, r, ctx, cloudClient)
	})
}

// withCloud checks against CSRF attacks and provides an initiliazed instance of the cloud client API for public requests.
func (p *Plugin) withCloud(fn cloudPublicHandler) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
		if err := csrf.VerifyHTTPHeader(r); err != nil {
			p.Log.Warnf("unable to validate CSRF token %v", err)
			return nil, trace.AccessDenied("access denied")
		}

		proxyClient := p.h.GetProxyClient()
		client, ok := proxyClient.(*auth.Client)
		if !ok {
			return nil, trace.BadParameter("expected *auth.Client, got: %T", client)
		}

		cloudClient, err := cloud.NewClientFromConnection(client.GetConnection())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		res, err := fn(w, r, params, cloudClient)
		if err != nil {
			// Hide 429 error.
			if trace.IsLimitExceeded(err) {
				p.Log.Warn(err)
				return nil, trace.AccessDenied("unable to process your request")
			}
			return nil, trace.Wrap(err)
		}

		return res, nil
	})
}
