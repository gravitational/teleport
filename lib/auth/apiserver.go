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
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/recorder"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/codahale/lunk"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

// Config is APIServer config
type Config struct {
	Backend backend.Backend
	Addr    string
}

// APIServer implements http API server for AuthServer interface
type APIServer struct {
	httprouter.Router
	a    *AuthWithRoles
	s    *AuthServer
	elog events.Log
	se   session.Service
	rec  recorder.Recorder
}

// NewAPIServer returns a new instance of APIServer HTTP handler
func NewAPIServer(a *AuthWithRoles) *APIServer {
	srv := &APIServer{
		a: a,
	}
	srv.Router = *httprouter.New()

	// Operations on certificate authorities
	srv.GET("/v1/domain", httplib.MakeHandler(srv.getLocalDomain))
	srv.POST("/v1/authorities/:type", httplib.MakeHandler(srv.upsertCertAuthority))
	srv.DELETE("/v1/authorities/:type/:domain", httplib.MakeHandler(srv.deleteCertAuthority))
	srv.GET("/v1/authorities/:type", httplib.MakeHandler(srv.getCertAuthorities))

	// Generating certificates for user and host authorities
	srv.POST("/v1/ca/host/certs", httplib.MakeHandler(srv.generateHostCert))
	srv.POST("/v1/ca/user/certs", httplib.MakeHandler(srv.generateUserCert))

	// Operations on users
	srv.GET("/v1/users", httplib.MakeHandler(srv.getUsers))
	srv.GET("/v1/users/:user", httplib.MakeHandler(srv.getUser))
	srv.DELETE("/v1/users/:user", httplib.MakeHandler(srv.deleteUser))

	// Generating keypairs
	srv.POST("/v1/keypair", httplib.MakeHandler(srv.generateKeyPair))

	// Passwords and sessions
	srv.POST("/v1/users", httplib.MakeHandler(srv.upsertUser))
	srv.POST("/v1/users/:user/web/password", httplib.MakeHandler(srv.upsertPassword))
	srv.POST("/v1/users/:user/web/password/check", httplib.MakeHandler(srv.checkPassword))
	srv.POST("/v1/users/:user/web/signin", httplib.MakeHandler(srv.signIn))
	srv.POST("/v1/users/:user/web/sessions", httplib.MakeHandler(srv.createWebSession))
	srv.GET("/v1/users/:user/web/sessions/:sid", httplib.MakeHandler(srv.getWebSession))
	srv.DELETE("/v1/users/:user/web/sessions/:sid", httplib.MakeHandler(srv.deleteWebSession))
	srv.GET("/v1/signuptokens/:token", httplib.MakeHandler(srv.getSignupTokenData))
	srv.POST("/v1/signuptokens/users", httplib.MakeHandler(srv.createUserWithToken))
	srv.POST("/v1/signuptokens", httplib.MakeHandler(srv.createSignupToken))

	// Servers and presence heartbeat
	srv.POST("/v1/nodes", httplib.MakeHandler(srv.upsertNode))
	srv.GET("/v1/nodes", httplib.MakeHandler(srv.getNodes))
	srv.POST("/v1/authservers", httplib.MakeHandler(srv.upsertAuthServer))
	srv.GET("/v1/authservers", httplib.MakeHandler(srv.getAuthServers))
	srv.POST("/v1/proxies", httplib.MakeHandler(srv.upsertProxy))
	srv.GET("/v1/proxies", httplib.MakeHandler(srv.getProxies))

	// Reverse tunnels
	srv.POST("/v1/reversetunnels", httplib.MakeHandler(srv.upsertReverseTunnel))
	srv.GET("/v1/reversetunnels", httplib.MakeHandler(srv.getReverseTunnels))
	srv.DELETE("/v1/reversetunnels/:domain", httplib.MakeHandler(srv.deleteReverseTunnel))

	// Tokens
	srv.POST("/v1/tokens", httplib.MakeHandler(srv.generateToken))
	srv.POST("/v1/tokens/register", httplib.MakeHandler(srv.registerUsingToken))
	srv.POST("/v1/tokens/register/auth", httplib.MakeHandler(srv.registerNewAuthServer))

	// Events
	srv.POST("/v1/events", httplib.MakeHandler(srv.submitEvents))
	srv.GET("/v1/events", httplib.MakeHandler(srv.getEvents))

	srv.POST("/v1/events/sessions", httplib.MakeHandler(srv.logSessionEvents))
	srv.GET("/v1/events/sessions", httplib.MakeHandler(srv.getSessionEvents))

	// Recorded sessions
	srv.POST("/v1/records/:sid/chunks", httplib.MakeHandler(srv.submitChunks))
	srv.GET("/v1/records/:sid/chunks", httplib.MakeHandler(srv.getChunks))
	srv.GET("/v1/records/:sid/chunkscount", httplib.MakeHandler(srv.getChunksCount))

	// Sesssions
	srv.POST("/v1/sessions/:id/parties", httplib.MakeHandler(srv.upsertSessionParty))
	srv.POST("/v1/sessions", httplib.MakeHandler(srv.createSession))
	srv.PUT("/v1/sessions/:id", httplib.MakeHandler(srv.updateSession))
	srv.GET("/v1/sessions", httplib.MakeHandler(srv.getSessions))
	srv.GET("/v1/sessions/:id", httplib.MakeHandler(srv.getSession))

	// OIDC stuff
	srv.POST("/v1/oidc/connectors", httplib.MakeHandler(srv.upsertOIDCConnector))
	srv.GET("/v1/oidc/connectors", httplib.MakeHandler(srv.getOIDCConnectors))
	srv.GET("/v1/oidc/connectors/:id", httplib.MakeHandler(srv.getOIDCConnector))
	srv.DELETE("/v1/oidc/connectors/:id", httplib.MakeHandler(srv.deleteOIDCConnector))
	srv.POST("/v1/oidc/requests/create", httplib.MakeHandler(srv.createOIDCAuthRequest))
	srv.POST("/v1/oidc/requests/validate", httplib.MakeHandler(srv.validateOIDCAuthCallback))

	return srv
}

