/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package reversetunnel

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/defaults"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/authclient"
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
	// remotePingCacheTTL sets the time between calls to webclient.Find.
	remotePingCacheTTL = time.Second * 5
)

// ServerHandler implements an interface which can handle a connection
// (perform a handshake then process). This is needed because importing
// lib/srv in lib/reversetunnel causes a circular import.
type ServerHandler interface {
	// HandleConnection performs a handshake then process the connection.
	HandleConnection(conn net.Conn)
}

type newAgentFunc func(context.Context, *track.Tracker, *track.Lease) (Agent, error)

// AgentPool manages a pool of reverse tunnel agents.
type AgentPool struct {
	AgentPoolConfig
	active  *agentStore
	tracker *track.Tracker

	// runtimeConfig contains dynamic configuration values.
	runtimeConfig *agentPoolRuntimeConfig

	// events receives agent state change events.
	events chan Agent

	// newAgentFunc is used during testing to mock new agents.
	newAgentFunc newAgentFunc

	// wg waits for the pool and all agents to complete.
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	// backoff limits the rate at which new agents are created.
	backoff retryutils.Retry
	logger  *slog.Logger
}

// AgentPoolConfig holds configuration parameters for the agent pool
type AgentPoolConfig struct {
	// Client is client to the auth server this agent connects to receive
	// a list of pools
	Client authclient.ClientI
	// AccessPoint is a lightweight access point
	// that can optionally cache some values
	AccessPoint authclient.AccessCache
	// AuthMethods contains SSH credentials that this pool connects as.
	AuthMethods []ssh.AuthMethod
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
	ReverseTunnelServer reversetunnelclient.Server
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
	if len(cfg.AuthMethods) == 0 {
		return trace.BadParameter("missing 'AuthMethods' parameter")
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
}

// NewAgentPool returns new instance of the agent pool.
func NewAgentPool(ctx context.Context, config AgentPoolConfig) (*AgentPool, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		Step:      time.Second,
		Max:       maxBackoff,
		Jitter:    retryutils.DefaultJitter,
		AutoReset: 4,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pool := &AgentPool{
		AgentPoolConfig: config,
		active:          newAgentStore(),
		events:          make(chan Agent),
		backoff:         retry,
		logger: slog.With(
			teleport.ComponentKey, teleport.ComponentReverseTunnelAgent,
			"target_cluster", config.Cluster,
			"local_cluster", config.LocalCluster,
		),
		runtimeConfig: newAgentPoolRuntimeConfig(),
	}

	pool.runtimeConfig.isRemoteCluster = pool.IsRemoteCluster
	pool.newAgentFunc = pool.newAgent

	pool.ctx, pool.cancel = context.WithCancel(ctx)
	pool.tracker, err = track.New(track.Config{ClusterName: pool.Cluster})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return pool, nil
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
	p.logger.DebugContext(p.ctx, "Updating connected proxies", "proxies", proxies)
	p.AgentPoolConfig.ConnectedProxyGetter.setProxyIDs(proxies)
}

// Count is the number connected agents.
func (p *AgentPool) Count() int {
	return p.active.len()
}

// Start starts the agent pool in the background.
func (p *AgentPool) Start() error {
	p.logger.DebugContext(p.ctx, "Starting agent pool",
		"host_uuid", p.HostUUID,
		"cluster", p.Cluster,
	)

	p.wg.Add(1)
	go func() {
		if err := p.run(); err != nil {
			p.logger.WarnContext(p.ctx, "Agent pool exited", "error", err)
		}

		p.cancel()
		p.wg.Done()
	}()
	return nil
}

// run connects agents until the agent pool context is done.
func (p *AgentPool) run() error {
	for {
		agent, err := p.connectAgent(p.ctx, p.events)
		if err != nil {
			if p.ctx.Err() != nil {
				return nil
			} else if isProxyAlreadyClaimed(err) {
				// "proxy already claimed" is a fairly benign error, we should not
				// spam the log with stack traces for it
				p.logger.DebugContext(p.ctx, "Failed to connect agent", "error", err)
			} else {
				p.logger.DebugContext(p.ctx, "Failed to connect agent", "error", err)
			}
		} else {
			p.wg.Add(1)
			p.active.add(agent)
			p.updateConnectedProxies()
		}

		err = p.waitForBackoff(p.ctx, p.events)
		if p.ctx.Err() != nil {
			return nil
		} else if err != nil {
			p.logger.DebugContext(p.ctx, "Failed to wait for backoff", "error", err)
		}
	}
}

// connectAgent connects a new agent and processes any agent events blocking until a
// new agent is connected or an error occurs.
func (p *AgentPool) connectAgent(ctx context.Context, events <-chan Agent) (Agent, error) {
	lease, err := p.waitForLease(ctx, events)
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

func (p *AgentPool) updateRuntimeConfig(ctx context.Context) error {
	netConfig, err := p.AccessPoint.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	p.runtimeConfig.update(ctx, netConfig, p.Resolver)

	restrictConnectionCount := p.runtimeConfig.restrictConnectionCount()
	connectionCount := p.runtimeConfig.getConnectionCount()

	p.logger.DebugContext(ctx, "Runtime config updated",
		"restrict_connection_count", restrictConnectionCount,
		"connection_count", connectionCount,
	)

	if restrictConnectionCount {
		p.tracker.SetConnectionCount(connectionCount)
	} else {
		p.tracker.SetConnectionCount(0)
	}

	return nil
}

// processEvents handles all events in the queue.
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

	return nil
}

// waitForLease processes events while waiting to acquire a lease.
func (p *AgentPool) waitForLease(ctx context.Context, events <-chan Agent) (*track.Lease, error) {
	t := time.NewTicker(time.Second)
	defer t.Stop()

	for ctx.Err() == nil {
		if lease := p.tracker.TryAcquire(); lease != nil {
			return lease, nil
		}

		select {
		case <-ctx.Done():
		case <-t.C:
		case agent := <-events:
			p.handleEvent(ctx, agent)
		}
	}

	return nil, trace.Wrap(ctx.Err())
}

// waitForBackoff processes events while waiting for the backoff.
func (p *AgentPool) waitForBackoff(ctx context.Context, events <-chan Agent) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-p.backoff.After():
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
	p.logger.DebugContext(ctx, "Processed agent event", "active_agent_count", p.active.len())
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
func (p *AgentPool) newAgent(ctx context.Context, tracker *track.Tracker, lease *track.Lease) (Agent, error) {
	addr, _, err := p.Resolver(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = p.runtimeConfig.updateRemote(ctx, addr)
	if err != nil {
		p.logger.DebugContext(ctx, "Failed to update remote config", "error", err)
	}

	options := []proxy.DialerOptionFunc{proxy.WithInsecureSkipTLSVerify(lib.IsInsecureDevMode())}
	if p.runtimeConfig.useALPNRouting() {
		options = append(options, proxy.WithALPNDialer(p.runtimeConfig.alpnDialerConfig(p.getClusterCAs)))
	}

	dialer := &agentDialer{
		client:      p.Client,
		fips:        p.FIPS,
		authMethods: p.AuthMethods,
		options:     options,
		username:    p.HostUUID,
		logger:      p.logger,
		isClaimed:   p.tracker.IsClaimed,
	}

	agent, err := newAgent(agentConfig{
		addr:               *addr,
		keepAlive:          p.runtimeConfig.keepAliveInterval,
		sshDialer:          dialer,
		transportHandler:   p,
		versionGetter:      p,
		tracker:            tracker,
		lease:              lease,
		clock:              p.Clock,
		logger:             p.logger,
		localAuthAddresses: p.LocalAuthAddresses,
		proxySigner:        p.PROXYSigner,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	agent.stateCallback = p.getStateCallback(agent)
	return agent, nil
}

func (p *AgentPool) getClusterCAs(ctx context.Context) (*x509.CertPool, error) {
	clusterCAs, _, err := authclient.ClientCertPool(ctx, p.AccessPoint, p.Cluster, types.HostCA)
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

// handleTransport runs a new teleport-transport channel.
func (p *AgentPool) handleTransport(ctx context.Context, channel ssh.Channel, requests <-chan *ssh.Request, conn sshutils.Conn) {
	if !p.IsRemoteCluster {
		p.handleLocalTransport(ctx, channel, requests, conn)
		return
	}

	t := &transport{
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
		logger:               p.logger,
		authServers:          p.LocalAuthAddresses,
		proxySigner:          p.PROXYSigner,
		forwardClientAddress: true,
	}

	// If the AgentPool is being used for Proxy to Proxy communication between two clusters, then
	// we check if the reverse tunnel server is capable of tracking user connections. This allows
	// the leaf proxy to track sessions that are initiated via the root cluster. Without providing
	// the user tracker the leaf cluster metrics will be incorrect and graceful shutdown will not
	// wait for user sessions to be terminated prior to proceeding with the shutdown operation.
	if p.ReverseTunnelServer != nil {
		t.trackUserConnection = p.ReverseTunnelServer.TrackUserConnection
	}

	t.start()
}

func (p *AgentPool) handleLocalTransport(ctx context.Context, channel ssh.Channel, reqC <-chan *ssh.Request, sconn sshutils.Conn) {
	defer channel.Close()
	go io.Copy(io.Discard, channel.Stderr())

	// the only valid teleport-transport-dial request here is to reach the local service
	var req *ssh.Request
	select {
	case <-ctx.Done():
		go ssh.DiscardRequests(reqC)
		return
	case <-time.After(apidefaults.DefaultIOTimeout):
		go ssh.DiscardRequests(reqC)
		p.logger.WarnContext(ctx, "Timed out waiting for transport dial request")
		return
	case r, ok := <-reqC:
		if !ok {
			return
		}
		go ssh.DiscardRequests(reqC)
		req = r
	}

	// sconn should never be nil, but it's sourced from the agent state and
	// starts as nil, and the original transport code checked against it
	if sconn == nil || p.Server == nil {
		p.logger.ErrorContext(ctx, "Missing client or server (this is a bug)")
		fmt.Fprintf(channel.Stderr(), "internal server error")
		req.Reply(false, nil)
		return
	}

	if err := req.Reply(true, nil); err != nil {
		p.logger.ErrorContext(ctx, "Failed to respond to dial request", "error", err)
		return
	}

	var conn net.Conn = sshutils.NewChConn(sconn, channel)

	dialReq := parseDialReq(req.Payload)
	switch dialReq.Address {
	case reversetunnelclient.LocalNode, reversetunnelclient.LocalKubernetes, reversetunnelclient.LocalWindowsDesktop:
	default:
		p.logger.WarnContext(ctx, "Received dial request for unexpected address, routing to the local service anyway",
			"dial_addr", dialReq.Address,
		)
	}
	if src, err := utils.ParseAddr(dialReq.ClientSrcAddr); err == nil {
		conn = utils.NewConnWithSrcAddr(conn, getTCPAddr(src))
	}

	p.Server.HandleConnection(conn)
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
	// isRemoteCluster forces the agent pool to connect to all proxies
	// regardless of the configured tunnel strategy.
	isRemoteCluster bool
	// tlsRoutingConnUpgradeRequired indicates that ALPN connection upgrades
	// are required for making TLS routing requests.
	tlsRoutingConnUpgradeRequired bool

	// remoteTLSRoutingEnabled caches a remote clusters tls routing setting. This helps prevent
	// proxy endpoint stagnation where an even numbers of proxies are hidden behind a round robin
	// load balancer. For instance in a situation where there are two proxies [A, B] due to
	// the agent pools sequential webclient.Find and ssh dial, the Find call will always reach
	// Proxy A and the ssh dial call will always be forwarded to Proxy B.
	remoteTLSRoutingEnabled bool
	// lastRemotePing is the time of the last ping attempt.
	lastRemotePing *time.Time

	mu             sync.RWMutex
	updateRemoteMu sync.Mutex
	clock          clockwork.Clock
}

func newAgentPoolRuntimeConfig() *agentPoolRuntimeConfig {
	return &agentPoolRuntimeConfig{
		tunnelStrategyType: types.AgentMesh,
		connectionCount:    defaultAgentConnectionCount,
		proxyListenerMode:  types.ProxyListenerMode_Separate,
		keepAliveInterval:  defaults.KeepAliveInterval(),
		clock:              clockwork.NewRealClock(),
	}
}

// reportConnectedProxies returns true if the connected proxies should be reported.
func (c *agentPoolRuntimeConfig) reportConnectedProxies() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.isRemoteCluster {
		return false
	}
	return c.tunnelStrategyType == types.ProxyPeering
}

// reportConnectedProxies returns true if the number of agents should be restricted.
func (c *agentPoolRuntimeConfig) restrictConnectionCount() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.isRemoteCluster {
		return false
	}
	return c.tunnelStrategyType == types.ProxyPeering
}

func (c *agentPoolRuntimeConfig) getConnectionCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connectionCount
}

// useReverseTunnelV2Locked returns true if reverse tunnel should be used.
func (c *agentPoolRuntimeConfig) useReverseTunnelV2Locked() bool {
	if c.isRemoteCluster {
		return false
	}
	return c.tunnelStrategyType == types.ProxyPeering
}

// alpnDialerConfig creates a config for ALPN dialer.
func (c *agentPoolRuntimeConfig) alpnDialerConfig(getClusterCAs client.GetClusterCAsFunc) client.ALPNDialerConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	protocols := []alpncommon.Protocol{alpncommon.ProtocolReverseTunnel}
	if c.useReverseTunnelV2Locked() {
		protocols = []alpncommon.Protocol{alpncommon.ProtocolReverseTunnelV2, alpncommon.ProtocolReverseTunnel}
	}

	return client.ALPNDialerConfig{
		TLSConfig: &tls.Config{
			NextProtos:         alpncommon.ProtocolsToString(protocols),
			InsecureSkipVerify: lib.IsInsecureDevMode(),
		},
		KeepAlivePeriod:         c.keepAliveInterval,
		ALPNConnUpgradeRequired: c.tlsRoutingConnUpgradeRequired,
		GetClusterCAs:           getClusterCAs,
	}
}

// useALPNRouting returns true agents should connect using alpn routing.
func (c *agentPoolRuntimeConfig) useALPNRouting() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.isRemoteCluster {
		return c.remoteTLSRoutingEnabled
	}

	return c.proxyListenerMode == types.ProxyListenerMode_Multiplex
}

