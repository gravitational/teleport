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
		h.log.Debugf("Unable to fetch applications while listing user groups, unable to display associated applications: %v", err)
	}

	appServerLookup := make(map[string]types.AppServer, len(appServers))
	for _, appServer := range appServers {
		appServerLookup[appServer.GetName()] = appServer
	}

	userGroupsToApps := map[string]types.Apps{}
	for _, userGroup := range page.Resources {
		apps := make(types.Apps, 0, len(userGroup.GetApplications()))
		for _, appName := range userGroup.GetApplications() {
			app := appServerLookup[appName]
			if app == nil {
				h.log.Debugf("Unable to find application %s when creating user groups, skipping", appName)
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
