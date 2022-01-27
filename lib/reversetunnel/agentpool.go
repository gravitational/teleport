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

package reversetunnel

import (
	"context"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnel/track"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// ServerHandler implements an interface which can handle a connection
// (perform a handshake then process). This is needed because importing
// lib/srv in lib/reversetunnel causes a circular import.
type ServerHandler interface {
	// HandleConnection performs a handshake then process the connection.
	HandleConnection(conn net.Conn)
}

// AgentPool manages the pool of outbound reverse tunnel agents.
// The agent pool watches the reverse tunnel entries created by the admin and
// connects/disconnects to added/deleted tunnels.
type AgentPool struct {
	log          *log.Entry
	cfg          AgentPoolConfig
	proxyTracker *track.Tracker

	// ctx controls the lifespan of the agent pool, and is used to control
	// all of the sub-processes it spawns.
	ctx    context.Context
	cancel context.CancelFunc
	// spawnLimiter limits agent spawn rate
	spawnLimiter utils.Retry

	mu     sync.Mutex
	agents []*Agent
}

// AgentPoolConfig holds configuration parameters for the agent pool
type AgentPoolConfig struct {
	// Client is client to the auth server this agent connects to receive
	// a list of pools
	Client auth.ClientI
	// AccessPoint is a lightweight access point
	// that can optionally cache some values
	AccessPoint auth.AccessCache
	// HostSigner is a host signer this agent presents itself as
	HostSigner ssh.Signer
	// HostUUID is a unique ID of this host
	HostUUID string
	// LocalCluster is a cluster name this client is a member of.
	LocalCluster string
	// Clock is a clock used to get time, if not set,
	// system clock is used
	Clock clockwork.Clock
	// KubeDialAddr is an address of a kubernetes proxy
	KubeDialAddr utils.NetAddr
	// Server is either an SSH or application server. It can handle a connection
	// (perform handshake and handle request).
	Server ServerHandler
	// Component is the Teleport component this agent pool is running in. It can
	// either be proxy (trusted clusters) or node (dial back).
	Component string
	// ReverseTunnelServer holds all reverse tunnel connections.
	ReverseTunnelServer Server
	// Resolver retrieves the reverse tunnel address
	Resolver Resolver
	// Cluster is a cluster name of the proxy.
	Cluster string
	// FIPS indicates if Teleport was started in FIPS mode.
	FIPS bool
}

// CheckAndSetDefaults checks and sets defaults
func (cfg *AgentPoolConfig) CheckAndSetDefaults() error {
	if cfg.Client == nil {
		return trace.BadParameter("missing 'Client' parameter")
	}
	if cfg.AccessPoint == nil {
		return trace.BadParameter("missing 'AccessPoint' parameter")
	}
	if cfg.HostSigner == nil {
		return trace.BadParameter("missing 'HostSigner' parameter")
	}
	if len(cfg.HostUUID) == 0 {
		return trace.BadParameter("missing 'HostUUID' parameter")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.Cluster == "" {
		return trace.BadParameter("missing 'Cluster' parameter")
	}
	return nil
}

// NewAgentPool returns new instance of the agent pool
func NewAgentPool(ctx context.Context, cfg AgentPoolConfig) (*AgentPool, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	retry, err := utils.NewLinear(utils.LinearConfig{
		Step:      time.Second,
		Max:       time.Second * 8,
		Jitter:    utils.NewJitter(),
		AutoReset: 4,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(ctx)
	tr, err := track.New(ctx, track.Config{ClusterName: cfg.Cluster})
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	pool := &AgentPool{
		agents:       nil,
		proxyTracker: tr,
		cfg:          cfg,
		ctx:          ctx,
		cancel:       cancel,
		spawnLimiter: retry,
		log: log.WithFields(log.Fields{
			trace.Component: teleport.ComponentReverseTunnelAgent,
			trace.ComponentFields: log.Fields{
				"cluster": cfg.Cluster,
			},
		}),
	}
	pool.proxyTracker.Start()
	return pool, nil
}

// Start starts the agent pool
func (m *AgentPool) Start() error {
	m.log.Debugf("Starting agent pool %s.%s...", m.cfg.HostUUID, m.cfg.Cluster)
	go m.pollAndSyncAgents()
	go m.processSeekEvents()
	return nil
}

// Stop stops the agent pool
func (m *AgentPool) Stop() {
	if m == nil {
		return
	}
	m.cancel()
}

// Wait returns when agent pool is closed
func (m *AgentPool) Wait() {
	if m == nil {
		return
	}
	<-m.ctx.Done()
}

// processSeekEvents receives acquisition messages from the ProxyTracker
// (i.e. "I've found a proxy that you may not know about") and routes the
// new proxy address to the AgentPool, which will manage the connection
// to that address.
func (m *AgentPool) processSeekEvents() {
	limiter := m.spawnLimiter.Clone()
	for {
		select {
		case <-m.ctx.Done():
			m.log.Debugf("Halting seek event processing (pool closing)")
			return

		// The proxy tracker has given us permission to act on a given
		// tunnel address
		case lease := <-m.proxyTracker.Acquire():
			m.withLock(func() {
				// Note that ownership of the lease is transferred to agent
				// pool for the lifetime of the connection
				if err := m.addAgent(lease); err != nil {
					m.log.WithError(err).Errorf("Failed to add agent.")
				}
			})
		}
		select {
		case <-m.ctx.Done():
			m.log.Debugf("Halting seek event processing (pool closing)")
			return
		case <-limiter.After():
			limiter.Inc()
		}
	}
}

func (m *AgentPool) withLock(f func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f()
}

type matchAgentFn func(a *Agent) bool

func (m *AgentPool) closeAgents() {
	agents := filterAndClose(m.agents, func(*Agent) bool { return true })
	if len(agents) <= 0 {
		m.agents = nil
	} else {
		m.agents = agents
	}
}

func filterAndClose(agents []*Agent, matchAgent matchAgentFn) []*Agent {
	var filtered []*Agent
	for i := range agents {
		agent := agents[i]
		if matchAgent(agent) {
			agent.log.Debugf("Pool is closing agent.")
			if err := agent.Close(); err != nil {
				agent.log.WithError(err).Warnf("Failed to close agent")
			}
		} else {
			filtered = append(filtered, agent)
		}
	}
	return filtered
}

func (m *AgentPool) pollAndSyncAgents() {
	ticker := time.NewTicker(defaults.ResyncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			m.withLock(m.closeAgents)
			m.log.Debugf("Closing.")
			return
		case <-ticker.C:
			m.withLock(m.removeDisconnected)
		}
	}
}

// getReverseTunnelDetails gets the cached ReverseTunnelDetails obtained during the oldest cached agent.connect call.
// This function should be called under a lock.
func (m *AgentPool) getReverseTunnelDetails() *reverseTunnelDetails {
	if len(m.agents) <= 0 {
		return nil
	}
	return m.agents[0].reverseTunnelDetails
}

// addAgent adds a new agent to the pool. Note that ownership of the lease
// transfers into the AgentPool, and will be released when the AgentPool
// is done with it.
func (m *AgentPool) addAgent(lease track.Lease) error {
	addr, err := m.cfg.Resolver()
	if err != nil {
		return trace.Wrap(err)
	}

	agent, err := NewAgent(AgentConfig{
		Addr:                 *addr,
		ClusterName:          m.cfg.Cluster,
		Username:             m.cfg.HostUUID,
		Signer:               m.cfg.HostSigner,
		Client:               m.cfg.Client,
		AccessPoint:          m.cfg.AccessPoint,
		Context:              m.ctx,
		KubeDialAddr:         m.cfg.KubeDialAddr,
		Server:               m.cfg.Server,
		ReverseTunnelServer:  m.cfg.ReverseTunnelServer,
		LocalClusterName:     m.cfg.LocalCluster,
		Component:            m.cfg.Component,
		Tracker:              m.proxyTracker,
		Lease:                lease,
		FIPS:                 m.cfg.FIPS,
		reverseTunnelDetails: m.getReverseTunnelDetails(),
	})
	if err != nil {
		// ensure that lease has been released; OK to call multiple times.
		lease.Release()
		return trace.Wrap(err)
	}
	m.log.Debugf("Adding %v.", agent)
	// start the agent in a goroutine. no need to handle Start() errors: Start() will be
	// retrying itself until the agent is closed
	go agent.Start()
	m.agents = append(m.agents, agent)
	return nil
}

// Count returns a count of the number of proxies an outbound tunnel is
// connected to. Used in tests to determine if a proxy has been found and/or
// removed.
func (m *AgentPool) Count() int {
	var out int
	m.withLock(func() {
		for _, agent := range m.agents {
			if agent.getState() == agentStateConnected {
				out++
			}
		}
	})

	return out
}

// removeDisconnected removes disconnected agents from the list of agents.
// This function should be called under a lock.
func (m *AgentPool) removeDisconnected() {
	// Filter and close all disconnected agents.
	agents := filterAndClose(m.agents, func(agent *Agent) bool {
		return agent.getState() == agentStateDisconnected
	})

	if len(agents) <= 0 {
		m.agents = nil
	} else {
		m.agents = agents
	}
}

// Make sure ServerHandlerToListener implements both interfaces.
var _ = net.Listener(ServerHandlerToListener{})
var _ = ServerHandler(ServerHandlerToListener{})

// ServerHandlerToListener is an adapter from ServerHandler to net.Listener. It
// can be used as a Server field in AgentPoolConfig, while also being passed to
// http.Server.Serve (or any other func Serve(net.Listener)).
type ServerHandlerToListener struct {
	connCh     chan net.Conn
	closeOnce  *sync.Once
	tunnelAddr string
}

// NewServerHandlerToListener creates a new ServerHandlerToListener adapter.
func NewServerHandlerToListener(tunnelAddr string) ServerHandlerToListener {
	return ServerHandlerToListener{
		connCh:     make(chan net.Conn),
		closeOnce:  new(sync.Once),
		tunnelAddr: tunnelAddr,
	}
}

func (l ServerHandlerToListener) HandleConnection(c net.Conn) {
	// HandleConnection must block as long as c is used.
	// Wrap c to only return after c.Close() has been called.
	cc := newConnCloser(c)
	l.connCh <- cc
	cc.wait()
}

func (l ServerHandlerToListener) Accept() (net.Conn, error) {
	c, ok := <-l.connCh
	if !ok {
		return nil, io.EOF
	}
	return c, nil
}

func (l ServerHandlerToListener) Close() error {
	l.closeOnce.Do(func() { close(l.connCh) })
	return nil
}

func (l ServerHandlerToListener) Addr() net.Addr {
	return reverseTunnelAddr(l.tunnelAddr)
}

type connCloser struct {
	net.Conn
	closeOnce *sync.Once
	closed    chan struct{}
}

func newConnCloser(c net.Conn) connCloser {
	return connCloser{Conn: c, closeOnce: new(sync.Once), closed: make(chan struct{})}
}

func (c connCloser) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return c.Conn.Close()
}

func (c connCloser) wait() { <-c.closed }

// reverseTunnelAddr is a net.Addr implementation for a listener based on a
// reverse tunnel.
type reverseTunnelAddr string

func (reverseTunnelAddr) Network() string  { return "ssh-reversetunnel" }
func (a reverseTunnelAddr) String() string { return string(a) }
