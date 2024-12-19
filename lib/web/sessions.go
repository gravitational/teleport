/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package web

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/net/http2"
	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	kubeproto "github.com/gravitational/teleport/api/gen/proto/go/teleport/kube/v1"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// SessionContext is a context associated with a user's
// web session. An instance of the context is created for
// each web session generated for the user and provides
// a basic client cache for remote auth server connections.
type SessionContext struct {
	// SessionContextConfig contains dependency injected configurations
	cfg SessionContextConfig
	// remoteClientCache holds the remote clients that have been used in this
	// session.
	remoteClientCache
	// remoteClientGroup prevents duplicate requests to create remote clients
	// for a given site
	remoteClientGroup singleflight.Group

	// mu guards kubeGRPCServiceConn.
	mu sync.Mutex
	// kubeGRPCServiceConn is a connection to the kubernetes service.
	kubeGRPCServiceConn *grpc.ClientConn
}

type SessionContextConfig struct {
	// Log is used to emit logs
	Log *slog.Logger
	// User is the name of the current user
	User string

	// RootClusterName is the name of the root cluster
	RootClusterName string

	// RootClient holds a connection to the root auth. Note that requests made using this
	// client are made with the identity of the user and are NOT cached.
	RootClient *authclient.Client

	// UnsafeCachedAuthClient holds a read-only cache to root auth. Note this access
	// point cache is authenticated with the identity of the node, not of the
	// user. This is why its prefixed with "unsafe".
	//
	// This access point should only be used if the identity of the caller will
	// not affect the result of the RPC. For example, never use it to call
	// "GetNodes".
	UnsafeCachedAuthClient authclient.ReadProxyAccessPoint

	Parent *sessionCache
	// Resources is a persistent resource store this context is bound to.
	// The store maintains a list of resources between session renewals
	Resources *sessionResources
	// Session refers the web session created for the user.
	Session types.WebSession

	// newRemoteClient is used by tests to override how remote clients are constructed to allow for fake sites
	newRemoteClient func(ctx context.Context, sessionContext *SessionContext, site reversetunnelclient.RemoteSite) (authclient.ClientI, error)
}

func (c *SessionContextConfig) CheckAndSetDefaults() error {
	if c.RootClient == nil {
		return trace.BadParameter("RootClient required")
	}

	if c.UnsafeCachedAuthClient == nil {
		return trace.BadParameter("UnsafeCachedAuthClient required")
	}

	if c.Parent == nil {
		return trace.BadParameter("Parent required")
	}

	if c.Resources == nil {
		return trace.BadParameter("Resources required")
	}

	if c.Session == nil {
		return trace.BadParameter("Session required")
	}

	if c.Log == nil {
		c.Log = slog.With(
			"user", c.User,
			"session", c.Session.GetShortName(),
		)
	}

	if c.newRemoteClient == nil {
		c.newRemoteClient = newRemoteClient
	}

	if c.RootClusterName == "" {
		c.RootClusterName = c.Parent.clusterName
	}

	return nil
}

func NewSessionContext(cfg SessionContextConfig) (*SessionContext, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &SessionContext{
		cfg: cfg,
	}, nil
}

// String returns the text representation of this context
func (c *SessionContext) String() string {
	return fmt.Sprintf("WebSession(user=%v,id=%v,expires=%v,bearer_expires=%v)",
		c.cfg.User,
		c.cfg.Session.GetShortName(),
		c.cfg.Session.GetExpiryTime(),
		c.cfg.Session.GetBearerTokenExpiryTime(),
	)
}

// AddClosers adds the specified closers to this context
func (c *SessionContext) AddClosers(closers ...io.Closer) {
	c.cfg.Resources.addClosers(closers...)
}

// RemoveCloser removes the specified closer from this context
func (c *SessionContext) RemoveCloser(closer io.Closer) {
	c.cfg.Resources.removeCloser(closer)
}

// Invalidate invalidates this context by removing the underlying session
// and closing all underlying closers
func (c *SessionContext) Invalidate(ctx context.Context) error {
	return c.cfg.Parent.invalidateSession(ctx, c)
}

