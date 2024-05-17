package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/safetext/shsprintf"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/utils/oidc"
	"github.com/gravitational/teleport/lib/web/scripts/oneoff"
)

// azureOIDCConfigureIdP returns a script that configures Azure OIDC Integration
// by creating an Enterprise Application in the Azure account
// with Teleport OIDC as a trusted credential issuer.
func (h *Handler) azureOIDCConfigure(w http.ResponseWriter, r *http.Request, p httprouter.Params) (any, error) {
	queryParams := r.URL.Query()

	oidcIssuer, err := oidc.IssuerFromPublicAddress(h.cfg.PublicProxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authConnectorName := queryParams.Get("authConnectorName")
	if authConnectorName == "" {
		return nil, trace.BadParameter("authConnectorName must be specified")
	}

	// The script must execute the following command:
	argsList := []string{
		"integration", "configure", "azure-oidc",
		fmt.Sprintf("--proxy-public-addr=%s", shsprintf.EscapeDefaultContext(oidcIssuer)),
		fmt.Sprintf("--auth-connector-name=%s", shsprintf.EscapeDefaultContext(authConnectorName)),
	}

	if tagParam := queryParams.Get("accessGraph"); tagParam != "" {
		argsList = append(argsList, "--access-graph")
	}

	script, err := oneoff.BuildScript(oneoff.OneOffScriptParams{
		TeleportArgs:   strings.Join(argsList, " "),
		SuccessMessage: "Success! You can now go back to the browser to use the integration with Azure.",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httplib.SetScriptHeaders(w.Header())
	_, err = fmt.Fprint(w, script)

	return nil, trace.Wrap(err)
}
