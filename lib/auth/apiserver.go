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

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/defaults"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/machineid/workloadidentityv1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

type APIConfig struct {
	PluginRegistry plugin.Registry
	AuthServer     *Server
	AuditLog       events.AuditLogSessionStreamer
	Authorizer     authz.Authorizer
	Emitter        apievents.Emitter
	// KeepAlivePeriod defines period between keep alives
	KeepAlivePeriod time.Duration
	// KeepAliveCount specifies amount of missed keep alives
	// to wait for until declaring connection as broken
	KeepAliveCount int
	// MetadataGetter retrieves additional metadata about session uploads.
	// Will be nil if audit logging is not enabled.
	MetadataGetter events.UploadMetadataGetter
	// AccessGraph contains the configuration about the access graph service
	AccessGraph AccessGraphConfig
	// MutateRevocationsServiceConfig is a function that allows to mutate
	// the revocation service configuration for testing.
	MutateRevocationsServiceConfig func(config *workloadidentityv1.RevocationServiceConfig)
}

// CheckAndSetDefaults checks and sets default values
func (a *APIConfig) CheckAndSetDefaults() error {
	if a.KeepAlivePeriod == 0 {
		a.KeepAlivePeriod = apidefaults.ServerKeepAliveTTL()
	}
	if a.KeepAliveCount == 0 {
		a.KeepAliveCount = apidefaults.KeepAliveCountMax
	}
	if a.Authorizer == nil {
		return trace.BadParameter("authorizer is missing")
	}
	return nil
}

// AccessGraphConfig contains the configuration about the access graph service
// and whether it is enabled or not.
type AccessGraphConfig struct {
	// Enabled is a flag that indicates whether the access graph service is enabled.
	Enabled bool
	// Address is the address of the access graph service. The address is in the
	// form of "host:port".
	Address string
	// CA is the PEM-encoded CA certificate of the access graph service.
	CA []byte
	// Insecure is a flag that indicates whether the access graph service should
	// skip verifying the server's certificate chain and host name.
	Insecure bool
}

// APIServer implements http API server for AuthServer interface
type APIServer struct {
	APIConfig
	httprouter.Router
	clockwork.Clock
}

