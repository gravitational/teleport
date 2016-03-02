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
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
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
	se   session.SessionServer
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
	srv.DELETE("/v1/users/:user", httplib.MakeHandler(srv.deleteUser))

	// Generating keypairs
	srv.POST("/v1/keypair", httplib.MakeHandler(srv.generateKeyPair))

	// Passwords and sessions
	srv.POST("/v1/users/:user/web/password", httplib.MakeHandler(srv.upsertPassword))
	srv.POST("/v1/users/:user/web/password/check", httplib.MakeHandler(srv.checkPassword))
	srv.POST("/v1/users/:user/web/signin", httplib.MakeHandler(srv.signIn))
	srv.POST("/v1/users/:user/web/sessions", httplib.MakeHandler(srv.createWebSession))
	srv.GET("/v1/users/:user/web/sessions/:sid", httplib.MakeHandler(srv.getWebSession))
	srv.GET("/v1/users/:user/web/sessions", httplib.MakeHandler(srv.getWebSessions))
	srv.DELETE("/v1/users/:user/web/sessions/:sid", httplib.MakeHandler(srv.deleteWebSession))
	srv.GET("/v1/signuptokens/:token", httplib.MakeHandler(srv.getSignupTokenData))
	srv.POST("/v1/signuptokens/users", httplib.MakeHandler(srv.createUserWithToken))
	srv.POST("/v1/signuptokens", httplib.MakeHandler(srv.createSignupToken))

	// Servers and presence heartbeat
	srv.POST("/v1/servers", httplib.MakeHandler(srv.upsertServer))
	srv.GET("/v1/servers", httplib.MakeHandler(srv.getServers))
	srv.GET("/v1/auth/servers", httplib.MakeHandler(srv.getAuthServers))

	// Tokens
	srv.POST("/v1/tokens", httplib.MakeHandler(srv.generateToken))
	srv.POST("/v1/tokens/register", httplib.MakeHandler(srv.registerUsingToken))
	srv.POST("/v1/tokens/register/auth", httplib.MakeHandler(srv.registerNewAuthServer))

	// Events
	srv.POST("/v1/events", httplib.MakeHandler(srv.submitEvents))
	srv.GET("/v1/events", httplib.MakeHandler(srv.getEvents))

	// Recorded sessions
	srv.POST("/v1/records/:sid/chunks", httplib.MakeHandler(srv.submitChunks))
	srv.GET("/v1/records/:sid/chunks", httplib.MakeHandler(srv.getChunks))

	// Sesssions
	srv.POST("/v1/sessions/:id/parties", httplib.MakeHandler(srv.upsertSessionParty))
	srv.POST("/v1/sessions", httplib.MakeHandler(srv.upsertSession))
	srv.GET("/v1/sessions", httplib.MakeHandler(srv.getSessions))
	srv.GET("/v1/sessions/:id", httplib.MakeHandler(srv.getSession))
	srv.DELETE("/v1/sessions/:id", httplib.MakeHandler(srv.deleteSession))

	// Backend Keys
	srv.GET("/v1/backend/keys", httplib.MakeHandler(srv.getSealKeys))
	srv.GET("/v1/backend/keys/:id", httplib.MakeHandler(srv.getSealKey))
	srv.DELETE("/v1/backend/keys/:id", httplib.MakeHandler(srv.deleteSealKey))
	srv.POST("/v1/backend/keys", httplib.MakeHandler(srv.addSealKey))
	srv.POST("/v1/backend/generatekey", httplib.MakeHandler(srv.generateSealKey))

	return srv
}

func (s *APIServer) getSealKeys(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	keys, err := s.a.GetSealKeys()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keys, nil
}

func (s *APIServer) getSealKey(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	id := p[0].Value
	key, err := s.a.GetSealKey(id)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return key, nil
}

func (s *APIServer) deleteSealKey(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	id := p[0].Value
	err := s.a.DeleteSealKey(id)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return id, nil
}

