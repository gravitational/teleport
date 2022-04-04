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
	"crypto/tls"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/reversetunnel/track"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/proxy"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

const (
	defaultAgentConnectionCount = 1
)

// ServerHandler implements an interface which can handle a connection
// (perform a handshake then process). This is needed because importing
// lib/srv in lib/reversetunnel causes a circular import.
type ServerHandler interface {
	// HandleConnection performs a handshake then process the connection.
	HandleConnection(conn net.Conn)
}

// AgentPool manages a pool of reverse tunnel agents.
type AgentPool struct {
	AgentPoolConfig
	active  *agentStore
	tracker *track.Tracker

	// runtimeConfig contains dynamic configuration values.
	runtimeConfig *agentPoolRuntimeConfig
	// events receives agent state change events.
	events chan *Agent

	// wg waits for the pool and all agents to complete.
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	// backoff limits the rate at which new agents are created.
	backoff utils.Retry
	log     logrus.FieldLogger
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
	// StateCallback is called for each agent event. This is an optional
	// parameter that is currently only used for testing.
	StateCallback AgentStateCallback
	// ProxiedServiceUpdater updates a proxied service with the proxies it is connected to.
	ProxiedServiceUpdater *ProxiedServiceUpdater
	// IgnoreTunnelStrategy ignores the tunnel strategy and connects to all proxies.
	IgnoreTunnelStrategy bool
}

// CheckAndSetDefaults checks and sets defaults.
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
	if cfg.Cluster == "" {
		return trace.BadParameter("missing 'Cluster' parameter")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.ProxiedServiceUpdater == nil {
		cfg.ProxiedServiceUpdater = NewProxiedServiceUpdater(cfg.Clock)
	}
	return nil
}

// NewAgentPool returns new instance of the agent pool.
func NewAgentPool(ctx context.Context, config AgentPoolConfig) (*AgentPool, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
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

	pool := &AgentPool{
		AgentPoolConfig: config,
		active:          newAgentStore(),
		events:          make(chan *Agent),
		wg:              sync.WaitGroup{},
		backoff:         retry,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentReverseTunnelAgent,
			trace.ComponentFields: logrus.Fields{
				"cluster": config.Cluster,
			},
		}),
		runtimeConfig: newAgentPoolRuntimeConfig(),
	}

	pool.runtimeConfig.ignoreTunnelStrategy = pool.IgnoreTunnelStrategy

	pool.ctx, pool.cancel = context.WithCancel(ctx)
	pool.tracker, err = track.New(pool.ctx, track.Config{ClusterName: pool.Cluster})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pool.tracker.Start()

	return pool, nil
}

// GetProxiedServiceUpdater returns the ProxiedServiceUpdater for this agent pool.
func (p *AgentPool) GetProxiedServiceUpdater() *ProxiedServiceUpdater {
	return p.ProxiedServiceUpdater
}

func (p *AgentPool) updateConnectedProxies() {
	if !p.runtimeConfig.reportConnectedProxies() {
		p.ProxiedServiceUpdater.setProxiesIDs([]string{})
		return
	}

	p.AgentPoolConfig.ProxiedServiceUpdater.setProxiesIDs(p.active.proxyIDs())
}

// Count is the number connected agents.
func (p *AgentPool) Count() int {
	return p.active.len()
}

// Start starts the agent pool in the background.
func (p *AgentPool) Start() error {
	p.log.Debugf("Starting agent pool %s.%s...", p.HostUUID, p.Cluster)
	p.tracker.Start()

	p.wg.Add(1)
	go func() {
		err := p.run()
		p.log.WithError(err).Warn("Agent pool exited.")

		p.cancel()
		p.wg.Done()
	}()
	return nil
}

// run connects agents until the agent pool context is done.
func (p *AgentPool) run() error {
	for {
		if p.ctx.Err() != nil {
			return trace.Wrap(p.ctx.Err())
		}

		err := p.connectAgent(p.ctx, p.tracker.Acquire(), p.events)
		if err != nil {
			p.log.WithError(err).Debugf("Failed to connect agent.")
		}

		err = p.waitForBackoff(p.ctx, p.events)
		if err != nil {
			p.log.WithError(err).Debugf("Failed to wait for backoff.")
		}
	}
}

