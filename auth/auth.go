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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/session"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/bcrypt"
	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/teleport/services"
)

// Authority implements minimal key-management facility for generating OpenSSH
//compatible public/private key pairs and OpenSSH certificates
type Authority interface {
	GenerateKeyPair(passphrase string) (privKey []byte, pubKey []byte, err error)

	// GenerateHostCert generates host certificate, it takes pkey as a signing
	// private key (host certificate authority)
	GenerateHostCert(pkey, key []byte, id, hostname string, ttl time.Duration) ([]byte, error)

	// GenerateHostCert generates user certificate, it takes pkey as a signing
	// private key (user certificate authority)
	GenerateUserCert(pkey, key []byte, id, username string, ttl time.Duration) ([]byte, error)
}

type Session struct {
	SID session.SecureID
	PID session.PlainID
	WS  services.WebSession
}

func NewAuthServer(bk backend.Backend, a Authority, scrt *secret.Service) *AuthServer {
	as := AuthServer{}

	as.bk = bk
	as.a = a
	as.scrt = scrt

	as.caS = services.NewCAService(as.bk)
	as.lockS = services.NewLockService(as.bk)
	as.presenceS = services.NewPresenceService(as.bk)
	as.provisioningS = services.NewProvisioningService(as.bk)
	as.userS = services.NewUserService(as.bk)
	as.webS = services.NewWebService(as.bk)

	return &as
}

// AuthServer implements key signing, generation and ACL functionality
// used by teleport
type AuthServer struct {
	bk   backend.Backend
	a    Authority
	scrt *secret.Service

	caS           *services.CAService
	lockS         *services.LockService
	presenceS     *services.PresenceService
	provisioningS *services.ProvisioningService
	userS         *services.UserService
	webS          *services.WebService
}

func (s *AuthServer) UpsertServer(srv services.Server, ttl time.Duration) error {
	return s.presenceS.UpsertServer(srv, ttl)
}

func (s *AuthServer) GetServers() ([]services.Server, error) {
	return s.presenceS.GetServers()
}

// UpsertUserKey takes user's public key, generates certificate for it
// and adds it to the authorized keys database. It returns certificate signed
// by user CA in case of success, error otherwise. The certificate will be
// valid for the duration of the ttl passed in.
func (s *AuthServer) UpsertUserKey(
	user string, key services.AuthorizedKey, ttl time.Duration) ([]byte, error) {

	cert, err := s.GenerateUserCert(key.Value, key.ID, user, ttl)
	if err != nil {
		return nil, err
	}
	key.Value = cert
	if err := s.userS.UpsertUserKey(user, key, ttl); err != nil {
		return nil, err
	}
	return cert, nil
}

func (s *AuthServer) GetUsers() ([]string, error) {
	return s.userS.GetUsers()
}

func (s *AuthServer) DeleteUser(user string) error {
	return s.userS.DeleteUser(user)
}

func (s *AuthServer) GetUserKeys(user string) ([]services.AuthorizedKey, error) {
	return s.userS.GetUserKeys(user)
}

// DeleteUserKey deletes user key by given ID
func (s *AuthServer) DeleteUserKey(user, key string) error {
	return s.userS.DeleteUserKey(user, key)
}

// GenerateKeyPair generates private and public key pair of OpenSSH certs
func (s *AuthServer) GenerateKeyPair(pass string) ([]byte, []byte, error) {
	return s.a.GenerateKeyPair(pass)
}

func (s *AuthServer) UpsertRemoteCert(cert services.RemoteCert, ttl time.Duration) error {
	return s.caS.UpsertRemoteCert(cert, ttl)
}

func (s *AuthServer) GetRemoteCerts(ctype string, fqdn string) ([]services.RemoteCert, error) {
	return s.caS.GetRemoteCerts(ctype, fqdn)
}

func (s *AuthServer) DeleteRemoteCert(ctype string, fqdn, id string) error {
	return s.caS.DeleteRemoteCert(ctype, fqdn, id)
}

// ResetHostCA generates host certificate authority and updates the backend
func (s *AuthServer) ResetHostCA(pass string) error {
	priv, pub, err := s.a.GenerateKeyPair(pass)
	if err != nil {
		return err
	}
	return s.caS.UpsertHostCA(services.CA{Pub: pub, Priv: priv})
}

// ResetHostCA generates user certificate authority and updates the backend
func (s *AuthServer) ResetUserCA(pass string) error {
	priv, pub, err := s.a.GenerateKeyPair(pass)
	if err != nil {
		return err
	}
	return s.caS.UpsertUserCA(services.CA{Pub: pub, Priv: priv})
}

// GetHostCAPub returns a public key for host key signing authority
func (s *AuthServer) GetHostCAPub() ([]byte, error) {
	return s.caS.GetHostCAPub()
}

