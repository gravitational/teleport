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

// Package web implements web proxy handler that provides
// web interface to view and connect to teleport nodes
package web

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web/app"
)

type GetAppDetailsRequest ResolveAppParams

type GetAppDetailsResponse struct {
	// FQDN is application FQDN.
	FQDN string `json:"fqdn"`
	// RequiredAppFQDNs is a list of required app fqdn
	RequiredAppFQDNs []string `json:"requiredAppFQDNs"`
}

// getAppDetails resolves the input params to a known application and returns
// its app details.
//
// GET /v1/webapi/apps/:fqdnHint/:clusterName/:publicAddr
func (h *Handler) getAppDetails(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext) (interface{}, error) {
	values := r.URL.Query()

	isRedirectFlow := values.Get("required-apps") != ""
	clusterName := p.ByName("clusterName")

	req := GetAppDetailsRequest{
		FQDNHint:    p.ByName("fqdnHint"),
		ClusterName: clusterName,
		PublicAddr:  p.ByName("publicAddr"),
	}

	// Use the information the caller provided to attempt to resolve to an
	// application running within either the root or leaf cluster.
	result, err := h.resolveApp(r.Context(), ctx, ResolveAppParams(req))
	if err != nil {
		return nil, trace.Wrap(err, "unable to resolve FQDN: %v", req.FQDNHint)
	}

	resp := &GetAppDetailsResponse{
		FQDN: result.FQDN,
	}

	requiredAppNames := result.App.GetRequiredAppNames()

	if !isRedirectFlow {
		// TODO (avatus) this would be nice if the string in the RequiredApps spec was the fqdn of the required app
		// so we could skip the resolution step all together but this would break existing configs.

		// if clusterName is not supplied in the params, the initial app must have been fetched with fqdn hint only.
		// We can use the clusterName of the initially resolved app
		if clusterName == "" {
			clusterName = result.ClusterName
		}
		for _, requiredAppName := range requiredAppNames {
			if result.App.GetUseAnyProxyPublicAddr() {
				proxyDNSName := utils.FindMatchingProxyDNS(req.FQDNHint, h.proxyDNSNames())
				requiredAppFQDN := fmt.Sprintf("%s.%s", requiredAppName, proxyDNSName)
				resp.RequiredAppFQDNs = append(resp.RequiredAppFQDNs, requiredAppFQDN)
				continue
			}
			res, err := h.resolveApp(r.Context(), ctx, ResolveAppParams{ClusterName: clusterName, AppName: requiredAppName})
			if err != nil {
				h.logger.ErrorContext(r.Context(), "Error getting app details for associated required app", "required_app", requiredAppName, "app", result.App.GetName(), "error", err)
				continue
			}
			resp.RequiredAppFQDNs = append(resp.RequiredAppFQDNs, res.FQDN)
		}
		// append self to end of required apps so that it can be the final entry in the redirect "chain".
		resp.RequiredAppFQDNs = append(resp.RequiredAppFQDNs, result.FQDN)
	}

	return resp, nil
}

// CreateAppSessionResponse is a request to POST /v1/webapi/sessions/app
type CreateAppSessionRequest struct {
	// ResolveAppParams contains info used to resolve an application
	ResolveAppParams
	// AWSRole is the AWS role ARN when accessing AWS management console.
	AWSRole string `json:"arn,omitempty"`
	// MFAResponse is an optional MFA response used to create an MFA verified app session.
	MFAResponse client.MFAChallengeResponse `json:"mfaResponse"`
	// TODO(Joerger): DELETE IN v19.0.0
	// Backwards compatible version of MFAResponse
	MFAResponseJSON string `json:"mfa_response"`
}

// CreateAppSessionResponse is a response to POST /v1/webapi/sessions/app
type CreateAppSessionResponse struct {
	// CookieValue is the application session cookie value.
	CookieValue string `json:"cookie_value"`
	// SubjectCookieValue is the application session subject cookie token.
	SubjectCookieValue string `json:"subject_cookie_value"`
	// FQDN is application FQDN.
	FQDN string `json:"fqdn"`
}

