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
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/recorder"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"

	log "github.com/Sirupsen/logrus"
	"github.com/codahale/lunk"
	"github.com/gravitational/form"
	"github.com/gravitational/roundtrip"
	websession "github.com/gravitational/session"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/crypto/ssh"
)

type Config struct {
	Backend backend.Backend
	Addr    string
}

// APISrv implements http API server for authority
type APIServer struct {
	httprouter.Router
	a    *AuthWithRoles
	s    *AuthServer
	elog events.Log
	se   session.SessionServer
	rec  recorder.Recorder
}

func NewAPIServer(a *AuthWithRoles) *APIServer {
	srv := &APIServer{
		a: a,
	}
	srv.Router = *httprouter.New()

	// Auth is for operations involving Certificate Authority
	srv.POST("/v1/ca/host/keys", srv.resetHostCertificateAuthority)
	srv.POST("/v1/ca/user/keys", srv.resetUserCertificateAuthority)

	// Retrieving authority public keys
	srv.GET("/v1/ca/host/keys/pub", srv.getHostCertificateAuthority)
	srv.GET("/v1/ca/user/keys/pub", srv.getUserCertificateAuthority)

	// Generating certificates for user and host authorities
	srv.POST("/v1/ca/host/certs", srv.generateHostCert)
	srv.POST("/v1/ca/user/certs", srv.generateUserCert)

	// Operations on users
	srv.GET("/v1/users", srv.getUsers)
	srv.DELETE("/v1/users/:user", srv.deleteUser)

	// Generating keypairs
	srv.POST("/v1/keypair", srv.generateKeyPair)

	// Operations on remote authorities we trust
	srv.POST("/v1/ca/remote/:type/hosts/:domain", srv.upsertRemoteCertificate)
	srv.DELETE("/v1/ca/remote/:type/hosts/:domain/:id", srv.deleteRemoteCertificate)
	srv.GET("/v1/ca/remote/:type", srv.getRemoteCertificates)
	srv.GET("/v1/ca/trusted/:type", srv.getTrustedCertificates)
	srv.GET("/v1/ca/id/:type", srv.getCertificateID)

	// Passwords and sessions
	srv.POST("/v1/users/:user/web/password", srv.upsertPassword)
	srv.POST("/v1/users/:user/web/password/check", srv.checkPassword)
	srv.POST("/v1/users/:user/web/signin", srv.signIn)
	srv.GET("/v1/users/:user/web/sessions/:sid", srv.getWebSession)
	srv.GET("/v1/users/:user/web/sessions", srv.getWebSessions)
	srv.DELETE("/v1/users/:user/web/sessions/:sid", srv.deleteWebSession)
	srv.GET("/v1/signuptokens/:token", srv.getSignupTokenData)
	srv.POST("/v1/signuptokens/users", srv.createUserWithToken)
	srv.POST("/v1/signuptokens", srv.createSignupToken)

	// Web tunnels
	srv.POST("/v1/tunnels/web", srv.upsertWebTun)
	srv.GET("/v1/tunnels/web", srv.getWebTuns)
	srv.GET("/v1/tunnels/web/:prefix", srv.getWebTun)
	srv.DELETE("/v1/tunnels/web/:prefix", srv.deleteWebTun)

	// Servers and presence heartbeat
	srv.POST("/v1/servers", srv.upsertServer)
	srv.GET("/v1/servers", srv.getServers)

	// Tokens
	srv.POST("/v1/tokens", srv.generateToken)
	srv.POST("/v1/tokens/register", srv.registerUsingToken)
	srv.POST("/v1/tokens/register/auth", srv.registerNewAuthServer)
	// Events
	srv.POST("/v1/events", srv.submitEvents)
	srv.GET("/v1/events", srv.getEvents)

	// Recorded sessions
	srv.POST("/v1/records/:sid/chunks", srv.submitChunks)
	srv.GET("/v1/records/:sid/chunks", srv.getChunks)

	// Sesssions
	srv.POST("/v1/sessions/:id/parties", srv.upsertSessionParty)
	srv.POST("/v1/sessions", srv.upsertSession)
	srv.GET("/v1/sessions", srv.getSessions)
	srv.GET("/v1/sessions/:id", srv.getSession)
	srv.DELETE("/v1/sessions/:id", srv.deleteSession)

	// Backend Keys
	srv.GET("/v1/backend/keys", srv.getSealKeys)
	srv.GET("/v1/backend/keys/:id", srv.getSealKey)
	srv.DELETE("/v1/backend/keys/:id", srv.deleteSealKey)
	srv.POST("/v1/backend/keys", srv.addSealKey)
	srv.POST("/v1/backend/generatekey", srv.generateSealKey)

	// User mapping
	srv.POST("/v1/usermappings", srv.upsertUserMapping)
	srv.DELETE("/v1/usermappings/:id", srv.deleteUserMapping)
	srv.GET("/v1/usermappings", srv.getAllUserMapping)
	srv.GET("/v1/usermappings/:id", srv.userMappingExists)

	return srv
}

