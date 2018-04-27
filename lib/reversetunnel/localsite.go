/*
Copyright 2016 Gravitational, Inc.

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

	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/forward"

	"github.com/gravitational/trace"
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
		log: log.WithFields(log.Fields{
			trace.Component: teleport.ComponentReverseTunnelServer,
			trace.ComponentFields: map[string]string{
				"cluster": domainName,
			},
		}),
	}, nil
}

// localSite allows to directly access the remote servers
// not using any tunnel, and using standard SSH
//
// it implements RemoteSite interface
type localSite struct {
	sync.Mutex

	authServer  string
	log         *log.Entry
	domainName  string
	connections []*remoteConn
	lastUsed    int
	lastActive  time.Time
	srv         *server

	// client provides access to the Auth Server API of the local cluster.
	client auth.ClientI
	// accessPoint provides access to a cached subset of the Auth Server API of
	// the local cluster.
	accessPoint auth.AccessPoint

	// certificateCache caches host certificates for the forwarding server.
	certificateCache *certificateCache
}

func (s *localSite) CachingAccessPoint() (auth.AccessPoint, error) {
	return s.accessPoint, nil
}

func (s *localSite) GetClient() (auth.ClientI, error) {
	return s.client, nil
}

func (s *localSite) String() string {
	return fmt.Sprintf("local(%v)", s.domainName)
}

func (s *localSite) GetStatus() string {
	return teleport.RemoteClusterStatusOnline
}

func (s *localSite) GetName() string {
	return s.domainName
}

func (s *localSite) GetLastConnected() time.Time {
	return time.Now()
}

func (s *localSite) DialAuthServer() (conn net.Conn, err error) {
	// get list of local auth servers
	authServers, err := s.client.GetAuthServers()
	if err != nil {
		return nil, trace.Wrap(err)
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

func (s *localSite) Dial(from net.Addr, to net.Addr, userAgent agent.Agent) (net.Conn, error) {
	// Fetch the cluster configuration at the start of a dial. This will both be
	// used to check what mode the cluster is in, and if sessions are recorded
	// at the proxy, it will be passed to the in-memory server so a cached copy
	// can be used for the duration of that connection.
	clusterConfig, err := s.accessPoint.GetClusterConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If the proxy is in recording mode use the agent to dial and build a
	// in-memory forwarding server.
	if clusterConfig.GetSessionRecording() == services.RecordAtProxy {
		if userAgent == nil {
			return nil, trace.BadParameter("user agent missing")
		}
		return s.dialWithAgent(from, to, userAgent, clusterConfig)
	}

	return s.dial(from, to)
}

func (s *localSite) dial(from net.Addr, to net.Addr) (net.Conn, error) {
	s.log.Debugf("Dialing from %v to %v", from, to)

	return net.DialTimeout(to.Network(), to.String(), defaults.DefaultDialTimeout)
}

func (s *localSite) dialWithAgent(from net.Addr, to net.Addr, userAgent agent.Agent, clusterConfig services.ClusterConfig) (net.Conn, error) {
	s.log.Debugf("Dialing with an agent from %v to %v", from, to)

	// Get a host certificate for the forwarding node from the cache.
	hostCertificate, err := s.certificateCache.GetHostCertificate(to.String())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get a net.Conn to the target server.
	targetConn, err := net.DialTimeout(to.Network(), to.String(), defaults.DefaultDialTimeout)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a forwarding server that serves a single ssh connection on it. we
	// don't need to close this server it will close and release all resources
	// once conn is closed.
	serverConfig := forward.ServerConfig{
		AuthClient:      s.client,
		UserAgent:       userAgent,
		TargetConn:      targetConn,
		SrcAddr:         from,
		DstAddr:         to,
		HostCertificate: hostCertificate,
		Ciphers:         s.srv.Config.Ciphers,
		KEXAlgorithms:   s.srv.Config.KEXAlgorithms,
		MACAlgorithms:   s.srv.Config.MACAlgorithms,
		DataDir:         s.srv.Config.DataDir,
		ClusterConfig:   clusterConfig,
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

func findServer(addr string, servers []services.Server) (services.Server, error) {
	for i := range servers {
		srv := servers[i]
		_, port, err := net.SplitHostPort(srv.GetAddr())
		if err != nil {
			log.Warningf("server %v(%v) has incorrect address format (%v)",
				srv.GetAddr(), srv.GetHostname(), err.Error())
		} else {
			if (len(srv.GetHostname()) != 0) && (len(port) != 0) && (addr == srv.GetHostname()+":"+port || addr == srv.GetAddr()) {
				return srv, nil
			}
		}
	}
	return nil, trace.NotFound("server %v is unknown", addr)
}
