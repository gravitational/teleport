/*

 Copyright 2023 Gravitational, Inc.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.


*/

package web

import (
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/web/ui"
)

func (h *Handler) getUserGroups(_ http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *SessionContext, site reversetunnel.RemoteSite) (any, error) {
	// Get a client to the Auth Server with the logged in user's identity. The
	// identity of the logged in user is used to fetch the list of nodes.
	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	page, err := listResources(clt, r, types.KindUserGroup)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessChecker, err := sctx.GetUserAccessChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userGroups, err := types.ResourcesWithLabels(page.Resources).AsUserGroups()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiUserGroups, err := ui.MakeUserGroups(site.GetName(), userGroups, accessChecker.Roles())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return listResourcesGetResponse{
		Items:      uiUserGroups,
		StartKey:   page.NextKey,
		TotalCount: page.TotalCount,
	}, nil
}