// GetHostCAPub returns a public key for user key signing authority
func (s *AuthServer) GetUserCAPub() ([]byte, error) {
	return s.caS.GetUserCAPub()
}

// GenerateHostCert generates host certificate, it takes pkey as a signing
// private key (host certificate authority)
func (s *AuthServer) GenerateHostCert(
	key []byte, id, hostname string, ttl time.Duration) ([]byte, error) {

	hk, err := s.caS.GetHostCA()
	if err != nil {
		return nil, err
	}
	return s.a.GenerateHostCert(hk.Priv, key, id, hostname, ttl)
}

// GenerateHostCert generates user certificate, it takes pkey as a signing
// private key (user certificate authority)
func (s *AuthServer) GenerateUserCert(
	key []byte, id, username string, ttl time.Duration) ([]byte, error) {

	hk, err := s.caS.GetUserCA()
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
	return s.webS.UpsertPasswordHash(user, hash)
}

func (s *AuthServer) CheckPassword(user string, password []byte) error {
	if err := verifyPassword(password); err != nil {
		return err
	}
	hash, err := s.webS.GetPasswordHash(user)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword(hash, password); err != nil {
		return &teleport.BadParameterError{Err: "passwords do not match"}
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

func (s *AuthServer) GenerateToken(fqdn string, ttl time.Duration) (string, error) {
	p, err := session.NewID(s.scrt)
	if err != nil {
		return "", err
	}
	if err := s.provisioningS.UpsertToken(string(p.PID), fqdn, ttl); err != nil {
		return "", err
	}
	return string(p.SID), nil
}

func (s *AuthServer) ValidateToken(token, fqdn string) error {
	pid, err := session.DecodeSID(session.SecureID(token), s.scrt)
	if err != nil {
		return err
	}
	out, err := s.provisioningS.GetToken(string(pid))
	if err != nil {
		return err
	}
	if out != fqdn {
		return fmt.Errorf("fqdn does not match")
	}
	return nil
}

func (s *AuthServer) DeleteToken(token string) error {
	pid, err := session.DecodeSID(session.SecureID(token), s.scrt)
	if err != nil {
		return err
	}
	return s.provisioningS.DeleteToken(string(pid))
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
	hk, err := s.caS.GetUserCA()
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
		WS:  services.WebSession{Priv: priv, Pub: cert},
	}
	return sess, nil
}

func (s *AuthServer) UpsertWebSession(user string, sess *Session, ttl time.Duration) error {
	return s.webS.UpsertWebSession(user, string(sess.PID), sess.WS, ttl)
}

func (s *AuthServer) GetWebSession(user string, sid session.SecureID) (*Session, error) {
	pid, err := session.DecodeSID(sid, s.scrt)
	if err != nil {
		return nil, err
	}
	ws, err := s.webS.GetWebSession(user, string(pid))
	if err != nil {
		return nil, err
	}
	return &Session{
		SID: sid,
		PID: pid,
		WS:  *ws,
	}, nil
}

func (s *AuthServer) GetWebSessionsKeys(user string) ([]services.AuthorizedKey, error) {
	return s.webS.GetWebSessionsKeys(user)
}

func (s *AuthServer) DeleteWebSession(user string, sid session.SecureID) error {
	pid, err := session.DecodeSID(sid, s.scrt)
	if err != nil {
		return err
	}
	return s.webS.DeleteWebSession(user, string(pid))
}

func (s *AuthServer) UpsertWebTun(t services.WebTun, ttl time.Duration) error {
	return s.webS.UpsertWebTun(t, ttl)
}

func (s *AuthServer) GetWebTun(prefix string) (*services.WebTun, error) {
	return s.webS.GetWebTun(prefix)
}

func (s *AuthServer) GetWebTuns() ([]services.WebTun, error) {
	return s.webS.GetWebTuns()
}

func (s *AuthServer) DeleteWebTun(prefix string) error {
	return s.webS.DeleteWebTun(prefix)
}

// make sure password satisfies our requirements (relaxed),
// mostly to avoid putting garbage in
func verifyPassword(password []byte) error {
	if len(password) < MinPasswordLength {
		return &teleport.BadParameterError{
			Param: "password",
			Err: fmt.Sprintf(
				"password is too short, min length is %v", MinPasswordLength),
		}
	}
	if len(password) > MaxPasswordLength {
		return &teleport.BadParameterError{
			Param: "password",
			Err: fmt.Sprintf(
				"password is too long, max length is %v", MaxPasswordLength),
		}
	}
	return nil
}

const (
	Week              = time.Hour * 24 * 7
	MinPasswordLength = 6
	MaxPasswordLength = 128
)
