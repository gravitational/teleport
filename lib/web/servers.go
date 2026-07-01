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
	"strings"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/client"
	linuxdesktopv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	webui "github.com/gravitational/teleport/lib/web/ui"
)

// clusterKubesGet returns a list of kube clusters in a form the UI can present.
func (h *Handler) clusterKubesGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req, err := convertListResourcesRequest(r, types.KindKubernetesCluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	page, err := client.GetResourcePage[types.KubeCluster](r.Context(), clt, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessChecker, err := sctx.GetUserAccessChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return listResourcesGetResponse{
		Items:      webui.MakeKubeClusters(page.Resources, accessChecker),
		StartKey:   page.NextKey,
		TotalCount: page.Total,
	}, nil
}

// clusterKubeResourcesGet returns supported requested kubernetes subresources eg: pods, namespaces, secrets etc.
func (h *Handler) clusterKubeResourcesGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	kind := r.URL.Query().Get("kind")
	kubeCluster := r.URL.Query().Get("kubeCluster")

	if kubeCluster == "" {
		return nil, trace.BadParameter("missing param %q", "kubeCluster")
	}

	if kind == "" {
		return nil, trace.BadParameter("missing param %q", "kind")
	}

	if !slices.Contains(types.KubernetesResourcesKinds, kind) && !strings.HasPrefix(kind, types.AccessRequestPrefixKindKube) {
		return nil, trace.BadParameter("kind is not valid, valid kinds %v %s<kind>", types.KubernetesResourcesKinds, types.AccessRequestPrefixKindKube)
	}

	clt, err := sctx.NewKubernetesServiceClient(r.Context(), h.cfg.ProxyWebAddr.Addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := listKubeResources(r.Context(), clt, r.URL.Query(), cluster.GetName(), kind)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return listResourcesGetResponse{
		Items:      webui.MakeKubeResources(resp.GetResources(), kubeCluster),
		StartKey:   resp.GetNextKey(),
		TotalCount: int(resp.GetTotalCount()),
	}, nil
}

// clusterKubeServersList returns a list of kube servers in a form the UI can present.
func (h *Handler) clusterKubeServersList(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := ctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req, err := convertListResourcesRequest(r, types.KindKubeServer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	page, err := client.GetResourcePage[types.KubeServer](r.Context(), clt, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return listResourcesGetResponse{
		Items:    page.Resources,
		StartKey: page.NextKey,
	}, nil
}

// clusterDatabasesGet returns a list of db servers in a form the UI can present.
func (h *Handler) clusterDatabasesGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req, err := convertListResourcesRequest(r, types.KindDatabaseServer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	page, err := client.GetResourcePage[types.DatabaseServer](r.Context(), clt, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessChecker, err := sctx.GetUserAccessChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiItems := make([]webui.Database, 0, len(page.Resources))
	for _, dbServer := range page.Resources {
		db := webui.MakeDatabaseFromDatabaseServer(dbServer, accessChecker, h.cfg.DatabaseREPLRegistry, false /* requires reset*/)
		uiItems = append(uiItems, db)
	}

	return listResourcesGetResponse{
		Items:      uiItems,
		StartKey:   page.NextKey,
		TotalCount: page.Total,
	}, nil
}

// clusterDatabaseGet returns a database in a form the UI can present.
func (h *Handler) clusterDatabaseGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	databaseName := p.ByName("database")
	if databaseName == "" {
		return nil, trace.BadParameter("database name is required")
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dbServers, err := fetchDatabaseServersWithName(r.Context(), clt, r, databaseName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	aggregateStatus := types.AggregateHealthStatus(func(yield func(types.TargetHealthStatus) bool) {
		for _, srv := range dbServers {
			if !yield(srv.GetTargetHealthStatus()) {
				return
			}
		}
	})
	dbServers[0].SetTargetHealthStatus(aggregateStatus)

	accessChecker, err := sctx.GetUserAccessChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return webui.MakeDatabaseFromDatabaseServer(
		dbServers[0],
		accessChecker,
		h.cfg.DatabaseREPLRegistry,
		false, /* requiresRequest */
	), nil
}

// clusterDatabaseServicesList returns a list of DatabaseServices (database agents) in a form the UI can present.
func (h *Handler) clusterDatabaseServicesList(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := ctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req, err := convertListResourcesRequest(r, types.KindDatabaseService)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	page, err := client.GetResourcePage[types.DatabaseService](r.Context(), clt, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return listResourcesGetResponse{
		Items:      webui.MakeDatabaseServices(page.Resources),
		StartKey:   page.NextKey,
		TotalCount: page.Total,
	}, nil
}

// clusterDatabaseServersList returns a list of database servers in a form the UI can present.
func (h *Handler) clusterDatabaseServersList(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := ctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req, err := convertListResourcesRequest(r, types.KindDatabaseServer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	page, err := client.GetResourcePage[types.DatabaseServer](r.Context(), clt, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return listResourcesGetResponse{
		Items:    page.Resources,
		StartKey: page.NextKey,
	}, nil
}

// clusterDesktopsGet returns a list of desktops in a form the UI can present.
func (h *Handler) clusterDesktopsGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req, err := convertListResourcesRequest(r, types.KindWindowsDesktop)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	page, err := client.GetEnrichedResourcePage(r.Context(), clt, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiDesktops := make([]webui.Desktop, 0, len(page.Resources))
	for _, r := range page.Resources {
		switch desktop := r.ResourceWithLabels.(type) {
		case types.WindowsDesktop:
			uiDesktops = append(uiDesktops, webui.MakeWindowsDesktop(desktop, r.Logins, false /* requiresRequest */))
		case types.Resource153UnwrapperT[*linuxdesktopv1.LinuxDesktop]:
			uiDesktops = append(uiDesktops, webui.MakeLinuxDesktop(desktop.UnwrapT(), r.Logins, false /* requiresRequest */))
		}
	}

	return listResourcesGetResponse{
		Items:      uiDesktops,
		StartKey:   page.NextKey,
		TotalCount: page.Total,
	}, nil
}

// clusterDesktopServicesGet returns a list of desktop services in a form the UI can present.
func (h *Handler) clusterDesktopServicesGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	// Get a client to the Auth Server with the logged in user's identity. The
	// identity of the logged in user is used to fetch the list of desktop services.
	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req, err := convertListResourcesRequest(r, types.KindWindowsDesktopService)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	page, err := client.GetResourcePage[types.WindowsDesktopService](r.Context(), clt, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return listResourcesGetResponse{
		Items:      webui.MakeDesktopServices(page.Resources),
		StartKey:   page.NextKey,
		TotalCount: page.Total,
	}, nil
}

// getDesktopHandle returns a desktop.
func (h *Handler) getDesktopHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	desktopName := p.ByName("desktopName")

	windowsDesktops, err := clt.GetWindowsDesktops(r.Context(), types.WindowsDesktopFilter{Name: desktopName})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(windowsDesktops) == 0 {
		return nil, trace.NotFound("expected at least 1 desktop, got 0")
	}

	accessChecker, err := sctx.GetUserAccessChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// windowsDesktops may contain the same desktop multiple times
	// if multiple Windows Desktop Services are in use. We only need
	// to see the desktop once in the UI, so just take the first one.
	desktop := windowsDesktops[0]

	logins, err := accessChecker.GetAllowedLoginsForResource(desktop)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return webui.MakeWindowsDesktop(desktop, logins, false /* requiresRequest */), nil
}

// desktopIsActive checks if a desktop has an active session and returns a desktopIsActive.
//
// GET /v1/webapi/sites/:site/desktops/:desktopName/active
//
// Response body:
//
// {"active": bool}
func (h *Handler) desktopIsActive(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	desktopName := p.ByName("desktopName")
	trackers, err := h.auth.proxyClient.GetActiveSessionTrackersWithFilter(r.Context(), &types.SessionTrackerFilter{
		Kind: string(types.WindowsDesktopSessionKind),
		State: &types.NullableSessionState{
			State: types.SessionState_SessionStateRunning,
		},
		DesktopName: desktopName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, tracker := range trackers {
		// clt is an auth.ClientI with the role of the user, so
		// clt.GetWindowsDesktops() can be used to confirm that
		// the user has access to the requested desktop.
		desktops, err := clt.GetWindowsDesktops(r.Context(),
			types.WindowsDesktopFilter{Name: tracker.GetDesktopName()})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if len(desktops) == 0 {
			// There are no active sessions for this desktop
			// or the user doesn't have access to it
			break
		} else {
			return desktopIsActive{true}, nil
		}
	}

	return desktopIsActive{false}, nil
}

type desktopIsActive struct {
	Active bool `json:"active"`
}