// NewAPIServer returns a new instance of APIServer HTTP handler
func NewAPIServer(config *APIConfig) (http.Handler, error) {
	srv := APIServer{
		APIConfig: *config,
		Clock:     clockwork.NewRealClock(),
	}
	srv.Router = *httprouter.New()
	srv.Router.UseRawPath = true

	// Passwords and sessions
	srv.POST("/:version/users/:user/web/sessions", srv.WithAuth(srv.createWebSession))
	srv.POST("/:version/users/:user/web/authenticate", srv.WithAuth(srv.authenticateWebUser))
	srv.POST("/:version/users/:user/ssh/authenticate", srv.WithAuth(srv.authenticateSSHUser))
	srv.GET("/:version/users/:user/web/sessions/:sid", srv.WithAuth(srv.getWebSession))
	srv.DELETE("/:version/users/:user/web/sessions/:sid", srv.WithAuth(srv.deleteWebSession))

	// Servers and presence heartbeat
	srv.POST("/:version/namespaces/:namespace/nodes/keepalive", srv.WithAuth(srv.keepAliveNode))
	srv.POST("/:version/authservers", srv.WithAuth(srv.upsertAuthServer))
	srv.GET("/:version/authservers", srv.WithAuth(srv.getAuthServers))
	srv.POST("/:version/proxies", srv.WithAuth(srv.upsertProxy))
	srv.GET("/:version/proxies", srv.WithAuth(srv.getProxies))
	srv.DELETE("/:version/proxies", srv.WithAuth(srv.deleteAllProxies))
	srv.DELETE("/:version/proxies/:name", srv.WithAuth(srv.deleteProxy))
	srv.POST("/:version/tunnelconnections", srv.WithAuth(srv.upsertTunnelConnection))
	srv.GET("/:version/tunnelconnections/:cluster", srv.WithAuth(srv.getTunnelConnections))
	srv.GET("/:version/tunnelconnections", srv.WithAuth(srv.getAllTunnelConnections))
	srv.DELETE("/:version/tunnelconnections/:cluster/:conn", srv.WithAuth(srv.deleteTunnelConnection))
	srv.DELETE("/:version/tunnelconnections/:cluster", srv.WithAuth(srv.deleteTunnelConnections))
	srv.DELETE("/:version/tunnelconnections", srv.WithAuth(srv.deleteAllTunnelConnections))

	// trusted clusters
	srv.POST("/:version/trustedclusters/validate", srv.WithAuth(srv.validateTrustedCluster))

	// these endpoints are still in use by v17 agents since they cache
	// KindNamespace
	//
	// TODO(espadolini): REMOVE IN v19
	srv.GET("/:version/namespaces", srv.WithAuth(srv.getNamespaces))
	srv.GET("/:version/namespaces/:namespace", srv.WithAuth(srv.getNamespace))

	// cluster configuration
	// TODO(noah): DELETE IN v19.0.0 - from v18 we switched to a gRPC equiv.
	srv.GET("/:version/configuration/name", srv.WithAuth(srv.getClusterName))

	// SSO validation handlers
	srv.POST("/:version/github/requests/validate", srv.WithAuth(srv.validateGithubAuthCallback))

	// Migrated/deleted endpoints with 501 Not Implemented handlers.
	srv.POST("/:version/reversetunnels", httpMigratedHandler)
	srv.GET("/:version/reversetunnels", httpMigratedHandler)
	srv.DELETE("/:version/reversetunnels/:domain", httpMigratedHandler)

	if config.PluginRegistry != nil {
		if err := config.PluginRegistry.RegisterAuthWebHandlers(&srv); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return httplib.RewritePaths(&srv.Router,
		httplib.Rewrite("/v1/nodes", "/v1/namespaces/default/nodes"),
		httplib.Rewrite("/v1/sessions", "/v1/namespaces/default/sessions"),
		httplib.Rewrite("/v1/sessions/([^/]+)/(.*)", "/v1/namespaces/default/sessions/$1/$2"),
	), nil
}

// httpMigratedHandler returns a 501 Not Implemented error. This is used when
// removing a HTTP handler that has been migrated to gRPC, where, a 404 NotFound
// could be misinterpreted as the resource not existing, rather than the RPC
// no longer existing.
var httpMigratedHandler = httplib.MakeHandler(func(
	w http.ResponseWriter, r *http.Request, p httprouter.Params,
) (any, error) {
	return nil, trace.NotImplemented("HTTP RPC migrated to gRPC, check client version")
})

// HandlerWithAuthFunc is http handler with passed auth context
type HandlerWithAuthFunc func(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error)

func (s *APIServer) WithAuth(handler HandlerWithAuthFunc) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		// HTTPS server expects auth context to be set by the auth middleware
		authContext, err := s.Authorizer.Authorize(r.Context())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		auth := &ServerWithRoles{
			authServer: s.AuthServer,
			context:    *authContext,
			alog:       s.AuthServer,
		}
		version := p.ByName("version")
		if version == "" {
			return nil, trace.BadParameter("missing version")
		}
		return handler(auth, w, r, p, version)
	})
}

type upsertServerRawReq struct {
	Server json.RawMessage `json:"server"`
	TTL    time.Duration   `json:"ttl"`
}

// presenceForAPIServer is a subset of [services.Presence].
type presenceForAPIServer interface {
	UpsertNode(ctx context.Context, s types.Server) (*types.KeepAlive, error)
	UpsertAuthServer(ctx context.Context, s types.Server) error
	UpsertProxy(ctx context.Context, s types.Server) error
}

// upsertServer is a common utility function
func (s *APIServer) upsertServer(auth presenceForAPIServer, role types.SystemRole, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req upsertServerRawReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	var kind string
	switch role {
	case types.RoleNode:
		kind = types.KindNode
	case types.RoleAuth:
		kind = types.KindAuthServer
	case types.RoleProxy:
		kind = types.KindProxy
	default:
		return nil, trace.BadParameter("upsertServer with unknown role: %q", role)
	}
	server, err := services.UnmarshalServer(req.Server, kind)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// if server sent "local" IP address to us, replace the ip/host part with the remote address we see
	// on the socket, but keep the original port:
	server.SetAddr(utils.ReplaceLocalhost(server.GetAddr(), r.RemoteAddr))
	if req.TTL != 0 {
		server.SetExpiry(s.Now().UTC().Add(req.TTL))
	}
	switch role {
	case types.RoleNode:
		namespace := p.ByName("namespace")
		if !types.IsValidNamespace(namespace) {
			return nil, trace.BadParameter("invalid namespace %q", namespace)
		}
		server.SetNamespace(namespace)
		handle, err := auth.UpsertNode(r.Context(), server)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return handle, nil
	case types.RoleAuth:
		if err := auth.UpsertAuthServer(r.Context(), server); err != nil {
			return nil, trace.Wrap(err)
		}
	case types.RoleProxy:
		if err := auth.UpsertProxy(r.Context(), server); err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		return nil, trace.BadParameter("unknown server role %q", role)
	}
	return message("ok"), nil
}

