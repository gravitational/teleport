package web

import (
	"net/http"

	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/lib/web"
)

func (p *Plugin) getReleases(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, err
	}

	return clt.ListReleases(r.Context())
}
