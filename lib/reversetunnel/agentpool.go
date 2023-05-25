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
	"crypto/x509"
	"errors"
	"io"
	"math"
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/reversetunnel/track"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/proxy"
)

const (
	// defaultAgentConnectionCount is the default connection count used when in
	// proxy peering mode.
	defaultAgentConnectionCount = 1
	// maxBackoff sets the maximum backoff for creating new agents.
	maxBackoff = time.Second * 8
)

// ServerHandler implements an interface which can handle a connection
// (perform a handshake then process). This is needed because importing
// lib/srv in lib/reversetunnel causes a circular import.
type ServerHandler interface {
	// HandleConnection performs a handshake then process the connection.
	HandleConnection(conn net.Conn)
}

type newAgentFunc func(context.Context, *track.Tracker, track.Lease) (Agent, error)

// AgentPool manages a pool of reverse tunnel agents.
type AgentPool struct {
	AgentPoolConfig
	active  *agentStore
	tracker *track.Tracker

	// runtimeConfig contains dynamic configuration values.
	runtimeConfig *runtimeConfig

	// events receives agent state change events.
	events chan Agent

	// newAgentFunc is used during testing to mock new agents.
	newAgentFunc              newAgentFunc
	tlsRoutingUpgradeRequired func() bool

	// wg waits for the pool and all agents to complete.
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	// backoff limits the rate at which new agents are created.
	backoff retryutils.Retry
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
	Resolver reversetunnelclient.Resolver
	// Cluster is a cluster name of the proxy.
	Cluster string
	// FIPS indicates if Teleport was started in FIPS mode.
	FIPS bool
	// ProxiedServiceUpdater updates a proxied service with the proxies it is connected to.
	ConnectedProxyGetter *ConnectedProxyGetter
	// IsRemoteCluster indicates the agent pool is connecting to a remote cluster.
	// This means the tunnel strategy should be ignored and tls routing is determined
	// by the remote cluster.
	IsRemoteCluster bool
	// DisableCreateHostUser disables host user creation on a node.
	DisableCreateHostUser bool
	// LocalAuthAddresses is a list of auth servers to use when dialing back to
	// the local cluster.
	LocalAuthAddresses []string
	// PROXYSigner is used to sign PROXY headers for securely propagating client IP address
	PROXYSigner multiplexer.PROXYHeaderSigner
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
	if cfg.ConnectedProxyGetter == nil {
		cfg.ConnectedProxyGetter = NewConnectedProxyGetter()
	}
	return nil
}

// Agent represents a reverse tunnel agent.
type Agent interface {
	// Start starts the agent in the background.
	Start(context.Context) error
	// Stop closes the agent and releases resources.
	Stop() error
	// GetState returns the current state of the agent.
	GetState() AgentState
	// GetProxyID returns the proxy id of the proxy the agent is connected to.
	GetProxyID() (string, bool)
	// GetClusterNetworkConfig attempts to retrieve the cluster networing config
	// over the agents reversetunnel.
	GetClusterNetworkConfig(context.Context) (types.ClusterNetworkingConfig, error)
}

// NewAgentPool returns new instance of the agent pool.
func NewAgentPool(ctx context.Context, config AgentPoolConfig) (*AgentPool, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		Step:      time.Second,
		Max:       maxBackoff,
		Jitter:    retryutils.NewJitter(),
		AutoReset: 4,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pool := &AgentPool{
		AgentPoolConfig: config,
		active:          newAgentStore(),
		events:          make(chan Agent),
		wg:              sync.WaitGroup{},
		backoff:         retry,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentReverseTunnelAgent,
			trace.ComponentFields: logrus.Fields{
				"targetCluster": config.Cluster,
				"localCluster":  config.LocalCluster,
			},
		}),
		runtimeConfig: newRuntimeConfig(),
	}

	pool.newAgentFunc = pool.newAgent
	pool.tlsRoutingUpgradeRequired = tlsRoutingUpgradeRequiredCache(ctx, time.Second*15, config.Resolver, pool.Clock, pool.log)

	if err := pool.updateRuntimeConfig(ctx); err != nil {
		pool.log.WithError(err).Debug("unable to init runtime config")
	}

	pool.ctx, pool.cancel = context.WithCancel(ctx)
	pool.tracker, err = track.New(pool.ctx, track.Config{ClusterName: pool.Cluster})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pool.tracker.Start()

	return pool, nil
}

