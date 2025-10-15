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

package reversetunnel

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/proxy/peer"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/srv/git"
	"github.com/gravitational/teleport/lib/srv/ingress"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var (
	remoteClustersStats = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: teleport.MetricRemoteClusters,
			Help: "Number of inbound connections from remote clusters",
		},
		[]string{"cluster"},
	)

	trustedClustersStats = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: teleport.MetricTrustedClusters,
			Help: "Number of outbound connections to remote clusters",
		},
		[]string{"cluster"},
	)

	prometheusCollectors = []prometheus.Collector{remoteClustersStats, trustedClustersStats}
)

// server is a "reverse tunnel server". it exposes the cluster capabilities
// (like access to a cluster's auth) to remote trusted clients
// (also known as 'reverse tunnel agents').
type server struct {
	sync.RWMutex
	Config

	// localAuthClient provides access to the full Auth Server API for the
	// local cluster.
	localAuthClient authclient.ClientI
	// localAccessPoint provides access to a cached subset of the Auth
	// Server API.
	localAccessPoint authclient.ProxyAccessPoint

	// srv is the "base class" i.e. the underlying SSH server
	srv     *sshutils.Server
	limiter *limiter.Limiter

	// leafClusters is the list of connected leaf clusters
	leafClusters []*leafCluster

	// localCluster is the local Teleport cluster.
	localCluster *localCluster

	// expectedLeafClusters is a map of clusters connected to peer proxies
	// via reverse tunnels
	expectedLeafClusters map[string]*expectedLeafClusters

	// cancel function will cancel the
	cancel context.CancelFunc

	// ctx is a context used for signaling and broadcast
	ctx context.Context

	// logger specifies the logger
	logger *slog.Logger

	// proxyWatcher monitors changes to the proxies
	// and broadcasts updates
	proxyWatcher *services.GenericWatcher[types.Server, readonly.Server]

	// offlineThreshold is how long to wait for a keep alive message before
	// marking a reverse tunnel connection as invalid.
	offlineThreshold time.Duration

	// proxySigner is used to sign PROXY headers to securely propagate client IP information
	proxySigner multiplexer.PROXYHeaderSigner

	// gitKeyManager manages keys for git proxies.
	gitKeyManager *git.KeyManager
}

// EICESigner is a function that is used to obatin an [ssh.Signer] for an EICE instance. The
// [ssh.Signer] is required for clients to be able to connect to the instance.
type EICESigner func(ctx context.Context, target types.Server, integration types.Integration, login, token string, ap cryptosuites.AuthPreferenceGetter) (ssh.Signer, error)

// EICEDialer is a function that is used to dial and obtain a connection to an EICE instance.
type EICEDialer func(ctx context.Context, target types.Server, integration types.Integration, token string) (net.Conn, error)

// Config is a reverse tunnel server configuration
type Config struct {
	// ID is the ID of this server proxy
	ID string
	// ClusterName is a name of this cluster
	ClusterName string
	// ClientTLSCipherSuites optionally contains a list of TLS ciphersuites to
	// use when connecting to other clusters.
	ClientTLSCipherSuites []uint16
	// GetClientTLSCertificate returns a TLS certificate to use when connecting
	// to other clusters.
	GetClientTLSCertificate utils.GetCertificateFunc
	// Listener is a listener address for reverse tunnel server
	Listener net.Listener
	// HostSigners is a list of host signers
	GetHostSigners sshutils.GetHostSignersFunc
	// HostKeyCallback
	// Limiter is optional request limiter
	Limiter *limiter.Limiter
	// LocalAuthClient provides access to a full AuthClient for the local cluster.
	LocalAuthClient authclient.ClientI
	// AccessPoint provides access to a subset of AuthClient of the cluster.
	// AccessPoint caches values and can still return results during connection
	// problems.
	LocalAccessPoint authclient.ProxyAccessPoint
	// NewCachingAccessPoint returns new caching access points
	// per leaf cluster
	NewCachingAccessPoint authclient.NewRemoteProxyCachingAccessPoint
	// Context is a signaling context
	Context context.Context
	// Clock is a clock used in the server, set up to
	// wall clock if not set
	Clock clockwork.Clock

	// KeyGen is a process wide key generator. It is shared to speed up
	// generation of public/private keypairs.
	KeyGen sshca.Authority

	// Ciphers is a list of ciphers that the server supports. If omitted,
	// the defaults will be used.
	Ciphers []string

	// KEXAlgorithms is a list of key exchange (KEX) algorithms that the
	// server supports. If omitted, the defaults will be used.
	KEXAlgorithms []string

	// MACAlgorithms is a list of message authentication codes (MAC) that
	// the server supports. If omitted the defaults will be used.
	MACAlgorithms []string

	// DataDir is a local server data directory
	DataDir string

	// PollingPeriod specifies polling period for internal sync
	// goroutines, used to speed up sync-ups in tests.
	PollingPeriod time.Duration

	// Component is a component used in logs
	Component string

	// Logger specifies the logger
	Logger *slog.Logger

	// FIPS means Teleport was started in a FedRAMP/FIPS 140-2 compliant
	// configuration.
	FIPS bool

	// Emitter is event emitter
	Emitter events.StreamEmitter

	// PeerClient is a client to peer proxy servers.
	PeerClient *peer.Client

	// LockWatcher is a lock watcher.
	LockWatcher *services.LockWatcher

	// NodeWatcher is a node watcher.
	NodeWatcher *services.GenericWatcher[types.Server, readonly.Server]

	// GitServerWatcher is a Git server watcher.
	GitServerWatcher *services.GenericWatcher[types.Server, readonly.Server]

	// CertAuthorityWatcher is a cert authority watcher.
	CertAuthorityWatcher *services.CertAuthorityWatcher

	// CircuitBreakerConfig configures the auth client circuit breaker
	CircuitBreakerConfig breaker.Config

	// LocalAuthAddresses is a list of auth servers to use when dialing back to
	// the local cluster.
	LocalAuthAddresses []string

	// IngressReporter reports new and active connections.
	IngressReporter *ingress.Reporter

	// PROXYSigner is used to sign PROXY headers to securely propagate client IP information.
	PROXYSigner multiplexer.PROXYHeaderSigner

	// EICEDialer is used to open a connection to an EICE instance.
	EICEDialer EICEDialer
	// EICESigner is used to generate and upload credentials to an EICE instance.
	EICESigner EICESigner
}

