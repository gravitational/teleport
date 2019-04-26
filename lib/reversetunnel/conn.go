/*
Copyright 2019 Gravitational, Inc.

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
	"io"
	"io/ioutil"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/proxy"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

// remoteConn holds a connection to a remote host, either node or proxy.
type remoteConn struct {
	*connConfig
	mu  sync.Mutex
	log *logrus.Entry

	// discoveryCh is the channel over which discovery requests are sent.
	discoveryCh ssh.Channel

	// invalid indicates the connection is invalid and connections can no longer
	// be made on it.
	invalid int32

	// lastError is the last error that occured before this connection became
	// invalid.
	lastError error

	// Used to make sure calling Close on the connection multiple times is safe.
	closed int32

	// closeContext and closeCancel are used to signal to any waiting goroutines
	// that the remoteConn is now closed and to release any resources.
	closeContext context.Context
	closeCancel  context.CancelFunc

	// clock is used to control time in tests.
	clock clockwork.Clock

	// lastHeartbeat is the last time a heartbeat was received.
	lastHeartbeat int64
}

// connConfig is the configuration for the remoteConn.
type connConfig struct {
	// conn is the underlying net.Conn.
	conn net.Conn

	// sconn is the underlying SSH connection.
	sconn ssh.Conn

	// accessPoint provides access to the Auth Server API.
	accessPoint auth.AccessPoint

	// tunnelID is the tunnel ID. This is either the cluster name (trusted
	// clusters) or hostUUID.clusterName (nodes dialing back).
	tunnelID string

	// tunnelType is the type of tunnel connection, either proxy or node.
	tunnelType string

	// proxyName is the name of the proxy this remoteConn is located in.
	proxyName string

	// clusterName is the name of the cluster this tunnel is associated with.
	clusterName string
}

func newRemoteConn(cfg *connConfig) *remoteConn {
	c := &remoteConn{
		log: logrus.WithFields(logrus.Fields{
			trace.Component: "discovery",
		}),
		connConfig: cfg,
		clock:      clockwork.NewRealClock(),
	}

	c.closeContext, c.closeCancel = context.WithCancel(context.Background())

	// Continue sending periodic discovery requests over this connection so
	// that all local proxies can be discovered.
	go c.periodicSendDiscoveryRequests()

	return c
}

func (c *remoteConn) String() string {
	return fmt.Sprintf("remoteConn(remoteAddr=%v)", c.conn.RemoteAddr())
}

func (c *remoteConn) Close() error {
	defer c.closeCancel()

	// If the connection has already been closed, return right away.
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil
	}

	// Close the discovery channel.
	if c.discoveryCh != nil {
		c.discoveryCh.Close()
		c.discoveryCh = nil
	}

	// Close the SSH connection which will close the underlying net.Conn as well.
	err := c.sconn.Close()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil

}

// OpenChannel will open a SSH channel to the remote side.
func (c *remoteConn) OpenChannel(name string, data []byte) (ssh.Channel, error) {
	channel, _, err := c.sconn.OpenChannel(name, data)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return channel, nil
}

// ChannelConn creates a net.Conn over a SSH channel.
func (c *remoteConn) ChannelConn(channel ssh.Channel) net.Conn {
	return utils.NewChConn(c.sconn, channel)
}

func (c *remoteConn) markInvalid(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	atomic.StoreInt32(&c.invalid, 1)
	c.lastError = err
	c.log.Errorf("Disconnecting connection to %v: %v.", c.conn.RemoteAddr(), err)
}

func (c *remoteConn) isInvalid() bool {
	return atomic.LoadInt32(&c.invalid) == 1
}

func (c *remoteConn) setLastHeartbeat(tm time.Time) {
	atomic.StoreInt64(&c.lastHeartbeat, tm.UnixNano())
}

// isReady returns true when connection is ready to be tried,
// it returns true when connection has received the first heartbeat
func (c *remoteConn) isReady() bool {
	return atomic.LoadInt64(&c.lastHeartbeat) != 0
}

func (c *remoteConn) openDiscoveryChannel() (ssh.Channel, error) {
	var err error

	if c.isInvalid() {
		return nil, trace.Wrap(c.lastError)
	}

	// If a discovery channel has already been opened, return it right away.
	if c.discoveryCh != nil {
		return c.discoveryCh, nil
	}

	c.discoveryCh, _, err = c.sconn.OpenChannel(chanDiscovery, nil)
	if err != nil {
		c.markInvalid(err)
		return nil, trace.Wrap(err)
	}
	return c.discoveryCh, nil
}

func (c *remoteConn) periodicSendDiscoveryRequests() {
	ticker := time.NewTicker(defaults.ReverseTunnelAgentHeartbeatPeriod)
	defer ticker.Stop()

	if err := c.findAndSend(); err != nil {
		c.log.Warnf("Failed to send discovery request: %v.", err)
	}

	for {
		select {
		case <-ticker.C:
			err := c.findAndSend()
			if err != nil {
				c.log.Warnf("Failed to send discovery request: %v.", err)
			}
		case <-c.closeContext.Done():
			return
		}
	}
}

// sendDiscovery requests sends special "Discovery Requests" back to the
// connected agent. Discovery request consists of the proxies that are part
// of the cluster, but did not receive the connection from the agent. Agent
// will act on a discovery request attempting to establish connection to the
// proxies that were not discovered.
func (c *remoteConn) findAndSend() error {
	// Find all proxies that don't have a connection to a remote agent. If all
	// proxies have connections, return right away.
	disconnectedProxies, err := c.findDisconnectedProxies()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(disconnectedProxies) == 0 {
		return nil
	}

	c.log.Debugf("Proxy %v sending %v discovery request with tunnel ID: %v and disconnected proxies: %v.",
		c.proxyName, string(c.tunnelType), c.tunnelID, Proxies(disconnectedProxies))

	req := discoveryRequest{
		TunnelID: c.tunnelID,
		Type:     string(c.tunnelType),
		Proxies:  disconnectedProxies,
	}

	err = c.sendDiscoveryRequests(req)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// findDisconnectedProxies finds proxies that do not have inbound reverse tunnel
// connections.
func (c *remoteConn) findDisconnectedProxies() ([]services.Server, error) {
	// Find all proxies that have connection from the remote domain.
	conns, err := c.accessPoint.GetTunnelConnections(c.clusterName, services.SkipValidation())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connected := make(map[string]bool)
	for _, conn := range conns {
		if c.isOnline(conn) {
			connected[conn.GetProxyName()] = true
		}
	}

	// Build a list of local proxies that do not have a remote connection to them.
	var missing []services.Server
	proxies, err := c.accessPoint.GetProxies()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for i := range proxies {
		proxy := proxies[i]

		// A proxy should never add itself to the list of missing proxies.
		if proxy.GetName() == c.proxyName {
			continue
		}

		if !connected[proxy.GetName()] {
			missing = append(missing, proxy)
		}
	}

	return missing, nil
}

// sendDiscoveryRequests sends a discovery request with missing proxies.
func (c *remoteConn) sendDiscoveryRequests(req discoveryRequest) error {
	discoveryCh, err := c.openDiscoveryChannel()
	if err != nil {
		return trace.Wrap(err)
	}

	// Marshal and send the request. If the connection failed, mark the
	// connection as invalid so it will be removed later.
	payload, err := marshalDiscoveryRequest(req)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = discoveryCh.SendRequest(chanDiscoveryReq, false, payload)
	if err != nil {
		c.markInvalid(err)
		return trace.Wrap(err)
	}

	return nil
}

func (c *remoteConn) isOnline(conn services.TunnelConnection) bool {
	return services.TunnelConnectionStatus(c.clock, conn) == teleport.RemoteClusterStatusOnline
}

type transportParams struct {
	component    string
	log          *logrus.Entry
	closeContext context.Context
	authClient   auth.ClientI
	channel      ssh.Channel
	requestCh    <-chan *ssh.Request

	// kubeDialAddr is the address of the Kubernetes proxy.
	kubeDialAddr utils.NetAddr

	// sconn is a SSH connection to the remote host. Used for dial back nodes.
	sconn ssh.Conn
	// server is the underlying SSH server. Used for dial back nodes.
	server ServerHandler
}

// TunnelAuthDialer connects to the Auth Server through the reverse tunnel.
func TunnelAuthDialer(proxyAddr string, sshConfig *ssh.ClientConfig) auth.DialContext {
	return func(ctx context.Context, network string, addr string) (net.Conn, error) {
		// Connect to the reverse tunnel server.
		dialer := proxy.DialerFromEnvironment(proxyAddr)
		sconn, err := dialer.Dial("tcp", proxyAddr, sshConfig)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		conn, err := connectProxyTransport(sconn.Conn, RemoteAuthServer)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return conn, nil
	}
}

// connectProxyTransport opens a channel over the remote tunnel and connects
// to the requested host.
func connectProxyTransport(sconn ssh.Conn, addr string) (net.Conn, error) {
	channel, _, err := sconn.OpenChannel(chanTransport, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Send a special SSH out-of-band request called "teleport-transport"
	// the agent on the other side will create a new TCP/IP connection to
	// 'addr' on its network and will start proxying that connection over
	// this SSH channel.
	ok, err := channel.SendRequest(chanTransportDialReq, true, []byte(addr))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !ok {
		defer channel.Close()

		// Pull the error message from the tunnel client (remote cluster)
		// passed to us via stderr.
		errMessage, _ := ioutil.ReadAll(channel.Stderr())
		if errMessage == nil {
			errMessage = []byte("failed connecting to " + addr)
		}
		return nil, trace.Errorf(strings.TrimSpace(string(errMessage)))
	}

	return utils.NewChConn(sconn, channel), nil
}

// proxyTransport runs either in the agent or reverse tunnel itself. It's
// used to establish connections from remote clusters into the main cluster
// or for remote nodes that have no direct network access to the cluster.
func proxyTransport(p *transportParams) {
	defer p.channel.Close()

	// Always push space into stderr to make sure the caller can always
	// safely call read (stderr) without blocking. This stderr is only used
	// to request proxying of TCP/IP via reverse tunnel.
	fmt.Fprint(p.channel.Stderr(), " ")

	// Wait for a request to come in from the other side telling the server
	// where to dial to.
	var req *ssh.Request
	select {
	case <-p.closeContext.Done():
		return
	case req = <-p.requestCh:
		if req == nil {
			return
		}
	case <-time.After(defaults.DefaultDialTimeout):
		p.log.Warnf("Transport request failed: timed out waiting for request.")
		return
	}

	server := string(req.Payload)
	var servers []string

	// Handle special non-resolvable addresses first.
	switch server {
	// Connect to an Auth Server.
	case RemoteAuthServer:
		authServers, err := p.authClient.GetAuthServers()
		if err != nil {
			p.log.Errorf("Transport request failed: unable to get list of Auth Servers: %v.", err)
			req.Reply(false, []byte("connection rejected: failed to connect to auth server"))
			return
		}
		if len(authServers) == 0 {
			p.log.Errorf("Transport request failed: no auth servers found.")
			req.Reply(false, []byte("connection rejected: failed to connect to auth server"))
			return
		}
		for _, as := range authServers {
			servers = append(servers, as.GetAddr())
		}
	// Connect to the Kubernetes proxy.
	case RemoteKubeProxy:
		if p.component == teleport.ComponentReverseTunnelServer {
			req.Reply(false, []byte("connection rejected: no remote kubernetes proxy"))
			return
		}

		// If Kubernetes is not configured, reject the connection.
		if p.kubeDialAddr.IsEmpty() {
			req.Reply(false, []byte("connection rejected: configure kubernetes proxy for this cluster."))
			return
		}
		servers = append(servers, p.kubeDialAddr.Addr)
	// LocalNode requests are for the single server running in the agent pool.
	case LocalNode:
		if p.component == teleport.ComponentReverseTunnelServer {
			req.Reply(false, []byte("connection rejected: no local node"))
			return
		}
		if p.server == nil {
			req.Reply(false, []byte("connection rejected: server missing"))
			return
		}
		if p.sconn == nil {
			req.Reply(false, []byte("connection rejected: server connection missing"))
			return
		}

		req.Reply(true, []byte("Connected."))

		// Hand connection off to the SSH server.
		p.server.HandleConnection(utils.NewChConn(p.sconn, p.channel))
		return
	default:
		servers = append(servers, server)
	}

	p.log.Debugf("Received out-of-band proxy transport request: %v", servers)

	// Loop over all servers and try and connect to one of them.
	var err error
	var conn net.Conn
	for _, s := range servers {
		conn, err = net.Dial("tcp", s)
		if err == nil {
			break
		}

		// Log the reason the connection failed.
		p.log.Debugf(trace.DebugReport(err))
	}

	// If all net.Dial attempts failed, write the last connection error to stderr
	// of the caller (via SSH channel) so the error will be propagated all the
	// way back to the client (tsh or ssh).
	if err != nil {
		fmt.Fprint(p.channel.Stderr(), err.Error())
		req.Reply(false, []byte(err.Error()))
		return
	}

	// Dail was successful.
	req.Reply(true, []byte("Connected."))
	p.log.Debugf("Successfully dialed to %v, start proxying.", server)

	errorCh := make(chan error, 2)

	go func() {
		// Make sure that we close the client connection on a channel
		// close, otherwise the other goroutine would never know
		// as it will block on read from the connection.
		defer conn.Close()
		_, err := io.Copy(conn, p.channel)
		errorCh <- err
	}()

	go func() {
		_, err := io.Copy(p.channel, conn)
		errorCh <- err
	}()

	for i := 0; i < 2; i++ {
		select {
		case err := <-errorCh:
			if err != nil && err != io.EOF {
				p.log.Warnf("Proxy transport failed: %v %T.", trace.DebugReport(err), err)
			}
		case <-p.closeContext.Done():
			p.log.Warnf("Proxy transport failed: closing context.")
			return
		}
	}
}
