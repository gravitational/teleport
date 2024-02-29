// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
