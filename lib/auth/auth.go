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
	"context"
	"crypto/x509"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/coreos/go-oidc/oauth2"
	"github.com/coreos/go-oidc/oidc"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	saml2 "github.com/russellhaering/gosaml2"
	"github.com/tstranex/u2f"
)

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
	if cfg.ClusterConfiguration == nil {
		cfg.ClusterConfiguration = local.NewClusterConfigurationService(cfg.Backend)
	}
	if cfg.AuditLog == nil {
		cfg.AuditLog = events.NewDiscardAuditLog()
	}
	closeCtx, cancelFunc := context.WithCancel(context.TODO())
	as := AuthServer{
		bk:                   cfg.Backend,
		Authority:            cfg.Authority,
		Trust:                cfg.Trust,
		Presence:             cfg.Presence,
		Provisioner:          cfg.Provisioner,
		Identity:             cfg.Identity,
		Access:               cfg.Access,
		AuthServiceName:      cfg.AuthServiceName,
		ClusterConfiguration: cfg.ClusterConfiguration,
		IAuditLog:            cfg.AuditLog,
		oidcClients:          make(map[string]*oidcClient),
		samlProviders:        make(map[string]*samlProvider),
		githubClients:        make(map[string]*githubClient),
		cancelFunc:           cancelFunc,
		closeCtx:             closeCtx,
	}
	for _, o := range opts {
		o(&as)
	}
	if as.clock == nil {
		as.clock = clockwork.NewRealClock()
	}

	// start sync loop that keeps cluster config synced with what's in backend.
	// this is used by the context passed in requests to always have a fresh copy
	// of the cluster config without hammering the backend.
	go as.syncCachedClusterConfigLoop()

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
	lock          sync.Mutex
	oidcClients   map[string]*oidcClient
	samlProviders map[string]*samlProvider
	githubClients map[string]*githubClient
	clock         clockwork.Clock
	bk            backend.Backend
	closeCtx      context.Context
	cancelFunc    context.CancelFunc

	// cachedClusterConfig stores a cached copy of the cluster config to avoid
	// hamming the backend with too many requests.
	cachedClusterConfig   services.ClusterConfig
	cachedClusterConfigMu sync.RWMutex

	sshca.Authority

	// AuthServiceName is a human-readable name of this CA. If several Auth services are running
	// (managing multiple teleport clusters) this field is used to tell them apart in UIs
	// It usually defaults to the hostname of the machine the Auth service runs on.
	AuthServiceName string

	services.Trust
	services.Presence
	services.Provisioner
	services.Identity
	services.Access
	services.ClusterConfiguration
	events.IAuditLog
}

func (a *AuthServer) Close() error {
	a.cancelFunc()
	if a.bk != nil {
		return trace.Wrap(a.bk.Close())
	}
	return nil
}

// SetClock sets clock, used in tests
func (a *AuthServer) SetClock(clock clockwork.Clock) {
	a.clock = clock
}

// SetAuditLog sets the server's audit log
func (a *AuthServer) SetAuditLog(auditLog events.IAuditLog) {
	a.IAuditLog = auditLog
}

// GetDomainName returns the domain name that identifies this authority server.
// Also known as "cluster name"
func (a *AuthServer) GetDomainName() (string, error) {
	cn, err := a.GetClusterName()
	if err != nil {
		return "", trace.Wrap(err)
	}

	return cn.GetClusterName(), nil
}

// GenerateHostCert uses the private key of the CA to sign the public key of the host
// (along with meta data like host ID, node name, roles, and ttl) to generate a host certificate.
func (s *AuthServer) GenerateHostCert(hostPublicKey []byte, hostID, nodeName string, principals []string, clusterName string, roles teleport.Roles, ttl time.Duration) ([]byte, error) {
	domainName, err := s.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// get the certificate authority that will be signing the public key of the host
	ca, err := s.Trust.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: domainName,
	}, true)
	if err != nil {
		return nil, trace.BadParameter("failed to load host CA for '%s': %v", domainName, err)
	}

	// get the private key of the certificate authority
	caPrivateKey, err := ca.FirstSigningKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// create and sign!
	return s.Authority.GenerateHostCert(services.HostCertParams{
		PrivateCASigningKey: caPrivateKey,
		PublicHostKey:       hostPublicKey,
		HostID:              hostID,
		NodeName:            nodeName,
		Principals:          principals,
		ClusterName:         clusterName,
		Roles:               roles,
		TTL:                 ttl,
	})
}

