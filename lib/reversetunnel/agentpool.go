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
	ctx          context.Context
	cancel       context.CancelFunc
	// spawnLimiter limits agent spawn rate
	spawnLimiter utils.Retry

	mu     sync.Mutex
	agents map[track.Key][]*Agent
}

// AgentPoolConfig holds configuration parameters for the agent pool
type AgentPoolConfig struct {
	// Client is client to the auth server this agent connects to receive
	// a list of pools
	Client auth.ClientI
	// AccessPoint is a lightweight access point
	// that can optionally cache some values
	AccessPoint auth.AccessPoint
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
	// Server is a SSH server that can handle a connection (perform a handshake
	// then process). Only set with the agent is running within a node.
	Server ServerHandler
	// Component is the Teleport component this agent pool is running in. It can
	// either be proxy (trusted clusters) or node (dial back).
	Component string
	// ReverseTunnelServer holds all reverse tunnel connections.
	ReverseTunnelServer Server
	// ProxyAddr points to the address of the ssh proxy
	ProxyAddr string
	// Cluster is a cluster name of the proxy.
	Cluster string
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
	return nil
}

// NewAgentPool returns new isntance of the agent pool
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
	proxyAddr, err := utils.ParseAddr(cfg.ProxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(ctx)
	pool := &AgentPool{
		agents:       make(map[track.Key][]*Agent),
		proxyTracker: track.New(ctx, track.Config{}),
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
	pool.proxyTracker.Start(track.Key{Cluster: cfg.Cluster, Addr: *proxyAddr})
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
	m.cancel()
}

// Wait returns when agent pool is closed
func (m *AgentPool) Wait() {
	<-m.ctx.Done()
}

func (m *AgentPool) processSeekEvents() {
	limiter := m.spawnLimiter.Clone()
	for {
		select {
		case <-m.ctx.Done():
			m.log.Debugf("Halting seek event processing (pool closing)")
			return
		case lease := <-m.proxyTracker.Acquire():
			m.log.Debugf("Seeking: %+v.", lease.Key())
			m.withLock(func() {
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
	for key, agents := range m.agents {
		m.agents[key] = filterAndClose(agents, func(*Agent) bool { return true })
		if len(m.agents[key]) == 0 {
			delete(m.agents, key)
		}
	}
}

func filterAndClose(agents []*Agent, matchAgent matchAgentFn) []*Agent {
	var filtered []*Agent
	for i := range agents {
		agent := agents[i]
		if matchAgent(agent) {
			agent.Debugf("Pool is closing agent.")
			agent.Close()
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

func (m *AgentPool) addAgent(lease track.Lease) error {
	key := lease.Key().(track.Key)
	agent, err := NewAgent(AgentConfig{
		Addr:                key.Addr,
		ClusterName:         m.cfg.Cluster,
		Username:            m.cfg.HostUUID,
		Signer:              m.cfg.HostSigner,
		Client:              m.cfg.Client,
		AccessPoint:         m.cfg.AccessPoint,
		Context:             m.ctx,
		KubeDialAddr:        m.cfg.KubeDialAddr,
		Server:              m.cfg.Server,
		ReverseTunnelServer: m.cfg.ReverseTunnelServer,
		LocalClusterName:    m.cfg.LocalCluster,
		Tracker:             m.proxyTracker,
		Lease:               lease,
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
	m.agents[key] = append(m.agents[key], agent)
	return nil
}

// Counts returns a count of the number of proxies a outbound tunnel is
// connected to. Used in tests to determine if a proxy has been found and/or
// removed.
func (m *AgentPool) Count() int {
	var out int
	m.withLock(func() {
		for _, agents := range m.agents {
			for _, agent := range agents {
				if agent.getState() == agentStateConnected {
					out++
				}
			}
		}
	})

	return out
}

// removeDisconnected removes disconnected agents from the list of agents.
// This function should be called under a lock.
func (m *AgentPool) removeDisconnected() {
	for agentKey, agentSlice := range m.agents {
		// Filter and close all disconnected agents.
		validAgents := filterAndClose(agentSlice, func(agent *Agent) bool {
			return agent.getState() == agentStateDisconnected
		})

		// Update (or delete) agent key with filter applied.
		if len(validAgents) > 0 {
			m.agents[agentKey] = validAgents
		} else {
			delete(m.agents, agentKey)
		}
	}
}
