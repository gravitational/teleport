package web

import (
	"context"
	"net/http"

	"github.com/gravitational/teleport/api/types"
	enterpriseui "github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/teleport/lib/web/ui"

	"github.com/gravitational/trace"

	"github.com/julienschmidt/httprouter"
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

func (p *Plugin) upsertSAMLConnectorHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req ui.ResourceItem
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	return upsertSAMLConnector(r.Context(), clt, req.Content, r.Method)
}

func upsertSAMLConnector(ctx context.Context, clt resourcesAPIGetter, content, httpMethod string) (*ui.ResourceItem, error) {
	extractedRes, err := web.ExtractResourceAndValidate(content)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if extractedRes.Kind != types.KindSAMLConnector {
		return nil, trace.BadParameter("resource kind %q is invalid", extractedRes.Kind)
	}

	_, err = clt.GetSAMLConnector(ctx, extractedRes.Metadata.Name, false)
	if err := web.CheckResourceUpsertableByError(err, httpMethod, extractedRes.Metadata.Name); err != nil {
		return nil, trace.Wrap(err)
	}

	conn, err := services.UnmarshalSAMLConnector(extractedRes.Raw)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := clt.UpsertSAMLConnector(ctx, conn); err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.NewResourceItem(conn)
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

func (p *Plugin) upsertOIDCConnectorHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req ui.ResourceItem
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	return upsertOIDCConnector(r.Context(), clt, req.Content, r.Method)
}

func upsertOIDCConnector(ctx context.Context, clt resourcesAPIGetter, content, httpMethod string) (*ui.ResourceItem, error) {
	extractedRes, err := web.ExtractResourceAndValidate(content)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if extractedRes.Kind != types.KindOIDCConnector {
		return nil, trace.BadParameter("resource kind %q is invalid", extractedRes.Kind)
	}

	_, err = clt.GetOIDCConnector(ctx, extractedRes.Metadata.Name, false)
	if err := web.CheckResourceUpsertableByError(err, httpMethod, extractedRes.Metadata.Name); err != nil {
		return nil, trace.Wrap(err)
	}

	conn, err := services.UnmarshalOIDCConnector(extractedRes.Raw)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := clt.UpsertOIDCConnector(ctx, conn); err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.NewResourceItem(conn)
}

type resourcesAPIGetter interface {
	// GetGithubConnectors returns all configured Github connectors
	GetGithubConnectors(ctx context.Context, withSecrets bool) ([]types.GithubConnector, error)
	// UpsertSAMLConnector updates or creates SAML connector
	UpsertSAMLConnector(ctx context.Context, connector types.SAMLConnector) error
	// GetSAMLConnector returns SAML connector information by id
	GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error)
	// GetSAMLConnectors gets SAML connectors list
	GetSAMLConnectors(ctx context.Context, withSecrets bool) ([]types.SAMLConnector, error)
	// UpsertOIDCConnector updates or creates OIDC connector
	UpsertOIDCConnector(ctx context.Context, connector types.OIDCConnector) error
	// GetOIDCConnector returns OIDC connector information by id
	GetOIDCConnector(ctx context.Context, id string, withSecrets bool) (types.OIDCConnector, error)
	// GetOIDCConnectors gets OIDC connectors list
	GetOIDCConnectors(ctx context.Context, withSecrets bool) ([]types.OIDCConnector, error)
}
