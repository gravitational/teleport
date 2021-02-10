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

// Package server implements certificate signing authority and access control server
// Authority server is composed of several parts:
//
// * Authority server itself that implements signing and acl logic
// * HTTP server wrapper for authority server
//
package server

import (
	"context"
	"crypto"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/local"
	"github.com/gravitational/teleport/lib/auth/resource"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/coreos/go-oidc/oauth2"
	"github.com/coreos/go-oidc/oidc"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
	"github.com/prometheus/client_golang/prometheus"
	saml2 "github.com/russellhaering/gosaml2"
	"golang.org/x/crypto/ssh"
)

// Option allows setting options as functional arguments to Server
type Option func(*Server)

// PackedKeys is a collection of private key, SSH host certificate
// and TLS certificate and certificate authority issued the certificate
type PackedKeys struct {
	// Key is a private key
	Key []byte `json:"key"`
	// Cert is an SSH host cert
	Cert []byte `json:"cert"`
	// TLSCert is an X509 certificate
	TLSCert []byte `json:"tls_cert"`
	// TLSCACerts is a list of TLS certificate authorities.
	TLSCACerts [][]byte `json:"tls_ca_certs"`
	// SSHCACerts is a list of SSH certificate authorities.
	SSHCACerts [][]byte `json:"ssh_ca_certs"`
}