// keepAliveNode updates node TTL in the backend
func (s *APIServer) keepAliveNode(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var handle types.KeepAlive
	if err := httplib.ReadJSON(r, &handle); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.KeepAliveServer(r.Context(), handle); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// upsertProxy is called by remote SSH nodes when they ping back into the auth service
func (s *APIServer) upsertProxy(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	return s.upsertServer(auth, types.RoleProxy, r, p)
}

// getProxies returns registered proxies
func (s *APIServer) getProxies(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	servers, err := auth.GetProxies()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return marshalServers(servers, version)
}

// deleteAllProxies deletes all proxies
func (s *APIServer) deleteAllProxies(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	err := auth.DeleteAllProxies()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// deleteProxy deletes proxy
func (s *APIServer) deleteProxy(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	name := p.ByName("name")
	if name == "" {
		return nil, trace.BadParameter("missing proxy name")
	}
	err := auth.DeleteProxy(r.Context(), name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// upsertAuthServer is called by remote Auth servers when they ping back into the auth service
func (s *APIServer) upsertAuthServer(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	return s.upsertServer(auth, types.RoleAuth, r, p)
}

// getAuthServers returns registered auth servers
func (s *APIServer) getAuthServers(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	servers, err := auth.GetAuthServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return marshalServers(servers, version)
}

func marshalServers(servers []types.Server, version string) (interface{}, error) {
	items := make([]json.RawMessage, len(servers))
	for i, server := range servers {
		data, err := services.MarshalServer(server, services.WithVersion(version), services.PreserveRevision())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		items[i] = data
	}
	return items, nil
}

func (s *APIServer) validateTrustedCluster(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var validateRequestRaw authclient.ValidateTrustedClusterRequestRaw
	if err := httplib.ReadJSON(r, &validateRequestRaw); err != nil {
		return nil, trace.Wrap(err)
	}

	validateRequest, err := validateRequestRaw.ToNative()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	validateResponse, err := auth.ValidateTrustedCluster(r.Context(), validateRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	validateResponseRaw, err := validateResponse.ToRaw()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return validateResponseRaw, nil
}

func (s *APIServer) deleteWebSession(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	user, sessionID := p.ByName("user"), p.ByName("sid")
	err := auth.WebSessions().Delete(r.Context(), types.DeleteWebSessionRequest{
		User:      user,
		SessionID: sessionID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("session %q for user %q deleted", sessionID, user)), nil
}

func (s *APIServer) getWebSession(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	user, sid := p.ByName("user"), p.ByName("sid")
	sess, err := auth.GetWebSessionInfo(r.Context(), user, sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rawMessage(services.MarshalWebSession(sess, services.WithVersion(version)))
}

func (s *APIServer) createWebSession(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req authclient.WebSessionReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.PrevSessionID != "" {
		sess, err := auth.ExtendWebSession(r.Context(), req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return sess, nil
	}

	sess, err := auth.CreateWebSession(r.Context(), req.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rawMessage(services.MarshalWebSession(sess, services.WithVersion(version)))
}

func (s *APIServer) authenticateWebUser(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req authclient.AuthenticateUserRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	req.Username = p.ByName("user")
	sess, err := auth.AuthenticateWebUser(r.Context(), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rawMessage(services.MarshalWebSession(sess, services.WithVersion(version)))
}

func (s *APIServer) authenticateSSHUser(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req authclient.AuthenticateSSHRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	req.Username = p.ByName("user")
	return auth.AuthenticateSSHUser(r.Context(), req)
}

func rawMessage(data []byte, err error) (interface{}, error) {
	if err != nil {
		return nil, trace.Wrap(err)
	}
	m := json.RawMessage(data)
	return &m, nil
}

// validateGithubAuthCallbackReq is a request to validate Github OAuth2 callback
type validateGithubAuthCallbackReq struct {
	// Query is the callback query string
	Query url.Values `json:"query"`
}

// githubAuthRawResponse is returned when auth server validated callback
// parameters returned from Github during OAuth2 flow
type githubAuthRawResponse struct {
	// Username is authenticated teleport username
	Username string `json:"username"`
	// Identity contains validated OIDC identity
	Identity types.ExternalIdentity `json:"identity"`
	// Web session will be generated by auth server if requested in OIDCAuthRequest
	Session json.RawMessage `json:"session,omitempty"`
	// Cert will be generated by certificate authority
	Cert []byte `json:"cert,omitempty"`
	// TLSCert is PEM encoded TLS certificate
	TLSCert []byte `json:"tls_cert,omitempty"`
	// Req is original oidc auth request
	Req authclient.GithubAuthRequest `json:"req"`
	// HostSigners is a list of signing host public keys
	// trusted by proxy, used in console login
	HostSigners []json.RawMessage `json:"host_signers"`
}

/*
validateGithubAuthRequest validates Github auth callback redirect

	POST /:version/github/requests/validate

	Success response: githubAuthRawResponse
*/
func (s *APIServer) validateGithubAuthCallback(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req validateGithubAuthCallbackReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := auth.ValidateGithubAuthCallback(r.Context(), req.Query)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	raw := githubAuthRawResponse{
		Username: response.Username,
		Identity: response.Identity,
		Cert:     response.Cert,
		TLSCert:  response.TLSCert,
		Req:      response.Req,
	}
	if response.Session != nil {
		rawSession, err := services.MarshalWebSession(
			response.Session, services.WithVersion(version), services.PreserveRevision())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		raw.Session = rawSession
	}
	raw.HostSigners = make([]json.RawMessage, len(response.HostSigners))
	for i, ca := range response.HostSigners {
		data, err := services.MarshalCertAuthority(
			ca, services.WithVersion(version), services.PreserveRevision())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		raw.HostSigners[i] = data
	}
	return &raw, nil
}

func (*APIServer) getNamespaces(*ServerWithRoles, http.ResponseWriter, *http.Request, httprouter.Params, string) (any, error) {
	return []types.Namespace{{
		Kind:    types.KindNamespace,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      defaults.Namespace,
			Namespace: defaults.Namespace,
		},
	}}, nil
}

func (*APIServer) getNamespace(_ *ServerWithRoles, _ http.ResponseWriter, _ *http.Request, p httprouter.Params, _ string) (any, error) {
	name := p.ByName("namespace")
	if !types.IsValidNamespace(name) {
		return nil, trace.BadParameter("invalid namespace %q", name)
	}
	if name != defaults.Namespace {
		return nil, trace.NotFound("namespace %q is not found", name)
	}

	return &types.Namespace{
		Kind:    types.KindNamespace,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      defaults.Namespace,
			Namespace: defaults.Namespace,
		},
	}, nil
}

func (s *APIServer) getClusterName(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	cn, err := auth.GetClusterName(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rawMessage(services.MarshalClusterName(cn, services.WithVersion(version), services.PreserveRevision()))
}

type upsertTunnelConnectionRawReq struct {
	TunnelConnection json.RawMessage `json:"tunnel_connection"`
}

// upsertTunnelConnection updates or inserts tunnel connection
func (s *APIServer) upsertTunnelConnection(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req upsertTunnelConnectionRawReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	conn, err := services.UnmarshalTunnelConnection(req.TunnelConnection)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.UpsertTunnelConnection(conn); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// getTunnelConnections returns a list of tunnel connections from a cluster
func (s *APIServer) getTunnelConnections(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	conns, err := auth.GetTunnelConnections(p.ByName("cluster"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	items := make([]json.RawMessage, len(conns))
	for i, conn := range conns {
		data, err := services.MarshalTunnelConnection(conn, services.WithVersion(version), services.PreserveRevision())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		items[i] = data
	}
	return items, nil
}

// getAllTunnelConnections returns a list of tunnel connections from a cluster
func (s *APIServer) getAllTunnelConnections(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	conns, err := auth.GetAllTunnelConnections()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	items := make([]json.RawMessage, len(conns))
	for i, conn := range conns {
		data, err := services.MarshalTunnelConnection(conn, services.WithVersion(version), services.PreserveRevision())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		items[i] = data
	}
	return items, nil
}

// deleteTunnelConnection deletes tunnel connection by name
func (s *APIServer) deleteTunnelConnection(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	err := auth.DeleteTunnelConnection(p.ByName("cluster"), p.ByName("conn"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// deleteTunnelConnections deletes all tunnel connections for cluster
func (s *APIServer) deleteTunnelConnections(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	err := auth.DeleteTunnelConnections(p.ByName("cluster"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// deleteAllTunnelConnections deletes all tunnel connections
func (s *APIServer) deleteAllTunnelConnections(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	err := auth.DeleteAllTunnelConnections()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

func message(msg string) map[string]interface{} {
	return map[string]interface{}{"message": msg}
}