// CheckAndSetDefaults checks parameters and sets default values
func (cfg *Config) CheckAndSetDefaults() error {
	if cfg.ID == "" {
		return trace.BadParameter("missing parameter ID")
	}
	if cfg.ClusterName == "" {
		return trace.BadParameter("missing parameter ClusterName")
	}
	if cfg.GetClientTLSCertificate == nil {
		return trace.BadParameter("missing parameter GetClientTLSCertificate")
	}
	if cfg.Listener == nil {
		return trace.BadParameter("missing parameter Listener")
	}
	if cfg.DataDir == "" {
		return trace.BadParameter("missing parameter DataDir")
	}
	if cfg.Emitter == nil {
		return trace.BadParameter("missing parameter Emitter")
	}
	if cfg.Context == nil {
		cfg.Context = context.TODO()
	}
	if cfg.PollingPeriod == 0 {
		cfg.PollingPeriod = defaults.HighResPollingPeriod
	}
	if cfg.Limiter == nil {
		var err error
		cfg.Limiter, err = limiter.NewLimiter(limiter.Config{})
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.Component == "" {
		cfg.Component = teleport.Component(teleport.ComponentProxy, teleport.ComponentServer)
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	cfg.Logger = cfg.Logger.With(teleport.ComponentKey, cfg.Component)

	if cfg.LockWatcher == nil {
		return trace.BadParameter("missing parameter LockWatcher")
	}
	if cfg.NodeWatcher == nil {
		return trace.BadParameter("missing parameter NodeWatcher")
	}
	if cfg.GitServerWatcher == nil {
		return trace.BadParameter("missing parameter GitServerWatcher")
	}
	if cfg.CertAuthorityWatcher == nil {
		return trace.BadParameter("missing parameter CertAuthorityWatcher")
	}

	if cfg.EICEDialer == nil {
		return trace.BadParameter("missing parameter EICEDialer")
	}
	if cfg.EICESigner == nil {
		return trace.BadParameter("missing parameter EICESigner")
	}
	return nil
}

// NewServer creates and returns a reverse tunnel server which is fully
// initialized but hasn't been started yet
func NewServer(cfg Config) (reversetunnelclient.Server, error) {
	err := metrics.RegisterPrometheusCollectors(prometheusCollectors...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	netConfig, err := cfg.LocalAccessPoint.GetClusterNetworkingConfig(cfg.Context)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	offlineThreshold := time.Duration(netConfig.GetKeepAliveCountMax()) * netConfig.GetKeepAliveInterval()

	ctx, cancel := context.WithCancel(cfg.Context)

	proxyWatcher, err := services.NewProxyWatcher(ctx, services.ProxyWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: cfg.Component,
			Client:    cfg.LocalAccessPoint,
			Logger:    cfg.Logger,
		},
		ProxiesC:    make(chan []types.Server, 10),
		ProxyGetter: cfg.LocalAccessPoint,
	})
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	gitKeyManager, err := git.NewKeyManager(&git.KeyManagerConfig{
		ParentContext: ctx,
		AuthClient:    cfg.LocalAuthClient,
		AccessPoint:   cfg.LocalAccessPoint,
	})
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	srv := &server{
		Config:               cfg,
		localAuthClient:      cfg.LocalAuthClient,
		localAccessPoint:     cfg.LocalAccessPoint,
		limiter:              cfg.Limiter,
		ctx:                  ctx,
		cancel:               cancel,
		proxyWatcher:         proxyWatcher,
		expectedLeafClusters: make(map[string]*expectedLeafClusters),
		logger:               cfg.Logger,
		offlineThreshold:     offlineThreshold,
		proxySigner:          cfg.PROXYSigner,
		gitKeyManager:        gitKeyManager,
	}

	srv.localCluster, err = newLocalCluster(srv, cfg.ClusterName, cfg.LocalAuthAddresses)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s, err := sshutils.NewServer(
		teleport.ComponentReverseTunnelServer,
		// TODO(klizhentas): improve interface, use struct instead of parameter list
		// this address is not used
		utils.NetAddr{Addr: "127.0.0.1:1", AddrNetwork: "tcp"},
		srv,
		cfg.GetHostSigners,
		sshutils.AuthMethods{
			PublicKey: srv.keyAuth,
		},
		sshutils.SetLogger(cfg.Logger),
		sshutils.SetLimiter(cfg.Limiter),
		sshutils.SetCiphers(cfg.Ciphers),
		sshutils.SetKEXAlgorithms(cfg.KEXAlgorithms),
		sshutils.SetMACAlgorithms(cfg.MACAlgorithms),
		sshutils.SetFIPS(cfg.FIPS),
		sshutils.SetClock(cfg.Clock),
		sshutils.SetIngressReporter(ingress.Tunnel, cfg.IngressReporter),
	)
	if err != nil {
		return nil, err
	}
	srv.srv = s
	go srv.periodicFunctions()
	return srv, nil
}

func remoteClustersMap(rc []types.RemoteCluster) map[string]types.RemoteCluster {
	out := make(map[string]types.RemoteCluster, len(rc))
	for i := range rc {
		out[rc[i].GetName()] = rc[i]
	}
	return out
}

// disconnectClusters disconnects reverse tunnel connections from leaf clusters
// that were deleted from the local cluster side and cleans up in memory objects.
// In this case all local trust has been deleted, so all the tunnel connections have to be dropped.
func (s *server) disconnectClusters(connectedLeafClusters []*leafCluster, remoteMap map[string]types.RemoteCluster) error {
	for _, cluster := range connectedLeafClusters {
		if _, ok := remoteMap[cluster.GetName()]; !ok {
			s.logger.InfoContext(s.ctx, "Leaf cluster has been deleted, disconnecting it from the proxy", "leaf_cluster", cluster.GetName())
			if err := s.onClusterTunnelClose(&alwaysClose{Cluster: cluster}); err != nil {
				s.logger.DebugContext(s.ctx, "Failure closing cluster", "leaf_cluster", cluster.GetName(), "error", err)
			}
			remoteClustersStats.DeleteLabelValues(cluster.GetName())
		}
	}
	return nil
}

func (s *server) periodicFunctions() {
	ticker := time.NewTicker(defaults.ResyncInterval)
	defer ticker.Stop()

	if err := s.fetchExpectedLeafClusters(); err != nil {
		s.logger.WarnContext(s.Context, "Failed to fetch expected leaf cluster", "error", err)
	}
	for {
		select {
		case <-s.ctx.Done():
			s.logger.DebugContext(s.ctx, "Closing")
			return
		// Proxies have been updated, notify connected agents about the update.
		case proxies := <-s.proxyWatcher.ResourcesC:
			s.fanOutProxies(proxies)
		case <-ticker.C:
			if err := s.fetchExpectedLeafClusters(); err != nil {
				s.logger.WarnContext(s.ctx, "Failed to fetch expected leaf clusters", "error", err)
			}

			connectedRemoteClusters := s.getLeafClusters()

			remoteClusters, err := s.localAccessPoint.GetRemoteClusters(s.ctx)
			if err != nil {
				s.logger.WarnContext(s.ctx, "Failed to get remote clusters", "error", err)
			}

			remoteMap := remoteClustersMap(remoteClusters)

			if err := s.disconnectClusters(connectedRemoteClusters, remoteMap); err != nil {
				s.logger.WarnContext(s.ctx, "Failed to disconnect clusters", "error", err)
			}

			if err := s.reportClusterStats(connectedRemoteClusters, remoteMap); err != nil {
				s.logger.WarnContext(s.ctx, "Failed to report cluster stats", "error", err)
			}
		}
	}
}

// fetchExpectedLeafClusters pulls back all proxies that have registered themselves
// (created a services.TunnelConnection) in the backend and compares them to
// what was found in the previous iteration and updates the in-memory cluster
// placeholders. This map is used later by Cluster(s) to return either local or
// leaf cluster, or if no match, a placeholder.
func (s *server) fetchExpectedLeafClusters() error {
	conns, err := s.LocalAccessPoint.GetAllTunnelConnections()
	if err != nil {
		return trace.Wrap(err)
	}
	newConns := make(map[string]types.TunnelConnection)
	for i := range conns {
		newConn := conns[i]
		// Filter out non-proxy tunnels.
		if newConn.GetType() != types.ProxyTunnel {
			continue
		}
		// Filter out peer records for own proxy.
		if newConn.GetProxyName() == s.ID {
			continue
		}

		// Filter out tunnels which are not online.
		if services.TunnelConnectionStatus(s.Clock, newConn, s.offlineThreshold) != teleport.RemoteClusterStatusOnline {
			continue
		}

		newConns[newConn.GetName()] = newConn
	}
	existingConns := s.existingConns()
	connsToAdd, connsToUpdate, connsToRemove := s.diffConns(newConns, existingConns)
	s.removeExpectedLeafCluster(connsToRemove)
	s.updateExpectedLeafClusters(connsToUpdate)
	return s.addExpectedLeafClusters(connsToAdd)
}

func (s *server) reportClusterStats(connectedRemoteClusters []*leafCluster, remoteMap map[string]types.RemoteCluster) error {
	// zero out counters for remote clusters that have a
	// resource in the backend but no associated cluster
	for cluster := range remoteMap {
		var exists bool
		for _, connectedCluster := range connectedRemoteClusters {
			if connectedCluster.GetName() == cluster {
				exists = true
				break
			}
		}

		if !exists {
			remoteClustersStats.WithLabelValues(cluster).Set(0)
		}
	}

	// update the counters for any remote clusters that have
	// both a resource in the backend AND an associated leaf cluster
	// with connections
	for _, cluster := range connectedRemoteClusters {
		rc, ok := remoteMap[cluster.GetName()]
		if !ok {
			remoteClustersStats.WithLabelValues(cluster.GetName()).Set(0)
			continue
		}

		if rc.GetConnectionStatus() == teleport.RemoteClusterStatusOnline && cluster.GetStatus() == teleport.RemoteClusterStatusOnline {
			remoteClustersStats.WithLabelValues(cluster.GetName()).Set(float64(cluster.GetTunnelsCount()))
		} else {
			remoteClustersStats.WithLabelValues(cluster.GetName()).Set(0)
		}
	}

	return nil
}

func (s *server) addExpectedLeafClusters(conns map[string]types.TunnelConnection) error {
	for key := range conns {
		connInfo := conns[key]
		s.addExpectedLeafCluster(newExpectedLeafCluster(s, connInfo, s.offlineThreshold))
	}
	return nil
}

func (s *server) updateExpectedLeafClusters(conns map[string]types.TunnelConnection) {
	for key := range conns {
		connInfo := conns[key]
		s.updateExpectedLeafCluster(connInfo)
	}
}

func (s *server) addExpectedLeafCluster(cluster *expectedLeafCluster) {
	s.Lock()
	defer s.Unlock()
	clusterName := cluster.connInfo.GetClusterName()
	clusters, ok := s.expectedLeafClusters[clusterName]
	if !ok {
		clusters = newExpectedLeafClusters(clusterName)
		s.expectedLeafClusters[clusterName] = clusters
	}
	clusters.addCluster(cluster)
}

func (s *server) updateExpectedLeafCluster(conn types.TunnelConnection) bool {
	s.Lock()
	defer s.Unlock()
	clusterName := conn.GetClusterName()
	clusters, ok := s.expectedLeafClusters[clusterName]
	if !ok {
		return false
	}
	return clusters.updateCluster(conn)
}

func (s *server) removeExpectedLeafCluster(conns []types.TunnelConnection) {
	s.Lock()
	defer s.Unlock()
	for _, conn := range conns {
		clusters, ok := s.expectedLeafClusters[conn.GetClusterName()]
		if !ok {
			s.logger.WarnContext(s.ctx, "failed to remove missing expected leaf cluster", "tunnel_connection", logutils.StringerAttr(conn))
			continue
		}
		clusters.removeCluster(conn)
		s.logger.DebugContext(s.ctx, "Removed expected cluster", "tunnel_connection", logutils.StringerAttr(conn))
	}
}

func (s *server) existingConns() map[string]types.TunnelConnection {
	s.RLock()
	defer s.RUnlock()
	conns := make(map[string]types.TunnelConnection)
	for _, clusters := range s.expectedLeafClusters {
		for _, cluster := range clusters.clusters {
			conns[cluster.connInfo.GetName()] = cluster.connInfo
		}
	}
	return conns
}

func (s *server) diffConns(newConns, existingConns map[string]types.TunnelConnection) (map[string]types.TunnelConnection, map[string]types.TunnelConnection, []types.TunnelConnection) {
	connsToAdd := make(map[string]types.TunnelConnection)
	connsToUpdate := make(map[string]types.TunnelConnection)
	var connsToRemove []types.TunnelConnection

	for existingKey := range existingConns {
		conn := existingConns[existingKey]
		if _, ok := newConns[existingKey]; !ok { // tunnel was removed
			connsToRemove = append(connsToRemove, conn)
		}
	}

	for newKey := range newConns {
		conn := newConns[newKey]
		if _, ok := existingConns[newKey]; !ok { // tunnel was added
			connsToAdd[newKey] = conn
		} else {
			connsToUpdate[newKey] = conn
		}
	}

	return connsToAdd, connsToUpdate, connsToRemove
}

func (s *server) Wait(ctx context.Context) {
	s.srv.Wait(ctx)
}

func (s *server) Start() error {
	go s.srv.Serve(s.Listener)
	return nil
}

func (s *server) Close() error {
	s.cancel()
	s.proxyWatcher.Close()
	return s.srv.Close()
}

// DrainConnections closes the listener and sends reconnects to connected agents without
// closing open connections.
func (s *server) DrainConnections(ctx context.Context) error {
	// Ensure listener is closed before sending reconnects.
	err := s.srv.Close()
	s.RLock()
	s.logger.DebugContext(ctx, "Advising reconnect to local cluster", "local_cluster", s.localCluster.GetName())
	go s.localCluster.adviseReconnect(ctx)

	for _, cluster := range s.leafClusters {
		s.logger.DebugContext(ctx, "Advising reconnect to leaf cluster", "leaf_cluster", cluster.GetName())
		go cluster.adviseReconnect(ctx)
	}
	s.RUnlock()

	s.srv.Wait(ctx)
	return trace.Wrap(err)
}

func (s *server) Shutdown(ctx context.Context) error {
	err := s.srv.Shutdown(ctx)

	s.proxyWatcher.Close()
	s.cancel()

	return trace.Wrap(err)
}

func (s *server) HandleNewChan(ctx context.Context, ccx *sshutils.ConnectionContext, nch ssh.NewChannel) {
	conn := ccx.NetConn
	sconn := ccx.ServerConn

	channelType := nch.ChannelType()
	switch channelType {
	// Heartbeats can come from nodes or proxies.
	case chanHeartbeat:
		s.handleHeartbeat(ctx, conn, sconn, nch)
	// Transport requests come from nodes requesting a connection to the Auth
	// Server through the reverse tunnel.
	case constants.ChanTransport:
		s.handleTransport(sconn, nch)
	default:
		msg := fmt.Sprintf("reversetunnel received unknown channel request %v from %v",
			nch.ChannelType(), sconn)

		// If someone is trying to open a new SSH session by talking to a reverse
		// tunnel, they're most likely using the wrong port number. Give them an
		// explicit hint.
		if channelType == "session" {
			msg = "Cannot open new SSH session on reverse tunnel. Are you connecting to the right port?"
		}
		//nolint:sloglint // message should be a constant but in this case we are creating it at runtime.
		s.logger.WarnContext(ctx, msg)
		s.rejectRequest(nch, ssh.ConnectionFailed, msg)
		return
	}
}

func (s *server) handleTransport(sconn *ssh.ServerConn, nch ssh.NewChannel) {
	s.logger.DebugContext(s.ctx, "Received transport request")
	channel, requestC, err := nch.Accept()
	if err != nil {
		sconn.Close()
		// avoid WithError to reduce log spam on network errors
		s.logger.WarnContext(s.ctx, "Failed to accept request", "error", err)
		return
	}

	go s.handleTransportChannel(sconn, channel, requestC)
}

func (s *server) handleTransportChannel(sconn *ssh.ServerConn, ch ssh.Channel, reqC <-chan *ssh.Request) {
	defer ch.Close()
	go io.Copy(io.Discard, ch.Stderr())

	// the only valid teleport-transport-dial request here is to reach the auth server
	var req *ssh.Request
	select {
	case <-s.ctx.Done():
		go ssh.DiscardRequests(reqC)
		return
	case <-time.After(apidefaults.DefaultIOTimeout):
		go ssh.DiscardRequests(reqC)
		s.logger.WarnContext(s.ctx, "Timed out waiting for transport dial request")
		return
	case r, ok := <-reqC:
		if !ok {
			return
		}
		go ssh.DiscardRequests(reqC)
		req = r
	}

	dialReq := parseDialReq(req.Payload)
	if dialReq.Address != constants.RemoteAuthServer {
		s.logger.WarnContext(s.ctx, "Received dial request for unexpected address, routing to the auth server anyway", "address", dialReq.Address)
	}

	authAddress := utils.ChooseRandomString(s.LocalAuthAddresses)
	if authAddress == "" {
		s.logger.ErrorContext(s.ctx, "No auth servers configured")
		fmt.Fprint(ch.Stderr(), "internal server error")
		req.Reply(false, nil)
		return
	}

	var proxyHeader []byte
	clientSrcAddr := sconn.RemoteAddr()
	clientDstAddr := sconn.LocalAddr()
	if s.proxySigner != nil && clientSrcAddr != nil && clientDstAddr != nil {
		h, err := s.proxySigner.SignPROXYHeader(clientSrcAddr, clientDstAddr)
		if err != nil {
			s.logger.ErrorContext(s.ctx, "Failed to create signed PROXY header", "error", err)
			fmt.Fprint(ch.Stderr(), "internal server error")
			req.Reply(false, nil)
		}
		proxyHeader = h
	}

	d := net.Dialer{Timeout: apidefaults.DefaultIOTimeout}
	conn, err := d.DialContext(s.ctx, "tcp", authAddress)
	if err != nil {
		s.logger.ErrorContext(s.ctx, "Failed to dial auth", "error", err)
		fmt.Fprint(ch.Stderr(), "failed to dial auth server")
		req.Reply(false, nil)
		return
	}
	defer conn.Close()

	_ = conn.SetWriteDeadline(time.Now().Add(apidefaults.DefaultIOTimeout))
	if _, err := conn.Write(proxyHeader); err != nil {
		s.logger.ErrorContext(s.ctx, "Failed to send PROXY header", "error", err)
		fmt.Fprint(ch.Stderr(), "failed to dial auth server")
		req.Reply(false, nil)
		return
	}
	_ = conn.SetWriteDeadline(time.Time{})

	if err := req.Reply(true, nil); err != nil {
		s.logger.ErrorContext(s.ctx, "Failed to respond to dial request", "error", err)
		return
	}

	_ = utils.ProxyConn(s.ctx, ch, conn)
}

// TODO(awly): unit test this
func (s *server) handleHeartbeat(ctx context.Context, conn net.Conn, sconn *ssh.ServerConn, nch ssh.NewChannel) {
	s.logger.DebugContext(ctx, "New tunnel established", "remote_addr", logutils.StringerAttr(sconn.RemoteAddr()))
	if sconn.Permissions.Extensions[utils.ExtIntCertType] != utils.ExtIntCertTypeHost {
		s.logger.ErrorContext(ctx, "can't retrieve certificate type in certtype@teleport extension")
		return
	}

	// Extract the role. For proxies, it's another cluster asking to join, for
	// nodes it's a node dialing back.
	val, ok := sconn.Permissions.Extensions[extCertRole]
	if !ok {
		s.logger.ErrorContext(ctx, "Failed to accept connection, missing role extension")
		s.rejectRequest(nch, ssh.ConnectionFailed, "unknown role")
		return
	}

	role := types.SystemRole(val)
	switch role {
	// Node is dialing back.
	case types.RoleNode:
		s.handleNewService(ctx, role, conn, sconn, nch, types.NodeTunnel)
	// App is dialing back.
	case types.RoleApp:
		s.handleNewService(ctx, role, conn, sconn, nch, types.AppTunnel)
	// Kubernetes service is dialing back.
	case types.RoleKube:
		s.handleNewService(ctx, role, conn, sconn, nch, types.KubeTunnel)
	// Database proxy is dialing back.
	case types.RoleDatabase:
		s.handleNewService(ctx, role, conn, sconn, nch, types.DatabaseTunnel)
	// Proxy is dialing back.
	case types.RoleProxy:
		s.handleNewCluster(ctx, conn, sconn, nch)
	case types.RoleWindowsDesktop:
		s.handleNewService(ctx, role, conn, sconn, nch, types.WindowsDesktopTunnel)
	case types.RoleOkta:
		s.handleNewService(ctx, role, conn, sconn, nch, types.OktaTunnel)
	// Unknown role.
	default:
		s.logger.ErrorContext(ctx, "Unsupported role attempting to connect", "role", val)
		s.rejectRequest(nch, ssh.ConnectionFailed, fmt.Sprintf("unsupported role %v", val))
	}
}

func (s *server) handleNewService(ctx context.Context, role types.SystemRole, conn net.Conn, sconn *ssh.ServerConn, nch ssh.NewChannel, connType types.TunnelType) {
	cluster, rconn, err := s.upsertServiceConn(conn, sconn, connType)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to upsert service connection", "role", role, "error", err)
		sconn.Close()
		return
	}

	ch, req, err := nch.Accept()
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to accept on channel", "error", err)
		sconn.Close()
		return
	}

	go cluster.handleHeartbeat(ctx, rconn, ch, req)
}

