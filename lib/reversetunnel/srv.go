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

package reversetunnel

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/state"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// server is a "reverse tunnel server". it exposes the cluster capabilities
// (like access to a cluster's auth) to remote trusted clients
// (also known as 'reverse tunnel agents'.
type server struct {
	sync.RWMutex
	Config

	// localAuthClient provides access to the full Auth Server API for the
	// local cluster.
	localAuthClient auth.ClientI
	// localAccessPoint provides access to a cached subset of the Auth
	// Server API.
	localAccessPoint auth.AccessPoint

	hostCertChecker ssh.CertChecker
	userCertChecker ssh.CertChecker

	// srv is the "base class" i.e. the underlying SSH server
	srv     *sshutils.Server
	limiter *limiter.Limiter

	// remoteSites is the list of conencted remote clusters
	remoteSites []*remoteSite

	// localSites is the list of local (our own cluster) tunnel clients,
	// usually each of them is a local proxy.
	localSites []*localSite

	// clusterPeers is a map of clusters connected to peer proxies
	// via reverse tunnels
	clusterPeers map[string]*clusterPeers

	// newAccessPoint returns new caching access point
	newAccessPoint state.NewCachingAccessPoint

	// cancel function will cancel the
	cancel context.CancelFunc

	// ctx is a context used for signalling and broadcast
	ctx context.Context

	*log.Entry
}

