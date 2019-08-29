/*
Copyright 2015-2019 Gravitational, Inc.

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
	"crypto"
	"fmt"
	"golang.org/x/crypto/ssh"
	"math/rand"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/limiter"
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
	"github.com/prometheus/client_golang/prometheus"
	saml2 "github.com/russellhaering/gosaml2"
	"github.com/tstranex/u2f"
)

// AuthServerOption allows setting options as functional arguments to AuthServer
type AuthServerOption func(*AuthServer)

// NewAuthServer creates and configures a new AuthServer instance
func NewAuthServer(cfg *InitConfig, opts ...AuthServerOption) (*AuthServer, error) {
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
	if cfg.Events == nil {
		cfg.Events = local.NewEventsService(cfg.Backend)
	}
	if cfg.AuditLog == nil {
		cfg.AuditLog = events.NewDiscardAuditLog()
	}

	limiter, err := limiter.NewConnectionsLimiter(limiter.LimiterConfig{
		MaxConnections: defaults.LimiterMaxConcurrentSignatures,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	closeCtx, cancelFunc := context.WithCancel(context.TODO())
	as := AuthServer{
		bk:              cfg.Backend,
		limiter:         limiter,
		Authority:       cfg.Authority,
		AuthServiceName: cfg.AuthServiceName,
		oidcClients:     make(map[string]*oidcClient),
		samlProviders:   make(map[string]*samlProvider),
		githubClients:   make(map[string]*githubClient),
		cancelFunc:      cancelFunc,
		closeCtx:        closeCtx,
		AuthServices: AuthServices{
			Trust:                cfg.Trust,
			Presence:             cfg.Presence,
			Provisioner:          cfg.Provisioner,
			Identity:             cfg.Identity,
			Access:               cfg.Access,
			ClusterConfiguration: cfg.ClusterConfiguration,
			IAuditLog:            cfg.AuditLog,
			Events:               cfg.Events,
		},
	}
	for _, o := range opts {
		o(&as)
	}
	if as.clock == nil {
		as.clock = clockwork.NewRealClock()
	}

	return &as, nil
}

type AuthServices struct {
	services.Trust
	services.Presence
	services.Provisioner
	services.Identity
	services.Access
	services.ClusterConfiguration
	services.Events
	events.IAuditLog
}

var (
	generateRequestsCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: teleport.MetricGenerateRequests,
			Help: "Number of requests to generate new server keys",
		},
	)
	generateThrottledRequestsCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: teleport.MetricGenerateRequestsThrottled,
			Help: "Number of throttled requests to generate new server keys",
		},
	)
	generateRequestsCurrent = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: teleport.MetricGenerateRequestsCurrent,
			Help: "Number of current generate requests",
		},
	)
	generateRequestsLatencies = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: teleport.MetricGenerateRequestsHistogram,
			Help: "Latency for generate requests",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
	)
)

// AuthServer keeps the cluster together. It acts as a certificate authority (CA) for
// a cluster and:
//   - generates the keypair for the node it's running on
//	 - invites other SSH nodes to a cluster, by issuing invite tokens
//	 - adds other SSH nodes to a cluster, by checking their token and signing their keys
//   - same for users and their sessions
//   - checks public keys to see if they're signed by it (can be trusted or not)
type AuthServer struct {
	lock          sync.RWMutex
	oidcClients   map[string]*oidcClient
	samlProviders map[string]*samlProvider
	githubClients map[string]*githubClient
	clock         clockwork.Clock
	bk            backend.Backend

	closeCtx   context.Context
	cancelFunc context.CancelFunc

	sshca.Authority

	// AuthServiceName is a human-readable name of this CA. If several Auth services are running
	// (managing multiple teleport clusters) this field is used to tell them apart in UIs
	// It usually defaults to the hostname of the machine the Auth service runs on.
	AuthServiceName string

	// AuthServices encapsulate services - provisioner, trust, etc
	// used by the auth server in a separate structure
	AuthServices

	// privateKey is used in tests to use pre-generated private keys
	privateKey []byte

	// cipherSuites is a list of ciphersuites that the auth server supports.
	cipherSuites []uint16

	// cache is a fast cache that allows auth server
	// to use cache for most frequent operations,
	// if not set, cache uses itself
	cache AuthCache

	limiter *limiter.ConnectionsLimiter
}

// SetCache sets cache used by auth server
func (a *AuthServer) SetCache(clt AuthCache) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.cache = clt
}

// GetCache returns cache used by auth server
func (a *AuthServer) GetCache() AuthCache {
	a.lock.RLock()
	defer a.lock.RUnlock()
	if a.cache == nil {
		return &a.AuthServices
	}
	return a.cache
}

// runPeriodicOperations runs some periodic bookkeeping operations
// performed by auth server
func (a *AuthServer) runPeriodicOperations() {
	// run periodic functions with a semi-random period
	// to avoid contention on the database in case if there are multiple
	// auth servers running - so they don't compete trying
	// to update the same resources.
	r := rand.New(rand.NewSource(a.GetClock().Now().UnixNano()))
	period := defaults.HighResPollingPeriod + time.Duration(r.Intn(int(defaults.HighResPollingPeriod/time.Second)))*time.Second
	log.Debugf("Ticking with period: %v.", period)
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	for {
		select {
		case <-a.closeCtx.Done():
			return
		case <-ticker.C:
			err := a.autoRotateCertAuthorities()
			if err != nil {
				if trace.IsCompareFailed(err) {
					log.Debugf("Cert authority has been updated concurrently: %v.", err)
				} else {
					log.Errorf("Failed to perform cert rotation check: %v.", err)
				}
			}
		}
	}
}

func (a *AuthServer) Close() error {
	a.cancelFunc()
	if a.bk != nil {
		return trace.Wrap(a.bk.Close())
	}
	return nil
}

func (a *AuthServer) GetClock() clockwork.Clock {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.clock
}

// SetClock sets clock, used in tests
func (a *AuthServer) SetClock(clock clockwork.Clock) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.clock = clock
}

// SetAuditLog sets the server's audit log
func (a *AuthServer) SetAuditLog(auditLog events.IAuditLog) {
	a.IAuditLog = auditLog
}

// GetClusterConfig gets ClusterConfig from the backend.
func (a *AuthServer) GetClusterConfig(opts ...services.MarshalOption) (services.ClusterConfig, error) {
	return a.GetCache().GetClusterConfig(opts...)
}

// GetClusterName returns the domain name that identifies this authority server.
// Also known as "cluster name"
func (a *AuthServer) GetClusterName(opts ...services.MarshalOption) (services.ClusterName, error) {
	return a.GetCache().GetClusterName(opts...)
}

// GetDomainName returns the domain name that identifies this authority server.
// Also known as "cluster name"
func (a *AuthServer) GetDomainName() (string, error) {
	clusterName, err := a.GetClusterName()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return clusterName.GetClusterName(), nil
}

// LocalCAResponse contains PEM-encoded local CAs.
type LocalCAResponse struct {
	// TLSCA is the PEM-encoded TLS certificate authority.
	TLSCA []byte `json:"tls_ca"`
}

// GetClusterCACert returns the CAs for the local cluster without signing keys.
func (a *AuthServer) GetClusterCACert() (*LocalCAResponse, error) {
	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Extract the TLS CA for this cluster.
	hostCA, err := a.GetCache().GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: clusterName.GetClusterName(),
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCA, err := hostCA.TLSCA()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Marshal to PEM bytes to send the CA over the wire.
	pemBytes, err := tlsca.MarshalCertificatePEM(tlsCA.Cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &LocalCAResponse{
		TLSCA: pemBytes,
	}, nil
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
	// ssh is PEM encoded SSH certificate
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
	// overrideRoleTTL is used for requests when the requested TTL should not be
	// adjusted based off the role of the user. This is used by tctl to allow
	// creating long lived user certs.
	overrideRoleTTL bool
	// usage is a list of acceptable usages to be encoded in X509 certificate,
	// is used to limit ways the certificate can be used, for example
	// the cert can be only used against kubernetes endpoint, and not auth endpoint,
	// no usage means unrestricted (to keep backwards compatibility)
	usage []string
	// routeToCluster is an optional cluster name to route the certificate requests to,
	// this cluster name will be used to route the requests to in case of kubernetes
	routeToCluster string
}

// GenerateUserTestCerts is used to generate user certificate, used internally for tests
func (a *AuthServer) GenerateUserTestCerts(key []byte, username string, ttl time.Duration, compatibility, routeToCluster string) ([]byte, []byte, error) {
	user, err := a.Identity.GetUser(username, false)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	checker, err := services.FetchRoles(user.GetRoles(), a.Access, user.GetTraits())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	certs, err := a.generateUserCert(certRequest{
		user:           user,
		roles:          checker,
		ttl:            ttl,
		compatibility:  compatibility,
		publicKey:      key,
		routeToCluster: routeToCluster,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return certs.ssh, certs.tls, nil
}

// generateUserCert generates user certificates
func (s *AuthServer) generateUserCert(req certRequest) (*certs, error) {
	// reuse the same RSA keys for SSH and TLS keys
	cryptoPubKey, err := sshutils.CryptoPublicKey(req.publicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// extract the passed in certificate format. if nothing was passed in, fetch
	// the certificate format from the role.
	certificateFormat, err := utils.CheckCertificateFormatFlag(req.compatibility)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if certificateFormat == teleport.CertificateFormatUnspecified {
		certificateFormat = req.roles.CertificateFormat()
	}

	var sessionTTL time.Duration
	var allowedLogins []string

	// If the role TTL is ignored, do not restrict session TTL and allowed logins.
	// The only caller setting this parameter should be "tctl auth sign".
	// Otherwise set the session TTL to the smallest of all roles and
	// then only grant access to allowed logins based on that.
	if req.overrideRoleTTL {
		// Take whatever was passed in. Pass in 0 to CheckLoginDuration so all
		// logins are returned for the role set.
		sessionTTL = req.ttl
		allowedLogins, err = req.roles.CheckLoginDuration(0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		// Adjust session TTL to the smaller of two values: the session TTL
		// requested in tsh or the session TTL for the role.
		sessionTTL = req.roles.AdjustSessionTTL(req.ttl)

		// Return a list of logins that meet the session TTL limit. This means if
		// the requested session TTL is larger than the max session TTL for a login,
		// that login will not be included in the list of allowed logins.
		allowedLogins, err = req.roles.CheckLoginDuration(sessionTTL)
		if err != nil {
			return nil, trace.Wrap(err)
		}
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
		CertificateFormat:     certificateFormat,
		PermitPortForwarding:  req.roles.CanPortForward(),
		PermitAgentForwarding: req.roles.CanForwardAgents(),
		RouteToCluster:        req.routeToCluster,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	userCA, err := s.Trust.GetCertAuthority(services.CertAuthID{
		Type:       services.UserCA,
		DomainName: clusterName,
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// generate TLS certificate
	tlsAuthority, err := userCA.TLSCA()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity := tlsca.Identity{
		Username:       req.user.GetName(),
		Groups:         req.roles.RoleNames(),
		Principals:     allowedLogins,
		Usage:          req.usage,
		RouteToCluster: req.routeToCluster,
	}
	certRequest := tlsca.CertificateRequest{
		Clock:     s.clock,
		PublicKey: cryptoPubKey,
		Subject:   identity.Subject(),
		NotAfter:  s.clock.Now().UTC().Add(sessionTTL),
	}
	tlsCert, err := tlsAuthority.GenerateCertificate(certRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}
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
	user, err := s.Identity.GetUser(username, false)
	if err != nil {
		return trace.Wrap(err)
	}
	status := user.GetStatus()
	if status.IsLocked && status.LockExpires.After(s.clock.Now().UTC()) {
		return trace.AccessDenied("%v exceeds %v failed login attempts, locked until %v",
			user.GetName(), defaults.MaxLoginAttempts, utils.HumanTimeFormat(status.LockExpires))
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

// GenerateTokenRequest is a request to generate auth token
type GenerateTokenRequest struct {
	// Token if provided sets the token value, otherwise will be auto generated
	Token string `json:"token"`
	// Roles is a list of roles this token authenticates as
	Roles teleport.Roles `json:"roles"`
	// TTL is a time to live for token
	TTL time.Duration `json:"ttl"`
}

// CheckAndSetDefaults checks and sets default values of request
func (req *GenerateTokenRequest) CheckAndSetDefaults() error {
	for _, role := range req.Roles {
		if err := role.Check(); err != nil {
			return trace.Wrap(err)
		}
	}
	if req.TTL == 0 {
		req.TTL = defaults.ProvisioningTokenTTL
	}
	if req.Token == "" {
		token, err := utils.CryptoRandomHex(TokenLenBytes)
		if err != nil {
			return trace.Wrap(err)
		}
		req.Token = token
	}
	return nil
}

// GenerateToken generates multi-purpose authentication token
func (s *AuthServer) GenerateToken(req GenerateTokenRequest) (string, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return "", trace.Wrap(err)
	}
	token, err := services.NewProvisionToken(req.Token, req.Roles, s.clock.Now().UTC().Add(req.TTL))
	if err != nil {
		return "", trace.Wrap(err)
	}
	if err := s.Provisioner.UpsertToken(token); err != nil {
		return "", trace.Wrap(err)
	}
	return req.Token, nil
}

// ExtractHostID returns host id based on the hostname
func ExtractHostID(hostName string, clusterName string) (string, error) {
	suffix := "." + clusterName
	if !strings.HasSuffix(hostName, suffix) {
		return "", trace.BadParameter("expected suffix %q in %q", suffix, hostName)
	}
	return strings.TrimSuffix(hostName, suffix), nil
}

// HostFQDN consits of host UUID and cluster name joined via .
func HostFQDN(hostUUID, clusterName string) string {
	return fmt.Sprintf("%v.%v", hostUUID, clusterName)
}

// GenerateServerKeysRequest is a request to generate server keys
type GenerateServerKeysRequest struct {
	// HostID is a unique ID of the host
	HostID string `json:"host_id"`
	// NodeName is a user friendly host name
	NodeName string `json:"node_name"`
	// Roles is a list of roles assigned to node
	Roles teleport.Roles `json:"roles"`
	// AdditionalPrincipals is a list of additional principals
	// to include in OpenSSH and X509 certificates
	AdditionalPrincipals []string `json:"additional_principals"`
	// DNSNames is a list of DNS names
	// to include in the x509 client certificate
	DNSNames []string `json:"dns_names"`
	// PublicTLSKey is a PEM encoded public key
	// used for TLS setup
	PublicTLSKey []byte `json:"public_tls_key"`
	// PublicSSHKey is a SSH encoded public key,
	// if present will be signed as a return value
	// otherwise, new public/private key pair will be generated
	PublicSSHKey []byte `json:"public_ssh_key"`
	// RemoteAddr is the IP address of the remote host requesting a host
	// certificate. RemoteAddr is used to replace 0.0.0.0 in the list of
	// additional principals.
	RemoteAddr string `json:"remote_addr"`
	// Rotation allows clients to send the certificate authority rotation state
	// expected by client of the certificate authority backends, so auth servers
	// can avoid situation when clients request certs assuming one
	// state, and auth servers issue another
	Rotation *services.Rotation `json:"rotation,omitempty"`
	// NoCache is argument that only local callers can supply to bypass cache
	NoCache bool `json:"-"`
}

// CheckAndSetDefaults checks and sets default values
func (req *GenerateServerKeysRequest) CheckAndSetDefaults() error {
	if req.HostID == "" {
		return trace.BadParameter("missing parameter HostID")
	}
	if len(req.Roles) != 1 {
		return trace.BadParameter("expected only one system role, got %v", len(req.Roles))
	}
	return nil
}

// GenerateServerKeys generates new host private keys and certificates (signed
// by the host certificate authority) for a node.
func (s *AuthServer) GenerateServerKeys(req GenerateServerKeysRequest) (*PackedKeys, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.limiter.AcquireConnection(req.Roles.String()); err != nil {
		generateThrottledRequestsCount.Inc()
		log.Debugf("Node %q [%v] is rate limited: %v.", req.NodeName, req.HostID, req.Roles)
		return nil, trace.Wrap(err)
	}
	defer s.limiter.ReleaseConnection(req.Roles.String())

	// only observe latencies for non-throttled requests
	start := s.clock.Now()
	defer generateRequestsLatencies.Observe(time.Since(start).Seconds())

	generateRequestsCount.Inc()
	generateRequestsCurrent.Inc()
	defer generateRequestsCurrent.Dec()

	clusterName, err := s.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If the request contains 0.0.0.0, this implies an advertise IP was not
	// specified on the node. Try and guess what the address by replacing 0.0.0.0
	// with the RemoteAddr as known to the Auth Server.
	if utils.SliceContainsStr(req.AdditionalPrincipals, defaults.AnyAddress) {
		remoteHost, err := utils.Host(req.RemoteAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		req.AdditionalPrincipals = utils.ReplaceInSlice(
			req.AdditionalPrincipals,
			defaults.AnyAddress,
			remoteHost)
	}

	var cryptoPubKey crypto.PublicKey
	var privateKeyPEM, pubSSHKey []byte
	if req.PublicSSHKey != nil || req.PublicTLSKey != nil {
		_, _, _, _, err := ssh.ParseAuthorizedKey(req.PublicSSHKey)
		if err != nil {
			return nil, trace.BadParameter("failed to parse SSH public key")
		}
		pubSSHKey = req.PublicSSHKey
		cryptoPubKey, err = tlsca.ParsePublicKeyPEM(req.PublicTLSKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		// generate private key
		privateKeyPEM, pubSSHKey, err = s.GenerateKeyPair("")
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// reuse the same RSA keys for SSH and TLS keys
		cryptoPubKey, err = sshutils.CryptoPublicKey(pubSSHKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

	}

	// get the certificate authority that will be signing the public key of the host,
	client := s.GetCache()
	if req.NoCache {
		client = &s.AuthServices
	}
	ca, err := client.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: clusterName.GetClusterName(),
	}, true)
	if err != nil {
		return nil, trace.BadParameter("failed to load host CA for %q: %v", clusterName.GetClusterName(), err)
	}

	// could be a couple of scenarios, either client data is out of sync,
	// or auth server is out of sync, either way, for now check that
	// cache is out of sync, this will result in higher read rate
	// to the backend, which is a fine tradeoff
	if !req.NoCache && req.Rotation != nil && !req.Rotation.Matches(ca.GetRotation()) {
		log.Debugf("Client sent rotation state %v, cache state is %v, using state from the DB.", req.Rotation, ca.GetRotation())
		ca, err = s.GetCertAuthority(services.CertAuthID{
			Type:       services.HostCA,
			DomainName: clusterName.GetClusterName(),
		}, true)
		if err != nil {
			return nil, trace.BadParameter("failed to load host CA for %q: %v", clusterName.GetClusterName(), err)
		}
		if !req.Rotation.Matches(ca.GetRotation()) {
			return nil, trace.BadParameter("the client expected state is out of sync, server rotation state: %v, client rotation state: %v, re-register the client from scratch to fix the issue.", ca.GetRotation(), req.Rotation)
		}
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
		HostID:              req.HostID,
		NodeName:            req.NodeName,
		ClusterName:         clusterName.GetClusterName(),
		Roles:               req.Roles,
		Principals:          req.AdditionalPrincipals,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// generate host TLS certificate
	identity := tlsca.Identity{
		Username: HostFQDN(req.HostID, clusterName.GetClusterName()),
		Groups:   req.Roles.StringSlice(),
	}
	certRequest := tlsca.CertificateRequest{
		Clock:     s.clock,
		PublicKey: cryptoPubKey,
		Subject:   identity.Subject(),
		NotAfter:  s.clock.Now().UTC().Add(defaults.CATTL),
		DNSNames:  append([]string{}, req.AdditionalPrincipals...),
	}
	// HTTPS requests need to specify DNS name that should be present in the
	// certificate as one of the DNS Names. It is not known in advance,
	// that is why there is a default one for all certificates
	if req.Roles.Include(teleport.RoleAuth) || req.Roles.Include(teleport.RoleAdmin) {
		certRequest.DNSNames = append(certRequest.DNSNames, "*."+teleport.APIDomain, teleport.APIDomain)
	}
	// Unlike additional pricinpals, DNS Names is x509 specific
	// and is limited to auth servers and proxies
	if req.Roles.Include(teleport.RoleAuth) || req.Roles.Include(teleport.RoleAdmin) || req.Roles.Include(teleport.RoleProxy) {
		certRequest.DNSNames = append(certRequest.DNSNames, req.DNSNames...)
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
		SSHCACerts: ca.GetCheckingKeys(),
	}, nil
}

// ValidateToken takes a provisioning token value and finds if it's valid. Returns
// a list of roles this token allows its owner to assume, or an error if the token
// cannot be found.
func (s *AuthServer) ValidateToken(token string) (roles teleport.Roles, e error) {
	tkns, err := s.GetCache().GetStaticTokens()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// First check if the token is a static token. If it is, return right away.
	// Static tokens have no expiration.
	for _, st := range tkns.GetStaticTokens() {
		if st.GetName() == token {
			return st.GetRoles(), nil
		}
	}

	// If it's not a static token, check if it's a ephemeral token in the backend.
	// If a ephemeral token is found, make sure it's still valid.
	tok, err := s.GetCache().GetToken(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !s.checkTokenTTL(tok) {
		return nil, trace.AccessDenied("token expired")
	}

	return tok.GetRoles(), nil
}

// checkTokenTTL checks if the token is still valid. If it is not, the token
// is removed from the backend and returns false. Otherwise returns true.
func (s *AuthServer) checkTokenTTL(tok services.ProvisionToken) bool {
	now := s.clock.Now().UTC()
	if tok.Expiry().Before(now) {
		err := s.DeleteToken(tok.GetName())
		if err != nil {
			if !trace.IsNotFound(err) {
				log.Warnf("Unable to delete token from backend: %v.", err)
			}
		}
		return false
	}
	return true
}

// RegisterUsingTokenRequest is a request to register with
// auth server using authentication token
type RegisterUsingTokenRequest struct {
	// HostID is a unique host ID, usually a UUID
	HostID string `json:"hostID"`
	// NodeName is a node name
	NodeName string `json:"node_name"`
	// Role is a system role, e.g. Proxy
	Role teleport.Role `json:"role"`
	// Token is an authentication token
	Token string `json:"token"`
	// AdditionalPrincipals is a list of additional principals
	AdditionalPrincipals []string `json:"additional_principals"`
	// DNSNames is a list of DNS names to include in the x509 client certificate
	DNSNames []string `json:"dns_names"`
	// PublicTLSKey is a PEM encoded public key
	// used for TLS setup
	PublicTLSKey []byte `json:"public_tls_key"`
	// PublicSSHKey is a SSH encoded public key,
	// if present will be signed as a return value
	// otherwise, new public/private key pair will be generated
	PublicSSHKey []byte `json:"public_ssh_key"`
	// RemoteAddr is the remote address of the host requesting a host certificate.
	// It is used to replace 0.0.0.0 in the list of additional principals.
	RemoteAddr string `json:"remote_addr"`
}

// CheckAndSetDefaults checks for errors and sets defaults
func (r *RegisterUsingTokenRequest) CheckAndSetDefaults() error {
	if r.HostID == "" {
		return trace.BadParameter("missing parameter HostID")
	}
	if r.Token == "" {
		return trace.BadParameter("missing parameter Token")
	}
	if err := r.Role.Check(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// RegisterUsingToken adds a new node to the Teleport cluster using previously issued token.
// A node must also request a specific role (and the role must match one of the roles
// the token was generated for).
//
// If a token was generated with a TTL, it gets enforced (can't register new nodes after TTL expires)
// If a token was generated with a TTL=0, it means it's a single-use token and it gets destroyed
// after a successful registration.
func (s *AuthServer) RegisterUsingToken(req RegisterUsingTokenRequest) (*PackedKeys, error) {
	log.Infof("Node %q [%v] is trying to join with role: %v.", req.NodeName, req.HostID, req.Role)

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// make sure the token is valid
	roles, err := s.ValidateToken(req.Token)
	if err != nil {
		log.Warningf("%q [%v] can not join the cluster with role %s, token error: %v", req.NodeName, req.HostID, req.Role, err)
		return nil, trace.AccessDenied(fmt.Sprintf("%q [%v] can not join the cluster with role %s, the token is not valid", req.NodeName, req.HostID, req.Role))
	}

	// make sure the caller is requested the role allowed by the token
	if !roles.Include(req.Role) {
		msg := fmt.Sprintf("node %q [%v] can not join the cluster, the token does not allow %q role", req.NodeName, req.HostID, req.Role)
		log.Warn(msg)
		return nil, trace.BadParameter(msg)
	}

	// generate and return host certificate and keys
	keys, err := s.GenerateServerKeys(GenerateServerKeysRequest{
		HostID:               req.HostID,
		NodeName:             req.NodeName,
		Roles:                teleport.Roles{req.Role},
		AdditionalPrincipals: req.AdditionalPrincipals,
		PublicTLSKey:         req.PublicTLSKey,
		PublicSSHKey:         req.PublicSSHKey,
		RemoteAddr:           req.RemoteAddr,
		DNSNames:             req.DNSNames,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.Infof("Node %q [%v] has joined the cluster.", req.NodeName, req.HostID)
	return keys, nil
}

func (s *AuthServer) RegisterNewAuthServer(token string) error {
	tok, err := s.Provisioner.GetToken(token)
	if err != nil {
		return trace.Wrap(err)
	}
	if !tok.GetRoles().Include(teleport.RoleAuth) {
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
		if st.GetName() == token {
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
func (s *AuthServer) GetTokens(opts ...services.MarshalOption) (tokens []services.ProvisionToken, err error) {
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
		tok, err := services.NewProvisionToken(t.Token, roles, t.Expires)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tokens = append(tokens, tok)
	}
	return tokens, nil
}

func (s *AuthServer) NewWebSession(username string) (services.WebSession, error) {
	user, err := s.GetUser(username, false)
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
	nodes, err := s.Presence.GetNodes(namespace, services.SkipValidation())
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

// NewWatcher returns a new event watcher. In case of an auth server
// this watcher will return events as seen by the auth server's
// in memory cache, not the backend.
func (a *AuthServer) NewWatcher(ctx context.Context, watch services.Watch) (services.Watcher, error) {
	return a.GetCache().NewWatcher(ctx, watch)
}

func (a *AuthServer) DeleteRole(name string) error {
	// check if this role is used by CA or Users
	users, err := a.Identity.GetUsers(false)
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

// NewKeepAliver returns a new instance of keep aliver
func (a *AuthServer) NewKeepAliver(ctx context.Context) (services.KeepAliver, error) {
	cancelCtx, cancel := context.WithCancel(ctx)
	k := &authKeepAliver{
		a:           a,
		ctx:         cancelCtx,
		cancel:      cancel,
		keepAlivesC: make(chan services.KeepAlive),
	}
	go k.forwardKeepAlives()
	return k, nil
}

// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
// controls if signing keys are loaded
func (a *AuthServer) GetCertAuthority(id services.CertAuthID, loadSigningKeys bool, opts ...services.MarshalOption) (services.CertAuthority, error) {
	return a.GetCache().GetCertAuthority(id, loadSigningKeys, opts...)
}

// GetCertAuthorities returns a list of authorities of a given type
// loadSigningKeys controls whether signing keys should be loaded or not
func (a *AuthServer) GetCertAuthorities(caType services.CertAuthType, loadSigningKeys bool, opts ...services.MarshalOption) ([]services.CertAuthority, error) {
	return a.GetCache().GetCertAuthorities(caType, loadSigningKeys, opts...)
}

// GetStaticTokens gets the list of static tokens used to provision nodes.
func (a *AuthServer) GetStaticTokens() (services.StaticTokens, error) {
	return a.GetCache().GetStaticTokens()
}

// GetToken finds and returns token by ID
func (a *AuthServer) GetToken(token string) (services.ProvisionToken, error) {
	return a.GetCache().GetToken(token)
}

// GetRoles is a part of auth.AccessPoint implementation
func (a *AuthServer) GetRoles() ([]services.Role, error) {
	return a.GetCache().GetRoles()
}

// GetRole is a part of auth.AccessPoint implementation
func (a *AuthServer) GetRole(name string) (services.Role, error) {
	return a.GetCache().GetRole(name)
}

// GetNamespace returns namespace
func (a *AuthServer) GetNamespace(name string) (*services.Namespace, error) {
	return a.GetCache().GetNamespace(name)
}

// GetNamespaces is a part of auth.AccessPoint implementation
func (a *AuthServer) GetNamespaces() ([]services.Namespace, error) {
	return a.GetCache().GetNamespaces()
}

// GetNodes is a part of auth.AccessPoint implementation
func (a *AuthServer) GetNodes(namespace string, opts ...services.MarshalOption) ([]services.Server, error) {
	return a.GetCache().GetNodes(namespace, opts...)
}

// GetReverseTunnels is a part of auth.AccessPoint implementation
func (a *AuthServer) GetReverseTunnels(opts ...services.MarshalOption) ([]services.ReverseTunnel, error) {
	return a.GetCache().GetReverseTunnels(opts...)
}

// GetProxies is a part of auth.AccessPoint implementation
func (a *AuthServer) GetProxies() ([]services.Server, error) {
	return a.GetCache().GetProxies()
}

// GetUser is a part of auth.AccessPoint implementation.
func (a *AuthServer) GetUser(name string, withSecrets bool) (user services.User, err error) {
	return a.GetCache().GetUser(name, withSecrets)
}

// GetUsers is a part of auth.AccessPoint implementation
func (a *AuthServer) GetUsers(withSecrets bool) (users []services.User, err error) {
	return a.GetCache().GetUsers(withSecrets)
}

// GetTunnelConnections is a part of auth.AccessPoint implementation
// GetTunnelConnections are not using recent cache as they are designed
// to be called periodically and always return fresh data
func (a *AuthServer) GetTunnelConnections(clusterName string, opts ...services.MarshalOption) ([]services.TunnelConnection, error) {
	return a.GetCache().GetTunnelConnections(clusterName, opts...)
}

// GetAllTunnelConnections is a part of auth.AccessPoint implementation
// GetAllTunnelConnections are not using recent cache, as they are designed
// to be called periodically and always return fresh data
func (a *AuthServer) GetAllTunnelConnections(opts ...services.MarshalOption) (conns []services.TunnelConnection, err error) {
	return a.GetCache().GetAllTunnelConnections(opts...)
}

// authKeepAliver is a keep aliver using auth server directly
type authKeepAliver struct {
	sync.RWMutex
	a           *AuthServer
	ctx         context.Context
	cancel      context.CancelFunc
	keepAlivesC chan services.KeepAlive
	err         error
}

// KeepAlives returns a channel accepting keep alive requests
func (k *authKeepAliver) KeepAlives() chan<- services.KeepAlive {
	return k.keepAlivesC
}

func (k *authKeepAliver) forwardKeepAlives() {
	for {
		select {
		case <-k.a.closeCtx.Done():
			k.Close()
			return
		case <-k.ctx.Done():
			return
		case keepAlive := <-k.keepAlivesC:
			err := k.a.KeepAliveNode(k.ctx, keepAlive)
			if err != nil {
				k.closeWithError(err)
				return
			}
		}
	}
}

func (k *authKeepAliver) closeWithError(err error) {
	k.Close()
	k.Lock()
	defer k.Unlock()
	k.err = err
}

// Error returns the error if keep aliver
// has been closed
func (k *authKeepAliver) Error() error {
	k.RLock()
	defer k.RUnlock()
	return k.err
}

// Done returns channel that is closed whenever
// keep aliver is closed
func (k *authKeepAliver) Done() <-chan struct{} {
	return k.ctx.Done()
}

// Close closes keep aliver and cancels all goroutines
func (k *authKeepAliver) Close() error {
	k.cancel()
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

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(generateRequestsCount)
	prometheus.MustRegister(generateThrottledRequestsCount)
	prometheus.MustRegister(generateRequestsCurrent)
	prometheus.MustRegister(generateRequestsLatencies)
}
