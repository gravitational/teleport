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
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
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

// siteAppsGet returns a list of applications in a form the UI can present.
func (h *Handler) siteAppsGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
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

	// Get the public address of the proxy and remove the port. An empty public
	// address is fine here, it will be used to denote fallback to cluster name.
	proxyHost := h.cfg.ProxySettings.SSH.PublicAddr
	if proxyHost != "" {
		proxyHost, err = utils.Host(proxyHost)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return makeResponse(ui.MakeApps(h.auth.clusterName, proxyHost, appClusterName, appServers))
}

type CreateAppSessionRequest struct {
	// FQDN is the fully qualified domain name of the application.
	FQDN string `json:"fqdn"`

	// PublicAddr is the public address of the application.
	PublicAddr string `json:"public_addr"`

	// ClusterName is the cluster within which this application is running.
	ClusterName string `json:"cluster_name"`
}

type CreateAppSessionResponse struct {
	// CookieValue is the application session cookie value.
	CookieValue string `json:"value"`
	// FQDN is application FQDN.
	FQDN string `json:"fqdn"`
}

func (h *Handler) createAppSession(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext) (interface{}, error) {
	var req *CreateAppSessionRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	// Use the information the caller provided to attempt to resolve to an
	// application running within either the root or leaf cluster.
	result, err := h.validateAppSessionRequest(r.Context(), req)
	if err != nil {
		return nil, trace.Wrap(err, "Unable to resolve FQDN: %v", req.FQDN)
	}

	h.log.Debugf("Creating application web session for %v in %v.", result.PublicAddr, result.ClusterName)

	// Get an auth client connected with the users identity.
	authClient, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create an application web session.
	//
	// ParentSession is used to derive the TTL for the application session.
	// Application sessions should not last longer than the parent session.
	//
	// PublicAddr and ClusterName will get encoded within the certificate and
	// used for request routing.
	ws, err := authClient.CreateAppSession(r.Context(), services.CreateAppSessionRequest{
		Username:      ctx.GetUser(),
		ParentSession: ctx.sess.GetName(),
		PublicAddr:    result.PublicAddr,
		ClusterName:   result.ClusterName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Block and wait a few seconds for the session that was created to show up
	// in the cache. If this request is not blocked here, it can get stuck in a
	// racy session creation loop.
	err = h.waitForAppSession(r.Context(), ws.GetName())
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
func (h *Handler) waitForAppSession(ctx context.Context, sessionID string) error {
	// Establish a watch on application session.
	watcher, err := h.cfg.AccessPoint.NewWatcher(ctx, services.Watch{
		Name: teleport.ComponentAppProxy,
		Kinds: []services.WatchKind{
			{
				Kind:    services.KindWebSession,
				SubKind: services.KindAppSession,
			},
		},
		MetricComponent: teleport.ComponentAppProxy,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()
	sessionProber := func() error {
		_, err = h.cfg.AccessPoint.GetAppSession(ctx, services.GetAppSessionRequest{
			SessionID: sessionID,
		})
		return trace.Wrap(err)
	}
	matcher := func(event services.Event) bool {
		return event.Type == backend.OpPut && event.Resource.GetName() == sessionID
	}
	return waitForSession(ctx, watcher, sessionProber, matcher)
}

func (h *Handler) validateAppSessionRequest(ctx context.Context, req *CreateAppSessionRequest) (*validateAppSessionResult, error) {
	// To safely redirect a user to the app URL, the FQDN should be always
	// resolved. This is to prevent open redirects.
	app, server, clusterName, err := h.resolveFQDN(ctx, req.FQDN)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If the request contains a public address and cluster name (for example, if it came
	// from the application launcher in the Web UI) then directly exactly resolve the
	// application that the caller is requesting. If it does not, do best effort FQDN resolution.
	if req.PublicAddr != "" && req.ClusterName != "" {
		app, server, clusterName, err = h.resolveDirect(ctx, req.PublicAddr, req.ClusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &validateAppSessionResult{
		ServerID:    server.GetName(),
		FQDN:        req.FQDN,
		PublicAddr:  app.PublicAddr,
		ClusterName: clusterName,
	}, nil
}

type validateAppSessionResult struct {
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

// resolveDirect takes a public address and cluster name and exactly resolves
// the application and the server on which it is running.
func (h *Handler) resolveDirect(ctx context.Context, publicAddr string, clusterName string) (*services.App, services.Server, string, error) {
	clusterClient, err := h.cfg.Proxy.GetSite(clusterName)
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
// running a root or leaf cluster.
func (h *Handler) resolveFQDN(ctx context.Context, fqdn string) (*services.App, services.Server, string, error) {
	return app.ResolveFQDN(ctx, h.cfg.ProxyClient, h.cfg.Proxy, h.auth.clusterName, fqdn)
}
