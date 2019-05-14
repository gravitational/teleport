package auth

import (
	"context"
	"net/http"

	"github.com/gravitational/teleport/e/lib/pro"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/httplib"

	"github.com/gravitational/reporting/types"
	"github.com/julienschmidt/httprouter"
)

// AuthPlugin extends OSS auth server API with enterprise-specific features
type AuthPlugin struct {
	// enforcer is a service that heartbeats back to the control plane,
	// here it provides access to the heartbeat results for the handler
	enforcer *pro.Enforcer
}

// InitPlugin sets the plugin that adds extra auth server handlers
func InitPlugin() {
	auth.SetPlugin(plugin)
}

// SetEnforcer sets the enforcer service on this plugin
func SetEnforcer(enforcer *pro.Enforcer) {
	plugin.enforcer = enforcer
}

// AddHandler plugs in new handlers into OSS auth server router
func (ap *AuthPlugin) AddHandlers(srv *auth.APIServer) {
	srv.GET("/:version/license/status", httplib.MakeHandler(ap.getLicenseCheckResult))
}

func (ap *AuthPlugin) getLicenseCheckResult(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	if ap.enforcer == nil {
		return types.NewHeartbeat(), nil
	}
	return ap.enforcer.GetLicenseCheckResult(context.TODO())
}

// plugin is the auth plugin
var plugin = &AuthPlugin{}
