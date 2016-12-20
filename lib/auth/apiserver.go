/*
Copyright 2015 Gravitational, Inc.

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

package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/tstranex/u2f"
)

type APIConfig struct {
	AuthServer     *AuthServer
	SessionService session.Service
	AuditLog       events.IAuditLog
	NewChecker     NewChecker
}

// APIServer implements http API server for AuthServer interface
type APIServer struct {
	APIConfig
	httprouter.Router
}

// NewAPIServer returns a new instance of APIServer HTTP handler
func NewAPIServer(config *APIConfig) http.Handler {
	srv := APIServer{
		APIConfig: *config,
	}
	srv.Router = *httprouter.New()

	// Operations on certificate authorities
	srv.GET("/v1/domain", srv.withAuth(srv.getDomainName))
	srv.POST("/v1/authorities/:type", srv.withAuth(srv.upsertCertAuthority))
	srv.DELETE("/v1/authorities/:type/:domain", srv.withAuth(srv.deleteCertAuthority))
	srv.GET("/v1/authorities/:type/:domain", srv.withAuth(srv.getCertAuthority))
	srv.GET("/v1/authorities/:type", srv.withAuth(srv.getCertAuthorities))

	// Generating certificates for user and host authorities
	srv.POST("/v1/ca/host/certs", srv.withAuth(srv.generateHostCert))
	srv.POST("/v1/ca/user/certs", srv.withAuth(srv.generateUserCert))

	// Operations on users
	srv.GET("/v1/users", srv.withAuth(srv.getUsers))
	srv.GET("/v1/users/:user", srv.withAuth(srv.getUser))
	srv.DELETE("/v1/users/:user", srv.withAuth(srv.deleteUser))

	// Generating keypairs
	srv.POST("/v1/keypair", srv.withAuth(srv.generateKeyPair))

	// Passwords and sessions
	srv.POST("/v1/users", srv.withAuth(srv.upsertUser))
	srv.POST("/v1/users/:user/web/password", srv.withAuth(srv.upsertPassword))
	srv.POST("/v1/users/:user/web/password/check", srv.withAuth(srv.checkPassword))
	srv.POST("/v1/users/:user/web/signin", srv.withAuth(srv.signIn))
	srv.GET("/v1/users/:user/web/signin/preauth", srv.withAuth(srv.preAuthenticatedSignIn))
	srv.POST("/v1/users/:user/web/sessions", srv.withAuth(srv.createWebSession))
	srv.GET("/v1/users/:user/web/sessions/:sid", srv.withAuth(srv.getWebSession))
	srv.DELETE("/v1/users/:user/web/sessions/:sid", srv.withAuth(srv.deleteWebSession))
	srv.GET("/v1/signuptokens/:token", srv.withAuth(srv.getSignupTokenData))
	srv.POST("/v1/signuptokens/users", srv.withAuth(srv.createUserWithToken))
	srv.POST("/v1/signuptokens", srv.withAuth(srv.createSignupToken))

	// Servers and presence heartbeat
	srv.POST("/v1/namespaces/:namespace/nodes", srv.withAuth(srv.upsertNode))
	srv.GET("/v1/namespaces/:namespace/nodes", srv.withAuth(srv.getNodes))
	srv.POST("/v1/authservers", srv.withAuth(srv.upsertAuthServer))
	srv.GET("/v1/authservers", srv.withAuth(srv.getAuthServers))
	srv.POST("/v1/proxies", srv.withAuth(srv.upsertProxy))
	srv.GET("/v1/proxies", srv.withAuth(srv.getProxies))

	// Reverse tunnels
	srv.POST("/v1/reversetunnels", srv.withAuth(srv.upsertReverseTunnel))
	srv.GET("/v1/reversetunnels", srv.withAuth(srv.getReverseTunnels))
	srv.DELETE("/v1/reversetunnels/:domain", srv.withAuth(srv.deleteReverseTunnel))

	// Tokens
	srv.POST("/v1/tokens", srv.withAuth(srv.generateToken))
	srv.POST("/v1/tokens/register", srv.withAuth(srv.registerUsingToken))
	srv.POST("/v1/tokens/register/auth", srv.withAuth(srv.registerNewAuthServer))

	// Sesssions
	srv.POST("/v1/namespaces/:namespace/sessions", srv.withAuth(srv.createSession))
	srv.PUT("/v1/namespaces/:namespace/sessions/:id", srv.withAuth(srv.updateSession))
	srv.GET("/v1/namespaces/:namespace/sessions", srv.withAuth(srv.getSessions))
	srv.GET("/v1/namespaces/:namespace/sessions/:id", srv.withAuth(srv.getSession))
	srv.POST("/v1/namespaces/:namespace/sessions/:id/stream", srv.withAuth(srv.postSessionChunk))
	srv.GET("/v1/namespaces/:namespace/sessions/:id/stream", srv.withAuth(srv.getSessionChunk))
	srv.GET("/v1/namespaces/:namespace/sessions/:id/events", srv.withAuth(srv.getSessionEvents))

	// OIDC stuff
	srv.POST("/v1/oidc/connectors", srv.withAuth(srv.upsertOIDCConnector))
	srv.GET("/v1/oidc/connectors", srv.withAuth(srv.getOIDCConnectors))
	srv.GET("/v1/oidc/connectors/:id", srv.withAuth(srv.getOIDCConnector))
	srv.DELETE("/v1/oidc/connectors/:id", srv.withAuth(srv.deleteOIDCConnector))
	srv.POST("/v1/oidc/requests/create", srv.withAuth(srv.createOIDCAuthRequest))
	srv.POST("/v1/oidc/requests/validate", srv.withAuth(srv.validateOIDCAuthCallback))

	// Namespaces
	srv.POST("/v1/namespaces", srv.withAuth(srv.upsertNamespace))
	srv.GET("/v1/namespaces", srv.withAuth(srv.getNamespaces))
	srv.GET("/v1/namespaces/:namespace", srv.withAuth(srv.getNamespace))
	srv.DELETE("/v1/namespaces/:namespace", srv.withAuth(srv.deleteNamespace))

	// Roles
	srv.POST("/v1/roles", srv.withAuth(srv.upsertRole))
	srv.GET("/v1/roles", srv.withAuth(srv.getRoles))
	srv.GET("/v1/roles/:role", srv.withAuth(srv.getRole))
	srv.DELETE("/v1/roles/:role", srv.withAuth(srv.deleteRole))

	// U2F stuff
	srv.GET("/v1/u2f/signuptokens/:token", srv.withAuth(srv.getSignupU2FRegisterRequest))
	srv.POST("/v1/u2f/users", srv.withAuth(srv.createUserWithU2FToken))
	srv.POST("/v1/u2f/users/:user/sign", srv.withAuth(srv.u2fSignRequest))
	srv.GET("/v1/u2f/appid", srv.withAuth(srv.getU2FAppID))

	// Provisioning tokens
	srv.GET("/v1/tokens", srv.withAuth(srv.getTokens))
	srv.GET("/v1/tokens/:token", srv.withAuth(srv.getToken))
	srv.DELETE("/v1/tokens/:token", srv.withAuth(srv.deleteToken))

	// Audit logs AKA events
	srv.POST("/v1/events", srv.withAuth(srv.emitAuditEvent))
	srv.GET("/v1/events", srv.withAuth(srv.searchEvents))

	return httplib.RewritePaths(&srv.Router,
		httplib.Rewrite("/v1/nodes", "/v1/namespaces/default/nodes"),
		httplib.Rewrite("/v1/sessions", "/v1/namespaces/default/sessions"),
		httplib.Rewrite("/v1/sessions/([^/]+)/(.*)", "/v1/namespaces/default/sessions/$1/$2"),
	)
}

// HandlerWithAuthFunc is http handler with passed auth context
type HandlerWithAuthFunc func(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error)

func (s *APIServer) withAuth(handler HandlerWithAuthFunc) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		// SSH-to-HTTP gateway (tun server) sets HTTP basic auth to SSH cert principal
		// This allows us to make sure that users can only request new certificates
		// only for themselves, except admin users
		caller, _, ok := r.BasicAuth()
		if !ok {
			return nil, trace.AccessDenied("missing username or password")
		}
		checker, err := s.NewChecker(caller)
		if err != nil {
			log.Debugf("failed to create checker: %v for %v", err, caller)
			return nil, trace.AccessDenied("missing username or password")
		}
		auth := &AuthWithRoles{
			authServer: s.AuthServer,
			user:       caller,
			checker:    checker,
			sessions:   s.SessionService,
			alog:       s.AuditLog,
		}
		return handler(auth, w, r, p)
	})
}

type upsertServerReq struct {
	Server services.Server `json:"server"`
	TTL    time.Duration   `json:"ttl"`
}

// upsertServer is a common utility function
func (s *APIServer) upsertServer(auth ClientI, role teleport.Role, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req upsertServerReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	// if server sent "local" IP address to us, replace the ip/host part with the remote address we see
	// on the socket, but keep the original port:
	req.Server.Addr = utils.ReplaceLocalhost(req.Server.Addr, r.RemoteAddr)

	switch role {
	case teleport.RoleNode:
		req.Server.Namespace = p.ByName("namespace")
		if err := auth.UpsertNode(req.Server, req.TTL); err != nil {
			return nil, trace.Wrap(err)
		}
	case teleport.RoleAuth:
		if err := auth.UpsertAuthServer(req.Server, req.TTL); err != nil {
			return nil, trace.Wrap(err)
		}
	case teleport.RoleProxy:
		if err := auth.UpsertProxy(req.Server, req.TTL); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return message("ok"), nil
}

// upsertNode is called by remote SSH nodes when they ping back into the auth service
func (s *APIServer) upsertNode(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	return s.upsertServer(auth, teleport.RoleNode, w, r, p)
}

// getNodes returns registered SSH nodes
func (s *APIServer) getNodes(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	servers, err := auth.GetNodes(p.ByName("namespace"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return servers, nil
}

// upsertProxy is called by remote SSH nodes when they ping back into the auth service
func (s *APIServer) upsertProxy(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	return s.upsertServer(auth, teleport.RoleProxy, w, r, p)
}

// getProxies returns registered proxies
func (s *APIServer) getProxies(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	servers, err := auth.GetProxies()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return servers, nil
}

// upsertAuthServer is called by remote Auth servers when they ping back into the auth service
func (s *APIServer) upsertAuthServer(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	return s.upsertServer(auth, teleport.RoleAuth, w, r, p)
}

// getAuthServers returns registered auth servers
func (s *APIServer) getAuthServers(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	servers, err := auth.GetAuthServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return servers, nil
}

type upsertReverseTunnelReq struct {
	ReverseTunnel services.ReverseTunnel `json:"reverse_tunnel"`
	TTL           time.Duration          `json:"ttl"`
}

// upsertReverseTunnel is called by admin to create a reverse tunnel to remote proxy
func (s *APIServer) upsertReverseTunnel(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req upsertReverseTunnelReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.UpsertReverseTunnel(req.ReverseTunnel, req.TTL); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// getReverseTunnels returns a list of reverse tunnels
func (s *APIServer) getReverseTunnels(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	reverseTunnels, err := auth.GetReverseTunnels()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return reverseTunnels, nil
}

// deleteReverseTunnel deletes reverse tunnel
func (s *APIServer) deleteReverseTunnel(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	domainName := p[0].Value
	err := auth.DeleteReverseTunnel(domainName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("reverse tunnel %v deleted", domainName)), nil
}

// getTokens returns a list of active provisioning tokens. expired (inactive) tokens are not returned
func (s *APIServer) getTokens(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	tokens, err := auth.GetTokens()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tokens, nil
}

// getTokens returns provisioning token by name
func (s *APIServer) getToken(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	token, err := auth.GetToken(p.ByName("token"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return token, nil
}

// deleteToken deletes (revokes) a token by its value
func (s *APIServer) deleteToken(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	token := p.ByName("token")
	if err := auth.DeleteToken(token); err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("Token %v deleted", token)), nil
}

func (s *APIServer) deleteWebSession(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	user, sid := p[0].Value, p[1].Value
	err := auth.DeleteWebSession(user, sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("session '%v' for user '%v' deleted", sid, user)), nil
}

func (s *APIServer) getWebSession(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	user, sid := p[0].Value, p[1].Value
	sess, err := auth.GetWebSessionInfo(user, sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sess, nil
}

type signInReq struct {
	Password string `json:"password"`
}

func (s *APIServer) signIn(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *signInReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	user := p[0].Value
	sess, err := auth.SignIn(user, []byte(req.Password))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sess, nil
}

func (s *APIServer) preAuthenticatedSignIn(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	user := p[0].Value
	sess, err := auth.PreAuthenticatedSignIn(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sess, nil
}

func (s *APIServer) u2fSignRequest(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *signInReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	user := p[0].Value
	pass := []byte(req.Password)
	u2fSignReq, err := auth.GetU2FSignRequest(user, pass)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return u2fSignReq, nil
}

type createWebSessionReq struct {
	PrevSessionID string `json:"prev_session_id"`
}

func (s *APIServer) createWebSession(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *createWebSessionReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	user := p[0].Value
	if req.PrevSessionID != "" {
		sess, err := auth.ExtendWebSession(user, req.PrevSessionID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return sess, nil
	}
	sess, err := auth.CreateWebSession(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sess, nil
}

type upsertPasswordReq struct {
	Password string `json:"password"`
}

type upsertPasswordResponse struct {
	HotpURL string `json:"hotp_url"`
	HotpQR  []byte `json:"hotp_qr"`
}

func (s *APIServer) upsertPassword(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *upsertPasswordReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	user := p[0].Value
	hotpURL, hotpQR, err := auth.UpsertPassword(user, []byte(req.Password))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &upsertPasswordResponse{HotpURL: hotpURL, HotpQR: hotpQR}, nil
}

type upsertUserReq struct {
	User services.User `json:"user"`
}

type upsertUserReqRaw struct {
	User json.RawMessage `json:"user"`
}

func (s *APIServer) upsertUser(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *upsertUserReqRaw
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := services.GetUserMarshaler().UnmarshalUser(req.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.UpsertUser(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("'%v' user upserted", user.GetName())), nil
}

type checkPasswordReq struct {
	Password  string `json:"password"`
	HOTPToken string `json:"hotp_token"`
}

func (s *APIServer) checkPassword(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req checkPasswordReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	user := p[0].Value
	if err := auth.CheckPassword(user, []byte(req.Password), req.HOTPToken); err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("'%v' user password matches", user)), nil
}

func (s *APIServer) getUser(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	user, err := auth.GetUser(p[0].Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return user, nil
}

func (s *APIServer) getUsers(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	users, err := auth.GetUsers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return users, nil
}

func (s *APIServer) deleteUser(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	user := p[0].Value
	if err := auth.DeleteUser(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("user '%v' deleted", user)), nil
}

type generateKeyPairReq struct {
	Password string `json:"password"`
}

type generateKeyPairResponse struct {
	PrivKey []byte `json:"privkey"`
	PubKey  string `json:"pubkey"`
}

func (s *APIServer) generateKeyPair(auth ClientI, w http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	var req *generateKeyPairReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	priv, pub, err := auth.GenerateKeyPair(req.Password)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &generateKeyPairResponse{PrivKey: priv, PubKey: string(pub)}, nil
}

type generateHostCertReq struct {
	Key        []byte         `json:"key"`
	Hostname   string         `json:"hostname"`
	AuthDomain string         `json:"auth_domain"`
	Roles      teleport.Roles `json:"roles"`
	TTL        time.Duration  `json:"ttl"`
}

func (s *APIServer) generateHostCert(auth ClientI, w http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	var req *generateHostCertReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := auth.GenerateHostCert(req.Key, req.Hostname, req.AuthDomain, req.Roles, req.TTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return string(cert), nil
}

type generateUserCertReq struct {
	Key  []byte        `json:"key"`
	User string        `json:"user"`
	TTL  time.Duration `json:"ttl"`
}

func (s *APIServer) generateUserCert(auth ClientI, w http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	var req *generateUserCertReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := auth.GenerateUserCert(req.Key, req.User, req.TTL)
	if err != nil {
		log.Error(trace.DebugReport(err))
		return nil, trace.Wrap(err)
	}
	return string(cert), nil
}

type generateTokenReq struct {
	Roles teleport.Roles `json:"roles"`
	TTL   time.Duration  `json:"ttl"`
}

func (s *APIServer) generateToken(auth ClientI, w http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	var req *generateTokenReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	token, err := auth.GenerateToken(req.Roles, req.TTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return string(token), nil
}

type registerUsingTokenReq struct {
	HostID string        `json:"hostID"`
	Role   teleport.Role `json:"role"`
	Token  string        `json:"token"`
}

func (s *APIServer) registerUsingToken(auth ClientI, w http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	var req *registerUsingTokenReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	keys, err := auth.RegisterUsingToken(req.Token, req.HostID, req.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keys, nil
}

type registerNewAuthServerReq struct {
	Token string `json:"token"`
}

func (s *APIServer) registerNewAuthServer(auth ClientI, w http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	var req *registerNewAuthServerReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	err := auth.RegisterNewAuthServer(req.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

type upsertCertAuthorityReq struct {
	CA  services.CertAuthority `json:"ca"`
	TTL time.Duration          `json:"ttl"`
}

func (s *APIServer) upsertCertAuthority(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *upsertCertAuthorityReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.UpsertCertAuthority(req.CA, req.TTL); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

func (s *APIServer) getCertAuthorities(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	loadKeys, _, err := httplib.ParseBool(r.URL.Query(), "load_keys")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certs, err := auth.GetCertAuthorities(services.CertAuthType(p[0].Value), loadKeys)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return certs, nil
}

func (s *APIServer) getCertAuthority(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	loadKeys, _, err := httplib.ParseBool(r.URL.Query(), "load_keys")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	id := services.CertAuthID{
		Type:       services.CertAuthType(p.ByName("type")),
		DomainName: p.ByName("domain"),
	}
	ca, err := auth.GetCertAuthority(id, loadKeys)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ca, nil
}

func (s *APIServer) getDomainName(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	domain, err := auth.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return domain, nil
}

// getU2FAppID returns the U2F AppID in the auth configuration
func (s *APIServer) getU2FAppID(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	appID, err := auth.GetU2FAppID()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	w.Header().Set("Content-Type", "application/fido.trusted-apps+json")
	return appID, nil
}

func (s *APIServer) deleteCertAuthority(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	id := services.CertAuthID{
		DomainName: p[1].Value,
		Type:       services.CertAuthType(p[0].Value),
	}
	if err := auth.DeleteCertAuthority(id); err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("cert '%v' deleted", id)), nil
}

type createSessionReq struct {
	Session session.Session `json:"session"`
}

func (s *APIServer) createSession(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *createSessionReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	req.Session.Namespace = p.ByName("namespace")
	if err := auth.CreateSession(req.Session); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

type updateSessionReq struct {
	Update session.UpdateRequest `json:"update"`
}

func (s *APIServer) updateSession(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *updateSessionReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	req.Update.Namespace = p.ByName("namespace")
	if err := auth.UpdateSession(req.Update); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

func (s *APIServer) getSessions(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	sessions, err := auth.GetSessions(p.ByName("namespace"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sessions, nil
}

func (s *APIServer) getSession(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	sid, err := session.ParseID(p.ByName("id"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	se, err := auth.GetSession(p.ByName("namespace"), *sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return se, nil
}

type getSignupTokenDataResponse struct {
	User            string   `json:"user"`
	QRImg           []byte   `json:"qrimg"`
	HotpFirstValues []string `json:"hotp_first_values"`
}

// getSignupTokenData auth API method creates a new sign-up token for adding a new user
func (s *APIServer) getSignupTokenData(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	token := p[0].Value

	user, QRImg, hotpFirstValues, err := auth.GetSignupTokenData(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &getSignupTokenDataResponse{
		User:            user,
		QRImg:           QRImg,
		HotpFirstValues: hotpFirstValues,
	}, nil
}

func (s *APIServer) getSignupU2FRegisterRequest(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	token := p[0].Value
	u2fRegReq, err := auth.GetSignupU2FRegisterRequest(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return u2fRegReq, nil
}

type createSignupTokenReq struct {
	User services.User `json:"user"`
}

type createSignupTokenReqRaw struct {
	User json.RawMessage `json:"user"`
}

func (s *APIServer) createSignupToken(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *createSignupTokenReqRaw
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := services.GetUserMarshaler().UnmarshalUser(req.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	token, err := auth.CreateSignupToken(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return token, nil
}

type createUserWithTokenReq struct {
	Token     string `json:"token"`
	Password  string `json:"password"`
	HOTPToken string `json:"hotp_token"`
}

func (s *APIServer) createUserWithToken(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *createUserWithTokenReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	sess, err := auth.CreateUserWithToken(req.Token, req.Password, req.HOTPToken)
	if err != nil {
		log.Error(trace.DebugReport(err))
		return nil, trace.Wrap(err)
	}
	return sess, nil
}

type createUserWithU2FTokenReq struct {
	Token               string               `json:"token"`
	Password            string               `json:"password"`
	U2FRegisterResponse u2f.RegisterResponse `json:"u2f_register_response"`
}

func (s *APIServer) createUserWithU2FToken(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *createUserWithU2FTokenReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	sess, err := auth.CreateUserWithU2FToken(req.Token, req.Password, req.U2FRegisterResponse)
	if err != nil {
		log.Error(err)
		return nil, trace.Wrap(err)
	}
	return sess, nil
}

type upsertOIDCConnectorReq struct {
	Connector services.OIDCConnector `json:"connector"`
	TTL       time.Duration          `json:"ttl"`
}

func (s *APIServer) upsertOIDCConnector(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *upsertOIDCConnectorReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	err := auth.UpsertOIDCConnector(req.Connector, req.TTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

func (s *APIServer) getOIDCConnector(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	withSecrets, _, err := httplib.ParseBool(r.URL.Query(), "with_secrets")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connector, err := auth.GetOIDCConnector(p[0].Value, withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return connector, nil
}

func (s *APIServer) deleteOIDCConnector(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	err := auth.DeleteOIDCConnector(p[0].Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

func (s *APIServer) getOIDCConnectors(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	withSecrets, _, err := httplib.ParseBool(r.URL.Query(), "with_secrets")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connectors, err := auth.GetOIDCConnectors(withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return connectors, nil
}

type createOIDCAuthRequestReq struct {
	Req services.OIDCAuthRequest `json:"req"`
}

func (s *APIServer) createOIDCAuthRequest(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *createOIDCAuthRequestReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := auth.CreateOIDCAuthRequest(req.Req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return response, nil
}

type validateOIDCAuthCallbackReq struct {
	Query url.Values `json:"query"`
}

func (s *APIServer) validateOIDCAuthCallback(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *validateOIDCAuthCallbackReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := auth.ValidateOIDCAuthCallback(req.Query)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return response, nil
}

// HTTP GET /v1/events?query
//
// Query fields:
//	'from'  : time filter in RFC3339 format
//	'to'    : time filter in RFC3339 format
//  ...     : other fields are passed directly to the audit backend
func (s *APIServer) searchEvents(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
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
	// remove 'to' & 'from' fields, passing the rest of the query unmodified
	// to whatever pluggable search is implemented by the backend
	query.Del("to")
	query.Del("from")
	eventsList, err := auth.SearchEvents(from, to, query.Encode())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return eventsList, nil
}

type auditEventReq struct {
	Type   string             `json:"type"`
	Fields events.EventFields `json:"fields"`
}

// HTTP	POST /v1/events
func (s *APIServer) emitAuditEvent(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req auditEventReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.EmitAuditEvent(req.Type, req.Fields); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// HTTP POST /v1/sessions/:id/stream
func (s *APIServer) postSessionChunk(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	sid, err := session.ParseID(p.ByName("id"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	namespace := p.ByName("namespace")
	if err = auth.PostSessionChunk(namespace, *sid, r.Body); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// HTTP GET /v1/sessions/:id/stream?offset=x&bytes=y
// Query parameters:
//   "offset"   : bytes from the beginning
//   "bytes"    : number of bytes to read (it won't return more than 512Kb)
func (s *APIServer) getSessionChunk(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	sid, err := session.ParseID(p.ByName("id"))
	if err != nil {
		return nil, trace.BadParameter("missing parameter id")
	}
	namespace := p.ByName("namespace")

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

// HTTP GET /v1/sessions/:id/events?maxage=n
// Query:
//    'after' : cursor value to return events newer than N. Defaults to 0, (return all)
func (s *APIServer) getSessionEvents(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	sid, err := session.ParseID(p.ByName("id"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	namespace := p.ByName("namespace")
	afterN, err := strconv.Atoi(r.URL.Query().Get("after"))
	if err != nil {
		afterN = 0
	}
	log.Debugf("[AUTH] api.getSessionEvents(%v, after=%d)", *sid, afterN)
	return auth.GetSessionEvents(namespace, *sid, afterN)
}

type upsertNamespaceReq struct {
	Namespace services.Namespace `json:"namespace"`
}

func (s *APIServer) upsertNamespace(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *upsertNamespaceReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.UpsertNamespace(req.Namespace); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

func (s *APIServer) getNamespaces(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	namespaces, err := auth.GetNamespaces()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return namespaces, nil
}

func (s *APIServer) getNamespace(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	name := p.ByName("namespace")
	namespace, err := auth.GetNamespace(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return namespace, nil
}

func (s *APIServer) deleteNamespace(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	name := p.ByName("namespace")
	err := auth.DeleteNamespace(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

type upsertRoleReq struct {
	Role services.Role `json:"role"`
}

type upsertRoleReqRaw struct {
	Role json.RawMessage `json:"role"`
}

func (s *APIServer) upsertRole(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *upsertRoleReqRaw
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	role, err := services.GetRoleMarshaler().UnmarshalRole(req.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.UpsertRole(role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("'%v' role upserted", role.GetMetadata().Name)), nil
}

func (s *APIServer) getRole(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	role, err := auth.GetRole(p[0].Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return role, nil
}

func (s *APIServer) getRoles(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	roles, err := auth.GetRoles()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return roles, nil
}

func (s *APIServer) deleteRole(auth ClientI, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	role := p[0].Value
	if err := auth.DeleteRole(role); err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("role '%v' deleted", role)), nil
}

func message(msg string) map[string]interface{} {
	return map[string]interface{}{"message": msg}
}
