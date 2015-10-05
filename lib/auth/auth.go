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
	"github.com/gravitational/teleport/lib/backend/encryptedbk"
	"github.com/gravitational/teleport/lib/services"
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

func NewAuthServer(bk *encryptedbk.ReplicatedBackend, a Authority, scrt secret.SecretService) *AuthServer {
	as := AuthServer{}

	as.bk = bk
	as.Authority = a
	as.scrt = scrt

	as.CAService = services.NewCAService(as.bk)
	as.LockService = services.NewLockService(as.bk)
	as.PresenceService = services.NewPresenceService(as.bk)
	as.ProvisioningService = services.NewProvisioningService(as.bk)
	as.UserService = services.NewUserService(as.bk)
	as.WebService = services.NewWebService(as.bk)
	as.BkKeysService = services.NewBkKeysService(as.bk)

	return &as
}

// AuthServer implements key signing, generation and ACL functionality
// used by teleport
type AuthServer struct {
	bk *encryptedbk.ReplicatedBackend
	Authority
	scrt secret.SecretService

	*services.CAService
	*services.LockService
	*services.PresenceService
	*services.ProvisioningService
	*services.UserService
	*services.WebService
	*services.BkKeysService
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
	if err := s.UserService.UpsertUserKey(user, key, ttl); err != nil {
		return nil, err
	}
	return cert, nil
}

// ResetHostCA generates host certificate authority and updates the backend
func (s *AuthServer) ResetHostCA(pass string) error {
	priv, pub, err := s.Authority.GenerateKeyPair(pass)
	if err != nil {
		return err
	}
	return s.CAService.UpsertHostCA(services.CA{Pub: pub, Priv: priv})
}

// ResetHostCA generates user certificate authority and updates the backend
func (s *AuthServer) ResetUserCA(pass string) error {
	priv, pub, err := s.Authority.GenerateKeyPair(pass)
	if err != nil {
		return err
	}
	return s.CAService.UpsertUserCA(services.CA{Pub: pub, Priv: priv})
}

// GenerateHostCert generates host certificate, it takes pkey as a signing
// private key (host certificate authority)
func (s *AuthServer) GenerateHostCert(
	key []byte, id, hostname string, ttl time.Duration) ([]byte, error) {

	hk, err := s.CAService.GetHostCA()
	if err != nil {
		return nil, err
	}
	return s.Authority.GenerateHostCert(hk.Priv, key, id, hostname, ttl)
}

// GenerateHostCert generates user certificate, it takes pkey as a signing
// private key (user certificate authority)
func (s *AuthServer) GenerateUserCert(
	key []byte, id, username string, ttl time.Duration) ([]byte, error) {

	hk, err := s.CAService.GetUserCA()
	if err != nil {
		return nil, err
	}
	return s.Authority.GenerateUserCert(hk.Priv, key, id, username, ttl)
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
	if err := s.ProvisioningService.UpsertToken(string(p.PID), fqdn, ttl); err != nil {
		return "", err
	}
	return string(p.SID), nil
}

func (s *AuthServer) ValidateToken(token, fqdn string) error {
	pid, err := session.DecodeSID(session.SecureID(token), s.scrt)
	if err != nil {
		return err
	}
	out, err := s.ProvisioningService.GetToken(string(pid))
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
	return s.ProvisioningService.DeleteToken(string(pid))
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
	hk, err := s.CAService.GetUserCA()
	if err != nil {
		return nil, err
	}
	cert, err := s.Authority.GenerateUserCert(hk.Priv, pub, user, user, 0)
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
	return s.WebService.UpsertWebSession(user, string(sess.PID), sess.WS, ttl)
}

func (s *AuthServer) GetWebSession(user string, sid session.SecureID) (*Session, error) {
	pid, err := session.DecodeSID(sid, s.scrt)
	if err != nil {
		return nil, err
	}
	ws, err := s.WebService.GetWebSession(user, string(pid))
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
	return s.WebService.DeleteWebSession(user, string(pid))
}

const (
	Week = time.Hour * 24 * 7
)
