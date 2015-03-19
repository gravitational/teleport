// package auth implements certificate signing authority and access control server
// Authority server is composed of several parts:
//
// * Authority server itself that implements signing and acl logic
// * HTTP server wrapper for authority server
// * HTTP client wrapper
//
package auth

import (
	"fmt"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/session"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/bcrypt"
	"github.com/gravitational/teleport/backend"
)

// Authority implements minimal key-management facility for generating OpenSSH
//compatible public/private key pairs and OpenSSH certificates
type Authority interface {
	GenerateKeyPair(passphrase string) (privKey []byte, pubKey []byte, err error)

	// GenerateHostCert generates host certificate, it takes pkey as a signing private key (host certificate authority)
	GenerateHostCert(pkey, key []byte, id, hostname string, ttl time.Duration) ([]byte, error)

	// GenerateHostCert generates user certificate, it takes pkey as a signing private key (user certificate authority)
	GenerateUserCert(pkey, key []byte, id, username string, ttl time.Duration) ([]byte, error)
}

type Session struct {
	SID session.SecureID
	PID session.PlainID
	WS  backend.WebSession
}

func NewAuthServer(b backend.Backend, a Authority, scrt *secret.Service) *AuthServer {
	return &AuthServer{
		b:    b,
		a:    a,
		scrt: scrt,
	}
}

// AuthServer implements key signing, generation and ACL functionality used by teleport
type AuthServer struct {
	b    backend.Backend
	a    Authority
	scrt *secret.Service
}

func (s *AuthServer) UpsertServer(srv backend.Server, ttl time.Duration) error {
	return s.b.UpsertServer(srv, ttl)
}

func (s *AuthServer) GetServers() ([]backend.Server, error) {
	return s.b.GetServers()
}

// UpsertUserKey takes user's public key, generates certificate for it
// and adds it to the authorized keys database. It returns certificate signed by user CA in case of success,
// error otherwise. The certificate will be valid for the duration of the ttl passed in.
func (s *AuthServer) UpsertUserKey(user string, key backend.AuthorizedKey, ttl time.Duration) ([]byte, error) {
	cert, err := s.GenerateUserCert(key.Value, key.ID, user, ttl)
	if err != nil {
		return nil, err
	}
	key.Value = cert
	if err := s.b.UpsertUserKey(user, key, ttl); err != nil {
		return nil, err
	}
	return cert, nil
}

func (s *AuthServer) GetUsers() ([]string, error) {
	return s.b.GetUsers()
}

func (s *AuthServer) DeleteUser(user string) error {
	return s.b.DeleteUser(user)
}

func (s *AuthServer) GetUserKeys(user string) ([]backend.AuthorizedKey, error) {
	return s.b.GetUserKeys(user)
}

// DeleteUserKey deletes user key by given ID
func (s *AuthServer) DeleteUserKey(user, key string) error {
	return s.b.DeleteUserKey(user, key)
}

// GenerateKeyPair generates private and public key pair of OpenSSH style certificates
func (s *AuthServer) GenerateKeyPair(pass string) ([]byte, []byte, error) {
	return s.a.GenerateKeyPair(pass)
}

// ResetHostCA generates host certificate authority and updates the backend
func (s *AuthServer) ResetHostCA(pass string) error {
	priv, pub, err := s.a.GenerateKeyPair(pass)
	if err != nil {
		return err
	}
	return s.b.UpsertHostCA(backend.CA{Pub: pub, Priv: priv})
}

// ResetHostCA generates user certificate authority and updates the backend
func (s *AuthServer) ResetUserCA(pass string) error {
	priv, pub, err := s.a.GenerateKeyPair(pass)
	if err != nil {
		return err
	}
	return s.b.UpsertUserCA(backend.CA{Pub: pub, Priv: priv})
}

// GetHostCAPub returns a public key for host key signing authority
func (s *AuthServer) GetHostCAPub() ([]byte, error) {
	return s.b.GetHostCAPub()
}

// GetHostCAPub returns a public key for user key signing authority
func (s *AuthServer) GetUserCAPub() ([]byte, error) {
	return s.b.GetUserCAPub()
}

// GenerateHostCert generates host certificate, it takes pkey as a signing private key (host certificate authority)
func (s *AuthServer) GenerateHostCert(key []byte, id, hostname string, ttl time.Duration) ([]byte, error) {
	hk, err := s.b.GetHostCA()
	if err != nil {
		return nil, err
	}
	return s.a.GenerateHostCert(hk.Priv, key, id, hostname, ttl)
}

