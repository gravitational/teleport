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

	"github.com/gravitational/teleport/lib/httplib"
	samlidpscripts "github.com/gravitational/teleport/lib/web/scripts/samlidp"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

func (h *Handler) gcpWIFConfigurationScript(w http.ResponseWriter, r *http.Request, p httprouter.Params) (any, error) {
	queryParams := r.URL.Query()
	orgId := queryParams.Get("orgId")
	poolName := queryParams.Get("poolName")
	poolProviderName := queryParams.Get("poolProviderName")
	metadataEndpoint := fmt.Sprintf("https://%s/enterprise/saml-idp/metadata", h.PublicProxyAddr())
	script, err := samlidpscripts.BuildScript(samlidpscripts.GCPWorkforceConfigParams{
		OrgID:            orgId,
		PoolName:         poolName,
		PoolProviderName: poolProviderName,
		MetadataEndpoint: metadataEndpoint,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httplib.SetScriptHeaders(w.Header())
	_, err = fmt.Fprint(w, script)

	return nil, trace.Wrap(err)
}
