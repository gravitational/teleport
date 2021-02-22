package web

import (
	"net/http"

	enterpriseauth "github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/web"

	"github.com/gravitational/trace"

	"github.com/julienschmidt/httprouter"
)

// Plugin is our plugins to web API of teleport OSS
type Plugin struct {
	// ProxyClient is authenticated auth server client
	ProxyClient auth.ClientI
}

// InitPlugin initializes sets plugin that adds extra web handlers
func InitPlugin() {
	web.SetPlugin(&Plugin{})
}

// AddHandlers registries Plugin handlers
func (p *Plugin) AddHandlers(h *web.Handler) {
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
	h.GET("/enterprise/accessrequest/:requestId", h.WithAuth(p.getAccessRequestHandle))
	h.GET("/enterprise/accessrequest", h.WithAuth(p.getAccessRequestsHandle))

	h.GET("/scripts/:token/install-node.sh", httplib.MakeHandler(p.getNodeJoinScriptHandle))
	h.GET("/scripts/:token/install-app.sh", httplib.MakeHandler(p.getAppJoinScriptHandle))

	p.ProxyClient = h.GetProxyClient()
}

// getLicenseCheckStatusHandle is GET handle that returns the license check status
func (p *Plugin) getLicenseCheckStatusHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	client, ok := p.ProxyClient.(*auth.Client)
	if !ok {
		return nil, trace.BadParameter("expected *auth.Client, got: %T", client)
	}
	enterpriseClient, err := enterpriseauth.NewClient(client)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	licenseCheckResult, err := enterpriseClient.GetLicenseCheckResult()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ui.NewLicenseCheckStatus(licenseCheckResult), nil
}
