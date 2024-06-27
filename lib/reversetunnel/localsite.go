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
	"net"
	"slices"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/proxy/peer"
	"github.com/gravitational/teleport/lib/reversetunnel/track"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/forward"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"
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

// withCertificateCache sets the certificateCache of the site. This is particularly
// helpful for tests because construction of the default cache will
// call [native.PrecomputeKeys] which will consume a decent amount of CPU
// to generate keys.
func withCertificateCache(cache *certificateCache) func(site *localSite) {
	return func(site *localSite) {
		site.certificateCache = cache
	}
}

func newLocalSite(srv *server, domainName string, authServers []string, opts ...func(*localSite)) (*localSite, error) {
	err := metrics.RegisterPrometheusCollectors(localClusterCollectors...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s := &localSite{
		srv:         srv,
		client:      srv.localAuthClient,
		accessPoint: srv.LocalAccessPoint,
		domainName:  domainName,
		authServers: authServers,
		remoteConns: make(map[connKey][]*remoteConn),
		clock:       srv.Clock,
		log: log.WithFields(log.Fields{
			teleport.ComponentKey: teleport.ComponentReverseTunnelServer,
			teleport.ComponentFields: map[string]string{
				"cluster": domainName,
			},
		}),
		offlineThreshold:         srv.offlineThreshold,
		peerClient:               srv.PeerClient,
		periodicFunctionInterval: periodicFunctionInterval,
		proxySyncInterval:        proxySyncInterval,
	}

	for _, opt := range opts {
		opt(s)
	}

	if s.certificateCache == nil {
		// instantiate a cache of host certificates for the forwarding server. the
		// certificate cache is created in each site (instead of creating it in
		// reversetunnel.server and passing it along) so that the host certificate
		// is signed by the correct certificate authority.
		certificateCache, err := newHostCertificateCache(srv.localAuthClient)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		s.certificateCache = certificateCache
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
	log         log.FieldLogger
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
func (s *localSite) NodeWatcher() (*services.NodeWatcher, error) {
	return s.srv.NodeWatcher, nil
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
	return !(signer == nil ||
		useTunnel ||
		isAgentlessNode ||
		srcAddr == nil ||
		dstAddr == nil)
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
	s.log.Debugf("Dialing %v.", params)

	conn, useTunnel, err := s.getConn(params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.log.Debugf("Succeeded dialing %v.", params)

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
			s.log.Debugf("Sending reconnect: %s", conn.nodeID)

			wg.Add(1)
			go func(conn *remoteConn) {
				if err := conn.adviseReconnect(); err != nil {
					s.log.WithError(err).Warn("Failed sending reconnect advisory")
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

func (s *localSite) dialAndForward(params reversetunnelclient.DialParams) (_ net.Conn, retErr error) {
	if params.GetUserAgent == nil && !params.IsAgentlessNode {
		return nil, trace.BadParameter("agentless node require an agent getter")
	}
	s.log.Debugf("Dialing and forwarding from %v to %v.", params.From, params.To)

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
	hostCertificate, err := s.certificateCache.getHostCertificate(context.TODO(), params.Address, params.Principals)
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

	s.log.Debugf("Tunnel dialing to %v, client source %v", dreq.ServerID, dreq.ClientSrcAddr)

	conn, err := s.chanTransportConn(rconn, dreq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
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

	return fmt.Sprintf(errorMessageTemplate, params.ConnType, toAddr, connStr, err)
}

func stringOrEmpty(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	return addr.String()
}

func (s *localSite) setupTunnelForOpenSSHEICENode(ctx context.Context, targetServer types.Server) (*awsoidc.OpenTunnelEC2Response, error) {
	awsInfo := targetServer.GetAWSInfo()
	if awsInfo == nil {
		return nil, trace.BadParameter("missing aws cloud metadata")
	}

	token, err := s.client.GenerateAWSOIDCToken(ctx, awsInfo.Integration)
	if err != nil {
		return nil, trace.BadParameter("failed to generate aws token: %v", err)
	}

	integration, err := s.client.GetIntegration(ctx, awsInfo.Integration)
	if err != nil {
		return nil, trace.BadParameter("failed to fetch integration details: %v", err)
	}

	if integration.GetAWSOIDCIntegrationSpec() == nil {
		return nil, trace.BadParameter("integration does not have aws oidc spec fields %q", awsInfo.Integration)
	}

	openTunnelClt, err := awsoidc.NewOpenTunnelEC2Client(ctx, &awsoidc.AWSClientRequest{
		IntegrationName: integration.GetName(),
		Token:           token,
		RoleARN:         integration.GetAWSOIDCIntegrationSpec().RoleARN,
		Region:          awsInfo.Region,
	})
	if err != nil {
		return nil, trace.BadParameter("failed to create the ec2 open tunnel client: %v", err)
	}

	openTunnelResp, err := awsoidc.OpenTunnelEC2(ctx, openTunnelClt, awsoidc.OpenTunnelEC2Request{
		Region:        awsInfo.Region,
		VPCID:         awsInfo.VPCID,
		EC2InstanceID: awsInfo.InstanceID,
		EC2Address:    targetServer.GetAddr(),
	})
	if err != nil {
		return nil, trace.BadParameter("failed to open AWS EC2 Instance Connect Endpoint tunnel: %v", err)
	}

	// OpenTunnelResp has the tcp connection that should be used to access the EC2 instance directly.
	return openTunnelResp, nil
}

func (s *localSite) getConn(params reversetunnelclient.DialParams) (conn net.Conn, useTunnel bool, err error) {
	dialStart := s.srv.Clock.Now()

	// Creates a connection to the target EC2 instance using its private IP.
	// This relies on EC2 Instance Connect Endpoint service.
	if params.TargetServer != nil && params.TargetServer.GetSubKind() == types.SubKindOpenSSHEICENode {
		eiceTunnelConn, err := s.setupTunnelForOpenSSHEICENode(s.srv.Context, params.TargetServer)
		if err != nil {
			return nil, false, trace.Wrap(err)
		}

		return newMetricConn(eiceTunnelConn.Tunnel, dialTypeDirect, dialStart, s.srv.Clock), false, nil
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

	if s.tryProxyPeering(params) {
		s.log.Info("Dialing over peer proxy")
		conn, peerErr = s.peerClient.DialNode(
			params.ProxyIDs, params.ServerID, params.From, params.To, params.ConnType,
		)
		if peerErr == nil {
			return newMetricConn(conn, dialTypePeer, dialStart, s.srv.Clock), true, nil
		}
	}

	err = trace.NewAggregate(tunnelErr, peerErr)
	tunnelMsg := getTunnelErrorMessage(params, "reverse tunnel", err)

	// Skip direct dial when the tunnel error is not a not found error. This
	// means the agent is tunneling but the connection failed for some reason.
	if !trace.IsNotFound(tunnelErr) {
		return nil, false, trace.ConnectionProblem(err, tunnelMsg)
	}

	skip, err := s.skipDirectDial(params)
	if err != nil {
		return nil, false, trace.Wrap(err)
	} else if skip {
		return nil, false, trace.ConnectionProblem(err, tunnelMsg)
	}

	// If no tunnel connection was found, dial to the target host.
	dialer := proxyutils.DialerFromEnvironment(params.To.String())
	conn, directErr = dialer.DialTimeout(s.srv.Context, params.To.Network(), params.To.String(), apidefaults.DefaultIOTimeout)
	if directErr != nil {
		directMsg := getTunnelErrorMessage(params, "direct dial", directErr)
		s.log.WithField("address", params.To.String()).Debugf("All attempted dial methods failed. tunnel=%q, peer=%q, direct=%q", tunnelErr, peerErr, directErr)
		aggregateErr := trace.NewAggregate(tunnelErr, peerErr, directErr)
		return nil, false, trace.ConnectionProblem(aggregateErr, directMsg)
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
func (s *localSite) handleHeartbeat(rconn *remoteConn, ch ssh.Channel, reqC <-chan *ssh.Request) {
	sshutils.DiscardChannelData(ch)
	if ch != nil {
		defer func() {
			if err := ch.Close(); err != nil {
				s.log.Warnf("Failed to close heartbeat channel: %v", err)
			}
		}()
	}

	logger := s.log.WithFields(log.Fields{
		"serverID": rconn.nodeID,
		"addr":     rconn.conn.RemoteAddr().String(),
	})

	firstHeartbeat := true
	proxyResyncTicker := s.clock.NewTicker(s.proxySyncInterval)
	defer func() {
		proxyResyncTicker.Stop()
		logger.Warn("Closing remote connection to agent.")
		s.removeRemoteConn(rconn)
		if err := rconn.Close(); err != nil && !utils.IsOKNetworkError(err) {
			logger.WithError(err).Warn("Failed to close remote connection")
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
			logger.Info("Closing")
			return
		case <-proxyResyncTicker.Chan():
			var req discoveryRequest
			req.SetProxies(s.srv.proxyWatcher.GetCurrent())

			if err := rconn.sendDiscoveryRequest(req); err != nil {
				logger.WithError(err).Debug("Marking connection invalid on error")
				rconn.markInvalid(err)
				return
			}
		case proxies := <-rconn.newProxiesC:
			var req discoveryRequest
			req.SetProxies(proxies)

			if err := rconn.sendDiscoveryRequest(req); err != nil {
				logger.WithError(err).Debug("Failed to send discovery request to agent")
				rconn.markInvalid(err)
				return
			}
		case req := <-reqC:
			if req == nil {
				logger.Debug("Agent disconnected.")
				rconn.markInvalid(trace.ConnectionProblem(nil, "agent disconnected"))
				return
			}
			if firstHeartbeat {
				// as soon as the agent connects and sends a first heartbeat
				// send it the list of current proxies back
				current := s.srv.proxyWatcher.GetCurrent()
				if len(current) > 0 {
					rconn.updateProxies(current)
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
				log = logger.WithField("latency", roundtrip.String())
			}
			log.Debugf("Ping <- %v", rconn.conn.RemoteAddr())

			rconn.setLastHeartbeat(s.clock.Now().UTC())
			rconn.markValid()
		case t := <-offlineThresholdTimer.Chan():
			rconn.markInvalid(trace.ConnectionProblem(nil, "no heartbeats for %v", s.offlineThreshold))

			// terminate and remove the connection if offline, otherwise warn and wait for the next heartbeat
			if rconn.isOffline(t, s.offlineThreshold*missedHeartBeatThreshold) {
				logger.Errorf("Closing unhealthy and idle connection. Heartbeat last received at %s", rconn.getLastHeartbeat())
				return
			}
			logger.Warnf("Deferring closure of unhealthy connection due to %d active connections", rconn.activeSessions())

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
	s.log.Debugf("Connecting to %v through tunnel.", rconn.conn.RemoteAddr())

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
				s.log.Warningf("Failed to report SSH tunnel statistics for: %v: %v.", s.domainName, err)
			}
		}
	}
}

// sshTunnelStats reports SSH tunnel statistics for the cluster.
func (s *localSite) sshTunnelStats() error {
	missing := s.srv.NodeWatcher.GetNodes(s.srv.ctx, func(server services.Node) bool {
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

	// Update Prometheus metrics and also log if any tunnels are missing.
	missingSSHTunnels.Set(float64(len(missing)))

	if len(missing) > 0 {
		// Don't show all the missing nodes, thousands could be missing, just show
		// the first 10.
		n := len(missing)
		if n > 10 {
			n = 10
		}
		s.log.Debugf("Cluster %v is missing %v tunnels. A small number of missing tunnels is normal, for example, a node could have just been shut down, the proxy restarted, etc. However, if this error persists with an elevated number of missing tunnels, it often indicates nodes can not discover all registered proxies. Check that all of your proxies are behind a load balancer and the load balancer is using a round robin strategy. Some of the missing hosts: %v.", s.domainName, len(missing), missing[:n])
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
