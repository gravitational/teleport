/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/web/ui"
)

func (h *Handler) gitServerCreateOrUpsert(_ http.ResponseWriter, r *http.Request, _ httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	var req *ui.CreateGitServerRequest
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Only GitHub server is supported. Above req.Check() performs necessary
	// checks to ensure all the fields are set.
	gitServer, err := types.NewGitHubServerWithName(req.Name, types.GitHubServerMetadata{
		Organization: req.GitHub.Organization,
		Integration:  req.GitHub.Integration,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userClient, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	gitServiceClient := userClient.GitServerClient()

	if req.Overwrite {
		upserted, err := gitServiceClient.UpsertGitServer(r.Context(), gitServer)
		return upserted, trace.Wrap(err)
	}

	created, err := gitServiceClient.CreateGitServer(r.Context(), gitServer)
	return created, trace.Wrap(err)
}

func (h *Handler) gitServerGet(_ http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	name := p.ByName("name")
	if name == "" {
		return nil, trace.BadParameter("git server name is required")
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	gitServer, err := clt.GitServerClient().GetGitServer(r.Context(), name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ui.MakeGitServer(site.GetName(), gitServer, false), nil
}

func (h *Handler) gitServerDelete(_ http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	name := p.ByName("name")
	if name == "" {
		return nil, trace.BadParameter("git server name is required")
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := clt.GitServerClient().DeleteGitServer(r.Context(), name); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}
