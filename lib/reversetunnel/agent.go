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

// Package reversetunnel sets up persistent reverse tunnel
// between remote site and teleport proxy, when site agents
// dial to teleport proxy's socket and teleport proxy can connect
// to any server through this tunnel.
package reversetunnel

import (
	"context"
	"fmt"
	"io"
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
)

// AgentConfig holds configuration for agent
type AgentConfig struct {
	// Addr is target address to dial
	Addr utils.NetAddr
	// RemoteCluster is a remote cluster name to connect to
	RemoteCluster string
	// Signers contains authentication signers
	Signers []ssh.Signer
	// Client is a client to the local auth servers
	Client *auth.TunClient
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
	hostKeyCallback utils.HostKeyCallback
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
		return fmt.Sprintf("agent(%v) -> %v:%v", a.getState(), a.RemoteCluster, a.Addr.String())
	}
	return fmt.Sprintf("agent(%v) -> %v:%v, discover %v", a.getState(), a.RemoteCluster, a.Addr.String(), Proxies(a.DiscoverProxies))
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
	a.Debugf("changing state %v -> %v", prev, state)
	a.state = state
	a.stateChange = a.Clock.Now().UTC()
	a.principals = principals
}
func (a *Agent) setState(state string) {
	a.Lock()
	defer a.Unlock()
	prev := a.state
	a.Debugf("changing state %v -> %v", prev, state)
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

// connectedTo returns true if connected services.Server passed in.
func (a *Agent) connectedTo(proxy services.Server) bool {
	principals := a.getPrincipals()
	proxyID := fmt.Sprintf("%v.%v", proxy.GetName(), a.RemoteCluster)
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
		dialer := proxy.DialerFromEnvironment()
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

func (a *Agent) proxyAccessPoint(ch ssh.Channel, req <-chan *ssh.Request) {
	a.Debugf("proxyAccessPoint")
	defer ch.Close()

	conn, err := a.Client.GetDialer()()
	if err != nil {
		a.Warningf("error dialing: %v", err)
		return
	}

	// apply read/write timeouts to this connection that are 10x of what normal
	// reverse tunnel ping is supposed to be:
	conn = utils.ObeyIdleTimeout(conn,
		defaults.ReverseTunnelAgentHeartbeatPeriod*10,
		"reverse tunnel client")

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer conn.Close()
		io.Copy(conn, ch)
	}()

	go func() {
		defer wg.Done()
		defer conn.Close()
		io.Copy(ch, conn)
	}()

	wg.Wait()
}

// proxyTransport runs as a goroutine running inside a reverse tunnel client
// and it establishes and maintains the following remote connection:
//
// tsh -> proxy(also reverse-tunnel-server) -> reverse-tunnel-agent
//
// ch   : SSH channel which received "teleport-transport" out-of-band request
// reqC : request payload
func (a *Agent) proxyTransport(ch ssh.Channel, reqC <-chan *ssh.Request) {
	a.Debugf("proxyTransport")
	defer ch.Close()

	// always push space into stderr to make sure the caller can always
	// safely call read(stderr) without blocking. this stderr is only used
	// to request proxying of TCP/IP via reverse tunnel.
	fmt.Fprint(ch.Stderr(), " ")

	var req *ssh.Request
	select {
	case <-a.ctx.Done():
		a.Infof("is closed, returning")
		return
	case req = <-reqC:
		if req == nil {
			a.Infof("connection closed, returning")
			return
		}
	case <-time.After(defaults.DefaultDialTimeout):
		a.Warningf("timeout waiting for dial")
		return
	}

	server := string(req.Payload)
	var servers []string

	// if the request is for the special string @remote-auth-server, then get the
	// list of auth servers and return that. otherwise try and connect to the
	// passed in server.
	if server == RemoteAuthServer {
		authServers, err := a.Client.GetAuthServers()
		if err != nil {
			a.Warningf("unable to find auth servers: %v", err)
			return
		}
		for _, as := range authServers {
			servers = append(servers, as.GetAddr())
		}
	} else {
		servers = append(servers, server)
	}

	a.Debugf("got out of band request %v", servers)

	var conn net.Conn
	var err error

	// loop over all servers and try and connect to one of them
	for _, s := range servers {
		conn, err = net.Dial("tcp", s)
		if err == nil {
			break
		}

		// log the reason we were not able to connect
		log.Debugf(trace.DebugReport(err))
	}

	// if we were not able to connect to any server, write the last connection
	// error to stderr of the caller (via SSH channel) so the error will be
	// propagated all the way back to the client (most likely tsh)
	if err != nil {
		fmt.Fprint(ch.Stderr(), err.Error())
		req.Reply(false, []byte(err.Error()))
		return
	}

	// successfully dialed
	req.Reply(true, []byte("connected"))
	a.Debugf("successfully dialed to %v, start proxying", server)

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		// make sure that we close the client connection on a channel
		// close, otherwise the other goroutine would never know
		// as it will block on read from the connection
		defer conn.Close()
		io.Copy(conn, ch)
	}()

	go func() {
		defer wg.Done()
		io.Copy(ch, conn)
	}()

	wg.Wait()
}

