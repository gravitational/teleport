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
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/proxy/peer"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/ingress"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
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

	// remoteSites is the list of connected remote clusters
	remoteSites []*remoteSite

	// localSite is the  local (our own cluster) tunnel client.
	localSite *localSite

	// clusterPeers is a map of clusters connected to peer proxies
	// via reverse tunnels
	clusterPeers map[string]*clusterPeers

	// cancel function will cancel the
	cancel context.CancelFunc

	// ctx is a context used for signaling and broadcast
	ctx context.Context

	// log specifies the logger
	log log.FieldLogger

	// proxyWatcher monitors changes to the proxies
	// and broadcasts updates
	proxyWatcher *services.ProxyWatcher

	// offlineThreshold is how long to wait for a keep alive message before
	// marking a reverse tunnel connection as invalid.
	offlineThreshold time.Duration

	// proxySigner is used to sign PROXY headers to securely propagate client IP information
	proxySigner multiplexer.PROXYHeaderSigner
}

// Config is a reverse tunnel server configuration
type Config struct {
	// ID is the ID of this server proxy
	ID string
	// ClusterName is a name of this cluster
	ClusterName string
	// ClientTLS is a TLS config associated with this proxy
	// used to connect to remote auth servers on remote clusters
	ClientTLS *tls.Config
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
	// per remote cluster
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

	// Log specifies the logger
	Log log.FieldLogger

	// FIPS means Teleport was started in a FedRAMP/FIPS 140-2 compliant
	// configuration.
	FIPS bool

	// Emitter is event emitter
	Emitter events.StreamEmitter

	// DELETE IN: 8.0.0
	//
	// NewCachingAccessPointOldProxy is an access point that can be configured
	// with the old access point policy until all clusters are migrated to 7.0.0
	// and above.
	NewCachingAccessPointOldProxy authclient.NewRemoteProxyCachingAccessPoint

	// PeerClient is a client to peer proxy servers.
	PeerClient *peer.Client

	// LockWatcher is a lock watcher.
	LockWatcher *services.LockWatcher

	// NodeWatcher is a node watcher.
	NodeWatcher *services.NodeWatcher

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
}

// CheckAndSetDefaults checks parameters and sets default values
func (cfg *Config) CheckAndSetDefaults() error {
	if cfg.ID == "" {
		return trace.BadParameter("missing parameter ID")
	}
	if cfg.ClusterName == "" {
		return trace.BadParameter("missing parameter ClusterName")
	}
	if cfg.ClientTLS == nil {
		return trace.BadParameter("missing parameter ClientTLS")
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
	logger := cfg.Log
	if cfg.Log == nil {
		logger = log.StandardLogger()
	}
	cfg.Log = logger.WithFields(log.Fields{
		teleport.ComponentKey: cfg.Component,
	})
	if cfg.LockWatcher == nil {
		return trace.BadParameter("missing parameter LockWatcher")
	}
	if cfg.NodeWatcher == nil {
		return trace.BadParameter("missing parameter NodeWatcher")
	}
	if cfg.CertAuthorityWatcher == nil {
		return trace.BadParameter("missing parameter CertAuthorityWatcher")
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
			Log:       cfg.Log,
		},
		ProxiesC:    make(chan []types.Server, 10),
		ProxyGetter: cfg.LocalAccessPoint,
		ProxyDiffer: func(_, _ types.Server) bool {
			return true // we always want to store the most recently heartbeated proxy
		},
	})
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	srv := &server{
		Config:           cfg,
		localAuthClient:  cfg.LocalAuthClient,
		localAccessPoint: cfg.LocalAccessPoint,
		limiter:          cfg.Limiter,
		ctx:              ctx,
		cancel:           cancel,
		proxyWatcher:     proxyWatcher,
		clusterPeers:     make(map[string]*clusterPeers),
		log:              cfg.Log,
		offlineThreshold: offlineThreshold,
		proxySigner:      cfg.PROXYSigner,
	}

	localSite, err := newLocalSite(srv, cfg.ClusterName, cfg.LocalAuthAddresses)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	srv.localSite = localSite

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
		sshutils.SetLogger(cfg.Log),
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

