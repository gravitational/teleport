package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/form"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/session"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/julienschmidt/httprouter"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/gravitational/teleport/backend"
)

type Config struct {
	Backend backend.Backend
	Addr    string
}

// APISrv implements http API server for authority
type APIServer struct {
	httprouter.Router
	s *AuthServer
}

func NewAPIServer(s *AuthServer) *APIServer {
	srv := &APIServer{
		s: s,
	}
	srv.Router = *httprouter.New()

	// Auth is for operations involving CA
	srv.POST("/v1/ca/host/keys", srv.resetHostCA)
	srv.POST("/v1/ca/user/keys", srv.resetUserCA)

	// Retrieving authority public keys
	srv.GET("/v1/ca/host/keys/pub", srv.getHostCAPub)
	srv.GET("/v1/ca/user/keys/pub", srv.getUserCAPub)

	// Generating certificates for user and host authorities
	srv.POST("/v1/ca/host/certs", srv.generateHostCert)
	srv.POST("/v1/ca/user/certs", srv.generateUserCert)

	// Operations on users
	srv.GET("/v1/users", srv.getUsers)
	srv.DELETE("/v1/users/:user", srv.deleteUser)

	// Operations on user keys
	srv.POST("/v1/users/:user/keys", srv.upsertUserKey)
	srv.DELETE("/v1/users/:user/keys/:key", srv.deleteUserKey)
	srv.GET("/v1/users/:user/keys", srv.getUserKeys)

	// Generating keypairs
	srv.POST("/v1/keypair", srv.generateKeyPair)

	// Passwords and sessions
	srv.POST("/v1/users/:user/web/password", srv.upsertPassword)
	srv.POST("/v1/users/:user/web/password/check", srv.checkPassword)
	srv.POST("/v1/users/:user/web/signin", srv.signIn)
	srv.GET("/v1/users/:user/web/sessions/:sid", srv.getWebSession)
	srv.DELETE("/v1/users/:user/web/sessions/:sid", srv.deleteWebSession)

	// Web tunnels
	srv.POST("/v1/tunnels/web", srv.upsertWebTun)
	srv.GET("/v1/tunnels/web", srv.getWebTuns)
	srv.GET("/v1/tunnels/web/:prefix", srv.getWebTun)
	srv.DELETE("/v1/tunnels/web/:prefix", srv.deleteWebTun)

	// Servers and presense heartbeat
	srv.POST("/v1/servers", srv.upsertServer)
	srv.GET("/v1/servers", srv.getServers)

	return srv
}

func (s *APIServer) upsertServer(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var id, addr string
	var ttl time.Duration

	err := form.Parse(r,
		form.String("id", &id, form.Required()),
		form.String("addr", &addr, form.Required()),
		form.Duration("ttl", &ttl),
	)
	if err != nil {
		replyErr(w, err)
		return
	}
	if err := s.s.UpsertServer(backend.Server{ID: id, Addr: addr}, ttl); err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, message("server upserted"))
}

func (s *APIServer) getServers(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	servers, err := s.s.GetServers()
	if err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, serversResponse{Servers: servers})
}

func (s *APIServer) upsertWebTun(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var prefix, targetAddr, proxyAddr string
	var ttl time.Duration

	err := form.Parse(r,
		form.String("prefix", &prefix, form.Required()),
		form.String("target", &targetAddr, form.Required()),
		form.String("proxy", &proxyAddr, form.Required()),
		form.Duration("ttl", &ttl),
	)
	if err != nil {
		replyErr(w, err)
		return
	}
	t, err := backend.NewWebTun(prefix, proxyAddr, targetAddr)
	if err != nil {
		replyErr(w, err)
		return
	}
	if err := s.s.UpsertWebTun(*t, ttl); err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, &webTunResponse{Tunnel: *t})
}

func (s *APIServer) deleteWebTun(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	prefix := p[0].Value
	err := s.s.DeleteWebTun(prefix)
	if err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, message(fmt.Sprintf("web tunnel '%v' deleted", prefix)))
}