// certs is a pair of SSH and TLS certificates
type certs struct {
	// sshis PEM encoded SSH certificate
	ssh []byte
	// tls is PEM encoded TLS certificate
	tls []byte
}

type certRequest struct {
	// user is a user to generate certificate for
	user services.User
	// roles is a list of user roles with rendered variables
	roles services.AccessChecker
	// ttl is Duration of the certificate
	ttl time.Duration
	// publicKey is RSA public key in authorized_keys format
	publicKey []byte
	// compatibility is compatibility mode
	compatibility string
}

// GenerateUserCerts is used to generate user certificate, used internally for tests
func (a *AuthServer) GenerateUserCerts(key []byte, username string, ttl time.Duration, compatibility string) ([]byte, []byte, error) {
	user, err := a.GetUser(username)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	checker, err := services.FetchRoles(user.GetRoles(), a, user.GetTraits())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	certs, err := a.generateUserCert(certRequest{
		user:          user,
		roles:         checker,
		ttl:           ttl,
		compatibility: compatibility,
		publicKey:     key,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return certs.ssh, certs.tls, nil
}

// generateUserCert generates user certificate, it takes pkey as a signing
// private key (user certificate authority)
func (s *AuthServer) generateUserCert(req certRequest) (*certs, error) {
	// reuse the same RSA keys for SSH and TLS keys
	cryptoPubKey, err := sshutils.CryptoPublicKey(req.publicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// create the user certificate
	compatibility, err := utils.CheckCompatibilityFlag(req.compatibility)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// adjust session ttl to the smaller of two values: the session
	// ttl requested in tsh or the session ttl for the role.
	sessionTTL := req.roles.AdjustSessionTTL(req.ttl)

	// check signing TTL and return a list of allowed logins
	allowedLogins, err := req.roles.CheckLoginDuration(sessionTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterName, err := s.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := s.Trust.GetCertAuthority(services.CertAuthID{
		Type:       services.UserCA,
		DomainName: clusterName,
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privateKey, err := ca.FirstSigningKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshCert, err := s.Authority.GenerateUserCert(services.UserCertParams{
		PrivateCASigningKey:   privateKey,
		PublicUserKey:         req.publicKey,
		Username:              req.user.GetName(),
		AllowedLogins:         allowedLogins,
		TTL:                   sessionTTL,
		Roles:                 req.user.GetRoles(),
		Compatibility:         compatibility,
		PermitPortForwarding:  req.roles.CanPortForward(),
		PermitAgentForwarding: req.roles.CanForwardAgents(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	hostCA, err := s.Trust.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: clusterName,
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// generate TLS certificate
	tlsAuthority, err := hostCA.TLSCA()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity := tlsca.Identity{
		Username: req.user.GetName(),
		Groups:   req.roles.RoleNames(),
	}
	certRequest := tlsca.CertificateRequest{
		Clock:     s.clock,
		PublicKey: cryptoPubKey,
		Subject:   identity.Subject(),
		NotAfter:  s.clock.Now().UTC().Add(req.ttl),
	}
	tlsCert, err := tlsAuthority.GenerateCertificate(certRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.EmitAuditEvent(events.UserLoginEvent, events.EventFields{
		events.EventUser:   req.user.GetName(),
		events.LoginMethod: events.LoginMethodLocal,
	})
	return &certs{ssh: sshCert, tls: tlsCert}, nil
}

// WithUserLock executes function authenticateFn that performs user authentication
// if authenticateFn returns non nil error, the login attempt will be logged in as failed.
// The only exception to this rule is ConnectionProblemError, in case if it occurs
// access will be denied, but login attempt will not be recorded
// this is done to avoid potential user lockouts due to backend failures
// In case if user exceeds defaults.MaxLoginAttempts
// the user account will be locked for defaults.AccountLockInterval
func (s *AuthServer) WithUserLock(username string, authenticateFn func() error) error {
	user, err := s.Identity.GetUser(username)
	if err != nil {
		return trace.Wrap(err)
	}
	status := user.GetStatus()
	if status.IsLocked && status.LockExpires.After(s.clock.Now().UTC()) {
		return trace.AccessDenied("user %v is locked until %v", user, utils.HumanTimeFormat(status.LockExpires))
	}
	fnErr := authenticateFn()
	if fnErr == nil {
		// upon successful login, reset the failed attempt counter
		err = s.DeleteUserLoginAttempts(username)
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}

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

// DELETE IN: 2.6.0
// This method is no longer used in 2.5.0 and is replaced by AuthenticateUser methods
func (s *AuthServer) SignIn(user string, password []byte) (services.WebSession, error) {
	err := s.WithUserLock(user, func() error {
		return s.CheckPasswordWOToken(user, password)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.EmitAuditEvent(events.UserLoginEvent, events.EventFields{
		events.EventUser:   user,
		events.LoginMethod: events.LoginMethodLocal,
	})
	return s.PreAuthenticatedSignIn(user)
}

// PreAuthenticatedSignIn is for 2-way authentication methods like U2F where the password is
// already checked before issuing the second factor challenge
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
	cap, err := s.GetAuthPreference()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	universalSecondFactor, err := cap.GetU2F()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.WithUserLock(user, func() error {
		return s.CheckPasswordWOToken(user, password)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	registration, err := s.GetU2FRegistration(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	challenge, err := u2f.NewChallenge(universalSecondFactor.AppID, universalSecondFactor.Facets)
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
	cap, err := s.GetAuthPreference()
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = cap.GetU2F()
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

// ClientCertPool returns trusted x509 cerificate authority pool
func (s *AuthServer) ClientCertPool() (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	authorities, err := s.GetCertAuthorities(services.HostCA, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, auth := range authorities {
		for _, keyPair := range auth.GetTLSKeyPairs() {
			cert, err := tlsca.ParseCertificatePEM(keyPair.Cert)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			pool.AddCert(cert)
		}
	}
	return pool, nil
}

// GenerateServerKeys generates new host private keys and certificates (signed
// by the host certificate authority) for a node.
func (s *AuthServer) GenerateServerKeys(hostID string, nodeName string, roles teleport.Roles) (*PackedKeys, error) {
	clusterName, err := s.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// generate private key
	privateKeyPEM, pubSSHKey, err := s.GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// reuse the same RSA keys for SSH and TLS keys
	cryptoPubKey, err := sshutils.CryptoPublicKey(pubSSHKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// get the certificate authority that will be signing the public key of the host
	ca, err := s.Trust.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: clusterName,
	}, true)
	if err != nil {
		return nil, trace.BadParameter("failed to load host CA for '%s': %v", clusterName, err)
	}

	tlsAuthority, err := ca.TLSCA()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// get the private key of the certificate authority
	caPrivateKey, err := ca.FirstSigningKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// generate hostSSH certificate
	hostSSHCert, err := s.Authority.GenerateHostCert(services.HostCertParams{
		PrivateCASigningKey: caPrivateKey,
		PublicHostKey:       pubSSHKey,
		HostID:              hostID,
		NodeName:            nodeName,
		ClusterName:         clusterName,
		Roles:               roles,
	})

	// generate host TLS certificate
	identity := tlsca.Identity{
		Username: fmt.Sprintf("%v.%v", hostID, clusterName),
		Groups:   roles.StringSlice(),
	}
	certRequest := tlsca.CertificateRequest{
		Clock:     s.clock,
		PublicKey: cryptoPubKey,
		Subject:   identity.Subject(),
		NotAfter:  s.clock.Now().UTC().Add(defaults.CATTL),
	}
	// HTTPS requests need to specify DNS name that should be present in the
	// certificate as one of the DNS Names. It is not known in advance,
	// that is why there is a default one for all certificates
	if roles.Include(teleport.RoleAuth) || roles.Include(teleport.RoleAdmin) {
		certRequest.DNSNames = []string{teleport.APIDomain}
	}
	hostTLSCert, err := tlsAuthority.GenerateCertificate(certRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &PackedKeys{
		Key:        privateKeyPEM,
		Cert:       hostSSHCert,
		TLSCert:    hostTLSCert,
		TLSCACerts: services.TLSCerts(ca),
	}, nil
}

// ValidateToken takes a provisioning token value and finds if it's valid. Returns
// a list of roles this token allows its owner to assume, or an error if the token
// cannot be found
func (s *AuthServer) ValidateToken(token string) (roles teleport.Roles, e error) {
	tkns, err := s.GetStaticTokens()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// look at static tokens first:
	for _, st := range tkns.GetStaticTokens() {
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
	log.Infof("node %q [%v] trying to join with role: %v", nodeName, hostID, role)
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
		log.Warn(msg)
		return nil, trace.AccessDenied(msg)
	}

	// make sure the caller is requested wthe role allowed by the token
	if !roles.Include(role) {
		msg := fmt.Sprintf("%q [%v] can not join the cluster, the token does not allow %q role", nodeName, hostID, role)
		log.Warn(msg)
		return nil, trace.BadParameter(msg)
	}
	if !s.checkTokenTTL(token) {
		return nil, trace.AccessDenied("%q [%v] can not join the cluster, token has expired", nodeName, hostID)
	}

	// generate and return host certificate and keys
	keys, err := s.GenerateServerKeys(hostID, nodeName, teleport.Roles{role})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.Infof("node %q [%v] joined the cluster", nodeName, hostID)
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
	tkns, err := s.GetStaticTokens()
	if err != nil {
		return trace.Wrap(err)
	}

	// is this a static token?
	for _, st := range tkns.GetStaticTokens() {
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
	tkns, err := s.GetStaticTokens()
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if err == nil {
		tokens = append(tokens, tkns.GetStaticTokens()...)
	}
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

func (s *AuthServer) NewWebSession(username string) (services.WebSession, error) {
	user, err := s.GetUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roles, err := services.FetchRoles(user.GetRoles(), s.Access, user.GetTraits())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	priv, pub, err := s.GetNewKeyPairFromPool()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sessionTTL := roles.AdjustSessionTTL(defaults.CertDuration)
	certs, err := s.generateUserCert(certRequest{
		user:      user,
		roles:     roles,
		ttl:       sessionTTL,
		publicKey: pub,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	token, err := utils.CryptoRandomHex(TokenLenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	bearerToken, err := utils.CryptoRandomHex(TokenLenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	bearerTokenTTL := utils.MinTTL(sessionTTL, BearerTokenTTL)
	return services.NewWebSession(token, services.WebSessionSpecV2{
		User:               user.GetName(),
		Priv:               priv,
		Pub:                certs.ssh,
		TLSCert:            certs.tls,
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

// syncCachedClusterConfigLoop keeps updating the cached cluster config.
func (a *AuthServer) syncCachedClusterConfigLoop() {
	// update the cache at the same rate nodes heartbeat
	ticker := time.NewTicker(defaults.ServerHeartbeatTTL)
	defer ticker.Stop()

	for {
		select {
		case <-a.closeCtx.Done():
			return
		case <-ticker.C:
			err := a.syncCachedClusterConfig()
			if err != nil {
				log.Warnf("Failed to sync cluster config: %v.", err)
				continue
			}
		}
	}
}

// getCachedClusterConfig returns a copy of cached services.ClusterConfig. If
// nothing is cached yet, a safe default is returned.
func (a *AuthServer) getCachedClusterConfig() services.ClusterConfig {
	a.cachedClusterConfigMu.RLock()
	defer a.cachedClusterConfigMu.RUnlock()

	// if cache is empty, return a safe default
	clusterConfig := a.cachedClusterConfig
	if clusterConfig == nil {
		return services.DefaultClusterConfig()
	}

	return a.cachedClusterConfig.Copy()
}

// syncCachedClusterConfig gets cluster config from the backend and updated
// the cached value.
func (a *AuthServer) syncCachedClusterConfig() error {
	clusterConfig, err := a.GetClusterConfig()
	if err != nil {
		return trace.Wrap(err)
	}

	a.cachedClusterConfigMu.Lock()
	defer a.cachedClusterConfigMu.Unlock()

	a.cachedClusterConfig = clusterConfig
	return nil
}

const (
	// BearerTokenTTL specifies standard bearer token to exist before
	// it has to be renewed by the client
	BearerTokenTTL = 10 * time.Minute
	// TokenLenBytes is len in bytes of the invite token
	TokenLenBytes = 16
)

// oidcClient is internal structure that stores OIDC client and its config
type oidcClient struct {
	client *oidc.Client
	config oidc.ClientConfig
}

// samlProvider is internal structure that stores SAML client and its config
type samlProvider struct {
	provider  *saml2.SAMLServiceProvider
	connector services.SAMLConnector
}

// githubClient is internal structure that stores Github OAuth 2client and its config
type githubClient struct {
	client *oauth2.Client
	config oauth2.Config
}

// oidcConfigsEqual returns true if the provided OIDC configs are equal
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

// oauth2ConfigsEqual returns true if the provided OAuth2 configs are equal
func oauth2ConfigsEqual(a, b oauth2.Config) bool {
	if a.Credentials.ID != b.Credentials.ID {
		return false
	}
	if a.Credentials.Secret != b.Credentials.Secret {
		return false
	}
	if a.RedirectURL != b.RedirectURL {
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
	if a.AuthURL != b.AuthURL {
		return false
	}
	if a.TokenURL != b.TokenURL {
		return false
	}
	if a.AuthMethod != b.AuthMethod {
		return false
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
