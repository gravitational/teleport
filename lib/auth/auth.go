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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/coreos/go-oidc/jose"
	"github.com/coreos/go-oidc/oauth2"
	"github.com/coreos/go-oidc/oidc"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/tstranex/u2f"
)

// Authority implements minimal key-management facility for generating OpenSSH
// compatible public/private key pairs and OpenSSH certificates
type Authority interface {
	// GenerateKeyPair generates new keypair
	GenerateKeyPair(passphrase string) (privKey []byte, pubKey []byte, err error)

	// GetNewKeyPairFromPool returns new keypair from pre-generated in memory pool
	GetNewKeyPairFromPool() (privKey []byte, pubKey []byte, err error)

	// GenerateHostCert takes the private key of the CA, public key of the new host,
	// along with metadata (host ID, node name, cluster name, roles, and ttl) and generates
	// a host certificate.
	GenerateHostCert(certParams services.CertParams) ([]byte, error)

	// GenerateUserCert generates user certificate, it takes pkey as a signing
	// private key (user certificate authority)
	GenerateUserCert(pkey, key []byte, teleportUsername string, allowedLogins []string, ttl time.Duration, permitAgentForwarding bool) ([]byte, error)
}

// AuthServerOption allows setting options as functional arguments to AuthServer
type AuthServerOption func(*AuthServer)

// NewAuthServer creates and configures a new AuthServer instance
func NewAuthServer(cfg *InitConfig, opts ...AuthServerOption) *AuthServer {
	if cfg.Trust == nil {
		cfg.Trust = local.NewCAService(cfg.Backend)
	}
	if cfg.Presence == nil {
		cfg.Presence = local.NewPresenceService(cfg.Backend)
	}
	if cfg.Provisioner == nil {
		cfg.Provisioner = local.NewProvisioningService(cfg.Backend)
	}
	if cfg.Identity == nil {
		cfg.Identity = local.NewIdentityService(cfg.Backend)
	}
	if cfg.Access == nil {
		cfg.Access = local.NewAccessService(cfg.Backend)
	}
	if cfg.ClusterAuthPreferenceService == nil {
		cfg.ClusterAuthPreferenceService = local.NewClusterAuthPreferenceService(cfg.Backend)
	}
	if cfg.UniversalSecondFactorService == nil {
		cfg.UniversalSecondFactorService = local.NewUniversalSecondFactorService(cfg.Backend)
	}

	as := AuthServer{
		bk:                            cfg.Backend,
		Authority:                     cfg.Authority,
		Trust:                         cfg.Trust,
		Presence:                      cfg.Presence,
		Provisioner:                   cfg.Provisioner,
		Identity:                      cfg.Identity,
		Access:                        cfg.Access,
		DomainName:                    cfg.DomainName,
		AuthServiceName:               cfg.AuthServiceName,
		StaticTokens:                  cfg.StaticTokens,
		ClusterAuthPreference:         cfg.ClusterAuthPreferenceService,
		UniversalSecondFactorSettings: cfg.UniversalSecondFactorService,
		oidcClients:                   make(map[string]*oidcClient),
		DeveloperMode:                 cfg.DeveloperMode,
	}
	for _, o := range opts {
		o(&as)
	}
	if as.clock == nil {
		as.clock = clockwork.NewRealClock()
	}
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
	lock        sync.Mutex
	oidcClients map[string]*oidcClient
	clock       clockwork.Clock
	bk          backend.Backend

	// DeveloperMode should only be used during development as it does several
	// unsafe things like log sensitive information to console as well as
	// not verify certificates.
	DeveloperMode bool

	Authority

	// DomainName stores the FQDN of the signing CA (its certificate will have this
	// name embedded). It is usually set to the GUID of the host the Auth service runs on
	DomainName string

	// AuthServiceName is a human-readable name of this CA. If several Auth services are running
	// (managing multiple teleport clusters) this field is used to tell them apart in UIs
	// It usually defaults to the hostname of the machine the Auth service runs on.
	AuthServiceName string

	// StaticTokens are pre-defined host provisioning tokens supplied via config file for
	// environments where paranoid security is not needed
	StaticTokens []services.ProvisionToken

	services.Trust
	services.Presence
	services.Provisioner
	services.Identity
	services.Access
	services.ClusterAuthPreference
	services.UniversalSecondFactorSettings
}