func (c *SessionContext) validateBearerToken(ctx context.Context, token string) error {
	fetchedToken, err := c.cfg.Parent.readBearerToken(ctx, types.GetWebTokenRequest{
		User:  c.cfg.User,
		Token: token,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if fetchedToken.GetUser() != c.cfg.User {
		c.cfg.Log.WarnContext(ctx, "Failed validating bearer token: the user in bearer token did not match the user for session",
			"token_user", fetchedToken.GetUser(),
			"token", token,
			"session_user", c.cfg.User,
			"session_id", c.GetSessionID(),
		)
		return trace.AccessDenied("access denied")
	}

	return nil
}

// GetClient returns the client connected to the auth server
func (c *SessionContext) GetClient() (authclient.ClientI, error) {
	return c.cfg.RootClient, nil
}

// GetClientConnection returns a connection to Auth Service
func (c *SessionContext) GetClientConnection() *grpc.ClientConn {
	return c.cfg.RootClient.GetConnection()
}

// GetUserClient will return an [authclient.ClientI]  with the role of the user at
// the requested site. If the site is local a client with the users local role
// is returned. If the site is remote a client with the users remote role is
// returned.
func (c *SessionContext) GetUserClient(ctx context.Context, site reversetunnelclient.RemoteSite) (authclient.ClientI, error) {
	// if we're trying to access the local cluster, pass back the local client.
	if c.cfg.RootClusterName == site.GetName() {
		return c.cfg.RootClient, nil
	}

	// return the client for the requested remote site
	clt, err := c.remoteClient(ctx, site)
	return clt, trace.Wrap(err)
}

// remoteClient returns an [authclient.ClientI]  with the role of the user at
// the requested [site]. All remote clients are lazily created
// when they are first requested and then cached. Subsequent requests
// will return the previously created client to prevent having more than
// a single [authclient.ClientI]  per site for a user.
//
// A [singleflight.Group] is leveraged to prevent duplicate requests for remote
// clients at the same time to race.
func (c *SessionContext) remoteClient(ctx context.Context, site reversetunnelclient.RemoteSite) (authclient.ClientI, error) {
	cltI, err, _ := c.remoteClientGroup.Do(site.GetName(), func() (interface{}, error) {
		// check if we already have a connection to this cluster
		if clt, ok := c.remoteClientCache.getRemoteClient(site); ok {
			return clt, nil
		}

		rClt, err := c.cfg.newRemoteClient(ctx, c, site)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// we'll save the remote client in our session context so we don't have to
		// build a new connection next time. all remote clients will be closed when
		// the session context is closed.
		err = c.remoteClientCache.addRemoteClient(site, rClt)
		if err != nil {
			c.cfg.Log.InfoContext(ctx, "Failed closing stale remote client for site",
				"remote_site", site.GetName(),
				"error", err,
			)
		}

		return rClt, nil
	})

	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, ok := cltI.(authclient.ClientI)
	if !ok {
		return nil, trace.BadParameter("unexpected type %T received for auth client", cltI)
	}

	return clt, nil
}

// newRemoteClient returns a client to a remote cluster with the role of current user.
func newRemoteClient(ctx context.Context, sctx *SessionContext, site reversetunnelclient.RemoteSite) (authclient.ClientI, error) {
	clt, err := sctx.newRemoteTLSClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Clients lazily dial, so attempt an RPC to determine if this client
	// is functional or not.
	_, err = clt.GetDomainName(ctx)
	if err != nil {
		return nil, trace.NewAggregate(err, clt.Close())
	}

	return clt, nil
}

// clusterDialer returns DialContext function using cluster's dial function
func clusterDialer(remoteCluster reversetunnelclient.RemoteSite, src, dst net.Addr) apiclient.ContextDialer {
	return apiclient.ContextDialerFunc(func(in context.Context, network, _ string) (net.Conn, error) {
		dialParams := reversetunnelclient.DialParams{
			From:                  src,
			OriginalClientDstAddr: dst,
		}

		clientSrcAddr, clientDstAddr := authz.ClientAddrsFromContext(in)
		if dialParams.From == nil && clientSrcAddr != nil {
			dialParams.From = clientSrcAddr
		}
		if dialParams.OriginalClientDstAddr == nil && clientDstAddr != nil {
			dialParams.OriginalClientDstAddr = clientSrcAddr
		}

		return remoteCluster.DialAuthServer(dialParams)
	})
}

// NewKubernetesServiceClient returns a new KubernetesServiceClient.
func (c *SessionContext) NewKubernetesServiceClient(ctx context.Context, addr string) (kubeproto.KubeServiceClient, error) {
	c.mu.Lock()
	conn := c.kubeGRPCServiceConn
	c.mu.Unlock()
	if conn != nil {
		return kubeproto.NewKubeServiceClient(conn), nil
	}

	tlsConfig, err := c.ClientTLSConfig(ctx, c.cfg.RootClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Set the ALPN protocols to use when dialing the proxy gRPC mTLS endpoint.
	tlsConfig.NextProtos = []string{string(alpncommon.ProtocolProxyGRPCSecure), http2.NextProtoTLS}
	conn, err = grpc.DialContext(
		ctx,
		addr,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithChainUnaryInterceptor(
			//nolint:staticcheck // SA1019. There is a data race in the stats.Handler that is replacing
			// the interceptor. See https://github.com/open-telemetry/opentelemetry-go-contrib/issues/4576.
			otelgrpc.UnaryClientInterceptor(),
			metadata.UnaryClientInterceptor,
		),
		grpc.WithChainStreamInterceptor(
			//nolint:staticcheck // SA1019. There is a data race in the stats.Handler that is replacing
			// the interceptor. See https://github.com/open-telemetry/opentelemetry-go-contrib/issues/4576.
			otelgrpc.StreamClientInterceptor(),
			metadata.StreamClientInterceptor,
		),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.mu.Lock()
	c.kubeGRPCServiceConn = conn
	c.mu.Unlock()
	return kubeproto.NewKubeServiceClient(conn), nil
}

// ClientTLSConfig returns client TLS authentication associated
// with the web session context
func (c *SessionContext) ClientTLSConfig(ctx context.Context, clusterName ...string) (*tls.Config, error) {
	var certPool *x509.CertPool
	if len(clusterName) == 0 {
		certAuthorities, err := c.cfg.Parent.proxyClient.GetCertAuthorities(ctx, types.HostCA, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certPool, _, err = services.CertPoolFromCertAuthorities(certAuthorities)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		certAuthority, err := c.cfg.Parent.proxyClient.GetCertAuthority(ctx, types.CertAuthID{
			Type:       types.HostCA,
			DomainName: clusterName[0],
		}, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certPool, err = services.CertPool(certAuthority)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	tlsConfig := utils.TLSConfig(c.cfg.Parent.cipherSuites)
	tlsCert, err := tls.X509KeyPair(c.cfg.Session.GetTLSCert(), c.cfg.Session.GetTLSPriv())
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse TLS cert and key")
	}
	tlsConfig.Certificates = []tls.Certificate{tlsCert}
	tlsConfig.RootCAs = certPool
	tlsConfig.ServerName = apiutils.EncodeClusterName(c.cfg.Parent.clusterName)
	tlsConfig.Time = c.cfg.Parent.clock.Now
	return tlsConfig, nil
}

func (c *SessionContext) newRemoteTLSClient(ctx context.Context, cluster reversetunnelclient.RemoteSite) (authclient.ClientI, error) {
	tlsConfig, err := c.ClientTLSConfig(ctx, cluster.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientSrcAddr, clientDstAddr := authz.ClientAddrsFromContext(ctx)

	return authclient.NewClient(apiclient.Config{
		Context: ctx,
		Dialer:  clusterDialer(cluster, clientSrcAddr, clientDstAddr),
		Credentials: []apiclient.Credentials{
			apiclient.LoadTLS(tlsConfig),
		},
		CircuitBreakerConfig: breaker.NoopBreakerConfig(),
	})
}

// GetUser returns the authenticated teleport user
func (c *SessionContext) GetUser() string {
	return c.cfg.User
}

// extendWebSession creates a new web session for this user
// based on the previous session
func (c *SessionContext) extendWebSession(ctx context.Context, req renewSessionRequest) (types.WebSession, error) {
	session, err := c.cfg.RootClient.ExtendWebSession(ctx, authclient.WebSessionReq{
		User:            c.cfg.User,
		PrevSessionID:   c.cfg.Session.GetName(),
		AccessRequestID: req.AccessRequestID,
		Switchback:      req.Switchback,
		ReloadUser:      req.ReloadUser,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

// GetAgent returns agent that can be used to answer challenges
// for the web to ssh connection as well as certificate
func (c *SessionContext) GetAgent() (agent.ExtendedAgent, *ssh.Certificate, error) {
	cert, err := c.GetSSHCertificate()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if len(cert.ValidPrincipals) == 0 {
		return nil, nil, trace.BadParameter("expected at least valid principal in certificate")
	}
	privateKey, err := ssh.ParseRawPrivateKey(c.cfg.Session.GetSSHPriv())
	if err != nil {
		return nil, nil, trace.Wrap(err, "failed to parse SSH private key")
	}

	keyring, ok := agent.NewKeyring().(agent.ExtendedAgent)
	if !ok {
		return nil, nil, trace.Errorf("unexpected keyring type: %T, expected agent.ExtendedKeyring", keyring)
	}
	err = keyring.Add(agent.AddedKey{
		PrivateKey:  privateKey,
		Certificate: cert,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return keyring, cert, nil
}

func (c *SessionContext) getCheckers() ([]ssh.PublicKey, error) {
	ctx := context.TODO()
	cas, err := c.cfg.UnsafeCachedAuthClient.GetCertAuthorities(ctx, types.HostCA, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var keys []ssh.PublicKey
	for _, ca := range cas {
		checkers, err := sshutils.GetCheckers(ca)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		keys = append(keys, checkers...)
	}
	return keys, nil
}

// GetSSHCertificate returns the *ssh.Certificate associated with this session.
func (c *SessionContext) GetSSHCertificate() (*ssh.Certificate, error) {
	return apisshutils.ParseCertificate(c.cfg.Session.GetPub())
}

// GetX509Certificate returns the *x509.Certificate associated with this session.
func (c *SessionContext) GetX509Certificate() (*x509.Certificate, error) {
	tlsCert, err := tlsca.ParseCertificatePEM(c.cfg.Session.GetTLSCert())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tlsCert, nil
}

// GetUserAccessChecker returns AccessChecker derived from the SSH certificate
// associated with this session.
func (c *SessionContext) GetUserAccessChecker() (services.AccessChecker, error) {
	cert, err := c.GetSSHCertificate()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessInfo, err := services.AccessInfoFromLocalCertificate(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessChecker, err := services.NewAccessChecker(accessInfo, c.cfg.RootClusterName, c.cfg.UnsafeCachedAuthClient)
	return accessChecker, trace.Wrap(err)
}

// GetProxyListenerMode returns cluster proxy listener mode form cluster networking config.
func (c *SessionContext) GetProxyListenerMode(ctx context.Context) (types.ProxyListenerMode, error) {
	resp, err := c.cfg.UnsafeCachedAuthClient.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return types.ProxyListenerMode_Separate, trace.Wrap(err)
	}
	return resp.GetProxyListenerMode(), nil
}

// GetIdentity returns identity parsed from the session's TLS certificate.
func (c *SessionContext) GetIdentity() (*tlsca.Identity, error) {
	cert, err := c.GetX509Certificate()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return identity, nil
}

// GetSessionID returns the ID of the underlying user web session.
func (c *SessionContext) GetSessionID() string {
	return c.cfg.Session.GetName()
}

// GetRootClusterName returns the root cluster name.
func (c *SessionContext) GetRootClusterName() string {
	return c.cfg.RootClusterName
}

// Close cleans up resources associated with this context and removes it
// from the user context
func (c *SessionContext) Close() error {
	var err error
	c.mu.Lock()
	if c.kubeGRPCServiceConn != nil {
		err = c.kubeGRPCServiceConn.Close()
	}
	c.mu.Unlock()
	return trace.NewAggregate(c.remoteClientCache.Close(), c.cfg.RootClient.Close(), err)
}

// getToken returns the bearer token associated with the underlying
// session. Note that sessions are separate from bearer tokens and this
// is only useful immediately after a session has been created to query
// the token.
func (c *SessionContext) getToken() (types.WebToken, error) {
	t, err := types.NewWebToken(c.cfg.Session.GetBearerTokenExpiryTime(), types.WebTokenSpecV3{
		User:  c.cfg.Session.GetUser(),
		Token: c.cfg.Session.GetBearerToken(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return t, nil
}

// expired returns whether this context has expired.
// Session records in the backend are created with a built-in expiry that
// automatically deletes the session record from the back-end database at
// the end of its natural life.
// If a session record still exists in the backend, it is considered still
// alive, regardless of the time. If no such record exists then a record is
// considered expired when its bearer token TTL is in the past (subject to
// lingering threshold)
func (c *SessionContext) expired(ctx context.Context) bool {
	_, err := c.cfg.Parent.readSession(ctx, types.GetWebSessionRequest{
		User:      c.cfg.User,
		SessionID: c.cfg.Session.GetName(),
	})

	switch {
	case err == nil:
		// If looking up the session in the cache or backend succeeds, then
		// it by definition must not have expired yet.
		return false
	case trace.IsNotFound(err):
		// If the session doesn't exist in the cache or backend, then it
		// was removed during user logout, expire the session immediately.
		return true
	default:
		c.cfg.Log.DebugContext(ctx, "Failed to query web session", "error", err)
	}

	// If the session has no expiry time, then also by definition it
	// cannot be expired
	expiry := c.cfg.Session.GetBearerTokenExpiryTime()
	if expiry.IsZero() {
		return false
	}

	// Give the session some time to linger so existing users of the context
	// have successfully disposed of them.
	// If we remove the session immediately, a stale copy might still use the
	// cached site clients.
	// This is a cheaper way to avoid race without introducing object
	// reference counters.
	return c.cfg.Parent.clock.Since(expiry) > c.cfg.Parent.sessionLingeringThreshold
}

// cachedSessionLingeringThreshold specifies the maximum amount of time the session cache
// will hold onto a session before removing it. This period allows all outstanding references
// to disappear without fear of racing with the removal
const cachedSessionLingeringThreshold = 2 * time.Minute

type sessionCacheOptions struct {
	proxyClient  authclient.ClientI
	accessPoint  authclient.ReadProxyAccessPoint
	servers      []utils.NetAddr
	cipherSuites []uint16
	clock        clockwork.Clock
	// sessionLingeringThreshold specifies the time the session will linger
	// in the cache before getting purged after it has expired
	sessionLingeringThreshold time.Duration
	// proxySigner is used to sign PROXY header and securely propagate client's real IP
	proxySigner multiplexer.PROXYHeaderSigner
	// See [sessionCache.sessionWatcherStartImmediately]. Used for testing.
	sessionWatcherStartImmediately bool
	// See [sessionCache.sessionWatcherEventProcessedChannel]. Used for testing.
	sessionWatcherEventProcessedChannel chan struct{}
	logger                              *slog.Logger
}

// newSessionCache creates a [sessionCache] from the provided [config] and
// launches a goroutine that runs until [ctx] is completed which
// periodically purges invalid sessions.
func newSessionCache(ctx context.Context, config sessionCacheOptions) (*sessionCache, error) {
	clusterName, err := config.proxyClient.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if config.clock == nil {
		config.clock = clockwork.NewRealClock()
	}

	if config.logger == nil {
		config.logger = slog.Default()
	}

	cache := &sessionCache{
		clusterName:                         clusterName.GetClusterName(),
		proxyClient:                         config.proxyClient,
		accessPoint:                         config.accessPoint,
		sessions:                            make(map[string]*SessionContext),
		resources:                           make(map[string]*sessionResources),
		authServers:                         config.servers,
		closer:                              utils.NewCloseBroadcaster(),
		cipherSuites:                        config.cipherSuites,
		log:                                 config.logger,
		clock:                               config.clock,
		sessionLingeringThreshold:           config.sessionLingeringThreshold,
		proxySigner:                         config.proxySigner,
		sessionWatcherStartImmediately:      config.sessionWatcherStartImmediately,
		sessionWatcherEventProcessedChannel: config.sessionWatcherEventProcessedChannel,
	}

	// periodically close expired and unused sessions
	go cache.expireSessions(ctx)

	// Watch for session updates.
	go cache.watchWebSessions(ctx)

	return cache, nil
}

// sessionCache handles web session authentication,
// and holds in-memory contexts associated with each session
type sessionCache struct {
	log         *slog.Logger
	proxyClient authclient.ClientI
	authServers []utils.NetAddr
	accessPoint authclient.ReadProxyAccessPoint
	closer      *utils.CloseBroadcaster
	clusterName string
	clock       clockwork.Clock
	// sessionLingeringThreshold specifies the time the session will linger
	// in the cache before getting purged after it has expired
	sessionLingeringThreshold time.Duration
	// cipherSuites is the list of supported TLS cipher suites.
	cipherSuites []uint16

	mu sync.RWMutex
	// sessions maps user/sessionID to an active web session value between renewals.
	// This is the client-facing session handle
	sessions map[string]*SessionContext
	// sessionGroup ensures only a single SessionContext will exist for a
	// user+session.
	sessionGroup singleflight.Group

	// session cache maintains a list of resources per-user as long
	// as the user session is active even though individual session values
	// are periodically recycled.
	// Resources are disposed of when the corresponding session
	// is either explicitly invalidated (e.g. during logout) or the
	// resources are themselves closing
	resources map[string]*sessionResources

	// proxySigner is used to sign PROXY header and securely propagate client's real IP
	proxySigner multiplexer.PROXYHeaderSigner

	// sessionWatcherStartImmediately removes the First component of the linear
	// backoff used to start the WebSession watcher.
	// Used for testing.
	sessionWatcherStartImmediately bool

	// sessionWatcherEventProcessedChannel is used to signal that the
	// sessionWatcher processed an event.
	// May be nil.
	// Used for testing.
	sessionWatcherEventProcessedChannel chan struct{}
}

// Close closes all allocated resources and stops goroutines
func (s *sessionCache) Close() error {
	s.log.InfoContext(context.Background(), "Closing session cache")
	return s.closer.Close()
}

func (s *sessionCache) ActiveSessions() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

func (s *sessionCache) expireSessions(ctx context.Context) {
	ticker := s.clock.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.Chan():
			s.clearExpiredSessions(ctx)
		case <-s.closer.C:
			return
		}
	}
}

func (s *sessionCache) clearExpiredSessions(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.sessions {
		if !c.expired(ctx) {
			continue
		}
		s.removeSessionContextLocked(ctx, c.cfg.Session.GetUser(), c.cfg.Session.GetName())
		s.log.DebugContext(ctx, "Context expired", "context", logutils.StringerAttr(c))
	}
}

// watchWebSessions runs the WebSession watcher loop.
// It only stops when ctx is done.
func (s *sessionCache) watchWebSessions(ctx context.Context) {
	// Watcher not necessary for OSS.
	if modules.GetModules().BuildType() != modules.BuildEnterprise {
		return
	}

	linear := utils.NewDefaultLinear()
	if s.sessionWatcherStartImmediately {
		linear.First = 0
	}

	s.log.DebugContext(ctx, "sessionCache: Starting WebSession watcher")
	for {
		select {
		// Stop when the context tells us to.
		case <-ctx.Done():
			s.log.DebugContext(ctx, "sessionCache: Stopping WebSession watcher")
			return

		case <-linear.After():
			linear.Inc()
		}

		if err := s.watchWebSessionsOnce(ctx, linear.Reset); err != nil && !errors.Is(err, context.Canceled) {
			const msg = "" +
				"sessionCache: WebSession watcher aborted, re-connecting. " +
				"This may have an impact in device trust web sessions."
			s.log.WarnContext(ctx, msg, "error", err)
		}
	}
}

// watchWebSessionsOnce creates a watcher for WebSessions and watches for its
// events.
//
// Any session updated with device extensions is evicted from the cache. That is
// so the new certificates are forcefully loaded by the Proxy.
//
// Sessions updated for other reasons (no device extensions present) or cached
// sessions that already have device extensions are ignored. This avoids
// disconnecting clients during periodic bearer token refresh by the Web UI.
func (s *sessionCache) watchWebSessionsOnce(ctx context.Context, reset func()) error {
	watcher, err := s.proxyClient.NewWatcher(ctx, types.Watch{
		Name: teleport.ComponentWebProxy + ".sessionCache." + types.KindWebSession,
		Kinds: []types.WatchKind{
			{
				Kind: types.KindWebSession,
				// Watch only for KindWebSession.
				// SubKinds include KindAppSession, KindSAMLIdPSession, etc.
				SubKind: types.KindWebSession,
			},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()

	// notifyProcessed is a feedback mechanism for tests.
	notifyProcessed := func() {
		if s.sessionWatcherEventProcessedChannel != nil {
			s.sessionWatcherEventProcessedChannel <- struct{}{}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-watcher.Done():
			return errors.New("watcher closed")

		case event := <-watcher.Events():
			reset() // Reset linear backoff attempts.

			s.log.Log(ctx, logutils.TraceLevel, "sessionCache: Received watcher event",
				"event", logutils.StringerAttr(event),
			)

			if event.Type != types.OpPut {
				continue // We only care about OpPut at the moment.
			}

			session, ok := event.Resource.(types.WebSession)
			if !ok {
				s.log.WarnContext(ctx, "sessionCache: Received unexpected resource type",
					"resource_type", logutils.TypeAttr(event.Resource),
				)
				continue
			}
			if !session.GetHasDeviceExtensions() {
				s.log.DebugContext(ctx, "sessionCache: Updated session doesn't have device extensions, skipping",
					"session_id", session.GetName(),
				)
				notifyProcessed()
				continue
			}

			// Release existing and non-device-aware session.
			if err := s.releaseResourcesIfNoDeviceExtensions(ctx, session.GetUser(), session.GetName()); err != nil {
				s.log.DebugContext(ctx, "sessionCache: Failed to release updated session",
					"error", err,
					"session_id", session.GetName(),
				)
			}

			notifyProcessed()
		}
	}
}

func (s *sessionCache) releaseResourcesIfNoDeviceExtensions(ctx context.Context, user, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := sessionKey(user, sessionID)
	switch sessionCtx, ok := s.sessions[id]; {
	case !ok:
		return nil // Session not found
	case sessionCtx.cfg.Session.GetHasDeviceExtensions():
		s.log.DebugContext(ctx, "sessionCache: Session already has device extensions, skipping",
			"session_id", sessionID,
		)
		return nil
	}

	s.log.DebugContext(ctx, "sessionCache: Releasing session resources due to device extensions upgrade",
		"session_id", sessionID,
	)
	return s.releaseResourcesLocked(ctx, user, sessionID)
}

// AuthWithOTP authenticates the specified user with the given password and OTP token.
// Returns a new web session if successful.
func (s *sessionCache) AuthWithOTP(
	ctx context.Context,
	user, pass, otpToken string,
	clientMeta *authclient.ForwardedClientMetadata,
) (types.WebSession, error) {
	return s.proxyClient.AuthenticateWebUser(ctx, authclient.AuthenticateUserRequest{
		Username: user,
		Pass:     &authclient.PassCreds{Password: []byte(pass)},
		OTP: &authclient.OTPCreds{
			Password: []byte(pass),
			Token:    otpToken,
		},
		ClientMetadata: clientMeta,
	})
}

// AuthWithoutOTP authenticates the specified user with the given password.
// Returns a new web session if successful.
func (s *sessionCache) AuthWithoutOTP(
	ctx context.Context, user, pass string, clientMeta *authclient.ForwardedClientMetadata,
) (types.WebSession, error) {
	return s.proxyClient.AuthenticateWebUser(ctx, authclient.AuthenticateUserRequest{
		Username: user,
		Pass: &authclient.PassCreds{
			Password: []byte(pass),
		},
		ClientMetadata: clientMeta,
	})
}

func (s *sessionCache) AuthenticateWebUser(
	ctx context.Context, req *client.AuthenticateWebUserRequest, clientMeta *authclient.ForwardedClientMetadata,
) (types.WebSession, error) {
	authReq := authclient.AuthenticateUserRequest{
		Username:       req.User,
		ClientMetadata: clientMeta,
	}
	if req.WebauthnAssertionResponse != nil {
		authReq.Webauthn = req.WebauthnAssertionResponse
	}
	return s.proxyClient.AuthenticateWebUser(ctx, authReq)
}

func (s *sessionCache) AuthenticateSSHUser(
	ctx context.Context, c client.AuthenticateSSHUserRequest, clientMeta *authclient.ForwardedClientMetadata,
) (*authclient.SSHLoginResponse, error) {
	authReq := authclient.AuthenticateUserRequest{
		Username:       c.User,
		ClientMetadata: clientMeta,
		SSHPublicKey:   c.UserPublicKeys.SSHPubKey,
		TLSPublicKey:   c.UserPublicKeys.TLSPubKey,
	}
	if c.Password != "" {
		authReq.Pass = &authclient.PassCreds{Password: []byte(c.Password)}
	}
	if c.WebauthnChallengeResponse != nil {
		authReq.Webauthn = c.WebauthnChallengeResponse
	}
	if c.TOTPCode != "" {
		authReq.OTP = &authclient.OTPCreds{
			Password: []byte(c.Password),
			Token:    c.TOTPCode,
		}
	}
	return s.proxyClient.AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
		AuthenticateUserRequest: authReq,
		CompatibilityMode:       c.Compatibility,
		TTL:                     c.TTL,
		RouteToCluster:          c.RouteToCluster,
		KubernetesCluster:       c.KubernetesCluster,
		SSHAttestationStatement: c.UserPublicKeys.SSHAttestationStatement,
		TLSAttestationStatement: c.UserPublicKeys.TLSAttestationStatement,
	})
}

// Ping gets basic info about the auth server.
func (s *sessionCache) Ping(ctx context.Context) (proto.PingResponse, error) {
	return s.proxyClient.Ping(ctx)
}

func (s *sessionCache) ValidateTrustedCluster(ctx context.Context, validateRequest *authclient.ValidateTrustedClusterRequest) (*authclient.ValidateTrustedClusterResponse, error) {
	return s.proxyClient.ValidateTrustedCluster(ctx, validateRequest)
}

// getOrCreateSession gets the SessionContext for the user and session ID. If one does
// not exist, then a new one is created.
func (s *sessionCache) getOrCreateSession(ctx context.Context, user, sessionID string) (*SessionContext, error) {
	key := sessionKey(user, sessionID)

	// Use sessionGroup to prevent multiple requests from racing to create a SessionContext.
	i, err, _ := s.sessionGroup.Do(key, func() (any, error) {
		sessionCtx, ok := s.getContext(key)
		if ok {
			return sessionCtx, nil
		}

		return s.newSessionContext(ctx, user, sessionID)
	})

	if err != nil {
		return nil, trace.Wrap(err)
	}

	sctx, ok := i.(*SessionContext)
	if !ok {
		return nil, trace.BadParameter("expected SessionContext, got %T", i)
	}

	return sctx, nil
}

func (s *sessionCache) invalidateSession(ctx context.Context, sctx *SessionContext) error {
	defer sctx.Close()
	clt, err := sctx.GetClient()
	if err != nil {
		return trace.Wrap(err)
	}

	// App session, SAML session and web session deletion should be treated as a single transaction.
	// To avoid aborting deletion midpoint due to a failure in one of the session deletion,
	// we use sessionDeletionErr below to join errors and return them at last.
	var sessionDeletionErrs error
	if err := clt.DeleteUserAppSessions(ctx, &proto.DeleteUserAppSessionsRequest{Username: sctx.GetUser()}); err != nil {
		sessionDeletionErrs = err
	}

	// Delete just the session - leave the bearer token to linger to avoid
	// failing a client query still using the old token.
	if err := clt.WebSessions().Delete(ctx, types.DeleteWebSessionRequest{
		User:      sctx.GetUser(),
		SessionID: sctx.GetSessionID(),
	}); err != nil && !trace.IsNotFound(err) {
		sessionDeletionErrs = errors.Join(sessionDeletionErrs, err)
	}

	return trace.Wrap(sessionDeletionErrs)
}

func (s *sessionCache) getContext(key string) (*SessionContext, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ctx, ok := s.sessions[key]
	return ctx, ok
}

func (s *sessionCache) insertContext(user string, sctx *SessionContext) (exists bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := sessionKey(user, sctx.GetSessionID())
	if _, exists := s.sessions[id]; exists {
		return true
	}
	s.sessions[id] = sctx
	return false
}

func (s *sessionCache) releaseResources(ctx context.Context, user, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.releaseResourcesLocked(ctx, user, sessionID)
}

func (s *sessionCache) removeSessionContextLocked(ctx context.Context, user, sessionID string) error {
	id := sessionKey(user, sessionID)
	sess, ok := s.sessions[id]
	if !ok {
		return nil
	}
	delete(s.sessions, id)
	err := sess.Close()
	if err != nil {
		s.log.WarnContext(ctx, "Failed to close session context",
			"context", logutils.StringerAttr(sess),
			"error", err,
		)
		return trace.Wrap(err)
	}
	return nil
}

func (s *sessionCache) releaseResourcesLocked(ctx context.Context, user, sessionID string) error {
	var errors []error
	err := s.removeSessionContextLocked(ctx, user, sessionID)
	if err != nil {
		errors = append(errors, err)
	}
	if sess, ok := s.resources[user]; ok {
		delete(s.resources, user)
		if err := sess.Close(); err != nil {
			s.log.WarnContext(ctx, "Failed to clean up session context", "error", err)
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

func (s *sessionCache) upsertSessionContext(user string) *sessionResources {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx, exists := s.resources[user]; exists {
		return ctx
	}
	ctx := &sessionResources{
		log: s.log.With(
			teleport.ComponentKey, "user-session",
			"user", user,
		),
	}
	s.resources[user] = ctx
	return ctx
}

// newSessionContext creates a new web session context for the specified user/session ID
func (s *sessionCache) newSessionContext(ctx context.Context, user, sessionID string) (*SessionContext, error) {
	session, err := s.proxyClient.AuthenticateWebUser(ctx, authclient.AuthenticateUserRequest{
		Username: user,
		Session: &authclient.SessionCreds{
			ID: sessionID,
		},
	})
	if err != nil {
		// This will fail if the session has expired and was removed
		return nil, trace.Wrap(err)
	}
	return s.newSessionContextFromSession(ctx, session)
}

func (s *sessionCache) newSessionContextFromSession(ctx context.Context, session types.WebSession) (*SessionContext, error) {
	tlsConfig, err := s.tlsConfig(ctx, session.GetTLSCert(), session.GetTLSPriv())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userClient, err := authclient.NewClient(apiclient.Config{
		Addrs:                utils.NetAddrsToStrings(s.authServers),
		Credentials:          []apiclient.Credentials{apiclient.LoadTLS(tlsConfig)},
		CircuitBreakerConfig: breaker.NoopBreakerConfig(),
		PROXYHeaderGetter:    client.CreatePROXYHeaderGetter(ctx, s.proxySigner),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sctx, err := NewSessionContext(SessionContextConfig{
		Log: s.log.With(
			"user", session.GetUser(),
			"session", session.GetShortName(),
		),
		User:                   session.GetUser(),
		RootClient:             userClient,
		UnsafeCachedAuthClient: s.accessPoint,
		Parent:                 s,
		Resources:              s.upsertSessionContext(session.GetUser()),
		Session:                session,
		RootClusterName:        s.clusterName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if exists := s.insertContext(session.GetUser(), sctx); exists {
		// this means that someone has just inserted the context, so
		// close our extra context and return
		sctx.Close()
	}

	return sctx, nil
}

func (s *sessionCache) tlsConfig(ctx context.Context, cert, privKey []byte) (*tls.Config, error) {
	ca, err := s.proxyClient.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: s.clusterName,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certPool, err := services.CertPool(ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig := utils.TLSConfig(s.cipherSuites)
	tlsCert, err := tls.X509KeyPair(cert, privKey)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse TLS certificate and key")
	}
	tlsConfig.Certificates = []tls.Certificate{tlsCert}
	tlsConfig.RootCAs = certPool
	tlsConfig.ServerName = apiutils.EncodeClusterName(s.clusterName)
	tlsConfig.Time = s.clock.Now
	return tlsConfig, nil
}

func (s *sessionCache) readSession(ctx context.Context, req types.GetWebSessionRequest) (types.WebSession, error) {
	// Read session from the cache first
	session, err := s.accessPoint.GetWebSession(ctx, req)
	if err == nil {
		return session, nil
	}
	// Fallback to proxy otherwise
	return s.proxyClient.GetWebSession(ctx, req)
}

func (s *sessionCache) readBearerToken(ctx context.Context, req types.GetWebTokenRequest) (types.WebToken, error) {
	// Read token from the cache first
	token, err := s.accessPoint.GetWebToken(ctx, req)
	if err == nil {
		return token, nil
	}
	// Fallback to proxy otherwise
	return s.proxyClient.GetWebToken(ctx, req)
}

// Close releases all underlying resources for the user session.
func (c *sessionResources) Close() error {
	closers := c.transferClosers()
	var errors []error
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

// sessionResources persists resources initiated by a web session
// but which might outlive the session.
type sessionResources struct {
	log *slog.Logger

	mu      sync.Mutex
	closers []io.Closer
}

// addClosers adds the specified closers to this context
func (c *sessionResources) addClosers(closers ...io.Closer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closers = append(c.closers, closers...)
}

// removeCloser removes the specified closer from this context
func (c *sessionResources) removeCloser(closer io.Closer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, cls := range c.closers {
		if cls == closer {
			c.closers = append(c.closers[:i], c.closers[i+1:]...)
			return
		}
	}
}

func (c *sessionResources) transferClosers() []io.Closer {
	c.mu.Lock()
	defer c.mu.Unlock()
	closers := c.closers
	c.closers = nil
	return closers
}

func sessionKey(user, sessionID string) string {
	return user + sessionID
}

// remoteClientCache stores remote clients keyed by site name while also keeping
// track of the actual remote site associated with the client (in case the
// remote site has changed). Safe for concurrent access. Closes all clients and
// wipes the cache on Close.
type remoteClientCache struct {
	sync.Mutex
	clients map[string]struct {
		authclient.ClientI
		reversetunnelclient.RemoteSite
	}
}

func (c *remoteClientCache) addRemoteClient(site reversetunnelclient.RemoteSite, remoteClient authclient.ClientI) error {
	c.Lock()
	defer c.Unlock()
	if c.clients == nil {
		c.clients = make(map[string]struct {
			authclient.ClientI
			reversetunnelclient.RemoteSite
		})
	}
	var err error
	if c.clients[site.GetName()].ClientI != nil {
		err = c.clients[site.GetName()].ClientI.Close()
	}
	c.clients[site.GetName()] = struct {
		authclient.ClientI
		reversetunnelclient.RemoteSite
	}{remoteClient, site}
	return err
}

func (c *remoteClientCache) getRemoteClient(site reversetunnelclient.RemoteSite) (authclient.ClientI, bool) {
	c.Lock()
	defer c.Unlock()
	remoteClt, ok := c.clients[site.GetName()]
	return remoteClt.ClientI, ok && remoteClt.RemoteSite == site
}

func (c *remoteClientCache) Close() error {
	c.Lock()
	defer c.Unlock()

	errors := make([]error, 0, len(c.clients))
	for _, clt := range c.clients {
		errors = append(errors, clt.ClientI.Close())
	}
	c.clients = nil

	return trace.NewAggregate(errors...)
}

// sessionIDStatus indicates whether the session ID was received from
// the server or not, and if not why
type sessionIDStatus int

const (
	// sessionIDReceived indicates the the session ID was received
	sessionIDReceived sessionIDStatus = iota + 1
	// sessionIDNotSent indicates that the server set the session ID
	// but didn't send it to us
	sessionIDNotSent
	// sessionIDNotModified indicates that the server used the session
	// ID that was set by us
	sessionIDNotModified
)

// prepareToReceiveSessionID configures the TeleportClient to listen for
// the server to send the session ID it's using. The returned function
// will return the current session ID from the server or a reason why
// one wasn't received.
func prepareToReceiveSessionID(ctx context.Context, log *slog.Logger, nc *client.NodeClient) func() (session.ID, sessionIDStatus) {
	// send the session ID received from the server
	var gotSessionID atomic.Bool
	sessionIDFromServer := make(chan session.ID, 1)
	nc.TC.OnChannelRequest = func(req *ssh.Request) *ssh.Request {
		// ignore unrelated requests and handle only the first session
		// ID request
		if req.Type != teleport.CurrentSessionIDRequest || gotSessionID.Load() {
			return req
		}

		sid, err := session.ParseID(string(req.Payload))
		if err != nil {
			log.WarnContext(ctx, "Unable to parse session ID", "error", err)
			return nil
		}

		if gotSessionID.CompareAndSwap(false, true) {
			sessionIDFromServer <- *sid
		}

		return nil
	}

	// If the session is about to close and we haven't received a session
	// ID yet, ask if the server even supports sending one. Send the
	// request in a new goroutine so session establishment won't be
	// blocked on making this request
	serverWillSetSessionID := make(chan bool, 1)
	go func() {
		resp, _, err := nc.Client.SendRequest(ctx, teleport.SessionIDQueryRequest, true, nil)
		if err != nil {
			log.WarnContext(ctx, "Failed to send session ID query request", "error", err)
			serverWillSetSessionID <- false
		} else {
			serverWillSetSessionID <- resp
		}
	}()

	return func() (session.ID, sessionIDStatus) {
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()

		for {
			select {
			case sessionID := <-sessionIDFromServer:
				return sessionID, sessionIDReceived
			case sessionIDIsComing := <-serverWillSetSessionID:
				if !sessionIDIsComing {
					return session.ID(""), sessionIDNotModified
				}
				// the server will send the session ID, continue
				// waiting for it
			case <-ctx.Done():
				return session.ID(""), sessionIDNotSent
			case <-timer.C:
				return session.ID(""), sessionIDNotSent
			}
		}
	}
}