func (s *APIServer) getSealKeys(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	keys, err := s.a.GetSealKeys()
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, sealKeysResponse{Keys: keys})
}

func (s *APIServer) getSealKey(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	id := p[0].Value
	key, err := s.a.GetSealKey(id)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, sealKeyResponse{Key: key})
}

func (s *APIServer) deleteSealKey(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	id := p[0].Value
	err := s.a.DeleteSealKey(id)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, message("Key "+id+" was deleted"))
}

func (s *APIServer) addSealKey(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var keyJSON string
	err := form.Parse(r,
		form.String("key", &keyJSON, form.Required()),
	)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}

	var key encryptor.Key
	if err := json.Unmarshal([]byte(keyJSON), &key); err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}

	err = s.a.AddSealKey(key)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, message("ok"))

}

func (s *APIServer) generateSealKey(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var name string
	err := form.Parse(r,
		form.String("name", &name, form.Required()),
	)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}

	key, err := s.a.GenerateSealKey(name)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, sealKeyResponse{Key: key})
}

func (s *APIServer) upsertServer(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var args upsertServerArgs
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		replyErr(w, err)
		return
	}

	if err := s.a.UpsertServer(args.Server, args.TTL); err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, message("server upserted"))
}

func (s *APIServer) getServers(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	servers, err := s.a.GetServers()
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
	t, err := services.NewWebTun(prefix, proxyAddr, targetAddr)
	if err != nil {
		replyErr(w, err)
		return
	}
	if err := s.a.UpsertWebTun(*t, ttl); err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, &webTunResponse{Tunnel: *t})
}

func (s *APIServer) deleteWebTun(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	prefix := p[0].Value
	err := s.a.DeleteWebTun(prefix)
	if err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, message(fmt.Sprintf("web tunnel '%v' deleted", prefix)))
}

func (s *APIServer) getWebTun(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	prefix := p[0].Value
	wt, err := s.a.GetWebTun(prefix)
	if err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, &webTunResponse{Tunnel: *wt})
}

func (s *APIServer) getWebTuns(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ws, err := s.a.GetWebTuns()
	if err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, &webTunsResponse{Tunnels: ws})
}

func (s *APIServer) deleteWebSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	user, sid := p[0].Value, p[1].Value
	err := s.a.DeleteWebSession(user, websession.SecureID(sid))
	if err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, message(fmt.Sprintf("session '%v' for user '%v' deleted", sid, user)))
}

func (s *APIServer) getWebSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	user, sid := p[0].Value, p[1].Value
	ws, err := s.a.GetWebSession(user, websession.SecureID(sid))
	if err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, &webSessionResponse{SID: string(ws.SID)})
}

func (s *APIServer) getWebSessions(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	user := p[0].Value
	keys, err := s.a.GetWebSessionsKeys(user)
	if err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, &webSessionsResponse{Keys: keys})
}

func (s *APIServer) signIn(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var pass string

	err := form.Parse(r,
		form.String("password", &pass, form.Required()),
	)
	if err != nil {
		replyErr(w, err)
		return
	}
	user := p[0].Value
	ws, err := s.a.SignIn(user, []byte(pass))
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
	hotpURL, hotpQR, err := s.a.UpsertPassword(user, []byte(pass))
	if err != nil {
		replyErr(w, err)
		return
	}

	reply(w, http.StatusOK,
		&upsertPasswordResponse{HotpURL: hotpURL, HotpQR: hotpQR})
}