func (a *AuthServer) Close() error {
	if a.bk != nil {
		return trace.Wrap(a.bk.Close())
	}
	return nil
}

// GetDomainName returns the domain name that identifies this authority server.
// Also known as "cluster name"
func (a *AuthServer) GetDomainName() (string, error) {
	return a.DomainName, nil
}

// GenerateHostCert uses the private key of the CA to sign the public key of the host
// (along with meta data like host ID, node name, roles, and ttl) to generate a host certificate.
func (s *AuthServer) GenerateHostCert(hostPublicKey []byte, hostID, nodeName, clusterName string, roles teleport.Roles, ttl time.Duration) ([]byte, error) {
	// get the certificate authority that will be signing the public key of the host
	ca, err := s.Trust.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: s.DomainName,
	}, true)
	if err != nil {
		return nil, trace.BadParameter("failed to load host CA for '%s': %v", s.DomainName, err)
	}

	// get the private key of the certificate authority
	caPrivateKey, err := ca.FirstSigningKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// create and sign!
	return s.Authority.GenerateHostCert(services.CertParams{
		PrivateCASigningKey: caPrivateKey,
		PublicHostKey:       hostPublicKey,
		HostID:              hostID,
		NodeName:            nodeName,
		ClusterName:         clusterName,
		Roles:               roles,
		TTL:                 ttl,
	})
}

