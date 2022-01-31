/**
 * Copyright 2021 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package web

import (
	"net/http"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/web/ui"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

// clusterKubesGet returns a list of kube clusters in a form the UI can present.
func (h *Handler) clusterKubesGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resources, nextKey, err := listResources(clt, r, types.KindKubeService)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	kubeServers, err := types.ResourcesWithLabels(resources).AsServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return listResourcesGetResponse{
		Items:    ui.MakeKubes(h.auth.clusterName, kubeServers),
		StartKey: nextKey,
	}, nil
}

// clusterDatabasesGet returns a list of db servers in a form the UI can present.
func (h *Handler) clusterDatabasesGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resources, nextKey, err := listResources(clt, r, types.KindDatabaseServer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err := types.ResourcesWithLabels(resources).AsDatabaseServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Make a list of all proxied databases.
	var databases []types.Database
	for _, server := range servers {
		databases = append(databases, server.GetDatabase())
	}

	return listResourcesGetResponse{
		Items:    ui.MakeDatabases(h.auth.clusterName, types.DeduplicateDatabases(databases)),
		StartKey: nextKey,
	}, nil
}

// clusterDesktopsGet returns a list of desktops in a form the UI can present.
func (h *Handler) clusterDesktopsGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	windowsDesktops, err := clt.GetWindowsDesktops(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeDesktops(windowsDesktops), nil
}

// getDesktopHandle returns a desktop.
func (h *Handler) getDesktopHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	desktopName := p.ByName("desktopName")

	windowsDesktop, err := clt.GetWindowsDesktop(r.Context(), desktopName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeDesktop(windowsDesktop), nil
}
