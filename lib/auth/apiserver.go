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
	"strconv"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/julienschmidt/httprouter"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
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

	// Kubernetes extensions
	srv.POST("/:version/kube/csr", srv.WithAuth(srv.processKubeCSR))

	// TODO(Joerger): DELETE IN 16.0.0, migrated to gRPC.
	srv.POST("/:version/authorities/:type/rotate", srv.WithAuth(srv.rotateCertAuthority))
	// TODO(Joerger): DELETE IN v16.0.0, migrated to gRPC
	srv.POST("/:version/authorities/:type/rotate/external", srv.WithAuth(srv.rotateExternalCertAuthority))

	// Operations on users
	// TODO(tross): DELETE IN 17.0.0
	srv.POST("/:version/users", srv.WithAuth(srv.upsertUser))

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

	// Remote clusters
	// TODO(noah): DELETE IN 17.0.0 - all these methods are now gRPC.
	srv.GET("/:version/remoteclusters/:cluster", srv.WithAuth(srv.getRemoteCluster))
	srv.GET("/:version/remoteclusters", srv.WithAuth(srv.getRemoteClusters))
	srv.DELETE("/:version/remoteclusters/:cluster", srv.WithAuth(srv.deleteRemoteCluster))

	// Reverse tunnels
	srv.POST("/:version/reversetunnels", srv.WithAuth(srv.upsertReverseTunnel))
	srv.GET("/:version/reversetunnels", srv.WithAuth(srv.getReverseTunnels))
	srv.DELETE("/:version/reversetunnels/:domain", srv.WithAuth(srv.deleteReverseTunnel))

	// trusted clusters
	srv.POST("/:version/trustedclusters/validate", srv.WithAuth(srv.validateTrustedCluster))

	// Tokens
	srv.POST("/:version/tokens/register", srv.WithAuth(srv.registerUsingToken))

	// Active sessions
	// TODO(zmb3): remove these endpoints when Assist no longer needs them
	srv.GET("/:version/namespaces/:namespace/sessions/:id/stream", srv.WithAuth(srv.getSessionChunk))
	srv.GET("/:version/namespaces/:namespace/sessions/:id/events", srv.WithAuth(srv.getSessionEvents))

	// Namespaces
	srv.POST("/:version/namespaces", srv.WithAuth(srv.upsertNamespace))
	srv.GET("/:version/namespaces", srv.WithAuth(srv.getNamespaces))
	srv.GET("/:version/namespaces/:namespace", srv.WithAuth(srv.getNamespace))
	srv.DELETE("/:version/namespaces/:namespace", srv.WithAuth(srv.deleteNamespace))

	// cluster configuration
	srv.GET("/:version/configuration/name", srv.WithAuth(srv.getClusterName))
	srv.POST("/:version/configuration/name", srv.WithAuth(srv.setClusterName))

	// SSO validation handlers
	srv.POST("/:version/github/requests/validate", srv.WithAuth(srv.validateGithubAuthCallback))

	// Audit logs AKA events
	srv.GET("/:version/events", srv.WithAuth(srv.searchEvents))
	srv.GET("/:version/events/session", srv.WithAuth(srv.searchSessionEvents))

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
		data, err := services.MarshalServer(server, services.WithVersion(version), services.PreserveResourceID())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		items[i] = data
	}
	return items, nil
}

type upsertReverseTunnelRawReq struct {
	ReverseTunnel json.RawMessage `json:"reverse_tunnel"`
	TTL           time.Duration   `json:"ttl"`
}