func (c *agentPoolRuntimeConfig) updateRemote(ctx context.Context, addr *utils.NetAddr) error {
	c.updateRemoteMu.Lock()
	defer c.updateRemoteMu.Unlock()

	c.mu.RLock()
	if !c.isRemoteCluster {
		c.mu.RUnlock()
		return nil
	}

	if c.lastRemotePing != nil && c.clock.Since(*c.lastRemotePing) < remotePingCacheTTL {
		c.mu.RUnlock()
		return nil
	}

	c.mu.RUnlock()

	ctx, cancel := context.WithTimeout(ctx, defaults.DefaultIOTimeout)
	defer cancel()

	tlsRoutingEnabled := false

	ping, err := webclient.Find(&webclient.Config{
		Context:   ctx,
		ProxyAddr: addr.Addr,
		Insecure:  lib.IsInsecureDevMode(),
	})
	if err != nil {
		// If TLS Routing is disabled the address is the proxy reverse tunnel
		// address the ping call will always fail with tls.RecordHeaderError.
		if ok := errors.As(err, &tls.RecordHeaderError{}); !ok {
			return trace.Wrap(err)
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

	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.clock.Now()
	c.lastRemotePing = &now

	c.remoteTLSRoutingEnabled = tlsRoutingEnabled
	if c.remoteTLSRoutingEnabled {
		c.tlsRoutingConnUpgradeRequired = client.IsALPNConnUpgradeRequired(ctx, addr.Addr, lib.IsInsecureDevMode())
		slog.DebugContext(ctx, "ALPN upgrade required for remote cluster",
			"remote_addr", addr.Addr,
			"conn_upgrade_required", c.tlsRoutingConnUpgradeRequired,
		)
	}
	return nil
}

func (c *agentPoolRuntimeConfig) update(ctx context.Context, netConfig types.ClusterNetworkingConfig, resolver reversetunnelclient.Resolver) {
	c.mu.Lock()
	defer c.mu.Unlock()

	oldProxyListenerMode := c.proxyListenerMode
	c.keepAliveInterval = netConfig.GetKeepAliveInterval()
	c.proxyListenerMode = netConfig.GetProxyListenerMode()

	// Fallback to agent mesh strategy if there is an error.
	strategyType, err := netConfig.GetTunnelStrategyType()
	if err != nil {
		c.tunnelStrategyType = types.AgentMesh
		return
	}

	c.tunnelStrategyType = strategyType
	if c.tunnelStrategyType == types.ProxyPeering {
		strategy := netConfig.GetProxyPeeringTunnelStrategy()
		c.connectionCount = int(strategy.AgentConnectionCount)
	}
	if c.connectionCount <= 0 {
		c.connectionCount = defaultAgentConnectionCount
	}

	if c.proxyListenerMode == types.ProxyListenerMode_Multiplex && oldProxyListenerMode != c.proxyListenerMode {
		addr, _, err := resolver(ctx)
		if err == nil {
			c.tlsRoutingConnUpgradeRequired = client.IsALPNConnUpgradeRequired(ctx, addr.Addr, lib.IsInsecureDevMode())
		} else {
			slog.WarnContext(ctx, "Failed to resolve addr", "error", err)
		}
	}
}

// Make sure ServerHandlerToListener implements both interfaces.
var (
	_ = net.Listener(ServerHandlerToListener{})
	_ = ServerHandler(ServerHandlerToListener{})
)

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