func (s *APIServer) checkPassword(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var pass string
	var hotpToken string

	err := form.Parse(r,
		form.String("password", &pass, form.Required()),
		form.String("hotpToken", &hotpToken, form.Required()),
	)
	if err != nil {
		replyErr(w, err)
		return
	}
	user := p[0].Value
	if err := s.a.CheckPassword(user, []byte(pass), hotpToken); err != nil {
		replyErr(w, err)
		return
	}

	reply(w, http.StatusOK, message(fmt.Sprintf("'%v' user password matches", user)))
}

func (s *APIServer) getUsers(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	users, err := s.a.GetUsers()
	if err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, &usersResponse{Users: users})
}

func (s *APIServer) deleteUser(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	user := p[0].Value
	if err := s.a.DeleteUser(user); err != nil {
		replyErr(w, err)
		return
	}

	reply(w, http.StatusOK, message(fmt.Sprintf("user '%v' deleted", user)))
}

func (s *APIServer) generateKeyPair(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var pass string

	if err := form.Parse(r, form.String("pass", &pass)); err != nil {
		replyErr(w, err)
		return
	}

	priv, pub, err := s.a.GenerateKeyPair(pass)
	if err != nil {
		replyErr(w, err)
		return
	}

	reply(w, http.StatusOK, &keyPairResponse{PrivKey: priv, PubKey: string(pub)})
}

func (s *APIServer) resetHostCertificateAuthority(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var pass string

	err := form.Parse(r, form.String("pass", &pass))
	if err != nil {
		replyErr(w, err)
		return
	}

	if err := s.a.ResetHostCertificateAuthority(pass); err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, message("host Certificate Authority regenerated"))
}

func (s *APIServer) resetUserCertificateAuthority(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var pass string

	err := form.Parse(r, form.String("pass", &pass))
	if err != nil {
		replyErr(w, err)
		return
	}

	if err := s.a.ResetUserCertificateAuthority(pass); err != nil {
		replyErr(w, err)
		return
	}

	reply(w, http.StatusOK, message("user Certificate Authority regenerated"))
}

func (s *APIServer) getHostCertificateAuthority(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	cert, err := s.a.GetHostCertificateAuthority()
	if err != nil {
		replyErr(w, err)
		return
	}

	reply(w, http.StatusOK, *cert)
}

func (s *APIServer) getUserCertificateAuthority(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	cert, err := s.a.GetUserCertificateAuthority()
	if err != nil {
		replyErr(w, err)
		return
	}

	reply(w, http.StatusOK, *cert)
}

func (s *APIServer) generateHostCert(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var id, hostname, key, role string
	var ttl time.Duration

	err := form.Parse(r,
		form.String("key", &key, form.Required()),
		form.String("id", &id, form.Required()),
		form.String("hostname", &hostname, form.Required()),
		form.String("role", &role, form.Required()),
		form.Duration("ttl", &ttl))

	if err != nil {
		reply(w, http.StatusBadRequest, err.Error())
		return
	}

	cert, err := s.a.GenerateHostCert([]byte(key), id, hostname, role, ttl)
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
		return
	}
	cert, err := s.a.GenerateUserCert([]byte(key), id, user, ttl)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, certResponse{Cert: string(cert)})
}

func (s *APIServer) generateToken(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var domainName, role string
	var ttl time.Duration

	err := form.Parse(r,
		form.String("domain", &domainName, form.Required()),
		form.String("role", &role, form.Required()),
		form.Duration("ttl", &ttl))

	if err != nil {
		reply(w, http.StatusBadRequest, err.Error())
		return
	}
	token, err := s.a.GenerateToken(domainName, role, ttl)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, tokenResponse{Token: string(token)})
}

func (s *APIServer) registerUsingToken(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var token, domainName, role string

	err := form.Parse(r,
		form.String("token", &token, form.Required()),
		form.String("domain", &domainName, form.Required()),
		form.String("role", &role, form.Required()),
	)
	if err != nil {
		reply(w, http.StatusBadRequest, err.Error())
		return
	}
	keys, err := s.a.RegisterUsingToken(token, domainName, role)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, keys)
}