func (s *server) handleNewCluster(ctx context.Context, conn net.Conn, sshConn *ssh.ServerConn, nch ssh.NewChannel) {
	// add the incoming cluster to the list of active connections:
	cluster, remoteConn, err := s.upsertRemoteCluster(conn, sshConn)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to upsert remote cluster connection", "error", err)
		s.rejectRequest(nch, ssh.ConnectionFailed, "failed to accept incoming cluster connection")
		return
	}
	// accept the request and start the heartbeat on it:
	ch, req, err := nch.Accept()
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to accept on channel", "error", err)
		sshConn.Close()
		return
	}
	go cluster.handleHeartbeat(ctx, remoteConn, ch, req)
}

func (s *server) requireLocalAgentForConn(sconn *ssh.ServerConn, connType types.TunnelType) error {
	// Cluster name was extracted from certificate and packed into extensions.
	clusterName := sconn.Permissions.Extensions[extAuthority]
	if strings.TrimSpace(clusterName) == "" {
		return trace.BadParameter("empty cluster name")
	}

	if s.localCluster.domainName == clusterName {
		return nil
	}

	return trace.BadParameter("agent from cluster %s cannot register local service %s", clusterName, connType)
}

func (s *server) getTrustedCAKeysByID(id types.CertAuthID) ([]ssh.PublicKey, error) {
	ca, err := s.localAccessPoint.GetCertAuthority(context.TODO(), id, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sshutils.GetCheckers(ca)
}

func (s *server) keyAuth(conn ssh.ConnMetadata, key ssh.PublicKey) (perm *ssh.Permissions, err error) {
	logger := s.logger.With(
		"remote_addr", logutils.StringerAttr(conn.RemoteAddr()),
		"user", conn.User(),
	)
	// The crypto/x/ssh package won't log the returned error for us, do it
	// manually.
	defer func() {
		if err != nil {
			logger.WarnContext(s.ctx, "Failed to authenticate client", "error", err)
		}
	}()

	cert, ok := key.(*ssh.Certificate)
	if !ok {
		return nil, trace.BadParameter("server doesn't support provided key type")
	}

	ident, err := sshca.DecodeIdentity(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var clusterName, certRole, certType string
	var caType types.CertAuthType
	switch ident.CertType {
	case ssh.HostCert:
		if ident.ClusterName == "" {
			return nil, trace.BadParameter("certificate missing %q extension; this SSH host certificate was not issued by Teleport or issued by an older version of Teleport; try upgrading your Teleport nodes/proxies", utils.CertExtensionAuthority)
		}
		clusterName = ident.ClusterName

		if ident.SystemRole == "" {
			return nil, trace.BadParameter("certificate missing %q extension; this SSH host certificate was not issued by Teleport or issued by an older version of Teleport; try upgrading your Teleport nodes/proxies", utils.CertExtensionRole)
		}
		certRole = string(ident.SystemRole)
		certType = utils.ExtIntCertTypeHost
		caType = types.HostCA
	case ssh.UserCert:
		if ident.RouteToCluster != "" && ident.RouteToCluster != s.ClusterName {
			return nil, trace.BadParameter("this endpoint does not support cross-cluster routing (cannot route from %q to %q)", s.ClusterName, ident.RouteToCluster)
		}

		// only support certs signed by local user CA here. user certs don't encode the clustername of origin, but we can
		// effectively limit ourselves to only supporting local users by only checking the cert against local CAs.
		clusterName = s.ClusterName

		if len(ident.Roles) == 0 {
			return nil, trace.BadParameter("certificate missing roles in %q extension; make sure your user has some roles assigned (or ask your Teleport admin to) and log in again (or export an identity file, if that's what you used)", teleport.CertExtensionTeleportRoles)
		}
		certRole = ident.Roles[0]
		certType = utils.ExtIntCertTypeUser
		caType = types.UserCA
	default:
		return nil, trace.BadParameter("unsupported cert type: %v.", cert.CertType)
	}

	if err := s.checkClientCert(conn.User(), clusterName, cert, caType); err != nil {
		return nil, trace.Wrap(err)
	}
	return &ssh.Permissions{
		Extensions: map[string]string{
			extHost:              conn.User(),
			utils.ExtIntCertType: certType,
			extCertRole:          certRole,
			extAuthority:         clusterName,
		},
	}, nil
}

// checkClientCert verifies that client certificate is signed by the recognized
// certificate authority.
func (s *server) checkClientCert(user string, clusterName string, cert *ssh.Certificate, caType types.CertAuthType) error {
	// fetch keys of the certificate authority to check
	// if there is a match
	keys, err := s.getTrustedCAKeysByID(types.CertAuthID{
		Type:       caType,
		DomainName: clusterName,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// match key of the certificate authority with the signature key
	var match bool
	for _, k := range keys {
		if apisshutils.KeysEqual(k, cert.SignatureKey) {
			match = true
			break
		}
	}
	if !match {
		return trace.NotFound("cluster %v has no matching CA keys", clusterName)
	}

	checker := apisshutils.CertChecker{
		CertChecker: ssh.CertChecker{
			Clock: s.Clock.Now,
		},
		FIPS: s.FIPS,
	}
	if err := checker.CheckCert(user, cert); err != nil {
		return trace.BadParameter("%s", err)
	}

	return nil
}

func (s *server) upsertServiceConn(conn net.Conn, sconn *ssh.ServerConn, connType types.TunnelType) (*localCluster, *remoteConn, error) {
	s.Lock()
	defer s.Unlock()

	if err := s.requireLocalAgentForConn(sconn, connType); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	nodeID, ok := sconn.Permissions.Extensions[extHost]
	if !ok {
		return nil, nil, trace.BadParameter("host id not found")
	}

	rconn, err := s.localCluster.addConn(nodeID, connType, conn, sconn)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return s.localCluster, rconn, nil
}

func (s *server) upsertRemoteCluster(conn net.Conn, sshConn *ssh.ServerConn) (*leafCluster, *remoteConn, error) {
	domainName := sshConn.Permissions.Extensions[extAuthority]
	if strings.TrimSpace(domainName) == "" {
		return nil, nil, trace.BadParameter("cannot create reverse tunnel: empty cluster name")
	}

	s.Lock()
	defer s.Unlock()

	var cluster *leafCluster
	for _, st := range s.leafClusters {
		if st.domainName == domainName {
			cluster = st
			break
		}
	}
	var err error
	var remoteConn *remoteConn
	if cluster != nil {
		if remoteConn, err = cluster.addConn(conn, sshConn); err != nil {
			return nil, nil, trace.Wrap(err)
		}
	} else {
		cluster, err = newLeafCluster(s, domainName, sshConn.Conn)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		if remoteConn, err = cluster.addConn(conn, sshConn); err != nil {
			return nil, nil, trace.Wrap(err)
		}
		s.leafClusters = append(s.leafClusters, cluster)
	}
	cluster.logger.InfoContext(s.ctx, "Processed inbound connection from remote cluster",
		"source_addr", logutils.StringerAttr(conn.RemoteAddr()),
		"tunnel_count", len(s.leafClusters),
	)
	// treat first connection as a registered heartbeat,
	// otherwise the connection information will appear after initial
	// heartbeat delay
	go cluster.registerHeartbeat(s.Clock.Now())
	return cluster, remoteConn, nil
}

func (s *server) Clusters(context.Context) ([]reversetunnelclient.Cluster, error) {
	s.RLock()
	defer s.RUnlock()
	out := make([]reversetunnelclient.Cluster, 0, len(s.leafClusters)+len(s.expectedLeafClusters)+1)
	out = append(out, s.localCluster)

	haveLocalConnection := make(map[string]bool)
	for i := range s.leafClusters {
		cluster := s.leafClusters[i]
		haveLocalConnection[cluster.GetName()] = true
		out = append(out, cluster)
	}
	for i := range s.expectedLeafClusters {
		cluster := s.expectedLeafClusters[i]
		if _, ok := haveLocalConnection[cluster.GetName()]; !ok {
			out = append(out, cluster)
		}
	}
	return out, nil
}

func (s *server) getLeafClusters() []*leafCluster {
	s.RLock()
	defer s.RUnlock()
	out := make([]*leafCluster, len(s.leafClusters))
	copy(out, s.leafClusters)
	return out
}

// Cluster returns the [reversetunnelclient.Cluster] with the matching name. If
// the name does not match the local cluster or any connected leaf clusters, then a placeholder
// cluster is returned.
func (s *server) Cluster(_ context.Context, name string) (reversetunnelclient.Cluster, error) {
	s.RLock()
	defer s.RUnlock()
	if s.localCluster.GetName() == name {
		return s.localCluster, nil
	}
	for i := range s.leafClusters {
		if s.leafClusters[i].GetName() == name {
			return s.leafClusters[i], nil
		}
	}
	for i := range s.expectedLeafClusters {
		if s.expectedLeafClusters[i].GetName() == name {
			return s.expectedLeafClusters[i], nil
		}
	}
	return nil, trace.NotFound("cluster %q is not found", name)
}

// GetProxyPeerClient returns the proxy peer client
func (s *server) GetProxyPeerClient() *peer.Client {
	return s.PeerClient
}

// alwaysClose forces onClusterTunnelClose to remove and close
// the cluster by always returning false from HasValidConnections.
type alwaysClose struct {
	reversetunnelclient.Cluster
}

func (a *alwaysClose) HasValidConnections() bool {
	return false
}

// clusterCloser is used by onClusterTunnelClose to determine if a cluster should be closed
// when a tunnel is closed
type clusterCloser interface {
	GetName() string
	HasValidConnections() bool
	io.Closer
}

// onClusterTunnelClose will close and stop tracking the cluster with the given name
// if it has 0 active tunnels. This is done here to ensure that no new tunnels
// can be established while cleaning up a cluster.
func (s *server) onClusterTunnelClose(cluster clusterCloser) error {
	s.Lock()
	defer s.Unlock()

	if cluster.HasValidConnections() {
		return nil
	}

	for i := range s.leafClusters {
		if s.leafClusters[i].domainName == cluster.GetName() {
			s.leafClusters = append(s.leafClusters[:i], s.leafClusters[i+1:]...)
			return trace.Wrap(cluster.Close())
		}
	}

	return trace.NotFound("cluster %q is not found", cluster.GetName())
}

// fanOutProxies is a non-blocking call that updated the watches proxies
// list and notifies all clusters about the proxy list change
func (s *server) fanOutProxies(proxies []types.Server) {
	s.Lock()
	defer s.Unlock()
	s.localCluster.fanOutProxies(proxies)

	for _, cluster := range s.leafClusters {
		cluster.fanOutProxies(proxies)
	}
}

func (s *server) rejectRequest(ch ssh.NewChannel, reason ssh.RejectionReason, msg string) {
	if err := ch.Reject(reason, msg); err != nil {
		s.logger.WarnContext(s.ctx, "Failed rejecting new channel request", "error", err)
	}
}

// TrackUserConnection tracks a user connection that should prevent
// the server from being terminated if active. The returned function
// should be called when the connection is terminated.
func (s *server) TrackUserConnection() (release func()) {
	return s.srv.TrackUserConnection()
}

// newLeafCluster helper creates and initializes a leafCluster instance
func newLeafCluster(srv *server, domainName string, sconn ssh.Conn) (*leafCluster, error) {
	connInfo, err := types.NewTunnelConnection(
		fmt.Sprintf("%v-%v", srv.ID, domainName),
		types.TunnelConnectionSpecV2{
			ClusterName:   domainName,
			ProxyName:     srv.ID,
			LastHeartbeat: srv.Clock.Now().UTC(),
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connInfo.SetExpiry(srv.Clock.Now().Add(srv.offlineThreshold))

	closeContext, cancel := context.WithCancel(srv.ctx)
	defer func() {
		if err != nil {
			cancel()
		}
	}()
	leaf := &leafCluster{
		srv:        srv,
		domainName: domainName,
		connInfo:   connInfo,
		logger: slog.With(
			teleport.ComponentKey, teleport.ComponentReverseTunnelServer,
			"cluster", domainName,
		),
		ctx:               closeContext,
		cancel:            cancel,
		clock:             srv.Clock,
		offlineThreshold:  srv.offlineThreshold,
		proxySyncInterval: proxySyncInterval,
	}

	// configure access to the full Auth Server API and the cached subset for
	// the local cluster within which reversetunnelclient.Server is running.
	leaf.localClient = srv.localAuthClient
	leaf.localCache = srv.localAccessPoint

	clt, _, err := leaf.getLeafClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	leaf.leafClient = clt

	version, err := getLeafClusterAuthVersion(closeContext, sconn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessPoint, err := createLeafClusterCache(srv, clt, domainName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	leaf.leafCache = accessPoint
	nodeWatcher, err := services.NewNodeWatcher(closeContext, services.NodeWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component:    srv.Component,
			Client:       accessPoint,
			Logger:       srv.Logger,
			MaxStaleness: time.Minute,
		},
		NodesGetter: accessPoint,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	leaf.nodeWatcher = nodeWatcher
	// instantiate a cache of host certificates for the forwarding server. the
	// certificate cache is created in each cluster (instead of creating it in
	// reversetunnel.server and passing it along) so that the host certificate
	// is signed by the correct certificate authority.
	certificateCache, err := newHostCertificateCache(srv.localAuthClient, srv.localAccessPoint, srv.Clock)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	leaf.certificateCache = certificateCache

	caRetry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  retryutils.HalfJitter(srv.Config.PollingPeriod),
		Step:   srv.Config.PollingPeriod / 5,
		Max:    srv.Config.PollingPeriod,
		Jitter: retryutils.HalfJitter,
		Clock:  srv.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	leafClusterWatcher, err := services.NewCertAuthorityWatcher(srv.ctx, services.CertAuthorityWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentProxy,
			Logger:    srv.logger,
			Clock:     srv.Clock,
			Client:    leaf.leafCache,
		},
		Types: []types.CertAuthType{types.HostCA},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go func() {
		leaf.updateCertAuthorities(caRetry, leafClusterWatcher, version)
	}()

	lockRetry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  retryutils.HalfJitter(srv.Config.PollingPeriod),
		Step:   srv.Config.PollingPeriod / 5,
		Max:    srv.Config.PollingPeriod,
		Jitter: retryutils.HalfJitter,
		Clock:  srv.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go leaf.updateLocks(lockRetry)
	return leaf, nil
}

// createLeafClusterCache creates a new cache for the leaf cluster.
func createLeafClusterCache(srv *server, clt authclient.ClientI, domainName string) (authclient.RemoteProxyAccessPoint, error) {
	// Configure access to the cached subset of the Auth Server API of the leaf
	// cluster this leaf cluster provides access to.
	accessPoint, err := srv.Config.NewCachingAccessPoint(clt, []string{"reverse", domainName})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return accessPoint, nil
}

// getLeafClusterAuthVersion sends a version request to the leaf cluster.
func getLeafClusterAuthVersion(ctx context.Context, sconn ssh.Conn) (string, error) {
	errorCh := make(chan error, 1)
	versionCh := make(chan string, 1)

	go func() {
		ok, payload, err := sconn.SendRequest(versionRequest, true, nil)
		if err != nil {
			errorCh <- err
			return
		}
		if !ok {
			errorCh <- trace.BadParameter("no response to %v request", versionRequest)
			return
		}

		versionCh <- string(payload)
	}()

	select {
	case ver := <-versionCh:
		return ver, nil
	case err := <-errorCh:
		return "", trace.Wrap(err)
	case <-time.After(defaults.WaitCopyTimeout):
		return "", trace.BadParameter("timeout waiting for version")
	case <-ctx.Done():
		return "", trace.Wrap(ctx.Err())
	}
}

const (
	extHost      = "host@teleport"
	extAuthority = "auth@teleport"
	extCertRole  = "role"

	versionRequest = "x-teleport-version"
)
