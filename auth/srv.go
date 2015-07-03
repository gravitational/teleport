package auth

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/form"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/memlog"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/roundtrip"
	websession "github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/session"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/julienschmidt/httprouter"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/teleport/session"
)

type Config struct {
	Backend backend.Backend
	Addr    string
}

// APISrv implements http API server for authority
type APIServer struct {
	httprouter.Router
	s    *AuthServer
	elog memlog.Logger
	se   session.SessionServer
}

func NewAPIServer(s *AuthServer, elog memlog.Logger, se session.SessionServer) *APIServer {
	srv := &APIServer{
		s:    s,
		elog: elog,
		se:   se,
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

	// Operations on remote authorities we trust
	srv.POST("/v1/ca/remote/:type/hosts/:fqdn", srv.upsertRemoteCert)
	srv.DELETE("/v1/ca/remote/:type/hosts/:fqdn/:id", srv.deleteRemoteCert)
	srv.GET("/v1/ca/remote/:type", srv.getRemoteCerts)

	// Passwords and sessions
	srv.POST("/v1/users/:user/web/password", srv.upsertPassword)
	srv.POST("/v1/users/:user/web/password/check", srv.checkPassword)
	srv.POST("/v1/users/:user/web/signin", srv.signIn)
	srv.GET("/v1/users/:user/web/sessions/:sid", srv.getWebSession)
	srv.GET("/v1/users/:user/web/sessions", srv.getWebSessions)
	srv.DELETE("/v1/users/:user/web/sessions/:sid", srv.deleteWebSession)

	// Web tunnels
	srv.POST("/v1/tunnels/web", srv.upsertWebTun)
	srv.GET("/v1/tunnels/web", srv.getWebTuns)
	srv.GET("/v1/tunnels/web/:prefix", srv.getWebTun)
	srv.DELETE("/v1/tunnels/web/:prefix", srv.deleteWebTun)

	// Servers and presense heartbeat
	srv.POST("/v1/servers", srv.upsertServer)
	srv.GET("/v1/servers", srv.getServers)

	// Tokens
	srv.POST("/v1/tokens", srv.generateToken)

	// Events
	srv.POST("/v1/events", srv.submitEvents)
	srv.GET("/v1/events", srv.getEvents)

	// Sesssions
	srv.POST("/v1/sessions/:id/parties", srv.upsertSessionParty)
	srv.POST("/v1/sessions", srv.upsertSession)
	srv.GET("/v1/sessions", srv.getSessions)
	srv.GET("/v1/sessions/:id", srv.getSession)
	srv.DELETE("/v1/sessions/:id", srv.deleteSession)

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
	err := s.s.DeleteWebSession(user, websession.SecureID(sid))
	if err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, message(fmt.Sprintf("session '%v' for user '%v' deleted", sid, user)))
}

func (s *APIServer) getWebSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	user, sid := p[0].Value, p[1].Value
	ws, err := s.s.GetWebSession(user, websession.SecureID(sid))
	if err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, &webSessionResponse{SID: string(ws.SID)})
}

func (s *APIServer) getWebSessions(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	user := p[0].Value
	keys, err := s.s.GetWebSessionsKeys(user)
	if err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, &webSessionsResponse{Keys: keys})
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
	reply(w, http.StatusOK, &webSessionResponse{SID: string(ws.SID)})
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

func (s *APIServer) generateToken(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var fqdn string
	var ttl time.Duration

	err := form.Parse(r,
		form.String("fqdn", &fqdn, form.Required()),
		form.Duration("ttl", &ttl))

	if err != nil {
		reply(w, http.StatusBadRequest, err.Error())
	}
	token, err := s.s.GenerateToken(fqdn, ttl)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, tokenResponse{Token: string(token)})
}

func (s *APIServer) submitEvents(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var events form.Files

	err := form.Parse(r,
		form.FileSlice("event", &events))
	if err != nil {
		reply(w, http.StatusBadRequest, err.Error())
	}

	if len(events) == 0 {
		reply(w, http.StatusBadRequest,
			fmt.Errorf("at least one event is required"))
	}

	defer func() {
		// don't let get the error get lost
		if err := events.Close(); err != nil {
			log.Errorf("failed to close files: %v", err)
		}
	}()

	// submit events
	for _, e := range events {
		data, err := ioutil.ReadAll(e)
		if err != nil {
			log.Errorf("failed to read event: %v", err)
			reply(w, http.StatusBadRequest, fmt.Errorf("failed to read event"))
			return
		}
		if _, err := s.elog.Write(data); err != nil {
			log.Errorf("failed to write event: %v", err)
			reply(w, http.StatusInternalServerError,
				fmt.Errorf("failed to write event"))
			return
		}
	}

	reply(w, http.StatusOK, message("events submitted"))
}