func (p *AgentPool) updateRuntimeConfig(ctx context.Context) error {
	var (
		netconfig types.ClusterNetworkingConfig
		err       error
	)
	if p.IsRemoteCluster {
		netconfig, err = p.updateRemoteRuntimeConfig(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		netconfig, err = p.AccessPoint.GetClusterNetworkingConfig(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	var tlsRoutingUpgradeRequired bool
	if netconfig.GetProxyListenerMode() == types.ProxyListenerMode_Multiplex {
		tlsRoutingUpgradeRequired = p.tlsRoutingUpgradeRequired()
	}

	p.runtimeConfig.update(netconfig, tlsRoutingUpgradeRequired)
	return nil
}

func (p *AgentPool) updateRemoteRuntimeConfig(ctx context.Context) (types.ClusterNetworkingConfig, error) {
	var rejectedRequest bool
	agent, ok := p.active.getLatest()
	if ok {
		// Try to get netconfig from agent otherwise fallback to ping.
		netconfig, err := agent.GetClusterNetworkConfig(ctx)
		if err != nil && !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
		if err == nil {
			return netconfig, nil
		}
		rejectedRequest = true
	}

	addr, _, err := p.Resolver(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var tlsRoutingEnabled bool
	// If we are unable to get the network config from an agent hit the ping endpoint
	// to determine if tls routing is enabled or not.
	ping, err := webclient.Find(&webclient.Config{
		Context:   ctx,
		ProxyAddr: addr.Addr,
		Insecure:  lib.IsInsecureDevMode(),
	})
	if err != nil {
		// If TLS Routing is disabled the address is the proxy reverse tunnel
		// address the ping call will always fail with tls.RecordHeaderError.
		if ok := errors.As(err, &tls.RecordHeaderError{}); !ok {
			return nil, trace.Wrap(err)
		}
	}
	if ping != nil {
		// Only use the ping results if they weren't from a minimal handler.
		// The minimal API handler only exists when the proxy and reverse tunnel are
		// listening on separate ports, so it will never do TLS routing.
		isMinimalHandler := addr.Addr == ping.Proxy.SSH.TunnelListenAddr &&
			ping.Proxy.SSH.TunnelListenAddr != ping.Proxy.SSH.WebListenAddr
		if !isMinimalHandler {
			tlsRoutingEnabled = ping.Proxy.TLSRoutingEnabled
		}
	}

	netconfig := p.runtimeConfig.copy()
	if tlsRoutingEnabled {
		netconfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
	}

	// Fallback to agent mesh if proxies do not support netconfig requests.
	if rejectedRequest {
		netconfig.SetTunnelStrategy(&types.TunnelStrategyV1{
			Strategy: &types.TunnelStrategyV1_AgentMesh{AgentMesh: types.DefaultAgentMeshTunnelStrategy()},
		})
	}

	return netconfig, nil
}

// GetConnectedProxyGetter returns the ConnectedProxyGetter for this agent pool.
func (p *AgentPool) GetConnectedProxyGetter() *ConnectedProxyGetter {
	return p.ConnectedProxyGetter
}

func (p *AgentPool) updateConnectedProxies() {
	if p.IsRemoteCluster {
		trustedClustersStats.WithLabelValues(p.Cluster).Set(float64(p.active.len()))
	}

	if !p.runtimeConfig.reportConnectedProxies() {
		p.ConnectedProxyGetter.setProxyIDs(nil)
		return
	}

	proxies := p.active.proxyIDs()
	p.log.Debugf("Updating connected proxies: %v", proxies)
	p.AgentPoolConfig.ConnectedProxyGetter.setProxyIDs(proxies)
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

		agent, err := p.connectAgent(p.ctx, p.tracker.Acquire(), p.events)
		if err != nil {
			p.log.WithError(err).Debugf("Failed to connect agent.")
		} else {
			p.wg.Add(1)
			p.active.add(agent)
			p.updateConnectedProxies()
		}

		err = p.waitForBackoff(p.ctx, p.events)
		if err != nil {
			p.log.WithError(err).Debugf("Failed to wait for backoff.")
		}
	}
}

// connectAgent connects a new agent and processes any agent events blocking until a
// new agent is connected or an error occurs.
func (p *AgentPool) connectAgent(ctx context.Context, leases <-chan track.Lease, events <-chan Agent) (Agent, error) {
	lease, err := p.waitForLease(ctx, leases, events)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Ensure the lease is released on error.
	defer func() {
		if err != nil {
			lease.Release()
		}
	}()

	err = p.processEvents(ctx, events)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	agent, err := p.newAgentFunc(ctx, p.tracker, lease)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = agent.Start(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return agent, nil
}

// processEvents handles all events in the queue. Unblocking when a new agent
// is required.
func (p *AgentPool) processEvents(ctx context.Context, events <-chan Agent) error {
	// Processes any queued events without blocking.
	for {
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case agent := <-events:
			p.handleEvent(ctx, agent)
			continue
		default:
		}
		break
	}

	err := p.updateRuntimeConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	p.disconnectAgents()
	if p.isAgentRequired() {
		return nil
	}

	// Continue to process new events until an agent is required.
	for {
		p.log.Debugf("Processing events...")
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case agent := <-events:
			p.handleEvent(ctx, agent)

			err := p.updateRuntimeConfig(ctx)
			if err != nil {
				return trace.Wrap(err)
			}

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

	return p.active.len() < p.runtimeConfig.connectionCount()
}

// disconnectAgents handles disconnecting agents that are no longer required.
func (p *AgentPool) disconnectAgents() {
	if !p.runtimeConfig.restrictConnectionCount() {
		return
	}

	for {
		agent, ok := p.active.poplen(p.runtimeConfig.connectionCount())
		if !ok {
			p.updateConnectedProxies()
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
func (p *AgentPool) waitForLease(ctx context.Context, leases <-chan track.Lease, events <-chan Agent) (track.Lease, error) {
	for {
		select {
		case <-ctx.Done():
			return track.Lease{}, trace.Wrap(ctx.Err())
		case lease := <-leases:
			return lease, nil
		case agent := <-events:
			p.handleEvent(ctx, agent)
		}
	}
}

// waitForBackoff processes events while waiting for the backoff.
func (p *AgentPool) waitForBackoff(ctx context.Context, events <-chan Agent) error {
	backoffC := p.backoff.After()

	for {
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-backoffC:
			p.backoff.Inc()
			return nil
		case agent := <-events:
			p.handleEvent(ctx, agent)
		}
	}
}

// handleEvent processes a single event.
func (p *AgentPool) handleEvent(ctx context.Context, agent Agent) {
	state := agent.GetState()
	switch state {
	case AgentConnecting:
		return
	case AgentConnected:
	case AgentClosed:
		if ok := p.active.remove(agent); ok {
			p.wg.Done()
		}
	}
	p.updateConnectedProxies()
	p.log.Debugf("Active agent count: %d", p.active.len())
}

// stateCallback adds events to the queue for each agent state change.
func (p *AgentPool) getStateCallback(agent Agent) AgentStateCallback {
	return func(_ AgentState) {
		select {
		case <-p.ctx.Done():
			// Handle events directly when the pool is closing.
			p.handleEvent(p.ctx, agent)
		case p.events <- agent:
		}
	}
}

// newAgent creates a new agent instance.
func (p *AgentPool) newAgent(ctx context.Context, tracker *track.Tracker, lease track.Lease) (Agent, error) {
	addr, _, err := p.Resolver(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	options := []proxy.DialerOptionFunc{proxy.WithInsecureSkipTLSVerify(lib.IsInsecureDevMode())}
	if p.runtimeConfig.useALPNRouting() {
		options = append(options, proxy.WithALPNDialer(p.runtimeConfig.alpnDialerConfig(p.getClusterCAs)))
	}

	dialer := &agentDialer{
		client:      p.Client,
		fips:        p.FIPS,
		authMethods: []ssh.AuthMethod{ssh.PublicKeys(p.HostSigner)},
		options:     options,
		username:    p.HostUUID,
		log:         p.log,
	}

	agent, err := newAgent(agentConfig{
		addr:               *addr,
		keepAlive:          p.runtimeConfig.keepaliveInterval(),
		sshDialer:          dialer,
		transporter:        p,
		versionGetter:      p,
		tracker:            tracker,
		lease:              lease,
		clock:              p.Clock,
		log:                p.log,
		localAuthAddresses: p.LocalAuthAddresses,
		proxySigner:        p.PROXYSigner,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	agent.stateCallback = p.getStateCallback(agent)
	return agent, nil
}

func (p *AgentPool) getClusterCAs(_ context.Context) (*x509.CertPool, error) {
	clusterCAs, _, err := auth.ClientCertPool(p.AccessPoint, p.Cluster, types.HostCA)
	return clusterCAs, trace.Wrap(err)
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
	if p == nil {
		return
	}
	p.cancel()
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
func (p *AgentPool) transport(ctx context.Context, channel ssh.Channel, requests <-chan *ssh.Request, conn sshutils.Conn) *transport {
	return &transport{
		closeContext:         ctx,
		component:            p.Component,
		localClusterName:     p.LocalCluster,
		kubeDialAddr:         p.KubeDialAddr,
		authClient:           p.Client,
		reverseTunnelServer:  p.ReverseTunnelServer,
		server:               p.Server,
		emitter:              p.Client,
		sconn:                conn,
		channel:              channel,
		requestCh:            requests,
		log:                  p.log,
		authServers:          p.LocalAuthAddresses,
		proxySigner:          p.PROXYSigner,
		forwardClientAddress: true,
	}
}

// runtimeConfig contains configurations dynamically set and updated
// during runtime.
type runtimeConfig struct {
	netconfig                     types.ClusterNetworkingConfig
	tlsRoutingConnUpgradeRequired bool
	mu                            sync.RWMutex
}

var initialTunnelStrategy = &types.TunnelStrategyV1{Strategy: &types.TunnelStrategyV1_ProxyPeering{
	ProxyPeering: &types.ProxyPeeringTunnelStrategy{AgentConnectionCount: math.MaxInt}}}

func newRuntimeConfig() *runtimeConfig {
	rc := &runtimeConfig{
		netconfig: types.DefaultClusterNetworkingConfig(),
	}
	rc.netconfig.SetTunnelStrategy(initialTunnelStrategy)
	return rc
}

func (c *runtimeConfig) getTunnelStrategyType() types.TunnelStrategyType {
	t, err := c.netconfig.GetTunnelStrategyType()
	if err != nil {
		return types.AgentMesh
	}
	return t
}

func (c *runtimeConfig) copy() types.ClusterNetworkingConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.netconfig.Clone()
}

func (c *runtimeConfig) update(netconfig types.ClusterNetworkingConfig, tlsRoutingConnUpgradeRequired bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.netconfig = netconfig
	c.tlsRoutingConnUpgradeRequired = tlsRoutingConnUpgradeRequired
}

func (c *runtimeConfig) connectionCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	strategy := c.netconfig.GetProxyPeeringTunnelStrategy()
	if strategy == nil {
		return defaultAgentConnectionCount
	}
	return int(strategy.AgentConnectionCount)
}

// reportConnectedProxies returns true if the connected proxies should be reported.
func (c *runtimeConfig) reportConnectedProxies() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.getTunnelStrategyType() == types.ProxyPeering
}

// restrictConnectionCount returns true if the number of agents should be restricted.
func (c *runtimeConfig) restrictConnectionCount() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.getTunnelStrategyType() == types.ProxyPeering
}

// useReverseTunnelV2Unsafe returns true if reverse tunnel should be used.
func (c *runtimeConfig) useReverseTunnelV2Unsafe() bool {
	return c.getTunnelStrategyType() == types.ProxyPeering
}

func (c *runtimeConfig) keepaliveInterval() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.netconfig.GetKeepAliveInterval()
}

// alpnDialerConfig creates a config for ALPN dialer.
func (c *runtimeConfig) alpnDialerConfig(getClusterCAs client.GetClusterCAsFunc) client.ALPNDialerConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	protocols := []alpncommon.Protocol{alpncommon.ProtocolReverseTunnel}
	if c.useReverseTunnelV2Unsafe() {
		protocols = []alpncommon.Protocol{alpncommon.ProtocolReverseTunnelV2, alpncommon.ProtocolReverseTunnel}
	}

	return client.ALPNDialerConfig{
		TLSConfig: &tls.Config{
			NextProtos:         alpncommon.ProtocolsToString(protocols),
			InsecureSkipVerify: lib.IsInsecureDevMode(),
		},
		KeepAlivePeriod:         c.netconfig.GetKeepAliveInterval(),
		ALPNConnUpgradeRequired: c.tlsRoutingConnUpgradeRequired,
		GetClusterCAs:           getClusterCAs,
	}
}

// useALPNRouting returns true agents should connect using alpn routing.
func (c *runtimeConfig) useALPNRouting() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.netconfig.GetProxyListenerMode() == types.ProxyListenerMode_Multiplex
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

func tlsRoutingUpgradeRequiredCache(
	ctx context.Context, ttl time.Duration, resolver reversetunnelclient.Resolver, clock clockwork.Clock, log logrus.FieldLogger,
) func() bool {
	var (
		tlsRoutingUpgradeRequired bool
		nextRun                   time.Time
	)
	return func() bool {
		if clock.Now().Before(nextRun) {
			return tlsRoutingUpgradeRequired
		}
		addr, _, err := resolver(ctx)
		if err != nil {
			log.WithError(err).Warn("Failed to resolve addr.")
			return tlsRoutingUpgradeRequired
		}

		tlsRoutingUpgradeRequired = client.IsALPNConnUpgradeRequired(addr.Addr, lib.IsInsecureDevMode())
		nextRun = clock.Now().Add(ttl)

		return tlsRoutingUpgradeRequired
	}
}