func (s *APIServer) registerNewAuthServer(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var token, domainName, pkeyJSON string

	err := form.Parse(r,
		form.String("token", &token, form.Required()),
		form.String("domain", &domainName, form.Required()),
		form.String("key", &pkeyJSON, form.Required()),
	)
	if err != nil {
		reply(w, http.StatusBadRequest, err.Error())
		return
	}

	var pkey encryptor.Key
	err = json.Unmarshal([]byte(pkeyJSON), &pkey)
	if err != nil {
		reply(w, http.StatusBadRequest, err.Error())
		return
	}

	key, err := s.a.RegisterNewAuthServer(domainName, token, pkey)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, key)
}

func (s *APIServer) submitEvents(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var events form.Files

	err := form.Parse(r,
		form.FileSlice("event", &events))
	if err != nil {
		reply(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(events) == 0 {
		reply(w, http.StatusBadRequest,
			fmt.Errorf("at least one event is required"))
		return
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
		var e *lunk.Entry
		if err := json.Unmarshal(data, &e); err != nil {
			log.Errorf("failed to read event: %v", err)
			reply(w, http.StatusBadRequest, fmt.Errorf("failed to read event"))
			return
		}

		if err := s.a.LogEntry(*e); err != nil {
			log.Errorf("failed to write event: %v", err)
			reply(w, http.StatusInternalServerError,
				fmt.Errorf("failed to write event"))
			return
		}
	}

	reply(w, http.StatusOK, message("events submitted"))
}

func (s *APIServer) getEvents(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	f, err := events.FilterFromURL(r.URL.Query())
	if err != nil {
		log.Errorf("failed to parse filter: %v", err)
		reply(w, http.StatusBadRequest, fmt.Errorf("failed to parse filter"))
		return
	}
	es, err := s.a.GetEvents(*f)
	if err != nil {
		log.Errorf("failed to get events: %v", err)
		reply(w, http.StatusInternalServerError, fmt.Errorf("failed to get events"))
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, &eventsResponse{Events: es})
}

func (s *APIServer) submitChunks(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	sid := p[0].Value

	wr, err := s.a.GetChunkWriter(sid)
	if err != nil {
		log.Errorf("failed to get writer: %v", err)
		reply(w, http.StatusInternalServerError,
			fmt.Errorf("failed to write event"))
		return
	}
	defer wr.Close()

	var rawChunks form.Files
	err = form.Parse(r, form.FileSlice("chunk", &rawChunks))
	if err != nil {
		reply(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(rawChunks) == 0 {
		reply(w, http.StatusBadRequest,
			fmt.Errorf("at least one chunk is required"))
		return
	}

	defer func() {
		// don't let get the error get lost
		if err := rawChunks.Close(); err != nil {
			log.Errorf("failed to close files: %v", err)
			return
		}
	}()

	// decode chunks
	chunks := make([]recorder.Chunk, len(rawChunks))
	for i, c := range rawChunks {
		data, err := ioutil.ReadAll(c)
		if err != nil {
			log.Errorf("failed to read event: %v", err)
			reply(w, http.StatusBadRequest, fmt.Errorf("failed to read event"))
			return
		}
		var ch *recorder.Chunk
		if err := json.Unmarshal(data, &ch); err != nil {
			log.Errorf("failed to decode chunk: %v", err)
			reply(w, http.StatusInternalServerError,
				fmt.Errorf("failed to decode chunk"))
			return
		}
		chunks[i] = *ch
	}

	if err := wr.WriteChunks(chunks); err != nil {
		log.Errorf("failed to decode chunk: %v", err)
		reply(w, http.StatusInternalServerError,
			fmt.Errorf("failed to decode chunk"))
		return
	}

	reply(w, http.StatusOK, message("chunks submitted"))
}

func (s *APIServer) getChunks(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	sid := p[0].Value

	st, en := r.URL.Query().Get("start"), r.URL.Query().Get("end")
	start, err := strconv.Atoi(st)
	if err != nil {
		log.Errorf("failed to convert: %v", err)
		reply(w, http.StatusInternalServerError,
			fmt.Errorf("failed to convert"))
		return
	}
	end, err := strconv.Atoi(en)
	if err != nil {

		log.Errorf("failed to convert: %v", err)
		reply(w, http.StatusInternalServerError,
			fmt.Errorf("failed to convert"))
		return
	}

	re, err := s.a.GetChunkReader(sid)
	if err != nil {

		log.Errorf("failed to get reader: %v", err)
		reply(w, http.StatusInternalServerError,
			fmt.Errorf("failed to get reader"))
		return
	}
	defer re.Close()

	chunks, err := re.ReadChunks(start, end)
	if err != nil {
		log.Errorf("failed to read chunks: %v", err)
		reply(w, http.StatusInternalServerError,
			fmt.Errorf("failed to read chunks"))
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, &chunksResponse{Chunks: chunks})
}

func replyErr(w http.ResponseWriter, e error) {
	switch err := e.(type) {
	case *teleport.NotFoundError:
		reply(w, http.StatusNotFound, message(err.Error()))
		return
	case *teleport.MissingParameterError, *teleport.BadParameterError, *form.MissingParameterError, *form.BadParameterError:
		reply(w, http.StatusBadRequest, message(err.Error()))
		return
	}
	log.Errorf("auth server unexpected error: %v", e)
	// do not leak the unexpected error to the callee as we are not sure
	// if it's safe to disclose that information
	reply(w, http.StatusInternalServerError, message("internal server error: "+e.Error()))
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

func (s *APIServer) upsertRemoteCertificate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var id, key string
	var ttl time.Duration

	ctype, domainName := p[0].Value, p[1].Value

	err := form.Parse(r,
		form.String("key", &key, form.Required()),
		form.String("id", &id, form.Required()),
		form.Duration("ttl", &ttl))
	if err != nil {
		replyErr(w, err)
		return
	}
	cert := services.CertificateAuthority{ID: id, PublicKey: []byte(key), DomainName: domainName, Type: ctype}
	if err := s.a.UpsertRemoteCertificate(cert, ttl); err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, remoteCertResponse{RemoteCertificate: cert})
}

func (s *APIServer) getRemoteCertificates(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	domainName := r.URL.Query().Get("domain")
	ctype := p[0].Value

	certs, err := s.a.GetRemoteCertificates(ctype, domainName)
	if err != nil {
		log.Infof("error: %v", err)
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, &remoteCertsResponse{RemoteCertificates: certs})
}

func (s *APIServer) getTrustedCertificates(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	certType := p[0].Value

	certs, err := s.a.GetTrustedCertificates(certType)
	if err != nil {
		log.Infof("error: %v", err)
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, &remoteCertsResponse{RemoteCertificates: certs})
}

func (s *APIServer) deleteRemoteCertificate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctype, domainName, id := p[0].Value, p[1].Value, p[2].Value
	if err := s.a.DeleteRemoteCertificate(ctype, domainName, id); err != nil {
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
	if err := s.a.UpsertSession(sid, ttl); err != nil {
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
	if err := s.a.UpsertParty(sid, party, ttl); err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, partyResponse{Party: party})
}

func (s *APIServer) getSessions(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	sessions, err := s.a.GetSessions()
	if err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, &sessionsResponse{Sessions: sessions})
}

func (s *APIServer) getSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	sid := p[0].Value
	se, err := s.a.GetSession(sid)
	if err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, &sessionResponse{Session: *se})
}

