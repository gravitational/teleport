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
	"bytes"
	"context"
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/oauth2"
	"github.com/coreos/go-oidc/oidc"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
	"github.com/prometheus/client_golang/prometheus"
	saml2 "github.com/russellhaering/gosaml2"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/auth/u2f"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

// ServerOption allows setting options as functional arguments to Server
type ServerOption func(*Server)

// NewServer creates and configures a new Server instance
func NewServer(cfg *InitConfig, opts ...ServerOption) (*Server, error) {
	err := utils.RegisterPrometheusCollectors(prometheusCollectors...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

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
		clusterConfig, err := local.NewClusterConfigurationService(cfg.Backend)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cfg.ClusterConfiguration = clusterConfig
	}
	if cfg.Restrictions == nil {
		cfg.Restrictions = local.NewRestrictionsService(cfg.Backend)
	}
	if cfg.Apps == nil {
		cfg.Apps = local.NewAppService(cfg.Backend)
	}
	if cfg.Databases == nil {
		cfg.Databases = local.NewDatabasesService(cfg.Backend)
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
	if cfg.WindowsDesktops == nil {
		cfg.WindowsDesktops = local.NewWindowsDesktopService(cfg.Backend)
	}
	if cfg.KeyStoreConfig.RSAKeyPairSource == nil {
		cfg.KeyStoreConfig.RSAKeyPairSource = cfg.Authority.GenerateKeyPair
	}
	if cfg.KeyStoreConfig.HostUUID == "" {
		cfg.KeyStoreConfig.HostUUID = cfg.HostUUID
	}

	limiter, err := limiter.NewConnectionsLimiter(limiter.Config{
		MaxConnections: defaults.LimiterMaxConcurrentSignatures,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keyStore, err := keystore.NewKeyStore(cfg.KeyStoreConfig)
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
			Restrictions:         cfg.Restrictions,
			Apps:                 cfg.Apps,
			Databases:            cfg.Databases,
			IAuditLog:            cfg.AuditLog,
			Events:               cfg.Events,
			WindowsDesktops:      cfg.WindowsDesktops,
		},
		keyStore: keyStore,
	}
	for _, o := range opts {
		o(&as)
	}
	if as.clock == nil {
		as.clock = clockwork.NewRealClock()
	}

	return &as, nil
}

type Services struct {
	services.Trust
	services.Presence
	services.Provisioner
	services.Identity
	services.Access
	services.DynamicAccessExt
	services.ClusterConfiguration
	services.Restrictions
	services.Apps
	services.Databases
	services.WindowsDesktops
	types.Events
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
	// UserLoginCount counts user logins
	UserLoginCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: teleport.MetricUserLoginCount,
			Help: "Number of times there was a user login",
		},
	)

	heartbeatsMissedByAuth = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: teleport.MetricHeartbeatsMissed,
			Help: "Number of hearbeats missed by auth server",
		},
	)

	prometheusCollectors = []prometheus.Collector{
		generateRequestsCount, generateThrottledRequestsCount,
		generateRequestsCurrent, generateRequestsLatencies, UserLoginCount, heartbeatsMissedByAuth,
	}
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
	cache Cache

	limiter *limiter.ConnectionsLimiter

	// Emitter is events emitter, used to submit discrete events
	emitter apievents.Emitter

	// streamer is events sessionstreamer, used to create continuous
	// session related streams
	streamer events.Streamer

	// keyStore is an interface for interacting with private keys in CAs which
	// may be backed by HSMs
	keyStore keystore.KeyStore

	// lockWatcher is a lock watcher, used to verify cert generation requests.
	lockWatcher *services.LockWatcher
}

// SetCache sets cache used by auth server
func (a *Server) SetCache(clt Cache) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.cache = clt
}

// GetCache returns cache used by auth server
func (a *Server) GetCache() Cache {
	a.lock.RLock()
	defer a.lock.RUnlock()
	if a.cache == nil {
		return &a.Services
	}
	return a.cache
}

// SetLockWatcher sets the lock watcher.
func (a *Server) SetLockWatcher(lockWatcher *services.LockWatcher) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.lockWatcher = lockWatcher
}

func (a *Server) checkLockInForce(mode constants.LockingMode, targets []types.LockTarget) error {
	a.lock.RLock()
	defer a.lock.RUnlock()
	if a.lockWatcher == nil {
		return trace.BadParameter("lockWatcher is not set")
	}
	return a.lockWatcher.CheckLockInForce(mode, targets...)
}

// runPeriodicOperations runs some periodic bookkeeping operations
// performed by auth server
func (a *Server) runPeriodicOperations() {
	ctx := context.TODO()
	// run periodic functions with a semi-random period
	// to avoid contention on the database in case if there are multiple
	// auth servers running - so they don't compete trying
	// to update the same resources.
	r := rand.New(rand.NewSource(a.GetClock().Now().UnixNano()))
	period := defaults.HighResPollingPeriod + time.Duration(r.Intn(int(defaults.HighResPollingPeriod/time.Second)))*time.Second
	log.Debugf("Ticking with period: %v.", period)
	a.lock.RLock()
	ticker := a.clock.NewTicker(period)
	a.lock.RUnlock()
	// Create a ticker with jitter
	heartbeatCheckTicker := interval.New(interval.Config{
		Duration: apidefaults.ServerKeepAliveTTL * 2,
		Jitter:   utils.NewSeventhJitter(),
	})
	missedKeepAliveCount := 0
	defer ticker.Stop()
	defer heartbeatCheckTicker.Stop()
	for {
		select {
		case <-a.closeCtx.Done():
			return
		case <-ticker.Chan():
			err := a.autoRotateCertAuthorities()
			if err != nil {
				if trace.IsCompareFailed(err) {
					log.Debugf("Cert authority has been updated concurrently: %v.", err)
				} else {
					log.Errorf("Failed to perform cert rotation check: %v.", err)
				}
			}
		case <-heartbeatCheckTicker.Next():
			nodes, err := a.GetNodes(ctx, apidefaults.Namespace)
			if err != nil {
				log.Errorf("Failed to load nodes for heartbeat metric calculation: %v", err)
			}
			for _, node := range nodes {
				if services.NodeHasMissedKeepAlives(node) {
					missedKeepAliveCount++
				}
			}
			// Update prometheus gauge
			heartbeatsMissedByAuth.Set(float64(missedKeepAliveCount))
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

// GetAuthPreference gets AuthPreference from the cache.
func (a *Server) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	return a.GetCache().GetAuthPreference(ctx)
}

// GetClusterAuditConfig gets ClusterAuditConfig from the cache.
func (a *Server) GetClusterAuditConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterAuditConfig, error) {
	return a.GetCache().GetClusterAuditConfig(ctx, opts...)
}

// GetClusterNetworkingConfig gets ClusterNetworkingConfig from the cache.
func (a *Server) GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error) {
	return a.GetCache().GetClusterNetworkingConfig(ctx, opts...)
}

// GetSessionRecordingConfig gets SessionRecordingConfig from the cache.
func (a *Server) GetSessionRecordingConfig(ctx context.Context, opts ...services.MarshalOption) (types.SessionRecordingConfig, error) {
	return a.GetCache().GetSessionRecordingConfig(ctx, opts...)
}

