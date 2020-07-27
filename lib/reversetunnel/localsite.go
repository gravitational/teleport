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
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/forward"
	"github.com/gravitational/teleport/lib/utils/proxy"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

func newlocalSite(srv *server, domainName string, client auth.ClientI) (*localSite, error) {
	accessPoint, err := srv.newAccessPoint(client, []string{"reverse", domainName})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// instantiate a cache of host certificates for the forwarding server. the
	// certificate cache is created in each site (instead of creating it in
	// reversetunnel.server and passing it along) so that the host certificate
	// is signed by the correct certificate authority.
	certificateCache, err := NewHostCertificateCache(srv.Config.KeyGen, client)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &localSite{
		srv:              srv,
		client:           client,
		accessPoint:      accessPoint,
		certificateCache: certificateCache,
		domainName:       domainName,
		remoteConns:      make(map[string]*remoteConn),
		clock:            srv.Clock,
		log: log.WithFields(log.Fields{
			trace.Component: teleport.ComponentReverseTunnelServer,
			trace.ComponentFields: map[string]string{
				"cluster": domainName,
			},
		}),
		offlineThreshold: srv.offlineThreshold,
	}, nil
}

// localSite allows to directly access the remote servers
// not using any tunnel, and using standard SSH
//
// it implements RemoteSite interface
type localSite struct {
	sync.Mutex

	log        *log.Entry
	domainName string
	srv        *server

	// client provides access to the Auth Server API of the local cluster.
	client auth.ClientI
	// accessPoint provides access to a cached subset of the Auth Server API of
	// the local cluster.
	accessPoint auth.AccessPoint

	// certificateCache caches host certificates for the forwarding server.
	certificateCache *certificateCache

	// remoteConns maps UUID to a remote connection.
	remoteConns map[string]*remoteConn

	// clock is used to control time in tests.
	clock clockwork.Clock

	// offlineThreshold is how long to wait for a keep alive message before
	// marking a reverse tunnel connection as invalid.
	offlineThreshold time.Duration
}

// GetTunnelsCount always the number of tunnel connections to this cluster.
func (s *localSite) GetTunnelsCount() int {
	s.Lock()
	defer s.Unlock()

	return len(s.remoteConns)
}