// run is the main agent loop, constantly tries to re-establish
// the connection until stopped. It has several operation modes:
// at first it tries to connect with fast retries on errors,
// but after a certain threshold it slows down retry pace
// by switching to longer delays between retries.
//
// Once run connects to a proxy it starts processing requests
// from the proxy via SSH channels opened by the remote Proxy.
//
// Agent sends periodic heartbeats back to the Proxy
// and that is how Proxy determines disconnects.
//
func (a *Agent) run() {
	ticker, err := utils.NewSwitchTicker(defaults.FastAttempts,
		defaults.NetworkRetryDuration, defaults.NetworkBackoffDuration)
	if err != nil {
		log.Errorf("failed to run: %v", err)
		return
	}
	defer ticker.Stop()
	firstAttempt := true
	for {
		if len(a.DiscoverProxies) != 0 {
			a.setStateAndPrincipals(agentStateDiscovering, nil)
		} else {
			a.setStateAndPrincipals(agentStateConnecting, nil)
		}

		// ignore timer and context on the first attempt
		if !firstAttempt {
			select {
			// abort if asked to stop:
			case <-a.ctx.Done():
				a.Debugf("agent has closed, exiting")
				return
				// wait backoff on network retries
			case <-ticker.Channel():
			}
		}

		// try and connect to remote cluster
		conn, err := a.connect()
		firstAttempt = false
		if err != nil || conn == nil {
			ticker.IncrementFailureCount()
			a.Warningf("failed to create remote tunnel: %v, conn: %v", err, conn)
			continue
		}

		// successfully connected to remote cluster
		ticker.Reset()
		a.Infof("connected to %s", conn.RemoteAddr())
		if len(a.DiscoverProxies) != 0 {
			// we did not connect to a proxy in the discover list (which means we
			// connected to a proxy we already have a connection to), try again
			if !a.connectedToRightProxy() {
				a.Debugf("missed, connected to %v instead of %v", a.getPrincipalsList(), Proxies(a.DiscoverProxies))
				conn.Close()
				continue
			}
			a.setState(agentStateDiscovered)
		} else {
			a.setState(agentStateConnected)
		}
		if a.EventsC != nil {
			select {
			case a.EventsC <- ConnectedEvent:
			case <-a.ctx.Done():
				a.Debugf("context is closing")
				return
			default:
			}
		}
		// start heartbeat even if error happend, it will reconnect
		// when this happens, this is #1 issue we have right now with Teleport. So we are making
		// it EASY to see in the logs. This condition should never be permanent (repeates
		// every XX seconds)
		if err := a.processRequests(conn); err != nil {
			log.Warn(err)
		}
	}
}

// ConnectedEvent is used to indicate that reverse tunnel has connected
const ConnectedEvent = "connected"

// processRequests is a blocking function which runs in a loop sending heartbeats
// to the given SSH connection and processes inbound requests from the
// remote proxy
func (a *Agent) processRequests(conn *ssh.Client) error {
	defer conn.Close()
	ticker := time.NewTicker(defaults.ReverseTunnelAgentHeartbeatPeriod)
	defer ticker.Stop()

	hb, reqC, err := conn.OpenChannel(chanHeartbeat, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	newAccesspointC := conn.HandleChannelOpen(chanAccessPoint)
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
				log.Error(err)
				return trace.Wrap(err)
			}
			a.Debugf("ping -> %v", conn.RemoteAddr())
		// ssh channel closed:
		case req := <-reqC:
			if req == nil {
				return trace.ConnectionProblem(nil, "heartbeat: connection closed")
			}
		// new access point request:
		case nch := <-newAccesspointC:
			if nch == nil {
				continue
			}
			a.Debugf("access point request: %v", nch.ChannelType())
			ch, req, err := nch.Accept()
			if err != nil {
				a.Warningf("failed to accept request: %v", err)
				continue
			}
			go a.proxyAccessPoint(ch, req)
		// new transport request:
		case nch := <-newTransportC:
			if nch == nil {
				continue
			}
			a.Debugf("transport request: %v", nch.ChannelType())
			ch, req, err := nch.Accept()
			if err != nil {
				a.Warningf("failed to accept request: %v", err)
				continue
			}
			go a.proxyTransport(ch, req)
		// new discovery request
		case nch := <-newDiscoveryC:
			if nch == nil {
				continue
			}
			a.Debugf("discovery request: %v", nch.ChannelType())
			ch, req, err := nch.Accept()
			if err != nil {
				a.Warningf("failed to accept request: %v", err)
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
	a.Debugf("handleDiscovery")
	defer ch.Close()

	for {
		var req *ssh.Request
		select {
		case <-a.ctx.Done():
			a.Infof("is closed, returning")
			return
		case req = <-reqC:
			if req == nil {
				a.Infof("connection closed, returning")
				return
			}
			r, err := unmarshalDiscoveryRequest(req.Payload)
			if err != nil {
				a.Warningf("bad payload: %v", err)
				return
			}
			r.ClusterAddr = a.Addr
			select {
			case a.DiscoveryC <- r:
			case <-a.ctx.Done():
				a.Infof("is closed, returning")
				return
			default:
			}
		}
	}
}

const (
	chanHeartbeat        = "teleport-heartbeat"
	chanAccessPoint      = "teleport-access-point"
	chanTransport        = "teleport-transport"
	chanTransportDialReq = "teleport-transport-dial"
	chanDiscovery        = "teleport-discovery"
)

const (
	// RemoteSiteStatusOffline indicates that site is considered as
	// offline, since it has missed a series of heartbeats
	RemoteSiteStatusOffline = "offline"
	// RemoteSiteStatusOnline indicates that site is sending heartbeats
	// at expected interval
	RemoteSiteStatusOnline = "online"
)

// RemoteAuthServer is a special non-resolvable address that indicates we want
// a connection to the remote auth server.
const RemoteAuthServer = "@remote-auth-server"
