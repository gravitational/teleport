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

// Package web implements web proxy handler that provides
// web interface to view and connect to teleport nodes
package web

import (
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/web/ui"
)

// clusterOktaAppsGet returns a list of Okta applications in a form the UI can present.
func (h *Handler) clusterOktaAppsGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	// Get a list of application servers and their proxied apps.
	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := listResources(clt, r, types.KindOktaApps)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	oktaApps, err := types.ResourcesAsType[types.OktaApplication](types.ResourcesWithLabels(resp.Resources))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiOktaApps := ui.MakeOktaApps(oktaApps)

	return listResourcesGetResponse{
		Items:      uiOktaApps,
		StartKey:   resp.NextKey,
		TotalCount: len(uiOktaApps),
	}, nil
}

// clusterOktaGroupsGet returns a list of Okta groups in a form the UI can present.
func (h *Handler) clusterOktaGroupsGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	// Get a list of application servers and their proxied apps.
	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := listResources(clt, r, types.KindOktaGroups)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	oktaGroups, err := types.ResourcesAsType[types.OktaGroup](types.ResourcesWithLabels(resp.Resources))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiOktaGroups := ui.MakeOktaGroup(oktaGroups)

	return listResourcesGetResponse{
		Items:      uiOktaGroups,
		StartKey:   resp.NextKey,
		TotalCount: len(uiOktaGroups),
	}, nil
}