// upsertReverseTunnel is called by admin to create a reverse tunnel to remote proxy
func (s *APIServer) upsertReverseTunnel(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req upsertReverseTunnelRawReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	tun, err := services.UnmarshalReverseTunnel(req.ReverseTunnel)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := services.ValidateReverseTunnel(tun); err != nil {
		return nil, trace.Wrap(err)
	}
	if req.TTL != 0 {
		tun.SetExpiry(s.Now().UTC().Add(req.TTL))
	}
	if err := auth.UpsertReverseTunnel(tun); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// getReverseTunnels returns a list of reverse tunnels
func (s *APIServer) getReverseTunnels(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	reverseTunnels, err := auth.GetReverseTunnels(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	items := make([]json.RawMessage, len(reverseTunnels))
	for i, tunnel := range reverseTunnels {
		data, err := services.MarshalReverseTunnel(tunnel, services.WithVersion(version), services.PreserveResourceID())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		items[i] = data
	}
	return items, nil
}

// deleteReverseTunnel deletes reverse tunnel
func (s *APIServer) deleteReverseTunnel(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	domainName := p.ByName("domain")
	err := auth.DeleteReverseTunnel(domainName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("reverse tunnel %v deleted", domainName)), nil
}

func (s *APIServer) validateTrustedCluster(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var validateRequestRaw ValidateTrustedClusterRequestRaw
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

type WebSessionReq struct {
	// User is the user name associated with the session id.
	User string `json:"user"`
	// PrevSessionID is the id of current session.
	PrevSessionID string `json:"prev_session_id"`
	// AccessRequestID is an optional field that holds the id of an approved access request.
	AccessRequestID string `json:"access_request_id"`
	// Switchback is a flag to indicate if user is wanting to switchback from an assumed role
	// back to their default role.
	Switchback bool `json:"switchback"`
	// ReloadUser is a flag to indicate if user needs to be refetched from the backend
	// to apply new user changes e.g. user traits were updated.
	ReloadUser bool `json:"reload_user"`
}

func (s *APIServer) createWebSession(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req WebSessionReq
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
	var req AuthenticateUserRequest
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
	var req AuthenticateSSHRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	req.Username = p.ByName("user")
	return auth.AuthenticateSSHUser(r.Context(), req)
}

type upsertUserRawReq struct {
	User json.RawMessage `json:"user"`
}

func (s *APIServer) upsertUser(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req *upsertUserRawReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := services.UnmarshalUser(req.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := services.ValidateUserRoles(r.Context(), user, auth); err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = auth.UpsertUser(r.Context(), user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("'%v' user upserted", user.GetName())), nil
}

func rawMessage(data []byte, err error) (interface{}, error) {
	if err != nil {
		return nil, trace.Wrap(err)
	}
	m := json.RawMessage(data)
	return &m, nil
}

func (s *APIServer) registerUsingToken(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, _ httprouter.Params, version string) (interface{}, error) {
	var req types.RegisterUsingTokenRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	certs, err := auth.RegisterUsingToken(r.Context(), &req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return certs, nil
}

// TODO(Joerger): DELETE IN 16.0.0, migrated to gRPC.
func (s *APIServer) rotateCertAuthority(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req types.RotateRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.RotateCertAuthority(r.Context(), req); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

type rotateExternalCertAuthorityRawReq struct {
	CA json.RawMessage `json:"ca"`
}

// TODO(Joerger): DELETE IN v16.0.0, migrated to gRPC
func (s *APIServer) rotateExternalCertAuthority(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req rotateExternalCertAuthorityRawReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := services.UnmarshalCertAuthority(req.CA)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.RotateExternalCertAuthority(r.Context(), ca); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
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
	Req GithubAuthRequest `json:"req"`
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
			response.Session, services.WithVersion(version), services.PreserveResourceID())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		raw.Session = rawSession
	}
	raw.HostSigners = make([]json.RawMessage, len(response.HostSigners))
	for i, ca := range response.HostSigners {
		data, err := services.MarshalCertAuthority(
			ca, services.WithVersion(version), services.PreserveResourceID())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		raw.HostSigners[i] = data
	}
	return &raw, nil
}

// HTTP GET /:version/events?query
//
// Query fields:
//
//		'from'  : time filter in RFC3339 format
//		'to'    : time filter in RFC3339 format
//	 ...     : other fields are passed directly to the audit backend
func (s *APIServer) searchEvents(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var err error
	to := time.Now().In(time.UTC)
	from := to.AddDate(0, -1, 0) // one month ago
	query := r.URL.Query()
	// parse 'to' and 'from' params:
	fromStr := query.Get("from")
	if fromStr != "" {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return nil, trace.BadParameter("from")
		}
	}
	toStr := query.Get("to")
	if toStr != "" {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			return nil, trace.BadParameter("to")
		}
	}
	var limit int
	limitStr := query.Get("limit")
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			return nil, trace.BadParameter("failed to parse limit: %q", limit)
		}
	}

	eventTypes := query[events.EventType]
	eventsList, _, err := auth.SearchEvents(r.Context(), events.SearchEventsRequest{
		From:       from,
		To:         to,
		EventTypes: eventTypes,
		Limit:      limit,
		Order:      types.EventOrderDescending,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return eventsList, nil
}

// searchSessionEvents only allows searching audit log for events related to session playback.
func (s *APIServer) searchSessionEvents(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var err error

	// default values for "to" and "from" fields
	to := time.Now().In(time.UTC) // now
	from := to.AddDate(0, -1, 0)  // one month ago

	// parse query for "to" and "from"
	query := r.URL.Query()
	fromStr := query.Get("from")
	if fromStr != "" {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return nil, trace.BadParameter("from")
		}
	}
	toStr := query.Get("to")
	if toStr != "" {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			return nil, trace.BadParameter("to")
		}
	}
	var limit int
	limitStr := query.Get("limit")
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			return nil, trace.BadParameter("failed to parse limit: %q", limit)
		}
	}
	// only pull back start and end events to build list of completed sessions
	eventsList, _, err := auth.SearchSessionEvents(r.Context(), events.SearchSessionEventsRequest{
		From:  from,
		To:    to,
		Limit: limit,
		Order: types.EventOrderDescending,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return eventsList, nil
}

// HTTP GET /:version/sessions/:id/stream?offset=x&bytes=y
// Query parameters:
//
//	"offset"   : bytes from the beginning
//	"bytes"    : number of bytes to read (it won't return more than 512Kb)
func (s *APIServer) getSessionChunk(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	sid, err := session.ParseID(p.ByName("id"))
	if err != nil {
		return nil, trace.BadParameter("missing parameter id")
	}
	namespace := p.ByName("namespace")
	if !types.IsValidNamespace(namespace) {
		return nil, trace.BadParameter("invalid namespace %q", namespace)
	}

	// "offset bytes" query param
	offsetBytes, err := strconv.Atoi(r.URL.Query().Get("offset"))
	if err != nil || offsetBytes < 0 {
		offsetBytes = 0
	}
	// "max bytes" query param
	max, err := strconv.Atoi(r.URL.Query().Get("bytes"))
	if err != nil || offsetBytes < 0 {
		offsetBytes = 0
	}
	log.Debugf("apiserver.GetSessionChunk(%v, %v, offset=%d)", namespace, *sid, offsetBytes)
	w.Header().Set("Content-Type", "text/plain")

	buffer, err := auth.GetSessionChunk(namespace, *sid, offsetBytes, max)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if _, err = w.Write(buffer); err != nil {
		return nil, trace.Wrap(err)
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	return nil, nil
}

// HTTP GET /:version/sessions/:id/events?maxage=n
// Query:
//
//	'after' : cursor value to return events newer than N. Defaults to 0, (return all)
func (s *APIServer) getSessionEvents(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	sid, err := session.ParseID(p.ByName("id"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	namespace := p.ByName("namespace")
	if !types.IsValidNamespace(namespace) {
		return nil, trace.BadParameter("invalid namespace %q", namespace)
	}
	afterN, err := strconv.Atoi(r.URL.Query().Get("after"))
	if err != nil {
		afterN = 0
	}

	return auth.GetSessionEvents(namespace, *sid, afterN)
}

type upsertNamespaceReq struct {
	Namespace types.Namespace `json:"namespace"`
}

func (s *APIServer) upsertNamespace(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req *upsertNamespaceReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.UpsertNamespace(req.Namespace); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

func (s *APIServer) getNamespaces(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	namespaces, err := auth.GetNamespaces()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return namespaces, nil
}

func (s *APIServer) getNamespace(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	name := p.ByName("namespace")
	if !types.IsValidNamespace(name) {
		return nil, trace.BadParameter("invalid namespace %q", name)
	}

	namespace, err := auth.GetNamespace(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return namespace, nil
}

func (s *APIServer) deleteNamespace(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	name := p.ByName("namespace")
	if !types.IsValidNamespace(name) {
		return nil, trace.BadParameter("invalid namespace %q", name)
	}

	err := auth.DeleteNamespace(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

func (s *APIServer) getClusterName(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	cn, err := auth.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rawMessage(services.MarshalClusterName(cn, services.WithVersion(version), services.PreserveResourceID()))
}

type setClusterNameReq struct {
	ClusterName json.RawMessage `json:"cluster_name"`
}

func (s *APIServer) setClusterName(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req setClusterNameReq

	err := httplib.ReadJSON(r, &req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cn, err := services.UnmarshalClusterName(req.ClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = auth.SetClusterName(cn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return message(fmt.Sprintf("cluster name set: %+v", cn)), nil
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
		data, err := services.MarshalTunnelConnection(conn, services.WithVersion(version), services.PreserveResourceID())
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
		data, err := services.MarshalTunnelConnection(conn, services.WithVersion(version), services.PreserveResourceID())
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

// getRemoteClusters returns a list of remote clusters
func (s *APIServer) getRemoteClusters(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	clusters, err := auth.GetRemoteClusters(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	items := make([]json.RawMessage, len(clusters))
	for i, cluster := range clusters {
		data, err := services.MarshalRemoteCluster(cluster, services.WithVersion(version), services.PreserveResourceID())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		items[i] = data
	}
	return items, nil
}

// getRemoteCluster returns a remote cluster by name
func (s *APIServer) getRemoteCluster(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	cluster, err := auth.GetRemoteCluster(r.Context(), p.ByName("cluster"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rawMessage(services.MarshalRemoteCluster(cluster, services.WithVersion(version), services.PreserveResourceID()))
}

// deleteRemoteCluster deletes remote cluster by name
func (s *APIServer) deleteRemoteCluster(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	err := auth.DeleteRemoteCluster(r.Context(), p.ByName("cluster"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

func (s *APIServer) processKubeCSR(auth *ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req KubeCSR

	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	re, err := auth.ProcessKubeCSR(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return re, nil
}

func message(msg string) map[string]interface{} {
	return map[string]interface{}{"message": msg}
}
