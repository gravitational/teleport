package web

import (
	"net/http"

	"github.com/gravitational/teleport/e/lib/web/ui"

	telebackend "github.com/gravitational/teleport/lib/backend"
	telehttplib "github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/web"
	teleui "github.com/gravitational/teleport/lib/web/ui"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

// Plugin is our plugins to web API of teleport OSS
type Plugin struct {
}

// AddHandlers registeres Plugin handlers
func (p *Plugin) AddHandlers(h *web.Handler) {
	h.GET("/enterprise/trustedclusters", h.WithAuth(p.getTrustedClusters))
	h.PUT("/enterprise/trustedclusters", h.WithAuth(p.upsertTrustedCluster))
	h.GET("/enterprise/roles", h.WithAuth(p.getRoles))
	h.POST("/enterprise/roles", h.WithAuth(p.createRole))
	h.PUT("/enterprise/roles", h.WithAuth(p.updateRole))
	h.DELETE("/enterprise/roles/:rolename", h.WithAuth(p.deleteRole))
	h.GET("/enterprise/oidc", h.WithAuth(p.getOIDConnectors))
	h.POST("/enterprise/oidc", h.WithAuth(p.createOIDConnector))
	h.PUT("/enterprise/oidc", h.WithAuth(p.updateOIDConnector))
	h.DELETE("/enterprise/oidc/:oidconnectorname", h.WithAuth(p.deleteOIDConnectors))
}

// getRoles is an example handler
//
// GET /v1/enterprise/roles
//
// Sucessful response:
//
// {"ok":"ok"}
//
func (p *Plugin) getTrustedClusters(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c *web.SessionContext) (interface{}, error) {
	clt, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	teleTrustedClusters, err := clt.GetTrustedClusters()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiTrustedClrs := []ui.TrustedCluster{}
	for _, item := range teleTrustedClusters {
		uiTrustedClrs = append(uiTrustedClrs, ui.NewTrustedCluster(item))
	}

	return uiTrustedClrs, nil
}

func (p *Plugin) upsertTrustedCluster(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c *web.SessionContext) (interface{}, error) {
	clt, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req *ui.TrustedCluster
	if err := telehttplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	teleTrustedClr, err := clt.GetTrustedCluster(req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	teleTrustedClr.SetEnabled(req.Enabled)

	if err := clt.UpsertTrustedCluster(teleTrustedClr); err != nil {
		return nil, trace.Wrap(err)
	}

	return ok(), nil
}

// getRoles is an example handler
//
// GET /v1/enterprise/roles
//
// Sucessful response:
//
// {"ok":"ok"}
//
func (p *Plugin) getRoles(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c *web.SessionContext) (interface{}, error) {
	clt, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	teleRoles, err := clt.GetRoles()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiroles := []teleui.Role{}
	for _, item := range teleRoles {
		uiroles = append(uiroles, *teleui.NewRole(item))
	}

	return uiroles, nil
}

// updateRole updates existing one
//
// PUT /v1/enterprise/roles
//
// Sucessful response:
//
// {"ok":"ok"}
//
func (p *Plugin) updateRole(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c *web.SessionContext) (interface{}, error) {
	clt, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req *teleui.Role
	if err := telehttplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	teleRole, err := clt.GetRole(req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req.Access.Apply(teleRole)

	if err := clt.UpsertRole(teleRole, telebackend.Forever); err != nil {
		return nil, trace.Wrap(err)
	}

	return ok(), nil
}

// createRole updates existing one
//
// POST /v1/enterprise/roles
//
// Sucessful response:
//
// {"ok":"ok"}
//
func (p *Plugin) createRole(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c *web.SessionContext) (interface{}, error) {
	clt, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req *teleui.Role
	if err := telehttplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	teleRole, err := req.ToTeleRole()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := clt.UpsertRole(teleRole, telebackend.Forever); err != nil {
		return nil, trace.Wrap(err)
	}

	return ok(), nil
}

// deleteRole updates existing one
//
// DELETE /v1/enterprise/roles
//
// Sucessful response:
//
// {"ok":"ok"}
//
func (p *Plugin) deleteRole(w http.ResponseWriter, r *http.Request, params httprouter.Params, c *web.SessionContext) (interface{}, error) {
	clt, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	roleName := params.ByName("rolename")
	if err := clt.DeleteRole(roleName); err != nil {
		return nil, trace.Wrap(err)
	}

	return ok(), nil
}

// getOIConnectors returns oicd connectors
//
// GET /v1/enterprise/iodc
//
// Sucessful response:
//
//  [{"name":"", "clientId":"","issuerUrl":"","redirectUrl":"","scopes":["one","two","three"],"roleMapping":{"claim":{"claim_value":["role_name"]}}}]
//
func (p *Plugin) getOIDConnectors(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c *web.SessionContext) (interface{}, error) {
	clt, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	teleConnectors, err := clt.GetOIDCConnectors(true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiConnectors := []ui.OIDConnector{}
	for _, item := range teleConnectors {
		uiConnectors = append(uiConnectors, ui.NewOIDConnector(item))
	}

	return uiConnectors, nil
}

// createOIConnector creates oidc connector
//
// POST /v1/enterprise/iodc
//
// Sucessful response:
//
// {"ok":"ok"}
//
func (p *Plugin) createOIDConnector(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c *web.SessionContext) (interface{}, error) {
	clt, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiConnector := ui.OIDConnector{}
	if err := telehttplib.ReadJSON(r, &uiConnector); err != nil {
		return nil, trace.Wrap(err)
	}

	storageConnector := uiConnector.ToStorageConnector()
	if err := clt.UpsertOIDCConnector(storageConnector); err != nil {
		return nil, trace.Wrap(err)
	}

	return ok(), nil
}

// deleteOIConnectors deletes oidc connector
//
// DELETE /v1/enterprise/iodc/:name
//
// Sucessful response:
//
// {"ok":"ok"}
//
func (p *Plugin) deleteOIDConnectors(w http.ResponseWriter, r *http.Request, params httprouter.Params, c *web.SessionContext) (interface{}, error) {
	clt, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = clt.DeleteOIDCConnector(params[0].Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ok(), nil
}

// updateOIConnector returns oicd connectors
//
// PUT /v1/enterprise/iodc
//
//  [{"name":"", "clientId":"","issuerUrl":"","redirectUrl":"","scopes":["one","two","three"],"roleMapping":{"claim":{"claim_value":["role_name"]}}}]
//
func (p *Plugin) updateOIDConnector(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c *web.SessionContext) (interface{}, error) {
	clt, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiConnector := ui.OIDConnector{}
	if err := telehttplib.ReadJSON(r, &uiConnector); err != nil {
		return nil, trace.Wrap(err)
	}

	teleConnector, err := clt.GetOIDCConnector(uiConnector.ID, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiConnector.Apply(teleConnector)
	if err := clt.UpsertOIDCConnector(teleConnector); err != nil {
		return nil, trace.Wrap(err)
	}

	return ok(), nil
}

// message returns structured message response
func message(msg string) interface{} {
	return map[string]string{"message": msg}
}

// ok returns structured OK response
func ok() interface{} {
	return message("OK")
}