func (s *APIServer) getWebTun(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	prefix := p[0].Value
	wt, err := s.s.GetWebTun(prefix)
	if err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, &webTunResponse{Tunnel: *wt})
}

func (s *APIServer) getWebTuns(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ws, err := s.s.GetWebTuns()
	if err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, &webTunsResponse{Tunnels: ws})
}

func (s *APIServer) deleteWebSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	user, sid := p[0].Value, p[1].Value
	err := s.s.DeleteWebSession(user, session.SecureID(sid))
	if err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, message(fmt.Sprintf("session '%v' for user '%v' deleted", sid, user)))
}

func (s *APIServer) getWebSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	user, sid := p[0].Value, p[1].Value
	ws, err := s.s.GetWebSession(user, session.SecureID(sid))
	if err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, &sessionResponse{SID: string(ws.SID)})
}

func (s *APIServer) signIn(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var pass string

	err := form.Parse(r,
		form.String("password", &pass, form.Required()))
	if err != nil {
		replyErr(w, err)
		return
	}
	user := p[0].Value
	ws, err := s.s.SignIn(user, []byte(pass))
	if err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, &sessionResponse{SID: string(ws.SID)})
}

func (s *APIServer) upsertPassword(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var pass string

	err := form.Parse(r,
		form.String("password", &pass, form.Required()))
	if err != nil {
		replyErr(w, err)
		return
	}
	user := p[0].Value
	if err := s.s.UpsertPassword(user, []byte(pass)); err != nil {
		replyErr(w, err)
		return
	}

	reply(w, http.StatusOK, message(fmt.Sprintf("'%v' user password updated successfully", user)))
}

func (s *APIServer) checkPassword(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var pass string

	err := form.Parse(r,
		form.String("password", &pass, form.Required()))
	if err != nil {
		replyErr(w, err)
		return
	}
	user := p[0].Value
	if err := s.s.CheckPassword(user, []byte(pass)); err != nil {
		replyErr(w, err)
		return
	}

	reply(w, http.StatusOK, message(fmt.Sprintf("'%v' user password matches", user)))
}

func (s *APIServer) upsertUserKey(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var id, key string
	var ttl time.Duration

	err := form.Parse(r,
		form.String("key", &key, form.Required()),
		form.String("id", &id, form.Required()),
		form.Duration("ttl", &ttl))
	if err != nil {
		replyErr(w, err)
		return
	}
	cert, err := s.s.UpsertUserKey(p[0].Value, backend.AuthorizedKey{ID: id, Value: []byte(key)}, ttl)
	if err != nil {
		replyErr(w, err)
		return
	}

	reply(w, http.StatusOK, certResponse{Cert: string(cert)})
}

func (s *APIServer) getUsers(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	users, err := s.s.GetUsers()
	if err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, &usersResponse{Users: users})
}

func (s *APIServer) deleteUser(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	user := p[0].Value
	if err := s.s.DeleteUser(user); err != nil {
		replyErr(w, err)
		return
	}

	reply(w, http.StatusOK, message(fmt.Sprintf("user '%v' deleted", user)))
}

func (s *APIServer) getUserKeys(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	keys, err := s.s.GetUserKeys(p[0].Value)
	if err != nil {
		replyErr(w, err)
		return
	}

	reply(w, http.StatusOK, &pubKeysResponse{PubKeys: keys})
}

func (s *APIServer) deleteUserKey(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	user, keyID := p[0].Value, p[1].Value
	if err := s.s.DeleteUserKey(user, keyID); err != nil {
		replyErr(w, err)
		return
	}

	reply(w, http.StatusOK, message(fmt.Sprintf("key '%v' deleted for user '%v'", keyID, user)))
}

func (s *APIServer) generateKeyPair(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var pass string

	if err := form.Parse(r, form.String("pass", &pass)); err != nil {
		replyErr(w, err)
		return
	}

	priv, pub, err := s.s.GenerateKeyPair(pass)
	if err != nil {
		replyErr(w, err)
		return
	}

	reply(w, http.StatusOK, &keyPairResponse{PrivKey: priv, PubKey: string(pub)})
}

