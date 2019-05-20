/*
Copyright 2015-2019 Gravitational, Inc.

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

// Package reversetunnel sets up persistent reverse tunnel
// between remote site and teleport proxy, when site agents
// dial to teleport proxy's socket and teleport proxy can connect
// to any server through this tunnel.
package reversetunnel

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/proxy"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

const (
	// agentStateConnecting is when agent is connecting to the target
	// without particular purpose
	agentStateConnecting = "connecting"
	// agentStateDiscovering is when agent is created with a goal
	// to discover one or many proxies
	agentStateDiscovering = "discovering"
	// agentStateConnected means that agent has connected to instance
	agentStateConnected = "connected"
	// agentStateDiscovered means that agent has discovered the right proxy
	agentStateDiscovered = "discovered"
	// agentStateDisconnected means that the agent has disconnected from the
	// proxy and this agent and be removed from the pool.
	agentStateDisconnected = "disconnected"
)

// AgentConfig holds configuration for agent
type AgentConfig struct {
	// Addr is target address to dial
	Addr utils.NetAddr
	// ClusterName is the name of the cluster the tunnel is connected to. When the
	// agent is running in a proxy, it's the name of the remote cluster, when the
	// agent is running in a node, it's the name of the local cluster.
	ClusterName string
	// Signers contains authentication signers
	Signers []ssh.Signer
	// Client is a client to the local auth servers
	Client auth.ClientI
	// AccessPoint is a caching access point to the local auth servers
	AccessPoint auth.AccessPoint
	// Context is a parent context
	Context context.Context
	// DiscoveryC is a channel that receives discovery requests
	// from reverse tunnel server
	DiscoveryC chan *discoveryRequest
	// Username is the name of this client used to authenticate on SSH
	Username string
	// DiscoverProxies is set when the agent is created in discovery mode
	// and is set to connect to one of the target proxies from the list
	DiscoverProxies []services.Server
	// Clock is a clock passed in tests, if not set wall clock
	// will be used
	Clock clockwork.Clock
	// EventsC is an optional events channel, used for testing purposes
	EventsC chan string
	// KubeDialAddr is a dial address for kubernetes proxy
	KubeDialAddr utils.NetAddr
	// Server is a SSH server that can handle a connection (perform a handshake
	// then process). Only set with the agent is running within a node.
	Server ServerHandler
	// ReverseTunnelServer holds all reverse tunnel connections.
	ReverseTunnelServer Server
	// LocalClusterName is the name of the cluster this agent is running in.
	LocalClusterName string
}

// CheckAndSetDefaults checks parameters and sets default values
func (a *AgentConfig) CheckAndSetDefaults() error {
	if a.Addr.IsEmpty() {
		return trace.BadParameter("missing parameter Addr")
	}
	if a.Context == nil {
		return trace.BadParameter("missing parameter Context")
	}
	if a.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	if a.AccessPoint == nil {
		return trace.BadParameter("missing parameter AccessPoint")
	}
	if len(a.Signers) == 0 {
		return trace.BadParameter("missing parameter Signers")
	}
	if len(a.Username) == 0 {
		return trace.BadParameter("missing parameter Username")
	}
	if a.Clock == nil {
		a.Clock = clockwork.NewRealClock()
	}
	return nil
}

// Agent is a reverse tunnel agent running as a part of teleport Proxies
// to establish outbound reverse tunnels to remote proxies.
//
// There are two operation modes for agents:
// * Standard agent attempts to establish connection
// to any available proxy. Standard agent transitions between
// "connecting" -> "connected states.
// * Discovering agent attempts to establish connection to a subset
// of remote proxies (specified in the config via DiscoverProxies parameter.)
// Discovering agent transitions between "discovering" -> "discovered" states.
type Agent struct {
	sync.RWMutex
	*log.Entry
	AgentConfig
	ctx             context.Context
	cancel          context.CancelFunc
	hostKeyCallback ssh.HostKeyCallback
	authMethods     []ssh.AuthMethod
	// state is the state of this agent
	state string
	// stateChange records last time the state was changed
	stateChange time.Time
	// principals is the list of principals of the server this agent
	// is currently connected to
	principals []string
}

// NewAgent returns a new reverse tunnel agent
func NewAgent(cfg AgentConfig) (*Agent, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(cfg.Context)
	a := &Agent{
		AgentConfig: cfg,
		ctx:         ctx,
		cancel:      cancel,
		authMethods: []ssh.AuthMethod{ssh.PublicKeys(cfg.Signers...)},
	}
	if len(cfg.DiscoverProxies) == 0 {
		a.state = agentStateConnecting
	} else {
		a.state = agentStateDiscovering
	}
	a.Entry = log.WithFields(log.Fields{
		trace.Component: teleport.ComponentReverseTunnelAgent,
		trace.ComponentFields: log.Fields{
			"target": cfg.Addr.String(),
		},
	})
	a.hostKeyCallback = a.checkHostSignature
	return a, nil
}

func (a *Agent) String() string {
	if len(a.DiscoverProxies) == 0 {
		return fmt.Sprintf("agent(%v) -> %v:%v", a.getState(), a.ClusterName, a.Addr.String())
	}
	return fmt.Sprintf("agent(%v) -> %v:%v, discover %v", a.getState(), a.ClusterName, a.Addr.String(), Proxies(a.DiscoverProxies))
}

func (a *Agent) getLastStateChange() time.Time {
	a.RLock()
	defer a.RUnlock()
	return a.stateChange
}

func (a *Agent) setStateAndPrincipals(state string, principals []string) {
	a.Lock()
	defer a.Unlock()
	prev := a.state
	a.Debugf("Changing state %v -> %v.", prev, state)
	a.state = state
	a.stateChange = a.Clock.Now().UTC()
	a.principals = principals
}
func (a *Agent) setState(state string) {
	a.Lock()
	defer a.Unlock()
	prev := a.state
	a.Debugf("Changing state %v -> %v.", prev, state)
	a.state = state
	a.stateChange = a.Clock.Now().UTC()
}

func (a *Agent) getState() string {
	a.RLock()
	defer a.RUnlock()
	return a.state
}

// Close signals to close all connections and operations
func (a *Agent) Close() error {
	a.cancel()
	return nil
}

// Start starts agent that attempts to connect to remote server
func (a *Agent) Start() {
	go a.run()
}

// Wait waits until all outstanding operations are completed
func (a *Agent) Wait() error {
	return nil
}

func (a *Agent) isDiscovering(proxy services.Server) bool {
	for _, discoverProxy := range a.DiscoverProxies {
		if a.getState() != agentStateDiscovering && a.getState() != agentStateConnecting {
			continue
		}

		proxyID := fmt.Sprintf("%v.%v", proxy.GetName(), a.ClusterName)
		discoverID := fmt.Sprintf("%v.%v", discoverProxy.GetName(), a.ClusterName)
		if proxyID == discoverID {
			return true
		}
	}
	return false
}

// connectedTo returns true if connected services.Server passed in.
func (a *Agent) connectedTo(proxy services.Server) bool {
	principals := a.getPrincipals()
	proxyID := fmt.Sprintf("%v.%v", proxy.GetName(), a.ClusterName)
	if _, ok := principals[proxyID]; ok {
		return true
	}
	return false
}

// connectedToRightProxy returns true if it connected to a proxy in the
// discover list.
func (a *Agent) connectedToRightProxy() bool {
	for _, proxy := range a.DiscoverProxies {
		if a.connectedTo(proxy) {
			return true
		}
	}
	return false
}

func (a *Agent) setPrincipals(principals []string) {
	a.Lock()
	defer a.Unlock()
	a.principals = principals
}

func (a *Agent) getPrincipalsList() []string {
	a.RLock()
	defer a.RUnlock()
	out := make([]string, len(a.principals))
	copy(out, a.principals)
	return out
}

func (a *Agent) getPrincipals() map[string]struct{} {
	a.RLock()
	defer a.RUnlock()
	out := make(map[string]struct{}, len(a.principals))
	for _, p := range a.principals {
		out[p] = struct{}{}
	}
	return out
}

func (a *Agent) checkHostSignature(hostport string, remote net.Addr, key ssh.PublicKey) error {
	cert, ok := key.(*ssh.Certificate)
	if !ok {
		return trace.BadParameter("expected certificate")
	}
	cas, err := a.AccessPoint.GetCertAuthorities(services.HostCA, false)
	if err != nil {
		return trace.Wrap(err, "failed to fetch remote certs")
	}
	for _, ca := range cas {
		checkers, err := ca.Checkers()
		if err != nil {
			return trace.BadParameter("error parsing key: %v", err)
		}
		for _, checker := range checkers {
			if sshutils.KeysEqual(checker, cert.SignatureKey) {
				a.setPrincipals(cert.ValidPrincipals)
				return nil
			}
		}
	}
	return trace.NotFound(
		"no matching keys found when checking server's host signature")
}

func (a *Agent) connect() (conn *ssh.Client, err error) {
	for _, authMethod := range a.authMethods {
		// if http_proxy is set, dial through the proxy
		dialer := proxy.DialerFromEnvironment(a.Addr.Addr)
		conn, err = dialer.Dial(a.Addr.AddrNetwork, a.Addr.Addr, &ssh.ClientConfig{
			User:            a.Username,
			Auth:            []ssh.AuthMethod{authMethod},
			HostKeyCallback: a.hostKeyCallback,
			Timeout:         defaults.DefaultDialTimeout,
		})
		if conn != nil {
			break
		}
	}
	return conn, err
}

// run is the main agent loop. It tries to establish a connection to the
// remote proxy and then process requests that come over the tunnel.
//
// Once run connects to a proxy it starts processing requests from the proxy
// via SSH channels opened by the remote Proxy.
//
// Agent sends periodic heartbeats back to the Proxy and that is how Proxy
// determines disconnects.
func (a *Agent) run() {
	defer a.setState(agentStateDisconnected)

	if len(a.DiscoverProxies) != 0 {
		a.setStateAndPrincipals(agentStateDiscovering, nil)
	} else {
		a.setStateAndPrincipals(agentStateConnecting, nil)
	}

	// Try and connect to remote cluster.
	conn, err := a.connect()
	if err != nil || conn == nil {
		a.Warningf("Failed to create remote tunnel: %v, conn: %v.", err, conn)
		return
	}

	// Successfully connected to remote cluster.
	a.Infof("Connected to %s", conn.RemoteAddr())
	if len(a.DiscoverProxies) != 0 {
		// If not connected to a proxy in the discover list (which means we
		// connected to a proxy we already have a connection to), try again.
		if !a.connectedToRightProxy() {
			a.Debugf("Missed, connected to %v instead of %v.", a.getPrincipalsList(), Proxies(a.DiscoverProxies))

			conn.Close()
			return
		}
		a.Debugf("Agent discovered proxy: %v.", a.getPrincipalsList())
		a.setState(agentStateDiscovered)
	} else {
		a.Debugf("Agent connected to proxy: %v.", a.getPrincipalsList())
		a.setState(agentStateConnected)
	}

	// Notify waiters that the agent has connected.
	if a.EventsC != nil {
		select {
		case a.EventsC <- ConnectedEvent:
		case <-a.ctx.Done():
			a.Debug("Context is closing.")
			return
		default:
		}
	}

	// A connection has been established start processing requests. Note that
	// this function blocks while the connection is up. It will unblock when
	// the connection is closed either due to intermittent connectivity issues
	// or permanent loss of a proxy.
	err = a.processRequests(conn)
	if err != nil {
		a.Warnf("Unable to continue processesing requests: %v.", err)
		return
	}
}

// ConnectedEvent is used to indicate that reverse tunnel has connected
const ConnectedEvent = "connected"

// processRequests is a blocking function which runs in a loop sending heartbeats
// to the given SSH connection and processes inbound requests from the
// remote proxy
func (a *Agent) processRequests(conn *ssh.Client) error {
	defer conn.Close()

	clusterConfig, err := a.AccessPoint.GetClusterConfig()
	if err != nil {
		return trace.Wrap(err)
	}

	ticker := time.NewTicker(clusterConfig.GetKeepAliveInterval())
	defer ticker.Stop()

	hb, reqC, err := conn.OpenChannel(chanHeartbeat, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	newTransportC := conn.HandleChannelOpen(chanTransport)
	newDiscoveryC := conn.HandleChannelOpen(chanDiscovery)

	// send first ping right away, then start a ping timer:
	hb.SendRequest("ping", false, nil)

	for {
		select {
		// need to exit:
		case <-a.ctx.Done():
			return trace.ConnectionProblem(nil, "heartbeat: agent is stopped")
		// time to ping:
		case <-ticker.C:
			bytes, _ := a.Clock.Now().UTC().MarshalText()
			_, err := hb.SendRequest("ping", false, bytes)
			if err != nil {
				a.Error(err)
				return trace.Wrap(err)
			}
			a.Debugf("Ping -> %v.", conn.RemoteAddr())
		// ssh channel closed:
		case req := <-reqC:
			if req == nil {
				return trace.ConnectionProblem(nil, "heartbeat: connection closed")
			}
		// new transport request:
		case nch := <-newTransportC:
			if nch == nil {
				continue
			}
			a.Debugf("Transport request: %v.", nch.ChannelType())
			ch, req, err := nch.Accept()
			if err != nil {
				a.Warningf("Failed to accept request: %v.", err)
				continue
			}

			go proxyTransport(&transportParams{
				log:                 a.Entry,
				closeContext:        a.ctx,
				authClient:          a.Client,
				kubeDialAddr:        a.KubeDialAddr,
				channel:             ch,
				requestCh:           req,
				sconn:               conn.Conn,
				server:              a.Server,
				component:           teleport.ComponentReverseTunnelAgent,
				reverseTunnelServer: a.ReverseTunnelServer,
				localClusterName:    a.LocalClusterName,
			})
		// new discovery request channel
		case nch := <-newDiscoveryC:
			if nch == nil {
				continue
			}
			a.Debugf("Discovery request channel opened: %v.", nch.ChannelType())
			ch, req, err := nch.Accept()
			if err != nil {
				a.Warningf("Failed to accept discovery channel request: %v.", err)
				continue
			}
			go a.handleDiscovery(ch, req)
		}
	}
}

// handleDisovery receives discovery requests from the reverse tunnel
// server, that informs agent about proxies registered in the remote
// cluster and the reverse tunnels already established
//
// ch   : SSH channel which received "teleport-transport" out-of-band request
// reqC : request payload
func (a *Agent) handleDiscovery(ch ssh.Channel, reqC <-chan *ssh.Request) {
	a.Debugf("handleDiscovery requests channel.")
	defer ch.Close()

	for {
		var req *ssh.Request
		select {
		case <-a.ctx.Done():
			a.Infof("Closed, returning.")
			return
		case req = <-reqC:
			if req == nil {
				a.Infof("Connection closed, returning")
				return
			}
			r, err := unmarshalDiscoveryRequest(req.Payload)
			if err != nil {
				a.Warningf("Bad payload: %v.", err)
				return
			}
			r.ClusterAddr = a.Addr
			select {
			case a.DiscoveryC <- r:
			case <-a.ctx.Done():
				a.Infof("Closed, returning.")
				return
			default:
			}
		}
	}
}

const (
	// chanTransport is a channel type that can be used to open a net.Conn
	// through the reverse tunnel server. Used for trusted clusters and dial back
	// nodes.
	chanTransport = "teleport-transport"

	// chanTransportDialReq is the first (and only) request sent on a
	// chanTransport channel. It's payload is the address of the host a
	// connection should be established to.
	chanTransportDialReq = "teleport-transport-dial"

	chanHeartbeat    = "teleport-heartbeat"
	chanDiscovery    = "teleport-discovery"
	chanDiscoveryReq = "discovery"
)

const (
	// LocalNode is a special non-resolvable address that indicates the request
	// wants to connect to a dialed back node.
	LocalNode = "@local-node"
	// RemoteAuthServer is a special non-resolvable address that indicates client
	// requests a connection to the remote auth server.
	RemoteAuthServer = "@remote-auth-server"
	// RemoteKubeProxy is a special non-resolvable address that indicates that clients
	// requests a connection to the remote kubernetes proxy.
	// This has to be a valid domain name, so it lacks @
	RemoteKubeProxy = "remote.kube.proxy.teleport.cluster.local"
)
