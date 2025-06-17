// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

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
	ctx := r.Context()
	queryParams := r.URL.Query()

	oidcIssuer, err := oidc.IssuerFromPublicAddress(h.cfg.PublicProxyAddr, "")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authConnectorName := queryParams.Get("authConnectorName")
	if authConnectorName == "" {
		return nil, trace.BadParameter("authConnectorName must be specified")
	}
	// Ensure the auth connector name is valid
	const withSecrets = false
	_, err = h.GetProxyClient().GetSAMLConnector(ctx, authConnectorName, withSecrets)
	// NotFound error is ignored to prevent disclosure of whether the integration exists in a public/no-auth endpoint.
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
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
		EntrypointArgs: strings.Join(argsList, " "),
		SuccessMessage: "Success! You can now go back to the Teleport Web UI to use the integration with Azure.",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httplib.SetScriptHeaders(w.Header())
	_, err = w.Write([]byte(script))

	return nil, trace.Wrap(err)
}
