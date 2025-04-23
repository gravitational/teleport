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
	"net/http"

	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/types"
)

// jwksOkta returns public keys used to verify JWT tokens signed for use with Okta API Service App
// machine-to-machine authentication.
// https://developer.okta.com/docs/guides/implement-oauth-for-okta-serviceapp/main/
func (h *Handler) jwksOkta(_ http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	return h.jwks(r.Context(), types.OktaCA, false /* includeBlankKeyID */)
}
