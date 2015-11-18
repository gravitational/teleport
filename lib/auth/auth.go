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

	"github.com/gravitational/teleport/lib/backend/encryptedbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/session"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"
)

// Authority implements minimal key-management facility for generating OpenSSH
//compatible public/private key pairs and OpenSSH certificates
type Authority interface {
	GenerateKeyPair(passphrase string) (privKey []byte, pubKey []byte, err error)

	// GenerateHostCert generates host certificate, it takes pkey as a signing
	// private key (host certificate authority)
	GenerateHostCert(pkey, key []byte, id, hostname, role string, ttl time.Duration) ([]byte, error)

	// GenerateHostCert generates user certificate, it takes pkey as a signing
	// private key (user certificate authority)
	GenerateUserCert(pkey, key []byte, id, username string, ttl time.Duration) ([]byte, error)
}

type Session struct {
	SID session.SecureID
	PID session.PlainID
	WS  services.WebSession
}

func NewAuthServer(bk *encryptedbk.ReplicatedBackend, a Authority,
	scrt secret.SecretService, hostname string) *AuthServer {
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

	as.Hostname = hostname
	return &as
}

// AuthServer implements key signing, generation and ACL functionality
// used by teleport
type AuthServer struct {
	bk *encryptedbk.ReplicatedBackend
	Authority
	scrt     secret.SecretService
	Hostname string

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
// by user Certificate Authority in case of success, error otherwise. The certificate will be
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

// ResetHostCertificateAuthority generates host certificate authority and updates the backend
func (s *AuthServer) ResetHostCertificateAuthority(pass string) error {
	priv, pub, err := s.Authority.GenerateKeyPair(pass)
	if err != nil {
		return err
	}
	return s.CAService.UpsertHostCertificateAuthority(
		services.CertificateAuthority{
			PublicCertificate: services.PublicCertificate{
				Type:     services.HostCert,
				FQDN:     s.Hostname,
				PubValue: pub,
				ID:       string(pub),
			},
			PrivValue: priv},
	)
}

// ResetHostCertificateAuthority generates user certificate authority and updates the backend
func (s *AuthServer) ResetUserCertificateAuthority(pass string) error {
	priv, pub, err := s.Authority.GenerateKeyPair(pass)
	if err != nil {
		return err
	}
	return s.CAService.UpsertUserCertificateAuthority(
		services.CertificateAuthority{
			PublicCertificate: services.PublicCertificate{
				Type:     services.UserCert,
				FQDN:     s.Hostname,
				PubValue: pub,
				ID:       string(pub),
			},
			PrivValue: priv},
	)
}

// GenerateHostCert generates host certificate, it takes pkey as a signing
// private key (host certificate authority)
func (s *AuthServer) GenerateHostCert(
	key []byte, id, hostname, role string,
	ttl time.Duration) ([]byte, error) {

	hk, err := s.CAService.GetHostCertificateAuthority()
	if err != nil {
		return nil, err
	}
	return s.Authority.GenerateHostCert(hk.PrivValue, key, id, hostname, role, ttl)
}

// GenerateHostCert generates user certificate, it takes pkey as a signing
// private key (user certificate authority)
func (s *AuthServer) GenerateUserCert(
	key []byte, id, username string, ttl time.Duration) ([]byte, error) {

	hk, err := s.CAService.GetUserCertificateAuthority()
	if err != nil {
		return nil, err
	}
	return s.Authority.GenerateUserCert(hk.PrivValue, key, id, username, ttl)
}

func (s *AuthServer) SignIn(user string, password []byte) (*Session, error) {
	if err := s.CheckPasswordWOToken(user, password); err != nil {
		return nil, err
	}
	sess, err := s.NewWebSession(user)
	if err != nil {
		return nil, err
	}
	if err := s.UpsertWebSession(user, sess, WebSessionTTL); err != nil {
		return nil, err
	}
	return sess, nil
}

func (s *AuthServer) GenerateToken(fqdn, role string, ttl time.Duration) (string, error) {
	p, err := session.NewID(s.scrt)
	if err != nil {
		return "", err
	}
	if err := s.ProvisioningService.UpsertToken(string(p.PID), fqdn, role, ttl); err != nil {
		return "", err
	}
	return string(p.SID), nil
}

func (s *AuthServer) ValidateToken(token, fqdn string) (role string, e error) {
	pid, err := session.DecodeSID(session.SecureID(token), s.scrt)
	if err != nil {
		return "", trace.Wrap(err)
	}
	tok, err := s.ProvisioningService.GetToken(string(pid))
	if err != nil {
		return "", trace.Wrap(err)
	}
	if tok.FQDN != fqdn {
		return "", trace.Errorf("fqdn does not match")
	}
	return tok.Role, nil
}

func (s *AuthServer) RegisterUsingToken(token, fqdn, role string) (keys PackedKeys, e error) {
	pid, err := session.DecodeSID(session.SecureID(token), s.scrt)
	if err != nil {
		return PackedKeys{}, trace.Wrap(err)
	}
	tok, err := s.ProvisioningService.GetToken(string(pid))
	if err != nil {
		return PackedKeys{}, trace.Wrap(err)
	}
	if tok.FQDN != fqdn {
		return PackedKeys{}, trace.Errorf("fqdn does not match")
	}

	if tok.Role != role {
		return PackedKeys{}, trace.Errorf("role does not match")
	}

	k, pub, err := s.GenerateKeyPair("")
	if err != nil {
		return PackedKeys{}, trace.Wrap(err)
	}
	c, err := s.GenerateHostCert(pub, fqdn+"_"+role, fqdn, role, 0)
	if err != nil {
		return PackedKeys{}, trace.Wrap(err)
	}

	keys = PackedKeys{
		Key:  k,
		Cert: c,
	}

	if err := s.DeleteToken(token); err != nil {
		return PackedKeys{}, trace.Wrap(err)
	}

	return keys, nil
}

func (s *AuthServer) RegisterNewAuthServer(fqdn, token string,
	publicSealKey encryptor.Key) (masterKey encryptor.Key, e error) {
	pid, err := session.DecodeSID(session.SecureID(token), s.scrt)
	if err != nil {
		return encryptor.Key{}, trace.Wrap(err)
	}
	tok, err := s.ProvisioningService.GetToken(string(pid))
	if err != nil {
		return encryptor.Key{}, trace.Wrap(err)
	}
	if tok.FQDN != fqdn {
		return encryptor.Key{}, trace.Errorf("fqdn does not match")
	}

	if tok.Role != RoleAuth {
		return encryptor.Key{}, trace.Errorf("role does not match")
	}

	if err := s.DeleteToken(token); err != nil {
		return encryptor.Key{}, trace.Wrap(err)
	}

	if err := s.BkKeysService.AddSealKey(publicSealKey); err != nil {
		return encryptor.Key{}, trace.Wrap(err)
	}

	localKey, err := s.BkKeysService.GetSignKey()
	if err != nil {
		return encryptor.Key{}, trace.Wrap(err)
	}

	return localKey.Public(), nil
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
	hk, err := s.CAService.GetUserCertificateAuthority()
	if err != nil {
		return nil, err
	}
	cert, err := s.Authority.GenerateUserCert(hk.PrivValue, pub, user, user, WebSessionTTL)
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
	Week          = time.Hour * 24 * 7
	WebSessionTTL = time.Hour * 10
)