// DirectCluster is used to access cluster directly
type DirectCluster struct {
	// Name is a cluster name
	Name string
	// Client is a client to the cluster
	Client auth.ClientI
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
	HostSigners []ssh.Signer
	// HostKeyCallback
	// Limiter is optional request limiter
	Limiter *limiter.Limiter
	// LocalAuthClient provides access to a full AuthClient for the local cluster.
	LocalAuthClient auth.ClientI
	// AccessPoint provides access to a subset of AuthClient of the cluster.
	// AccessPoint caches values and can still return results during connection
	// problems.
	LocalAccessPoint auth.AccessPoint
	// NewCachingAccessPoint returns new caching access points
	// per remote cluster
	NewCachingAccessPoint state.NewCachingAccessPoint
	// DirectClusters is a list of clusters accessed directly
	DirectClusters []DirectCluster
	// Context is a signalling context
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
	if cfg.Context == nil {
		cfg.Context = context.TODO()
	}
	if cfg.PollingPeriod == 0 {
		cfg.PollingPeriod = defaults.HighResPollingPeriod
	}
	if cfg.Limiter == nil {
		var err error
		cfg.Limiter, err = limiter.NewLimiter(limiter.LimiterConfig{})
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	return nil
}

// NewServer creates and returns a reverse tunnel server which is fully
// initialized but hasn't been started yet
func NewServer(cfg Config) (Server, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(cfg.Context)
	srv := &server{
		Config:           cfg,
		localSites:       []*localSite{},
		remoteSites:      []*remoteSite{},
		localAuthClient:  cfg.LocalAuthClient,
		localAccessPoint: cfg.LocalAccessPoint,
		newAccessPoint:   cfg.NewCachingAccessPoint,
		limiter:          cfg.Limiter,
		ctx:              ctx,
		cancel:           cancel,
		clusterPeers:     make(map[string]*clusterPeers),
		Entry: log.WithFields(log.Fields{
			trace.Component: teleport.ComponentReverseTunnelServer,
		}),
	}

	for _, clusterInfo := range cfg.DirectClusters {
		cluster, err := newlocalSite(srv, clusterInfo.Name, clusterInfo.Client)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		srv.localSites = append(srv.localSites, cluster)
	}

	var err error
	s, err := sshutils.NewServer(
		teleport.ComponentReverseTunnelServer,
		// TODO(klizhentas): improve interface, use struct instead of parameter list
		// this address is not used
		utils.NetAddr{Addr: "127.0.0.1:1", AddrNetwork: "tcp"},
		srv,
		cfg.HostSigners,
		sshutils.AuthMethods{
			PublicKey: srv.keyAuth,
		},
		sshutils.SetLimiter(cfg.Limiter),
		sshutils.SetCiphers(cfg.Ciphers),
		sshutils.SetKEXAlgorithms(cfg.KEXAlgorithms),
		sshutils.SetMACAlgorithms(cfg.MACAlgorithms),
	)
	if err != nil {
		return nil, err
	}
	srv.userCertChecker = ssh.CertChecker{IsUserAuthority: srv.isUserAuthority}
	srv.hostCertChecker = ssh.CertChecker{IsHostAuthority: srv.isHostAuthority}
	srv.srv = s
	go srv.periodicFunctions()
	return srv, nil
}

func remoteClustersMap(rc []services.RemoteCluster) map[string]services.RemoteCluster {
	out := make(map[string]services.RemoteCluster)
	for i := range rc {
		out[rc[i].GetName()] = rc[i]
	}
	return out
}

// disconnectClusters disconnects reverse tunnel connections from remote clusters
// that were deleted from the the local cluster side and cleans up in memory objects.
// In this case all local trust has been deleted, so all the tunnel connections have to be dropped.
func (s *server) disconnectClusters() error {
	connectedRemoteClusters := s.getRemoteClusters()
	if len(connectedRemoteClusters) == 0 {
		return nil
	}
	remoteClusters, err := s.localAuthClient.GetRemoteClusters()
	if err != nil {
		return trace.Wrap(err)
	}
	remoteMap := remoteClustersMap(remoteClusters)
	for _, cluster := range connectedRemoteClusters {
		if _, ok := remoteMap[cluster.GetName()]; !ok {
			s.Infof("Remote cluster %q has been deleted. Disconnecting it from the proxy.", cluster.GetName())
			s.RemoveSite(cluster.GetName())
			err := cluster.Close()
			if err != nil {
				s.Debugf("Failure closing cluster %q: %v.", cluster.GetName(), err)
			}
		}
	}
	return nil
}

func (s *server) periodicFunctions() {
	ticker := time.NewTicker(defaults.ReverseTunnelAgentHeartbeatPeriod)
	defer ticker.Stop()
	if err := s.fetchClusterPeers(); err != nil {
		s.Warningf("Failed to fetch cluster peers: %v.", err)
	}
	for {
		select {
		case <-s.ctx.Done():
			s.Debugf("Closing.")
			return
		case <-ticker.C:
			err := s.fetchClusterPeers()
			if err != nil {
				s.Warningf("Failed to fetch cluster peers: %v.", err)
			}
			err = s.disconnectClusters()
			if err != nil {
				s.Warningf("Failed to disconnect clusters: %v.", err)
			}
		}
	}
}

// fetchClusterPeers pulls back all proxies that have registered themselves
// (created a services.TunnelConnection) in the backend and compares them to
// what was found in the previous iteration and updates the in-memory cluster
// peer map. This map is used later by GetSite(s) to return either local or
// remote site, or if non match, a cluster peer.
func (s *server) fetchClusterPeers() error {
	conns, err := s.LocalAccessPoint.GetAllTunnelConnections()
	if err != nil {
		return trace.Wrap(err)
	}
	newConns := make(map[string]services.TunnelConnection)
	for i := range conns {
		newConn := conns[i]
		// filter out peer records for our own proxy
		if newConn.GetProxyName() == s.ID {
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

func (s *server) addClusterPeers(conns map[string]services.TunnelConnection) error {
	for key := range conns {
		connInfo := conns[key]
		peer, err := newClusterPeer(s, connInfo)
		if err != nil {
			return trace.Wrap(err)
		}
		s.addClusterPeer(peer)
	}
	return nil
}

func (s *server) updateClusterPeers(conns map[string]services.TunnelConnection) {
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

func (s *server) updateClusterPeer(conn services.TunnelConnection) bool {
	s.Lock()
	defer s.Unlock()
	clusterName := conn.GetClusterName()
	peers, ok := s.clusterPeers[clusterName]
	if !ok {
		return false
	}
	return peers.updatePeer(conn)
}

func (s *server) removeClusterPeers(conns []services.TunnelConnection) {
	s.Lock()
	defer s.Unlock()
	for _, conn := range conns {
		peers, ok := s.clusterPeers[conn.GetClusterName()]
		if !ok {
			s.Warningf("failed to remove cluster peer, not found peers for %v", conn)
			continue
		}
		peers.removePeer(conn)
		s.Debugf("removed cluster peer %v", conn)
	}
}

func (s *server) existingConns() map[string]services.TunnelConnection {
	s.RLock()
	defer s.RUnlock()
	conns := make(map[string]services.TunnelConnection)
	for _, peers := range s.clusterPeers {
		for _, cluster := range peers.peers {
			conns[cluster.connInfo.GetName()] = cluster.connInfo
		}
	}
	return conns
}

func (s *server) diffConns(newConns, existingConns map[string]services.TunnelConnection) (map[string]services.TunnelConnection, map[string]services.TunnelConnection, []services.TunnelConnection) {
	connsToAdd := make(map[string]services.TunnelConnection)
	connsToUpdate := make(map[string]services.TunnelConnection)
	var connsToRemove []services.TunnelConnection

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

func (s *server) Wait() {
	s.srv.Wait(context.TODO())
}

func (s *server) Start() error {
	go s.srv.Serve(s.Listener)
	return nil
}

func (s *server) Close() error {
	s.cancel()
	return s.srv.Close()
}

func (s *server) Shutdown(ctx context.Context) error {
	s.cancel()
	return s.srv.Shutdown(ctx)
}

func (s *server) HandleNewChan(conn net.Conn, sconn *ssh.ServerConn, nch ssh.NewChannel) {
	// apply read/write timeouts to the server connection
	conn = utils.ObeyIdleTimeout(conn,
		defaults.ReverseTunnelAgentHeartbeatPeriod*10,
		"reverse tunnel server")

	ct := nch.ChannelType()
	if ct != chanHeartbeat {
		msg := fmt.Sprintf("reversetunnel received unknown channel request %v from %v",
			nch.ChannelType(), sconn)
		// if someone is trying to open a new SSH session by talking to a reverse tunnel,
		// they're most likely using the wrong port number. Lets give them the explicit hint:
		if ct == "session" {
			msg = "Cannot open new SSH session on reverse tunnel. Are you connecting to the right port?"
		}
		s.Warning(msg)
		nch.Reject(ssh.ConnectionFailed, msg)
		return
	}
	s.Debugf("new tunnel from %s", sconn.RemoteAddr())
	if sconn.Permissions.Extensions[extCertType] != extCertTypeHost {
		s.Error(trace.BadParameter("can't retrieve certificate type in certType"))
		return
	}
	// add the incoming site (cluster) to the list of active connections:
	site, remoteConn, err := s.upsertSite(conn, sconn)
	if err != nil {
		log.Error(trace.Wrap(err))
		nch.Reject(ssh.ConnectionFailed, "failed to accept incoming cluster connection")
		return
	}
	// accept the request and start the heartbeat on it:
	ch, req, err := nch.Accept()
	if err != nil {
		log.Error(trace.Wrap(err))
		sconn.Close()
		return
	}
	go site.handleHeartbeat(remoteConn, ch, req)
}

// isHostAuthority is called during checking the client key, to see if the signing
// key is the real host CA authority key.
func (s *server) isHostAuthority(auth ssh.PublicKey, address string) bool {
	keys, err := s.getTrustedCAKeys(services.HostCA)
	if err != nil {
		s.Errorf("failed to retrieve trusted keys, err: %v", err)
		return false
	}
	for _, k := range keys {
		if sshutils.KeysEqual(k, auth) {
			return true
		}
	}
	return false
}

// isUserAuthority is called during checking the client key, to see if the signing
// key is the real user CA authority key.
func (s *server) isUserAuthority(auth ssh.PublicKey) bool {
	keys, err := s.getTrustedCAKeys(services.UserCA)
	if err != nil {
		s.Errorf("failed to retrieve trusted keys, err: %v", err)
		return false
	}
	for _, k := range keys {
		if sshutils.KeysEqual(k, auth) {
			return true
		}
	}
	return false
}

func (s *server) getTrustedCAKeys(CertType services.CertAuthType) ([]ssh.PublicKey, error) {
	cas, err := s.localAccessPoint.GetCertAuthorities(CertType, false)
	if err != nil {
		return nil, err
	}
	out := []ssh.PublicKey{}
	for _, ca := range cas {
		checkers, err := ca.Checkers()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, checkers...)
	}
	return out, nil
}

func (s *server) checkTrustedKey(CertType services.CertAuthType, domainName string, key ssh.PublicKey) error {
	cas, err := s.localAccessPoint.GetCertAuthorities(CertType, false)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, ca := range cas {
		if ca.GetClusterName() != domainName {
			continue
		}
		checkers, err := ca.Checkers()
		if err != nil {
			return trace.Wrap(err)
		}
		for _, checker := range checkers {
			if sshutils.KeysEqual(key, checker) {
				return nil
			}
		}
	}
	return trace.NotFound("authority domain %v not found or has no mathching keys", domainName)
}

func (s *server) keyAuth(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	logger := s.WithFields(log.Fields{
		"remote": conn.RemoteAddr(),
		"user":   conn.User(),
	})

	cert, ok := key.(*ssh.Certificate)
	if !ok {
		logger.Warningf("server doesn't support provided key type")
		return nil, trace.BadParameter("server doesn't support provided key type")
	}

	switch cert.CertType {
	case ssh.HostCert:
		authDomain, ok := cert.Extensions[utils.CertExtensionAuthority]
		if !ok || authDomain == "" {
			err := trace.BadParameter("missing authority domainName parameter")
			logger.Warningf("Failed to authenticate host, err: %v.", err)
			return nil, err
		}
		// CheckHostKey expects the addr that is passed in to be in the format
		// host:port. This is because this function is usually used by a client to
		// check if the host it attempted to connect to presented a certificate for
		// the host requested (this prevents man-in-the-middle attacks).
		//
		// In this situation however, it's a server essentially performing user
		// authentication, but since it's machine-to-machine communication, the
		// "user" is presenting a host certificate. To make CheckHostKey behave
		// like Authenticate we pass in a addr in the host:port format it expects.
		addr := formatAddr(conn.User())
		err := s.hostCertChecker.CheckHostKey(addr, conn.RemoteAddr(), key)
		if err != nil {
			logger.Warningf("Failed to authenticate host, err: %v.", err)
			return nil, trace.Wrap(err)
		}
		if err := s.hostCertChecker.CheckCert(conn.User(), cert); err != nil {
			logger.Warningf("Failed to authenticate host err: %v.", err)
			return nil, trace.Wrap(err)
		}
		// this fixes possible injection attack
		// when we have 2 trusted remote sites, and one can simply
		// pose as another. so we have to check that authority
		// matches by some other way (in absence of x509 chains)
		if err := s.checkTrustedKey(services.HostCA, authDomain, cert.SignatureKey); err != nil {
			logger.Warningf("This client claims to be signed as cluster %q, but no matching signing keys found", authDomain)
			return nil, trace.Wrap(err)
		}
		return &ssh.Permissions{
			Extensions: map[string]string{
				extHost:      conn.User(),
				extCertType:  extCertTypeHost,
				extAuthority: authDomain,
			},
		}, nil
	case ssh.UserCert:
		_, err := s.userCertChecker.Authenticate(conn, key)
		if err != nil {
			logger.Warningf("Failed to authenticate user, err: %v.", err)
			return nil, err
		}

		if err := s.userCertChecker.CheckCert(conn.User(), cert); err != nil {
			logger.Warningf("Failed to authenticate user err: %v.", err)
			return nil, trace.Wrap(err)
		}

		return &ssh.Permissions{
			Extensions: map[string]string{
				extHost:     conn.User(),
				extCertType: extCertTypeUser,
			},
		}, nil
	default:
		return nil, trace.BadParameter("unsupported cert type: %v", cert.CertType)
	}
}

func (s *server) upsertSite(conn net.Conn, sshConn *ssh.ServerConn) (*remoteSite, *remoteConn, error) {
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
		site, err = newRemoteSite(s, domainName)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		if remoteConn, err = site.addConn(conn, sshConn); err != nil {
			return nil, nil, trace.Wrap(err)
		}
		s.remoteSites = append(s.remoteSites, site)
	}
	site.Infof("Connection <- %v, clusters: %d.", conn.RemoteAddr(), len(s.remoteSites))
	// treat first connection as a registered heartbeat,
	// otherwise the connection information will appear after initial
	// heartbeat delay
	go site.registerHeartbeat(time.Now())
	return site, remoteConn, nil
}

func (s *server) GetSites() []RemoteSite {
	s.RLock()
	defer s.RUnlock()
	out := make([]RemoteSite, 0, len(s.remoteSites)+len(s.localSites)+len(s.clusterPeers))
	for i := range s.localSites {
		out = append(out, s.localSites[i])
	}
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
	return out
}

func (s *server) getRemoteClusters() []*remoteSite {
	s.RLock()
	defer s.RUnlock()
	out := make([]*remoteSite, len(s.remoteSites))
	copy(out, s.remoteSites)
	return out
}

// GetSite returns a RemoteSite. The first attempt is to find and return a
// remote site and that is what is returned if a remote remote agent has
// connected to this proxy. Next we loop over local sites and try and try and
// return a local site. If that fails, we return a cluster peer. This happens
// when you hit proxy that has never had an agent connect to it. If you end up
// with a cluster peer your best bet is to wait until the agent has discovered
// all proxies behind a the load balancer. Note, the cluster peer is a
// services.TunnelConnection that was created by another proxy.
func (s *server) GetSite(name string) (RemoteSite, error) {
	s.RLock()
	defer s.RUnlock()
	for i := range s.remoteSites {
		if s.remoteSites[i].GetName() == name {
			return s.remoteSites[i], nil
		}
	}
	for i := range s.localSites {
		if s.localSites[i].GetName() == name {
			return s.localSites[i], nil
		}
	}
	for i := range s.clusterPeers {
		if s.clusterPeers[i].GetName() == name {
			return s.clusterPeers[i], nil
		}
	}
	return nil, trace.NotFound("cluster %q is not found", name)
}

func (s *server) RemoveSite(domainName string) error {
	s.Lock()
	defer s.Unlock()
	for i := range s.remoteSites {
		if s.remoteSites[i].domainName == domainName {
			s.remoteSites = append(s.remoteSites[:i], s.remoteSites[i+1:]...)
			return nil
		}
	}
	for i := range s.localSites {
		if s.localSites[i].domainName == domainName {
			s.localSites = append(s.localSites[:i], s.localSites[i+1:]...)
			return nil
		}
	}
	return trace.NotFound("cluster %q is not found", domainName)
}

type remoteConn struct {
	sshConn      ssh.Conn
	conn         net.Conn
	invalid      int32
	log          *log.Entry
	counter      int32
	discoveryC   ssh.Channel
	discoveryErr error
	closed       int32
}

func (rc *remoteConn) openDiscoveryChannel() (ssh.Channel, error) {
	if rc.discoveryC != nil {
		return rc.discoveryC, nil
	}
	if rc.discoveryErr != nil {
		return nil, trace.Wrap(rc.discoveryErr)
	}
	discoveryC, _, err := rc.sshConn.OpenChannel(chanDiscovery, nil)
	if err != nil {
		rc.discoveryErr = err
		return nil, trace.Wrap(err)
	}
	rc.discoveryC = discoveryC
	return rc.discoveryC, nil
}

func (rc *remoteConn) String() string {
	return fmt.Sprintf("remoteConn(remoteAddr=%v)", rc.conn.RemoteAddr())
}

func (rc *remoteConn) Close() error {
	if !atomic.CompareAndSwapInt32(&rc.closed, 0, 1) {
		// already closed
		return nil
	}
	if rc.discoveryC != nil {
		rc.discoveryC.Close()
		rc.discoveryC = nil
	}
	return rc.sshConn.Close()
}

func (rc *remoteConn) markInvalid(err error) {
	atomic.StoreInt32(&rc.invalid, 1)
}

func (rc *remoteConn) isInvalid() bool {
	return atomic.LoadInt32(&rc.invalid) == 1
}

// newRemoteSite helper creates and initializes 'remoteSite' instance
func newRemoteSite(srv *server, domainName string) (*remoteSite, error) {
	connInfo, err := services.NewTunnelConnection(
		fmt.Sprintf("%v-%v", srv.ID, domainName),
		services.TunnelConnectionSpecV2{
			ClusterName:   domainName,
			ProxyName:     srv.ID,
			LastHeartbeat: time.Now().UTC(),
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	closeContext, cancel := context.WithCancel(srv.ctx)
	remoteSite := &remoteSite{
		srv:        srv,
		domainName: domainName,
		connInfo:   connInfo,
		Entry: log.WithFields(log.Fields{
			trace.Component: teleport.ComponentReverseTunnelServer,
			trace.ComponentFields: log.Fields{
				"cluster": domainName,
			},
		}),
		ctx:    closeContext,
		cancel: cancel,
		clock:  srv.Clock,
	}

	// transport uses connection do dial out to the remote address
	remoteSite.transport = &http.Transport{
		Dial: remoteSite.dialAccessPoint,
	}

	// configure access to the full Auth Server API and the cached subset for
	// the local cluster within which reversetunnel.Server is running.
	remoteSite.localClient = srv.localAuthClient
	remoteSite.localAccessPoint = srv.localAccessPoint

	clt, _, err := remoteSite.getRemoteClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	remoteSite.remoteClient = clt

	// configure access to the cached subset of the Auth Server API of the remote
	// cluster this remote site provides access to.
	accessPoint, err := srv.newAccessPoint(clt, []string{"reverse", domainName})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	remoteSite.remoteAccessPoint = accessPoint

	// instantiate a cache of host certificates for the forwarding server. the
	// certificate cache is created in each site (instead of creating it in
	// reversetunnel.server and passing it along) so that the host certificate
	// is signed by the correct certificate authority.
	certificateCache, err := NewHostCertificateCache(srv.Config.KeyGen, srv.localAuthClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	remoteSite.certificateCache = certificateCache

	go remoteSite.periodicSendDiscoveryRequests()
	go remoteSite.periodicUpdateCertAuthorities()

	return remoteSite, nil
}

// formatAddr adds :port to the passed in string if it's not in
// host:port format.
func formatAddr(s string) string {
	i := strings.Index(s, ":")
	if i == -1 {
		return s + ":0"
	}
	if i == len(s)-1 {
		return s[:len(s)-1] + ":0"
	}

	port, err := strconv.Atoi(s[i+1:])
	if err != nil {
		return s[:i] + ":0"
	}

	if port < 0 || port > 65535 {
		return s[:i] + ":0"
	}

	return s
}

const (
	extHost         = "host@teleport"
	extCertType     = "certtype@teleport"
	extAuthority    = "auth@teleport"
	extCertTypeHost = "host"
	extCertTypeUser = "user"
)
