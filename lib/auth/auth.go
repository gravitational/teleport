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
	"net/url"
	"os"
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
//compatible public/private key pairs and OpenSSH certificates
type Authority interface {
	// GenerateKeyPair generates new keypair
	GenerateKeyPair(passphrase string) (privKey []byte, pubKey []byte, err error)

	// GetNewKeyPairFromPool returns new keypair from pre-generated in memory pool
	GetNewKeyPairFromPool() (privKey []byte, pubKey []byte, err error)

	// GenerateHostCert generates host certificate, it takes pkey as a signing
	// private key (host certificate authority)
	GenerateHostCert(pkey, key []byte, hostID, authDomain string, roles teleport.Roles, ttl time.Duration) ([]byte, error)

	// GenerateUserCert generates user certificate, it takes pkey as a signing
	// private key (user certificate authority)
	GenerateUserCert(pkey, key []byte, teleportUsername string, allowedLogins []string, ttl time.Duration) ([]byte, error)
}

// Session is a web session context, stores temporary key-value pair and session id
type Session struct {
	// ID is a session ID
	ID string `json:"id"`
	// Username is a user this session belongs to
	Username string `json:"username"`
	// ExpiresAt is an optional expiry time, if set
	// that means this web session and all derived web sessions
	// can not continue after this time, used in OIDC use case
	// when expiry is set by external identity provider, so user
	// has to relogin (or later on we'd need to refresh the token)
	ExpiresAt time.Time `json:"expires_at"`
	// WS is a private keypair used for signing requests
	WS services.WebSession `json:"web"`
}

// AuthServerOption allows setting options as functional arguments to AuthServer
type AuthServerOption func(*AuthServer)

