/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"slices"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/connectmycomputer"
	"github.com/gravitational/teleport/lib/web/ui"
)

// connectMyComputerLoginsList is a handler for GET /webapi/connectmycomputer/logins.
func (h *Handler) connectMyComputerLoginsList(_ http.ResponseWriter, r *http.Request, _ httprouter.Params, sctx *SessionContext) (any, error) {
	connectMyComputerRoleName := connectmycomputer.GetRoleNameForUser(sctx.GetUser())

	identity, err := sctx.GetIdentity()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !slices.Contains(identity.Groups, connectMyComputerRoleName) {
		return nil, trace.NotFound("User %s does not have the %s role in the session cert.", sctx.GetUser(), connectMyComputerRoleName)
	}

	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	role, err := authClient.GetRole(r.Context(), connectMyComputerRoleName)
	if err != nil {
		// The user is always able to read roles that they hold, see lib/auth.ServerWithRoles.GetRole.
		// Because of this, we don't have to worry about access denied here.
		//
		// NotFound is also not a factor here. If the role exists in the cert but it has since been
		// removed from the cluster, the auth server will respond with access denied.
		return nil, trace.Wrap(err, "fetching %q role", connectMyComputerRoleName)
	}

	logins := role.GetLogins(types.Allow)

	return ui.ConnectMyComputerLoginsListResponse{
		Logins: logins,
	}, nil
}
