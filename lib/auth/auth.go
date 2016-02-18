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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend/encryptedbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/trace"
)

// Authority implements minimal key-management facility for generating OpenSSH
//compatible public/private key pairs and OpenSSH certificates
type Authority interface {
	// GenerateKeyPair generates new keypair
	GenerateKeyPair(passphrase string) (privKey []byte, pubKey []byte, err error)

	// GetNewKeyPairFromPool returns new keypair from pre-generated in memory pool
	GetNewKeyPairFromPool() (privKey []byte, pubKey []byte, err error)

	// GenerateHostCert generates host certificate, it takes pkey as a signing
	// private key (host certificate authority)
	GenerateHostCert(pkey, key []byte, id, hostname, role string, ttl time.Duration) ([]byte, error)

	// GenerateHostCert generates user certificate, it takes pkey as a signing
	// private key (user certificate authority)
	GenerateUserCert(pkey, key []byte, id, username string, ttl time.Duration) ([]byte, error)
}

type Session struct {
	ID string
	WS services.WebSession
}

func NewAuthServer(bk *encryptedbk.ReplicatedBackend, a Authority, hostname string) *AuthServer {
	as := AuthServer{}

	as.bk = bk
	as.Authority = a

	as.CAService = services.NewCAService(as.bk)
	as.LockService = services.NewLockService(as.bk)
	as.PresenceService = services.NewPresenceService(as.bk)
	as.ProvisioningService = services.NewProvisioningService(as.bk)
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
	Hostname string

	*services.CAService
	*services.LockService
	*services.PresenceService
	*services.ProvisioningService
	*services.WebService
	*services.BkKeysService
}

// GetLocalDomain returns domain name that identifies this authority server
func (a *AuthServer) GetLocalDomain() (string, error) {
	return a.Hostname, nil
}

// GenerateHostCert generates host certificate, it takes pkey as a signing
// private key (host certificate authority)
func (s *AuthServer) GenerateHostCert(
	key []byte, id, hostname, role string,
	ttl time.Duration) ([]byte, error) {

	ca, err := s.CAService.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: s.Hostname,
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privateKey, err := ca.FirstSigningKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return s.Authority.GenerateHostCert(privateKey, key, id, hostname, role, ttl)
}

