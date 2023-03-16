package web

import (
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/web"
)

func (p *Plugin) getPluginsHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	const pageSize = apidefaults.DefaultChunkSize
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, err
	}

	pluginsClt := clt.PluginsClient()

	// TODO(justinas): actually paginate
	results, err := pluginsClt.ListPlugins(r.Context(), &pluginspb.ListPluginsRequest{PageSize: pageSize})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Avoid returning nil/null to the UI when there are no plugins, make an empty slice.
	plugins := make([]*ui.Plugin, 0)

	for _, p := range results.Plugins {
		plugin, err := ui.NewPlugin(p)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		plugins = append(plugins, plugin)
	}
	return plugins, nil
}

func (p *Plugin) deletePluginHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, err
	}

	pluginName := params.ByName("name")
	if pluginName == "" {
		return nil, trace.BadParameter("name must be specified")
	}

	pluginsClt := clt.PluginsClient()

	_, err = pluginsClt.DeletePlugin(r.Context(), &pluginspb.DeletePluginRequest{Name: pluginName})

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return web.OK(), nil
}
