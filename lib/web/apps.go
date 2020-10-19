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
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web/ui"

	"github.com/gravitational/trace"

	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

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

	// Get the public address of the proxy and remove the port.
	proxyHost, err := utils.Host(h.cfg.ProxySettings.SSH.PublicAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return makeResponse(ui.MakeApps(h.auth.clusterName, proxyHost, appClusterName, appServers))
}

type createAppSessionRequest struct {
	// FQDN is the full qualified domain name of the application.
	FQDN string `json:"fqdn"`

	// PublicAddr is the public address the application.
	PublicAddr string `json:"public_addr"`

	// ClusterName is the cluster within which this application is running.
	ClusterName string `json:"cluster_name"`
}

type createAppSessionResponse struct {
	// CookieValue is the application session cookie value.
	CookieValue string `json:"value"`
	// FQDN is application FQDN.
	FQDN string `json:"fqdn"`
}

func (h *Handler) createAppSession(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext) (interface{}, error) {
	var req *createAppSessionRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	// Use the information the caller provided to attempt to resolve to an
	// application running within either the root or leaf cluster.
	result, err := h.validateAppSessionRequest(r.Context(), req)
	if err != nil {
		return nil, trace.Wrap(err, "Unable to resolve FQDN: %v", req.FQDN)
	}

	log.Debugf("Creating application web session for %v in %v.", result.PublicAddr, result.ClusterName)

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
	// in the cache. If this request is not blocked here, it can get struck in a
	// racy session creation loop.
	err = h.waitForSession(r.Context(), ws.GetName())
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
			Type: events.AppSessionStartEvent,
			Code: events.AppSessionStartCode,
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

	return &createAppSessionResponse{
		CookieValue: ws.GetName(),
		FQDN:        result.FQDN,
	}, nil
}

// waitForSession will block until the requested session shows up in the
// cache or a timeout occurs.
func (h *Handler) waitForSession(ctx context.Context, sessionID string) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(defaults.WebHeadersTimeout)
	defer timeout.Stop()

	for {
		select {
		case <-ticker.C:
			_, err := h.cfg.AccessPoint.GetAppSession(ctx, services.GetAppSessionRequest{
				SessionID: sessionID,
			})
			if err == nil {
				return nil
			}
		case <-timeout.C:
			return trace.BadParameter("timed out waiting for session")
		}
	}
}

func (h *Handler) validateAppSessionRequest(ctx context.Context, req *createAppSessionRequest) (*validateAppSessionResult, error) {
	// To safely redirect a user to the app URL, the FQDN should be always
	// resolved. This is to prevent open redirects.
	app, server, clusterName, err := h.resolveFQDN(ctx, req.FQDN)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If the request contains a public address and cluster name (for example, if
	// it came from the application launcher in the Web UI) directly exactly
	// resolve the caller is requesting instead of best effort FQDN resolution.
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

	app, server, err := h.match(ctx, authClient, matchPublicAddr(publicAddr))
	if err != nil {
		return nil, nil, "", trace.Wrap(err)
	}

	return app, server, clusterName, nil
}

// resolveFQDN makes a best effort attempt to resolve FQDN to an application
// running a root or leaf cluster.
//
// Known edge cases this function can incorrectly resolve an application exist.
// For example, if you have an application named "acme" within both the root
// and leaf cluster, this method will always return "acme" running within the
// root cluster. Always supply public address and cluster name to
// deterministically resolve an application.
func (h *Handler) resolveFQDN(ctx context.Context, fqdn string) (*services.App, services.Server, string, error) {
	// Parse the address to remove the port if it's set.
	addr, err := utils.ParseAddr(fqdn)
	if err != nil {
		return nil, nil, "", trace.Wrap(err)
	}

	// Try and match FQDN to public address of application within cluster.
	app, server, err := h.match(ctx, h.cfg.ProxyClient, matchPublicAddr(addr.Host()))
	if err == nil {
		return app, server, h.auth.clusterName, nil
	}

	// Extract the first subdomain from the FQDN and attempt to use this as the
	// application name.
	appName := strings.Split(addr.Host(), ".")[0]

	// Try and match application name to an application within the cluster.
	app, server, err = h.match(ctx, h.cfg.ProxyClient, matchName(appName))
	if err == nil {
		return app, server, h.auth.clusterName, nil
	}

	// Loop over all clusters and try and match application name to an
	// application with the cluster.
	for _, remoteClient := range h.cfg.Proxy.GetSites() {
		authClient, err := remoteClient.CachingAccessPoint()
		if err != nil {
			return nil, nil, "", trace.Wrap(err)
		}

		app, server, err = h.match(ctx, authClient, matchName(appName))
		if err == nil {
			return app, server, remoteClient.GetName(), nil
		}
	}

	return nil, nil, "", trace.NotFound("failed to resolve %v to any application within any cluster", fqdn)
}

// match will match an application with the passed in matcher function. Matcher
// functions that can match on public address and name are available.
func (h *Handler) match(ctx context.Context, authClient appGetter, fn matcher) (*services.App, services.Server, error) {
	servers, err := authClient.GetAppServers(ctx, defaults.Namespace)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	var ma []*services.App
	var ms []services.Server

	for _, server := range servers {
		for _, app := range server.GetApps() {
			if fn(app) {
				ma = append(ma, app)
				ms = append(ms, server)
			}
		}
	}

	if len(ma) == 0 {
		return nil, nil, trace.NotFound("failed to match application")
	}
	index := rand.Intn(len(ma))
	return ma[index], ms[index], nil
}

type matcher func(*services.App) bool

func matchPublicAddr(publicAddr string) matcher {
	return func(app *services.App) bool {
		return app.PublicAddr == publicAddr
	}
}

func matchName(name string) matcher {
	return func(app *services.App) bool {
		return app.Name == name
	}
}

type appGetter interface {
	GetAppServers(context.Context, string, ...services.MarshalOption) ([]services.Server, error)
}
