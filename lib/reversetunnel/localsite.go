/*
Copyright 2016-2019 Gravitational, Inc.

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
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/proxy"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/forward"
	"github.com/gravitational/teleport/lib/utils"
	proxyutils "github.com/gravitational/teleport/lib/utils/proxy"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

func newlocalSite(srv *server, domainName string, authServers []string, client auth.ClientI, peerClient *proxy.Client) (*localSite, error) {
	err := utils.RegisterPrometheusCollectors(localClusterCollectors...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessPoint, err := srv.newAccessPoint(client, []string{"reverse", domainName})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// instantiate a cache of host certificates for the forwarding server. the
	// certificate cache is created in each site (instead of creating it in
	// reversetunnel.server and passing it along) so that the host certificate
	// is signed by the correct certificate authority.
	certificateCache, err := newHostCertificateCache(srv.Config.KeyGen, client)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s := &localSite{
		srv:              srv,
		client:           client,
		accessPoint:      accessPoint,
		certificateCache: certificateCache,
		domainName:       domainName,
		authServers:      authServers,
		remoteConns:      make(map[connKey][]*remoteConn),
		clock:            srv.Clock,
		log: log.WithFields(log.Fields{
			trace.Component: teleport.ComponentReverseTunnelServer,
			trace.ComponentFields: map[string]string{
				"cluster": domainName,
			},
		}),
		offlineThreshold: srv.offlineThreshold,
		peerClient:       peerClient,
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
	client auth.ClientI
	// accessPoint provides access to a cached subset of the Auth Server API of
	// the local cluster.
	accessPoint auth.RemoteProxyAccessPoint

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

	peerClient *proxy.Client
}

// GetTunnelsCount always the number of tunnel connections to this cluster.
func (s *localSite) GetTunnelsCount() int {
	s.remoteConnsMtx.Lock()
	defer s.remoteConnsMtx.Unlock()

	return len(s.remoteConns)
}

// CachingAccessPoint returns an auth.RemoteProxyAccessPoint for this cluster.
func (s *localSite) CachingAccessPoint() (auth.RemoteProxyAccessPoint, error) {
	return s.accessPoint, nil
}

// NodeWatcher returns a services.NodeWatcher for this cluster.
func (s *localSite) NodeWatcher() (*services.NodeWatcher, error) {
	return s.srv.NodeWatcher, nil
}

// GetClient returns a client to the full Auth Server API.
func (s *localSite) GetClient() (auth.ClientI, error) {
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

func (s *localSite) DialAuthServer() (net.Conn, error) {
	if len(s.authServers) == 0 {
		return nil, trace.ConnectionProblem(nil, "no auth servers available")
	}

	addr := utils.ChooseRandomString(s.authServers)
	conn, err := net.DialTimeout("tcp", addr, apidefaults.DefaultDialTimeout)
	if err != nil {
		return nil, trace.ConnectionProblem(err, "unable to connect to auth server")
	}

	return conn, nil
}

func (s *localSite) Dial(params DialParams) (net.Conn, error) {
	recConfig, err := s.accessPoint.GetSessionRecordingConfig(s.srv.Context)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If the proxy is in recording mode and a SSH connection is being requested,
	// use the agent to dial and build an in-memory forwarding server.
	if params.ConnType == types.NodeTunnel && services.IsRecordAtProxy(recConfig.GetMode()) && !params.FromPeerProxy {
		return s.dialWithAgent(params)
	}

	// Attempt to perform a direct TCP dial.
	return s.DialTCP(params)
}

// TODO(awly): unit test this
func (s *localSite) DialTCP(params DialParams) (net.Conn, error) {
	s.log.Debugf("Dialing %v.", params)

	conn, _, err := s.getConn(params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.log.Debugf("Succeeded dialing %v.", params)

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
				conn.adviseReconnect()
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

func (s *localSite) dialWithAgent(params DialParams) (net.Conn, error) {
	if params.GetUserAgent == nil {
		return nil, trace.BadParameter("user agent getter missing")
	}
	s.log.Debugf("Dialing with an agent from %v to %v.", params.From, params.To)

	// request user agent connection
	userAgent, err := params.GetUserAgent()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If server ID matches a node that has self registered itself over the tunnel,
	// return a connection to that node. Otherwise net.Dial to the target host.
	targetConn, useTunnel, err := s.getConn(params)
	if err != nil {
		userAgent.Close()
		return nil, trace.Wrap(err)
	}

	// Get a host certificate for the forwarding node from the cache.
	hostCertificate, err := s.certificateCache.getHostCertificate(params.Address, params.Principals)
	if err != nil {
		userAgent.Close()
		return nil, trace.Wrap(err)
	}

	// Create a forwarding server that serves a single SSH connection on it. This
	// server does not need to close, it will close and release all resources
	// once conn is closed.
	serverConfig := forward.ServerConfig{
		AuthClient:      s.client,
		UserAgent:       userAgent,
		TargetConn:      targetConn,
		SrcAddr:         params.From,
		DstAddr:         params.To,
		HostCertificate: hostCertificate,
		Ciphers:         s.srv.Config.Ciphers,
		KEXAlgorithms:   s.srv.Config.KEXAlgorithms,
		MACAlgorithms:   s.srv.Config.MACAlgorithms,
		DataDir:         s.srv.Config.DataDir,
		Address:         params.Address,
		UseTunnel:       useTunnel,
		HostUUID:        s.srv.ID,
		Emitter:         s.srv.Config.Emitter,
		ParentContext:   s.srv.Context,
		LockWatcher:     s.srv.LockWatcher,
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

	s.log.Debugf("Tunnel dialing to %v.", dreq.ServerID)

	conn, err := s.chanTransportConn(rconn, dreq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
}

// tryProxyPeering determines whether the node should try to be reached over
// a peer proxy.
func (s *localSite) tryProxyPeering(params DialParams) bool {
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
func (s *localSite) skipDirectDial(params DialParams) (bool, error) {
	// Connections to application and database servers should never occur
	// over a direct dial.
	switch params.ConnType {
	case types.KubeTunnel, types.NodeTunnel, types.ProxyTunnel, types.WindowsDesktopTunnel:
	case types.AppTunnel, types.DatabaseTunnel:
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
	// remotely.
	if params.To == nil || params.To.String() == "" || params.To.String() == LocalNode {
		return true, nil
	}

	return false, nil
}

func getTunnelErrorMessage(params DialParams, connStr string, err error) string {
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

func (s *localSite) getConn(params DialParams) (conn net.Conn, useTunnel bool, err error) {
	dreq := &sshutils.DialReq{
		ServerID: params.ServerID,
		ConnType: params.ConnType,
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
		return conn, true, nil
	}
	s.log.WithError(tunnelErr).WithField("address", dreq.Address).Debug("Error occurred while dialing through a tunnel.")

	if s.tryProxyPeering(params) {
		s.log.Info("Dialing over peer proxy")
		conn, peerErr = s.peerClient.DialNode(
			params.ProxyIDs, params.ServerID, params.From, params.To, params.ConnType,
		)
		if peerErr == nil {
			return conn, true, nil
		}
		s.log.WithError(peerErr).WithField("address", dreq.Address).Debug("Error occurred while dialing over peer proxy.")
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
	conn, directErr = dialer.DialTimeout(s.srv.Context, params.To.Network(), params.To.String(), apidefaults.DefaultDialTimeout)
	if directErr != nil {
		directMsg := getTunnelErrorMessage(params, "direct dial", directErr)
		s.log.WithError(directErr).WithField("address", params.To.String()).Debug("Error occurred while dialing directly.")
		aggregateErr := trace.NewAggregate(tunnelErr, peerErr, directErr)
		return nil, false, trace.ConnectionProblem(aggregateErr, directMsg)
	}

	// Return a direct dialed connection.
	return conn, false, nil
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

// handleHearbeat receives heartbeat messages from the connected agent
// if the agent has missed several heartbeats in a row, Proxy marks
// the connection as invalid.
func (s *localSite) handleHeartbeat(rconn *remoteConn, ch ssh.Channel, reqC <-chan *ssh.Request) {
	defer func() {
		s.log.Debugf("Cluster connection closed.")
		rconn.Close()
	}()

	firstHeartbeat := true
	for {
		select {
		case <-s.srv.ctx.Done():
			s.log.Infof("closing")
			return
		case proxies := <-rconn.newProxiesC:
			req := discoveryRequest{
				ClusterName: s.srv.ClusterName,
				Type:        rconn.tunnelType,
				Proxies:     proxies,
			}
			if err := rconn.sendDiscoveryRequest(req); err != nil {
				s.log.Debugf("Marking connection invalid on error: %v.", err)
				rconn.markInvalid(err)
				return
			}
		case req := <-reqC:
			if req == nil {
				s.log.Debugf("Cluster agent disconnected.")
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
				defer reverseSSHTunnels.WithLabelValues(rconn.tunnelType).Dec()
				firstHeartbeat = false
			}
			var timeSent time.Time
			var roundtrip time.Duration
			if req.Payload != nil {
				if err := timeSent.UnmarshalText(req.Payload); err == nil {
					roundtrip = s.srv.Clock.Now().Sub(timeSent)
				}
			}
			if roundtrip != 0 {
				s.log.WithFields(log.Fields{"latency": roundtrip, "nodeID": rconn.nodeID}).Debugf("Ping <- %v", rconn.conn.RemoteAddr())
			} else {
				s.log.WithFields(log.Fields{"nodeID": rconn.nodeID}).Debugf("Ping <- %v", rconn.conn.RemoteAddr())
			}
			tm := time.Now().UTC()
			rconn.setLastHeartbeat(tm)
		// Note that time.After is re-created everytime a request is processed.
		case <-time.After(s.offlineThreshold):
			rconn.markInvalid(trace.ConnectionProblem(nil, "no heartbeats for %v", s.offlineThreshold))
		}
	}
}

func (s *localSite) getRemoteConn(dreq *sshutils.DialReq) (*remoteConn, error) {
	s.remoteConnsMtx.Lock()
	defer s.remoteConnsMtx.Unlock()

	// Loop over all connections and remove and invalid connections from the
	// connection map.
	for key, conns := range s.remoteConns {
		validConns := conns[:0]
		for _, conn := range conns {
			if !conn.isInvalid() {
				validConns = append(validConns, conn)
			}
		}
		if len(validConns) == 0 {
			delete(s.remoteConns, key)
		} else {
			s.remoteConns[key] = validConns
		}
	}

	key := connKey{
		uuid:     dreq.ServerID,
		connType: dreq.ConnType,
	}
	if len(s.remoteConns[key]) == 0 {
		return nil, trace.NotFound("no %v reverse tunnel for %v found", dreq.ConnType, dreq.ServerID)
	}

	conns := s.remoteConns[key]
	for i := len(conns) - 1; i >= 0; i-- {
		if conns[i].isReady() {
			return conns[i], nil
		}
	}
	return nil, trace.NotFound("%v is offline: no active %v tunnels found", dreq.ConnType, dreq.ServerID)
}

func (s *localSite) chanTransportConn(rconn *remoteConn, dreq *sshutils.DialReq) (net.Conn, error) {
	s.log.Debugf("Connecting to %v through tunnel.", rconn.conn.RemoteAddr())

	conn, markInvalid, err := sshutils.ConnectProxyTransport(rconn.sconn, dreq, false)
	if err != nil {
		if markInvalid {
			rconn.markInvalid(err)
		}
		return nil, trace.Wrap(err)
	}

	return conn, nil
}

// periodicFunctions runs functions periodic functions for the local cluster.
func (s *localSite) periodicFunctions() {
	ticker := time.NewTicker(defaults.ResyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.srv.ctx.Done():
			return
		case <-ticker.C:
			if err := s.sshTunnelStats(); err != nil {
				s.log.Warningf("Failed to report SSH tunnel statistics for: %v: %v.", s.domainName, err)
			}
		}
	}
}

// sshTunnelStats reports SSH tunnel statistics for the cluster.
func (s *localSite) sshTunnelStats() error {
	missing := s.srv.NodeWatcher.GetNodes(func(server services.Node) bool {
		// Skip over any servers that that have a TTL larger than announce TTL (10
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

		// Check if the tunnel actually exists.
		_, err := s.getRemoteConn(&sshutils.DialReq{
			ServerID: fmt.Sprintf("%v.%v", server.GetName(), s.domainName),
			ConnType: types.NodeTunnel,
		})

		return err != nil
	})

	// Update Prometheus metrics and also log if any tunnels are missing.
	missingSSHTunnels.Set(float64(len(missing)))

	// Don't log if proxy peering is enabled as there will likely always be missing tunnels.
	if len(missing) > 0 && s.peerClient == nil {
		// Don't show all the missing nodes, thousands could be missing, just show
		// the first 10.
		n := len(missing)
		if n > 10 {
			n = 10
		}
		log.Debugf("Cluster %v is missing %v tunnels. A small number of missing tunnels is normal, for example, a node could have just been shut down, the proxy restarted, etc. However, if this error persists with an elevated number of missing tunnels, it often indicates nodes can not discover all registered proxies. Check that all of your proxies are behind a load balancer and the load balancer is using a round robin strategy. Some of the missing hosts: %v.", s.domainName, len(missing), missing[:n])
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

	localClusterCollectors = []prometheus.Collector{missingSSHTunnels, reverseSSHTunnels}
)