// CachingAccessPoint returns a auth.AccessPoint for this cluster.
func (s *localSite) CachingAccessPoint() (auth.AccessPoint, error) {
	return s.accessPoint, nil
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

func (s *localSite) DialAuthServer() (conn net.Conn, err error) {
	// get list of local auth servers
	authServers, err := s.client.GetAuthServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(authServers) < 1 {
		return nil, trace.ConnectionProblem(nil, "no auth servers available")
	}

	// try and dial to one of them, as soon as we are successful, return the net.Conn
	for _, authServer := range authServers {
		conn, err = net.DialTimeout("tcp", authServer.GetAddr(), defaults.DefaultDialTimeout)
		if err == nil {
			return conn, nil
		}
	}

	// return the last error
	return nil, trace.ConnectionProblem(err, "unable to connect to auth server")
}

func (s *localSite) Dial(params DialParams) (net.Conn, error) {
	// If the proxy is in recording mode use the agent to dial and build a
	// in-memory forwarding server.
	clusterConfig, err := s.accessPoint.GetClusterConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if clusterConfig.GetSessionRecording() == services.RecordAtProxy {
		return s.dialWithAgent(params)
	}

	// Attempt to perform a direct TCP dial.
	return s.DialTCP(params)
}

func (s *localSite) DialTCP(params DialParams) (net.Conn, error) {
	s.log.Debugf("Dialing %v.", params)

	conn, _, err := s.getConn(params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
}

// IsClosed always returns false because localSite is never closed.
func (s *localSite) IsClosed() bool { return false }

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
	hostCertificate, err := s.certificateCache.GetHostCertificate(params.Address, params.Principals)
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
func (s *localSite) dialTunnel(params DialParams) (net.Conn, error) {
	rconn, err := s.getRemoteConn(params.ServerID)
	if err != nil {
		return nil, trace.NotFound("no tunnel connection found: %v", err)
	}

	s.log.Debugf("Tunnel dialing to %v.", params.ServerID)

	conn, err := s.chanTransportConn(rconn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
}

func (s *localSite) getConn(params DialParams) (conn net.Conn, useTunnel bool, err error) {
	// If server ID matches a node that has self registered itself over the tunnel,
	// return a connection to that node. Otherwise net.Dial to the target host.
	conn, err = s.dialTunnel(params)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, false, trace.Wrap(err)
		}

		// This node can only be reached over a tunnel, don't attempt to dial
		// remotely.
		if params.To.String() == "" {
			return nil, false, trace.ConnectionProblem(err, "node is offline, please try again later")
		}
		// If no tunnel connection was found, dial to the target host.
		dialer := proxy.DialerFromEnvironment(params.To.String())
		conn, err = dialer.DialTimeout(params.To.Network(), params.To.String(), defaults.DefaultDialTimeout)
		if err != nil {
			return nil, false, trace.Wrap(err)
		}

		// Return a direct dialed connection.
		return conn, false, nil
	}

	// Return a tunnel dialed connection.
	return conn, true, nil
}

func (s *localSite) addConn(nodeID string, conn net.Conn, sconn ssh.Conn) (*remoteConn, error) {
	s.Lock()
	defer s.Unlock()

	rconn := newRemoteConn(&connConfig{
		conn:             conn,
		sconn:            sconn,
		accessPoint:      s.accessPoint,
		tunnelType:       string(services.NodeTunnel),
		proxyName:        s.srv.ID,
		clusterName:      s.domainName,
		nodeID:           nodeID,
		offlineThreshold: s.offlineThreshold,
	})
	s.remoteConns[nodeID] = rconn

	return rconn, nil
}

// fanOutProxies is a non-blocking call that puts the new proxies
// list so that remote connection can notify the remote agent
// about the list update
func (s *localSite) fanOutProxies(proxies []services.Server) {
	s.Lock()
	defer s.Unlock()
	for _, conn := range s.remoteConns {
		conn.updateProxies(proxies)
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
				s.log.WithFields(log.Fields{"latency": roundtrip}).Debugf("Ping <- %v.", rconn.conn.RemoteAddr())
			} else {
				log.Debugf("Ping <- %v.", rconn.conn.RemoteAddr())
			}
			tm := time.Now().UTC()
			rconn.setLastHeartbeat(tm)
		// Note that time.After is re-created everytime a request is processed.
		case <-time.After(s.offlineThreshold):
			rconn.markInvalid(trace.ConnectionProblem(nil, "no heartbeats for %v", s.offlineThreshold))
		}
	}
}

func (s *localSite) getRemoteConn(addr string) (*remoteConn, error) {
	s.Lock()
	defer s.Unlock()

	// Loop over all connections and remove and invalid connections from the
	// connection map.
	for key := range s.remoteConns {
		if s.remoteConns[key].isInvalid() {
			delete(s.remoteConns, key)
		}
	}

	rconn, ok := s.remoteConns[addr]
	if !ok {
		return nil, trace.NotFound("no reverse tunnel for %v found", addr)
	}
	if !rconn.isReady() {
		return nil, trace.NotFound("%v is offline: no active tunnels found", addr)
	}

	return rconn, nil
}

func (s *localSite) chanTransportConn(rconn *remoteConn) (net.Conn, error) {
	s.log.Debugf("Connecting to %v through tunnel.", rconn.conn.RemoteAddr())

	conn, markInvalid, err := connectProxyTransport(rconn.sconn, &dialReq{
		Address: LocalNode,
	})
	if err != nil {
		if markInvalid {
			rconn.markInvalid(err)
		}
		return nil, trace.Wrap(err)
	}

	return conn, nil
}