// GenerateHostCert generates user certificate, it takes pkey as a signing private key (user certificate authority)
func (s *AuthServer) GenerateUserCert(key []byte, id, username string, ttl time.Duration) ([]byte, error) {
	hk, err := s.b.GetUserCA()
	if err != nil {
		return nil, err
	}
	return s.a.GenerateUserCert(hk.Priv, key, id, username, ttl)
}

func (s *AuthServer) UpsertPassword(user string, password []byte) error {
	if err := verifyPassword(password); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.b.UpsertPasswordHash(user, hash)
}

func (s *AuthServer) CheckPassword(user string, password []byte) error {
	if err := verifyPassword(password); err != nil {
		return err
	}
	hash, err := s.b.GetPasswordHash(user)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword(hash, password); err != nil {
		return &BadParameterError{Msg: "passwords do not match"}
	}
	return nil
}

func (s *AuthServer) SignIn(user string, password []byte) (*Session, error) {
	if err := s.CheckPassword(user, password); err != nil {
		return nil, err
	}
	sess, err := s.NewWebSession(user)
	if err != nil {
		return nil, err
	}
	if err := s.UpsertWebSession(user, sess, time.Hour); err != nil {
		return nil, err
	}
	return sess, nil
}

func (s *AuthServer) NewWebSession(user string) (*Session, error) {
	p, err := session.NewID(s.scrt)
	if err != nil {
		return nil, err
	}
	priv, pub, err := s.GenerateKeyPair("")
	if err != nil {
		return nil, err
	}
	hk, err := s.b.GetUserCA()
	if err != nil {
		return nil, err
	}
	cert, err := s.a.GenerateUserCert(hk.Priv, pub, user, user, 0)
	if err != nil {
		return nil, err
	}
	sess := &Session{
		SID: p.SID,
		PID: p.PID,
		WS:  backend.WebSession{Priv: priv, Pub: cert},
	}
	return sess, nil
}

func (s *AuthServer) UpsertWebSession(user string, sess *Session, ttl time.Duration) error {
	return s.b.UpsertWebSession(user, string(sess.PID), sess.WS, ttl)
}

func (s *AuthServer) GetWebSession(user string, sid session.SecureID) (*Session, error) {
	pid, err := session.DecodeSID(sid, s.scrt)
	if err != nil {
		return nil, err
	}
	ws, err := s.b.GetWebSession(user, string(pid))
	if err != nil {
		return nil, err
	}
	return &Session{
		SID: sid,
		PID: pid,
		WS:  *ws,
	}, nil
}

func (s *AuthServer) DeleteWebSession(user string, sid session.SecureID) error {
	pid, err := session.DecodeSID(sid, s.scrt)
	if err != nil {
		return err
	}
	return s.b.DeleteWebSession(user, string(pid))
}

func (s *AuthServer) UpsertWebTun(t backend.WebTun, ttl time.Duration) error {
	return s.b.UpsertWebTun(t, ttl)
}

func (s *AuthServer) GetWebTun(prefix string) (*backend.WebTun, error) {
	return s.b.GetWebTun(prefix)
}

func (s *AuthServer) GetWebTuns() ([]backend.WebTun, error) {
	return s.b.GetWebTuns()
}

func (s *AuthServer) DeleteWebTun(prefix string) error {
	return s.b.DeleteWebTun(prefix)
}

// make sure password satisfies our requirements (relaxed), mostly to avoid putting garbage in
func verifyPassword(password []byte) error {
	if len(password) < MinPasswordLength {
		return &BadParameterError{
			Param: "password",
			Msg:   fmt.Sprintf("password is too short, min length is %v", MinPasswordLength),
		}
	}
	if len(password) > MaxPasswordLength {
		return &BadParameterError{
			Param: "password",
			Msg:   fmt.Sprintf("password is too long, max length is %v", MaxPasswordLength),
		}
	}
	return nil
}

const (
	Week              = time.Hour * 24 * 7
	MinPasswordLength = 6
	MaxPasswordLength = 128
)

type BadParameterError struct {
	Param string
	Msg   string
}

func (p *BadParameterError) Error() string {
	return fmt.Sprintf("bad parameter: %v, err: %v", p.Param, p.Msg)
}