// New creates and configures a new Server instance
func New(cfg *InitConfig, opts ...Option) (*Server, error) {
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
	if cfg.DynamicAccessExt == nil {
		cfg.DynamicAccessExt = local.NewDynamicAccessService(cfg.Backend)
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
	if cfg.Emitter == nil {
		cfg.Emitter = events.NewDiscardEmitter()
	}
	if cfg.Streamer == nil {
		cfg.Streamer = events.NewDiscardEmitter()
	}

	limiter, err := limiter.NewConnectionsLimiter(limiter.Config{
		MaxConnections: defaults.LimiterMaxConcurrentSignatures,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	closeCtx, cancelFunc := context.WithCancel(context.TODO())
	as := Server{
		bk:              cfg.Backend,
		limiter:         limiter,
		Authority:       cfg.Authority,
		AuthServiceName: cfg.AuthServiceName,
		oidcClients:     make(map[string]*oidcClient),
		samlProviders:   make(map[string]*samlProvider),
		githubClients:   make(map[string]*githubClient),
		caSigningAlg:    cfg.CASigningAlg,
		cancelFunc:      cancelFunc,
		closeCtx:        closeCtx,
		emitter:         cfg.Emitter,
		streamer:        cfg.Streamer,
		Services: Services{
			Trust:                cfg.Trust,
			Presence:             cfg.Presence,
			Provisioner:          cfg.Provisioner,
			Identity:             cfg.Identity,
			Access:               cfg.Access,
			DynamicAccessExt:     cfg.DynamicAccessExt,
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

// Services collects all auth server API modalities
type Services struct {
	local.Trust
	local.Presence
	local.Provisioner
	local.Identity
	local.Access
	local.ClusterConfiguration

	auth.DynamicAccessExt

	services.Events

	events.IAuditLog
}

// GetWebSession returns existing web session described by req.
// Implements ReadAccessPoint
func (r Services) GetWebSession(ctx context.Context, req types.GetWebSessionRequest) (types.WebSession, error) {
	return r.Identity.WebSessions().Get(ctx, req)
}

// GetWebToken returns existing web token described by req.
// Implements ReadAccessPoint
func (r Services) GetWebToken(ctx context.Context, req types.GetWebTokenRequest) (types.WebToken, error) {
	return r.Identity.WebTokens().Get(ctx, req)
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
			Help: "Number of current generate requests for server keys",
		},
	)
	generateRequestsLatencies = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: teleport.MetricGenerateRequestsHistogram,
			Help: "Latency for generate requests for server keys",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
	)
)

// Server keeps the cluster together. It acts as a certificate authority (CA) for
// a cluster and:
//   - generates the keypair for the node it's running on
//	 - invites other SSH nodes to a cluster, by issuing invite tokens
//	 - adds other SSH nodes to a cluster, by checking their token and signing their keys
//   - same for users and their sessions
//   - checks public keys to see if they're signed by it (can be trusted or not)
type Server struct {
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

	// Services encapsulate services - provisioner, trust, etc
	// used by the auth server in a separate structure
	Services

	// privateKey is used in tests to use pre-generated private keys
	privateKey []byte

	// cipherSuites is a list of ciphersuites that the auth server supports.
	cipherSuites []uint16

	// caSigningAlg is an SSH signing algorithm to use when generating new CAs.
	caSigningAlg *string

	// cache is a fast cache that allows auth server
	// to use cache for most frequent operations,
	// if not set, cache uses itself
	cache auth.Cache

	limiter *limiter.ConnectionsLimiter

	// Emitter is events emitter, used to submit discrete events
	emitter events.Emitter

	// streamer is events sessionstreamer, used to create continuous
	// session related streams
	streamer events.Streamer
}

// SetCache sets cache used by auth server
func (a *Server) SetCache(clt auth.Cache) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.cache = clt
}

// GetCache returns cache used by auth server
func (a *Server) GetCache() auth.Cache {
	a.lock.RLock()
	defer a.lock.RUnlock()
	if a.cache == nil {
		return &a.Services
	}
	return a.cache
}

// runPeriodicOperations runs some periodic bookkeeping operations
// performed by auth server
func (a *Server) runPeriodicOperations() {
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
			err := a.AutoRotateCertAuthorities()
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

func (a *Server) Close() error {
	a.cancelFunc()
	if a.bk != nil {
		return trace.Wrap(a.bk.Close())
	}
	return nil
}

func (a *Server) GetClock() clockwork.Clock {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.clock
}

// SetClock sets clock, used in tests
func (a *Server) SetClock(clock clockwork.Clock) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.clock = clock
}

// SetAuditLog sets the server's audit log
func (a *Server) SetAuditLog(auditLog events.IAuditLog) {
	a.IAuditLog = auditLog
}

// GetClusterConfig gets ClusterConfig from the backend.
func (a *Server) GetClusterConfig(opts ...auth.MarshalOption) (services.ClusterConfig, error) {
	return a.GetCache().GetClusterConfig(opts...)
}

// GetClusterName returns the domain name that identifies this authority server.
// Also known as "cluster name"
func (a *Server) GetClusterName(opts ...auth.MarshalOption) (services.ClusterName, error) {
	return a.GetCache().GetClusterName(opts...)
}

// GetDomainName returns the domain name that identifies this authority server.
// Also known as "cluster name"
func (a *Server) GetDomainName() (string, error) {
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
func (a *Server) GetClusterCACert() (*LocalCAResponse, error) {
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
	tlsCA, err := tlsca.FromAuthority(hostCA)
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
func (a *Server) GenerateHostCert(hostPublicKey []byte, hostID, nodeName string, principals []string, clusterName string, roles teleport.Roles, ttl time.Duration) ([]byte, error) {
	domainName, err := a.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// get the certificate authority that will be signing the public key of the host
	ca, err := a.Trust.GetCertAuthority(services.CertAuthID{
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
	return a.Authority.GenerateHostCert(auth.HostCertParams{
		PrivateCASigningKey: caPrivateKey,
		CASigningAlg:        sshutils.GetSigningAlgName(ca),
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
	// impersonator is a user who generates the certificate,
	// is set when different from the user in the certificate
	impersonator string
	// checker is used to perform RBAC checks.
	checker auth.AccessChecker
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
	// routeToCluster is an optional teleport cluster name to route the
	// certificate requests to, this teleport cluster name will be used to
	// route the requests to in case of kubernetes
	routeToCluster string
	// kubernetesCluster specifies the target kubernetes cluster for TLS
	// identities. This can be empty on older Teleport clients.
	kubernetesCluster string
	// traits hold claim data used to populate a role at runtime.
	traits wrappers.Traits
	// activeRequests tracks privilege escalation requests applied
	// during the construction of the certificate.
	activeRequests auth.RequestIDs
	// appSessionID is the session ID of the application session.
	appSessionID string
	// appPublicAddr is the public address of the application.
	appPublicAddr string
	// appClusterName is the name of the cluster this application is in.
	appClusterName string
	// appName is the name of the application to generate cert for.
	appName string
	// dbService identifies the name of the database service requests will
	// be routed to.
	dbService string
	// dbProtocol specifies the protocol of the database a certificate will
	// be issued for.
	dbProtocol string
	// dbUser is the optional database user which, if provided, will be used
	// as a default username.
	dbUser string
	// dbName is the optional database name which, if provided, will be used
	// as a default database.
	dbName string
	// mfaVerified is the UUID of an MFA device when this certRequest was
	// created immediately after an MFA check.
	mfaVerified string
	// clientIP is an IP of the client requesting the certificate.
	clientIP string
}

type certRequestOption func(*certRequest)

func certRequestMFAVerified(mfaID string) certRequestOption {
	return func(r *certRequest) { r.mfaVerified = mfaID }
}
func certRequestClientIP(ip string) certRequestOption {
	return func(r *certRequest) { r.clientIP = ip }
}

// TestUserCertRequest defines the request to generate user certificates in tests
type TestUserCertRequest struct {
	// Key specifies the public key
	Key []byte
	// User specifies the user
	User string
	// TTL defines the time to live
	TTL time.Duration
	// Usage optionally lists accepted key usage strings
	Usage []string
	// Compatibility optionally specifies the certificate compatibility mode
	Compatibility string
	// RouteToCluster optionally specifies the cluster name to route certificate requests
	RouteToCluster string
}

// GenerateUserTestCerts is used to generate user certificate, used internally for tests
func (a *Server) GenerateUserTestCerts(req TestUserCertRequest) ([]byte, []byte, error) {
	user, err := a.Identity.GetUser(req.User, false)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	checker, err := auth.FetchRoles(user.GetRoles(), a.Access, user.GetTraits())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	certs, err := a.generateUserCert(certRequest{
		user:           user,
		ttl:            req.TTL,
		compatibility:  req.Compatibility,
		publicKey:      req.Key,
		routeToCluster: req.RouteToCluster,
		usage:          req.Usage,
		checker:        checker,
		traits:         user.GetTraits(),
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return certs.ssh, certs.tls, nil
}

// AppTestCertRequest combines parameters for generating a test app access cert.
type AppTestCertRequest struct {
	// PublicKey is the public key to sign.
	PublicKey []byte
	// Username is the Teleport user name to sign certificate for.
	Username string
	// TTL is the test certificate validity period.
	TTL time.Duration
	// PublicAddr is the application public address. Used for routing.
	PublicAddr string
	// ClusterName is the name of the cluster application resides in. Used for routing.
	ClusterName string
	// SessionID is the optional session ID to encode. Used for routing.
	SessionID string
}

// GenerateUserAppTestCert generates an application specific certificate, used
// internally for tests.
func (a *Server) GenerateUserAppTestCert(req AppTestCertRequest) ([]byte, error) {
	user, err := a.Identity.GetUser(req.Username, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := auth.FetchRoles(user.GetRoles(), a.Access, user.GetTraits())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.New()
	}
	certs, err := a.generateUserCert(certRequest{
		user:      user,
		publicKey: req.PublicKey,
		checker:   checker,
		ttl:       req.TTL,
		// Set the login to be a random string. Application certificates are never
		// used to log into servers but SSH certificate generation code requires a
		// principal be in the certificate.
		traits: wrappers.Traits(map[string][]string{
			teleport.TraitLogins: {uuid.New()},
		}),
		// Only allow this certificate to be used for applications.
		usage: []string{teleport.UsageAppsOnly},
		// Add in the application routing information.
		appSessionID:   sessionID,
		appPublicAddr:  req.PublicAddr,
		appClusterName: req.ClusterName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return certs.tls, nil
}

// DatabaseTestCertRequest combines parameters for generating test database
// access certificate.
type DatabaseTestCertRequest struct {
	// PublicKey is the public key to sign.
	PublicKey []byte
	// Cluster is the Teleport cluster name.
	Cluster string
	// Username is the Teleport username.
	Username string
	// RouteToDatabase contains database routing information.
	RouteToDatabase tlsca.RouteToDatabase
}

// GenerateDatabaseTestCert generates a database access certificate for the
// provided parameters. Used only internally in tests.
func (a *Server) GenerateDatabaseTestCert(req DatabaseTestCertRequest) ([]byte, error) {
	user, err := a.Identity.GetUser(req.Username, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := auth.FetchRoles(user.GetRoles(), a.Access, user.GetTraits())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certs, err := a.generateUserCert(certRequest{
		user:      user,
		publicKey: req.PublicKey,
		checker:   checker,
		ttl:       time.Hour,
		traits: wrappers.Traits(map[string][]string{
			teleport.TraitLogins: {req.Username},
		}),
		routeToCluster: req.Cluster,
		dbService:      req.RouteToDatabase.ServiceName,
		dbProtocol:     req.RouteToDatabase.Protocol,
		dbUser:         req.RouteToDatabase.Username,
		dbName:         req.RouteToDatabase.Database,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return certs.tls, nil
}

// generateUserCert generates user certificates
func (a *Server) generateUserCert(req certRequest) (*certs, error) {
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
		certificateFormat = req.checker.CertificateFormat()
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
		allowedLogins, err = req.checker.CheckLoginDuration(0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		// Adjust session TTL to the smaller of two values: the session TTL
		// requested in tsh or the session TTL for the role.
		sessionTTL = req.checker.AdjustSessionTTL(req.ttl)

		// Return a list of logins that meet the session TTL limit. This means if
		// the requested session TTL is larger than the max session TTL for a login,
		// that login will not be included in the list of allowed logins.
		allowedLogins, err = req.checker.CheckLoginDuration(sessionTTL)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	clusterName, err := a.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if req.routeToCluster != "" && clusterName != req.routeToCluster {
		// Authorize access to a remote cluster.
		rc, err := a.Presence.GetRemoteCluster(req.routeToCluster)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := req.checker.CheckAccessToRemoteCluster(rc); err != nil {
			if trace.IsAccessDenied(err) {
				return nil, trace.NotFound("remote cluster %q not found", req.routeToCluster)
			}
			return nil, trace.Wrap(err)
		}
	}

	ca, err := a.Trust.GetCertAuthority(services.CertAuthID{
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
	params := auth.UserCertParams{
		PrivateCASigningKey:   privateKey,
		CASigningAlg:          sshutils.GetSigningAlgName(ca),
		PublicUserKey:         req.publicKey,
		Username:              req.user.GetName(),
		Impersonator:          req.impersonator,
		AllowedLogins:         allowedLogins,
		TTL:                   sessionTTL,
		Roles:                 req.checker.RoleNames(),
		CertificateFormat:     certificateFormat,
		PermitPortForwarding:  req.checker.CanPortForward(),
		PermitAgentForwarding: req.checker.CanForwardAgents(),
		PermitX11Forwarding:   req.checker.PermitX11Forwarding(),
		RouteToCluster:        req.routeToCluster,
		Traits:                req.traits,
		ActiveRequests:        req.activeRequests,
		MFAVerified:           req.mfaVerified,
		ClientIP:              req.clientIP,
	}
	sshCert, err := a.Authority.GenerateUserCert(params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	kubeGroups, kubeUsers, err := req.checker.CheckKubeGroupsAndUsers(sessionTTL, req.overrideRoleTTL)
	// NotFound errors are acceptable - this user may have no k8s access
	// granted and that shouldn't prevent us from issuing a TLS cert.
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	// Only validate/default kubernetes cluster name for the current teleport
	// cluster. If this cert is targeting a trusted teleport cluster, leave all
	// the kubernetes cluster validation up to them.
	if req.routeToCluster == "" || req.routeToCluster == clusterName {
		req.kubernetesCluster, err = kubeutils.CheckOrSetKubeCluster(a.closeCtx, a.Presence, req.kubernetesCluster, clusterName)
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			log.Debug("Failed setting default kubernetes cluster for user login (user did not provide a cluster); leaving KubernetesCluster extension in the TLS certificate empty")
		}
	}

	// See which database names and users this user is allowed to use.
	dbNames, dbUsers, err := req.checker.CheckDatabaseNamesAndUsers(sessionTTL, req.overrideRoleTTL)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	// generate TLS certificate
	tlsAuthority, err := tlsca.FromAuthority(ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity := tlsca.Identity{
		Username:          req.user.GetName(),
		Impersonator:      req.impersonator,
		Groups:            req.checker.RoleNames(),
		Principals:        allowedLogins,
		Usage:             req.usage,
		RouteToCluster:    req.routeToCluster,
		KubernetesCluster: req.kubernetesCluster,
		Traits:            req.traits,
		KubernetesGroups:  kubeGroups,
		KubernetesUsers:   kubeUsers,
		RouteToApp: tlsca.RouteToApp{
			SessionID:   req.appSessionID,
			PublicAddr:  req.appPublicAddr,
			ClusterName: req.appClusterName,
			Name:        req.appName,
		},
		TeleportCluster: clusterName,
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: req.dbService,
			Protocol:    req.dbProtocol,
			Username:    req.dbUser,
			Database:    req.dbName,
		},
		DatabaseNames: dbNames,
		DatabaseUsers: dbUsers,
		MFAVerified:   req.mfaVerified,
		ClientIP:      req.clientIP,
	}
	subject, err := identity.Subject()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certRequest := tlsca.CertificateRequest{
		Clock:     a.clock,
		PublicKey: cryptoPubKey,
		Subject:   subject,
		NotAfter:  a.clock.Now().UTC().Add(sessionTTL),
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
func (a *Server) WithUserLock(username string, authenticateFn func() error) error {
	user, err := a.Identity.GetUser(username, false)
	if err != nil {
		if trace.IsNotFound(err) {
			// If user is not found, still call authenticateFn. It should
			// always return an error. This prevents username oracles and
			// timing attacks.
			return authenticateFn()
		}
		return trace.Wrap(err)
	}
	status := user.GetStatus()
	if status.IsLocked && status.LockExpires.After(a.clock.Now().UTC()) {
		return trace.AccessDenied("%v exceeds %v failed login attempts, locked until %v",
			user.GetName(), defaults.MaxLoginAttempts, utils.HumanTimeFormat(status.LockExpires))
	}
	fnErr := authenticateFn()
	if fnErr == nil {
		// upon successful login, reset the failed attempt counter
		err = a.DeleteUserLoginAttempts(username)
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
	attempt := auth.LoginAttempt{Time: a.clock.Now().UTC(), Success: false}
	err = a.AddUserLoginAttempt(username, attempt, defaults.AttemptTTL)
	if err != nil {
		log.Error(trace.DebugReport(err))
		return trace.Wrap(fnErr)
	}
	loginAttempts, err := a.Identity.GetUserLoginAttempts(username)
	if err != nil {
		log.Error(trace.DebugReport(err))
		return trace.Wrap(fnErr)
	}
	if !auth.LastFailed(defaults.MaxLoginAttempts, loginAttempts) {
		log.Debugf("%v user has less than %v failed login attempts", username, defaults.MaxLoginAttempts)
		return trace.Wrap(fnErr)
	}
	lockUntil := a.clock.Now().UTC().Add(defaults.AccountLockInterval)
	message := fmt.Sprintf("%v exceeds %v failed login attempts, locked until %v",
		username, defaults.MaxLoginAttempts, utils.HumanTimeFormat(status.LockExpires))
	log.Debug(message)
	user.SetLocked(lockUntil, "user has exceeded maximum failed login attempts")
	err = a.Identity.UpsertUser(user)
	if err != nil {
		log.Error(trace.DebugReport(err))
		return trace.Wrap(fnErr)
	}
	return trace.AccessDenied(message)
}

// PreAuthenticatedSignIn is for 2-way authentication methods like U2F where the password is
// already checked before issuing the second factor challenge
func (a *Server) PreAuthenticatedSignIn(user string, identity tlsca.Identity) (services.WebSession, error) {
	roles, traits, err := resource.ExtractFromIdentity(a, identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess, err := a.NewWebSession(types.NewWebSessionRequest{
		User:   user,
		Roles:  roles,
		Traits: traits,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.upsertWebSession(context.TODO(), user, sess); err != nil {
		return nil, trace.Wrap(err)
	}
	return sess.WithoutSecrets(), nil
}

// MFAAuthenticateChallenge is a U2F authentication challenge sent on user
// login.
type MFAAuthenticateChallenge struct {
	// Before 6.0 teleport would only send 1 U2F challenge. Embed the old
	// challenge for compatibility with older clients. All new clients should
	// ignore this and read Challenges instead.
	*u2f.AuthenticateChallenge

	// U2FChallenges is a list of U2F challenges, one for each registered
	// device.
	U2FChallenges []u2f.AuthenticateChallenge `json:"u2f_challenges"`
	// TOTPChallenge specifies whether TOTP is supported for this user.
	TOTPChallenge bool `json:"totp_challenge"`
}

func (a *Server) GetMFAAuthenticateChallenge(user string, password []byte) (*MFAAuthenticateChallenge, error) {
	ctx := context.TODO()

	err := a.WithUserLock(user, func() error {
		return a.CheckPasswordWOToken(user, password)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	protoChal, err := a.mfaAuthChallenge(ctx, user, a.Identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Convert from proto to JSON format.
	chal := &MFAAuthenticateChallenge{
		TOTPChallenge: protoChal.TOTP != nil,
	}
	for _, u2fChal := range protoChal.U2F {
		ch := u2f.AuthenticateChallenge{
			Version:   u2fChal.Version,
			Challenge: u2fChal.Challenge,
			KeyHandle: u2fChal.KeyHandle,
			AppID:     u2fChal.AppID,
		}
		if chal.AuthenticateChallenge == nil {
			chal.AuthenticateChallenge = &ch
		}
		chal.U2FChallenges = append(chal.U2FChallenges, ch)
	}

	return chal, nil
}

func (a *Server) CheckU2FSignResponse(ctx context.Context, user string, response *u2f.AuthenticateChallengeResponse) (*types.MFADevice, error) {
	// before trying to register a user, see U2F is actually setup on the backend
	cap, err := a.GetAuthPreference()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = cap.GetU2F()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return a.checkU2F(ctx, user, *response, a.Identity)
}

// ExtendWebSession creates a new web session for a user based on a valid previous session.
// Additional roles are appended to initial roles if there is an approved access request.
// The new session expiration time will not exceed the expiration time of the old session.
func (a *Server) ExtendWebSession(user, prevSessionID, accessRequestID string, identity tlsca.Identity) (services.WebSession, error) {
	prevSession, err := a.GetWebSession(context.TODO(), types.GetWebSessionRequest{
		User:      user,
		SessionID: prevSessionID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// consider absolute expiry time that may be set for this session
	// by some external identity serivce, so we can not renew this session
	// any more without extra logic for renewal with external OIDC provider
	expiresAt := prevSession.GetExpiryTime()
	if !expiresAt.IsZero() && expiresAt.Before(a.clock.Now().UTC()) {
		return nil, trace.NotFound("web session has expired")
	}

	roles, traits, err := resource.ExtractFromIdentity(a, identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if accessRequestID != "" {
		newRoles, requestExpiry, err := a.getRolesAndExpiryFromAccessRequest(user, accessRequestID)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		roles = append(roles, newRoles...)
		roles = utils.Deduplicate(roles)

		// Let session expire with the shortest expiry time.
		if expiresAt.After(requestExpiry) {
			expiresAt = requestExpiry
		}
	}

	sessionTTL := utils.ToTTL(a.clock, expiresAt)
	sess, err := a.NewWebSession(types.NewWebSessionRequest{
		User:       user,
		Roles:      roles,
		Traits:     traits,
		SessionTTL: sessionTTL,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.upsertWebSession(context.TODO(), user, sess); err != nil {
		return nil, trace.Wrap(err)
	}

	return sess, nil
}

func (a *Server) getRolesAndExpiryFromAccessRequest(user, accessRequestID string) ([]string, time.Time, error) {
	reqFilter := services.AccessRequestFilter{
		User: user,
		ID:   accessRequestID,
	}

	reqs, err := a.GetAccessRequests(context.TODO(), reqFilter)
	if err != nil {
		return nil, time.Time{}, trace.Wrap(err)
	}

	if len(reqs) < 1 {
		return nil, time.Time{}, trace.NotFound("access request %q not found", accessRequestID)
	}

	req := reqs[0]

	if !req.GetState().IsApproved() {
		if req.GetState().IsDenied() {
			return nil, time.Time{}, trace.AccessDenied("access request %q has been denied", accessRequestID)
		}
		return nil, time.Time{}, trace.BadParameter("access request %q is awaiting approval", accessRequestID)
	}

	if err := auth.ValidateAccessRequestForUser(a, req); err != nil {
		return nil, time.Time{}, trace.Wrap(err)
	}

	accessExpiry := req.GetAccessExpiry()
	if accessExpiry.Before(a.GetClock().Now()) {
		return nil, time.Time{}, trace.BadParameter("access request %q has expired", accessRequestID)
	}

	return req.GetRoles(), accessExpiry, nil
}

// CreateWebSession creates a new web session for user without any
// checks, is used by admins
func (a *Server) CreateWebSession(user string) (services.WebSession, error) {
	u, err := a.GetUser(user, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess, err := a.NewWebSession(types.NewWebSessionRequest{
		User:   user,
		Roles:  u.GetRoles(),
		Traits: u.GetTraits(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.upsertWebSession(context.TODO(), user, sess); err != nil {
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
	// Labels sets token labels, e.g. {env: prod, region: us-west}.
	// Labels are later passed to resources that are joining
	// e.g. remote clusters and in the future versions, nodes and proxies.
	Labels map[string]string `json:"labels"`
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

// GenerateToken generates multi-purpose authentication token.
func (a *Server) GenerateToken(ctx context.Context, req GenerateTokenRequest) (string, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return "", trace.Wrap(err)
	}
	token, err := services.NewProvisionToken(req.Token, req.Roles, a.clock.Now().UTC().Add(req.TTL))
	if err != nil {
		return "", trace.Wrap(err)
	}
	if len(req.Labels) != 0 {
		meta := token.GetMetadata()
		meta.Labels = req.Labels
		token.SetMetadata(meta)
	}

	if err := a.Provisioner.UpsertToken(ctx, token); err != nil {
		return "", trace.Wrap(err)
	}

	user := ClientUsername(ctx)
	for _, role := range req.Roles {
		if role == teleport.RoleTrustedCluster {
			if err := a.emitter.EmitAuditEvent(ctx, &events.TrustedClusterTokenCreate{
				Metadata: events.Metadata{
					Type: events.TrustedClusterTokenCreateEvent,
					Code: events.TrustedClusterTokenCreateCode,
				},
				UserMetadata: events.UserMetadata{
					User:         user,
					Impersonator: ClientImpersonator(ctx),
				},
			}); err != nil {
				log.WithError(err).Warn("Failed to emit trusted cluster token create event.")
			}
		}
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
func (a *Server) GenerateServerKeys(req GenerateServerKeysRequest) (*PackedKeys, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.limiter.AcquireConnection(req.Roles.String()); err != nil {
		generateThrottledRequestsCount.Inc()
		log.Debugf("Node %q [%v] is rate limited: %v.", req.NodeName, req.HostID, req.Roles)
		return nil, trace.Wrap(err)
	}
	defer a.limiter.ReleaseConnection(req.Roles.String())

	// only observe latencies for non-throttled requests
	start := a.clock.Now()
	defer generateRequestsLatencies.Observe(time.Since(start).Seconds())

	generateRequestsCount.Inc()
	generateRequestsCurrent.Inc()
	defer generateRequestsCurrent.Dec()

	clusterName, err := a.GetClusterName()
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
		privateKeyPEM, pubSSHKey, err = a.GenerateKeyPair("")
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
	client := a.GetCache()
	if req.NoCache {
		client = &a.Services
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
		ca, err = a.GetCertAuthority(services.CertAuthID{
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

	tlsAuthority, err := tlsca.FromAuthority(ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// get the private key of the certificate authority
	caPrivateKey, err := ca.FirstSigningKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// generate hostSSH certificate
	hostSSHCert, err := a.Authority.GenerateHostCert(auth.HostCertParams{
		PrivateCASigningKey: caPrivateKey,
		CASigningAlg:        sshutils.GetSigningAlgName(ca),
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
		Username:        HostFQDN(req.HostID, clusterName.GetClusterName()),
		Groups:          req.Roles.StringSlice(),
		TeleportCluster: clusterName.GetClusterName(),
	}
	subject, err := identity.Subject()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certRequest := tlsca.CertificateRequest{
		Clock:     a.clock,
		PublicKey: cryptoPubKey,
		Subject:   subject,
		NotAfter:  a.clock.Now().UTC().Add(defaults.CATTL),
		DNSNames:  append([]string{}, req.AdditionalPrincipals...),
	}
	// HTTPS requests need to specify DNS name that should be present in the
	// certificate as one of the DNS Names. It is not known in advance,
	// that is why there is a default one for all certificates
	if req.Roles.Include(teleport.RoleAuth) || req.Roles.Include(teleport.RoleAdmin) || req.Roles.Include(teleport.RoleApp) {
		certRequest.DNSNames = append(certRequest.DNSNames, "*."+teleport.APIDomain, teleport.APIDomain)
	}
	// Unlike additional principals, DNS Names is x509 specific and is limited
	// to services with TLS endpoints (e.g. auth, proxies, kubernetes)
	if req.Roles.Include(teleport.RoleAuth) || req.Roles.Include(teleport.RoleAdmin) || req.Roles.Include(teleport.RoleProxy) || req.Roles.Include(teleport.RoleKube) {
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
		TLSCACerts: auth.GetTLSCerts(ca),
		SSHCACerts: ca.GetCheckingKeys(),
	}, nil
}

// ValidateToken takes a provisioning token value and finds if it's valid. Returns
// a list of roles this token allows its owner to assume and token labels, or an error if the token
// cannot be found.
func (a *Server) ValidateToken(token string) (teleport.Roles, map[string]string, error) {
	ctx := context.TODO()
	tkns, err := a.GetCache().GetStaticTokens()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// First check if the token is a static token. If it is, return right away.
	// Static tokens have no expiration.
	for _, st := range tkns.GetStaticTokens() {
		if subtle.ConstantTimeCompare([]byte(st.GetName()), []byte(token)) == 1 {
			return st.GetRoles(), nil, nil
		}
	}

	// If it's not a static token, check if it's a ephemeral token in the backend.
	// If a ephemeral token is found, make sure it's still valid.
	tok, err := a.GetCache().GetToken(ctx, token)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if !a.checkTokenTTL(tok) {
		return nil, nil, trace.AccessDenied("token expired")
	}

	return tok.GetRoles(), tok.GetMetadata().Labels, nil
}

// checkTokenTTL checks if the token is still valid. If it is not, the token
// is removed from the backend and returns false. Otherwise returns true.
func (a *Server) checkTokenTTL(tok services.ProvisionToken) bool {
	ctx := context.TODO()
	now := a.clock.Now().UTC()
	if tok.Expiry().Before(now) {
		err := a.DeleteToken(ctx, tok.GetName())
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
func (a *Server) RegisterUsingToken(req RegisterUsingTokenRequest) (*PackedKeys, error) {
	log.Infof("Node %q [%v] is trying to join with role: %v.", req.NodeName, req.HostID, req.Role)

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// make sure the token is valid
	roles, _, err := a.ValidateToken(req.Token)
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
	keys, err := a.GenerateServerKeys(GenerateServerKeysRequest{
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

func (a *Server) RegisterNewAuthServer(ctx context.Context, token string) error {
	tok, err := a.Provisioner.GetToken(ctx, token)
	if err != nil {
		return trace.Wrap(err)
	}
	if !tok.GetRoles().Include(teleport.RoleAuth) {
		return trace.AccessDenied("role does not match")
	}
	if err := a.DeleteToken(ctx, token); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (a *Server) DeleteToken(ctx context.Context, token string) (err error) {
	tkns, err := a.GetStaticTokens()
	if err != nil {
		return trace.Wrap(err)
	}

	// is this a static token?
	for _, st := range tkns.GetStaticTokens() {
		if subtle.ConstantTimeCompare([]byte(st.GetName()), []byte(token)) == 1 {
			return trace.BadParameter("token %s is statically configured and cannot be removed", token)
		}
	}
	// delete reset password token:
	if err = a.Identity.DeleteResetPasswordToken(ctx, token); err == nil {
		return nil
	}
	// delete node token:
	if err = a.Provisioner.DeleteToken(ctx, token); err == nil {
		return nil
	}
	return trace.Wrap(err)
}

// GetTokens returns all tokens (machine provisioning ones and user invitation tokens). Machine
// tokens usually have "node roles", like auth,proxy,node and user invitation tokens have 'signup' role
func (a *Server) GetTokens(ctx context.Context, opts ...auth.MarshalOption) (tokens []services.ProvisionToken, err error) {
	// get node tokens:
	tokens, err = a.Provisioner.GetTokens(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// get static tokens:
	tkns, err := a.GetStaticTokens()
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if err == nil {
		tokens = append(tokens, tkns.GetStaticTokens()...)
	}
	// get reset password tokens:
	resetPasswordTokens, err := a.Identity.GetResetPasswordTokens(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// convert reset password tokens to machine tokens:
	for _, t := range resetPasswordTokens {
		roles := teleport.Roles{teleport.RoleSignup}
		tok, err := services.NewProvisionToken(t.GetName(), roles, t.Expiry())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tokens = append(tokens, tok)
	}
	return tokens, nil
}

// NewWebSession creates and returns a new web session for the specified request
func (a *Server) NewWebSession(req types.NewWebSessionRequest) (services.WebSession, error) {
	user, err := a.GetUser(req.User, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := auth.FetchRoles(req.Roles, a.Access, req.Traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	priv, pub, err := a.GetNewKeyPairFromPool()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sessionTTL := req.SessionTTL
	if sessionTTL == 0 {
		sessionTTL = checker.AdjustSessionTTL(defaults.CertDuration)
	}
	certs, err := a.generateUserCert(certRequest{
		user:      user,
		ttl:       sessionTTL,
		publicKey: pub,
		checker:   checker,
		traits:    req.Traits,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	token, err := utils.CryptoRandomHex(SessionTokenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	bearerToken, err := utils.CryptoRandomHex(SessionTokenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	bearerTokenTTL := utils.MinTTL(sessionTTL, BearerTokenTTL)
	return services.NewWebSession(token, services.KindWebSession, services.KindWebSession, services.WebSessionSpecV2{
		User:               req.User,
		Priv:               priv,
		Pub:                certs.ssh,
		TLSCert:            certs.tls,
		Expires:            a.clock.Now().UTC().Add(sessionTTL),
		BearerToken:        bearerToken,
		BearerTokenExpires: a.clock.Now().UTC().Add(bearerTokenTTL),
	}), nil
}

// GetWebSessionInfo returns the web session specified with sessionID for the given user.
// The session is stripped of any authentication details.
// Implements auth.WebUIService
func (a *Server) GetWebSessionInfo(ctx context.Context, user, sessionID string) (services.WebSession, error) {
	sess, err := a.Identity.WebSessions().Get(ctx, types.GetWebSessionRequest{User: user, SessionID: sessionID})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sess.WithoutSecrets(), nil
}

func (a *Server) DeleteNamespace(namespace string) error {
	if namespace == defaults.Namespace {
		return trace.AccessDenied("can't delete default namespace")
	}
	nodes, err := a.Presence.GetNodes(namespace, resource.SkipValidation())
	if err != nil {
		return trace.Wrap(err)
	}
	if len(nodes) != 0 {
		return trace.BadParameter("can't delete namespace %v that has %v registered nodes", namespace, len(nodes))
	}
	return a.Presence.DeleteNamespace(namespace)
}

// NewWatcher returns a new event watcher. In case of an auth server
// this watcher will return events as seen by the auth server's
// in memory cache, not the backend.
func (a *Server) NewWatcher(ctx context.Context, watch services.Watch) (services.Watcher, error) {
	return a.GetCache().NewWatcher(ctx, watch)
}

// DeleteRole deletes a role by name of the role.
func (a *Server) DeleteRole(ctx context.Context, name string) error {
	// check if this role is used by CA or Users
	users, err := a.Identity.GetUsers(false)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, u := range users {
		for _, r := range u.GetRoles() {
			if r == name {
				// Mask the actual error here as it could be used to enumerate users
				// within the system.
				log.Warnf("Failed to delete role: role %v is used by user %v.", name, u.GetName())
				return trace.BadParameter("failed to delete role that still in use by a user. Check system server logs for more details.")
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
				// Mask the actual error here as it could be used to enumerate users
				// within the system.
				log.Warnf("Failed to delete role: role %v is used by user cert authority %v", name, a.GetClusterName())
				return trace.BadParameter("failed to delete role that still in use by a user. Check system server logs for more details.")
			}
		}
	}

	if err := a.Access.DeleteRole(ctx, name); err != nil {
		return trace.Wrap(err)
	}

	err = a.emitter.EmitAuditEvent(a.closeCtx, &events.RoleDelete{
		Metadata: events.Metadata{
			Type: events.RoleDeletedEvent,
			Code: events.RoleDeletedCode,
		},
		UserMetadata: events.UserMetadata{
			User:         ClientUsername(ctx),
			Impersonator: ClientImpersonator(ctx),
		},
		ResourceMetadata: events.ResourceMetadata{
			Name: name,
		},
	})
	if err != nil {
		log.WithError(err).Warnf("Failed to emit role deleted event.")
	}

	return nil
}

// UpsertRole creates or updates role.
func (a *Server) upsertRole(ctx context.Context, role services.Role) error {
	if err := a.UpsertRole(ctx, role); err != nil {
		return trace.Wrap(err)
	}

	err := a.emitter.EmitAuditEvent(a.closeCtx, &events.RoleCreate{
		Metadata: events.Metadata{
			Type: events.RoleCreatedEvent,
			Code: events.RoleCreatedCode,
		},
		UserMetadata: events.UserMetadata{
			User: ClientUsername(ctx),
		},
		ResourceMetadata: events.ResourceMetadata{
			Name: role.GetName(),
		},
	})
	if err != nil {
		log.WithError(err).Warnf("Failed to emit role create event.")
	}
	return nil
}

func (a *Server) CreateAccessRequest(ctx context.Context, req services.AccessRequest) error {
	err := auth.ValidateAccessRequestForUser(a, req,
		// if request is in state pending, variable expansion must be applied
		auth.ExpandVars(req.GetState().IsPending()),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	ttl, err := a.calculateMaxAccessTTL(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}
	now := a.clock.Now().UTC()
	req.SetCreationTime(now)
	exp := now.Add(ttl)
	// Set acccess expiry if an allowable default was not provided.
	if req.GetAccessExpiry().Before(now) || req.GetAccessExpiry().After(exp) {
		req.SetAccessExpiry(exp)
	}
	// By default, resource expiry should match access expiry.
	req.SetExpiry(req.GetAccessExpiry())
	// If the access-request is in a pending state, then the expiry of the underlying resource
	// is capped to to PendingAccessDuration in order to limit orphaned access requests.
	if req.GetState().IsPending() {
		pexp := now.Add(defaults.PendingAccessDuration)
		if pexp.Before(req.Expiry()) {
			req.SetExpiry(pexp)
		}
	}
	if err := a.DynamicAccessExt.CreateAccessRequest(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	err = a.emitter.EmitAuditEvent(a.closeCtx, &events.AccessRequestCreate{
		Metadata: events.Metadata{
			Type: events.AccessRequestCreateEvent,
			Code: events.AccessRequestCreateCode,
		},
		UserMetadata: events.UserMetadata{
			User:         req.GetUser(),
			Impersonator: ClientImpersonator(ctx),
		},
		ResourceMetadata: events.ResourceMetadata{
			Expires: req.GetAccessExpiry(),
		},
		Roles:        req.GetRoles(),
		RequestID:    req.GetName(),
		RequestState: req.GetState().String(),
		Reason:       req.GetRequestReason(),
	})
	if err != nil {
		log.WithError(err).Warn("Failed to emit access request create event.")
	}
	return nil
}

func (a *Server) SetAccessRequestState(ctx context.Context, params services.AccessRequestUpdate) error {
	if err := a.DynamicAccessExt.SetAccessRequestState(ctx, params); err != nil {
		return trace.Wrap(err)
	}
	event := &events.AccessRequestCreate{
		Metadata: events.Metadata{
			Type: events.AccessRequestUpdateEvent,
			Code: events.AccessRequestUpdateCode,
		},
		ResourceMetadata: events.ResourceMetadata{
			UpdatedBy: ClientUsername(ctx),
		},
		RequestID:    params.RequestID,
		RequestState: params.State.String(),
		Reason:       params.Reason,
		Roles:        params.Roles,
	}

	if delegator := client.GetDelegator(ctx); delegator != "" {
		event.Delegator = delegator
	}

	if len(params.Annotations) > 0 {
		annotations, err := events.EncodeMapStrings(params.Annotations)
		if err != nil {
			log.WithError(err).Debugf("Failed to encode access request annotations.")
		} else {
			event.Annotations = annotations
		}
	}
	err := a.emitter.EmitAuditEvent(a.closeCtx, event)
	if err != nil {
		log.WithError(err).Warn("Failed to emit access request update event.")
	}
	return trace.Wrap(err)
}

func (a *Server) SubmitAccessReview(ctx context.Context, params types.AccessReviewSubmission) (services.AccessRequest, error) {
	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// set up a checker for the review author
	checker, err := auth.NewReviewPermissionChecker(ctx, a, params.Review.Author)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// don't bother continuing if the author has no allow directives
	if !checker.HasAllowDirectives() {
		return nil, trace.AccessDenied("user %q cannot submit reviews", params.Review.Author)
	}

	// final permission checks and review application must be done by the local backend
	// service, as their validity depends upon optimistic locking.
	req, err := a.DynamicAccessExt.ApplyAccessReview(ctx, params, checker)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	event := &events.AccessRequestCreate{
		Metadata: events.Metadata{
			Type:        events.AccessRequestReviewEvent,
			Code:        events.AccessRequestReviewCode,
			ClusterName: clusterName.GetClusterName(),
		},
		ResourceMetadata: events.ResourceMetadata{
			Expires: req.GetAccessExpiry(),
		},
		RequestID:     params.RequestID,
		RequestState:  req.GetState().String(),
		ProposedState: params.Review.ProposedState.String(),
		Reason:        params.Review.Reason,
		Reviewer:      params.Review.Author,
	}

	if len(params.Review.Annotations) > 0 {
		annotations, err := events.EncodeMapStrings(params.Review.Annotations)
		if err != nil {
			log.WithError(err).Debugf("Failed to encode access request annotations.")
		} else {
			event.Annotations = annotations
		}
	}
	if err := a.emitter.EmitAuditEvent(a.closeCtx, event); err != nil {
		log.WithError(err).Warn("Failed to emit access request update event.")
	}

	return req, nil
}

func (a *Server) GetAccessCapabilities(ctx context.Context, req services.AccessCapabilitiesRequest) (*services.AccessCapabilities, error) {
	caps, err := auth.CalculateAccessCapabilities(ctx, a, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return caps, nil
}

// calculateMaxAccessTTL determines the maximum allowable TTL for a given access request
// based on the MaxSessionTTLs of the roles being requested (a access request's life cannot
// exceed the smallest allowable MaxSessionTTL value of the roles that it requests).
func (a *Server) calculateMaxAccessTTL(ctx context.Context, req services.AccessRequest) (time.Duration, error) {
	minTTL := defaults.MaxAccessDuration
	for _, roleName := range req.GetRoles() {
		role, err := a.GetRole(ctx, roleName)
		if err != nil {
			return 0, trace.Wrap(err)
		}
		roleTTL := time.Duration(role.GetOptions().MaxSessionTTL)
		if roleTTL > 0 && roleTTL < minTTL {
			minTTL = roleTTL
		}
	}
	return minTTL, nil
}

// NewKeepAliver returns a new instance of keep aliver
func (a *Server) NewKeepAliver(ctx context.Context) (services.KeepAliver, error) {
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
func (a *Server) GetCertAuthority(id services.CertAuthID, loadSigningKeys bool, opts ...auth.MarshalOption) (services.CertAuthority, error) {
	return a.GetCache().GetCertAuthority(id, loadSigningKeys, opts...)
}

// GetCertAuthorities returns a list of authorities of a given type
// loadSigningKeys controls whether signing keys should be loaded or not
func (a *Server) GetCertAuthorities(caType services.CertAuthType, loadSigningKeys bool, opts ...auth.MarshalOption) ([]services.CertAuthority, error) {
	return a.GetCache().GetCertAuthorities(caType, loadSigningKeys, opts...)
}

// GetStaticTokens gets the list of static tokens used to provision nodes.
func (a *Server) GetStaticTokens() (services.StaticTokens, error) {
	return a.GetCache().GetStaticTokens()
}

// GetToken finds and returns token by ID
func (a *Server) GetToken(ctx context.Context, token string) (services.ProvisionToken, error) {
	return a.GetCache().GetToken(ctx, token)
}

// GetRoles is a part of auth.AccessPoint implementation
func (a *Server) GetRoles(ctx context.Context) ([]services.Role, error) {
	return a.GetCache().GetRoles(ctx)
}

// GetRole is a part of auth.AccessPoint implementation
func (a *Server) GetRole(ctx context.Context, name string) (services.Role, error) {
	return a.GetCache().GetRole(ctx, name)
}

// GetNamespace returns namespace
func (a *Server) GetNamespace(name string) (*services.Namespace, error) {
	return a.GetCache().GetNamespace(name)
}

// GetNamespaces is a part of auth.AccessPoint implementation
func (a *Server) GetNamespaces() ([]services.Namespace, error) {
	return a.GetCache().GetNamespaces()
}

// GetNodes is a part of auth.AccessPoint implementation
func (a *Server) GetNodes(namespace string, opts ...auth.MarshalOption) ([]services.Server, error) {
	return a.GetCache().GetNodes(namespace, opts...)
}

// GetReverseTunnels is a part of auth.AccessPoint implementation
func (a *Server) GetReverseTunnels(opts ...auth.MarshalOption) ([]services.ReverseTunnel, error) {
	return a.GetCache().GetReverseTunnels(opts...)
}

// GetProxies is a part of auth.AccessPoint implementation
func (a *Server) GetProxies() ([]services.Server, error) {
	return a.GetCache().GetProxies()
}

// GetUser is a part of auth.AccessPoint implementation.
func (a *Server) GetUser(name string, withSecrets bool) (user services.User, err error) {
	return a.GetCache().GetUser(name, withSecrets)
}

// GetUsers is a part of auth.AccessPoint implementation
func (a *Server) GetUsers(withSecrets bool) (users []services.User, err error) {
	return a.GetCache().GetUsers(withSecrets)
}

// GetTunnelConnections is a part of auth.AccessPoint implementation
// GetTunnelConnections are not using recent cache as they are designed
// to be called periodically and always return fresh data
func (a *Server) GetTunnelConnections(clusterName string, opts ...auth.MarshalOption) ([]services.TunnelConnection, error) {
	return a.GetCache().GetTunnelConnections(clusterName, opts...)
}

// GetAllTunnelConnections is a part of auth.AccessPoint implementation
// GetAllTunnelConnections are not using recent cache, as they are designed
// to be called periodically and always return fresh data
func (a *Server) GetAllTunnelConnections(opts ...auth.MarshalOption) (conns []services.TunnelConnection, err error) {
	return a.GetCache().GetAllTunnelConnections(opts...)
}

// CreateAuditStream creates audit event stream
func (a *Server) CreateAuditStream(ctx context.Context, sid session.ID) (events.Stream, error) {
	streamer, err := a.modeStreamer()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return streamer.CreateAuditStream(ctx, sid)
}

// ResumeAuditStream resumes the stream that has been created
func (a *Server) ResumeAuditStream(ctx context.Context, sid session.ID, uploadID string) (events.Stream, error) {
	streamer, err := a.modeStreamer()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return streamer.ResumeAuditStream(ctx, sid, uploadID)
}

// modeStreamer creates streamer based on the event mode
func (a *Server) modeStreamer() (events.Streamer, error) {
	clusterConfig, err := a.GetClusterConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	mode := clusterConfig.GetSessionRecording()
	// In sync mode, auth server forwards session control to the event log
	// in addition to sending them and data events to the record storage.
	if auth.IsRecordSync(mode) {
		return events.NewTeeStreamer(a.streamer, a.emitter), nil
	}
	// In async mode, clients submit session control events
	// during the session in addition to writing a local
	// session recording to be uploaded at the end of the session,
	// so forwarding events here will result in duplicate events.
	return a.streamer, nil
}

// GetAppServers is a part of the auth.AccessPoint implementation.
func (a *Server) GetAppServers(ctx context.Context, namespace string, opts ...auth.MarshalOption) ([]services.Server, error) {
	return a.GetCache().GetAppServers(ctx, namespace, opts...)
}

// GetAppSession is a part of the auth.AccessPoint implementation.
func (a *Server) GetAppSession(ctx context.Context, req services.GetAppSessionRequest) (services.WebSession, error) {
	return a.GetCache().GetAppSession(ctx, req)
}

// GetDatabaseServers returns all registers database proxy servers.
func (a *Server) GetDatabaseServers(ctx context.Context, namespace string, opts ...auth.MarshalOption) ([]types.DatabaseServer, error) {
	return a.GetCache().GetDatabaseServers(ctx, namespace, opts...)
}

func (a *Server) isMFARequired(ctx context.Context, checker auth.AccessChecker, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error) {
	pref, err := a.GetAuthPreference()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if pref.GetRequireSessionMFA() {
		// Cluster always requires MFA, regardless of roles.
		return &proto.IsMFARequiredResponse{Required: true}, nil
	}
	var noMFAAccessErr, notFoundErr error
	switch t := req.Target.(type) {
	case *proto.IsMFARequiredRequest_Node:
		notFoundErr = trace.NotFound("node %q not found", t.Node)
		if t.Node.Node == "" {
			return nil, trace.BadParameter("empty Node field")
		}
		if t.Node.Login == "" {
			return nil, trace.BadParameter("empty Login field")
		}
		// Find the target node and check whether MFA is required.
		nodes, err := a.GetNodes(defaults.Namespace, resource.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var matches []types.Server
		for _, n := range nodes {
			// Get the server address without port number.
			addr, _, err := net.SplitHostPort(n.GetAddr())
			if err != nil {
				addr = n.GetAddr()
			}
			// Match NodeName to UUID, hostname or self-reported server address.
			if n.GetName() == t.Node.Node || n.GetHostname() == t.Node.Node || addr == t.Node.Node {
				matches = append(matches, n)
			}
		}
		if len(matches) == 0 {
			// If t.Node.Node is not a known registered node, it may be an
			// unregistered host running OpenSSH with a certificate created via
			// `tctl auth sign`. In these cases, let the user through without
			// extra checks.
			//
			// If t.Node.Node turns out to be an alias for a real node (e.g.
			// private network IP), and MFA check was actually required, the
			// Node itself will check the cert extensions and reject the
			// connection.
			return &proto.IsMFARequiredResponse{Required: false}, nil
		}
		// Check RBAC against all matching nodes and return the first error.
		// If at least one node requires MFA, we'll catch it.
		for _, n := range matches {
			err := checker.CheckAccessToServer(t.Node.Login, n, auth.AccessMFAParams{AlwaysRequired: false, Verified: false})
			if err != nil {
				noMFAAccessErr = err
				break
			}
		}

	case *proto.IsMFARequiredRequest_KubernetesCluster:
		notFoundErr = trace.NotFound("kubernetes cluster %q not found", t.KubernetesCluster)
		if t.KubernetesCluster == "" {
			return nil, trace.BadParameter("missing KubernetesCluster field in a kubernetes-only UserCertsRequest")
		}
		// Find the target cluster and check whether MFA is required.
		svcs, err := a.GetKubeServices(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var cluster *types.KubernetesCluster
	outer:
		for _, svc := range svcs {
			for _, c := range svc.GetKubernetesClusters() {
				if c.Name == t.KubernetesCluster {
					cluster = c
					break outer
				}
			}
		}
		if cluster == nil {
			return nil, trace.Wrap(notFoundErr)
		}
		noMFAAccessErr = checker.CheckAccessToKubernetes(defaults.Namespace, cluster, auth.AccessMFAParams{AlwaysRequired: false, Verified: false})

	case *proto.IsMFARequiredRequest_Database:
		notFoundErr = trace.NotFound("database service %q not found", t.Database.ServiceName)
		if t.Database.ServiceName == "" {
			return nil, trace.BadParameter("missing ServiceName field in a database-only UserCertsRequest")
		}
		dbs, err := a.GetDatabaseServers(ctx, defaults.Namespace, resource.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var db types.DatabaseServer
		for _, d := range dbs {
			if d.GetName() == t.Database.ServiceName {
				db = d
				break
			}
		}
		if db == nil {
			return nil, trace.Wrap(notFoundErr)
		}
		noMFAAccessErr = checker.CheckAccessToDatabase(db, auth.AccessMFAParams{AlwaysRequired: false, Verified: false})

	default:
		return nil, trace.BadParameter("unknown Target %T", req.Target)
	}
	// No error means that MFA is not required for this resource by
	// AccessChecker.
	if noMFAAccessErr == nil {
		return &proto.IsMFARequiredResponse{Required: false}, nil
	}
	// Errors other than ErrSessionMFARequired mean something else is wrong,
	// most likely access denied.
	if !errors.Is(noMFAAccessErr, auth.ErrSessionMFARequired) {
		if trace.IsAccessDenied(noMFAAccessErr) {
			log.Infof("Access to resource denied: %v", noMFAAccessErr)
			// notFoundErr should always be set by this point, but check it
			// just in case.
			if notFoundErr == nil {
				notFoundErr = trace.NotFound("target resource not found")
			}
			// Mask access denied errors to prevent resource name oracles.
			return nil, trace.Wrap(notFoundErr)
		}
		return nil, trace.Wrap(noMFAAccessErr)
	}
	// If we reach here, the error from AccessChecker was
	// ErrSessionMFARequired.

	return &proto.IsMFARequiredResponse{Required: true}, nil
}

// mfaAuthChallenge constructs an MFAAuthenticateChallenge for all MFA devices
// registered by the user.
func (a *Server) mfaAuthChallenge(ctx context.Context, user string, u2fStorage u2f.AuthenticationStorage) (*proto.MFAAuthenticateChallenge, error) {
	// Check what kind of MFA is enabled.
	apref, err := a.GetAuthPreference()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var enableTOTP, enableU2F bool
	switch apref.GetSecondFactor() {
	case constants.SecondFactorOTP:
		enableTOTP, enableU2F = true, false
	case constants.SecondFactorU2F:
		enableTOTP, enableU2F = false, true
	default:
		// Other AuthPreference types don't restrict us to a single MFA type,
		// send all challenges.
		enableTOTP, enableU2F = true, true
	}
	var u2fPref *types.U2F
	if enableU2F {
		u2fPref, err = apref.GetU2F()
		if trace.IsNotFound(err) {
			// If U2F parameters were not set in the auth server config,
			// disable U2F challenges.
			enableU2F = false
		} else if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	devs, err := a.GetMFADevices(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	challenge := new(proto.MFAAuthenticateChallenge)
	var u2fDevs []*types.MFADevice
	for _, dev := range devs {
		switch dev.Device.(type) {
		case *types.MFADevice_Totp:
			if !enableTOTP {
				continue
			}
			challenge.TOTP = new(proto.TOTPChallenge)
		case *types.MFADevice_U2F:
			if !enableU2F {
				continue
			}
			u2fDevs = append(u2fDevs, dev)
		default:
			log.Warningf("Skipping MFA device of unknown type %T.", dev.Device)
		}
	}
	if len(u2fDevs) > 0 {
		chals, err := u2f.AuthenticateInit(ctx, u2f.AuthenticateInitParams{
			AppConfig:  *u2fPref,
			Devs:       u2fDevs,
			Storage:    u2fStorage,
			StorageKey: user,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, ch := range chals {
			challenge.U2F = append(challenge.U2F, &proto.U2FChallenge{
				Version:   ch.Version,
				KeyHandle: ch.KeyHandle,
				Challenge: ch.Challenge,
				AppID:     ch.AppID,
			})
		}
	}
	return challenge, nil
}

func (a *Server) validateMFAAuthResponse(ctx context.Context, user string, resp *proto.MFAAuthenticateResponse, u2fStorage u2f.AuthenticationStorage) (*types.MFADevice, error) {
	switch res := resp.Response.(type) {
	case *proto.MFAAuthenticateResponse_TOTP:
		return a.checkOTP(user, res.TOTP.Code)
	case *proto.MFAAuthenticateResponse_U2F:
		return a.checkU2F(ctx, user, u2f.AuthenticateChallengeResponse{
			KeyHandle:     res.U2F.KeyHandle,
			ClientData:    res.U2F.ClientData,
			SignatureData: res.U2F.Signature,
		}, u2fStorage)
	default:
		return nil, trace.BadParameter("unknown or missing MFAAuthenticateResponse type %T", resp.Response)
	}
}

func (a *Server) checkU2F(ctx context.Context, user string, res u2f.AuthenticateChallengeResponse, u2fStorage u2f.AuthenticationStorage) (*types.MFADevice, error) {
	devs, err := a.GetMFADevices(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, dev := range devs {
		u2fDev := dev.GetU2F()
		if u2fDev == nil {
			continue
		}

		// U2F passes key handles around base64-encoded, without padding.
		kh := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(u2fDev.KeyHandle)
		if kh != res.KeyHandle {
			continue
		}
		if err := u2f.AuthenticateVerify(ctx, u2f.AuthenticateVerifyParams{
			Dev:        dev,
			Resp:       res,
			Storage:    u2fStorage,
			StorageKey: user,
			Clock:      a.clock,
		}); err != nil {
			// Since key handles are unique, no need to check other devices.
			return nil, trace.AccessDenied("U2F response validation failed for device %q: %v", dev.GetName(), err)
		}
		return dev, nil
	}
	return nil, trace.AccessDenied("U2F response validation failed: no device matches the response")
}

// WithClock is a functional server option that sets the server's clock
func WithClock(clock clockwork.Clock) func(*Server) {
	return func(s *Server) {
		s.clock = clock
	}
}

// WithPrivateKey is a functional server option that sets the server's private key.
// Used in tests
func WithPrivateKey(key []byte) func(*Server) {
	return func(s *Server) {
		s.privateKey = key
	}
}

func (a *Server) upsertWebSession(ctx context.Context, user string, session services.WebSession) error {
	if err := a.WebSessions().Upsert(ctx, session); err != nil {
		return trace.Wrap(err)
	}
	token := types.NewWebToken(session.GetBearerTokenExpiryTime(), types.WebTokenSpecV3{
		User:  session.GetUser(),
		Token: session.GetBearerToken(),
	})
	if err := a.WebTokens().Upsert(ctx, token); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// authKeepAliver is a keep aliver using auth server directly
type authKeepAliver struct {
	sync.RWMutex
	a           *Server
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
			err := k.a.KeepAliveServer(k.ctx, keepAlive)
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

	// SessionTokenBytes is the number of bytes of a web or application session.
	SessionTokenBytes = 32
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
