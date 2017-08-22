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
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"

	"github.com/mailgun/oxy/forward"
	"golang.org/x/crypto/ssh"
)

// remoteSite is a remote site who established the inbound connecton to
// the local reverse tunnel server, and now it can provide access to the
// cluster behind it.
type remoteSite struct {
	sync.Mutex

	log         *log.Entry
	domainName  string
	connections []*remoteConn
	lastUsed    int
	lastActive  time.Time
	srv         *server

	transport   *http.Transport
	clt         *auth.Client
	accessPoint auth.AccessPoint
}

func (s *remoteSite) CachingAccessPoint() (auth.AccessPoint, error) {
	return s.accessPoint, nil
}

func (s *remoteSite) GetClient() (auth.ClientI, error) {
	return s.clt, nil
}

func (s *remoteSite) String() string {
	return fmt.Sprintf("remoteSite(%v)", s.domainName)
}

func (s *remoteSite) connectionCount() int {
	s.Lock()
	defer s.Unlock()
	return len(s.connections)
}

func (s *remoteSite) nextConn() (*remoteConn, error) {
	s.Lock()
	defer s.Unlock()

	for {
		if len(s.connections) == 0 {
			return nil, trace.NotFound("no active tunnels to cluster %v", s.GetName())
		}
		s.lastUsed = (s.lastUsed + 1) % len(s.connections)
		remoteConn := s.connections[s.lastUsed]
		if !remoteConn.isInvalid() {
			return remoteConn, nil
		}
		s.connections = append(s.connections[:s.lastUsed], s.connections[s.lastUsed+1:]...)
		s.lastUsed = 0
		go remoteConn.Close()
	}
}

// addConn helper adds a new active remote cluster connection to the list
// of such connections
func (s *remoteSite) addConn(conn net.Conn, sshConn ssh.Conn) (*remoteConn, error) {
	rc := &remoteConn{
		sshConn: sshConn,
		conn:    conn,
		log:     s.log,
	}

	s.Lock()
	defer s.Unlock()

	s.connections = append(s.connections, rc)
	s.lastUsed = 0
	return rc, nil
}

func (s *remoteSite) GetStatus() string {
	s.Lock()
	defer s.Unlock()
	diff := time.Now().Sub(s.lastActive)
	if diff > 2*defaults.ReverseTunnelAgentHeartbeatPeriod {
		return RemoteSiteStatusOffline
	}
	return RemoteSiteStatusOnline
}

func (s *remoteSite) setLastActive(t time.Time) {
	s.Lock()
	defer s.Unlock()
	s.lastActive = t
}

func (s *remoteSite) handleHeartbeat(conn *remoteConn, ch ssh.Channel, reqC <-chan *ssh.Request) {
	defer func() {
		s.log.Infof("[TUNNEL] site connection closed: %v", s.domainName)
		conn.Close()
	}()
	for {
		select {
		case req := <-reqC:
			if req == nil {
				s.log.Infof("[TUNNEL] site disconnected: %v", s.domainName)
				conn.markInvalid(trace.ConnectionProblem(nil, "agent disconnected"))
				return
			}
			log.Debugf("[TUNNEL] ping from \"%s\" %s", s.domainName, conn.conn.RemoteAddr())
			s.setLastActive(time.Now())
		case <-time.After(3 * defaults.ReverseTunnelAgentHeartbeatPeriod):
			conn.markInvalid(trace.ConnectionProblem(nil, "agent missed 3 heartbeats"))
		}
	}
}

func (s *remoteSite) GetName() string {
	return s.domainName
}

func (s *remoteSite) GetLastConnected() time.Time {
	s.Lock()
	defer s.Unlock()
	return s.lastActive
}

// dialAccessPoint establishes a connection from the proxy (reverse tunnel server)
// back into the client using previously established tunnel.
func (s *remoteSite) dialAccessPoint(network, addr string) (net.Conn, error) {
	s.log.Infof("[TUNNEL] dial to site '%s'", s.GetName())

	try := func() (net.Conn, error) {
		remoteConn, err := s.nextConn()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		ch, _, err := remoteConn.sshConn.OpenChannel(chanAccessPoint, nil)
		if err != nil {
			remoteConn.markInvalid(err)
			s.log.Errorf("[TUNNEL] disconnecting site '%s' on %v. Err: %v",
				s.GetName(),
				remoteConn.conn.RemoteAddr(),
				err)
			return nil, trace.Wrap(err)
		}
		s.log.Infof("[TUNNEL] success dialing to site '%s'", s.GetName())
		return utils.NewChConn(remoteConn.sshConn, ch), nil
	}

	for {
		conn, err := try()
		if err != nil {
			if trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			continue
		}
		return conn, nil
	}
}

// Dial is used to connect a requesting client (say, tsh) to an SSH server
// located in a remote connected site, the connection goes through the
// reverse proxy tunnel.
func (s *remoteSite) Dial(from, to net.Addr) (conn net.Conn, err error) {
	s.log.Infof("[TUNNEL] dialing %v@%v through the tunnel", to, s.domainName)
	stop := false

	_, addr := to.Network(), to.String()

	try := func() (net.Conn, error) {
		remoteConn, err := s.nextConn()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var ch ssh.Channel
		ch, _, err = remoteConn.sshConn.OpenChannel(chanTransport, nil)
		if err != nil {
			remoteConn.markInvalid(err)
			return nil, trace.Wrap(err)
		}
		stop = true
		// send a special SSH out-of-band request called "teleport-transport"
		// the agent on the other side will create a new TCP/IP connection to
		// 'addr' on its network and will start proxying that connection over
		// this SSH channel:
		var dialed bool
		dialed, err = ch.SendRequest(chanTransportDialReq, true, []byte(addr))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !dialed {
			defer ch.Close()
			// pull the error message from the tunnel client (remote cluster)
			// passed to us via stderr:
			errMessage, _ := ioutil.ReadAll(ch.Stderr())
			if errMessage == nil {
				errMessage = []byte("failed connecting to " + addr)
			}
			return nil, trace.Errorf(strings.TrimSpace(string(errMessage)))
		}
		return utils.NewChConn(remoteConn.sshConn, ch), nil
	}
	// loop through existing TCP/IP connections (reverse tunnels) and try
	// to establish an inbound connection-over-ssh-channel to the remote
	// cluster (AKA "remotetunnel agent"):
	for i := 0; i < s.connectionCount() && !stop; i++ {
		conn, err = try()
		if err == nil {
			return conn, nil
		}
		s.log.Errorf("[TUNNEL] Dial(addr=%v) failed: %v", addr, err)
	}
	// didn't connect and no error? this means we didn't have any connected
	// tunnels to try
	if err == nil {
		err = trace.Errorf("%v is offline", s.GetName())
	}
	return nil, err
}

func (s *remoteSite) handleAuthProxy(w http.ResponseWriter, r *http.Request) {
	s.log.Infof("[TUNNEL] handleAuthProxy()")

	fwd, err := forward.New(forward.RoundTripper(s.transport), forward.Logger(s.log))
	if err != nil {
		roundtrip.ReplyJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	r.URL.Scheme = "http"
	r.URL.Host = "stub"
	fwd.ServeHTTP(w, r)
}
