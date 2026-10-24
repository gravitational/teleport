// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
)

type ListAssignedScopesResponse struct {
	Scopes        []string `json:"scopes"`
	NextPageToken string   `json:"next_page_token,omitempty"`
}

// listAssignedScopes returns a list of current user's assigned scopes in
// alphabetical order.
func (h *Handler) listAssignedScopes(
	w http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *SessionContext,
) (any, error) {
	if err := h.cfg.ScopesFeatures.AssertEnabled(); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	query := r.URL.Query()
	limit, err := QueryLimitAsInt32(query, "limit", 100)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lsRes, err := clt.ScopedAccessServiceClient().ListAssignedScopes(r.Context(), scopedaccessv1.ListAssignedScopesRequest_builder{
		PageToken: query.Get("page_token"),
		PageSize:  limit,
		Prefix:    query.Get("prefix"),
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ListAssignedScopesResponse{
		Scopes:        lsRes.GetScopes(),
		NextPageToken: lsRes.GetNextPageToken(),
	}, nil
}
