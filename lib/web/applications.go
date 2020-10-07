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

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web/app"
	"github.com/gravitational/teleport/lib/web/ui"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
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
	// CookieValue is aap application cookie value
	CookieValue string `json:"value"`
	// FQDN is application fqdn
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

	log.Debugf("Attempting to create application session for %v in %v.", result.PublicAddr, result.ClusterName)

	// Get a client connected to either to local auth or remote auth with the
	// users identity.
	var userClient auth.ClientI
	if result.ClusterName == h.cfg.DomainName {
		userClient, err = ctx.GetClient()
	} else {
		remoteClient, err := h.cfg.Proxy.GetSite(result.ClusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		userClient, err = ctx.GetUserClient(remoteClient)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Attempt to create an application session within whichever cluster the
	// user requested. This requests goes to the root (or leaf) auth cluster
	// where access to the application is checked.
	appSession, err := userClient.CreateAppSession(r.Context(), services.CreateAppSessionRequest{
		PublicAddr: result.PublicAddr,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get a client connected to the local auth server with the users identity.
	localClient, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create an application web session within the cluster the request arrived.
	webSession, err := localClient.CreateAppWebSession(r.Context(), services.CreateAppWebSessionRequest{
		Username:      ctx.GetUser(),
		ParentSession: ctx.sess.GetName(),
		AppSessionID:  appSession.GetName(),
		Expires:       appSession.Expiry(),
		ServerID:      result.ServerID,
		ClusterName:   result.ClusterName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Marshal cookie from application web session.
	appCookie := app.Cookie{
		Username:   webSession.GetUser(),
		ParentHash: webSession.GetParentHash(),
		SessionID:  webSession.GetName(),
	}
	appCookieValue, err := app.EncodeCookie(&appCookie)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &createAppSessionResponse{
		CookieValue: appCookieValue,
		FQDN:        result.FQDN,
	}, nil
}

func (h *Handler) validateAppSessionRequest(ctx context.Context, req *createAppSessionRequest) (*validateAppSessionResult, error) {
	// To safely redirect a user to the app URL, the FQDN should be always resolved
	app, server, clusterName, err := h.resolveFQDN(ctx, req.FQDN)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.PublicAddr != "" && req.ClusterName != "" {
		app, server, clusterName, err = h.resolveDirect(ctx, req.PublicAddr, req.ClusterName)
		return nil, trace.Wrap(err)
	}

	return &validateAppSessionResult{
		PublicAddr:  app.PublicAddr,
		ServerID:    server.GetName(),
		ClusterName: clusterName,
		FQDN:        req.FQDN,
	}, nil
}

type validateAppSessionResult struct {
	PublicAddr  string
	ServerID    string
	ClusterName string
	FQDN        string
}

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
