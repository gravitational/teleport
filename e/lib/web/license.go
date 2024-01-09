package web

import (
	"net/http"

	"github.com/gravitational/license/generate"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/web"
)

// getLicenseCheckStatusHandle is GET handle that returns the license check status
// TODO(espadolini): delete once we're sure that the web UI doesn't use this
func (p *Plugin) getLicenseCheckStatusHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	return (*struct{})(nil), nil
}

// getLicense is GET handle that returns the license
func (p *Plugin) getLicense(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pem, err := clt.GetLicense(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// we generate a unique anonymization key each time
	// a license is requested. It's up to the customer to
	// make sure to download the license once and use the
	// same license across all of their clusters in order
	// for us to properly monitor their usage.
	//
	// This anonymization key is never retained by Teleport,
	// which prevents us from being able to deanonymize any
	// data.
	appendedPem, err := generate.AppendAnonymizationKey([]byte(pem))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	licensefile, err := licensefile.FromPEM(appendedPem)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.GetLicenseResponse{
		PEM:    pem,
		Expiry: licensefile.License.Expiry(),
	}, err
}
