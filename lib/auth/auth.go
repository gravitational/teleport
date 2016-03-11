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

// Package auth implements certificate signing authority and access control server
// Authority server is composed of several parts:
//
// * Authority server itself that implements signing and acl logic
// * HTTP server wrapper for authority server
// * HTTP client wrapper
//
package auth

import (
	"fmt"

	"os"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
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
	GenerateHostCert(pkey, key []byte, hostID, authDomain string, role teleport.Role, ttl time.Duration) ([]byte, error)

	// GenerateHostCert generates user certificate, it takes pkey as a signing
	// private key (user certificate authority)
	GenerateUserCert(pkey, key []byte, teleportUsername string, allowedLogins []string, ttl time.Duration) ([]byte, error)
}

// Session is a web session context, stores temporary key-value pair and session id
type Session struct {
	// ID is a session ID
	ID string `json:"id"`
	// User is a user this session belongs to
	User services.User `json:"user"`
	// WS is a private keypair used for signing requests
	WS services.WebSession `json:"web"`
}

// AuthServerOption allows setting options as functional arguments to AuthServer
type AuthServerOption func(*AuthServer)

// AuthClock allows setting clock for auth server (used in tests)
func AuthClock(clock clockwork.Clock) AuthServerOption {
	return func(a *AuthServer) {
		a.clock = clock
	}
}

// NewAuthServer creates and configures a new AuthServer instance
func NewAuthServer(cfg *InitConfig, opts ...AuthServerOption) *AuthServer {
	as := AuthServer{
		bk:                  cfg.Backend,
		Authority:           cfg.Authority,
		CAService:           services.NewCAService(cfg.Backend),
		LockService:         services.NewLockService(cfg.Backend),
		PresenceService:     services.NewPresenceService(cfg.Backend),
		ProvisioningService: services.NewProvisioningService(cfg.Backend),
		WebService:          services.NewWebService(cfg.Backend),
		BkKeysService:       services.NewBkKeysService(cfg.Backend),
		DomainName:          cfg.DomainName,
		AuthServiceName:     cfg.AuthServiceName,
	}
	for _, o := range opts {
		o(&as)
	}
	if as.clock == nil {
		as.clock = clockwork.NewRealClock()
	}
	log.Infof("[AUTH] AuthServer '%v' is created signing as '%v'", as.AuthServiceName, as.DomainName)
	return &as
}

// AuthServer keeps the cluster together. It acts as a certificate authority (CA) for
// a cluster and:
//   - generates the keypair for the node it's running on
//	 - invites other SSH nodes to a cluster, by issuing invite tokens
//	 - adds other SSH nodes to a cluster, by checking their token and signing their keys
//   - same for users and their sessions
//   - checks public keys to see if they're signed by it (can be trusted or not)
type AuthServer struct {
	clock clockwork.Clock
	bk    backend.Backend
	Authority

	// DomainName stores the FQDN of the signing CA (its certificate will have this
	// name embedded). It is usually set to the GUID of the host the Auth service runs on
	DomainName string

	// AuthServiceName is a human-readable name of this CA. If several Auth services are running
	// (managing multiple teleport clusters) this field is used to tell them apart in UIs
	// It usually defaults to the hostname of the machine the Auth service runs on.
	AuthServiceName string

	*services.CAService
	*services.LockService
	*services.PresenceService
	*services.ProvisioningService
	*services.WebService
	*services.BkKeysService
}

// GetLocalDomain returns domain name that identifies this authority server
func (a *AuthServer) GetLocalDomain() (string, error) {
	return a.DomainName, nil
}

// GenerateHostCert generates host certificate, it takes pkey as a signing
// private key (host certificate authority)
func (s *AuthServer) GenerateHostCert(key []byte, hostID, authDomain string, role teleport.Role, ttl time.Duration) ([]byte, error) {
	ca, err := s.CAService.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: s.DomainName,
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privateKey, err := ca.FirstSigningKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return s.Authority.GenerateHostCert(privateKey, key, hostID, authDomain, role, ttl)
}