// GenerateUserCert generates user certificate, it takes pkey as a signing
// private key (user certificate authority)
func (s *AuthServer) GenerateUserCert(key []byte, username string, allowedLogins []string, ttl time.Duration, canForwardAgents bool) ([]byte, error) {
	ca, err := s.Trust.GetCertAuthority(services.CertAuthID{
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
	return s.Authority.GenerateUserCert(privateKey, key, username, allowedLogins, ttl, canForwardAgents)
}

// withUserLock executes function authenticateFn that perorms user authenticaton
// if authenticateFn returns non nil error, the login attempt will be logged in as failed.
// The only exception to this rule is ConnectionProblemError, in case if it occurs
// access will be denied, but login attempt will not be recorded
// this is done to avoid potential user lockouts due to backend failures
// In case if user exceeds defaults.MaxLoginAttempts
// the user account will be locked for defaults.AccountLockInterval
func (s *AuthServer) withUserLock(username string, authenticateFn func() error) error {
	user, err := s.Identity.GetUser(username)
	if err != nil {
		return trace.Wrap(err)
	}
	status := user.GetStatus()
	if status.IsLocked && status.LockExpires.After(s.clock.Now().UTC()) {
		return trace.AccessDenied("user %v is locked until %v", utils.HumanTimeFormat(status.LockExpires))
	}
	fnErr := authenticateFn()
	if fnErr == nil {
		return nil
	}
	// do not lock user in case if DB is flaky or down
	if trace.IsConnectionProblem(err) {
		return trace.Wrap(fnErr)
	}
	// log failed attempt and possibly lock user
	attempt := services.LoginAttempt{Time: s.clock.Now().UTC(), Success: false}
	err = s.AddUserLoginAttempt(username, attempt, defaults.AttemptTTL)
	if err != nil {
		log.Error(trace.DebugReport(err))
		return trace.Wrap(fnErr)
	}
	loginAttempts, err := s.Identity.GetUserLoginAttempts(username)
	if err != nil {
		log.Error(trace.DebugReport(err))
		return trace.Wrap(fnErr)
	}
	if !services.LastFailed(defaults.MaxLoginAttempts, loginAttempts) {
		log.Debugf("%v user has less than %v failed login attempts", username, defaults.MaxLoginAttempts)
		return trace.Wrap(fnErr)
	}
	lockUntil := s.clock.Now().UTC().Add(defaults.AccountLockInterval)
	message := fmt.Sprintf("%v exceeds %v failed login attempts, locked until %v",
		username, defaults.MaxLoginAttempts, utils.HumanTimeFormat(status.LockExpires))
	log.Debug(message)
	user.SetLocked(lockUntil, "user has exceeded maximum failed login attempts")
	err = s.Identity.UpsertUser(user)
	if err != nil {
		log.Error(trace.DebugReport(err))
		return trace.Wrap(fnErr)
	}
	return trace.AccessDenied(message)
}

func (s *AuthServer) SignIn(user string, password []byte) (services.WebSession, error) {
	err := s.withUserLock(user, func() error {
		return s.CheckPasswordWOToken(user, password)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return s.PreAuthenticatedSignIn(user)
}

// PreAuthenticatedSignIn is for 2-way authentication methods like U2F where the password is
// already checked before issueing the second factor challenge
func (s *AuthServer) PreAuthenticatedSignIn(user string) (services.WebSession, error) {
	sess, err := s.NewWebSession(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.UpsertWebSession(user, sess); err != nil {
		return nil, trace.Wrap(err)
	}
	return sess.WithoutSecrets(), nil
}

func (s *AuthServer) U2FSignRequest(user string, password []byte) (*u2f.SignRequest, error) {
	universalSecondFactor, err := s.GetUniversalSecondFactor()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.withUserLock(user, func() error {
		return s.CheckPasswordWOToken(user, password)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	registration, err := s.GetU2FRegistration(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	challenge, err := u2f.NewChallenge(universalSecondFactor.GetAppID(), universalSecondFactor.GetFacets())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.UpsertU2FSignChallenge(user, challenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	u2fSignReq := challenge.SignRequest(*registration)

	return u2fSignReq, nil
}

func (s *AuthServer) CheckU2FSignResponse(user string, response *u2f.SignResponse) error {
	// before trying to register a user, see U2F is actually setup on the backend
	_, err := s.GetUniversalSecondFactor()
	if err != nil {
		return trace.Wrap(err)
	}

	reg, err := s.GetU2FRegistration(user)
	if err != nil {
		return trace.Wrap(err)
	}

	counter, err := s.GetU2FRegistrationCounter(user)
	if err != nil {
		return trace.Wrap(err)
	}

	challenge, err := s.GetU2FSignChallenge(user)
	if err != nil {
		return trace.Wrap(err)
	}

	newCounter, err := reg.Authenticate(*response, *challenge, counter)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.UpsertU2FRegistrationCounter(user, newCounter)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// ExtendWebSession creates a new web session for a user based on a valid previous sessionID,
// method is used to renew the web session for a user
func (s *AuthServer) ExtendWebSession(user string, prevSessionID string) (services.WebSession, error) {
	prevSession, err := s.GetWebSession(user, prevSessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// consider absolute expiry time that may be set for this session
	// by some external identity serivce, so we can not renew this session
	// any more without extra logic for renewal with external OIDC provider
	expiresAt := prevSession.GetExpiryTime()
	if !expiresAt.IsZero() && expiresAt.Before(s.clock.Now().UTC()) {
		return nil, trace.NotFound("web session has expired")
	}

	sess, err := s.NewWebSession(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess.SetExpiryTime(expiresAt)
	bearerTokenTTL := utils.MinTTL(utils.ToTTL(s.clock, expiresAt), BearerTokenTTL)
	sess.SetBearerTokenExpiryTime(s.clock.Now().UTC().Add(bearerTokenTTL))
	if err := s.UpsertWebSession(user, sess); err != nil {
		return nil, trace.Wrap(err)
	}
	sess, err = services.GetWebSessionMarshaler().ExtendWebSession(sess)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sess, nil
}

// CreateWebSession creates a new web session for user without any
// checks, is used by admins
func (s *AuthServer) CreateWebSession(user string) (services.WebSession, error) {
	sess, err := s.NewWebSession(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.UpsertWebSession(user, sess); err != nil {
		return nil, trace.Wrap(err)
	}
	sess, err = services.GetWebSessionMarshaler().GenerateWebSession(sess)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sess, nil
}

func (s *AuthServer) GenerateToken(roles teleport.Roles, ttl time.Duration) (string, error) {
	for _, role := range roles {
		if err := role.Check(); err != nil {
			return "", trace.Wrap(err)
		}
	}
	token, err := utils.CryptoRandomHex(TokenLenBytes)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if err := s.Provisioner.UpsertToken(token, roles, ttl); err != nil {
		return "", err
	}
	return token, nil
}

// GenerateServerKeys generates new host private keys and certificates (signed
// by the host certificate authority) for a node.
func (s *AuthServer) GenerateServerKeys(hostID string, nodeName string, roles teleport.Roles) (*PackedKeys, error) {
	// generate private key
	k, pub, err := s.GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// generate host certificate with an infinite ttl
	c, err := s.GenerateHostCert(pub, hostID, nodeName, s.DomainName, roles, 0)
	if err != nil {
		log.Warningf("[AUTH] Node %q [%v] can not join: certificate generation error: %v", nodeName, hostID, err)
		return nil, trace.Wrap(err)
	}

	return &PackedKeys{
		Key:  k,
		Cert: c,
	}, nil
}

// ValidateToken takes a provisioning token value and finds if it's valid. Returns
// a list of roles this token allows its owner to assume, or an error if the token
// cannot be found
func (s *AuthServer) ValidateToken(token string) (roles teleport.Roles, e error) {
	// look at static tokens first:
	for _, st := range s.StaticTokens {
		if st.Token == token {
			return st.Roles, nil
		}
	}
	// look at the tokens in the token storage
	tok, err := s.Provisioner.GetToken(token)
	if err != nil {
		log.Info(err)
		return nil, trace.Errorf("token not recognized")
	}
	return tok.Roles, nil
}

// enforceTokenTTL deletes the given token if it's TTL is over. Returns 'false'
// if this token cannot be used
func (s *AuthServer) checkTokenTTL(token string) bool {
	// look at the tokens in the token storage
	tok, err := s.Provisioner.GetToken(token)
	if err != nil {
		log.Warn(err)
		return true
	}
	now := s.clock.Now().UTC()
	if tok.Expires.Before(now) {
		if err = s.DeleteToken(token); err != nil {
			log.Error(err)
		}
		return false
	}
	return true
}

// RegisterUsingToken adds a new node to the Teleport cluster using previously issued token.
// A node must also request a specific role (and the role must match one of the roles
// the token was generated for).
//
// If a token was generated with a TTL, it gets enforced (can't register new nodes after TTL expires)
// If a token was generated with a TTL=0, it means it's a single-use token and it gets destroyed
// after a successful registration.
func (s *AuthServer) RegisterUsingToken(token, hostID string, nodeName string, role teleport.Role) (*PackedKeys, error) {
	log.Infof("[AUTH] Node %q [%v] trying to join with role: %v", nodeName, hostID, role)
	if hostID == "" {
		return nil, trace.BadParameter("HostID cannot be empty")
	}

	if err := role.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	// make sure the token is valid
	roles, err := s.ValidateToken(token)
	if err != nil {
		msg := fmt.Sprintf("%q [%v] can not join the cluster with role %s, token error: %v", nodeName, hostID, role, err)
		log.Warnf("[AUTH] %s", msg)
		return nil, trace.AccessDenied(msg)
	}

	// make sure the caller is requested wthe role allowed by the token
	if !roles.Include(role) {
		msg := fmt.Sprintf("%q [%v] can not join the cluster, the token does not allow %q role", nodeName, hostID, role)
		log.Warningf("[AUTH] %s", msg)
		return nil, trace.BadParameter(msg)
	}
	if !s.checkTokenTTL(token) {
		return nil, trace.AccessDenied("%q [%v] can not join the cluster. Token has expired", nodeName, hostID)
	}

	// generate and return host certificate and keys
	keys, err := s.GenerateServerKeys(hostID, nodeName, teleport.Roles{role})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.Infof("[AUTH] Node %q [%v] joined the cluster", nodeName, hostID)

	return keys, nil
}

func (s *AuthServer) RegisterNewAuthServer(token string) error {
	tok, err := s.Provisioner.GetToken(token)
	if err != nil {
		return trace.Wrap(err)
	}
	if !tok.Roles.Include(teleport.RoleAuth) {
		return trace.AccessDenied("role does not match")
	}
	if err := s.DeleteToken(token); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *AuthServer) DeleteToken(token string) (err error) {
	// is this a static token?
	for _, st := range s.StaticTokens {
		if st.Token == token {
			return trace.BadParameter("token %s is statically configured and cannot be removed", token)
		}
	}
	// delete user token:
	if err = s.Identity.DeleteSignupToken(token); err == nil {
		return nil
	}
	// delete node token:
	if err = s.Provisioner.DeleteToken(token); err == nil {
		return nil
	}
	return trace.Wrap(err)
}

// GetTokens returns all tokens (machine provisioning ones and user invitation tokens). Machine
// tokens usually have "node roles", like auth,proxy,node and user invitation tokens have 'signup' role
func (s *AuthServer) GetTokens() (tokens []services.ProvisionToken, err error) {
	// get node tokens:
	tokens, err = s.Provisioner.GetTokens()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// get static tokens:
	tokens = append(tokens, s.StaticTokens...)
	// get user tokens:
	userTokens, err := s.Identity.GetSignupTokens()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// convert user tokens to machine tokens:
	for _, t := range userTokens {
		roles := teleport.Roles{teleport.RoleSignup}
		tokens = append(tokens, services.ProvisionToken{
			Token:   t.Token,
			Expires: t.Expires,
			Roles:   roles,
		})
	}
	return tokens, nil
}

func (s *AuthServer) NewWebSession(userName string) (services.WebSession, error) {
	token, err := utils.CryptoRandomHex(TokenLenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	bearerToken, err := utils.CryptoRandomHex(TokenLenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	priv, pub, err := s.GetNewKeyPairFromPool()
	if err != nil {
		return nil, err
	}
	ca, err := s.Trust.GetCertAuthority(services.CertAuthID{
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
	var roles services.RoleSet
	for _, roleName := range user.GetRoles() {
		role, err := s.Access.GetRole(roleName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roles = append(roles, role)
	}
	sessionTTL := roles.AdjustSessionTTL(defaults.CertDuration)
	bearerTokenTTL := utils.MinTTL(sessionTTL, BearerTokenTTL)

	allowedLogins, err := roles.CheckLogins(sessionTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// cert TTL is set to bearer token TTL as we expect active session to renew
	// the token every BearerTokenTTL period
	cert, err := s.Authority.GenerateUserCert(privateKey, pub, user.GetName(), allowedLogins, bearerTokenTTL, roles.CanForwardAgents())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.NewWebSession(token, services.WebSessionSpecV2{
		User:               user.GetName(),
		Priv:               priv,
		Pub:                cert,
		Expires:            s.clock.Now().UTC().Add(sessionTTL),
		BearerToken:        bearerToken,
		BearerTokenExpires: s.clock.Now().UTC().Add(bearerTokenTTL),
	}), nil
}

func (s *AuthServer) UpsertWebSession(user string, sess services.WebSession) error {
	return s.Identity.UpsertWebSession(user, sess.GetName(), sess)
}

func (s *AuthServer) GetWebSession(userName string, id string) (services.WebSession, error) {
	return s.Identity.GetWebSession(userName, id)
}

func (s *AuthServer) GetWebSessionInfo(userName string, id string) (services.WebSession, error) {
	sess, err := s.Identity.GetWebSession(userName, id)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sess.WithoutSecrets(), nil
}

func (s *AuthServer) DeleteNamespace(namespace string) error {
	if namespace == defaults.Namespace {
		return trace.AccessDenied("can't delete default namespace")
	}
	nodes, err := s.Presence.GetNodes(namespace)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(nodes) != 0 {
		return trace.BadParameter("can't delete namespace %v that has %v registered nodes", namespace, len(nodes))
	}
	return s.Presence.DeleteNamespace(namespace)
}

func (s *AuthServer) DeleteWebSession(user string, id string) error {
	return trace.Wrap(s.Identity.DeleteWebSession(user, id))
}

func (s *AuthServer) getOIDCClient(conn services.OIDCConnector) (*oidc.Client, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	config := oidc.ClientConfig{
		RedirectURL: conn.GetRedirectURL(),
		Credentials: oidc.ClientCredentials{
			ID:     conn.GetClientID(),
			Secret: conn.GetClientSecret(),
		},
		// open id notifies provider that we are using OIDC scopes
		Scope: utils.Deduplicate(append([]string{"openid", "email"}, conn.GetScope()...)),
	}

	clientPack, ok := s.oidcClients[conn.GetName()]
	if ok && oidcConfigsEqual(clientPack.config, config) {
		return clientPack.client, nil
	}
	delete(s.oidcClients, conn.GetName())

	client, err := oidc.NewClient(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client.SyncProviderConfig(conn.GetIssuerURL())

	s.oidcClients[conn.GetName()] = &oidcClient{client: client, config: config}

	return client, nil
}

func (s *AuthServer) UpsertOIDCConnector(connector services.OIDCConnector, ttl time.Duration) error {
	return s.Identity.UpsertOIDCConnector(connector, ttl)
}

func (s *AuthServer) DeleteOIDCConnector(connectorName string) error {
	return s.Identity.DeleteOIDCConnector(connectorName)
}

func (s *AuthServer) CreateOIDCAuthRequest(req services.OIDCAuthRequest) (*services.OIDCAuthRequest, error) {
	connector, err := s.Identity.GetOIDCConnector(req.ConnectorID, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	oidcClient, err := s.getOIDCClient(connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := utils.CryptoRandomHex(TokenLenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.StateToken = token

	oauthClient, err := oidcClient.OAuthClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// online is OIDC online scope, "select_account" forces user to always select account
	redirectURL := oauthClient.AuthCodeURL(req.StateToken, "online", "select_account")
	req.RedirectURL = redirectURL

	err = s.Identity.CreateOIDCAuthRequest(req, defaults.OIDCAuthRequestTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &req, nil
}

// OIDCAuthResponse is returned when auth server validated callback parameters
// returned from OIDC provider
type OIDCAuthResponse struct {
	// Username is authenticated teleport username
	Username string `json:"username"`
	// Identity contains validated OIDC identity
	Identity services.OIDCIdentity `json:"identity"`
	// Web session will be generated by auth server if requested in OIDCAuthRequest
	Session services.WebSession `json:"session,omitempty"`
	// Cert will be generated by certificate authority
	Cert []byte `json:"cert,omitempty"`
	// Req is original oidc auth request
	Req services.OIDCAuthRequest `json:"req"`
	// HostSigners is a list of signing host public keys
	// trusted by proxy, used in console login
	HostSigners []services.CertAuthority `json:"host_signers"`
}

func (a *AuthServer) createOIDCUser(connector services.OIDCConnector, ident *oidc.Identity, claims jose.Claims) error {
	roles := connector.MapClaims(claims)
	if len(roles) == 0 {
		log.Warningf("[OIDC] could not find any of expected claims: %v in the set returned by provider %v: %v",
			strings.Join(connector.GetClaims(), ","), connector.GetName(), strings.Join(services.GetClaimNames(claims), ","))
		return trace.AccessDenied("access denied to %v", ident.Email)
	}
	log.Debugf("[IDENTITY] %v/%v is a dynamic identity, generating user with roles: %v", connector.GetName(), ident.Email, roles)
	user, err := services.GetUserMarshaler().GenerateUser(&services.UserV2{
		Kind:    services.KindUser,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      ident.Email,
			Namespace: defaults.Namespace,
		},
		Spec: services.UserSpecV2{
			Roles:          roles,
			Expires:        ident.ExpiresAt,
			OIDCIdentities: []services.OIDCIdentity{{ConnectorID: connector.GetName(), Email: ident.Email}},
			CreatedBy: services.CreatedBy{
				User: services.UserRef{Name: "system"},
				Time: time.Now().UTC(),
				Connector: &services.ConnectorRef{
					Type:     teleport.ConnectorOIDC,
					ID:       connector.GetName(),
					Identity: ident.Email,
				},
			},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	err = a.CreateUser(user)
	if err == nil {
		return trace.Wrap(err)
	}
	if !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}
	existingUser, err := a.GetUser(ident.Email)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	} else {
		connectorRef := existingUser.GetCreatedBy().Connector
		if connectorRef == nil || connectorRef.Type != teleport.ConnectorOIDC || connectorRef.ID != connector.GetName() {
			return trace.AlreadyExists("user %v already exists and is not OIDC user", existingUser.GetName())
		}
	}
	return a.UpsertUser(user)
}

// claimsFromIDToken extracts claims from the ID token.
func claimsFromIDToken(oidcClient *oidc.Client, idToken string) (jose.Claims, error) {
	jwt, err := jose.ParseJWT(idToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = oidcClient.VerifyJWT(jwt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.Debugf("[OIDC] Extracting claims from ID token")

	claims, err := jwt.Claims()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return claims, nil
}

// claimsFromUserInfo finds the UserInfo endpoint from the provider config and then extracts claims from it.
//
// Note: We don't request signed JWT responses for UserInfo, instead we force the provider config and
// the issuer to be HTTPS and leave integrity and confidentiality to TLS. Authenticity is taken care of
// during the token exchange.
func claimsFromUserInfo(oidcClient *oidc.Client, issuerURL string, accessToken string) (jose.Claims, error) {
	err := isHTTPS(issuerURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	oac, err := oidcClient.OAuthClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	hc := oac.HttpClient()

	// go get the provider config so we can find out where the UserInfo endpoint is
	pc, err := oidc.FetchProviderConfig(oac.HttpClient(), issuerURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	endpoint := pc.UserInfoEndpoint.String()
	err = isHTTPS(endpoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.Debugf("[OIDC] Fetching claims from UserInfo endpoint: %q", endpoint)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp, err := hc.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, trace.AccessDenied("bad status code: %v", resp.StatusCode)
	}

	var claims jose.Claims
	err = json.NewDecoder(resp.Body).Decode(&claims)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return claims, nil
}

// mergeClaims merges b into a.
func mergeClaims(a jose.Claims, b jose.Claims) (jose.Claims, error) {
	for k, v := range b {
		_, ok := a[k]
		if !ok {
			a[k] = v
		}
	}

	return a, nil
}

// getClaims gets claims from ID token and UserInfo and returns UserInfo claims merged into ID token claims.
func (a *AuthServer) getClaims(oidcClient *oidc.Client, issuerURL string, code string) (jose.Claims, error) {
	var err error

	oac, err := oidcClient.OAuthClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	t, err := oac.RequestToken(oauth2.GrantTypeAuthCode, code)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	idTokenClaims, err := claimsFromIDToken(oidcClient, t.IDToken)
	if err != nil {
		log.Debugf("[OIDC] Unable to fetch ID token claims: %v", err)
		return nil, trace.Wrap(err)
	}
	log.Debugf("[OIDC] ID Token claims: %v", idTokenClaims)

	userInfoClaims, err := claimsFromUserInfo(oidcClient, issuerURL, t.AccessToken)
	if err != nil {
		log.Debugf("[OIDC] Unable to fetch UserInfo claims: %v", err)
		return nil, trace.Wrap(err)
	}
	log.Debugf("[OIDC] UserInfo claims: %v", userInfoClaims)

	// make sure that the subject in the userinfo claim matches the subject in
	// the id token otherwise there is the possibility of a token substitution attack.
	// see section 16.11 of the oidc spec for more details.
	var idsub string
	var uisub string
	var exists bool
	if idsub, exists, err = idTokenClaims.StringClaim("sub"); err != nil || !exists {
		log.Debugf("[OIDC] unable to extract sub from ID token")
		return nil, trace.Wrap(err)
	}
	if uisub, exists, err = userInfoClaims.StringClaim("sub"); err != nil || !exists {
		log.Debugf("[OIDC] unable to extract sub from UserInfo")
		return nil, trace.Wrap(err)
	}
	if idsub != uisub {
		log.Debugf("[OIDC] Claim subjects don't match %q != %q", idsub, uisub)
		return nil, trace.BadParameter("invalid subject in UserInfo")
	}

	claims, err := mergeClaims(idTokenClaims, userInfoClaims)
	if err != nil {
		log.Debugf("[OIDC] Unable to merge claims: %v", err)
		return nil, trace.Wrap(err)
	}

	return claims, nil
}

// ValidateOIDCAuthCallback is called by the proxy to check OIDC query parameters
// returned by OIDC Provider, if everything checks out, auth server
// will respond with OIDCAuthResponse, otherwise it will return error
func (a *AuthServer) ValidateOIDCAuthCallback(q url.Values) (*OIDCAuthResponse, error) {
	if error := q.Get("error"); error != "" {
		return nil, trace.OAuth2(oauth2.ErrorInvalidRequest, error, q)
	}

	code := q.Get("code")
	if code == "" {
		return nil, trace.OAuth2(
			oauth2.ErrorInvalidRequest, "code query param must be set", q)
	}

	stateToken := q.Get("state")
	if stateToken == "" {
		return nil, trace.OAuth2(
			oauth2.ErrorInvalidRequest, "missing state query param", q)
	}

	req, err := a.Identity.GetOIDCAuthRequest(stateToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connector, err := a.Identity.GetOIDCConnector(req.ConnectorID, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	oidcClient, err := a.getOIDCClient(connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// extract claims from both the id token and the userinfo endpoint and merge them
	claims, err := a.getClaims(oidcClient, connector.GetIssuerURL(), code)
	if err != nil {
		return nil, trace.OAuth2(
			oauth2.ErrorUnsupportedResponseType, "unable to construct claims", q)
	}
	log.Debugf("[OIDC] Claims: %v", claims)

	ident, err := oidc.IdentityFromClaims(claims)
	if err != nil {
		return nil, trace.OAuth2(
			oauth2.ErrorUnsupportedResponseType, "unable to convert claims to identity", q)
	}
	log.Debugf("[IDENTITY] %q expires at: %v", ident.Email, ident.ExpiresAt)

	response := &OIDCAuthResponse{
		Identity: services.OIDCIdentity{ConnectorID: connector.GetName(), Email: ident.Email},
		Req:      *req,
	}

	log.Debugf("[OIDC] Applying %v claims to roles mappings", len(connector.GetClaimsToRoles()))
	if len(connector.GetClaimsToRoles()) != 0 {
		if err := a.createOIDCUser(connector, ident, claims); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if !req.CheckUser {
		return response, nil
	}

	user, err := a.Identity.GetUserByOIDCIdentity(services.OIDCIdentity{
		ConnectorID: req.ConnectorID, Email: ident.Email})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	response.Username = user.GetName()

	var roles services.RoleSet
	roles, err = services.FetchRoles(user.GetRoles(), a.Access)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sessionTTL := roles.AdjustSessionTTL(utils.ToTTL(a.clock, ident.ExpiresAt))
	bearerTokenTTL := utils.MinTTL(BearerTokenTTL, sessionTTL)

	if req.CreateWebSession {
		sess, err := a.NewWebSession(user.GetName())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// session will expire based on identity TTL and allowed session TTL
		sess.SetExpiryTime(a.clock.Now().UTC().Add(sessionTTL))
		// bearer token will expire based on the expected session renewal
		sess.SetBearerTokenExpiryTime(a.clock.Now().UTC().Add(bearerTokenTTL))
		if err := a.UpsertWebSession(user.GetName(), sess); err != nil {
			return nil, trace.Wrap(err)
		}
		response.Session = sess
	}

	if len(req.PublicKey) != 0 {
		certTTL := utils.MinTTL(utils.ToTTL(a.clock, ident.ExpiresAt), req.CertTTL)
		allowedLogins, err := roles.CheckLogins(certTTL)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cert, err := a.GenerateUserCert(req.PublicKey, user.GetName(), allowedLogins, certTTL, roles.CanForwardAgents())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		response.Cert = cert

		authorities, err := a.GetCertAuthorities(services.HostCA, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, authority := range authorities {
			response.HostSigners = append(response.HostSigners, authority)
		}
	}

	return response, nil
}

func (a *AuthServer) DeleteRole(name string) error {
	// check if this role is used by CA or Users
	users, err := a.Identity.GetUsers()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, u := range users {
		for _, r := range u.GetRoles() {
			if r == name {
				return trace.BadParameter("role %v is used by user %v", name, u.GetName())
			}
		}
	}
	// check if it's used by some external cert authorities, e.g.
	// cert authorities related to external cluster
	cas, err := a.Trust.GetCertAuthorities(services.UserCA, false)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, a := range cas {
		for _, r := range a.GetRoles() {
			if r == name {
				return trace.BadParameter("role %v is used by user cert authority %v", name, a.GetClusterName())
			}
		}
	}
	return a.Access.DeleteRole(name)
}

const (
	// BearerTokenTTL specifies standard bearer token to exist before
	// it has to be renewed by the client
	BearerTokenTTL = 10 * time.Minute
	// TokenLenBytes is len in bytes of the invite token
	TokenLenBytes = 16
)

// oidcClient is internal structure that stores client and it's config
type oidcClient struct {
	client *oidc.Client
	config oidc.ClientConfig
}

// oidcConfigsEqual is a struct that helps us to verify that
// two oidc configs are equal
func oidcConfigsEqual(a, b oidc.ClientConfig) bool {
	if a.RedirectURL != b.RedirectURL {
		return false
	}
	if a.Credentials.ID != b.Credentials.ID {
		return false
	}
	if a.Credentials.Secret != b.Credentials.Secret {
		return false
	}
	if len(a.Scope) != len(b.Scope) {
		return false
	}
	for i := range a.Scope {
		if a.Scope[i] != b.Scope[i] {
			return false
		}
	}
	return true
}

// isHTTPS checks if the scheme for a URL is https or not.
func isHTTPS(u string) error {
	earl, err := url.Parse(u)
	if err != nil {
		return trace.Wrap(err)
	}
	if earl.Scheme != "https" {
		return trace.BadParameter("expected scheme https, got %q", earl.Scheme)
	}

	return nil
}