// GenerateUserCert generates user certificate, it takes pkey as a signing
// private key (user certificate authority)
func (s *AuthServer) GenerateUserCert(
	key []byte, id, username string, ttl time.Duration) ([]byte, error) {

	ca, err := s.CAService.GetCertAuthority(services.CertAuthID{
		Type:       services.UserCA,
		DomainName: s.Hostname,
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privateKey, err := ca.FirstSigningKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return s.Authority.GenerateUserCert(privateKey, key, id, username, ttl)
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

func (s *AuthServer) GenerateToken(nodeName, role string, ttl time.Duration) (string, error) {
	if !cstrings.IsValidDomainName(nodeName) {
		return "", trace.Wrap(teleport.BadParameter("nodeName",
			fmt.Sprintf("'%v' is not a valid dns name", nodeName)))
	}
	token, err := CryptoRandomHex(TokenLenBytes)
	if err != nil {
		return "", trace.Wrap(err)
	}
	outputToken, err := services.JoinTokenRole(token, role)
	if err != nil {
		return "", err
	}
	if err := s.ProvisioningService.UpsertToken(token, nodeName, role, ttl); err != nil {
		return "", err
	}
	return outputToken, nil
}

func (s *AuthServer) ValidateToken(token, domainName string) (role string, e error) {
	token, _, err := services.SplitTokenRole(token)
	if err != nil {
		return "", trace.Wrap(err)
	}
	tok, err := s.ProvisioningService.GetToken(token)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if tok.DomainName != domainName {
		return "", trace.Errorf("domainName does not match")
	}
	return tok.Role, nil
}

func (s *AuthServer) RegisterUsingToken(outputToken, nodename, role string) (keys PackedKeys, e error) {
	log.Infof("[AUTH] Node `%v` is trying to join", nodename)
	token, _, err := services.SplitTokenRole(outputToken)
	if err != nil {
		return PackedKeys{}, trace.Wrap(err)
	}
	tok, err := s.ProvisioningService.GetToken(token)
	if err != nil {
		log.Warningf("[AUTH] Node `%v` cannot join: token error. %v", nodename, err)
		return PackedKeys{}, trace.Wrap(err)
	}
	if tok.DomainName != nodename {
		return PackedKeys{}, trace.Errorf("domainName does not match")
	}

	if tok.Role != role {
		return PackedKeys{}, trace.Errorf("role does not match")
	}

	k, pub, err := s.GenerateKeyPair("")
	if err != nil {
		return PackedKeys{}, trace.Wrap(err)
	}
	fullHostName := fmt.Sprintf("%s.%s", nodename, s.Hostname)
	hostID := fmt.Sprintf("%s_%s", nodename, role)
	c, err := s.GenerateHostCert(pub, hostID, fullHostName, role, 0)
	if err != nil {
		log.Warningf("[AUTH] Node `%v` cannot join: cert generation error. %v", nodename, err)
		return PackedKeys{}, trace.Wrap(err)
	}

	keys = PackedKeys{
		Key:  k,
		Cert: c,
	}

	if err := s.DeleteToken(outputToken); err != nil {
		return PackedKeys{}, trace.Wrap(err)
	}

	utils.Consolef(os.Stdout, "[AUTH] Node `%v` joined the cluster", nodename)
	return keys, nil
}

func (s *AuthServer) RegisterNewAuthServer(domainName, outputToken string,
	publicSealKey encryptor.Key) (masterKey encryptor.Key, e error) {

	token, _, err := services.SplitTokenRole(outputToken)
	if err != nil {
		return encryptor.Key{}, trace.Wrap(err)
	}

	tok, err := s.ProvisioningService.GetToken(token)
	if err != nil {
		return encryptor.Key{}, trace.Wrap(err)
	}
	if tok.DomainName != domainName {
		return encryptor.Key{}, trace.Errorf("domainName does not match")
	}

	if tok.Role != RoleAuth {
		return encryptor.Key{}, trace.Errorf("role does not match")
	}

	if err := s.DeleteToken(outputToken); err != nil {
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

func (s *AuthServer) DeleteToken(outputToken string) error {
	token, _, err := services.SplitTokenRole(outputToken)
	if err != nil {
		return err
	}
	return s.ProvisioningService.DeleteToken(token)
}

func (s *AuthServer) NewWebSession(user string) (*Session, error) {
	token, err := CryptoRandomHex(WebSessionTokenLenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	priv, pub, err := s.GetNewKeyPairFromPool()
	if err != nil {
		return nil, err
	}
	ca, err := s.CAService.GetCertAuthority(services.CertAuthID{
		Type:       services.UserCA,
		DomainName: s.Hostname,
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privateKey, err := ca.FirstSigningKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := s.Authority.GenerateUserCert(privateKey, pub, user, user, WebSessionTTL)
	if err != nil {
		return nil, err
	}
	sess := &Session{
		ID: token,
		WS: services.WebSession{Priv: priv, Pub: cert},
	}
	return sess, nil
}

func (s *AuthServer) UpsertWebSession(user string, sess *Session, ttl time.Duration) error {
	return s.WebService.UpsertWebSession(user, sess.ID, sess.WS, ttl)
}

func (s *AuthServer) GetWebSession(user string, id string) (*Session, error) {
	ws, err := s.WebService.GetWebSession(user, id)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Session{
		ID: id,
		WS: *ws,
	}, nil
}

func (s *AuthServer) DeleteWebSession(user string, id string) error {
	return trace.Wrap(s.WebService.DeleteWebSession(user, id))
}

const (
	Week                    = time.Hour * 24 * 7
	WebSessionTTL           = time.Hour * 10
	TokenLenBytes           = 16
	WebSessionTokenLenBytes = 32
)

// CryptoRandomHex returns hex encoded random string generated with crypto-strong
// pseudo random generator of the given bytes
func CryptoRandomHex(len int) (string, error) {
	randomBytes := make([]byte, len)
	if _, err := rand.Reader.Read(randomBytes); err != nil {
		return "", trace.Wrap(err)
	}
	return hex.EncodeToString(randomBytes), nil
}