// createAppSession creates a new application session.
//
// POST /v1/webapi/sessions/app
func (h *Handler) createAppSession(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext) (interface{}, error) {
	var req CreateAppSessionRequest
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	result, err := h.resolveApp(r.Context(), ctx, req.ResolveAppParams)
	if err != nil {
		return nil, trace.Wrap(err, "unable to resolve FQDN: %v", req.FQDNHint)
	}

	h.logger.DebugContext(r.Context(), "Creating application web session", "app_public_addr", result.App.GetPublicAddr(), "cluster", result.ClusterName)

	// Ensuring proxy can handle the connection is only done when the request is
	// coming from the WebUI.
	if h.healthCheckAppServer != nil && !app.HasClientCert(r) {
		h.logger.DebugContext(r.Context(), "Ensuring proxy can handle requests requests for application", "app", result.App.GetName())
		err := h.healthCheckAppServer(r.Context(), result.App.GetPublicAddr(), result.ClusterName)
		if err != nil {
			return nil, trace.ConnectionProblem(err, "Unable to serve application requests. Please try again. If the issue persists, verify if the Application Services are connected to Teleport.")
		}
	}

	mfaResponse, err := req.MFAResponse.GetOptionalMFAResponseProtoReq()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Fallback to backwards compatible mfa response.
	if mfaResponse == nil && req.MFAResponseJSON != "" {
		mfaResponse, err = client.ParseMFAChallengeResponse([]byte(req.MFAResponseJSON))
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Get an auth client connected with the user's identity.
	authClient, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create an application web session.
	//
	// Application sessions should not last longer than the parent session.TTL
	// will be derived from the identity which has the same expiration as the
	// parent web session.
	//
	// PublicAddr and ClusterName will get encoded within the certificate and
	// used for request routing.
	ws, err := authClient.CreateAppSession(r.Context(), &proto.CreateAppSessionRequest{
		Username:    ctx.GetUser(),
		PublicAddr:  result.App.GetPublicAddr(),
		ClusterName: result.ClusterName,
		AWSRoleARN:  req.AWSRole,
		MFAResponse: mfaResponse,
		AppName:     result.App.GetName(),
		URI:         result.App.GetURI(),
		ClientAddr:  r.RemoteAddr,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &CreateAppSessionResponse{
		CookieValue:        ws.GetName(),
		SubjectCookieValue: ws.GetBearerToken(),
		FQDN:               result.FQDN,
	}, nil
}

type ResolveAppParams struct {
	// FQDNHint indicates (tentatively) the fully qualified domain name of the application.
	FQDNHint string `json:"fqdn,omitempty"`

	// PublicAddr is the public address of the application.
	PublicAddr string `json:"public_addr,omitempty"`

	// ClusterName is the cluster within which this application is running.
	ClusterName string `json:"cluster_name,omitempty"`

	// AppName is the name of the application
	AppName string `json:"app_name,omitempty"`
}

type resolveAppResult struct {
	// ServerID is the ID of the server this application is running on.
	ServerID string
	// FQDN is the best effort FQDN resolved for this application.
	FQDN string
	// ClusterName is the name of the cluster within which the application
	// is running.
	ClusterName string
	// App is the requested application.
	App types.Application
}

// Use the information the caller provided to attempt to resolve to an
// application running within either the root or leaf cluster.
func (h *Handler) resolveApp(ctx context.Context, scx *SessionContext, params ResolveAppParams) (*resolveAppResult, error) {
	// Get an auth client connected with the user's identity.
	authClient, err := scx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get a reverse tunnel proxy aware of the user's permissions.
	proxy, err := h.ProxyWithRoles(ctx, scx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var (
		server         types.AppServer
		appClusterName string
	)

	// If the request contains a public address and cluster name (for example, if it came
	// from the application launcher in the Web UI) then directly exactly resolve the
	// application that the caller is requesting. If it does not, do best effort FQDN resolution.
	switch {
	case params.AppName != "" && params.ClusterName != "":
		server, appClusterName, err = h.resolveAppByName(ctx, proxy, params.AppName, params.ClusterName)
	case params.PublicAddr != "" && params.ClusterName != "":
		server, appClusterName, err = h.resolveDirect(ctx, proxy, params.PublicAddr, params.ClusterName)
	case params.FQDNHint != "":
		server, appClusterName, err = h.resolveFQDN(ctx, authClient, proxy, params.FQDNHint)
	default:
		err = trace.BadParameter("no inputs to resolve application")
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyDNSName := h.proxyDNSName()
	if server.GetApp().GetUseAnyProxyPublicAddr() {
		proxyDNSName = utils.FindMatchingProxyDNS(params.FQDNHint, h.proxyDNSNames())
	}
	fqdn := utils.AssembleAppFQDN(h.auth.clusterName, proxyDNSName, appClusterName, server.GetApp())

	return &resolveAppResult{
		ServerID:    server.GetName(),
		FQDN:        fqdn,
		ClusterName: appClusterName,
		App:         server.GetApp(),
	}, nil
}

// resolveAppByName will take an application name and cluster name and exactly resolves
// the application and the server on which it is running.
func (h *Handler) resolveAppByName(ctx context.Context, proxy reversetunnelclient.Tunnel, appName string, clusterName string) (types.AppServer, string, error) {
	clusterClient, err := proxy.GetSite(clusterName)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	authClient, err := clusterClient.GetClient()
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	servers, err := app.MatchUnshuffled(ctx, authClient, app.MatchName(appName))
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	if len(servers) == 0 {
		return nil, "", trace.NotFound("failed to match applications with name %s", appName)
	}

	return servers[rand.N(len(servers))], clusterName, nil
}

// resolveDirect takes a public address and cluster name and exactly resolves
// the application and the server on which it is running.
func (h *Handler) resolveDirect(ctx context.Context, proxy reversetunnelclient.Tunnel, publicAddr string, clusterName string) (types.AppServer, string, error) {
	clusterClient, err := proxy.GetSite(clusterName)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	authClient, err := clusterClient.GetClient()
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	servers, err := app.MatchUnshuffled(ctx, authClient, app.MatchPublicAddr(publicAddr))
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	if len(servers) == 0 {
		return nil, "", trace.NotFound("failed to match applications with public addr %s", publicAddr)
	}

	return servers[rand.N(len(servers))], clusterName, nil
}

// resolveFQDN makes a best effort attempt to resolve FQDN to an application
// running within a root or leaf cluster.
func (h *Handler) resolveFQDN(ctx context.Context, clt app.Getter, proxy reversetunnelclient.Tunnel, fqdn string) (types.AppServer, string, error) {
	return app.ResolveFQDN(ctx, clt, proxy, h.proxyDNSNames(), fqdn)
}

// proxyDNSName is a DNS name the HTTP proxy is available at, where
// the local cluster name is used as a best-effort fallback.
func (h *Handler) proxyDNSName() string {
	dnsNames := h.proxyDNSNames()
	if len(dnsNames) == 0 {
		return h.auth.clusterName
	}
	return dnsNames[0]
}

// proxyDNSNames returns DNS names the HTTP proxy is available at, the local
// cluster name is used as a best-effort fallback.
func (h *Handler) proxyDNSNames() (dnsNames []string) {
	for _, addr := range h.cfg.ProxyPublicAddrs {
		dnsName, err := utils.DNSName(addr.String())
		if err != nil {
			continue
		}
		dnsNames = append(dnsNames, dnsName)
	}
	if len(dnsNames) == 0 {
		return []string{h.auth.clusterName}
	}
	return dnsNames
}
