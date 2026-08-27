package web

import (
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web"
)

// registerClientErrorHandlers registers the client-side web UI error reporting endpoint.
// Only registered where Cloud Panel itself is reachable, cloud-hosted or usage-based
// dashboard tenants
func (p *Plugin) registerClientErrorHandlers() {
	features := p.h.GetClusterFeatures()
	hasCloudPanelAccess := features.GetCloud() || (services.IsDashboard(features) && features.IsUsageBased)
	if !hasCloudPanelAccess {
		return
	}
	p.h.POST("/enterprise/log", p.h.WithAuth(p.reportClientErrorHandle))
}

// reportClientErrorRequest is the body of a POST to /enterprise/log
type reportClientErrorRequest struct {
	// ClientErrorComponent identifies which part of the client is reporting the error.
	ClientErrorComponent string `json:"client_component"`
	// ClientErrorSource categorizes the kind of failure being reported.
	ClientErrorSource string `json:"error_source"`
}

// Client-error component identifiers. Must stay in sync with the frontend's
// component strings (e.g. Cloud.tsx's reportClientError('cloud-panel', ...)).
const (
	clientErrorComponentCloudPanel = "cloud-panel"
)

// Client-error source identifiers. Must stay in sync with the frontend's
// ClientErrorSource type in `e/web/teleport/src/services/clienterror/clientError.ts`
const (
	clientErrorSourceNetwork = "network"
	clientErrorSourceRender  = "render"
)

var validClientErrorComponents = map[string]bool{
	clientErrorComponentCloudPanel: true,
}

var validClientErrorSources = map[string]bool{
	clientErrorSourceNetwork: true,
	clientErrorSourceRender:  true,
}

// reportClientErrorHandle logs client-side web UI errors so they show up alongside this proxy's other logs.
func (p *Plugin) reportClientErrorHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *web.SessionContext) (any, error) {
	limited := p.h.WithLimiterHandlerFunc(func(w http.ResponseWriter, r *http.Request, params httprouter.Params) (any, error) {
		var req reportClientErrorRequest
		if err := httplib.ReadJSON(r, &req); err != nil {
			return nil, trace.Wrap(err)
		}
		if !validClientErrorComponents[req.ClientErrorComponent] {
			return nil, trace.BadParameter("invalid client_component %q", req.ClientErrorComponent)
		}
		if !validClientErrorSources[req.ClientErrorSource] {
			return nil, trace.BadParameter("invalid error_source %q", req.ClientErrorSource)
		}

		p.Logger.WarnContext(r.Context(), "Web UI client error",
			"origin", "client",
			"client_component", req.ClientErrorComponent,
			"error_source", req.ClientErrorSource,
		)
		return nil, nil
	})
	return limited(w, r, params)
}
