package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/julienschmidt/httprouter"
	"github.com/gravitational/teleport/auth/form"
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
	srv.POST("/v1/ca/host/keys", srv.ResetHostCA)
	srv.POST("/v1/ca/user/keys", srv.ResetUserCA)

	// Retrieving authority public keys
	srv.GET("/v1/ca/host/keys/pub", srv.GetHostCAPub)
	srv.GET("/v1/ca/user/keys/pub", srv.GetUserCAPub)

	// Generating certificates for user and host authorities
	srv.POST("/v1/ca/host/certs", srv.GenerateHostCert)
	srv.POST("/v1/ca/user/certs", srv.GenerateUserCert)

	// Operations on users
	srv.GET("/v1/users", srv.GetUsers)
	srv.DELETE("/v1/users/:user", srv.DeleteUser)

	// Operations on user keys
	srv.POST("/v1/users/:user/keys", srv.UpsertUserKey)
	srv.DELETE("/v1/users/:user/keys/:key", srv.DeleteUserKey)
	srv.GET("/v1/users/:user/keys", srv.GetUserKeys)

	// Generating keypairs
	srv.POST("/v1/keypair", srv.GenerateKeyPair)

	return srv
}

func (s *APIServer) UpsertUserKey(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var id, key string
	var ttl time.Duration

	err := form.Parse(r,
		form.String("key", &key, form.Required()),
		form.String("id", &id, form.Required()),
		form.Duration("ttl", &ttl))
	if err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	cert, err := s.s.UpsertUserKey(p[0].Value, backend.AuthorizedKey{ID: id, Value: []byte(key)}, ttl)
	if err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}

	reply(w, http.StatusOK, certResponse{Cert: string(cert)})
}

func (s *APIServer) GetUsers(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	users, err := s.s.GetUsers()
	if err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	reply(w, http.StatusOK, &usersResponse{Users: users})
}

func (s *APIServer) DeleteUser(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	user := p[0].Value
	if err := s.s.DeleteUser(user); err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}

	reply(w, http.StatusOK, message(fmt.Sprintf("user '%v' deleted", user)))
}

func (s *APIServer) GetUserKeys(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	keys, err := s.s.GetUserKeys(p[0].Value)
	if err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}

	reply(w, http.StatusOK, &pubKeysResponse{PubKeys: keys})
}

func (s *APIServer) DeleteUserKey(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	user, keyID := p[0].Value, p[1].Value
	if err := s.s.DeleteUserKey(user, keyID); err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}

	reply(w, http.StatusOK, message(fmt.Sprintf("key '%v' deleted for user '%v'", keyID, user)))
}

func (s *APIServer) GenerateKeyPair(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var pass string

	if err := form.Parse(r, form.String("pass", &pass)); err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}

	priv, pub, err := s.s.GenerateKeyPair(pass)
	if err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}

	reply(w, http.StatusOK, &keyPairResponse{PrivKey: priv, PubKey: string(pub)})
}

func (s *APIServer) ResetHostCA(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var pass string

	if err := form.Parse(r, form.String("pass", &pass)); err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.s.ResetHostCA(pass); err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	reply(w, http.StatusOK, message("host CA regenerated"))
}

func (s *APIServer) ResetUserCA(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var pass string

	if err := form.Parse(r, form.String("pass", &pass)); err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}

	if err := s.s.ResetUserCA(pass); err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}

	reply(w, http.StatusOK, message("user CA regenerated"))
}

func (s *APIServer) GetHostCAPub(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	bytes, err := s.s.GetHostCAPub()
	if err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}

	reply(w, http.StatusOK, pubKeyResponse{PubKey: string(bytes)})
}

func (s *APIServer) GetUserCAPub(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	bytes, err := s.s.GetUserCAPub()
	if err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}

	reply(w, http.StatusOK, pubKeyResponse{PubKey: string(bytes)})
}

func (s *APIServer) GenerateHostCert(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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

func (s *APIServer) GenerateUserCert(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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

func replyErr(w http.ResponseWriter, code int, err error) {
	reply(w, code, message(err.Error()))
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

func message(msg string) map[string]interface{} {
	return map[string]interface{}{"message": msg}
}
