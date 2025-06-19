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
	"log/slog"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/proxy/peer"
	"github.com/gravitational/teleport/lib/reversetunnel/track"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/srv/forward"
	"github.com/gravitational/teleport/lib/srv/git"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	proxyutils "github.com/gravitational/teleport/lib/utils/proxy"
)

const (
	// periodicFunctionInterval is the interval at which periodic stats are calculated.
	periodicFunctionInterval = 3 * time.Minute

	// proxySyncInterval is the interval at which the current proxies are synchronized to
	// connected agents via a discovery request. It is a function of track.DefaultProxyExpiry
	// to ensure that the proxies are always synced before the tracker expiry.
	proxySyncInterval = track.DefaultProxyExpiry * 2 / 3

	// missedHeartBeatThreshold is the number of missed heart beats needed to terminate a connection.
	missedHeartBeatThreshold = 3
)

// withPeriodicFunctionInterval adjusts the periodic function interval
func withPeriodicFunctionInterval(interval time.Duration) func(site *localSite) {
	return func(site *localSite) {
		site.periodicFunctionInterval = interval
	}
}

// withProxySyncInterval adjusts the proxy sync interval
func withProxySyncInterval(interval time.Duration) func(site *localSite) {
	return func(site *localSite) {
		site.proxySyncInterval = interval
	}
}