func (s *APIServer) deleteSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	sid := p[0].Value
	if err := s.a.DeleteSession(sid); err != nil {
		replyErr(w, err)
		return
	}
	reply(w, http.StatusOK, message(fmt.Sprintf("session %v was deleted", sid)))
}

func (s *APIServer) getSignupTokenData(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	token := p[0].Value
	if len(token) == 0 {
		reply(w, http.StatusInternalServerError, "token is empty")
		return
	}

	user, QRImg, hotpFirstValues, err := s.a.GetSignupTokenData(token)
	if err != nil {
		reply(w, http.StatusInternalServerError, trace.Wrap(err).Error())
		return
	}

	reply(w, http.StatusOK, userTokenDataResponse{
		User:            user,
		QRImg:           QRImg,
		HotpFirstValues: hotpFirstValues,
	})
}

func (s *APIServer) createSignupToken(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var (
		user     string
		mappings []string
	)
	err := form.Parse(r,
		form.String("user", &user, form.Required()),
		form.StringSlice("mappings", &mappings, form.Required()),
	)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}

	token, err := s.a.CreateSignupToken(user, mappings)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}

	reply(w, http.StatusOK, message(token))
}

func (s *APIServer) createUserWithToken(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var token, password, hotpToken string
	err := form.Parse(r,
		form.String("token", &token, form.Required()),
		form.String("password", &password, form.Required()),
		form.String("hotptoken", &hotpToken, form.Required()),
	)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}

	err = s.a.CreateUserWithToken(token, password, hotpToken)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, message("ok"))
}

