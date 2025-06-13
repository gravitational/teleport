package web

import (
	"context"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	enterpriseui "github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/teleport/lib/web/ui"
)

func (p *Plugin) getAuthConnectorsHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connectors, err := getAuthConnectors(r.Context(), clt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	defaultConnectorName, defaultConnectorType, err := web.ProcessDefaultConnector(r.Context(), clt, connectors)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ui.ListAuthConnectorsResponse{
		DefaultConnectorName: defaultConnectorName,
		DefaultConnectorType: defaultConnectorType,
		Connectors:           connectors,
	}, nil
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

// getSAMLConnectorHandle returns a SAML connector by name.
func (p *Plugin) getSAMLConnectorHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connector, err := clt.GetSAMLConnector(r.Context(), params.ByName("name"), true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.NewResourceItem(connector)
}

// getOIDCConnectorHandle returns an OIDC connector by name.
func (p *Plugin) getOIDCConnectorHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connector, err := clt.GetOIDCConnector(r.Context(), params.ByName("name"), true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.NewResourceItem(connector)
}

func (p *Plugin) deleteSAMLConnectorHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connectorName := params.ByName("name")
	if err := clt.DeleteSAMLConnector(r.Context(), connectorName); err != nil {
		return nil, trace.Wrap(err)
	}

	authPref, err := clt.GetAuthPreference(r.Context())
	if err != nil {
		return nil, trace.Wrap(err, "failed to get auth preference")
	}

	defaultConnectorName := authPref.GetConnectorName()
	defaultConnectorType := authPref.GetType()
	// If the connector being deleted is the default, have the auth preference fallback to another connector.
	if defaultConnectorType == constants.SAML && defaultConnectorName == connectorName {
		connectors, err := getAuthConnectors(r.Context(), clt)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		_, _, err = web.ProcessDefaultConnector(r.Context(), clt, connectors)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return web.OK(), nil
}

func (p *Plugin) createSAMLConnectorHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := web.CreateResource(r, types.KindSAMLConnector, services.UnmarshalSAMLConnector, clt.CreateSAMLConnector)
	return item, trace.Wrap(err)
}

func (p *Plugin) updateSAMLConnectorHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := web.UpdateResource(r, params, types.KindSAMLConnector, services.UnmarshalSAMLConnector, clt.UpdateSAMLConnector)
	return item, trace.Wrap(err)
}

func (p *Plugin) deleteOIDCConnectorHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connectorName := params.ByName("name")
	if err := clt.DeleteOIDCConnector(r.Context(), connectorName); err != nil {
		return nil, trace.Wrap(err)
	}

	authPref, err := clt.GetAuthPreference(r.Context())
	if err != nil {
		return nil, trace.Wrap(err, "failed to get auth preference")
	}

	defaultConnectorName := authPref.GetConnectorName()
	defaultConnectorType := authPref.GetType()
	// If the connector being deleted is the default, have the auth preference fallback to another connector.
	if defaultConnectorType == constants.OIDC && defaultConnectorName == connectorName {
		connectors, err := getAuthConnectors(r.Context(), clt)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		_, _, err = web.ProcessDefaultConnector(r.Context(), clt, connectors)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return web.OK(), nil
}

func (p *Plugin) createOIDCConnectorHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := web.CreateResource(r, types.KindOIDC, services.UnmarshalOIDCConnector, clt.CreateOIDCConnector)
	return item, trace.Wrap(err)
}

func (p *Plugin) updateOIDCConnectorHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := web.UpdateResource(r, params, types.KindOIDC, services.UnmarshalOIDCConnector, clt.UpdateOIDCConnector)
	return item, trace.Wrap(err)
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
}