// connectAgent connects a new agent and processes any agent events blocking until a
// new agent is connected or an error occurs.
func (p *AgentPool) connectAgent(ctx context.Context, leases <-chan track.Lease, events <-chan *Agent) error {
	var agent *Agent
	lease, err := p.waitForLease(ctx, leases, events)
	if err != nil {
		return trace.Wrap(err)
	}

	// Wrap in closure so we can release the lease on error in one place.
	err = func() error {
		err = p.processEvents(ctx, events)
		if err != nil {
			return trace.Wrap(err)
		}

		agent, err = p.newAgent(ctx, p.tracker, lease)
		if err != nil {
			return trace.Wrap(err)
		}

		err = agent.Start(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	}()
	if err != nil {
		lease.Release()
		return trace.Wrap(err)
	}

	p.wg.Add(1)
	p.active.add(agent)
	p.updateConnectedProxies()

	return nil
}

func (p *AgentPool) updateRuntimeConfig(ctx context.Context) {
	netConfig, err := p.AccessPoint.GetClusterNetworkingConfig(ctx)
	if err != nil {
		p.log.WithError(err).Warnf("Failed to get latest cluster networking config.")
		return
	}

	p.runtimeConfig.update(netConfig)
}

// processEvents handles all events in the queue. Unblocking when a new agent
// is required.
func (p *AgentPool) processEvents(ctx context.Context, events <-chan *Agent) error {
	// Processes any queued events without blocking.
	err := func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case agent := <-events:
				p.handleEvent(ctx, agent)
			default:
				return nil
			}
		}
	}()
	if err != nil {
		return trace.Wrap(err)
	}

	p.updateRuntimeConfig(ctx)
	p.disconnectAgents()
	if p.isAgentRequired() {
		return nil
	}

	// Continue to process new events until an agent is required.
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case agent := <-events:
			p.handleEvent(ctx, agent)

			p.updateRuntimeConfig(ctx)
			p.disconnectAgents()
			if p.isAgentRequired() {
				return nil
			}
		}
	}
}

// isAgentRequired returns true if a new agent is required.
func (p *AgentPool) isAgentRequired() bool {
	if !p.runtimeConfig.restrictConnectionCount() {
		return true
	}

	return p.active.len() < p.runtimeConfig.connectionCount
}

// disconnectAgents handles disconnecting agents that are no longer required.
func (p *AgentPool) disconnectAgents() {
	if !p.runtimeConfig.restrictConnectionCount() {
		return
	}

	for {
		agent, ok := p.active.poplen(p.runtimeConfig.connectionCount)
		if !ok {
			return
		}

		p.log.Debugf("Disconnecting agent %s.", agent)
		go func() {
			agent.Stop()
			p.wg.Done()
		}()
	}
}

// waitForLease processes events while waiting to acquire a lease.
func (p *AgentPool) waitForLease(ctx context.Context, leases <-chan track.Lease, events <-chan *Agent) (track.Lease, error) {
	for {
		select {
		case <-ctx.Done():
			return track.Lease{}, ctx.Err()
		case lease := <-leases:
			return lease, nil
		case agent := <-events:
			p.handleEvent(ctx, agent)
		}
	}
}

// waitForBackoff processes events while waiting for the backoff.
func (p *AgentPool) waitForBackoff(ctx context.Context, events <-chan *Agent) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.backoff.After():
			p.backoff.Inc()
			return nil
		case agent := <-events:
			p.handleEvent(ctx, agent)
		}
	}
}

// handleEvent processes a single event.
func (p *AgentPool) handleEvent(ctx context.Context, agent *Agent) {
	state := agent.GetState()
	switch state {
	case AgentConnected:
	case AgentClosed:
		if ok := p.active.remove(agent); ok {
			p.wg.Done()
		}
	}

	p.log.Debugf("Active agent count: %d", p.active.len())
}

// stateCallback adds events to the queue for each agent state change.
func (p *AgentPool) stateCallback(agent *Agent) {
	if p.StateCallback != nil {
		go p.StateCallback(agent)
	}
	select {
	case <-p.ctx.Done():
		// Handle events directly when the pool is closing.
		p.handleEvent(p.ctx, agent)
	case p.events <- agent:
	}
}