func (s *APIServer) getCertificateID(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	certType := p[0].Value
	key := r.URL.Query().Get("key")
	if len(key) == 0 {
		reply(w, http.StatusInternalServerError, "key is not provided")
		return
	}

	parsedKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key))
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}

	id, found, err := s.a.GetCertificateID(certType, parsedKey)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, getCertificateIDResponse{
		ID:    id,
		Found: found,
	})
}

func (s *APIServer) upsertUserMapping(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var certificateID, teleportUser, osUser string
	var ttl time.Duration
	err := form.Parse(r,
		form.String("certificateID", &certificateID, form.Required()),
		form.String("teleportUser", &teleportUser, form.Required()),
		form.String("osUser", &osUser, form.Required()),
		form.Duration("ttl", &ttl, form.Required()),
	)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}

	err = s.a.UpsertUserMapping(certificateID, teleportUser, osUser, ttl)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, message("ok"))
}

func (s *APIServer) deleteUserMapping(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	id := p[0].Value
	certificateID, teleportUser, osUser, err := services.ParseUserMappingID(id)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}

	err = s.a.DeleteUserMapping(certificateID, teleportUser, osUser)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, message("ok"))
}

func (s *APIServer) userMappingExists(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	id := p[0].Value
	certificateID, teleportUser, osUser, err := services.ParseUserMappingID(id)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}

	exists, err := s.a.UserMappingExists(certificateID, teleportUser, osUser)
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, userMappingExistsResponse{exists})
}

func (s *APIServer) getAllUserMapping(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	IDs, err := s.a.GetAllUserMappings()
	if err != nil {
		reply(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, getAllUserMappingsResponse{IDs})
}

type userMappingExistsResponse struct {
	Exists bool `json:"exists"`
}

type getAllUserMappingsResponse struct {
	IDs []string `json:"ids"`
}

type getCertificateIDResponse struct {
	ID    string `json:"id"`
	Found bool   `json:"found"`
}

type userTokenDataResponse struct {
	User            string   `json:"user"`
	QRImg           []byte   `json:"qrimg"`
	HotpFirstValues []string `json:"hotpfirstvalue"`
}

type pubKeyResponse struct {
	PubKey string `json:"pubkey"`
}

type pubKeysResponse struct {
	PubKeys []services.AuthorizedKey `json:"pubkeys"`
}

type certResponse struct {
	Cert string `json:"cert"`
}

type remoteCertResponse struct {
	RemoteCertificate services.CertificateAuthority `hson:"remote_cert"`
}

type remoteCertsResponse struct {
	RemoteCertificates []services.CertificateAuthority `hson:"remote_certs"`
}

type usersResponse struct {
	Users []services.User `json:"users"`
}

type keyPairResponse struct {
	PrivKey []byte `json:"privkey"`
	PubKey  string `json:"pubkey"`
}

type webSessionResponse struct {
	SID string `json:"sid"`
}

type webSessionsResponse struct {
	Keys []services.AuthorizedKey `json:"keys"`
}

type webTunResponse struct {
	Tunnel services.WebTun `json:"tunnel"`
}

type webTunsResponse struct {
	Tunnels []services.WebTun `json:"tunnels"`
}

type serversResponse struct {
	Servers []services.Server `json:"servers"`
}

type tokenResponse struct {
	Token string `json:"token"`
}

type eventsResponse struct {
	Events []lunk.Entry `json:"events"`
}

type chunksResponse struct {
	Chunks []recorder.Chunk `json:"chunks"`
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

type sealKeyResponse struct {
	Key encryptor.Key
}

type sealKeysResponse struct {
	Keys []encryptor.Key
}

type upsertPasswordResponse struct {
	HotpURL string
	HotpQR  []byte
}

type upsertServerArgs struct {
	Server services.Server
	TTL    time.Duration
}

func message(msg string) map[string]interface{} {
	return map[string]interface{}{"message": msg}
}
