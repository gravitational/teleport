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
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/ui"
	webui "github.com/gravitational/teleport/lib/web/ui"
)

// clusterKubesGet returns a list of kube clusters in a form the UI can present.
func (h *Handler) clusterKubesGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	clt, err := sctx.GetUserClient(r.Context(), site)
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
func (h *Handler) clusterKubeResourcesGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
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

	resp, err := listKubeResources(r.Context(), clt, r.URL.Query(), site.GetName(), kind)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return listResourcesGetResponse{
		Items:      webui.MakeKubeResources(resp.Resources, kubeCluster),
		StartKey:   resp.NextKey,
		TotalCount: int(resp.TotalCount),
	}, nil
}

// clusterDatabasesGet returns a list of db servers in a form the UI can present.
func (h *Handler) clusterDatabasesGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	clt, err := sctx.GetUserClient(r.Context(), site)
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
func (h *Handler) clusterDatabaseGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	databaseName := p.ByName("database")
	if databaseName == "" {
		return nil, trace.BadParameter("database name is required")
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
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
func (h *Handler) clusterDatabaseServicesList(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	clt, err := ctx.GetUserClient(r.Context(), site)
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
func (h *Handler) clusterDatabaseServersList(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	clt, err := ctx.GetUserClient(r.Context(), site)
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
func (h *Handler) clusterDesktopsGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	clt, err := sctx.GetUserClient(r.Context(), site)
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
		desktop, ok := r.ResourceWithLabels.(types.WindowsDesktop)
		if !ok {
			continue
		}

		uiDesktops = append(uiDesktops, webui.MakeDesktop(desktop, r.Logins, false /* requiresRequest */))
	}

	return listResourcesGetResponse{
		Items:      uiDesktops,
		StartKey:   page.NextKey,
		TotalCount: page.Total,
	}, nil
}

// clusterDesktopServicesGet returns a list of desktop services in a form the UI can present.
func (h *Handler) clusterDesktopServicesGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	// Get a client to the Auth Server with the logged in user's identity. The
	// identity of the logged in user is used to fetch the list of desktop services.
	clt, err := sctx.GetUserClient(r.Context(), site)
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
func (h *Handler) getDesktopHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	clt, err := sctx.GetUserClient(r.Context(), site)
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

	return webui.MakeDesktop(desktop, logins, false /* requiresRequest */), nil
}

// desktopIsActive checks if a desktop has an active session and returns a desktopIsActive.
//
// GET /v1/webapi/sites/:site/desktops/:desktopName/active
//
// Response body:
//
// {"active": bool}
func (h *Handler) desktopIsActive(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
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

	clt, err := sctx.GetUserClient(r.Context(), site)
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

// createNodeRequest contains the required information to create a Node.
type createNodeRequest struct {
	Name     string             `json:"name,omitempty"`
	SubKind  string             `json:"subKind,omitempty"`
	Hostname string             `json:"hostname,omitempty"`
	Addr     string             `json:"addr,omitempty"`
	Labels   []ui.Label         `json:"labels,omitempty"`
	AWSInfo  *webui.AWSMetadata `json:"aws,omitempty"`
}

func (r *createNodeRequest) checkAndSetDefaults() error {
	if r.Name == "" {
		return trace.BadParameter("missing node name")
	}

	// Nodes provided by the Teleport Agent are not meant to be created by the user.
	// They connect to the cluster and heartbeat their information.
	//
	// Agentless Nodes with Teleport CA call the Teleport Proxy and upsert themselves,
	// so they are also not meant to be added from web api.
	if r.SubKind != types.SubKindOpenSSHEICENode {
		return trace.BadParameter("invalid subkind %q, only %q is supported", r.SubKind, types.SubKindOpenSSHEICENode)
	}

	if r.Hostname == "" {
		return trace.BadParameter("missing node hostname")
	}

	if r.Addr == "" {
		return trace.BadParameter("missing node addr")
	}

	return nil
}

// handleNodeCreate creates a Teleport Node.
func (h *Handler) handleNodeCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	var req *createNodeRequest
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	labels := make(map[string]string, len(req.Labels))
	for _, label := range req.Labels {
		labels[label.Name] = label.Value
	}

	server, err := types.NewNode(
		req.Name,
		req.SubKind,
		types.ServerSpecV2{
			Hostname: req.Hostname,
			Addr:     req.Addr,
			CloudMetadata: &types.CloudMetadata{
				AWS: &types.AWSInfo{
					AccountID:   req.AWSInfo.AccountID,
					InstanceID:  req.AWSInfo.InstanceID,
					Region:      req.AWSInfo.Region,
					VPCID:       req.AWSInfo.VPCID,
					Integration: req.AWSInfo.Integration,
					SubnetID:    req.AWSInfo.SubnetID,
				},
			},
		},
		labels,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := clt.UpsertNode(r.Context(), server); err != nil {
		return nil, trace.Wrap(err)
	}

	accessChecker, err := sctx.GetUserAccessChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	logins, err := accessChecker.GetAllowedLoginsForResource(server)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return webui.MakeServer(site.GetName(), server, logins, false /* requiresRequest */), nil
}