// GenerateUserCert generates user certificate, it takes pkey as a signing
// private key (user certificate authority)
func (s *AuthServer) GenerateUserCert(
	key []byte, username string, ttl time.Duration) ([]byte, error) {

	ca, err := s.CAService.GetCertAuthority(services.CertAuthID{
		Type:       services.UserCA,
		DomainName: s.DomainName,
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privateKey, err := ca.FirstSigningKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := s.GetUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return s.Authority.GenerateUserCert(privateKey, key, username, user.AllowedLogins, ttl)
}

func (s *AuthServer) SignIn(user string, password []byte) (*Session, error) {
	if err := s.CheckPasswordWOToken(user, password); err != nil {
		return nil, trace.Wrap(err)
	}
	sess, err := s.NewWebSession(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.UpsertWebSession(user, sess, WebSessionTTL); err != nil {
		return nil, trace.Wrap(err)
	}
	sess.WS.Priv = nil
	return sess, nil
}

// CreateWebSession creates a new web session for a user based on a valid previous sessionID,
// method is used to renew the web session for a user
func (s *AuthServer) CreateWebSession(user string, prevSessionID string) (*Session, error) {
	_, err := s.GetWebSession(user, prevSessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess, err := s.NewWebSession(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.UpsertWebSession(user, sess, WebSessionTTL); err != nil {
		return nil, trace.Wrap(err)
	}
	sess.WS.Priv = nil
	return sess, nil
}

func (s *AuthServer) GenerateToken(role teleport.Role, ttl time.Duration) (string, error) {
	if err := role.Check(); err != nil {
		return "", trace.Wrap(err)
	}
	token, err := utils.CryptoRandomHex(TokenLenBytes)
	if err != nil {
		return "", trace.Wrap(err)
	}
	outputToken, err := services.JoinTokenRole(token, string(role))
	if err != nil {
		return "", err
	}
	if err := s.ProvisioningService.UpsertToken(token, string(role), ttl); err != nil {
		return "", err
	}
	return outputToken, nil
}

func (s *AuthServer) ValidateToken(token string) (role string, e error) {
	token, _, err := services.SplitTokenRole(token)
	if err != nil {
		return "", trace.Wrap(err)
	}
	tok, err := s.ProvisioningService.GetToken(token)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return tok.Role, nil
}

func (s *AuthServer) RegisterUsingToken(outputToken, hostID string, role teleport.Role) (keys PackedKeys, e error) {
	log.Infof("[AUTH] Node `%v` is trying to join", hostID)
	if hostID == "" {
		return PackedKeys{}, trace.Wrap(fmt.Errorf("HostID cannot be empty"))
	}
	if err := role.Check(); err != nil {
		return PackedKeys{}, trace.Wrap(err)
	}
	token, _, err := services.SplitTokenRole(outputToken)
	if err != nil {
		return PackedKeys{}, trace.Wrap(err)
	}
	tok, err := s.ProvisioningService.GetToken(token)
	if err != nil {
		log.Warningf("[AUTH] Node `%v` cannot join: token error. %v", hostID, err)
		return PackedKeys{}, trace.Wrap(err)
	}
	if tok.Role != string(role) {
		return PackedKeys{}, trace.Wrap(
			teleport.BadParameter("token.Role", "role does not match"))
	}
	k, pub, err := s.GenerateKeyPair("")
	if err != nil {
		return PackedKeys{}, trace.Wrap(err)
	}
	// we always append authority's domain to resulting node name,
	// that's how we make sure that nodes are uniquely identified/found
	// in cases when we have multiple environments/organizations
	fqdn := fmt.Sprintf("%s.%s", hostID, s.DomainName)
	c, err := s.GenerateHostCert(pub, fqdn, s.DomainName, role, 0)
	if err != nil {
		log.Warningf("[AUTH] Node `%v` cannot join: cert generation error. %v", hostID, err)
		return PackedKeys{}, trace.Wrap(err)
	}

	keys = PackedKeys{
		Key:  k,
		Cert: c,
	}

	if err := s.DeleteToken(outputToken); err != nil {
		return PackedKeys{}, trace.Wrap(err)
	}

	utils.Consolef(os.Stdout, "[AUTH] Node `%v` joined the cluster", hostID)
	return keys, nil
}

func (s *AuthServer) RegisterNewAuthServer(outputToken string) error {

	token, _, err := services.SplitTokenRole(outputToken)
	if err != nil {
		return trace.Wrap(err)
	}

	tok, err := s.ProvisioningService.GetToken(token)
	if err != nil {
		return trace.Wrap(err)
	}

	if tok.Role != string(teleport.RoleAuth) {
		return trace.Wrap(teleport.AccessDenied("role does not match"))
	}

	if err := s.DeleteToken(outputToken); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *AuthServer) DeleteToken(outputToken string) error {
	token, _, err := services.SplitTokenRole(outputToken)
	if err != nil {
		return err
	}
	return s.ProvisioningService.DeleteToken(token)
}

func (s *AuthServer) NewWebSession(userName string) (*Session, error) {
	token, err := utils.CryptoRandomHex(WebSessionTokenLenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	bearerToken, err := utils.CryptoRandomHex(WebSessionTokenLenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	priv, pub, err := s.GetNewKeyPairFromPool()
	if err != nil {
		return nil, err
	}
	ca, err := s.CAService.GetCertAuthority(services.CertAuthID{
		Type:       services.UserCA,
		DomainName: s.DomainName,
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privateKey, err := ca.FirstSigningKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := s.GetUser(userName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := s.Authority.GenerateUserCert(privateKey, pub, user.Name, user.AllowedLogins, WebSessionTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess := &Session{
		ID:   token,
		User: *user,
		WS: services.WebSession{
			Priv:        priv,
			Pub:         cert,
			Expires:     s.clock.Now().UTC().Add(WebSessionTTL),
			BearerToken: bearerToken,
		},
	}
	return sess, nil
}

func (s *AuthServer) UpsertWebSession(user string, sess *Session, ttl time.Duration) error {
	return s.WebService.UpsertWebSession(user, sess.ID, sess.WS, ttl)
}

func (s *AuthServer) GetWebSession(userName string, id string) (*Session, error) {
	ws, err := s.WebService.GetWebSession(userName, id)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := s.GetUser(userName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Session{
		ID:   id,
		User: *user,
		WS:   *ws,
	}, nil
}

func (s *AuthServer) GetWebSessionInfo(userName string, id string) (*Session, error) {
	sess, err := s.WebService.GetWebSession(userName, id)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := s.GetUser(userName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess.Priv = nil
	return &Session{
		ID:   id,
		User: *user,
		WS:   *sess,
	}, nil
}

func (s *AuthServer) DeleteWebSession(user string, id string) error {
	return trace.Wrap(s.WebService.DeleteWebSession(user, id))
}

const (
	// WebSessionTTL specifies standard web session time to live
	WebSessionTTL = 10 * time.Minute
	// TokenLenBytes is len in bytes of the invite token
	TokenLenBytes = 16
	// WebSessionTokenLenBytes specifies len in bytes of the
	// web session random token
	WebSessionTokenLenBytes = 32
)
