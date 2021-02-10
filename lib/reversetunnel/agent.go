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
	"github.com/gravitational/teleport/api/constants"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnel/track"
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
	// agentStateConnected means that agent has connected to instance
	agentStateConnected = "connected"
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
	// Signers contains authentication signer
	Signer ssh.Signer
	// Client is a client to the local auth servers
	Client client.ClientI
	// AccessPoint is a caching access point to the local auth servers
	AccessPoint auth.AccessPoint
	// Context is a parent context
	Context context.Context
	// Username is the name of this client used to authenticate on SSH
	Username string
	// Clock is a clock passed in tests, if not set wall clock
	// will be used
	Clock clockwork.Clock
	// EventsC is an optional events channel, used for testing purposes
	EventsC chan string
	// KubeDialAddr is a dial address for kubernetes proxy
	KubeDialAddr utils.NetAddr
	// Server is either an SSH or application server. It can handle a connection
	// (perform handshake and handle request).
	Server ServerHandler
	// ReverseTunnelServer holds all reverse tunnel connections.
	ReverseTunnelServer Server
	// LocalClusterName is the name of the cluster this agent is running in.
	LocalClusterName string
	// Component is the teleport component that this agent runs in.
	// It's important for routing incoming requests for local services (like an
	// IoT node or kubernetes service).
	Component string
	// Tracker tracks proxy
	Tracker *track.Tracker
	// Lease manages gossip and exclusive claims.  Lease may be nil
	// when used in the context of tests.
	Lease track.Lease
	// Log optionally specifies the logger
	Log log.FieldLogger
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
	if a.Signer == nil {
		return trace.BadParameter("missing parameter Signer")
	}
	if len(a.Username) == 0 {
		return trace.BadParameter("missing parameter Username")
	}
	if a.Clock == nil {
		a.Clock = clockwork.NewRealClock()
	}
	logger := a.Log
	if a.Log == nil {
		logger = log.StandardLogger()
	}
	a.Log = logger.WithFields(log.Fields{
		trace.Component: teleport.Component(a.Component, teleport.ComponentReverseTunnelAgent),
		trace.ComponentFields: log.Fields{
			"target":  a.Addr.String(),
			"leaseID": a.Lease.ID(),
		},
	})
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
	AgentConfig
	log         log.FieldLogger
	ctx         context.Context
	cancel      context.CancelFunc
	authMethods []ssh.AuthMethod
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
		authMethods: []ssh.AuthMethod{ssh.PublicKeys(cfg.Signer)},
		state:       agentStateConnecting,
		log:         cfg.Log,
	}
	return a, nil
}

func (a *Agent) String() string {
	return fmt.Sprintf("agent(leaseID=%d,state=%v) -> %v:%v", a.Lease.ID(), a.getState(), a.ClusterName, a.Addr.String())
}

