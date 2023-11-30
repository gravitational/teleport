package web

import (
	"context"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/types"
	enterpriseui "github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/teleport/lib/web/ui"
)

func (p *Plugin) getAuthConnectorsHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return getAuthConnectors(r.Context(), clt)
}

func getAuthConnectors(ctx context.Context, clt resourcesAPIGetter) ([]ui.ResourceItem, error) {
	githubConns, err := clt.GetGithubConnectors(ctx, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	madeGithubConns, err := ui.NewGithubConnectors(githubConns)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	samlConns, err := clt.GetSAMLConnectors(ctx, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	madeSAMLConns, err := enterpriseui.NewSAMLConnectors(samlConns)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	oidcConns, err := clt.GetOIDCConnectors(ctx, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	madeOIDCConns, err := enterpriseui.NewOIDCConnectors(oidcConns)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cap := len(madeGithubConns) + len(madeSAMLConns) + len(madeOIDCConns)
	conns := make([]ui.ResourceItem, 0, cap)

	conns = append(conns, madeGithubConns...)
	conns = append(conns, madeSAMLConns...)
	conns = append(conns, madeOIDCConns...)

	return conns, nil
}

func (p *Plugin) deleteSAMLConnectorHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connectorName := params.ByName("name")
	if err := clt.DeleteSAMLConnector(r.Context(), connectorName); err != nil {
		return nil, trace.Wrap(err)
	}

	return web.OK(), nil
}

func (p *Plugin) createSAMLConnectorHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := web.CreateResource(r, types.KindSAMLConnector, services.UnmarshalSAMLConnector, clt.CreateSAMLConnector)
	return item, trace.Wrap(err)
}

func (p *Plugin) updateSAMLConnectorHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := web.UpdateResource(r, params, types.KindSAMLConnector, services.UnmarshalSAMLConnector, clt.UpdateSAMLConnector)
	return item, trace.Wrap(err)
}

func (p *Plugin) deleteOIDCConnectorHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connectorName := params.ByName("name")
	if err := clt.DeleteOIDCConnector(r.Context(), connectorName); err != nil {
		return nil, trace.Wrap(err)
	}

	return web.OK(), nil
}

func (p *Plugin) createOIDCConnectorHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := web.CreateResource(r, types.KindOIDC, services.UnmarshalOIDCConnector, clt.CreateOIDCConnector)
	return item, trace.Wrap(err)
}

func (p *Plugin) updateOIDCConnectorHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := web.UpdateResource(r, params, types.KindOIDC, services.UnmarshalOIDCConnector, clt.UpdateOIDCConnector)
	return item, trace.Wrap(err)
}

func (p *Plugin) upsertSAMLIdPServiceProviderHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req enterpriseui.CreateSAMLIdPServiceProviderRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	upsertedItem, err := upsertSAMLIdPServiceProvider(r.Context(), clt, req, r.Method, params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return upsertedItem, nil
}

func upsertSAMLIdPServiceProvider(ctx context.Context, clt resourcesAPIGetter, req enterpriseui.CreateSAMLIdPServiceProviderRequest, httpMethod string, params httprouter.Params) (*ui.ResourceItem, error) {
	get := func(ctx context.Context, name string) (types.Resource, error) {
		return clt.GetSAMLIdPServiceProvider(ctx, name)
	}

	if err := web.CheckResourceUpsert(ctx, httpMethod, params, req.Name, get); err != nil {
		return nil, trace.Wrap(err)
	}

	sp, err := services.GenerateIdPServiceProviderFromFields(req.Name, req.EntityDescriptor)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := clt.CreateSAMLIdPServiceProvider(ctx, sp); err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.NewResourceItem(sp)
}

type resourcesAPIGetter interface {
	// GetGithubConnectors returns all configured Github connectors
	GetGithubConnectors(ctx context.Context, withSecrets bool) ([]types.GithubConnector, error)
	// GetSAMLConnector returns SAML connector information by id
	GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error)
	// GetSAMLConnectors gets SAML connectors list
	GetSAMLConnectors(ctx context.Context, withSecrets bool) ([]types.SAMLConnector, error)
	// GetOIDCConnector returns OIDC connector information by id
	GetOIDCConnector(ctx context.Context, id string, withSecrets bool) (types.OIDCConnector, error)
	// GetOIDCConnectors gets OIDC connectors list
	GetOIDCConnectors(ctx context.Context, withSecrets bool) ([]types.OIDCConnector, error)
	// CreateSAMLIdPServiceProvider creates a new SAML IdP service provider resource.
	CreateSAMLIdPServiceProvider(ctx context.Context, sp types.SAMLIdPServiceProvider) error
	// GetSAMLIdPServiceProvider returns the specified SAML IdP service provider resources.
	GetSAMLIdPServiceProvider(ctx context.Context, name string) (types.SAMLIdPServiceProvider, error)
}
