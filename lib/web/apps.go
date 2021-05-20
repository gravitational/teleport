/*
Copyright 2020 Gravitational, Inc.

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
	"context"
	"net/http"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web/app"
	"github.com/gravitational/teleport/lib/web/ui"

	"github.com/gravitational/trace"

	"github.com/julienschmidt/httprouter"
)

// clusterAppsGet returns a list of applications in a form the UI can present.
func (h *Handler) clusterAppsGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	appClusterName := p.ByName("site")

	// Get a list of application servers.
	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	appServers, err := clt.GetAppServers(r.Context(), defaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return makeResponse(ui.MakeApps(h.auth.clusterName, h.proxyDNSName(), appClusterName, appServers))
}

type GetAppFQDNRequest resolveAppParams

type GetAppFQDNResponse struct {
	// FQDN is application FQDN.
	FQDN string `json:"fqdn"`
}

type CreateAppSessionRequest resolveAppParams

type CreateAppSessionResponse struct {
	// CookieValue is the application session cookie value.
	CookieValue string `json:"value"`
	// FQDN is application FQDN.
	FQDN string `json:"fqdn"`
}

// getAppFQDN resolves the input params to a known application and returns
// its valid FQDN.
//
// GET /v1/webapi/apps/:fqdnHint/:clusterName/:publicAddr
func (h *Handler) getAppFQDN(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext) (interface{}, error) {
	req := GetAppFQDNRequest{
		FQDNHint:    p.ByName("fqdnHint"),
		ClusterName: p.ByName("clusterName"),
		PublicAddr:  p.ByName("publicAddr"),
	}

	// Get an auth client connected with the user's identity.
	authClient, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get a reverse tunnel proxy aware of the user's permissions.
	proxy, err := h.ProxyWithRoles(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Use the information the caller provided to attempt to resolve to an
	// application running within either the root or leaf cluster.
	result, err := h.resolveApp(r.Context(), authClient, proxy, resolveAppParams(req))
	if err != nil {
		return nil, trace.Wrap(err, "unable to resolve FQDN: %v", req.FQDNHint)
	}

	return &GetAppFQDNResponse{
		FQDN: result.FQDN,
	}, nil
}

// createAppSession creates a new application session.
//
// POST /v1/webapi/sessions/app
func (h *Handler) createAppSession(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext) (interface{}, error) {
	var req CreateAppSessionRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	// Get an auth client connected with the user's identity.
	authClient, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get a reverse tunnel proxy aware of the user's permissions.
	proxy, err := h.ProxyWithRoles(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Use the information the caller provided to attempt to resolve to an
	// application running within either the root or leaf cluster.
	result, err := h.resolveApp(r.Context(), authClient, proxy, resolveAppParams(req))
	if err != nil {
		return nil, trace.Wrap(err, "unable to resolve FQDN: %v", req.FQDNHint)
	}

	h.log.Debugf("Creating application web session for %v in %v.", result.PublicAddr, result.ClusterName)

	// Create an application web session.
	//
	// Application sessions should not last longer than the parent session.TTL
	// will be derived from the identity which has the same expiration as the
	// parent web session.
	//
	// PublicAddr and ClusterName will get encoded within the certificate and
	// used for request routing.
	ws, err := authClient.CreateAppSession(r.Context(), services.CreateAppSessionRequest{
		Username:    ctx.GetUser(),
		PublicAddr:  result.PublicAddr,
		ClusterName: result.ClusterName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Block and wait a few seconds for the session that was created to show up
	// in the cache. If this request is not blocked here, it can get stuck in a
	// racy session creation loop.
	err = h.waitForAppSession(r.Context(), ws.GetName(), ctx.GetUser())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Extract the identity of the user.
	certificate, err := tlsca.ParseCertificatePEM(ws.GetTLSCert())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity, err := tlsca.FromSubject(certificate.Subject, certificate.NotAfter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Now that the certificate has been issued, emit a "new session created"
	// for all events associated with this certificate.
	appSessionStartEvent := &events.AppSessionStart{
		Metadata: events.Metadata{
			Type:        events.AppSessionStartEvent,
			Code:        events.AppSessionStartCode,
			ClusterName: identity.RouteToApp.ClusterName,
		},
		ServerMetadata: events.ServerMetadata{
			ServerID:        h.cfg.HostUUID,
			ServerNamespace: defaults.Namespace,
		},
		SessionMetadata: events.SessionMetadata{
			SessionID: identity.RouteToApp.SessionID,
			WithMFA:   identity.MFAVerified,
		},
		UserMetadata: events.UserMetadata{
			User: ws.GetUser(),
		},
		ConnectionMetadata: events.ConnectionMetadata{
			RemoteAddr: r.RemoteAddr,
		},
		PublicAddr: identity.RouteToApp.PublicAddr,
	}
	if err := h.cfg.Emitter.EmitAuditEvent(h.cfg.Context, appSessionStartEvent); err != nil {
		return nil, trace.Wrap(err)
	}

	return &CreateAppSessionResponse{
		CookieValue: ws.GetName(),
		FQDN:        result.FQDN,
	}, nil
}

// waitForAppSession will block until the requested application session shows up in the
// cache or a timeout occurs.
func (h *Handler) waitForAppSession(ctx context.Context, sessionID, user string) error {
	return auth.WaitForAppSession(ctx, sessionID, user, h.cfg.AccessPoint)
}

type resolveAppParams struct {
	// FQDNHint indicates (tentatively) the fully qualified domain name of the application.
	FQDNHint string `json:"fqdn"`

	// PublicAddr is the public address of the application.
	PublicAddr string `json:"public_addr"`

	// ClusterName is the cluster within which this application is running.
	ClusterName string `json:"cluster_name"`
}

type resolveAppResult struct {
	// ServerID is the ID of the server this application is running on.
	ServerID string
	// FQDN is the best effort FQDN resolved for this application.
	FQDN string
	// PublicAddr of application requested.
	PublicAddr string
	// ClusterName is the name of the cluster within which the application
	// is running.
	ClusterName string
}

func (h *Handler) resolveApp(ctx context.Context, clt app.Getter, proxy reversetunnel.Tunnel, params resolveAppParams) (*resolveAppResult, error) {
	var (
		app            *types.App
		server         services.Server
		appClusterName string
		err            error
	)

	// If the request contains a public address and cluster name (for example, if it came
	// from the application launcher in the Web UI) then directly exactly resolve the
	// application that the caller is requesting. If it does not, do best effort FQDN resolution.
	switch {
	case params.PublicAddr != "" && params.ClusterName != "":
		app, server, appClusterName, err = h.resolveDirect(ctx, proxy, params.PublicAddr, params.ClusterName)
	case params.FQDNHint != "":
		app, server, appClusterName, err = h.resolveFQDN(ctx, clt, proxy, params.FQDNHint)
	default:
		err = trace.BadParameter("no inputs to resolve application")
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fqdn := ui.AssembleAppFQDN(h.auth.clusterName, h.proxyDNSName(), appClusterName, app)

	return &resolveAppResult{
		ServerID:    server.GetName(),
		FQDN:        fqdn,
		PublicAddr:  app.PublicAddr,
		ClusterName: appClusterName,
	}, nil
}

// resolveDirect takes a public address and cluster name and exactly resolves
// the application and the server on which it is running.
func (h *Handler) resolveDirect(ctx context.Context, proxy reversetunnel.Tunnel, publicAddr string, clusterName string) (*types.App, services.Server, string, error) {
	clusterClient, err := proxy.GetSite(clusterName)
	if err != nil {
		return nil, nil, "", trace.Wrap(err)
	}

	authClient, err := clusterClient.GetClient()
	if err != nil {
		return nil, nil, "", trace.Wrap(err)
	}

	app, server, err := app.Match(ctx, authClient, app.MatchPublicAddr(publicAddr))
	if err != nil {
		return nil, nil, "", trace.Wrap(err)
	}

	return app, server, clusterName, nil
}

// resolveFQDN makes a best effort attempt to resolve FQDN to an application
// running within a root or leaf cluster.
func (h *Handler) resolveFQDN(ctx context.Context, clt app.Getter, proxy reversetunnel.Tunnel, fqdn string) (*types.App, services.Server, string, error) {
	return app.ResolveFQDN(ctx, clt, proxy, []string{h.proxyDNSName()}, fqdn)
}

// proxyDNSName is a DNS name the HTTP proxy is available at, where
// the local cluster name is used as a best-effort fallback.
func (h *Handler) proxyDNSName() string {
	dnsName, err := utils.DNSName(h.cfg.ProxySettings.SSH.PublicAddr)
	if err != nil {
		return h.auth.clusterName
	}
	return dnsName
}