// addSealKeyReq is a request to add seal key
type addSealKeyReq struct {
	Key encryptor.Key `json:"key"`
}

func (s *APIServer) addSealKey(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req addSealKeyReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.a.AddSealKey(req.Key); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// genSealKeyReq is a request to generate new seal key
type generateSealKeyReq struct {
	Name string `json:"name"`
}

func (s *APIServer) generateSealKey(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req generateSealKeyReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	key, err := s.a.GenerateSealKey(req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return key, nil
}

type upsertServerReq struct {
	Server services.Server `json:"server"`
	TTL    time.Duration   `json:"ttl"`
}

func (s *APIServer) upsertServer(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req upsertServerReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	log.Debugf("[AUTH API] upsertServer. RemoteAddr=%v", r.RemoteAddr)

	// if server sent "local" IP address to us, replace it with the address taken from
	// the connection
	req.Server.Addr = utils.ReplaceLocalhost(req.Server.Addr, r.RemoteAddr)
	// also, Server.ID right now is always set to nonsense, overwrite it with it's Addr:
	req.Server.ID = req.Server.Addr

	if err := s.a.UpsertServer(req.Server, req.TTL); err != nil {
		log.Error(err)
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

func (s *APIServer) getServers(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	servers, err := s.a.GetServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return servers, nil
}

func (s *APIServer) getAuthServers(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	servers, err := s.a.GetAuthServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return servers, nil
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

func (s *APIServer) getWebSessions(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	user := p[0].Value
	keys, err := s.a.GetWebSessionsKeys(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keys, nil
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
	Domain string        `json:"domain"`
	Role   teleport.Role `json:"role"`
	TTL    time.Duration `json:"ttl"`
}

func (s *APIServer) generateToken(w http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	var req *generateTokenReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	token, err := s.a.GenerateToken(req.Domain, req.Role, req.TTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return string(token), nil
}

type registerUsingTokenReq struct {
	Domain string        `json:"domain"`
	Role   teleport.Role `json:"role"`
	Token  string        `json:"token"`
}

func (s *APIServer) registerUsingToken(w http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	var req *registerUsingTokenReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	keys, err := s.a.RegisterUsingToken(req.Token, req.Domain, req.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keys, nil
}

type registerNewAuthServerReq struct {
	Domain string        `json:"domain"`
	Token  string        `json:"token"`
	Key    encryptor.Key `json:"key"`
}

func (s *APIServer) registerNewAuthServer(w http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	var req *registerNewAuthServerReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	key, err := s.a.RegisterNewAuthServer(req.Domain, req.Token, req.Key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return key, nil
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
	certs, err := s.a.GetCertAuthorities(services.CertAuthType(p[0].Value))
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

type upsertSessionReq struct {
	ID  string        `json:"id"`
	TTL time.Duration `json:"ttl"`
}

func (s *APIServer) upsertSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *upsertSessionReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.a.UpsertSession(req.ID, req.TTL); err != nil {
		return nil, trace.Wrap(err)
	}
	return req.ID, nil
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

	sid := p[0].Value
	if err := s.a.UpsertParty(sid, req.Party, req.TTL); err != nil {
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
	sid := p[0].Value
	se, err := s.a.GetSession(sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return se, nil
}

func (s *APIServer) deleteSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	sid := p[0].Value
	if err := s.a.DeleteSession(sid); err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("session %v was deleted", sid)), nil
}

type getSignupTokenDataResponse struct {
	User            string   `json:"user"`
	QRImg           []byte   `json:"qrimg"`
	HotpFirstValues []string `json:"hotp_first_values"`
}

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
	User          string   `json:"user"`
	AllowedLogins []string `json:"allowed_logins"`
}

func (s *APIServer) createSignupToken(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *createSignupTokenReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	token, err := s.a.CreateSignupToken(req.User, req.AllowedLogins)
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

func message(msg string) map[string]interface{} {
	return map[string]interface{}{"message": msg}
}