// disconnectClusters disconnects reverse tunnel connections from remote clusters
// that were deleted from the local cluster side and cleans up in memory objects.
// In this case all local trust has been deleted, so all the tunnel connections have to be dropped.
func (s *server) disconnectClusters(connectedRemoteClusters []*remoteSite, remoteMap map[string]types.RemoteCluster) error {
	for _, cluster := range connectedRemoteClusters {
		if _, ok := remoteMap[cluster.GetName()]; !ok {
			s.log.Infof("Remote cluster %q has been deleted. Disconnecting it from the proxy.", cluster.GetName())
			if err := s.onSiteTunnelClose(&alwaysClose{RemoteSite: cluster}); err != nil {
				s.log.Debugf("Failure closing cluster %q: %v.", cluster.GetName(), err)
			}
			remoteClustersStats.DeleteLabelValues(cluster.GetName())
		}
	}
	return nil
}

func (s *server) periodicFunctions() {
	ticker := time.NewTicker(defaults.ResyncInterval)
	defer ticker.Stop()

	if err := s.fetchClusterPeers(); err != nil {
		s.log.Warningf("Failed to fetch cluster peers: %v.", err)
	}
	for {
		select {
		case <-s.ctx.Done():
			s.log.Debugf("Closing.")
			return
		// Proxies have been updated, notify connected agents about the update.
		case proxies := <-s.proxyWatcher.ProxiesC:
			s.fanOutProxies(proxies)
		case <-ticker.C:
			if err := s.fetchClusterPeers(); err != nil {
				s.log.WithError(err).Warn("Failed to fetch cluster peers")
			}

			connectedRemoteClusters := s.getRemoteClusters()

			remoteClusters, err := s.localAccessPoint.GetRemoteClusters()
			if err != nil {
				s.log.WithError(err).Warn("Failed to get remote clusters")
			}

			remoteMap := remoteClustersMap(remoteClusters)

			if err := s.disconnectClusters(connectedRemoteClusters, remoteMap); err != nil {
				s.log.Warningf("Failed to disconnect clusters: %v.", err)
			}

			if err := s.reportClusterStats(connectedRemoteClusters, remoteMap); err != nil {
				s.log.Warningf("Failed to report cluster stats: %v.", err)
			}
		}
	}
}

// fetchClusterPeers pulls back all proxies that have registered themselves
// (created a services.TunnelConnection) in the backend and compares them to
// what was found in the previous iteration and updates the in-memory cluster
// peer map. This map is used later by GetSite(s) to return either local or
// remote site, or if no match, a cluster peer.
func (s *server) fetchClusterPeers() error {
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
	s.removeClusterPeers(connsToRemove)
	s.updateClusterPeers(connsToUpdate)
	return s.addClusterPeers(connsToAdd)
}

