// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package reversetunnelv3

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/reversetunnel/track"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
)

const (
	// poolMaxBackoff is the ceiling for exponential backoff between
	// connection attempts.
	poolMaxBackoff = 8 * time.Second
)

// AgentPoolConfig holds configuration for [AgentPool].
type AgentPoolConfig struct {
	Log *slog.Logger

	// HostID is the stable UUID of this Teleport instance.
	HostID string

	// ClusterName is the name of the cluster this agent belongs to.
	ClusterName string

	// Version is the Teleport version string of this agent.
	Version string

	// Scope is the resource scope encoded in this agent's certificate.
	Scope string

	// Handlers maps each tunnel service type to the local ServerHandler that
	// accepts inbound dial connections for that service. All services in this
	// map will be advertised in AgentHello and validated against the cert.
	Handlers map[types.TunnelType]ServerHandler

	// GetCertificate returns the current Instance TLS certificate for the agent.
	GetCertificate func() (*tls.Certificate, error)

	// GetPool returns the CA certificate pool for verifying proxies.
	GetPool func() (*x509.CertPool, error)

	Ciphersuites []uint16

	// Resolver resolves the address of the proxy to connect to.
	Resolver reversetunnelclient.Resolver

	// Cluster is the name of the cluster whose proxies we connect to. Used to
	// configure the tracker.
	Cluster string
}

// CheckAndSetDefaults validates required fields and sets defaults.
func (cfg *AgentPoolConfig) CheckAndSetDefaults() error {
	if cfg.Log == nil {
		return trace.BadParameter("missing Log")
	}
	if cfg.HostID == "" {
		return trace.BadParameter("missing HostID")
	}
	if cfg.ClusterName == "" {
		return trace.BadParameter("missing ClusterName")
	}
	if len(cfg.Handlers) == 0 {
		return trace.BadParameter("Handlers must not be empty")
	}
	if cfg.GetCertificate == nil {
		return trace.BadParameter("missing GetCertificate")
	}
	if cfg.GetPool == nil {
		return trace.BadParameter("missing GetPool")
	}
	if cfg.Resolver == nil {
		return trace.BadParameter("missing Resolver")
	}
	if cfg.Cluster == "" {
		cfg.Cluster = cfg.ClusterName
	}
	return nil
}

// AgentPool manages a set of reverse tunnel agent connections to one or more
// proxies. Each connection is a single yamux session over TLS that advertises
// all registered service types simultaneously.
//
// The pool's run loop:
//  1. Acquires a tracker lease (polls until one becomes available).
//  2. Resolves the current proxy address.
//  3. Calls newAgent, which blocks until the handshake succeeds or fails.
//  4. On success, stores the live agent and spawns a goroutine that removes it
//     from the store when agent.Done() closes.
//  5. Applies backoff and loops.
type AgentPool struct {
	cfg     AgentPoolConfig
	tracker *track.Tracker
	active  *agentStore

	// backoff limits the rate at which new connections are attempted.
	backoff retryutils.Retry

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// serviceList caches the sorted list of service types for AgentHello.
	serviceList []types.TunnelType
}

// NewAgentPool returns a new [AgentPool]. Call [AgentPool.Start] to begin
// connecting to proxies.
func NewAgentPool(ctx context.Context, cfg AgentPoolConfig) (*AgentPool, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	tracker, err := track.New(track.Config{ClusterName: cfg.Cluster})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	backoff, err := retryutils.NewLinear(retryutils.LinearConfig{
		Step:      time.Second,
		Max:       poolMaxBackoff,
		Jitter:    retryutils.DefaultJitter,
		AutoReset: 4,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	services := make([]types.TunnelType, 0, len(cfg.Handlers))
	for svc := range cfg.Handlers {
		services = append(services, svc)
	}

	poolCtx, cancel := context.WithCancel(ctx)
	return &AgentPool{
		cfg:         cfg,
		tracker:     tracker,
		active:      newAgentStore(),
		backoff:     backoff,
		ctx:         poolCtx,
		cancel:      cancel,
		serviceList: services,
	}, nil
}

// Start begins the pool's connection loop in the background.
func (p *AgentPool) Start() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.run()
	}()
}

// Stop cancels the pool context and waits for all goroutines to exit.
func (p *AgentPool) Stop() {
	p.cancel()
	p.wg.Wait()
}

// Count returns the number of currently active agents.
func (p *AgentPool) Count() int { return p.active.len() }

// ProxyIDs returns the proxy IDs of all currently active agents.
func (p *AgentPool) ProxyIDs() []string { return p.active.proxyIDs() }

// SetConnectionCount forwards connection-count updates to the tracker, used
// in proxy peering mode to cap the number of concurrent proxy connections.
// A value of 0 means full-mesh (connect to every known proxy).
func (p *AgentPool) SetConnectionCount(n int) {
	p.tracker.SetConnectionCount(n)
}

// run is the pool's main loop.
func (p *AgentPool) run() {
	for {
		if p.ctx.Err() != nil {
			return
		}

		agent, err := p.connectOnce(p.ctx)
		if err != nil {
			if p.ctx.Err() != nil {
				return
			}
			level := slog.LevelWarn
			if trace.IsAlreadyExists(err) {
				level = slog.LevelDebug
			}
			p.cfg.Log.Log(p.ctx, level, "Failed to establish reverse tunnel", "error", err)
		} else {
			p.active.add(agent)
			p.wg.Add(1)
			go func() {
				defer p.wg.Done()
				// Wait for the agent to terminate, then remove it.
				select {
				case <-agent.Done():
				case <-p.ctx.Done():
					_ = agent.Stop()
					<-agent.Done()
				}
				p.active.remove(agent)
			}()
		}

		select {
		case <-p.ctx.Done():
			return
		case <-p.backoff.After():
			p.backoff.Inc()
		}
	}
}

// connectOnce acquires a lease from the tracker, resolves the proxy address,
// and calls newAgent (which blocks until the handshake completes). On success
// the returned Agent is live. On any failure the lease has already been
// released by newAgent.
func (p *AgentPool) connectOnce(ctx context.Context) (Agent, error) {
	lease, err := p.waitForLease(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// lease is now held; newAgent is responsible for releasing it.

	addr, _, err := p.cfg.Resolver(ctx)
	if err != nil {
		lease.Release()
		return nil, trace.Wrap(err)
	}

	services := make([]types.TunnelType, len(p.serviceList))
	copy(services, p.serviceList)

	cfg := agentConfig{
		addr:        addr.Addr,
		hostID:      p.cfg.HostID,
		clusterName: p.cfg.ClusterName,
		version:     p.cfg.Version,
		scope:       p.cfg.Scope,
		services:    services,
		handlers:    p.cfg.Handlers,
		tracker:     p.tracker,
		lease:       lease,
	}

	agent, err := newAgent(ctx, cfg, p.cfg.Log, p.cfg.GetCertificate, p.cfg.GetPool, p.cfg.Ciphersuites)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return agent, nil
}

// waitForLease blocks until the tracker grants a lease or ctx is cancelled.
func (p *AgentPool) waitForLease(ctx context.Context) (*track.Lease, error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if lease := p.tracker.TryAcquire(); lease != nil {
			return lease, nil
		}
		select {
		case <-ctx.Done():
			return nil, trace.Wrap(ctx.Err())
		case <-ticker.C:
		}
	}
}
