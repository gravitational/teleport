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
	"net/http"
	"strings"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web/app"
	"github.com/gravitational/teleport/lib/web/ui"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

func (h *Handler) siteAppsGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	appServers, err := clt.GetApps(r.Context(), defaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxies, err := clt.GetProxies()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyClusterName, err := h.cfg.ProxyClient.GetClusterName()
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	proxyName := proxyClusterName.GetClusterName()
	proxyHost, _, err := services.GuessProxyHostAndVersion(proxies)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	// remove port number if any
	proxyHost = strings.Split(proxyHost, ":")[0]
	appClusterName := p.ByName("site")

	return makeResponse(ui.MakeApps(proxyName, proxyHost, appClusterName, appServers))
}

func (h *Handler) createAppSession(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext) (interface{}, error) {
	var req *createAppSessionRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	// Get a client to auth with the identity of the logged in user and use it
	// to request the creation of an application session for this user.
	client, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	session, err := client.CreateAppSession(r.Context(), services.CreateAppSessionRequest{
		// TODO: add app name
		// TODO: add app cluster id
		SessionID: ctx.GetWebSession().GetName(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	appCookie := app.Cookie{
		Username:   session.GetUser(),
		ParentHash: session.GetParentHash(),
		SessionID:  session.GetName(),
	}

	appCookieValue, err := app.EncodeCookie(&appCookie)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &createAppSessionResponse{
		CookieValue: appCookieValue,
	}, nil
}

type createAppSessionResponse struct {
	// CookieValue is aap application cookie value
	CookieValue string `json:"value"`
}

type createAppSessionRequest struct {
	// FQDN is the full qualified domain name of the application
	FQDN string `json:"fqdn"`
	// ClusterName is the cluster within which the application is running.
	ClusterName string `json:"cluster_name"`
}