// GetClusterName returns the domain name that identifies this authority server.
// Also known as "cluster name"
func (a *Server) GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error) {
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

// LocalCAResponse contains the concatenated PEM-encoded TLS certs for the local
// cluster's Host CA
type LocalCAResponse struct {
	// TLSCA is a PEM-encoded TLS certificate authority.
	TLSCA []byte `json:"tls_ca"`
}

// GetClusterCACert returns the PEM-encoded TLS certs for the local cluster. If
// the cluster has multiple TLS certs, they will all be concatenated.
func (a *Server) GetClusterCACert() (*LocalCAResponse, error) {
	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Extract the TLS CA for this cluster.
	hostCA, err := a.GetCache().GetCertAuthority(types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName.GetClusterName(),
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certs := services.GetTLSCerts(hostCA)
	if len(certs) < 1 {
		return nil, trace.NotFound("no tls certs found in host CA")
	}
	allCerts := bytes.Join(certs, []byte("\n"))

	return &LocalCAResponse{
		TLSCA: allCerts,
	}, nil
}

// GenerateHostCert uses the private key of the CA to sign the public key of the host
// (along with meta data like host ID, node name, roles, and ttl) to generate a host certificate.
func (a *Server) GenerateHostCert(hostPublicKey []byte, hostID, nodeName string, principals []string, clusterName string, role types.SystemRole, ttl time.Duration) ([]byte, error) {
	domainName, err := a.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// get the certificate authority that will be signing the public key of the host
	ca, err := a.Trust.GetCertAuthority(types.CertAuthID{
		Type:       types.HostCA,
		DomainName: domainName,
	}, true)
	if err != nil {
		return nil, trace.BadParameter("failed to load host CA for %q: %v", domainName, err)
	}

	caSigner, err := a.keyStore.GetSSHSigner(ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// create and sign!
	return a.generateHostCert(services.HostCertParams{
		CASigner:      caSigner,
		CASigningAlg:  sshutils.GetSigningAlgName(ca),
		PublicHostKey: hostPublicKey,
		HostID:        hostID,
		NodeName:      nodeName,
		Principals:    principals,
		ClusterName:   clusterName,
		Role:          role,
		TTL:           ttl,
	})
}

func (a *Server) generateHostCert(p services.HostCertParams) ([]byte, error) {
	authPref, err := a.GetAuthPreference(context.TODO())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if p.Role == types.RoleNode {
		if lockErr := a.checkLockInForce(authPref.GetLockingMode(),
			[]types.LockTarget{{Node: p.HostID}, {Node: HostFQDN(p.HostID, p.ClusterName)}},
		); lockErr != nil {
			return nil, trace.Wrap(lockErr)
		}
	}
	return a.Authority.GenerateHostCert(p)
}

// GetKeyStore returns the KeyStore used by the auth server
func (a *Server) GetKeyStore() keystore.KeyStore {
	return a.keyStore
}

type certRequest struct {
	// user is a user to generate certificate for
	user types.User
	// impersonator is a user who generates the certificate,
	// is set when different from the user in the certificate
	impersonator string
	// checker is used to perform RBAC checks.
	checker services.AccessChecker
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
	activeRequests services.RequestIDs
	// appSessionID is the session ID of the application session.
	appSessionID string
	// appPublicAddr is the public address of the application.
	appPublicAddr string
	// appClusterName is the name of the cluster this application is in.
	appClusterName string
	// appName is the name of the application to generate cert for.
	appName string
	// awsRoleARN is the role ARN to generate certificate for.
	awsRoleARN string
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

// check verifies the cert request is valid.
func (r *certRequest) check() error {
	if r.user == nil {
		return trace.BadParameter("missing parameter user")
	}
	if r.checker == nil {
		return trace.BadParameter("missing parameter checker")
	}

	// When generating certificate for MongoDB access, database username must
	// be encoded into it. This is required to be able to tell which database
	// user to authenticate the connection as.
	if r.dbProtocol == defaults.ProtocolMongoDB {
		if r.dbUser == "" {
			return trace.BadParameter("must provide database user name to generate certificate for database %q", r.dbService)
		}
	}
	return nil
}

type certRequestOption func(*certRequest)

func certRequestMFAVerified(mfaID string) certRequestOption {
	return func(r *certRequest) { r.mfaVerified = mfaID }
}

func certRequestClientIP(ip string) certRequestOption {
	return func(r *certRequest) { r.clientIP = ip }
}

// GenerateUserTestCerts is used to generate user certificate, used internally for tests
func (a *Server) GenerateUserTestCerts(key []byte, username string, ttl time.Duration, compatibility, routeToCluster string) ([]byte, []byte, error) {
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
		ttl:            ttl,
		compatibility:  compatibility,
		publicKey:      key,
		routeToCluster: routeToCluster,
		checker:        checker,
		traits:         user.GetTraits(),
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return certs.SSH, certs.TLS, nil
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
	// AWSRoleARN is optional AWS role ARN a user wants to assume to encode.
	AWSRoleARN string
}

// GenerateUserAppTestCert generates an application specific certificate, used
// internally for tests.
func (a *Server) GenerateUserAppTestCert(req AppTestCertRequest) ([]byte, error) {
	user, err := a.Identity.GetUser(req.Username, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := services.FetchRoles(user.GetRoles(), a.Access, user.GetTraits())
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
		awsRoleARN:     req.AWSRoleARN,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return certs.TLS, nil
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
	checker, err := services.FetchRoles(user.GetRoles(), a.Access, user.GetTraits())
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
	return certs.TLS, nil
}

// generateUserCert generates user certificates
func (a *Server) generateUserCert(req certRequest) (*proto.Certs, error) {
	err := req.check()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Reject the cert request if there is a matching lock in force.
	authPref, err := a.GetAuthPreference(context.TODO())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if lockErr := a.checkLockInForce(req.checker.LockingMode(authPref.GetLockingMode()), append(
		services.RolesToLockTargets(req.checker.RoleNames()),
		types.LockTarget{User: req.user.GetName()},
		types.LockTarget{MFADevice: req.mfaVerified},
	)); lockErr != nil {
		return nil, trace.Wrap(lockErr)
	}

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
	if req.routeToCluster == "" {
		req.routeToCluster = clusterName
	}
	if req.routeToCluster != clusterName {
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

	ca, err := a.Trust.GetCertAuthority(types.CertAuthID{
		Type:       types.UserCA,
		DomainName: clusterName,
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	caSigner, err := a.keyStore.GetSSHSigner(ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	params := services.UserCertParams{
		CASigner:              caSigner,
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
	if req.routeToCluster == clusterName {
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

	// See which AWS role ARNs this user is allowed to assume.
	roleARNs, err := req.checker.CheckAWSRoleARNs(sessionTTL, req.overrideRoleTTL)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	// generate TLS certificate
	cert, signer, err := a.keyStore.GetTLSCertAndSigner(ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsAuthority, err := tlsca.FromCertAndSigner(cert, signer)
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
			AWSRoleARN:  req.awsRoleARN,
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
		AWSRoleARNs:   roleARNs,
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
	return &proto.Certs{
		SSH:        sshCert,
		TLS:        tlsCert,
		TLSCACerts: services.GetTLSCerts(ca),
		SSHCACerts: services.GetSSHCheckingKeys(ca),
	}, nil
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
	if status.IsLocked {
		if status.RecoveryAttemptLockExpires.After(a.clock.Now().UTC()) {
			return trace.AccessDenied("%v exceeds %v failed account recovery attempts, locked until %v",
				user.GetName(), defaults.MaxAccountRecoveryAttempts, apiutils.HumanTimeFormat(status.RecoveryAttemptLockExpires))
		}
		if status.LockExpires.After(a.clock.Now().UTC()) {
			return trace.AccessDenied("%v exceeds %v failed login attempts, locked until %v",
				user.GetName(), defaults.MaxLoginAttempts, apiutils.HumanTimeFormat(status.LockExpires))
		}
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
	attempt := services.LoginAttempt{Time: a.clock.Now().UTC(), Success: false}
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
	if !services.LastFailed(defaults.MaxLoginAttempts, loginAttempts) {
		log.Debugf("%v user has less than %v failed login attempts", username, defaults.MaxLoginAttempts)
		return trace.Wrap(fnErr)
	}
	lockUntil := a.clock.Now().UTC().Add(defaults.AccountLockInterval)
	message := fmt.Sprintf("%v exceeds %v failed login attempts, locked until %v",
		username, defaults.MaxLoginAttempts, apiutils.HumanTimeFormat(status.LockExpires))
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
func (a *Server) PreAuthenticatedSignIn(user string, identity tlsca.Identity) (types.WebSession, error) {
	roles, traits, err := services.ExtractFromIdentity(a, identity)
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

// MFAAuthenticateChallenge is an MFA authentication challenge sent on user
// login / authentication ceremonies.
//
// TODO(kimlisa): Move this to lib/client/mfa.go, after deleting the auth HTTP endpoint
// "/u2f/users/sign" and its friends from lib/auth/apiserver.go (replaced by grpc CreateAuthenticateChallenge).
// After the endpoint removal, this struct is only used in the web directory.
type MFAAuthenticateChallenge struct {
	// Before 6.0 teleport would only send 1 U2F challenge. Embed the old
	// challenge for compatibility with older clients. All new clients should
	// ignore this and read Challenges instead.
	*u2f.AuthenticateChallenge

	// U2FChallenges is a list of U2F challenges, one for each registered
	// device.
	U2FChallenges []u2f.AuthenticateChallenge `json:"u2f_challenges"`
	// WebauthnChallenge contains a WebAuthn credential assertion used for
	// login/authentication ceremonies.
	// An assertion already contains, among other information, a list of allowed
	// credentials (one for each known U2F or Webauthn device), so there is no
	// need to send multiple assertions.
	WebauthnChallenge *wanlib.CredentialAssertion `json:"webauthn_challenge"`
	// TOTPChallenge specifies whether TOTP is supported for this user.
	TOTPChallenge bool `json:"totp_challenge"`
}

// CreateAuthenticateChallenge implements AuthService.CreateAuthenticateChallenge.
func (a *Server) CreateAuthenticateChallenge(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
	var username string

	switch req.GetRequest().(type) {
	case *proto.CreateAuthenticateChallengeRequest_UserCredentials:
		username = req.GetUserCredentials().GetUsername()

		if err := a.WithUserLock(username, func() error {
			return a.checkPasswordWOToken(username, req.GetUserCredentials().GetPassword())
		}); err != nil {
			log.Error(trace.DebugReport(err))
			return nil, trace.AccessDenied("invalid password or username")
		}

	case *proto.CreateAuthenticateChallengeRequest_RecoveryStartTokenID:
		token, err := a.GetUserToken(ctx, req.GetRecoveryStartTokenID())
		if err != nil {
			log.Error(trace.DebugReport(err))
			return nil, trace.AccessDenied("invalid token")
		}

		if err := a.verifyUserToken(token, UserTokenTypeRecoveryStart); err != nil {
			return nil, trace.Wrap(err)
		}

		username = token.GetUser()

	default:
		var err error
		username, err = GetClientUsername(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	challenges, err := a.mfaAuthChallenge(ctx, username, a.Identity /* u2f storage */)
	if err != nil {
		log.Error(trace.DebugReport(err))
		return nil, trace.AccessDenied("unable to create MFA challenges")
	}
	return challenges, nil
}

// CreateRegisterChallenge implements AuthService.CreateRegisterChallenge.
func (a *Server) CreateRegisterChallenge(ctx context.Context, req *proto.CreateRegisterChallengeRequest) (*proto.MFARegisterChallenge, error) {
	token, err := a.GetUserToken(ctx, req.GetTokenID())
	if err != nil {
		log.Error(trace.DebugReport(err))
		return nil, trace.AccessDenied("invalid token")
	}

	allowedTokenTypes := []string{
		UserTokenTypePrivilege,
		UserTokenTypePrivilegeException,
		UserTokenTypeResetPassword,
		UserTokenTypeResetPasswordInvite,
		UserTokenTypeRecoveryApproved,
	}

	if err := a.verifyUserToken(token, allowedTokenTypes...); err != nil {
		return nil, trace.AccessDenied("invalid token")
	}

	regChal, err := a.createRegisterChallenge(ctx, &newRegisterChallengeRequest{
		username:   token.GetUser(),
		token:      token,
		deviceType: req.GetDeviceType(),
	})

	return regChal, trace.Wrap(err)
}

type newRegisterChallengeRequest struct {
	username   string
	deviceType proto.DeviceType
	// token is a user token resource.
	// It is used as following:
	//  - TOTP:
	//    - create a UserTokenSecrets resource
	//    - store by token's ID using Server's IdentityService.
	//  - U2F:
	//    - store U2F challenge by the token's ID
	//    - store by token's ID using Server's IdentityService.
	// This field can be empty to use storage overrides.
	token types.UserToken
	// u2fStorageOverride is an optional RegistrationStorage override to be used
	// to store the U2F challenge.
	u2fStorageOverride u2f.RegistrationStorage

	// webIdentityOverride is an optional RegistrationIdentity override to be used
	// to store webauthn challenge. A common override is decorating the regular
	// Identity with an in-memory SessionData storage.
	// Defaults to the Server's IdentityService.
	webIdentityOverride wanlib.RegistrationIdentity
}

func (a *Server) createRegisterChallenge(ctx context.Context, req *newRegisterChallengeRequest) (*proto.MFARegisterChallenge, error) {
	switch req.deviceType {
	case proto.DeviceType_DEVICE_TYPE_TOTP:
		otpKey, otpOpts, err := a.newTOTPKey(req.username)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		challenge := &proto.TOTPRegisterChallenge{
			Secret:        otpKey.Secret(),
			Issuer:        otpKey.Issuer(),
			PeriodSeconds: uint32(otpOpts.Period),
			Algorithm:     otpOpts.Algorithm.String(),
			Digits:        uint32(otpOpts.Digits.Length()),
			Account:       otpKey.AccountName(),
		}

		if req.token != nil {
			secrets, err := a.createTOTPUserTokenSecrets(ctx, req.token, otpKey)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			challenge.QRCode = secrets.GetQRCode()
		}

		return &proto.MFARegisterChallenge{Request: &proto.MFARegisterChallenge_TOTP{TOTP: challenge}}, nil

	case proto.DeviceType_DEVICE_TYPE_U2F:
		cap, err := a.GetAuthPreference(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		u2fConfig, err := cap.GetU2F()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		storageKey := req.username
		if req.token != nil {
			storageKey = req.token.GetName()
		}

		storage := req.u2fStorageOverride
		if storage == nil || req.token != nil {
			storage = a.Identity
		}

		regChallenge, err := u2f.RegisterInit(u2f.RegisterInitParams{
			StorageKey: storageKey,
			AppConfig:  *u2fConfig,
			Storage:    storage,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &proto.MFARegisterChallenge{Request: &proto.MFARegisterChallenge_U2F{
			U2F: &proto.U2FRegisterChallenge{
				Challenge: regChallenge.Challenge,
				AppID:     regChallenge.AppID,
				Version:   regChallenge.Version,
			},
		}}, nil

	case proto.DeviceType_DEVICE_TYPE_WEBAUTHN:
		cap, err := a.GetAuthPreference(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		webConfig, err := cap.GetWebauthn()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		identity := req.webIdentityOverride
		if identity == nil {
			identity = a.Identity
		}

		webRegistration := &wanlib.RegistrationFlow{
			Webauthn: webConfig,
			Identity: identity,
		}

		credentialCreation, err := webRegistration.Begin(ctx, req.username)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &proto.MFARegisterChallenge{Request: &proto.MFARegisterChallenge_Webauthn{
			Webauthn: wanlib.CredentialCreationToProto(credentialCreation),
		}}, nil

	default:
		return nil, trace.BadParameter("MFA device type %q unsupported", req.deviceType.String())
	}
}

// GetMFADevices returns all mfa devices for the user defined in the token or the user defined in context.
func (a *Server) GetMFADevices(ctx context.Context, req *proto.GetMFADevicesRequest) (*proto.GetMFADevicesResponse, error) {
	var username string

	if req.GetRecoveryApprovedTokenID() != "" {
		token, err := a.GetUserToken(ctx, req.GetRecoveryApprovedTokenID())
		if err != nil {
			log.Error(trace.DebugReport(err))
			return nil, trace.AccessDenied("invalid token")
		}

		if err := a.verifyUserToken(token, UserTokenTypeRecoveryApproved); err != nil {
			return nil, trace.Wrap(err)
		}

		username = token.GetUser()
	}

	if username == "" {
		var err error
		username, err = GetClientUsername(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	devs, err := a.Identity.GetMFADevices(ctx, username, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.GetMFADevicesResponse{
		Devices: devs,
	}, nil
}

// DeleteMFADeviceSync implements AuthService.DeleteMFADeviceSync.
func (a *Server) DeleteMFADeviceSync(ctx context.Context, req *proto.DeleteMFADeviceSyncRequest) error {
	token, err := a.GetUserToken(ctx, req.GetTokenID())
	if err != nil {
		log.Error(trace.DebugReport(err))
		return trace.AccessDenied("invalid token")
	}

	if err := a.verifyUserToken(token, UserTokenTypeRecoveryApproved, UserTokenTypePrivilege); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(a.deleteMFADeviceSafely(ctx, token.GetUser(), req.GetDeviceName()))
}

// deleteMFADeviceSafely deletes the user's mfa device while preventing users from deleting their last device
// for clusters that require second factors, which prevents users from being locked out of their account.
func (a *Server) deleteMFADeviceSafely(ctx context.Context, user, deviceName string) error {
	devs, err := a.Identity.GetMFADevices(ctx, user, true)
	if err != nil {
		return trace.Wrap(err)
	}

	authPref, err := a.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	kindToSF := map[string]constants.SecondFactorType{
		fmt.Sprintf("%T", &types.MFADevice_Totp{}):     constants.SecondFactorOTP,
		fmt.Sprintf("%T", &types.MFADevice_U2F{}):      constants.SecondFactorU2F,
		fmt.Sprintf("%T", &types.MFADevice_Webauthn{}): constants.SecondFactorWebauthn,
	}
	sfToCount := make(map[constants.SecondFactorType]int)
	var knownDevices int
	var deviceToDelete *types.MFADevice

	// Find the device to delete and count devices.
	for _, d := range devs {
		// Match device by name or ID.
		if d.GetName() == deviceName || d.Id == deviceName {
			deviceToDelete = d
		}

		sf, ok := kindToSF[fmt.Sprintf("%T", d.Device)]
		switch {
		case !ok && d == deviceToDelete:
			return trace.NotImplemented("cannot delete device of type %T", d.Device)
		case !ok:
			log.Warnf("Ignoring unknown device with type %T in deletion.", d.Device)
			continue
		}

		sfToCount[sf]++
		knownDevices++
	}
	if deviceToDelete == nil {
		return trace.NotFound("MFA device %q does not exist", deviceName)
	}

	// Prevent users from deleting their last device for clusters that require second factors.
	const minDevices = 2
	switch sf := authPref.GetSecondFactor(); sf {
	case constants.SecondFactorOff, constants.SecondFactorOptional: // MFA is not required, allow deletion
	case constants.SecondFactorOn:
		if knownDevices < minDevices {
			return trace.BadParameter(
				"cannot delete the last MFA device for this user; add a replacement device first to avoid getting locked out")
		}
	case constants.SecondFactorOTP, constants.SecondFactorU2F, constants.SecondFactorWebauthn:
		if sfToCount[sf] < minDevices {
			return trace.BadParameter(
				"cannot delete the last %s device for this user; add a replacement device first to avoid getting locked out", sf)
		}
	default:
		return trace.BadParameter("unexpected second factor type: %s", sf)
	}

	if err := a.DeleteMFADevice(ctx, user, deviceToDelete.Id); err != nil {
		return trace.Wrap(err)
	}

	// Emit deleted event.
	clusterName, err := a.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.MFADeviceDelete{
		Metadata: apievents.Metadata{
			Type:        events.MFADeviceDeleteEvent,
			Code:        events.MFADeviceDeleteEventCode,
			ClusterName: clusterName.GetClusterName(),
		},
		UserMetadata: apievents.UserMetadata{
			User: user,
		},
		MFADeviceMetadata: mfaDeviceEventMetadata(deviceToDelete),
	}); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// AddMFADeviceSync implements AuthService.AddMFADeviceSync.
func (a *Server) AddMFADeviceSync(ctx context.Context, req *proto.AddMFADeviceSyncRequest) (*proto.AddMFADeviceSyncResponse, error) {
	privilegeToken, err := a.GetUserToken(ctx, req.GetPrivilegeTokenID())
	if err != nil {
		log.Error(trace.DebugReport(err))
		return nil, trace.AccessDenied("invalid token")
	}

	if err := a.verifyUserToken(privilegeToken, UserTokenTypePrivilege, UserTokenTypePrivilegeException); err != nil {
		return nil, trace.Wrap(err)
	}

	dev, err := a.verifyMFARespAndAddDevice(ctx, req.GetNewMFAResponse(), &newMFADeviceFields{
		username:      privilegeToken.GetUser(),
		newDeviceName: req.GetNewDeviceName(),
		tokenID:       privilegeToken.GetName(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.AddMFADeviceSyncResponse{Device: dev}, nil
}

type newMFADeviceFields struct {
	username      string
	newDeviceName string
	// tokenID is the ID of a reset/invite/recovery token.
	// It is used as following:
	//  - TOTP:
	//    - look up TOTP secret stored by token ID
	//  - U2F:
	//    - look up U2F challenge stored by token ID
	//    - use default u2f storage which is our services.Identity
	// This field can be empty to indicate that fields
	// "totpSecret" and "u2fStorage" is to be used instead.
	tokenID string
	// totpSecret is a secret shared by client and server to generate totp codes.
	// Field can be empty to get secret by "tokenID".
	totpSecret string
	// u2fStorage is the storage used to hold the u2f challenge.
	// Field can be empty to use default services.Identity storage.
	u2fStorage u2f.RegistrationStorage

	// webIdentityOverride is an optional RegistrationIdentity override to be used
	// for device registration. A common override is decorating the regular
	// Identity with an in-memory SessionData storage.
	// Defaults to the Server's IdentityService.
	webIdentityOverride wanlib.RegistrationIdentity
}

// verifyMFARespAndAddDevice validates MFA register response and on success adds the new MFA device.
func (a *Server) verifyMFARespAndAddDevice(ctx context.Context, regResp *proto.MFARegisterResponse, req *newMFADeviceFields) (*types.MFADevice, error) {
	cap, err := a.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cap.GetSecondFactor() == constants.SecondFactorOff {
		return nil, trace.BadParameter("second factor disabled by cluster configuration")
	}

	var dev *types.MFADevice
	switch regResp.GetResponse().(type) {
	case *proto.MFARegisterResponse_TOTP:
		dev, err = a.registerTOTPDevice(ctx, regResp, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case *proto.MFARegisterResponse_U2F:
		dev, err = a.registerU2FDevice(ctx, regResp, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case *proto.MFARegisterResponse_Webauthn:
		dev, err = a.registerWebauthnDevice(ctx, regResp, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		return nil, trace.BadParameter("MFARegisterResponse is an unknown response type %T", regResp.Response)
	}

	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.MFADeviceAdd{
		Metadata: apievents.Metadata{
			Type:        events.MFADeviceAddEvent,
			Code:        events.MFADeviceAddEventCode,
			ClusterName: clusterName.GetClusterName(),
		},
		UserMetadata: apievents.UserMetadata{
			User: req.username,
		},
		MFADeviceMetadata: mfaDeviceEventMetadata(dev),
	}); err != nil {
		log.WithError(err).Warn("Failed to emit add mfa device event.")
	}

	return dev, nil
}

func (a *Server) registerTOTPDevice(ctx context.Context, regResp *proto.MFARegisterResponse, req *newMFADeviceFields) (*types.MFADevice, error) {
	cap, err := a.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cap.IsSecondFactorTOTPAllowed() {
		return nil, trace.BadParameter("second factor TOTP not allowed by cluster")
	}

	var secret string
	switch {
	case req.tokenID != "":
		secrets, err := a.Identity.GetUserTokenSecrets(ctx, req.tokenID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		secret = secrets.GetOTPKey()
	case req.totpSecret != "":
		secret = req.totpSecret
	default:
		return nil, trace.BadParameter("missing TOTP secret")
	}

	dev, err := services.NewTOTPDevice(req.newDeviceName, secret, a.clock.Now())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.checkTOTP(ctx, req.username, regResp.GetTOTP().GetCode(), dev); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.UpsertMFADevice(ctx, req.username, dev); err != nil {
		return nil, trace.Wrap(err)
	}
	return dev, nil
}

func (a *Server) registerU2FDevice(ctx context.Context, regResp *proto.MFARegisterResponse, req *newMFADeviceFields) (*types.MFADevice, error) {
	cap, err := a.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cap.IsSecondFactorU2FAllowed() {
		return nil, trace.BadParameter("second factor U2F not allowed by cluster")
	}

	u2fConfig, err := cap.GetU2F()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var storageKey string
	var storage u2f.RegistrationStorage
	switch {
	case req.tokenID != "":
		storage = a.Identity
		storageKey = req.tokenID
	case req.u2fStorage != nil:
		storage = req.u2fStorage
		storageKey = req.username
	default:
		return nil, trace.BadParameter("missing U2F storage")
	}

	// u2f.RegisterVerify will upsert the new device internally.
	dev, err := u2f.RegisterVerify(ctx, u2f.RegisterVerifyParams{
		DevName: req.newDeviceName,
		Resp: u2f.RegisterChallengeResponse{
			RegistrationData: regResp.GetU2F().GetRegistrationData(),
			ClientData:       regResp.GetU2F().GetClientData(),
		},
		RegistrationStorageKey: req.username,
		ChallengeStorageKey:    storageKey,
		Storage:                storage,
		Clock:                  a.GetClock(),
		AttestationCAs:         u2fConfig.DeviceAttestationCAs,
	})
	return dev, trace.Wrap(err)
}

func (a *Server) registerWebauthnDevice(ctx context.Context, regResp *proto.MFARegisterResponse, req *newMFADeviceFields) (*types.MFADevice, error) {
	cap, err := a.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cap.IsSecondFactorWebauthnAllowed() {
		return nil, trace.BadParameter("second factor webauthn not allowed by cluster")
	}

	webConfig, err := cap.GetWebauthn()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity := req.webIdentityOverride // Override Identity, if supplied.
	if identity == nil {
		identity = a.Identity
	}
	webRegistration := &wanlib.RegistrationFlow{
		Webauthn: webConfig,
		Identity: identity,
	}
	// Finish upserts the device on success.
	dev, err := webRegistration.Finish(
		ctx, req.username, req.newDeviceName, wanlib.CredentialCreationResponseFromProto(regResp.GetWebauthn()))
	return dev, trace.Wrap(err)
}

func (a *Server) CheckU2FSignResponse(ctx context.Context, user string, response *u2f.AuthenticateChallengeResponse) (*types.MFADevice, error) {
	// before trying to register a user, see U2F is actually setup on the backend
	cap, err := a.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = cap.GetU2F()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return a.checkU2F(ctx, user, *response, a.Identity)
}

// ExtendWebSession creates a new web session for a user based on a valid previous (current) session.
//
// If there is an approved access request, additional roles are appended to the roles that were
// extracted from identity. The new session expiration time will not exceed the expiration time
// of the previous session.
//
// If there is a switchback request, the roles will switchback to user's default roles and
// the expiration time is derived from users recently logged in time.
func (a *Server) ExtendWebSession(req WebSessionReq, identity tlsca.Identity) (types.WebSession, error) {
	prevSession, err := a.GetWebSession(context.TODO(), types.GetWebSessionRequest{
		User:      req.User,
		SessionID: req.PrevSessionID,
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

	roles, traits, err := services.ExtractFromIdentity(a, identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.AccessRequestID != "" {
		newRoles, requestExpiry, err := a.getRolesAndExpiryFromAccessRequest(req.User, req.AccessRequestID)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		roles = append(roles, newRoles...)
		roles = apiutils.Deduplicate(roles)

		// Let session expire with the shortest expiry time.
		if expiresAt.After(requestExpiry) {
			expiresAt = requestExpiry
		}
	}

	if req.Switchback {
		if prevSession.GetLoginTime().IsZero() {
			return nil, trace.BadParameter("Unable to switchback, log in time was not recorded.")
		}

		// Get default/static roles.
		user, err := a.GetUser(req.User, false)
		if err != nil {
			return nil, trace.Wrap(err, "failed to switchback")
		}

		// Calculate expiry time.
		roleSet, err := services.FetchRoles(user.GetRoles(), a.Access, user.GetTraits())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		sessionTTL := roleSet.AdjustSessionTTL(apidefaults.CertDuration)

		// Set default roles and expiration.
		expiresAt = prevSession.GetLoginTime().UTC().Add(sessionTTL)
		roles = user.GetRoles()
	}

	sessionTTL := utils.ToTTL(a.clock, expiresAt)
	sess, err := a.NewWebSession(types.NewWebSessionRequest{
		User:       req.User,
		Roles:      roles,
		Traits:     traits,
		SessionTTL: sessionTTL,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Keep preserving the login time.
	sess.SetLoginTime(prevSession.GetLoginTime())

	if err := a.upsertWebSession(context.TODO(), req.User, sess); err != nil {
		return nil, trace.Wrap(err)
	}

	return sess, nil
}

func (a *Server) getRolesAndExpiryFromAccessRequest(user, accessRequestID string) ([]string, time.Time, error) {
	reqFilter := types.AccessRequestFilter{
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

	if err := services.ValidateAccessRequestForUser(a, req); err != nil {
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
func (a *Server) CreateWebSession(user string) (types.WebSession, error) {
	u, err := a.GetUser(user, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess, err := a.NewWebSession(types.NewWebSessionRequest{
		User:      user,
		Roles:     u.GetRoles(),
		Traits:    u.GetTraits(),
		LoginTime: a.clock.Now().UTC(),
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
	Roles types.SystemRoles `json:"roles"`
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
	token, err := types.NewProvisionToken(req.Token, req.Roles, a.clock.Now().UTC().Add(req.TTL))
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
		if role == types.RoleTrustedCluster {
			if err := a.emitter.EmitAuditEvent(ctx, &apievents.TrustedClusterTokenCreate{
				Metadata: apievents.Metadata{
					Type: events.TrustedClusterTokenCreateEvent,
					Code: events.TrustedClusterTokenCreateCode,
				},
				UserMetadata: apievents.UserMetadata{
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

// GenerateHostCerts generates new host certificates (signed
// by the host certificate authority) for a node.
func (a *Server) GenerateHostCerts(ctx context.Context, req *proto.HostCertsRequest) (*proto.Certs, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.Role.Check(); err != nil {
		return nil, err
	}

	if err := a.limiter.AcquireConnection(req.Role.String()); err != nil {
		generateThrottledRequestsCount.Inc()
		log.Debugf("Node %q [%v] is rate limited: %v.", req.NodeName, req.HostID, req.Role)
		return nil, trace.Wrap(err)
	}
	defer a.limiter.ReleaseConnection(req.Role.String())

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
	if apiutils.SliceContainsStr(req.AdditionalPrincipals, defaults.AnyAddress) {
		remoteHost, err := utils.Host(req.RemoteAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		req.AdditionalPrincipals = utils.ReplaceInSlice(
			req.AdditionalPrincipals,
			defaults.AnyAddress,
			remoteHost)
	}

	if _, _, _, _, err := ssh.ParseAuthorizedKey(req.PublicSSHKey); err != nil {
		return nil, trace.BadParameter("failed to parse SSH public key")
	}
	cryptoPubKey, err := tlsca.ParsePublicKeyPEM(req.PublicTLSKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// get the certificate authority that will be signing the public key of the host,
	client := a.GetCache()
	if req.NoCache {
		client = a.Services
	}
	ca, err := client.GetCertAuthority(types.CertAuthID{
		Type:       types.HostCA,
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
		ca, err = a.GetCertAuthority(types.CertAuthID{
			Type:       types.HostCA,
			DomainName: clusterName.GetClusterName(),
		}, true)
		if err != nil {
			return nil, trace.BadParameter("failed to load host CA for %q: %v", clusterName.GetClusterName(), err)
		}
		if !req.Rotation.Matches(ca.GetRotation()) {
			return nil, trace.BadParameter(""+
				"the client expected state is out of sync, server rotation state: %v, "+
				"client rotation state: %v, re-register the client from scratch to fix the issue.",
				ca.GetRotation(), req.Rotation)
		}
	}

	isAdminRole := req.Role == types.RoleAdmin

	cert, signer, err := a.keyStore.GetTLSCertAndSigner(ca)
	if trace.IsNotFound(err) && isAdminRole {
		// If there is no local TLS signer found in the host CA ActiveKeys, this
		// auth server may have a newly configured HSM and has only populated
		// local keys in the AdditionalTrustedKeys until the next CA rotation.
		// This is the only case where we should be able to get a signer from
		// AdditionalTrustedKeys but not ActiveKeys.
		cert, signer, err = a.keyStore.GetAdditionalTrustedTLSCertAndSigner(ca)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsAuthority, err := tlsca.FromCertAndSigner(cert, signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	caSigner, err := a.keyStore.GetSSHSigner(ca)
	if trace.IsNotFound(err) && isAdminRole {
		// If there is no local SSH signer found in the host CA ActiveKeys, this
		// auth server may have a newly configured HSM and has only populated
		// local keys in the AdditionalTrustedKeys until the next CA rotation.
		// This is the only case where we should be able to get a signer from
		// AdditionalTrustedKeys but not ActiveKeys.
		caSigner, err = a.keyStore.GetAdditionalTrustedSSHSigner(ca)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// generate host SSH certificate
	hostSSHCert, err := a.generateHostCert(services.HostCertParams{
		CASigner:      caSigner,
		CASigningAlg:  sshutils.GetSigningAlgName(ca),
		PublicHostKey: req.PublicSSHKey,
		HostID:        req.HostID,
		NodeName:      req.NodeName,
		ClusterName:   clusterName.GetClusterName(),
		Role:          req.Role,
		Principals:    req.AdditionalPrincipals,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// generate host TLS certificate
	identity := tlsca.Identity{
		Username:        HostFQDN(req.HostID, clusterName.GetClusterName()),
		Groups:          []string{req.Role.String()},
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
	if (types.SystemRoles{req.Role}).IncludeAny(types.RoleAuth, types.RoleAdmin, types.RoleProxy, types.RoleKube, types.RoleApp) {
		certRequest.DNSNames = append(certRequest.DNSNames, "*."+constants.APIDomain, constants.APIDomain)
	}
	// Unlike additional principals, DNS Names is x509 specific and is limited
	// to services with TLS endpoints (e.g. auth, proxies, kubernetes)
	if (types.SystemRoles{req.Role}).IncludeAny(types.RoleAuth, types.RoleAdmin, types.RoleProxy, types.RoleKube, types.RoleWindowsDesktop) {
		certRequest.DNSNames = append(certRequest.DNSNames, req.DNSNames...)
	}
	hostTLSCert, err := tlsAuthority.GenerateCertificate(certRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.Certs{
		SSH:        hostSSHCert,
		TLS:        hostTLSCert,
		TLSCACerts: services.GetTLSCerts(ca),
		SSHCACerts: services.GetSSHCheckingKeys(ca),
	}, nil
}

// ValidateToken takes a provisioning token value and finds if it's valid. Returns
// a list of roles this token allows its owner to assume and token labels, or an error if the token
// cannot be found.
func (a *Server) ValidateToken(token string) (types.SystemRoles, map[string]string, error) {
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
func (a *Server) checkTokenTTL(tok types.ProvisionToken) bool {
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
	Role types.SystemRole `json:"role"`
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
func (a *Server) RegisterUsingToken(req RegisterUsingTokenRequest) (*proto.Certs, error) {
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
	certs, err := a.GenerateHostCerts(context.Background(),
		&proto.HostCertsRequest{
			HostID:               req.HostID,
			NodeName:             req.NodeName,
			Role:                 req.Role,
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
	return certs, nil
}

func (a *Server) RegisterNewAuthServer(ctx context.Context, token string) error {
	tok, err := a.Provisioner.GetToken(ctx, token)
	if err != nil {
		return trace.Wrap(err)
	}
	if !tok.GetRoles().Include(types.RoleAuth) {
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
			return trace.BadParameter("token %s is statically configured and cannot be removed", backend.MaskKeyName(token))
		}
	}
	// Delete a user token.
	if err = a.Identity.DeleteUserToken(ctx, token); err == nil {
		return nil
	}
	// delete node token:
	if err = a.Provisioner.DeleteToken(ctx, token); err == nil {
		return nil
	}
	return trace.Wrap(err)
}

// GetTokens returns all tokens (machine provisioning ones and user tokens). Machine
// tokens usually have "node roles", like auth,proxy,node and user invitation tokens have 'signup' role
func (a *Server) GetTokens(ctx context.Context, opts ...services.MarshalOption) (tokens []types.ProvisionToken, err error) {
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
	// get user tokens:
	userTokens, err := a.Identity.GetUserTokens(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// convert user tokens to machine tokens:
	for _, t := range userTokens {
		roles := types.SystemRoles{types.RoleSignup}
		tok, err := types.NewProvisionToken(t.GetName(), roles, t.Expiry())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tokens = append(tokens, tok)
	}
	return tokens, nil
}

// NewWebSession creates and returns a new web session for the specified request
func (a *Server) NewWebSession(req types.NewWebSessionRequest) (types.WebSession, error) {
	user, err := a.GetUser(req.User, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := services.FetchRoles(req.Roles, a.Access, req.Traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	netCfg, err := a.GetClusterNetworkingConfig(context.TODO())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	priv, pub, err := a.GetNewKeyPairFromPool()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sessionTTL := req.SessionTTL
	if sessionTTL == 0 {
		sessionTTL = checker.AdjustSessionTTL(apidefaults.CertDuration)
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

	startTime := a.clock.Now()
	if !req.LoginTime.IsZero() {
		startTime = req.LoginTime
	}

	sessionSpec := types.WebSessionSpecV2{
		User:               req.User,
		Priv:               priv,
		Pub:                certs.SSH,
		TLSCert:            certs.TLS,
		Expires:            startTime.UTC().Add(sessionTTL),
		BearerToken:        bearerToken,
		BearerTokenExpires: startTime.UTC().Add(bearerTokenTTL),
		LoginTime:          req.LoginTime,
		IdleTimeout:        types.Duration(netCfg.GetWebIdleTimeout()),
	}
	UserLoginCount.Inc()

	sess, err := types.NewWebSession(token, types.KindWebSession, sessionSpec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sess, nil
}

// GetWebSessionInfo returns the web session specified with sessionID for the given user.
// The session is stripped of any authentication details.
// Implements auth.WebUIService
func (a *Server) GetWebSessionInfo(ctx context.Context, user, sessionID string) (types.WebSession, error) {
	sess, err := a.Identity.WebSessions().Get(ctx, types.GetWebSessionRequest{User: user, SessionID: sessionID})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sess.WithoutSecrets(), nil
}

func (a *Server) DeleteNamespace(namespace string) error {
	ctx := context.TODO()
	if namespace == apidefaults.Namespace {
		return trace.AccessDenied("can't delete default namespace")
	}
	nodes, err := a.Presence.GetNodes(ctx, namespace)
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
func (a *Server) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	return a.GetCache().NewWatcher(ctx, watch)
}

func (a *Server) CreateAccessRequest(ctx context.Context, req types.AccessRequest) error {
	err := services.ValidateAccessRequestForUser(a, req,
		// if request is in state pending, variable expansion must be applied
		services.ExpandVars(req.GetState().IsPending()),
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
	err = a.emitter.EmitAuditEvent(a.closeCtx, &apievents.AccessRequestCreate{
		Metadata: apievents.Metadata{
			Type: events.AccessRequestCreateEvent,
			Code: events.AccessRequestCreateCode,
		},
		UserMetadata: apievents.UserMetadata{
			User:         req.GetUser(),
			Impersonator: ClientImpersonator(ctx),
		},
		ResourceMetadata: apievents.ResourceMetadata{
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

func (a *Server) SetAccessRequestState(ctx context.Context, params types.AccessRequestUpdate) error {
	req, err := a.DynamicAccessExt.SetAccessRequestState(ctx, params)
	if err != nil {
		return trace.Wrap(err)
	}
	event := &apievents.AccessRequestCreate{
		Metadata: apievents.Metadata{
			Type: events.AccessRequestUpdateEvent,
			Code: events.AccessRequestUpdateCode,
		},
		ResourceMetadata: apievents.ResourceMetadata{
			UpdatedBy: ClientUsername(ctx),
			Expires:   req.GetAccessExpiry(),
		},
		RequestID:    params.RequestID,
		RequestState: params.State.String(),
		Reason:       params.Reason,
		Roles:        params.Roles,
	}

	if delegator := apiutils.GetDelegator(ctx); delegator != "" {
		event.Delegator = delegator
	}

	if len(params.Annotations) > 0 {
		annotations, err := apievents.EncodeMapStrings(params.Annotations)
		if err != nil {
			log.WithError(err).Debugf("Failed to encode access request annotations.")
		} else {
			event.Annotations = annotations
		}
	}
	err = a.emitter.EmitAuditEvent(a.closeCtx, event)
	if err != nil {
		log.WithError(err).Warn("Failed to emit access request update event.")
	}
	return trace.Wrap(err)
}

func (a *Server) SubmitAccessReview(ctx context.Context, params types.AccessReviewSubmission) (types.AccessRequest, error) {
	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// set up a checker for the review author
	checker, err := services.NewReviewPermissionChecker(ctx, a, params.Review.Author)
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

	event := &apievents.AccessRequestCreate{
		Metadata: apievents.Metadata{
			Type:        events.AccessRequestReviewEvent,
			Code:        events.AccessRequestReviewCode,
			ClusterName: clusterName.GetClusterName(),
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Expires: req.GetAccessExpiry(),
		},
		RequestID:     params.RequestID,
		RequestState:  req.GetState().String(),
		ProposedState: params.Review.ProposedState.String(),
		Reason:        params.Review.Reason,
		Reviewer:      params.Review.Author,
	}

	if len(params.Review.Annotations) > 0 {
		annotations, err := apievents.EncodeMapStrings(params.Review.Annotations)
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

func (a *Server) GetAccessCapabilities(ctx context.Context, req types.AccessCapabilitiesRequest) (*types.AccessCapabilities, error) {
	caps, err := services.CalculateAccessCapabilities(ctx, a, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return caps, nil
}

// calculateMaxAccessTTL determines the maximum allowable TTL for a given access request
// based on the MaxSessionTTLs of the roles being requested (a access request's life cannot
// exceed the smallest allowable MaxSessionTTL value of the roles that it requests).
func (a *Server) calculateMaxAccessTTL(ctx context.Context, req types.AccessRequest) (time.Duration, error) {
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
func (a *Server) NewKeepAliver(ctx context.Context) (types.KeepAliver, error) {
	cancelCtx, cancel := context.WithCancel(ctx)
	k := &authKeepAliver{
		a:           a,
		ctx:         cancelCtx,
		cancel:      cancel,
		keepAlivesC: make(chan types.KeepAlive),
	}
	go k.forwardKeepAlives()
	return k, nil
}

// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
// controls if signing keys are loaded
func (a *Server) GetCertAuthority(id types.CertAuthID, loadSigningKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error) {
	return a.GetCache().GetCertAuthority(id, loadSigningKeys, opts...)
}

// GetCertAuthorities returns a list of authorities of a given type
// loadSigningKeys controls whether signing keys should be loaded or not
func (a *Server) GetCertAuthorities(caType types.CertAuthType, loadSigningKeys bool, opts ...services.MarshalOption) ([]types.CertAuthority, error) {
	return a.GetCache().GetCertAuthorities(caType, loadSigningKeys, opts...)
}

// GetStaticTokens gets the list of static tokens used to provision nodes.
func (a *Server) GetStaticTokens() (types.StaticTokens, error) {
	return a.GetCache().GetStaticTokens()
}

// GetToken finds and returns token by ID
func (a *Server) GetToken(ctx context.Context, token string) (types.ProvisionToken, error) {
	return a.GetCache().GetToken(ctx, token)
}

// GetRoles is a part of auth.AccessPoint implementation
func (a *Server) GetRoles(ctx context.Context) ([]types.Role, error) {
	return a.GetCache().GetRoles(ctx)
}

// GetRole is a part of auth.AccessPoint implementation
func (a *Server) GetRole(ctx context.Context, name string) (types.Role, error) {
	return a.GetCache().GetRole(ctx, name)
}

// GetNamespace returns namespace
func (a *Server) GetNamespace(name string) (*types.Namespace, error) {
	return a.GetCache().GetNamespace(name)
}

// GetNamespaces is a part of auth.AccessPoint implementation
func (a *Server) GetNamespaces() ([]types.Namespace, error) {
	return a.GetCache().GetNamespaces()
}

// GetNodes is a part of auth.AccessPoint implementation
func (a *Server) GetNodes(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.Server, error) {
	return a.GetCache().GetNodes(ctx, namespace, opts...)
}

// ListNodes is a part of auth.AccessPoint implementation
func (a *Server) ListNodes(ctx context.Context, namespace string, limit int, startKey string) ([]types.Server, string, error) {
	return a.GetCache().ListNodes(ctx, namespace, limit, startKey)
}

// NodePageFunc is a function to run on each page iterated over.
type NodePageFunc func(next []types.Server) (stop bool, err error)

// IterateNodePages can be used to iterate over pages of nodes.
func (a *Server) IterateNodePages(ctx context.Context, namespace string, limit int, startKey string, f NodePageFunc) (string, error) {
	for {
		nextPage, nextKey, err := a.ListNodes(ctx, namespace, limit, startKey)
		if err != nil {
			return "", trace.Wrap(err)
		}

		stop, err := f(nextPage)
		if err != nil {
			return "", trace.Wrap(err)
		}

		// Iterator stopped before end of pages or
		// there are no more pages, return nextKey
		if stop || nextKey == "" {
			return nextKey, nil
		}

		startKey = nextKey
	}
}

// GetReverseTunnels is a part of auth.AccessPoint implementation
func (a *Server) GetReverseTunnels(opts ...services.MarshalOption) ([]types.ReverseTunnel, error) {
	return a.GetCache().GetReverseTunnels(opts...)
}

// GetProxies is a part of auth.AccessPoint implementation
func (a *Server) GetProxies() ([]types.Server, error) {
	return a.GetCache().GetProxies()
}

// GetUser is a part of auth.AccessPoint implementation.
func (a *Server) GetUser(name string, withSecrets bool) (user types.User, err error) {
	return a.GetCache().GetUser(name, withSecrets)
}

// GetUsers is a part of auth.AccessPoint implementation
func (a *Server) GetUsers(withSecrets bool) (users []types.User, err error) {
	return a.GetCache().GetUsers(withSecrets)
}

// GetTunnelConnections is a part of auth.AccessPoint implementation
// GetTunnelConnections are not using recent cache as they are designed
// to be called periodically and always return fresh data
func (a *Server) GetTunnelConnections(clusterName string, opts ...services.MarshalOption) ([]types.TunnelConnection, error) {
	return a.GetCache().GetTunnelConnections(clusterName, opts...)
}

// GetAllTunnelConnections is a part of auth.AccessPoint implementation
// GetAllTunnelConnections are not using recent cache, as they are designed
// to be called periodically and always return fresh data
func (a *Server) GetAllTunnelConnections(opts ...services.MarshalOption) (conns []types.TunnelConnection, err error) {
	return a.GetCache().GetAllTunnelConnections(opts...)
}

// CreateAuditStream creates audit event stream
func (a *Server) CreateAuditStream(ctx context.Context, sid session.ID) (apievents.Stream, error) {
	streamer, err := a.modeStreamer(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return streamer.CreateAuditStream(ctx, sid)
}

// ResumeAuditStream resumes the stream that has been created
func (a *Server) ResumeAuditStream(ctx context.Context, sid session.ID, uploadID string) (apievents.Stream, error) {
	streamer, err := a.modeStreamer(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return streamer.ResumeAuditStream(ctx, sid, uploadID)
}

// modeStreamer creates streamer based on the event mode
func (a *Server) modeStreamer(ctx context.Context) (events.Streamer, error) {
	recConfig, err := a.GetSessionRecordingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// In sync mode, auth server forwards session control to the event log
	// in addition to sending them and data events to the record storage.
	if services.IsRecordSync(recConfig.GetMode()) {
		return events.NewTeeStreamer(a.streamer, a.emitter), nil
	}
	// In async mode, clients submit session control events
	// during the session in addition to writing a local
	// session recording to be uploaded at the end of the session,
	// so forwarding events here will result in duplicate events.
	return a.streamer, nil
}

// GetAppServers is a part of the auth.AccessPoint implementation.
func (a *Server) GetAppServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.Server, error) {
	return a.GetCache().GetAppServers(ctx, namespace, opts...)
}

// GetAppSession is a part of the auth.AccessPoint implementation.
func (a *Server) GetAppSession(ctx context.Context, req types.GetAppSessionRequest) (types.WebSession, error) {
	return a.GetCache().GetAppSession(ctx, req)
}

// CreateApp creates a new application resource.
func (a *Server) CreateApp(ctx context.Context, app types.Application) error {
	if err := a.Apps.CreateApp(ctx, app); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.AppCreate{
		Metadata: apievents.Metadata{
			Type: events.AppCreateEvent,
			Code: events.AppCreateCode,
		},
		UserMetadata: apievents.UserMetadata{
			User:         ClientUsername(ctx),
			Impersonator: ClientImpersonator(ctx),
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    app.GetName(),
			Expires: app.Expiry(),
		},
		AppMetadata: apievents.AppMetadata{
			AppURI:        app.GetURI(),
			AppPublicAddr: app.GetPublicAddr(),
			AppLabels:     app.GetStaticLabels(),
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit app create event.")
	}
	return nil
}

// UpdateApp updates an existing application resource.
func (a *Server) UpdateApp(ctx context.Context, app types.Application) error {
	if err := a.Apps.UpdateApp(ctx, app); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.AppUpdate{
		Metadata: apievents.Metadata{
			Type: events.AppUpdateEvent,
			Code: events.AppUpdateCode,
		},
		UserMetadata: apievents.UserMetadata{
			User:         ClientUsername(ctx),
			Impersonator: ClientImpersonator(ctx),
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    app.GetName(),
			Expires: app.Expiry(),
		},
		AppMetadata: apievents.AppMetadata{
			AppURI:        app.GetURI(),
			AppPublicAddr: app.GetPublicAddr(),
			AppLabels:     app.GetStaticLabels(),
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit app update event.")
	}
	return nil
}

// DeleteApp deletes an application resource.
func (a *Server) DeleteApp(ctx context.Context, name string) error {
	if err := a.Apps.DeleteApp(ctx, name); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.AppDelete{
		Metadata: apievents.Metadata{
			Type: events.AppDeleteEvent,
			Code: events.AppDeleteCode,
		},
		UserMetadata: apievents.UserMetadata{
			User:         ClientUsername(ctx),
			Impersonator: ClientImpersonator(ctx),
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Name: name,
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit app delete event.")
	}
	return nil
}

// GetApps returns all application resources.
func (a *Server) GetApps(ctx context.Context) ([]types.Application, error) {
	return a.GetCache().GetApps(ctx)
}

// GetApp returns the specified application resource.
func (a *Server) GetApp(ctx context.Context, name string) (types.Application, error) {
	return a.GetCache().GetApp(ctx, name)
}

// GetDatabaseServers returns all registers database proxy servers.
func (a *Server) GetDatabaseServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.DatabaseServer, error) {
	return a.GetCache().GetDatabaseServers(ctx, namespace, opts...)
}

// CreateDatabase creates a new database resource.
func (a *Server) CreateDatabase(ctx context.Context, database types.Database) error {
	if err := a.Databases.CreateDatabase(ctx, database); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.DatabaseCreate{
		Metadata: apievents.Metadata{
			Type: events.DatabaseCreateEvent,
			Code: events.DatabaseCreateCode,
		},
		UserMetadata: apievents.UserMetadata{
			User:         ClientUsername(ctx),
			Impersonator: ClientImpersonator(ctx),
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    database.GetName(),
			Expires: database.Expiry(),
		},
		DatabaseMetadata: apievents.DatabaseMetadata{
			DatabaseProtocol:             database.GetProtocol(),
			DatabaseURI:                  database.GetURI(),
			DatabaseLabels:               database.GetStaticLabels(),
			DatabaseAWSRegion:            database.GetAWS().Region,
			DatabaseAWSRedshiftClusterID: database.GetAWS().Redshift.ClusterID,
			DatabaseGCPProjectID:         database.GetGCP().ProjectID,
			DatabaseGCPInstanceID:        database.GetGCP().InstanceID,
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit database create event.")
	}
	return nil
}

// UpdateDatabase updates an existing database resource.
func (a *Server) UpdateDatabase(ctx context.Context, database types.Database) error {
	if err := a.Databases.UpdateDatabase(ctx, database); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.DatabaseUpdate{
		Metadata: apievents.Metadata{
			Type: events.DatabaseUpdateEvent,
			Code: events.DatabaseUpdateCode,
		},
		UserMetadata: apievents.UserMetadata{
			User:         ClientUsername(ctx),
			Impersonator: ClientImpersonator(ctx),
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    database.GetName(),
			Expires: database.Expiry(),
		},
		DatabaseMetadata: apievents.DatabaseMetadata{
			DatabaseProtocol:             database.GetProtocol(),
			DatabaseURI:                  database.GetURI(),
			DatabaseLabels:               database.GetStaticLabels(),
			DatabaseAWSRegion:            database.GetAWS().Region,
			DatabaseAWSRedshiftClusterID: database.GetAWS().Redshift.ClusterID,
			DatabaseGCPProjectID:         database.GetGCP().ProjectID,
			DatabaseGCPInstanceID:        database.GetGCP().InstanceID,
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit database update event.")
	}
	return nil
}

// DeleteDatabase deletes a database resource.
func (a *Server) DeleteDatabase(ctx context.Context, name string) error {
	if err := a.Databases.DeleteDatabase(ctx, name); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.DatabaseDelete{
		Metadata: apievents.Metadata{
			Type: events.DatabaseDeleteEvent,
			Code: events.DatabaseDeleteCode,
		},
		UserMetadata: apievents.UserMetadata{
			User:         ClientUsername(ctx),
			Impersonator: ClientImpersonator(ctx),
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Name: name,
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit database delete event.")
	}
	return nil
}

// GetDatabases returns all database resources.
func (a *Server) GetDatabases(ctx context.Context) ([]types.Database, error) {
	return a.GetCache().GetDatabases(ctx)
}

// GetDatabase returns the specified database resource.
func (a *Server) GetDatabase(ctx context.Context, name string) (types.Database, error) {
	return a.GetCache().GetDatabase(ctx, name)
}

// GetLock gets a lock by name from the auth server's cache.
func (a *Server) GetLock(ctx context.Context, name string) (types.Lock, error) {
	return a.GetCache().GetLock(ctx, name)
}

// GetLocks gets all/in-force matching locks from the auth server's cache.
func (a *Server) GetLocks(ctx context.Context, inForceOnly bool, targets ...types.LockTarget) ([]types.Lock, error) {
	return a.GetCache().GetLocks(ctx, inForceOnly, targets...)
}

func (a *Server) isMFARequired(ctx context.Context, checker services.AccessChecker, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error) {
	pref, err := a.GetAuthPreference(ctx)
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
		if t.Node.Node == "" {
			return nil, trace.BadParameter("empty Node field")
		}
		if t.Node.Login == "" {
			return nil, trace.BadParameter("empty Login field")
		}
		// Find the target node and check whether MFA is required.
		nodes, err := a.GetNodes(ctx, apidefaults.Namespace)
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
			err := checker.CheckAccessToServer(t.Node.Login, n, services.AccessMFAParams{AlwaysRequired: false, Verified: false})

			// Ignore other errors; they'll be caught on the real access attempt.
			if err != nil && errors.Is(err, services.ErrSessionMFARequired) {
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
		noMFAAccessErr = checker.CheckAccessToKubernetes(apidefaults.Namespace, cluster, services.AccessMFAParams{AlwaysRequired: false, Verified: false})

	case *proto.IsMFARequiredRequest_Database:
		notFoundErr = trace.NotFound("database service %q not found", t.Database.ServiceName)
		if t.Database.ServiceName == "" {
			return nil, trace.BadParameter("missing ServiceName field in a database-only UserCertsRequest")
		}
		servers, err := a.GetDatabaseServers(ctx, apidefaults.Namespace)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var db types.Database
		for _, server := range servers {
			if server.GetDatabase().GetName() == t.Database.ServiceName {
				db = server.GetDatabase()
				break
			}
		}
		if db == nil {
			return nil, trace.Wrap(notFoundErr)
		}
		noMFAAccessErr = checker.CheckAccessToDatabase(db, services.AccessMFAParams{AlwaysRequired: false, Verified: false})

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
	if !errors.Is(noMFAAccessErr, services.ErrSessionMFARequired) {
		if !trace.IsAccessDenied(noMFAAccessErr) {
			log.WithError(noMFAAccessErr).Warn("Could not determine MFA access")
		}

		// Mask the access denied errors by returning false to prevent resource
		// name oracles. Auth will be denied (and generate an audit log entry)
		// when the client attempts to connect.
		return &proto.IsMFARequiredResponse{Required: false}, nil
	}
	// If we reach here, the error from AccessChecker was
	// ErrSessionMFARequired.

	return &proto.IsMFARequiredResponse{Required: true}, nil
}

// mfaAuthChallenge constructs an MFAAuthenticateChallenge for all MFA devices
// registered by the user.
func (a *Server) mfaAuthChallenge(ctx context.Context, user string, u2fStorage u2f.AuthenticationStorage) (*proto.MFAAuthenticateChallenge, error) {
	// Check what kind of MFA is enabled.
	apref, err := a.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var enableTOTP, enableU2F, enableWebauthn bool
	switch apref.GetSecondFactor() {
	case constants.SecondFactorOTP:
		enableTOTP = true
	case constants.SecondFactorU2F:
		enableU2F = true
	case constants.SecondFactorWebauthn:
		enableWebauthn = true
	case constants.SecondFactorOn, constants.SecondFactorOptional:
		enableTOTP, enableU2F, enableWebauthn = true, true, true
	case constants.SecondFactorOff: // All disabled.
	default:
		return nil, trace.BadParameter("unexpected second_factor value: %s", apref.GetSecondFactor())
	}

	// Read U2F configuration regardless, Webauthn uses it.
	u2fPref, err := apref.GetU2F()
	if err != nil && enableU2F {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		// If U2F parameters were not set in the auth server config, disable U2F
		// challenges.
		enableU2F = false
	}

	webConfig, err := apref.GetWebauthn()
	switch {
	case err == nil:
		enableWebauthn = enableWebauthn && !webConfig.Disabled // Apply global disable
	case err != nil && enableWebauthn:
		if apref.GetSecondFactor() == constants.SecondFactorWebauthn {
			// Fail explicitly for second_factor:"webauthn".
			return nil, trace.BadParameter("second_factor set to %s, but webauthn config not present", constants.SecondFactorWebauthn)
		}
		// Fail silently for other modes.
		log.WithError(err).Warningf("WebAuthn: failed to fetch configuration, disabling WebAuthn challenges")
		enableWebauthn = false
	}

	devs, err := a.Identity.GetMFADevices(ctx, user, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	groupedDevs := groupByDeviceType(devs, enableU2F, enableWebauthn)
	challenge := &proto.MFAAuthenticateChallenge{}
	if enableTOTP && groupedDevs.TOTP {
		challenge.TOTP = &proto.TOTPChallenge{}
	}

	if len(groupedDevs.U2F) > 0 {
		chals, err := u2f.AuthenticateInit(ctx, u2f.AuthenticateInitParams{
			AppConfig:  *u2fPref,
			Devs:       groupedDevs.U2F,
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
	if len(groupedDevs.Webauthn) > 0 {
		webLogin := &wanlib.LoginFlow{
			U2F:      u2fPref,
			Webauthn: webConfig,
			Identity: wanlib.WithDevices(a.Identity, groupedDevs.Webauthn),
		}
		assertion, err := webLogin.Begin(ctx, user)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		challenge.WebauthnChallenge = wanlib.CredentialAssertionToProto(assertion)
	}

	return challenge, nil
}

type devicesByType struct {
	TOTP     bool
	U2F      []*types.MFADevice
	Webauthn []*types.MFADevice
}

func groupByDeviceType(devs []*types.MFADevice, groupU2F, groupWebauthn bool) devicesByType {
	res := devicesByType{}
	for _, dev := range devs {
		switch dev.Device.(type) {
		case *types.MFADevice_Totp:
			res.TOTP = true
		case *types.MFADevice_U2F:
			if groupU2F {
				res.U2F = append(res.U2F, dev)
			}
			if groupWebauthn {
				res.Webauthn = append(res.Webauthn, dev)
			}
		case *types.MFADevice_Webauthn:
			if groupWebauthn {
				res.Webauthn = append(res.Webauthn, dev)
			}
		default:
			log.Warningf("Skipping MFA device of unknown type %T.", dev.Device)
		}
	}

	return res
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
	case *proto.MFAAuthenticateResponse_Webauthn:
		cap, err := a.GetAuthPreference(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		u2f, _ := cap.GetU2F()
		webConfig, err := cap.GetWebauthn()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		webLogin := &wanlib.LoginFlow{
			U2F:      u2f,
			Webauthn: webConfig,
			Identity: a.Identity,
		}
		return webLogin.Finish(ctx, user, wanlib.CredentialAssertionResponseFromProto(res.Webauthn))
	default:
		return nil, trace.BadParameter("unknown or missing MFAAuthenticateResponse type %T", resp.Response)
	}
}

func (a *Server) checkU2F(ctx context.Context, user string, res u2f.AuthenticateChallengeResponse, u2fStorage u2f.AuthenticationStorage) (*types.MFADevice, error) {
	devs, err := a.Identity.GetMFADevices(ctx, user, true)
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

func (a *Server) upsertWebSession(ctx context.Context, user string, session types.WebSession) error {
	if err := a.WebSessions().Upsert(ctx, session); err != nil {
		return trace.Wrap(err)
	}
	token, err := types.NewWebToken(session.GetBearerTokenExpiryTime(), types.WebTokenSpecV3{
		User:  session.GetUser(),
		Token: session.GetBearerToken(),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if err := a.WebTokens().Upsert(ctx, token); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetNetworkRestrictions returns the network restrictions from the cache
func (a *Server) GetNetworkRestrictions(ctx context.Context) (types.NetworkRestrictions, error) {
	return a.GetCache().GetNetworkRestrictions(ctx)
}

// SetNetworkRestrictions updates the network restrictions in the backend
func (a *Server) SetNetworkRestrictions(ctx context.Context, nr types.NetworkRestrictions) error {
	return a.Services.Restrictions.SetNetworkRestrictions(ctx, nr)
}

// DeleteNetworkRestrictions deletes the network restrictions in the backend
func (a *Server) DeleteNetworkRestrictions(ctx context.Context) error {
	return a.Services.Restrictions.DeleteNetworkRestrictions(ctx)
}

func mergeKeySets(a, b types.CAKeySet) types.CAKeySet {
	newKeySet := a.Clone()
	newKeySet.SSH = append(newKeySet.SSH, b.SSH...)
	newKeySet.TLS = append(newKeySet.TLS, b.TLS...)
	newKeySet.JWT = append(newKeySet.JWT, b.JWT...)
	return newKeySet
}

// addAdditionalTrustedKeysAtomic performs an atomic CompareAndSwap to update
// the given CA with newKeys added to the AdditionalTrustedKeys
func (a *Server) addAddtionalTrustedKeysAtomic(
	currentCA types.CertAuthority,
	newKeys types.CAKeySet,
	needsUpdate func(types.CertAuthority) bool) error {
	for {
		select {
		case <-a.closeCtx.Done():
			return trace.Wrap(a.closeCtx.Err())
		default:
		}
		if !needsUpdate(currentCA) {
			return nil
		}

		newCA := currentCA.Clone()
		currentKeySet := newCA.GetAdditionalTrustedKeys()
		mergedKeySet := mergeKeySets(currentKeySet, newKeys)
		if err := newCA.SetAdditionalTrustedKeys(mergedKeySet); err != nil {
			return trace.Wrap(err)
		}

		err := a.Trust.CompareAndSwapCertAuthority(newCA, currentCA)
		if err != nil && !trace.IsCompareFailed(err) {
			return trace.Wrap(err)
		}
		if err == nil {
			// success!
			return nil
		}
		// else trace.IsCompareFailed(err) == true (CA was concurrently updated)

		currentCA, err = a.Trust.GetCertAuthority(currentCA.GetID(), true)
		if err != nil {
			return trace.Wrap(err)
		}
	}
}

func newKeySet(keyStore keystore.KeyStore, caID types.CertAuthID) (types.CAKeySet, error) {
	var keySet types.CAKeySet
	switch caID.Type {
	case types.UserCA, types.HostCA:
		sshKeyPair, err := keyStore.NewSSHKeyPair()
		if err != nil {
			return keySet, trace.Wrap(err)
		}
		tlsKeyPair, err := keyStore.NewTLSKeyPair(caID.DomainName)
		if err != nil {
			return keySet, trace.Wrap(err)
		}
		keySet.SSH = append(keySet.SSH, sshKeyPair)
		keySet.TLS = append(keySet.TLS, tlsKeyPair)
	case types.JWTSigner:
		jwtKeyPair, err := keyStore.NewJWTKeyPair()
		if err != nil {
			return keySet, trace.Wrap(err)
		}
		keySet.JWT = append(keySet.JWT, jwtKeyPair)
	default:
		return keySet, trace.BadParameter("unknown ca type: %s", caID.Type)
	}
	return keySet, nil
}

// ensureLocalAdditionalKeys adds additional trusted keys to the CA if they are not
// already present.
func (a *Server) ensureLocalAdditionalKeys(ca types.CertAuthority) error {
	if a.keyStore.HasLocalAdditionalKeys(ca) {
		// nothing to do
		return nil
	}

	newKeySet, err := newKeySet(a.keyStore, ca.GetID())
	if err != nil {
		return trace.Wrap(err)
	}

	err = a.addAddtionalTrustedKeysAtomic(ca, newKeySet, func(ca types.CertAuthority) bool {
		return !a.keyStore.HasLocalAdditionalKeys(ca)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	log.Infof("Successfully added local additional trusted keys to %s CA.", ca.GetType())
	return nil
}

// createSelfSignedCA creates a new self-signed CA and writes it to the
// backend, with the type and clusterName given by the argument caID.
func (a *Server) createSelfSignedCA(caID types.CertAuthID) error {
	keySet, err := newKeySet(a.keyStore, caID)
	if err != nil {
		return trace.Wrap(err)
	}
	sigAlg := defaults.CASignatureAlgorithm
	if a.caSigningAlg != nil && *a.caSigningAlg != "" {
		sigAlg = *a.caSigningAlg
	}
	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        caID.Type,
		ClusterName: caID.DomainName,
		ActiveKeys:  keySet,
		SigningAlg:  sshutils.ParseSigningAlg(sigAlg),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if err := a.Trust.CreateCertAuthority(ca); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// deleteUnusedKeys deletes all teleport keys held in a connected HSM for this
// auth server which are not currently used in any CAs.
func (a *Server) deleteUnusedKeys() error {
	clusterName, err := a.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}

	var usedKeys [][]byte
	for _, caType := range []types.CertAuthType{types.HostCA, types.UserCA, types.JWTSigner} {
		caID := types.CertAuthID{Type: caType, DomainName: clusterName.GetClusterName()}
		ca, err := a.Trust.GetCertAuthority(caID, true)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, keySet := range []types.CAKeySet{ca.GetActiveKeys(), ca.GetAdditionalTrustedKeys()} {
			for _, sshKeyPair := range keySet.SSH {
				usedKeys = append(usedKeys, sshKeyPair.PrivateKey)
			}
			for _, tlsKeyPair := range keySet.TLS {
				usedKeys = append(usedKeys, tlsKeyPair.Key)
			}
			for _, jwtKeyPair := range keySet.JWT {
				usedKeys = append(usedKeys, jwtKeyPair.PrivateKey)
			}
		}
	}
	return trace.Wrap(a.keyStore.DeleteUnusedKeys(usedKeys))
}

// authKeepAliver is a keep aliver using auth server directly
type authKeepAliver struct {
	sync.RWMutex
	a           *Server
	ctx         context.Context
	cancel      context.CancelFunc
	keepAlivesC chan types.KeepAlive
	err         error
}

// KeepAlives returns a channel accepting keep alive requests
func (k *authKeepAliver) KeepAlives() chan<- types.KeepAlive {
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

	// RecoveryTokenLenBytes is len in bytes of a user token for recovery.
	RecoveryTokenLenBytes = 32

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
	connector types.SAMLConnector
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

// WithClusterCAs returns a TLS hello callback that returns a copy of the provided
// TLS config with client CAs pool of the specified cluster.
func WithClusterCAs(tlsConfig *tls.Config, ap AccessPoint, currentClusterName string, log logrus.FieldLogger) func(*tls.ClientHelloInfo) (*tls.Config, error) {
	return func(info *tls.ClientHelloInfo) (*tls.Config, error) {
		var clusterName string
		var err error
		if info.ServerName != "" {
			// Newer clients will set SNI that encodes the cluster name.
			clusterName, err = apiutils.DecodeClusterName(info.ServerName)
			if err != nil {
				if !trace.IsNotFound(err) {
					log.Debugf("Ignoring unsupported cluster name name %q.", info.ServerName)
					clusterName = ""
				}
			}
		}
		pool, err := ClientCertPool(ap, clusterName)
		if err != nil {
			log.WithError(err).Errorf("Failed to retrieve client pool for %q.", clusterName)
			// this falls back to the default config
			return nil, nil
		}

		// Per https://tools.ietf.org/html/rfc5246#section-7.4.4 the total size of
		// the known CA subjects sent to the client can't exceed 2^16-1 (due to
		// 2-byte length encoding). The crypto/tls stack will panic if this
		// happens.
		//
		// This usually happens on the root cluster with a very large (>500) number
		// of leaf clusters. In these cases, the client cert will be signed by the
		// current (root) cluster.
		//
		// If the number of CAs turns out too large for the handshake, drop all but
		// the current cluster CA. In the unlikely case where it's wrong, the
		// client will be rejected.
		var totalSubjectsLen int64
		for _, s := range pool.Subjects() {
			// Each subject in the list gets a separate 2-byte length prefix.
			totalSubjectsLen += 2
			totalSubjectsLen += int64(len(s))
		}
		if totalSubjectsLen >= int64(math.MaxUint16) {
			log.Debugf("Number of CAs in client cert pool is too large (%d) and cannot be encoded in a TLS handshake; this is due to a large number of trusted clusters; will use only the CA of the current cluster to validate.", len(pool.Subjects()))

			pool, err = ClientCertPool(ap, currentClusterName)
			if err != nil {
				log.WithError(err).Errorf("Failed to retrieve client pool for %q.", currentClusterName)
				// this falls back to the default config
				return nil, nil
			}
		}
		tlsCopy := tlsConfig.Clone()
		tlsCopy.ClientCAs = pool
		return tlsCopy, nil
	}
}
