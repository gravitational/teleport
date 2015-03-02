// package auth implements certificate signing authority and access control server
// Authority server is composed of several parts:
//
// * Authority server itself that implements signing and acl logic
// * HTTP server wrapper for authority server
// * HTTP client wrapper
//
package auth

import (
	"time"

	"github.com/gravitational/teleport/backend"
)

// Authority implements minimal key-management facility for generating OpenSSH
//compatible public/private key pairs and OpenSSH certificates
type Authority interface {
	GenerateKeyPair(passphrase string) ([]byte, []byte, error)

	// GenerateHostCert generates host certificate, it takes pkey as a signing private key (host certificate authority)
	GenerateHostCert(pkey, key []byte, id, hostname string, ttl time.Duration) ([]byte, error)

	// GenerateHostCert generates user certificate, it takes pkey as a signing private key (user certificate authority)
	GenerateUserCert(pkey, key []byte, id, username string, ttl time.Duration) ([]byte, error)
}

func NewAuthServer(b backend.Backend, a Authority) *AuthServer {
	return &AuthServer{
		b: b,
		a: a,
	}
}

// AuthServer implements key signing, generation and ACL functionality used by teleport
type AuthServer struct {
	b backend.Backend
	a Authority
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

const Week = time.Hour * 24 * 7