// NewAuthServer creates and configures a new AuthServer instance
func NewAuthServer(cfg *InitConfig, opts ...AuthServerOption) *AuthServer {
	if cfg.Trust == nil {
		cfg.Trust = local.NewCAService(cfg.Backend)
	}
	if cfg.Lock == nil {
		cfg.Lock = local.NewLockService(cfg.Backend)
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
	as := AuthServer{
		bk:              cfg.Backend,
		Authority:       cfg.Authority,
		Trust:           cfg.Trust,
		Lock:            cfg.Lock,
		Presence:        cfg.Presence,
		Provisioner:     cfg.Provisioner,
		Identity:        cfg.Identity,
		Access:          cfg.Access,
		DomainName:      cfg.DomainName,
		AuthServiceName: cfg.AuthServiceName,
		oidcClients:     make(map[string]*oidc.Client),
		StaticTokens:    cfg.StaticTokens,
		U2F:             cfg.U2F,
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
	oidcClients map[string]*oidc.Client
	clock       clockwork.Clock
	bk          backend.Backend
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

	// U2F is the configuration of the U2F 2 factor authentication
	U2F services.U2F

	services.Trust
	services.Lock
	services.Presence
	services.Provisioner
	services.Identity
	services.Access
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

func (a *AuthServer) GetU2FAppID() (string, error) {
	if err := a.CheckU2FEnabled(); err != nil {
		return "", trace.Wrap(err)
	}
	return a.U2F.AppID, nil
}

func (a *AuthServer) CheckU2FEnabled() error {
	if !a.U2F.Enabled {
		return trace.AccessDenied("U2F is disabled")
	}
	return nil
}

// GenerateHostCert generates host certificate, it takes pkey as a signing
// private key (host certificate authority)
func (s *AuthServer) GenerateHostCert(key []byte, hostID, authDomain string, roles teleport.Roles, ttl time.Duration) ([]byte, error) {
	ca, err := s.Trust.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: s.DomainName,
	}, true)
	if err != nil {
		return nil, trace.BadParameter("failed to load host CA for '%s': %v", s.DomainName, err)
	}
	privateKey, err := ca.FirstSigningKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return s.Authority.GenerateHostCert(privateKey, key, hostID, authDomain, roles, ttl)
}

// GenerateUserCert generates user certificate, it takes pkey as a signing
// private key (user certificate authority)
func (s *AuthServer) GenerateUserCert(key []byte, username string, allowedLogins []string, ttl time.Duration) ([]byte, error) {
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
	return s.Authority.GenerateUserCert(privateKey, key, username, allowedLogins, ttl)
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

func (s *AuthServer) SignIn(user string, password []byte) (*Session, error) {
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
func (s *AuthServer) PreAuthenticatedSignIn(user string) (*Session, error) {
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

func (s *AuthServer) U2FSignRequest(user string, password []byte) (*u2f.SignRequest, error) {
	err := s.CheckU2FEnabled()
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

	challenge, err := u2f.NewChallenge(s.U2F.AppID, s.U2F.Facets)
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
	err := s.CheckU2FEnabled()
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
func (s *AuthServer) ExtendWebSession(user string, prevSessionID string) (*Session, error) {
	prevSession, err := s.GetWebSession(user, prevSessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// consider absolute expiry time that may be set for this session
	// by some external identity serivce, so we can not renew this session
	// any more without extra logic for renewal with external OIDC provider
	expiresAt := prevSession.ExpiresAt
	if !expiresAt.IsZero() && expiresAt.Before(s.clock.Now().UTC()) {
		return nil, trace.NotFound("web session has expired")
	}

	sess, err := s.NewWebSession(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sessionTTL := minTTL(toTTL(s.clock, expiresAt), WebSessionTTL)
	sess.ExpiresAt = expiresAt
	if err := s.UpsertWebSession(user, sess, sessionTTL); err != nil {
		return nil, trace.Wrap(err)
	}
	sess.WS.Priv = nil
	return sess, nil
}

// CreateWebSession creates a new web session for user without any
// checks, is used by admins
func (s *AuthServer) CreateWebSession(user string) (*Session, error) {
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

// GenerateServerKeys generates private key and certificate signed
// by the host certificate authority, listing the role of this server
func (s *AuthServer) GenerateServerKeys(hostID string, roles teleport.Roles) (*PackedKeys, error) {
	k, pub, err := s.GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// we always append authority's domain to resulting node name,
	// that's how we make sure that nodes are uniquely identified/found
	// in cases when we have multiple environments/organizations
	fqdn := fmt.Sprintf("%s.%s", hostID, s.DomainName)
	c, err := s.GenerateHostCert(pub, fqdn, s.DomainName, roles, 0)
	if err != nil {
		log.Warningf("[AUTH] Node `%v` cannot join: cert generation error. %v", hostID, err)
		return nil, trace.Wrap(err)
	}

	return &PackedKeys{
		Key:  k,
		Cert: c,
	}, nil
}

// ValidteToken takes a provisioning token value and finds if it's valid. Returns
// a list of roles this token allows its owner to assume, or an error if the token
// cannot be found
func (s *AuthServer) ValidateToken(token string) (roles teleport.Roles, e error) {
	// look at static tokesn first:
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
func (s *AuthServer) RegisterUsingToken(token, hostID string, role teleport.Role) (*PackedKeys, error) {
	log.Infof("[AUTH] Node `%v` is trying to join as %v", hostID, role)
	if hostID == "" {
		return nil, trace.BadParameter("HostID cannot be empty")
	}
	if err := role.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	// make sure the token is valid:
	roles, err := s.ValidateToken(token)
	if err != nil {
		msg := fmt.Sprintf("`%v` cannot join the cluster as %s. Token error: %v", hostID, role, err)
		log.Warnf("[AUTH] %s", msg)
		return nil, trace.AccessDenied(msg)
	}
	// make sure the caller is requested wthe role allowed by the token:
	if !roles.Include(role) {
		msg := fmt.Sprintf("'%v' cannot join the cluster, the token does not allow '%s' role", hostID, role)
		log.Warningf("[AUTH] %s", msg)
		return nil, trace.BadParameter(msg)
	}
	if !s.checkTokenTTL(token) {
		return nil, trace.AccessDenied("'%v' cannot join the cluster. The token has expired", hostID)
	}
	// generate & return the node cert:
	keys, err := s.GenerateServerKeys(hostID, teleport.Roles{role})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	utils.Consolef(os.Stdout, "[AUTH] Node `%v` joined the cluster", hostID)
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

func (s *AuthServer) NewWebSession(userName string) (*Session, error) {
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
	allowedLogins, err := roles.CheckLogins(WebSessionTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cert, err := s.Authority.GenerateUserCert(privateKey, pub, user.GetName(), allowedLogins, WebSessionTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess := &Session{
		ID:       token,
		Username: user.GetName(),
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
	return s.Identity.UpsertWebSession(user, sess.ID, sess.WS, ttl)
}

func (s *AuthServer) GetWebSession(userName string, id string) (*Session, error) {
	ws, err := s.Identity.GetWebSession(userName, id)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := s.GetUser(userName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Session{
		ID:       id,
		Username: user.GetName(),
		WS:       *ws,
	}, nil
}

func (s *AuthServer) GetWebSessionInfo(userName string, id string) (*Session, error) {
	sess, err := s.Identity.GetWebSession(userName, id)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := s.GetUser(userName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess.Priv = nil
	return &Session{
		ID:       id,
		Username: user.GetName(),
		WS:       *sess,
	}, nil
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

	client, ok := s.oidcClients[conn.GetName()]
	if ok {
		return client, nil
	}

	config := oidc.ClientConfig{
		RedirectURL: conn.GetRedirectURL(),
		Credentials: oidc.ClientCredentials{
			ID:     conn.GetClientID(),
			Secret: conn.GetClientSecret(),
		},
		// open id notifies provider that we are using OIDC scopes
		Scope: utils.Deduplicate(append([]string{"openid", "email"}, conn.GetScope()...)),
	}

	client, err := oidc.NewClient(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client.SyncProviderConfig(conn.GetIssuerURL())

	s.oidcClients[conn.GetName()] = client

	return client, nil
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
	Session *Session `json:"session,omitempty"`
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
	}
	log.Debugf("[IDENTITY] %v/%v is a dynamic identity, generating user with roles: %v", connector.GetName(), ident.Email, roles)
	user, err := services.GetUserMarshaler().GenerateUser(&services.UserV2{
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

	tok, err := oidcClient.ExchangeAuthCode(code)
	if err != nil {
		return nil, trace.OAuth2(
			oauth2.ErrorUnsupportedResponseType,
			"unable to verify auth code with issuer", q)
	}

	claims, err := tok.Claims()
	if err != nil {
		return nil, trace.OAuth2(
			oauth2.ErrorUnsupportedResponseType, "unable to construct claims", q)
	}

	ident, err := oidc.IdentityFromClaims(claims)
	if err != nil {
		return nil, trace.OAuth2(
			oauth2.ErrorUnsupportedResponseType, "unable to convert claims to identity", q)
	}

	log.Debugf("[IDENTITY] %v expires at: %v", ident.Email, ident.ExpiresAt)

	response := &OIDCAuthResponse{
		Identity: services.OIDCIdentity{ConnectorID: connector.GetName(), Email: ident.Email},
		Req:      *req,
	}

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

	if req.CreateWebSession {
		sess, err := a.NewWebSession(user.GetName())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		sess.ExpiresAt = ident.ExpiresAt.UTC()
		sessionTTL := minTTL(toTTL(a.clock, ident.ExpiresAt), WebSessionTTL)
		if err := a.UpsertWebSession(user.GetName(), sess, sessionTTL); err != nil {
			return nil, trace.Wrap(err)
		}
		response.Session = sess
	}

	var roles services.RoleSet
	if len(req.PublicKey) != 0 {
		certTTL := minTTL(toTTL(a.clock, ident.ExpiresAt), req.CertTTL)
		roles, err = services.FetchRoles(user.GetRoles(), a.Access)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		allowedLogins, err := roles.CheckLogins(certTTL)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cert, err := a.GenerateUserCert(req.PublicKey, user.GetName(), allowedLogins, certTTL)
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
	// WebSessionTTL specifies standard web session time to live
	WebSessionTTL = 10 * time.Minute
	// TokenLenBytes is len in bytes of the invite token
	TokenLenBytes = 16
)

// minTTL finds min non 0 TTL duration,
// if both durations are 0, fails
func minTTL(a, b time.Duration) time.Duration {
	if a == 0 {
		return b
	}
	if b == 0 {
		return a
	}
	if a < b {
		return a
	}
	return b
}

// toTTL converts expiration time to TTL duration
// relative to current time as provided by clock
func toTTL(c clockwork.Clock, tm time.Time) time.Duration {
	now := c.Now().UTC()
	if tm.IsZero() || tm.Before(now) {
		return 0
	}
	return tm.Sub(now)
}
