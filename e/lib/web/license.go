package web

import (
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	eauth "github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/auth"
)

// getLicenseCheckStatusHandle is GET handle that returns the license check status
func (p *Plugin) getLicenseCheckStatusHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	proxyClient := p.h.GetProxyClient()
	client, ok := proxyClient.(*auth.Client)
	if !ok {
		return nil, trace.BadParameter("expected *auth.Client, got: %T", client)
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