func (s *APIServer) getEvents(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	roundtrip.ReplyJSON(
		w, http.StatusOK, &eventsResponse{Events: s.elog.LastEvents()})
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

func (s *APIServer) upsertRemoteCert(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var id, key string
	var ttl time.Duration

	ctype, fqdn := p[0].Value, p[1].Value

	err := form.Parse(r,
		form.String("key", &key, form.Required()),
		form.String("id", &id, form.Required()),
		form.Duration("ttl", &ttl))
	if err != nil {
		replyErr(w, err)
		return
	}
	cert := backend.RemoteCert{ID: id, Value: []byte(key), FQDN: fqdn, Type: ctype}
	if err := s.s.UpsertRemoteCert(cert, ttl); err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, remoteCertResponse{RemoteCert: cert})
}

func (s *APIServer) getRemoteCerts(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	fqdn := r.URL.Query().Get("fqdn")
	ctype := p[0].Value

	certs, err := s.s.GetRemoteCerts(ctype, fqdn)
	if err != nil {
		fmt.Printf("error: %v", err)
		log.Infof("error: %v", err)
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, &remoteCertsResponse{RemoteCerts: certs})
}

func (s *APIServer) deleteRemoteCert(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctype, fqdn, id := p[0].Value, p[1].Value, p[2].Value
	if err := s.s.DeleteRemoteCert(ctype, fqdn, id); err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, message(fmt.Sprintf("cert '%v' deleted", id)))
}

func (s *APIServer) upsertSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var sid string
	var ttl time.Duration

	err := form.Parse(r,
		form.String("id", &sid, form.Required()),
		form.Duration("ttl", &ttl))
	if err != nil {
		replyErr(w, err)
		return
	}
	if err := s.se.UpsertSession(sid, ttl); err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, sessionResponse{Session: session.Session{ID: sid}})
}

func (s *APIServer) upsertSessionParty(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var party session.Party

	sid := p[0].Value
	var ttl time.Duration

	err := form.Parse(r,
		form.String("id", &party.ID, form.Required()),
		form.String("site", &party.Site, form.Required()),
		form.String("user", &party.User, form.Required()),
		form.String("server_addr", &party.ServerAddr, form.Required()),
		form.Time("last_active", &party.LastActive),
		form.Duration("ttl", &ttl),
	)
	if err != nil {
		replyErr(w, err)
		return
	}
	if err := s.se.UpsertParty(sid, party, ttl); err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, partyResponse{Party: party})
}

func (s *APIServer) getSessions(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	sessions, err := s.se.GetSessions()
	if err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, &sessionsResponse{Sessions: sessions})
}

func (s *APIServer) getSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	sid := p[0].Value
	se, err := s.se.GetSession(sid)
	if err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, &sessionResponse{Session: *se})
}

func (s *APIServer) deleteSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	sid := p[0].Value
	if err := s.se.DeleteSession(sid); err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, message(fmt.Sprintf("session %v was deleted", sid)))
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

type remoteCertResponse struct {
	RemoteCert backend.RemoteCert `hson:"remote_cert"`
}

type remoteCertsResponse struct {
	RemoteCerts []backend.RemoteCert `hson:"remote_certs"`
}

type usersResponse struct {
	Users []string `json:"users"`
}

type keyPairResponse struct {
	PrivKey []byte `json:"privkey"`
	PubKey  string `json:"pubkey"`
}

type webSessionResponse struct {
	SID string `json:"sid"`
}

type webSessionsResponse struct {
	Keys []backend.AuthorizedKey `json:"keys"`
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

type tokenResponse struct {
	Token string `json:"token"`
}

type eventsResponse struct {
	Events []interface{} `json:"events"`
}

type partyResponse struct {
	Party session.Party `json:"party"`
}

type sessionsResponse struct {
	Sessions []session.Session `json:"sessions"`
}

type sessionResponse struct {
	Session session.Session `json:"session"`
}

func message(msg string) map[string]interface{} {
	return map[string]interface{}{"message": msg}
}