func (a *Agent) setState(state string) {
	a.Lock()
	defer a.Unlock()
	prev := a.state
	if prev != state {
		a.log.Debugf("Changing state %v -> %v.", prev, state)
	}
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
		checkers, err := sshutils.GetCheckers(ca)
		if err != nil {
			return trace.BadParameter("error parsing key: %v", err)
		}
		for _, checker := range checkers {
			if apisshutils.KeysEqual(checker, cert.SignatureKey) {
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
		// Create a dialer (that respects HTTP proxies) and connect to remote host.
		dialer := proxy.DialerFromEnvironment(a.Addr.Addr)
		pconn, err := dialer.DialTimeout(a.Addr.AddrNetwork, a.Addr.Addr, defaults.DefaultDialTimeout)
		if err != nil {
			a.log.Debugf("Dial to %v failed: %v.", a.Addr.Addr, err)
			continue
		}

		// Build a new client connection. This is done to get access to incoming
		// global requests which dialer.Dial would not provide.
		conn, chans, reqs, err := ssh.NewClientConn(pconn, a.Addr.Addr, &ssh.ClientConfig{
			User:            a.Username,
			Auth:            []ssh.AuthMethod{authMethod},
			HostKeyCallback: a.checkHostSignature,
			Timeout:         defaults.DefaultDialTimeout,
		})
		if err != nil {
			a.log.Debugf("Failed to create client to %v: %v.", a.Addr.Addr, err)
			continue
		}

		// Create an empty channel and close it right away. This will prevent
		// ssh.NewClient from attempting to process any incoming requests.
		emptyCh := make(chan *ssh.Request)
		close(emptyCh)

		client := ssh.NewClient(conn, chans, emptyCh)

		// Start a goroutine to process global requests from the server.
		go a.handleGlobalRequests(a.ctx, reqs)

		return client, nil
	}
	return nil, trace.BadParameter("failed to dial: all auth methods failed")
}

// handleGlobalRequests processes global requests from the server.
func (a *Agent) handleGlobalRequests(ctx context.Context, requestCh <-chan *ssh.Request) {
	for {
		select {
		case r := <-requestCh:
			// When the channel is closing, nil is returned.
			if r == nil {
				return
			}

			switch r.Type {
			case versionRequest:
				err := r.Reply(true, []byte(teleport.Version))
				if err != nil {
					log.Debugf("Failed to reply to %v request: %v.", r.Type, err)
					continue
				}
			default:
				// This handles keep-alive messages and matches the behaviour of OpenSSH.
				err := r.Reply(false, nil)
				if err != nil {
					log.Debugf("Failed to reply to %v request: %v.", r.Type, err)
					continue
				}
			}
		case <-ctx.Done():
			return
		}
	}
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
	defer a.Lease.Release()

	a.setState(agentStateConnecting)

	// Try and connect to remote cluster.
	conn, err := a.connect()
	if err != nil || conn == nil {
		a.log.Warningf("Failed to create remote tunnel: %v, conn: %v.", err, conn)
		return
	}
	defer conn.Close()

	// Successfully connected to remote cluster.
	a.log.WithFields(log.Fields{
		"addr":        conn.LocalAddr().String(),
		"remote-addr": conn.RemoteAddr().String(),
	}).Info("Connected.")

	// wrap up remaining business logic in closure for easy
	// conditional execution.
	doWork := func() {
		a.log.Debugf("Agent connected to proxy: %v.", a.getPrincipalsList())
		a.setState(agentStateConnected)
		// Notify waiters that the agent has connected.
		if a.EventsC != nil {
			select {
			case a.EventsC <- ConnectedEvent:
			case <-a.ctx.Done():
				a.log.Debug("Context is closing.")
				return
			default:
			}
		}

		// A connection has been established start - processing requests. Note that
		// this function blocks while the connection is up. It will unblock when
		// the connection is closed either due to intermittent connectivity issues
		// or permanent loss of a proxy.
		err = a.processRequests(conn)
		if err != nil {
			a.log.Warnf("Unable to continue processesing requests: %v.", err)
			return
		}
	}
	// if Tracker was provided, then the agent shouldn't continue unless
	// no other agents hold a claim.
	if a.Tracker != nil {
		if !a.Tracker.WithProxy(doWork, a.Lease, a.getPrincipalsList()...) {
			a.log.Debugf("Proxy already held by other agent: %v, releasing.", a.getPrincipalsList())
		}
	} else {
		doWork()
	}
}

// ConnectedEvent is used to indicate that reverse tunnel has connected
const ConnectedEvent = "connected"

// processRequests is a blocking function which runs in a loop sending heartbeats
// to the given SSH connection and processes inbound requests from the
// remote proxy
func (a *Agent) processRequests(conn *ssh.Client) error {
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
	newTransportC := conn.HandleChannelOpen(constants.ChanTransport)
	newDiscoveryC := conn.HandleChannelOpen(chanDiscovery)

	// send first ping right away, then start a ping timer:
	if _, err := hb.SendRequest("ping", false, nil); err != nil {
		return trace.Wrap(err)
	}

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
				a.log.Error(err)
				return trace.Wrap(err)
			}
			a.log.Debugf("Ping -> %v.", conn.RemoteAddr())
		// ssh channel closed:
		case req := <-reqC:
			if req == nil {
				return trace.ConnectionProblem(nil, "heartbeat: connection closed")
			}
		// Handle transport requests.
		case nch := <-newTransportC:
			if nch == nil {
				continue
			}
			a.log.Debugf("Transport request: %v.", nch.ChannelType())
			ch, req, err := nch.Accept()
			if err != nil {
				a.log.Warningf("Failed to accept transport request: %v.", err)
				continue
			}

			t := &transport{
				log:                 a.log,
				closeContext:        a.ctx,
				authClient:          a.Client,
				kubeDialAddr:        a.KubeDialAddr,
				channel:             ch,
				requestCh:           req,
				sconn:               conn.Conn,
				server:              a.Server,
				component:           a.Component,
				reverseTunnelServer: a.ReverseTunnelServer,
				localClusterName:    a.LocalClusterName,
			}
			go t.start()
		// new discovery request channel
		case nch := <-newDiscoveryC:
			if nch == nil {
				continue
			}
			a.log.Debugf("Discovery request channel opened: %v.", nch.ChannelType())
			ch, req, err := nch.Accept()
			if err != nil {
				a.log.Warningf("Failed to accept discovery channel request: %v.", err)
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
	a.log.Debugf("handleDiscovery requests channel.")
	defer ch.Close()

	for {
		var req *ssh.Request
		select {
		case <-a.ctx.Done():
			return
		case req = <-reqC:
			if req == nil {
				a.log.Infof("Connection closed, returning")
				return
			}
			r, err := unmarshalDiscoveryRequest(req.Payload)
			if err != nil {
				a.log.Warningf("Bad payload: %v.", err)
				return
			}
			r.ClusterAddr = a.Addr
			if a.Tracker != nil {
				// Notify tracker of all known proxies.
				for _, p := range r.Proxies {
					a.Tracker.TrackExpected(a.Lease, p.GetName())
				}
			}
		}
	}
}

const (
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
	// LocalKubernetes is a special non-resolvable address that indicates that clients
	// requests a connection to the kubernetes endpoint of the local proxy.
	// This has to be a valid domain name, so it lacks @
	LocalKubernetes = "remote.kube.proxy.teleport.cluster.local"
)