func (s *server) reportClusterStats(connectedRemoteClusters []*remoteSite, remoteMap map[string]types.RemoteCluster) error {
	// zero out counters for remote clusters that have a
	// resource in the backend but no associated remoteSite
	for cluster := range remoteMap {
		var exists bool
		for _, site := range connectedRemoteClusters {
			if site.GetName() == cluster {
				exists = true
				break
			}
		}

		if !exists {
			remoteClustersStats.WithLabelValues(cluster).Set(0)
		}
	}

	// update the counters for any remote clusters that have
	// both a resource in the backend AND an associated remote
	// site with connections
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

func (s *server) addClusterPeers(conns map[string]types.TunnelConnection) error {
	for key := range conns {
		connInfo := conns[key]
		peer, err := newClusterPeer(s, connInfo, s.offlineThreshold)
		if err != nil {
			return trace.Wrap(err)
		}
		s.addClusterPeer(peer)
	}
	return nil
}

func (s *server) updateClusterPeers(conns map[string]types.TunnelConnection) {
	for key := range conns {
		connInfo := conns[key]
		s.updateClusterPeer(connInfo)
	}
}

func (s *server) addClusterPeer(peer *clusterPeer) {
	s.Lock()
	defer s.Unlock()
	clusterName := peer.connInfo.GetClusterName()
	peers, ok := s.clusterPeers[clusterName]
	if !ok {
		peers = newClusterPeers(clusterName)
		s.clusterPeers[clusterName] = peers
	}
	peers.addPeer(peer)
}

func (s *server) updateClusterPeer(conn types.TunnelConnection) bool {
	s.Lock()
	defer s.Unlock()
	clusterName := conn.GetClusterName()
	peers, ok := s.clusterPeers[clusterName]
	if !ok {
		return false
	}
	return peers.updatePeer(conn)
}

func (s *server) removeClusterPeers(conns []types.TunnelConnection) {
	s.Lock()
	defer s.Unlock()
	for _, conn := range conns {
		peers, ok := s.clusterPeers[conn.GetClusterName()]
		if !ok {
			s.log.Warningf("failed to remove cluster peer, not found peers for %v.", conn)
			continue
		}
		peers.removePeer(conn)
		s.log.Debugf("Removed cluster peer %v.", conn)
	}
}

func (s *server) existingConns() map[string]types.TunnelConnection {
	s.RLock()
	defer s.RUnlock()
	conns := make(map[string]types.TunnelConnection)
	for _, peers := range s.clusterPeers {
		for _, cluster := range peers.peers {
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
	s.log.Debugf("Advising reconnect to local site: %s", s.localSite.GetName())
	go s.localSite.adviseReconnect(ctx)

	for _, site := range s.remoteSites {
		s.log.Debugf("Advising reconnect to remote site: %s", site.GetName())
		go site.adviseReconnect(ctx)
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
		s.handleHeartbeat(conn, sconn, nch)
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
		s.log.Warn(msg)
		s.rejectRequest(nch, ssh.ConnectionFailed, msg)
		return
	}
}

func (s *server) handleTransport(sconn *ssh.ServerConn, nch ssh.NewChannel) {
	s.log.Debug("Received transport request.")
	channel, requestC, err := nch.Accept()
	if err != nil {
		sconn.Close()
		// avoid WithError to reduce log spam on network errors
		s.log.Warnf("Failed to accept request: %v.", err)
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
		s.log.Warn("Timed out waiting for transport dial request.")
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
		s.log.WithField("address", dialReq.Address).
			Warn("Received dial request for unexpected address, routing to the auth server anyway.")
	}

	authAddress := utils.ChooseRandomString(s.LocalAuthAddresses)
	if authAddress == "" {
		s.log.Error("No auth servers configured.")
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
			s.log.WithError(err).Error("Failed to create signed PROXY header.")
			fmt.Fprint(ch.Stderr(), "internal server error")
			req.Reply(false, nil)
		}
		proxyHeader = h
	}

	d := net.Dialer{Timeout: apidefaults.DefaultIOTimeout}
	conn, err := d.DialContext(s.ctx, "tcp", authAddress)
	if err != nil {
		s.log.Errorf("Failed to dial auth: %v.", err)
		fmt.Fprint(ch.Stderr(), "failed to dial auth server")
		req.Reply(false, nil)
		return
	}
	defer conn.Close()

	_ = conn.SetWriteDeadline(time.Now().Add(apidefaults.DefaultIOTimeout))
	if _, err := conn.Write(proxyHeader); err != nil {
		s.log.Errorf("Failed to send PROXY header: %v.", err)
		fmt.Fprint(ch.Stderr(), "failed to dial auth server")
		req.Reply(false, nil)
		return
	}
	_ = conn.SetWriteDeadline(time.Time{})

	if err := req.Reply(true, nil); err != nil {
		s.log.Errorf("Failed to respond to dial request: %v.", err)
		return
	}

	_ = utils.ProxyConn(s.ctx, ch, conn)
}

// TODO(awly): unit test this
func (s *server) handleHeartbeat(conn net.Conn, sconn *ssh.ServerConn, nch ssh.NewChannel) {
	s.log.Debugf("New tunnel from %v.", sconn.RemoteAddr())
	if sconn.Permissions.Extensions[utils.ExtIntCertType] != utils.ExtIntCertTypeHost {
		s.log.Error(trace.BadParameter("can't retrieve certificate type in certType"))
		return
	}

	// Extract the role. For proxies, it's another cluster asking to join, for
	// nodes it's a node dialing back.
	val, ok := sconn.Permissions.Extensions[extCertRole]
	if !ok {
		s.log.Errorf("Failed to accept connection, missing %q extension", extCertRole)
		s.rejectRequest(nch, ssh.ConnectionFailed, "unknown role")
		return
	}

	role := types.SystemRole(val)
	switch role {
	// Node is dialing back.
	case types.RoleNode:
		s.handleNewService(role, conn, sconn, nch, types.NodeTunnel)
	// App is dialing back.
	case types.RoleApp:
		s.handleNewService(role, conn, sconn, nch, types.AppTunnel)
	// Kubernetes service is dialing back.
	case types.RoleKube:
		s.handleNewService(role, conn, sconn, nch, types.KubeTunnel)
	// Database proxy is dialing back.
	case types.RoleDatabase:
		s.handleNewService(role, conn, sconn, nch, types.DatabaseTunnel)
	// Proxy is dialing back.
	case types.RoleProxy:
		s.handleNewCluster(conn, sconn, nch)
	case types.RoleWindowsDesktop:
		s.handleNewService(role, conn, sconn, nch, types.WindowsDesktopTunnel)
	case types.RoleOkta:
		s.handleNewService(role, conn, sconn, nch, types.OktaTunnel)
	// Unknown role.
	default:
		s.log.Errorf("Unsupported role attempting to connect: %v", val)
		s.rejectRequest(nch, ssh.ConnectionFailed, fmt.Sprintf("unsupported role %v", val))
	}
}

func (s *server) handleNewService(role types.SystemRole, conn net.Conn, sconn *ssh.ServerConn, nch ssh.NewChannel, connType types.TunnelType) {
	cluster, rconn, err := s.upsertServiceConn(conn, sconn, connType)
	if err != nil {
		s.log.Errorf("Failed to upsert %s: %v.", role, err)
		sconn.Close()
		return
	}

	ch, req, err := nch.Accept()
	if err != nil {
		s.log.Errorf("Failed to accept on channel: %v.", err)
		sconn.Close()
		return
	}

	go cluster.handleHeartbeat(rconn, ch, req)
}

func (s *server) handleNewCluster(conn net.Conn, sshConn *ssh.ServerConn, nch ssh.NewChannel) {
	// add the incoming site (cluster) to the list of active connections:
	site, remoteConn, err := s.upsertRemoteCluster(conn, sshConn)
	if err != nil {
		s.log.Error(trace.Wrap(err))
		s.rejectRequest(nch, ssh.ConnectionFailed, "failed to accept incoming cluster connection")
		return
	}
	// accept the request and start the heartbeat on it:
	ch, req, err := nch.Accept()
	if err != nil {
		s.log.Error(trace.Wrap(err))
		sshConn.Close()
		return
	}
	go site.handleHeartbeat(remoteConn, ch, req)
}

func (s *server) requireLocalAgentForConn(sconn *ssh.ServerConn, connType types.TunnelType) error {
	// Cluster name was extracted from certificate and packed into extensions.
	clusterName := sconn.Permissions.Extensions[extAuthority]
	if strings.TrimSpace(clusterName) == "" {
		return trace.BadParameter("empty cluster name")
	}

	if s.localSite.domainName == clusterName {
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
	logger := s.log.WithFields(log.Fields{
		"remote": conn.RemoteAddr(),
		"user":   conn.User(),
	})
	// The crypto/x/ssh package won't log the returned error for us, do it
	// manually.
	defer func() {
		if err != nil {
			logger.Warnf("Failed to authenticate client, err: %v.", err)
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

	if err := s.checkClientCert(logger, conn.User(), clusterName, cert, caType); err != nil {
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
func (s *server) checkClientCert(logger *log.Entry, user string, clusterName string, cert *ssh.Certificate, caType types.CertAuthType) error {
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
		return trace.BadParameter(err.Error())
	}

	return nil
}

func (s *server) upsertServiceConn(conn net.Conn, sconn *ssh.ServerConn, connType types.TunnelType) (*localSite, *remoteConn, error) {
	s.Lock()
	defer s.Unlock()

	if err := s.requireLocalAgentForConn(sconn, connType); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	nodeID, ok := sconn.Permissions.Extensions[extHost]
	if !ok {
		return nil, nil, trace.BadParameter("host id not found")
	}

	rconn, err := s.localSite.addConn(nodeID, connType, conn, sconn)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return s.localSite, rconn, nil
}

func (s *server) upsertRemoteCluster(conn net.Conn, sshConn *ssh.ServerConn) (*remoteSite, *remoteConn, error) {
	domainName := sshConn.Permissions.Extensions[extAuthority]
	if strings.TrimSpace(domainName) == "" {
		return nil, nil, trace.BadParameter("cannot create reverse tunnel: empty cluster name")
	}

	s.Lock()
	defer s.Unlock()

	var site *remoteSite
	for _, st := range s.remoteSites {
		if st.domainName == domainName {
			site = st
			break
		}
	}
	var err error
	var remoteConn *remoteConn
	if site != nil {
		if remoteConn, err = site.addConn(conn, sshConn); err != nil {
			return nil, nil, trace.Wrap(err)
		}
	} else {
		site, err = newRemoteSite(s, domainName, sshConn.Conn)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		if remoteConn, err = site.addConn(conn, sshConn); err != nil {
			return nil, nil, trace.Wrap(err)
		}
		s.remoteSites = append(s.remoteSites, site)
	}
	site.logger.Infof("Connection <- %v, clusters: %d.", conn.RemoteAddr(), len(s.remoteSites))
	// treat first connection as a registered heartbeat,
	// otherwise the connection information will appear after initial
	// heartbeat delay
	go site.registerHeartbeat(s.Clock.Now())
	return site, remoteConn, nil
}

func (s *server) GetSites() ([]reversetunnelclient.RemoteSite, error) {
	s.RLock()
	defer s.RUnlock()
	out := make([]reversetunnelclient.RemoteSite, 0, len(s.remoteSites)+len(s.clusterPeers)+1)
	out = append(out, s.localSite)

	haveLocalConnection := make(map[string]bool)
	for i := range s.remoteSites {
		site := s.remoteSites[i]
		haveLocalConnection[site.GetName()] = true
		out = append(out, site)
	}
	for i := range s.clusterPeers {
		cluster := s.clusterPeers[i]
		if _, ok := haveLocalConnection[cluster.GetName()]; !ok {
			out = append(out, cluster)
		}
	}
	return out, nil
}

func (s *server) getRemoteClusters() []*remoteSite {
	s.RLock()
	defer s.RUnlock()
	out := make([]*remoteSite, len(s.remoteSites))
	copy(out, s.remoteSites)
	return out
}

// GetSite returns a RemoteSite. The first attempt is to find and return a
// remote site and that is what is returned if a remote agent has
// connected to this proxy. Next we loop over local sites and try and try and
// return a local site. If that fails, we return a cluster peer. This happens
// when you hit proxy that has never had an agent connect to it. If you end up
// with a cluster peer your best bet is to wait until the agent has discovered
// all proxies behind a load balancer. Note, the cluster peer is a
// services.TunnelConnection that was created by another proxy.
func (s *server) GetSite(name string) (reversetunnelclient.RemoteSite, error) {
	s.RLock()
	defer s.RUnlock()
	if s.localSite.GetName() == name {
		return s.localSite, nil
	}
	for i := range s.remoteSites {
		if s.remoteSites[i].GetName() == name {
			return s.remoteSites[i], nil
		}
	}
	for i := range s.clusterPeers {
		if s.clusterPeers[i].GetName() == name {
			return s.clusterPeers[i], nil
		}
	}
	return nil, trace.NotFound("cluster %q is not found", name)
}

// GetProxyPeerClient returns the proxy peer client
func (s *server) GetProxyPeerClient() *peer.Client {
	return s.PeerClient
}

// alwaysClose forces onSiteTunnelClose to remove and close
// the site by always returning false from HasValidConnections.
type alwaysClose struct {
	reversetunnelclient.RemoteSite
}

func (a *alwaysClose) HasValidConnections() bool {
	return false
}

// siteCloser is used by onSiteTunnelClose to determine if a site should be closed
// when a tunnel is closed
type siteCloser interface {
	GetName() string
	HasValidConnections() bool
	io.Closer
}

// onSiteTunnelClose will close and stop tracking the site with the given name
// if it has 0 active tunnels. This is done here to ensure that no new tunnels
// can be established while cleaning up a site.
func (s *server) onSiteTunnelClose(site siteCloser) error {
	s.Lock()
	defer s.Unlock()

	if site.HasValidConnections() {
		return nil
	}

	for i := range s.remoteSites {
		if s.remoteSites[i].domainName == site.GetName() {
			s.remoteSites = append(s.remoteSites[:i], s.remoteSites[i+1:]...)
			return trace.Wrap(site.Close())
		}
	}

	return trace.NotFound("site %q is not found", site.GetName())
}

// fanOutProxies is a non-blocking call that updated the watches proxies
// list and notifies all clusters about the proxy list change
func (s *server) fanOutProxies(proxies []types.Server) {
	s.Lock()
	defer s.Unlock()
	s.localSite.fanOutProxies(proxies)

	for _, cluster := range s.remoteSites {
		cluster.fanOutProxies(proxies)
	}
}

func (s *server) rejectRequest(ch ssh.NewChannel, reason ssh.RejectionReason, msg string) {
	if err := ch.Reject(reason, msg); err != nil {
		s.log.Warnf("Failed rejecting new channel request: %v", err)
	}
}

// TrackUserConnection tracks a user connection that should prevent
// the server from being terminated if active. The returned function
// should be called when the connection is terminated.
func (s *server) TrackUserConnection() (release func()) {
	return s.srv.TrackUserConnection()
}

// newRemoteSite helper creates and initializes 'remoteSite' instance
func newRemoteSite(srv *server, domainName string, sconn ssh.Conn) (*remoteSite, error) {
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
	remoteSite := &remoteSite{
		srv:        srv,
		domainName: domainName,
		connInfo:   connInfo,
		logger: log.WithFields(log.Fields{
			teleport.ComponentKey: teleport.ComponentReverseTunnelServer,
			teleport.ComponentFields: log.Fields{
				"cluster": domainName,
			},
		}),
		ctx:               closeContext,
		cancel:            cancel,
		clock:             srv.Clock,
		offlineThreshold:  srv.offlineThreshold,
		proxySyncInterval: proxySyncInterval,
	}

	// configure access to the full Auth Server API and the cached subset for
	// the local cluster within which reversetunnelclient.Server is running.
	remoteSite.localClient = srv.localAuthClient
	remoteSite.localAccessPoint = srv.localAccessPoint

	clt, _, err := remoteSite.getRemoteClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	remoteSite.remoteClient = clt

	remoteVersion, err := getRemoteAuthVersion(closeContext, sconn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessPoint, err := createRemoteAccessPoint(srv, clt, remoteVersion, domainName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	remoteSite.remoteAccessPoint = accessPoint
	nodeWatcher, err := services.NewNodeWatcher(closeContext, services.NodeWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component:    srv.Component,
			Client:       accessPoint,
			Log:          srv.Log,
			MaxStaleness: time.Minute,
		},
		NodesGetter: accessPoint,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	remoteSite.nodeWatcher = nodeWatcher
	// instantiate a cache of host certificates for the forwarding server. the
	// certificate cache is created in each site (instead of creating it in
	// reversetunnel.server and passing it along) so that the host certificate
	// is signed by the correct certificate authority.
	certificateCache, err := newHostCertificateCache(srv.localAuthClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	remoteSite.certificateCache = certificateCache

	caRetry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  utils.HalfJitter(srv.Config.PollingPeriod),
		Step:   srv.Config.PollingPeriod / 5,
		Max:    srv.Config.PollingPeriod,
		Jitter: retryutils.NewHalfJitter(),
		Clock:  srv.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	remoteWatcher, err := services.NewCertAuthorityWatcher(srv.ctx, services.CertAuthorityWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentProxy,
			Log:       srv.log,
			Clock:     srv.Clock,
			Client:    remoteSite.remoteAccessPoint,
		},
		Types: []types.CertAuthType{types.HostCA},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go func() {
		remoteSite.updateCertAuthorities(caRetry, remoteWatcher, remoteVersion)
	}()

	lockRetry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  utils.HalfJitter(srv.Config.PollingPeriod),
		Step:   srv.Config.PollingPeriod / 5,
		Max:    srv.Config.PollingPeriod,
		Jitter: retryutils.NewHalfJitter(),
		Clock:  srv.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go remoteSite.updateLocks(lockRetry)

	return remoteSite, nil
}

// createRemoteAccessPoint creates a new access point for the remote cluster.
// Checks if the cluster that is connecting is a pre-v13 cluster. If it is,
// we disable the watcher for resources not supported in a v12 leaf cluster:
// - (to fill when we add new resources)
//
// **WARNING**: Ensure that the version below matches the version in which backward incompatible
// changes were introduced so that the cache is created successfully. Otherwise, the remote cache may
// never become healthy due to unknown resources.
func createRemoteAccessPoint(srv *server, clt authclient.ClientI, version, domainName string) (authclient.RemoteProxyAccessPoint, error) {
	ok, err := utils.MinVerWithoutPreRelease(version, utils.VersionBeforeAlpha("13.0.0"))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessPointFunc := srv.Config.NewCachingAccessPoint
	if !ok {
		srv.log.Debugf("cluster %q running %q is connecting, loading old cache policy.", domainName, version)
		accessPointFunc = srv.Config.NewCachingAccessPointOldProxy
	}

	// Configure access to the cached subset of the Auth Server API of the remote
	// cluster this remote site provides access to.
	accessPoint, err := accessPointFunc(clt, []string{"reverse", domainName})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return accessPoint, nil
}

// getRemoteAuthVersion sends a version request to the remote agent.
func getRemoteAuthVersion(ctx context.Context, sconn ssh.Conn) (string, error) {
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