// newAgent creates a new agent instance.
func (p *AgentPool) newAgent(ctx context.Context, tracker *track.Tracker, lease track.Lease) (*Agent, error) {
	addr, err := p.Resolver()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	options := make([]proxy.DialerOptionFunc, 0)

	if p.runtimeConfig.useALPNRouting() {
		tlsConfig := &tls.Config{
			NextProtos: []string{string(alpncommon.ProtocolReverseTunnel)},
		}

		if p.runtimeConfig.useReverseTunnelV2() {
			tlsConfig.NextProtos = []string{
				string(alpncommon.ProtocolReverseTunnelV2),
				string(alpncommon.ProtocolReverseTunnel),
			}
		}

		options = append(options, proxy.WithALPNDialer(tlsConfig))
	}

	dialer := &agentDialer{
		client:      p.Client,
		fips:        p.FIPS,
		authMethods: []ssh.AuthMethod{ssh.PublicKeys(p.HostSigner)},
		options:     options,
		username:    p.HostUUID,
		log:         p.log,
	}

	agent, err := NewAgent(&agentConfig{
		addr:          *addr,
		keepAlive:     p.runtimeConfig.keepAliveInterval,
		stateCallback: p.stateCallback,
		sshDialer:     dialer,
		transporter:   p,
		versionGetter: p,
		tracker:       tracker,
		lease:         lease,
		clock:         p.Clock,
		log:           p.log,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return agent, nil
}

// Wait blocks until the pool context is stopped.
func (p *AgentPool) Wait() {
	if p == nil {
		return
	}

	<-p.ctx.Done()
	p.wg.Wait()
}

// Stop stops the pool and waits for all resources to be released.
func (p *AgentPool) Stop() {
	if p.cancel != nil {
		p.cancel()
	}

	p.wg.Wait()
}

// getVersion gets the connected auth server version.
func (p *AgentPool) getVersion(ctx context.Context) (string, error) {
	pong, err := p.Client.Ping(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return pong.ServerVersion, nil
}

// transport creates a new transport instance.
func (p *AgentPool) transport(ctx context.Context, channel ssh.Channel, requests <-chan *ssh.Request, conn ssh.Conn) *transport {
	return &transport{
		closeContext:        ctx,
		component:           p.Component,
		localClusterName:    p.LocalCluster,
		kubeDialAddr:        p.KubeDialAddr,
		authClient:          p.Client,
		reverseTunnelServer: p.ReverseTunnelServer,
		server:              p.Server,
		emitter:             p.Client,
		sconn:               conn,
		channel:             channel,
		requestCh:           requests,
		log:                 p.log,
	}
}

// agentPoolRuntimeConfig contains configurations dynamically set and updated
// during runtime.
type agentPoolRuntimeConfig struct {
	proxyListenerMode types.ProxyListenerMode
	// tunnelStrategyType is the tunnel strategy configured for the cluster.
	tunnelStrategyType types.TunnelStrategyType
	// connectionCount determines how many proxy servers the agent pool will
	// connect to. This settings is ignored for the AgentMesh tunnel strategy.
	connectionCount int
	// keepAliveInterval is the interval agents will send heartbeats at.
	keepAliveInterval time.Duration
	// ignoreTunnelStrategy forces the agent pool to connect to all proxies
	// regardless of the configured tunnel strategy.
	ignoreTunnelStrategy bool
}

func newAgentPoolRuntimeConfig() *agentPoolRuntimeConfig {
	return &agentPoolRuntimeConfig{
		tunnelStrategyType: types.AgentMesh,
		connectionCount:    defaultAgentConnectionCount,
		proxyListenerMode:  types.ProxyListenerMode_Separate,
		keepAliveInterval:  defaults.KeepAliveInterval(),
	}
}

// reportConnectedProxies returns true if the connected proxies should be reported.
func (c *agentPoolRuntimeConfig) reportConnectedProxies() bool {
	if c.ignoreTunnelStrategy {
		return false
	}
	return c.tunnelStrategyType == types.ProxyPeering
}

// reportConnectedProxies returns true if the the number of agents should be restricted.
func (c *agentPoolRuntimeConfig) restrictConnectionCount() bool {
	if c.ignoreTunnelStrategy {
		return false
	}
	return c.tunnelStrategyType == types.ProxyPeering
}

// useReverseTunnelV2 returns true if reverse tunnel should be used.
func (c *agentPoolRuntimeConfig) useReverseTunnelV2() bool {
	if c.ignoreTunnelStrategy {
		return false
	}
	return c.tunnelStrategyType == types.ProxyPeering
}

// useALPNRouting returns true agents should connect using alpn routing.
func (c *agentPoolRuntimeConfig) useALPNRouting() bool {
	return c.proxyListenerMode == types.ProxyListenerMode_Multiplex
}

func (c *agentPoolRuntimeConfig) update(netConfig types.ClusterNetworkingConfig) error {
	c.keepAliveInterval = netConfig.GetKeepAliveInterval()
	c.proxyListenerMode = netConfig.GetProxyListenerMode()

	// Fallback to agent mesh strategy if there is an error.
	strategyType, err := netConfig.GetTunnelStrategyType()
	if err != nil {
		c.tunnelStrategyType = types.AgentMesh
		return nil
	}

	c.tunnelStrategyType = strategyType
	if c.tunnelStrategyType == types.ProxyPeering {
		strategy := netConfig.GetProxyPeeringTunnelStrategy()
		c.connectionCount = int(strategy.AgentConnectionCount)
	}
	if c.connectionCount <= 0 {
		c.connectionCount = defaultAgentConnectionCount
	}

	return nil
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