func (s *APIServer) resetHostCA(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var pass string

	if err := form.Parse(r, form.String("pass", &pass)); err != nil {
		replyErr(w, err)
		return
	}
	if err := s.s.ResetHostCA(pass); err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, message("host CA regenerated"))
}

func (s *APIServer) resetUserCA(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var pass string

	if err := form.Parse(r, form.String("pass", &pass)); err != nil {
		replyErr(w, err)
		return
	}

	if err := s.s.ResetUserCA(pass); err != nil {
		replyErr(w, err)
		return
	}

	reply(w, http.StatusOK, message("user CA regenerated"))
}

func (s *APIServer) getHostCAPub(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	bytes, err := s.s.GetHostCAPub()
	if err != nil {
		replyErr(w, err)
		return
	}

	reply(w, http.StatusOK, pubKeyResponse{PubKey: string(bytes)})
}

func (s *APIServer) getUserCAPub(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	bytes, err := s.s.GetUserCAPub()
	if err != nil {
		replyErr(w, err)
		return
	}

	reply(w, http.StatusOK, pubKeyResponse{PubKey: string(bytes)})
}

func (s *APIServer) generateHostCert(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var id, hostname, key string
	var ttl time.Duration

	err := form.Parse(r,
		form.String("key", &key, form.Required()),
		form.String("id", &id, form.Required()),
		form.String("hostname", &hostname, form.Required()),
		form.Duration("ttl", &ttl))

	if err != nil {
		reply(w, http.StatusBadRequest, err.Error())
	}

	cert, err := s.s.GenerateHostCert([]byte(key), id, hostname, ttl)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, certResponse{Cert: string(cert)})
}

func (s *APIServer) generateUserCert(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var id, user, key string
	var ttl time.Duration

	err := form.Parse(r,
		form.String("key", &key, form.Required()),
		form.String("id", &id, form.Required()),
		form.String("user", &user, form.Required()),
		form.Duration("ttl", &ttl))

	if err != nil {
		reply(w, http.StatusBadRequest, err.Error())
	}
	cert, err := s.s.GenerateUserCert([]byte(key), id, user, ttl)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, certResponse{Cert: string(cert)})
}

func replyErr(w http.ResponseWriter, e error) {
	switch err := e.(type) {
	case *backend.NotFoundError:
		reply(w, http.StatusNotFound, message(err.Error()))
		return
	case *backend.MissingParameterError, *BadParameterError, *form.MissingParameterError, *form.BadParameterError:
		reply(w, http.StatusBadRequest, message(err.Error()))
		return
	}
	log.Errorf("Unexpected error: %v", e)
	// do not leak the unexpected error to the callee as we are not sure
	// if it's safe to disclose that information
	reply(w, http.StatusInternalServerError, message("internal server error"))
}

func reply(w http.ResponseWriter, code int, message interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	out, err := json.Marshal(message)
	if err != nil {
		out = []byte(`{"msg": "internal marshal error"}`)
	}
	w.Write(out)
}

type pubKeyResponse struct {
	PubKey string `json:"pubkey"`
}

type pubKeysResponse struct {
	PubKeys []backend.AuthorizedKey `json:"pubkeys"`
}

type certResponse struct {
	Cert string `json:"cert"`
}

type usersResponse struct {
	Users []string `json:"users"`
}

type keyPairResponse struct {
	PrivKey []byte `json:"privkey"`
	PubKey  string `json:"pubkey"`
}

type sessionResponse struct {
	SID string `json:"sid"`
}

type webTunResponse struct {
	Tunnel backend.WebTun `json:"tunnel"`
}

type webTunsResponse struct {
	Tunnels []backend.WebTun `json:"tunnels"`
}

type serversResponse struct {
	Servers []backend.Server `json:"servers"`
}

func message(msg string) map[string]interface{} {
	return map[string]interface{}{"message": msg}
}
