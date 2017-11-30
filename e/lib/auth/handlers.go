package auth

import (
	"net/http"

	"github.com/gravitational/teleport/e/lib/pro"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/httplib"

	"github.com/gravitational/reporting/types"
	"github.com/julienschmidt/httprouter"
)

// AuthPlugin extends OSS auth server API with enterprise-specific features
type AuthPlugin struct {
	// Enforcer is a service that heartbeats back to the control plane,
	// here it provides access to the heartbeat results for the handler
	Enforcer *pro.Enforcer
}

// InitPlugin sets the plugin that adds extra auth server handlers
func InitPlugin(enforcer *pro.Enforcer) {
	auth.SetPlugin(&AuthPlugin{
		Enforcer: enforcer,
	})
}

// AddHandler plugs in new handlers into OSS auth server router
func (ap *AuthPlugin) AddHandlers(srv *auth.APIServer) {
	srv.GET("/:version/heartbeat", httplib.MakeHandler(ap.getHeartbeat))
}

func (ap *AuthPlugin) getHeartbeat(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	if ap.Enforcer == nil {
		return types.NewHeartbeat(), nil
	}
	return ap.Enforcer.GetHeartbeatResult()
}
