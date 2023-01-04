package web

import (
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	eauth "github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/web"
)

// getLicenseCheckStatusHandle is GET handle that returns the license check status
func (p *Plugin) getLicenseCheckStatusHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	client, err := p.getAuthClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proClient, err := eauth.NewProClient(client)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	licenseCheckResult, err := proClient.GetLicenseCheckResult(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.NewLicenseCheckStatus(licenseCheckResult), nil
}

func (p *Plugin) getLicense(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, err
	}

	return clt.GetLicense(r.Context())
}
