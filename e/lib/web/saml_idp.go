package web

import (
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	samlidpui "github.com/gravitational/teleport/e/lib/web/ui/samlidp"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/web"
)

func (p *Plugin) getSAMLIdPServiceProviderHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	authClt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	appName := params.ByName("name")
	sp, err := authClt.GetSAMLIdPServiceProvider(r.Context(), appName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sp, nil
}

func (p *Plugin) createSAMLIdPServiceProviderHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	authClt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req samlidpui.CreateSAMLIdPServiceProviderRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	spV1, err := samlidpui.TransformToProtoType(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authClt.CreateSAMLIdPServiceProvider(r.Context(), spV1); err != nil {
		return nil, trace.Wrap(err)
	}

	return spV1, nil
}

func (p *Plugin) updateSAMLIdPServiceProviderHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	authClt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req samlidpui.CreateSAMLIdPServiceProviderRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	appName := params.ByName("name")
	if appName != req.Name {
		return nil, trace.BadParameter("resource renaming is not supported, please create a different resource and then delete this one")
	}

	spV1, err := samlidpui.TransformToProtoType(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authClt.UpdateSAMLIdPServiceProvider(r.Context(), spV1); err != nil {
		return nil, trace.Wrap(err)
	}

	return spV1, nil
}

func (p *Plugin) deleteSAMLIdPServiceProviderHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	authClt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	appName := params.ByName("name")
	if err := authClt.DeleteSAMLIdPServiceProvider(r.Context(), appName); err != nil {
		return nil, trace.Wrap(err)
	}

	return web.OK(), nil
}
