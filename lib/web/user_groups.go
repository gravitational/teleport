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
	"sort"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/web/ui"
)

func (h *Handler) getUserGroups(_ http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	// Get a client to the Auth Server with the logged in user's identity. The
	// identity of the logged in user is used to fetch the list of nodes.
	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req, err := convertListResourcesRequest(r, types.KindUserGroup)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	page, err := apiclient.GetResourcePage[types.UserGroup](r.Context(), clt, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	appServers, err := apiclient.GetAllResources[types.AppServer](r.Context(), clt, &proto.ListResourcesRequest{
		ResourceType:     types.KindAppServer,
		Namespace:        apidefaults.Namespace,
		UseSearchAsRoles: true,
	})
	if err != nil {
		h.logger.DebugContext(r.Context(), "Unable to fetch applications while listing user groups, unable to display associated applications", "error", err)
	}

	appServerLookup := make(map[string]types.AppServer, len(appServers))
	for _, appServer := range appServers {
		appServerLookup[appServer.GetApp().GetName()] = appServer
	}

	userGroupsToApps := map[string]types.Apps{}
	for _, userGroup := range page.Resources {
		apps := make(types.Apps, 0, len(userGroup.GetApplications()))
		for _, appName := range userGroup.GetApplications() {
			app := appServerLookup[appName]
			if app == nil {
				h.logger.DebugContext(r.Context(), "Unable to find application when creating user groups, skipping", "app", appName)
				continue
			}
			apps = append(apps, app.GetApp())
		}
		sort.Sort(apps)
		userGroupsToApps[userGroup.GetName()] = apps
	}

	userGroups, err := ui.MakeUserGroups(page.Resources, userGroupsToApps)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return listResourcesGetResponse{
		Items:      userGroups,
		StartKey:   page.NextKey,
		TotalCount: page.Total,
	}, nil
}