func newLocalSite(srv *server, domainName string, authServers []string, opts ...func(*localSite)) (*localSite, error) {
	err := metrics.RegisterPrometheusCollectors(localClusterCollectors...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// instantiate a cache of host certificates for the forwarding server. the
	// certificate cache is created in each site (instead of creating it in
	// reversetunnel.server and passing it along) so that the host certificate
	// is signed by the correct certificate authority.
	certificateCache, err := newHostCertificateCache(srv.localAuthClient, srv.localAccessPoint, srv.Clock)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s := &localSite{
		srv:              srv,
		client:           srv.localAuthClient,
		accessPoint:      srv.LocalAccessPoint,
		certificateCache: certificateCache,
		domainName:       domainName,
		authServers:      authServers,
		remoteConns:      make(map[connKey][]*remoteConn),
		clock:            srv.Clock,
		logger: slog.With(
			teleport.ComponentKey, teleport.ComponentReverseTunnelServer,
			"cluster", domainName,
		),
		offlineThreshold:         srv.offlineThreshold,
		peerClient:               srv.PeerClient,
		periodicFunctionInterval: periodicFunctionInterval,
		proxySyncInterval:        proxySyncInterval,
	}

	for _, opt := range opts {
		opt(s)
	}

	// Start periodic functions for the local cluster in the background.
	go s.periodicFunctions()

	return s, nil
}

// localSite allows to directly access the remote servers
// not using any tunnel, and using standard SSH
//
// it implements RemoteSite interface
type localSite struct {
	logger      *slog.Logger
	domainName  string
	authServers []string
	srv         *server

	// client provides access to the Auth Server API of the local cluster.
	client authclient.ClientI
	// accessPoint provides access to a cached subset of the Auth Server API of
	// the local cluster.
	accessPoint authclient.RemoteProxyAccessPoint

	// certificateCache caches host certificates for the forwarding server.
	certificateCache *certificateCache

	// remoteConns maps UUID and connection type to remote connections, oldest to newest.
	remoteConns map[connKey][]*remoteConn

	// remoteConnsMtx protects remoteConns.
	remoteConnsMtx sync.Mutex

	// clock is used to control time in tests.
	clock clockwork.Clock

	// offlineThreshold is how long to wait for a keep alive message before
	// marking a reverse tunnel connection as invalid.
	offlineThreshold time.Duration

	// peerClient is the proxy peering client
	peerClient *peer.Client

	// periodicFunctionInterval defines the interval period functions run at
	periodicFunctionInterval time.Duration

	// proxySyncInterval defines the interval at which discovery requests are
	// sent to keep agents in sync
	proxySyncInterval time.Duration
}

// GetTunnelsCount always the number of tunnel connections to this cluster.
func (s *localSite) GetTunnelsCount() int {
	s.remoteConnsMtx.Lock()
	defer s.remoteConnsMtx.Unlock()

	return len(s.remoteConns)
}

// CachingAccessPoint returns an auth.RemoteProxyAccessPoint for this cluster.
func (s *localSite) CachingAccessPoint() (authclient.RemoteProxyAccessPoint, error) {
	return s.accessPoint, nil
}

// NodeWatcher returns a services.NodeWatcher for this cluster.
func (s *localSite) NodeWatcher() (*services.GenericWatcher[types.Server, readonly.Server], error) {
	return s.srv.NodeWatcher, nil
}

// GitServerWatcher returns a Git server watcher for this cluster.
func (s *localSite) GitServerWatcher() (*services.GenericWatcher[types.Server, readonly.Server], error) {
	return s.srv.GitServerWatcher, nil
}

// GetClient returns a client to the full Auth Server API.
func (s *localSite) GetClient() (authclient.ClientI, error) {
	return s.client, nil
}

// String returns a string representing this cluster.
func (s *localSite) String() string {
	return fmt.Sprintf("local(%v)", s.domainName)
}

// GetStatus always returns online because the localsite is never offline.
func (s *localSite) GetStatus() string {
	return teleport.RemoteClusterStatusOnline
}

// GetName returns the name of the cluster.
func (s *localSite) GetName() string {
	return s.domainName
}

// GetLastConnected returns the current time because the localsite is always
// connected.
func (s *localSite) GetLastConnected() time.Time {
	return s.clock.Now()
}

func (s *localSite) DialAuthServer(params reversetunnelclient.DialParams) (net.Conn, error) {
	if len(s.authServers) == 0 {
		return nil, trace.ConnectionProblem(nil, "no auth servers available")
	}

	addr := utils.ChooseRandomString(s.authServers)
	conn, err := net.DialTimeout("tcp", addr, apidefaults.DefaultIOTimeout)
	if err != nil {
		return nil, trace.ConnectionProblem(err, "unable to connect to auth server")
	}

	if err := s.maybeSendSignedPROXYHeader(params, conn, false); err != nil {
		return nil, trace.ConnectionProblem(err, "unable to send signed PROXY header to auth server")
	}

	return conn, nil
}

// shouldDialAndForward returns whether a connection should be proxied
// and forwarded or not.
func shouldDialAndForward(params reversetunnelclient.DialParams, recConfig types.SessionRecordingConfig) bool {
	// connection is already being tunneled, do not forward
	if params.FromPeerProxy {
		return false
	}
	// the node is an agentless node, the connection must be forwarded
	if params.TargetServer != nil && params.TargetServer.IsOpenSSHNode() {
		return true
	}
	// proxy session recording mode is being used and an SSH session
	// is being requested, the connection must be forwarded
	if params.ConnType == types.NodeTunnel && services.IsRecordAtProxy(recConfig.GetMode()) {
		return true
	}
	return false
}

func (s *localSite) Dial(params reversetunnelclient.DialParams) (net.Conn, error) {
	if params.TargetServer != nil && params.TargetServer.GetKind() == types.KindGitServer {
		return s.dialAndForwardGit(params)
	}

	recConfig, err := s.accessPoint.GetSessionRecordingConfig(s.srv.Context)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If the proxy is in recording mode and a SSH connection is being
	// requested or the target server is a registered OpenSSH node, build
	// an in-memory forwarding server.
	if shouldDialAndForward(params, recConfig) {
		return s.dialAndForward(params)
	}
	// Attempt to perform a direct TCP dial.
	return s.DialTCP(params)
}

func shouldSendSignedPROXYHeader(signer multiplexer.PROXYHeaderSigner, useTunnel, isAgentlessNode bool, srcAddr, dstAddr net.Addr) bool {
	return signer != nil &&
		!useTunnel &&
		!isAgentlessNode &&
		srcAddr != nil &&
		dstAddr != nil
}

func (s *localSite) maybeSendSignedPROXYHeader(params reversetunnelclient.DialParams, conn net.Conn, useTunnel bool) error {
	if !shouldSendSignedPROXYHeader(s.srv.proxySigner, useTunnel, params.IsAgentlessNode, params.From, params.OriginalClientDstAddr) {
		return nil
	}

	header, err := s.srv.proxySigner.SignPROXYHeader(params.From, params.OriginalClientDstAddr)
	if err != nil {
		return trace.Wrap(err, "could not create signed PROXY header")
	}

	_, err = conn.Write(header)
	if err != nil {
		return trace.Wrap(err, "could not write signed PROXY header into connection")
	}
	return nil
}

// TODO(awly): unit test this
func (s *localSite) DialTCP(params reversetunnelclient.DialParams) (net.Conn, error) {
	ctx := s.srv.ctx
	logger := s.logger.With("dial_params", logutils.StringerAttr(params))
	logger.DebugContext(ctx, "Initiating dial request")

	conn, useTunnel, err := s.getConn(params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logger.DebugContext(ctx, "Succeeded dialing")

	if err := s.maybeSendSignedPROXYHeader(params, conn, useTunnel); err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
}

// IsClosed always returns false because localSite is never closed.
func (s *localSite) IsClosed() bool { return false }

// Close always returns nil because a localSite isn't closed.
func (s *localSite) Close() error { return nil }

// adviseReconnect sends reconnects to agents in the background blocking until
// the requests complete or the context is done.
func (s *localSite) adviseReconnect(ctx context.Context) {
	wg := &sync.WaitGroup{}
	s.remoteConnsMtx.Lock()
	for _, conns := range s.remoteConns {
		for _, conn := range conns {
			s.logger.DebugContext(ctx, "Sending reconnect to server ", "server_id", conn.nodeID)

			wg.Add(1)
			go func(conn *remoteConn) {
				if err := conn.adviseReconnect(); err != nil {
					s.logger.WarnContext(ctx, "Failed sending reconnect advisory", "error", err)
				}
				wg.Done()
			}(conn)
		}
	}
	s.remoteConnsMtx.Unlock()

	wait := make(chan struct{})
	go func() {
		wg.Wait()
		close(wait)
	}()

	select {
	case <-ctx.Done():
	case <-wait:
	}
}

func (s *localSite) dialAndForwardGit(params reversetunnelclient.DialParams) (_ net.Conn, retErr error) {
	s.logger.DebugContext(s.srv.ctx, "Dialing and forwarding git", "from", params.From, "to", params.To)

	dialStart := s.srv.Clock.Now()
	targetConn, err := s.dialDirect(params)
	if err != nil {
		return nil, trace.ConnectionProblem(err, "failed to connect to git server")
	}

	// Get a host certificate for the forwarding node from the cache.
	hostCertificate, err := s.certificateCache.getHostCertificate(context.TODO(), params.Address, params.Principals)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a forwarding server that serves a single SSH connection on it. This
	// server does not need to close, it will close and release all resources
	// once conn is closed.
	serverConfig := &git.ForwardServerConfig{
		AuthClient:      s.client,
		AccessPoint:     s.accessPoint,
		TargetConn:      newMetricConn(targetConn, dialTypeDirect, dialStart, s.srv.Clock),
		SrcAddr:         params.From,
		DstAddr:         params.To,
		HostCertificate: hostCertificate,
		Ciphers:         s.srv.Config.Ciphers,
		KEXAlgorithms:   s.srv.Config.KEXAlgorithms,
		MACAlgorithms:   s.srv.Config.MACAlgorithms,
		Emitter:         s.srv.Config.Emitter,
		ParentContext:   s.srv.Context,
		LockWatcher:     s.srv.LockWatcher,
		HostUUID:        s.srv.ID,
		TargetServer:    params.TargetServer,
		Clock:           s.clock,
		KeyManager:      s.srv.gitKeyManager,
	}
	remoteServer, err := git.NewForwardServer(serverConfig)
	if err != nil {
		s.logger.ErrorContext(s.srv.ctx, "Failed to create git forward server", "error", err)
		return nil, trace.Wrap(err)
	}
	go remoteServer.Serve()

	return remoteServer.Dial()
}

func (s *localSite) dialAndForward(params reversetunnelclient.DialParams) (_ net.Conn, retErr error) {
	ctx := s.srv.ctx

	if params.GetUserAgent == nil && !params.IsAgentlessNode {
		return nil, trace.BadParameter("agentless node require an agent getter")
	}
	s.logger.DebugContext(ctx, "Initiating dial and forwarding request",
		"source_addr", logutils.StringerAttr(params.From),
		"target_addr", logutils.StringerAttr(params.To),
	)

	// request user agent connection if a SSH user agent is set
	var userAgent teleagent.Agent
	if params.GetUserAgent != nil {
		ua, err := params.GetUserAgent()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		userAgent = ua
		defer func() {
			if retErr != nil {
				retErr = trace.NewAggregate(retErr, userAgent.Close())
			}
		}()
	}

	// If server ID matches a node that has self registered itself over the tunnel,
	// return a connection to that node. Otherwise net.Dial to the target host.
	targetConn, useTunnel, err := s.getConn(params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.maybeSendSignedPROXYHeader(params, targetConn, useTunnel); err != nil {
		return nil, trace.Wrap(err)
	}

	// Get a host certificate for the forwarding node from the cache.
	hostCertificate, err := s.certificateCache.getHostCertificate(ctx, params.Address, params.Principals)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a forwarding server that serves a single SSH connection on it. This
	// server does not need to close, it will close and release all resources
	// once conn is closed.
	serverConfig := forward.ServerConfig{
		LocalAuthClient:          s.client,
		TargetClusterAccessPoint: s.accessPoint,
		UserAgent:                userAgent,
		IsAgentlessNode:          params.IsAgentlessNode,
		AgentlessSigner:          params.AgentlessSigner,
		TargetConn:               targetConn,
		SrcAddr:                  params.From,
		DstAddr:                  params.To,
		HostCertificate:          hostCertificate,
		Ciphers:                  s.srv.Config.Ciphers,
		KEXAlgorithms:            s.srv.Config.KEXAlgorithms,
		MACAlgorithms:            s.srv.Config.MACAlgorithms,
		DataDir:                  s.srv.Config.DataDir,
		Address:                  params.Address,
		UseTunnel:                useTunnel,
		HostUUID:                 s.srv.ID,
		Emitter:                  s.srv.Config.Emitter,
		ParentContext:            s.srv.Context,
		LockWatcher:              s.srv.LockWatcher,
		TargetID:                 params.ServerID,
		TargetAddr:               params.To.String(),
		TargetHostname:           params.Address,
		TargetServer:             params.TargetServer,
		Clock:                    s.clock,
		EICESigner:               s.srv.EICESigner,
	}
	// Ensure the hostname is set correctly if we have details of the target
	if params.TargetServer != nil {
		serverConfig.TargetHostname = params.TargetServer.GetHostname()
	}
	remoteServer, err := forward.New(serverConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go remoteServer.Serve()

	// Return a connection to the forwarding server.
	conn, err := remoteServer.Dial()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
}

// dialTunnel connects to the target host through a tunnel.
func (s *localSite) dialTunnel(dreq *sshutils.DialReq) (net.Conn, error) {
	rconn, err := s.getRemoteConn(dreq)
	if err != nil {
		return nil, trace.NotFound("no tunnel connection found: %v", err)
	}

	s.logger.DebugContext(s.srv.ctx, "Tunnel dialing to host",
		"target_host_id", dreq.ServerID,
		"src_addr", dreq.ClientSrcAddr,
	)

	conn, err := s.chanTransportConn(rconn, dreq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
}

func (s *localSite) dialDirect(params reversetunnelclient.DialParams) (net.Conn, error) {
	dialer := proxyutils.DialerFromEnvironment(params.To.String())

	dialTimeout := apidefaults.DefaultIOTimeout
	if cnc, err := s.accessPoint.GetClusterNetworkingConfig(s.srv.Context); err != nil {
		s.logger.WarnContext(s.srv.ctx, "Failed to get cluster networking config - using default dial timeout", "error", err)
	} else {
		dialTimeout = cnc.GetSSHDialTimeout()
	}
	return dialer.DialTimeout(s.srv.Context, params.To.Network(), params.To.String(), dialTimeout)
}

// tryProxyPeering determines whether the node should try to be reached over
// a peer proxy.
func (s *localSite) tryProxyPeering(params reversetunnelclient.DialParams) bool {
	if s.peerClient == nil {
		return false
	}
	if params.FromPeerProxy {
		return false
	}
	if params.ConnType == "" || params.ConnType == types.ProxyTunnel {
		return false
	}

	return true
}

// skipDirectDial determines if a direct dial attempt should be made.
func (s *localSite) skipDirectDial(params reversetunnelclient.DialParams) (bool, error) {
	// Connections to application and database servers should never occur
	// over a direct dial.
	switch params.ConnType {
	case types.KubeTunnel, types.NodeTunnel, types.ProxyTunnel, types.WindowsDesktopTunnel:
	case types.AppTunnel, types.DatabaseTunnel, types.OktaTunnel:
		return true, nil
	default:
		return true, trace.BadParameter("unknown tunnel type: %s", params.ConnType)
	}

	// Never direct dial when the client is already connecting from
	// a peer proxy.
	if params.FromPeerProxy {
		return true, nil
	}

	// This node can only be reached over a tunnel, don't attempt to dial
	// directly.
	if params.To == nil || params.To.String() == "" || params.To.String() == reversetunnelclient.LocalNode {
		return true, nil
	}

	return false, nil
}

func getTunnelErrorMessage(params reversetunnelclient.DialParams, connStr string, err error) string {
	errorMessageTemplate := `Teleport proxy failed to connect to %q agent %q over %s:

  %v

This usually means that the agent is offline or has disconnected. Check the
agent logs and, if the issue persists, try restarting it or re-registering it
with the cluster.`

	var toAddr string
	if params.To != nil {
		toAddr = params.To.String()
	}

	// Prefer providing the hostname over an address.
	if params.TargetServer != nil {
		toAddr = params.TargetServer.GetHostname()
	}

	return fmt.Sprintf(errorMessageTemplate, params.ConnType, toAddr, connStr, err)
}

func stringOrEmpty(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	return addr.String()
}

func (s *localSite) getConn(params reversetunnelclient.DialParams) (conn net.Conn, useTunnel bool, err error) {
	dialStart := s.srv.Clock.Now()

	// Creates a connection to the target EC2 instance using its private IP.
	// This relies on EC2 Instance Connect Endpoint service.
	if params.TargetServer != nil && params.TargetServer.GetSubKind() == types.SubKindOpenSSHEICENode {
		awsInfo := params.TargetServer.GetAWSInfo()
		if awsInfo == nil {
			return nil, false, trace.BadParameter("missing aws cloud metadata")
		}

		token, err := s.client.GenerateAWSOIDCToken(s.srv.ctx, awsInfo.Integration)
		if err != nil {
			return nil, false, trace.BadParameter("failed to generate aws token: %v", err)
		}

		integration, err := s.client.GetIntegration(s.srv.ctx, awsInfo.Integration)
		if err != nil {
			return nil, false, trace.BadParameter("failed to fetch integration details: %v", err)
		}

		conn, err := s.srv.EICEDialer(s.srv.ctx, params.TargetServer, integration, token)
		if err != nil {
			return nil, false, trace.Wrap(err, "failed dialing instance")
		}

		return newMetricConn(conn, dialTypeDirect, dialStart, s.srv.Clock), false, nil
	}

	dreq := &sshutils.DialReq{
		ServerID:      params.ServerID,
		ConnType:      params.ConnType,
		ClientSrcAddr: stringOrEmpty(params.From),
		ClientDstAddr: stringOrEmpty(params.OriginalClientDstAddr),
	}
	if params.To != nil {
		dreq.Address = params.To.String()
	}

	var (
		tunnelErr error
		peerErr   error
		directErr error
	)

	// If server ID matches a node that has self registered itself over the tunnel,
	// return a tunnel connection to that node. Otherwise net.Dial to the target host.
	conn, tunnelErr = s.dialTunnel(dreq)
	if tunnelErr == nil {
		dt := dialTypeTunnel
		if params.FromPeerProxy {
			dt = dialTypePeerTunnel
		}

		return newMetricConn(conn, dt, dialStart, s.srv.Clock), true, nil
	}

	peeringEnabled := s.tryProxyPeering(params)
	if peeringEnabled {
		s.logger.InfoContext(s.srv.ctx, "Dialing over peer proxy")
		conn, peerErr = s.peerClient.DialNode(
			params.ProxyIDs, params.ServerID, params.From, params.To, params.ConnType,
		)
		if peerErr == nil {
			return newMetricConn(conn, dialTypePeer, dialStart, s.srv.Clock), true, nil
		}
	}

	// If a connection via tunnel failed directly and via a remote peer,
	// then update the tunnel message to indicate that tunnels were not
	// found in either place. Avoid aggregating the local and peer errors
	// to reduce duplicate data since this message makes its way back to
	// users and can be confusing.
	msg := "reverse tunnel"
	if peeringEnabled {
		msg = "local and peer reverse tunnels"
	}
	tunnelMsg := getTunnelErrorMessage(params, msg, tunnelErr)

	// Skip direct dial when the tunnel error is not a not found error. This
	// means the agent is tunneling but the connection failed for some reason.
	if !trace.IsNotFound(tunnelErr) {
		return nil, false, trace.ConnectionProblem(tunnelErr, "%s", tunnelMsg)
	}

	skip, err := s.skipDirectDial(params)
	if err != nil {
		return nil, false, trace.Wrap(err)
	} else if skip {
		return nil, false, trace.ConnectionProblem(tunnelErr, "%s", tunnelMsg)
	}

	// If no tunnel connection was found, dial to the target host.
	conn, directErr = s.dialDirect(params)
	if directErr != nil {
		directMsg := getTunnelErrorMessage(params, "direct dial", directErr)
		s.logger.DebugContext(s.srv.ctx, "All attempted dial methods failed",
			"target_addr", logutils.StringerAttr(params.To),
			"tunnel_error", tunnelErr,
			"peer_error", peerErr,
			"direct_error", directErr,
		)
		aggregateErr := trace.NewAggregate(tunnelErr, peerErr, directErr)
		return nil, false, trace.ConnectionProblem(aggregateErr, "%s", directMsg)
	}

	// Return a direct dialed connection.
	return newMetricConn(conn, dialTypeDirect, dialStart, s.srv.Clock), false, nil
}

func (s *localSite) addConn(nodeID string, connType types.TunnelType, conn net.Conn, sconn ssh.Conn) (*remoteConn, error) {
	s.remoteConnsMtx.Lock()
	defer s.remoteConnsMtx.Unlock()

	rconn := newRemoteConn(&connConfig{
		conn:             conn,
		sconn:            sconn,
		tunnelType:       string(connType),
		proxyName:        s.srv.ID,
		clusterName:      s.domainName,
		nodeID:           nodeID,
		offlineThreshold: s.offlineThreshold,
	})
	key := connKey{
		uuid:     nodeID,
		connType: connType,
	}
	s.remoteConns[key] = append(s.remoteConns[key], rconn)

	return rconn, nil
}

// fanOutProxies is a non-blocking call that puts the new proxies
// list so that remote connection can notify the remote agent
// about the list update
func (s *localSite) fanOutProxies(proxies []types.Server) {
	s.remoteConnsMtx.Lock()
	defer s.remoteConnsMtx.Unlock()

	for _, conns := range s.remoteConns {
		for _, conn := range conns {
			conn.updateProxies(proxies)
		}
	}
}

// handleHeartbeat receives heartbeat messages from the connected agent
// if the agent has missed several heartbeats in a row, Proxy marks
// the connection as invalid.
func (s *localSite) handleHeartbeat(ctx context.Context, rconn *remoteConn, ch ssh.Channel, reqC <-chan *ssh.Request) {
	sshutils.DiscardChannelData(ch)
	if ch != nil {
		defer func() {
			if err := ch.Close(); err != nil {
				s.logger.WarnContext(ctx, "Failed to close heartbeat channel", "error", err)
			}
		}()
	}

	logger := s.logger.With(
		"server_id", rconn.nodeID,
		"addr", logutils.StringerAttr(rconn.conn.RemoteAddr()),
	)

	firstHeartbeat := true
	proxyResyncTicker := s.clock.NewTicker(s.proxySyncInterval)
	defer func() {
		proxyResyncTicker.Stop()
		logger.WarnContext(ctx, "Closing remote connection to agent")
		s.removeRemoteConn(rconn)
		if err := rconn.Close(); err != nil && !utils.IsOKNetworkError(err) {
			logger.WarnContext(ctx, "Failed to close remote connection", "error", err)
		}
		if !firstHeartbeat {
			reverseSSHTunnels.WithLabelValues(rconn.tunnelType).Dec()
		}
	}()

	offlineThresholdTimer := s.clock.NewTimer(s.offlineThreshold)
	defer offlineThresholdTimer.Stop()
	for {
		select {
		case <-s.srv.ctx.Done():
			logger.InfoContext(ctx, "Closing")
			return
		case <-proxyResyncTicker.Chan():
			var req discoveryRequest
			proxies, err := s.srv.proxyWatcher.CurrentResources(ctx)
			if err != nil {
				logger.WarnContext(ctx, "Failed to get proxy set", "error", err)
			}
			req.SetProxies(proxies)

			if err := rconn.sendDiscoveryRequest(ctx, req); err != nil {
				logger.DebugContext(ctx, "Marking connection invalid on error", "error", err)
				rconn.markInvalid(err)
				return
			}
		case proxies := <-rconn.newProxiesC:
			var req discoveryRequest
			req.SetProxies(proxies)

			if err := rconn.sendDiscoveryRequest(ctx, req); err != nil {
				logger.DebugContext(ctx, "Failed to send discovery request to agent", "error", err)
				rconn.markInvalid(err)
				return
			}
		case req := <-reqC:
			if req == nil {
				logger.DebugContext(ctx, "Agent disconnected")
				rconn.markInvalid(trace.ConnectionProblem(nil, "agent disconnected"))
				return
			}
			if firstHeartbeat {
				// as soon as the agent connects and sends a first heartbeat
				// send it the list of current proxies back
				proxies, err := s.srv.proxyWatcher.CurrentResources(s.srv.ctx)
				if err != nil {
					logger.WarnContext(ctx, "Failed to get proxy set", "error", err)
				}
				if len(proxies) > 0 {
					rconn.updateProxies(proxies)
				}
				reverseSSHTunnels.WithLabelValues(rconn.tunnelType).Inc()
				firstHeartbeat = false
			}
			var timeSent time.Time
			var roundtrip time.Duration
			if req.Payload != nil {
				if err := timeSent.UnmarshalText(req.Payload); err == nil {
					roundtrip = s.srv.Clock.Now().Sub(timeSent)
				}
			}

			log := logger
			if roundtrip != 0 {
				log = logger.With("latency", logutils.StringerAttr(roundtrip))
			}
			log.DebugContext(ctx, "Received ping request", "remote_addr", logutils.StringerAttr(rconn.conn.RemoteAddr()))

			rconn.setLastHeartbeat(s.clock.Now().UTC())
			rconn.markValid()
		case t := <-offlineThresholdTimer.Chan():
			rconn.markInvalid(trace.ConnectionProblem(nil, "no heartbeats for %v", s.offlineThreshold))

			// terminate and remove the connection if offline, otherwise warn and wait for the next heartbeat
			if rconn.isOffline(t, s.offlineThreshold*missedHeartBeatThreshold) {
				logger.ErrorContext(ctx, "Closing unhealthy and idle connection", "last_heartbeat", rconn.getLastHeartbeat())
				return
			}
			logger.WarnContext(ctx, "Deferring closure of unhealthy connection due to active connections", "active_conn_count", rconn.activeSessions())

			offlineThresholdTimer.Reset(s.offlineThreshold)
			continue
		}

		if !offlineThresholdTimer.Stop() {
			<-offlineThresholdTimer.Chan()
		}
		offlineThresholdTimer.Reset(s.offlineThreshold)
	}
}

func (s *localSite) removeRemoteConn(rconn *remoteConn) {
	s.remoteConnsMtx.Lock()
	defer s.remoteConnsMtx.Unlock()

	key := connKey{
		uuid:     rconn.nodeID,
		connType: types.TunnelType(rconn.tunnelType),
	}

	conns := s.remoteConns[key]
	for i, conn := range conns {
		if conn == rconn {
			s.remoteConns[key] = append(conns[:i], conns[i+1:]...)
			if len(s.remoteConns[key]) == 0 {
				delete(s.remoteConns, key)
			}
			return
		}
	}
}

func (s *localSite) getRemoteConn(dreq *sshutils.DialReq) (*remoteConn, error) {
	s.remoteConnsMtx.Lock()
	defer s.remoteConnsMtx.Unlock()

	key := connKey{
		uuid:     dreq.ServerID,
		connType: dreq.ConnType,
	}

	conns := s.remoteConns[key]
	if len(conns) == 0 {
		return nil, trace.NotFound("no %v reverse tunnel for %v found", dreq.ConnType, dreq.ServerID)
	}

	// Check the remoteConns from newest to oldest for one
	// that has heartbeated and is valid. If none are valid, try
	// the newest ready but invalid connection.
	var newestInvalidConn *remoteConn
	for i := len(conns) - 1; i >= 0; i-- {
		switch {
		case !conns[i].isReady(): // skip remoteConn that haven't heartbeated yet
			continue
		case !conns[i].isInvalid(): // return the first valid remoteConn that has heartbeated
			return conns[i], nil
		case newestInvalidConn == nil && conns[i].isInvalid(): // cache the first invalid remoteConn in case none are valid
			newestInvalidConn = conns[i]
		}
	}

	// This indicates that there were no ready and valid connections, but at least
	// one ready and invalid connection. We can at least attempt to connect on the
	// invalid connection instead of giving up entirely. If anything the error might
	// be more informative than the default offline message returned below.
	if newestInvalidConn != nil {
		return newestInvalidConn, nil
	}

	// The agent is having issues and there is no way to connect
	return nil, trace.NotFound("%v is offline: no active %v tunnels found", dreq.ConnType, dreq.ServerID)
}

func (s *localSite) chanTransportConn(rconn *remoteConn, dreq *sshutils.DialReq) (net.Conn, error) {
	s.logger.DebugContext(s.srv.ctx, "Connecting to target through tunnel", "target_addr", logutils.StringerAttr(rconn.conn.RemoteAddr()))

	conn, markInvalid, err := sshutils.ConnectProxyTransport(rconn.sconn, dreq, false)
	if err != nil {
		if markInvalid {
			rconn.markInvalid(err)
			// If not serving any connections close and remove this connection immediately.
			// Otherwise, let the heartbeat handler detect this connection is down.
			if rconn.activeSessions() == 0 {
				s.removeRemoteConn(rconn)
				return nil, trace.NewAggregate(trace.Wrap(err), rconn.Close())
			}
		}
		return nil, trace.Wrap(err)
	}

	return newSessionTrackingConn(rconn, conn), nil
}

// sessionTrackingConn wraps a net.Conn in order
// to maintain the number of active sessions for
// a remoteConn.
type sessionTrackingConn struct {
	net.Conn
	rc *remoteConn
}

// newSessionTrackingConn wraps the provided net.Conn to alert the remoteConn
// when it is no longer active. Prior to returning the remoteConn active sessions
// are incremented. Close must be called to decrement the count.
func newSessionTrackingConn(rconn *remoteConn, conn net.Conn) *sessionTrackingConn {
	rconn.incrementActiveSessions()
	return &sessionTrackingConn{
		rc:   rconn,
		Conn: conn,
	}
}

// Close decrements the remoteConn active session count and then
// closes the underlying net.Conn
func (c *sessionTrackingConn) Close() error {
	c.rc.decrementActiveSessions()
	return c.Conn.Close()
}

// periodicFunctions runs functions periodic functions for the local cluster.
func (s *localSite) periodicFunctions() {
	ticker := s.clock.NewTicker(s.periodicFunctionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.srv.ctx.Done():
			return
		case <-ticker.Chan():
			if err := s.sshTunnelStats(); err != nil {
				s.logger.WarnContext(s.srv.ctx, "Failed to report SSH tunnel statistics ", "cluster", s.domainName, "error", err)
			}
		}
	}
}

// sshTunnelStats reports SSH tunnel statistics for the cluster.
func (s *localSite) sshTunnelStats() error {
	missing, err := s.srv.NodeWatcher.CurrentResourcesWithFilter(s.srv.ctx, func(server readonly.Server) bool {
		// Skip over any servers that have a TTL larger than announce TTL (10
		// minutes) and are non-IoT SSH servers (they won't have tunnels).
		//
		// Servers with a TTL larger than the announce TTL skipped over to work around
		// an issue with DynamoDB where objects can hang around for 48 hours after
		// their TTL value.
		ttl := s.clock.Now().Add(-1 * apidefaults.ServerAnnounceTTL)
		if server.Expiry().Before(ttl) {
			return false
		}
		if !server.GetUseTunnel() {
			return false
		}

		ids := server.GetProxyIDs()

		// In proxy peering mode, a node is expected to be connected to the
		// current proxy if the proxy id is present. A node is expected to be
		// connected to all proxies if no proxy ids are present.
		if s.peerClient != nil && len(ids) != 0 && !slices.Contains(ids, s.srv.ID) {
			return false
		}

		// Check if the tunnel actually exists.
		_, err := s.getRemoteConn(&sshutils.DialReq{
			ServerID: fmt.Sprintf("%v.%v", server.GetName(), s.domainName),
			ConnType: types.NodeTunnel,
		})

		return err != nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Update Prometheus metrics and also log if any tunnels are missing.
	missingSSHTunnels.Set(float64(len(missing)))

	if len(missing) > 0 {
		// Don't show all the missing nodes, thousands could be missing, just show
		// the first 10.
		n := len(missing)
		if n > 10 {
			n = 10
		}
		s.logger.DebugContext(s.srv.ctx, "Cluster is missing some tunnels. A small number of missing tunnels is normal, for example, a node could have just been shut down, the proxy restarted, etc. However, if this error persists with an elevated number of missing tunnels, it often indicates nodes can not discover all registered proxies. Check that all of your proxies are behind a load balancer and the load balancer is using a round robin strategy",
			"cluster", s.domainName,
			"missing_count", len(missing),
			"missing", missing[:n],
		)
	}
	return nil
}

var (
	missingSSHTunnels = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: teleport.MetricMissingSSHTunnels,
			Help: "Number of missing SSH tunnels",
		},
	)
	reverseSSHTunnels = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricReverseSSHTunnels,
			Help:      "Number of reverse SSH tunnels connected to the Teleport Proxy Service by Teleport instances",
		},
		[]string{teleport.TagType},
	)

	localClusterCollectors = []prometheus.Collector{missingSSHTunnels, reverseSSHTunnels, connLatency}
)
