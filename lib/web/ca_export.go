// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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
	"net/http"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/lib/client"
)

// authExportPublic returns the CA Certs that can be used to set up a chain of trust which includes the current Teleport Cluster
//
// GET /webapi/sites/:site/auth/export?type=<auth type>
// GET /webapi/auth/export?type=<auth type>
func (h *Handler) authExportPublic(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	err := rateLimitRequest(r, h.limiter)
	if err != nil {
		http.Error(w, err.Error(), trace.ErrorToCode(err))
		return
	}
	authorities, err := client.ExportAuthorities(
		r.Context(),
		h.GetProxyClient(),
		client.ExportAuthoritiesRequest{
			AuthType: r.URL.Query().Get("type"),
		},
	)
	if err != nil {
		h.logger.DebugContext(r.Context(), "Failed to generate CA Certs", "error", err)
		http.Error(w, err.Error(), trace.ErrorToCode(err))
		return
	}

	reader := strings.NewReader(authorities)

	// ServeContent sets the correct headers: Content-Type, Content-Length and Accept-Ranges.
	// It also handles the Range negotiation
	http.ServeContent(w, r, "authorized_hosts.txt", time.Now(), reader)
}
