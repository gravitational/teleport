/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/safetext/shsprintf"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/integrations/samlidp/samlidpconfig"
	"github.com/gravitational/teleport/lib/web/scripts/oneoff"
)

func (h *Handler) gcpWorkforceConfigScript(w http.ResponseWriter, r *http.Request, p httprouter.Params) (any, error) {
	queryParams := r.URL.Query()

	samlIdPMetadataURL := fmt.Sprintf("https://%s/enterprise/saml-idp/metadata", h.PublicProxyAddr())
	// validate queryParams params
	if err := (samlidpconfig.GCPWorkforceAPIParams{
		OrganizationID:     queryParams.Get("orgId"),
		PoolName:           queryParams.Get("poolName"),
		PoolProviderName:   queryParams.Get("poolProviderName"),
		SAMLIdPMetadataURL: samlIdPMetadataURL,
	}).CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// The script must execute the following command:
	// teleport integration configure samlidp gcp-workforce
	argsList := []string{
		"integration", "configure", "samlidp", "gcp-workforce",
		fmt.Sprintf("--org-id=%s", shsprintf.EscapeDefaultContext(queryParams.Get("orgId"))),
		fmt.Sprintf("--pool-name=%s", shsprintf.EscapeDefaultContext(queryParams.Get("poolName"))),
		fmt.Sprintf("--pool-provider-name=%s", shsprintf.EscapeDefaultContext(queryParams.Get("poolProviderName"))),
		fmt.Sprintf("--idp-metadata-url=%s", shsprintf.EscapeDefaultContext(samlIdPMetadataURL)),
	}
	script, err := oneoff.BuildScript(oneoff.OneOffScriptParams{
		TeleportArgs:   strings.Join(argsList, " "),
		SuccessMessage: "Success! You can now go back to the browser to complete enrolling this workforce pool to Teleport SAML Identity Provider.",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httplib.SetScriptHeaders(w.Header())
	_, err = fmt.Fprint(w, script)

	return nil, trace.Wrap(err)
}