type upsertServerReq struct {
	Server services.Server `json:"server"`
	TTL    time.Duration   `json:"ttl"`
}

// upsertNode is called by remote SSH nodes when they ping back into the auth service
func (s *APIServer) upsertServer(role teleport.Role, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req upsertServerReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	log.Debugf("[AUTH API] ping from %v %v (%v) at %v", role, req.Server.ID, req.Server.Hostname, r.RemoteAddr)

	// if server sent "local" IP address to us, replace the ip/host part with the remote address we see
	// on the socket, but keep the original port:
	req.Server.Addr = utils.ReplaceLocalhost(req.Server.Addr, r.RemoteAddr)

	switch role {
	case teleport.RoleNode:
		if err := s.a.UpsertNode(req.Server, req.TTL); err != nil {
			return nil, trace.Wrap(err)
		}
	case teleport.RoleAuth:
		if err := s.a.UpsertAuthServer(req.Server, req.TTL); err != nil {
			return nil, trace.Wrap(err)
		}
	case teleport.RoleProxy:
		if err := s.a.UpsertProxy(req.Server, req.TTL); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return message("ok"), nil
}

// upsertNode is called by remote SSH nodes when they ping back into the auth service
func (s *APIServer) upsertNode(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	return s.upsertServer(teleport.RoleNode, w, r, p)
}

// getNodes returns registered SSH nodes
func (s *APIServer) getNodes(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	servers, err := s.a.GetNodes()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return servers, nil
}

// upsertProxy is called by remote SSH nodes when they ping back into the auth service
func (s *APIServer) upsertProxy(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	return s.upsertServer(teleport.RoleProxy, w, r, p)
}

// getProxies returns registered proxies
func (s *APIServer) getProxies(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	servers, err := s.a.GetProxies()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return servers, nil
}

// upsertAuthServer is called by remote Auth servers when they ping back into the auth service
func (s *APIServer) upsertAuthServer(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	return s.upsertServer(teleport.RoleAuth, w, r, p)
}

// getAuthServers returns registered auth servers
func (s *APIServer) getAuthServers(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	servers, err := s.a.GetAuthServers()
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
func (s *APIServer) upsertReverseTunnel(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req upsertReverseTunnelReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.a.UpsertReverseTunnel(req.ReverseTunnel, req.TTL); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// getReverseTunnels returns a list of reverse tunnels
func (s *APIServer) getReverseTunnels(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	reverseTunnels, err := s.a.GetReverseTunnels()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return reverseTunnels, nil
}

// deleteReverseTunnel deletes reverse tunnel
func (s *APIServer) deleteReverseTunnel(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	domainName := p[0].Value
	err := s.a.DeleteReverseTunnel(domainName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("reverse tunnel %v deleted", domainName)), nil
}

func (s *APIServer) deleteWebSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	user, sid := p[0].Value, p[1].Value
	err := s.a.DeleteWebSession(user, sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("session '%v' for user '%v' deleted", sid, user)), nil
}

func (s *APIServer) getWebSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	user, sid := p[0].Value, p[1].Value
	sess, err := s.a.GetWebSessionInfo(user, sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sess, nil
}

type signInReq struct {
	Password string `json:"password"`
}

func (s *APIServer) signIn(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *signInReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	user := p[0].Value
	sess, err := s.a.SignIn(user, []byte(req.Password))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sess, nil
}

type createWebSessionReq struct {
	PrevSessionID string `json:"prev_session_id"`
}

func (s *APIServer) createWebSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *createWebSessionReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	user := p[0].Value
	sess, err := s.a.CreateWebSession(user, req.PrevSessionID)
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

func (s *APIServer) upsertPassword(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *upsertPasswordReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	user := p[0].Value
	hotpURL, hotpQR, err := s.a.UpsertPassword(user, []byte(req.Password))
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

func (s *APIServer) upsertUser(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *upsertUserReqRaw
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := services.GetUserUnmarshaler()(req.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = s.a.UpsertUser(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("'%v' user upserted", user.GetName())), nil
}

type checkPasswordReq struct {
	Password  string `json:"password"`
	HOTPToken string `json:"hotp_token"`
}

func (s *APIServer) checkPassword(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req checkPasswordReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	user := p[0].Value
	if err := s.a.CheckPassword(user, []byte(req.Password), req.HOTPToken); err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("'%v' user password matches", user)), nil
}

func (s *APIServer) getUser(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	user, err := s.a.GetUser(p[0].Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return user, nil
}

func (s *APIServer) getUsers(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	users, err := s.a.GetUsers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return users, nil
}

func (s *APIServer) deleteUser(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	user := p[0].Value
	if err := s.a.DeleteUser(user); err != nil {
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

func (s *APIServer) generateKeyPair(w http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	var req *generateKeyPairReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	priv, pub, err := s.a.GenerateKeyPair(req.Password)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &generateKeyPairResponse{PrivKey: priv, PubKey: string(pub)}, nil
}

type generateHostCertReq struct {
	Key        []byte        `json:"key"`
	Hostname   string        `json:"hostname"`
	AuthDomain string        `json:"auth_domain"`
	Role       teleport.Role `json:"role"`
	TTL        time.Duration `json:"ttl"`
}

func (s *APIServer) generateHostCert(w http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	var req *generateHostCertReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := s.a.GenerateHostCert(req.Key, req.Hostname, req.AuthDomain, req.Role, req.TTL)
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

func (s *APIServer) generateUserCert(w http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	var req *generateUserCertReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		log.Errorf("failed parsing JSON request. %v", err)
		return nil, trace.Wrap(err)
	}
	cert, err := s.a.GenerateUserCert(req.Key, req.User, req.TTL)
	if err != nil {
		log.Error(err)
		return nil, trace.Wrap(err)
	}
	return string(cert), nil
}

type generateTokenReq struct {
	Role teleport.Role `json:"role"`
	TTL  time.Duration `json:"ttl"`
}

func (s *APIServer) generateToken(w http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	var req *generateTokenReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	token, err := s.a.GenerateToken(req.Role, req.TTL)
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

func (s *APIServer) registerUsingToken(w http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	var req *registerUsingTokenReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	keys, err := s.a.RegisterUsingToken(req.Token, req.HostID, req.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keys, nil
}

type registerNewAuthServerReq struct {
	Token string `json:"token"`
}

func (s *APIServer) registerNewAuthServer(w http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	var req *registerNewAuthServerReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	err := s.a.RegisterNewAuthServer(req.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

type submitEventsReq struct {
	Events []lunk.Entry `json:"events"`
}

func (s *APIServer) submitEvents(w http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	var req *submitEventsReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	for _, e := range req.Events {
		if err := s.a.LogEntry(e); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return message("events submitted"), nil
}

type logSessionsReq struct {
	Sessions []session.Session `json:"sessions"`
}

func (s *APIServer) logSessionEvents(w http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	var req *logSessionsReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	for i := range req.Sessions {
		if err := s.a.LogSession(req.Sessions[i]); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return message("session events submitted"), nil
}

func (s *APIServer) getEvents(w http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	f, err := events.FilterFromURL(r.URL.Query())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	events, err := s.a.GetEvents(*f)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return events, nil
}

func (s *APIServer) getSessionEvents(w http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	f, err := events.FilterFromURL(r.URL.Query())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sessionEvents, err := s.a.GetSessionEvents(*f)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sessionEvents, nil
}

type writeChunksReq struct {
	Chunks []recorder.Chunk `json:"chunk"`
}

func (s *APIServer) submitChunks(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	sid := p[0].Value
	var req *writeChunksReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	wr, err := s.a.GetChunkWriter(sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer wr.Close()

	if err := wr.WriteChunks(req.Chunks); err != nil {
		return nil, trace.Wrap(err)
	}

	return message("chunks submitted"), nil
}

func (s *APIServer) getChunks(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	sid := p[0].Value

	st, en := r.URL.Query().Get("start"), r.URL.Query().Get("end")
	start, err := strconv.Atoi(st)
	if err != nil {
		return nil, trace.Wrap(teleport.BadParameter("start", "need integer"))
	}
	end, err := strconv.Atoi(en)
	if err != nil {
		return nil, trace.Wrap(teleport.BadParameter("end", "need integer"))
	}

	re, err := s.a.GetChunkReader(sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer re.Close()

	chunks, err := re.ReadChunks(start, end)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return chunks, nil
}

func (s *APIServer) getChunksCount(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	sid := p[0].Value

	re, err := s.a.GetChunkReader(sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer re.Close()

	count, err := re.GetChunksCount()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return count, nil
}

type upsertCertAuthorityReq struct {
	CA  services.CertAuthority `json:"ca"`
	TTL time.Duration          `json:"ttl"`
}

func (s *APIServer) upsertCertAuthority(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *upsertCertAuthorityReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.a.UpsertCertAuthority(req.CA, req.TTL); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

func (s *APIServer) getCertAuthorities(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	loadKeys, _, err := httplib.ParseBool(r.URL.Query(), "load_keys")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certs, err := s.a.GetCertAuthorities(services.CertAuthType(p[0].Value), loadKeys)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return certs, nil
}

func (s *APIServer) getLocalDomain(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	domain, err := s.a.GetLocalDomain()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return domain, nil
}

func (s *APIServer) deleteCertAuthority(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	id := services.CertAuthID{
		DomainName: p[1].Value,
		Type:       services.CertAuthType(p[0].Value),
	}
	if err := s.a.DeleteCertAuthority(id); err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("cert '%v' deleted", id)), nil
}

type createSessionReq struct {
	Session session.Session `json:"session"`
}

func (s *APIServer) createSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *createSessionReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.a.CreateSession(req.Session); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

type updateSessionReq struct {
	Update session.UpdateRequest `json:"update"`
}

func (s *APIServer) updateSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *updateSessionReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.a.UpdateSession(req.Update); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

type upsertPartyReq struct {
	Party session.Party `json:"party"`
	TTL   time.Duration `json:"ttl"`
}

func (s *APIServer) upsertSessionParty(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *upsertPartyReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	sid, err := session.ParseID(p[0].Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.a.UpsertParty(*sid, req.Party, req.TTL); err != nil {
		return nil, trace.Wrap(err)
	}
	return req.Party, nil
}

func (s *APIServer) getSessions(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	sessions, err := s.a.GetSessions()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sessions, nil
}

func (s *APIServer) getSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	sid, err := session.ParseID(p[0].Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	se, err := s.a.GetSession(*sid)
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
func (s *APIServer) getSignupTokenData(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	token := p[0].Value

	user, QRImg, hotpFirstValues, err := s.a.GetSignupTokenData(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &getSignupTokenDataResponse{
		User:            user,
		QRImg:           QRImg,
		HotpFirstValues: hotpFirstValues,
	}, nil
}

type createSignupTokenReq struct {
	User services.User `json:"user"`
}

type createSignupTokenReqRaw struct {
	User json.RawMessage `json:"user"`
}

func (s *APIServer) createSignupToken(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *createSignupTokenReqRaw
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := services.GetUserUnmarshaler()(req.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	token, err := s.a.CreateSignupToken(user)
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

func (s *APIServer) createUserWithToken(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *createUserWithTokenReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	sess, err := s.a.CreateUserWithToken(req.Token, req.Password, req.HOTPToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sess, nil
}

type upsertOIDCConnectorReq struct {
	Connector services.OIDCConnector `json:"connector"`
	TTL       time.Duration          `json:"ttl"`
}

func (s *APIServer) upsertOIDCConnector(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *upsertOIDCConnectorReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	err := s.a.UpsertOIDCConnector(req.Connector, req.TTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

func (s *APIServer) getOIDCConnector(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	withSecrets, _, err := httplib.ParseBool(r.URL.Query(), "with_secrets")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connector, err := s.a.GetOIDCConnector(p[0].Value, withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return connector, nil
}

func (s *APIServer) deleteOIDCConnector(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	err := s.a.DeleteOIDCConnector(p[0].Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

func (s *APIServer) getOIDCConnectors(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	withSecrets, _, err := httplib.ParseBool(r.URL.Query(), "with_secrets")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connectors, err := s.a.GetOIDCConnectors(withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return connectors, nil
}

type createOIDCAuthRequestReq struct {
	Req services.OIDCAuthRequest `json:"req"`
}

func (s *APIServer) createOIDCAuthRequest(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *createOIDCAuthRequestReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := s.a.CreateOIDCAuthRequest(req.Req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return response, nil
}

type validateOIDCAuthCallbackReq struct {
	Query url.Values `json:"query"`
}

func (s *APIServer) validateOIDCAuthCallback(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *validateOIDCAuthCallbackReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := s.a.ValidateOIDCAuthCallback(req.Query)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return response, nil
}

func message(msg string) map[string]interface{} {
	return map[string]interface{}{"message": msg}
}
