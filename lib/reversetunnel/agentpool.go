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

package reversetunnel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

var (
	tunnelStats = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tunnels",
			Help: "Number of tunnels per state",
		},
		[]string{"cluster", "state"},
	)
)

// AgentPool manages the pool of outbound reverse tunnel agents.
// The agent pool watches the reverse tunnel entries created by the admin and
// connects/disconnects to added/deleted tunnels.
type AgentPool struct {
	sync.Mutex
	*log.Entry
	cfg        AgentPoolConfig
	agents     map[agentKey][]*Agent
	ctx        context.Context
	cancel     context.CancelFunc
	discoveryC chan *discoveryRequest
	// lastReport is the last time the agent has reported the stats
	lastReport time.Time
}

// AgentPoolConfig holds configuration parameters for the agent pool
type AgentPoolConfig struct {
	// Client is client to the auth server this agent connects to receive
	// a list of pools
	Client auth.ClientI
	// AccessPoint is a lightweight access point
	// that can optionally cache some values
	AccessPoint auth.AccessPoint
	// HostSigners is a list of host signers this agent presents itself as
	HostSigners []ssh.Signer
	// HostUUID is a unique ID of this host
	HostUUID string
	// Context is an optional context
	Context context.Context
	// Cluster is a cluster name
	Cluster string
	// Clock is a clock used to get time, if not set,
	// system clock is used
	Clock clockwork.Clock
}

// CheckAndSetDefaults checks and sets defaults
func (cfg *AgentPoolConfig) CheckAndSetDefaults() error {
	if cfg.Client == nil {
		return trace.BadParameter("missing 'Client' parameter")
	}
	if cfg.AccessPoint == nil {
		return trace.BadParameter("missing 'AccessPoint' parameter")
	}
	if len(cfg.HostSigners) == 0 {
		return trace.BadParameter("missing 'HostSigners' parameter")
	}
	if len(cfg.HostUUID) == 0 {
		return trace.BadParameter("missing 'HostUUID' parameter")
	}
	if cfg.Context == nil {
		cfg.Context = context.TODO()
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	return nil
}

// NewAgentPool returns new isntance of the agent pool
func NewAgentPool(cfg AgentPoolConfig) (*AgentPool, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(cfg.Context)
	pool := &AgentPool{
		agents:     make(map[agentKey][]*Agent),
		cfg:        cfg,
		ctx:        ctx,
		cancel:     cancel,
		discoveryC: make(chan *discoveryRequest),
	}
	pool.Entry = log.WithFields(log.Fields{
		trace.Component: teleport.ComponentReverseTunnelAgent,
		trace.ComponentFields: log.Fields{
			"cluster": cfg.Cluster,
		},
	})
	return pool, nil
}

// Start starts the agent pool
func (m *AgentPool) Start() error {
	go m.pollAndSyncAgents()
	go m.processDiscoveryRequests()
	return nil
}

// Stop stops the agent pool
func (m *AgentPool) Stop() {
	m.cancel()
}

// Wait returns when agent pool is closed
func (m *AgentPool) Wait() error {
	select {
	case <-m.ctx.Done():
		break
	}
	return nil
}

func (m *AgentPool) processDiscoveryRequests() {
	for {
		select {
		case <-m.ctx.Done():
			m.Debugf("closing")
			return
		case req := <-m.discoveryC:
			if req == nil {
				m.Debugf("channel closed")
				return
			}
			m.tryDiscover(*req)
		}
	}
}

func foundInOneOf(proxy services.Server, agents []*Agent) bool {
	for _, agent := range agents {
		if agent.connectedTo(proxy) {
			return true
		}
	}
	return false
}

func (m *AgentPool) tryDiscover(req discoveryRequest) {
	proxies := Proxies(req.Proxies)
	m.Lock()
	defer m.Unlock()

	matchKey := req.key()

	// if one of the proxies have been discovered or connected to
	// remove proxy from discovery request
	var filtered Proxies
	agents := m.agents[matchKey]
	for i := range proxies {
		proxy := proxies[i]
		if !foundInOneOf(proxy, agents) {
			filtered = append(filtered, proxy)
		}
	}
	m.Debugf("tryDiscover original(%v) -> filtered(%v)", proxies, filtered)
	// nothing to do
	if len(filtered) == 0 {
		return
	}
	// close agents that are discovering proxies that are somehow
	// different from discovery request
	var foundAgent bool
	m.closeAgentsIf(&matchKey, func(agent *Agent) bool {
		if agent.getState() != agentStateDiscovering {
			return false
		}
		if filtered.Equal(agent.DiscoverProxies) {
			foundAgent = true
			agent.Debugf("agent is already discovering the same proxies as requested in %v", filtered)
			return false
		}
		agent.Debugf("is obsolete, going to close", agent.getState(), agent.DiscoverProxies)
		return true
	})

	// if we haven't found any discovery agent
	if !foundAgent {
		m.addAgent(req.key(), req.Proxies)
	}
}

// FetchAndSyncAgents executes one time fetch and sync request
// (used in tests instead of polling)
func (m *AgentPool) FetchAndSyncAgents() error {
	tunnels, err := m.cfg.AccessPoint.GetReverseTunnels()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := m.syncAgents(tunnels); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (m *AgentPool) withLock(f func()) {
	m.Lock()
	defer m.Unlock()
	f()
}

type matchAgentFn func(a *Agent) bool

func (m *AgentPool) closeAgentsIf(matchKey *agentKey, matchAgent matchAgentFn) {
	if matchKey != nil {
		m.agents[*matchKey] = filterAndClose(m.agents[*matchKey], matchAgent)
		return
	}
	for key, agents := range m.agents {
		m.agents[key] = filterAndClose(agents, matchAgent)
	}
}

func filterAndClose(agents []*Agent, matchAgent matchAgentFn) []*Agent {
	var filtered []*Agent
	for i := range agents {
		agent := agents[i]
		if matchAgent(agent) {
			agent.Debugf("pool is closing agent")
			agent.Close()
		} else {
			filtered = append(filtered, agent)
		}
	}
	return filtered
}

func (m *AgentPool) closeAgents(matchKey *agentKey) {
	m.closeAgentsIf(matchKey, func(*Agent) bool {
		// close all agents matching the matchKey
		return true
	})
}

func (m *AgentPool) pollAndSyncAgents() {
	ticker := time.NewTicker(defaults.ReverseTunnelAgentHeartbeatPeriod)
	defer ticker.Stop()
	m.FetchAndSyncAgents()
	for {
		select {
		case <-m.ctx.Done():
			m.withLock(func() {
				m.closeAgents(nil)
			})
			m.Debugf("closing")
			return
		case <-ticker.C:
			err := m.FetchAndSyncAgents()
			if err != nil {
				m.Warningf("failed to get reverse tunnels: %v", err)
				continue
			}
		}
	}
}

func (m *AgentPool) addAgent(key agentKey, discoverProxies []services.Server) error {
	agent, err := NewAgent(AgentConfig{
		Addr:            key.addr,
		RemoteCluster:   key.domainName,
		Username:        m.cfg.HostUUID,
		Signers:         m.cfg.HostSigners,
		Client:          m.cfg.Client,
		AccessPoint:     m.cfg.AccessPoint,
		Context:         m.ctx,
		DiscoveryC:      m.discoveryC,
		DiscoverProxies: discoverProxies,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	m.Debugf("adding %v", agent)
	// start the agent in a goroutine. no need to handle Start() errors: Start() will be
	// retrying itself until the agent is closed
	go agent.Start()
	agents, _ := m.agents[key]
	agents = append(agents, agent)
	m.agents[key] = agents
	return nil
}

// reportStats submits report about agents state once in a while
func (m *AgentPool) reportStats() {
	var logReport bool
	if m.cfg.Clock.Now().Sub(m.lastReport) > defaults.ReportingPeriod {
		m.lastReport = m.cfg.Clock.Now()
		logReport = true
	}
	for key, agents := range m.agents {
		countPerState := make(map[string]int)
		for _, a := range agents {
			countPerState[a.getState()] += 1
		}
		for state, count := range countPerState {
			gauge, err := tunnelStats.GetMetricWithLabelValues(key.domainName, state)
			if err != nil {
				m.Warningf("%v", err)
				continue
			}
			gauge.Set(float64(count))
		}
		if logReport {
			m.WithFields(log.Fields{"target": key.domainName, "stats": countPerState}).Infof("outbound tunnel stats")
		}
	}
}

func (m *AgentPool) syncAgents(tunnels []services.ReverseTunnel) error {
	m.Lock()
	defer m.Unlock()

	keys, err := tunnelsToAgentKeys(tunnels)
	if err != nil {
		return trace.Wrap(err)
	}
	agentsToAdd, agentsToRemove := diffTunnels(m.agents, keys)
	// remove agents from deleted reverse tunnels
	for _, key := range agentsToRemove {
		m.closeAgents(&key)
	}
	// add agents from added reverse tunnels
	for _, key := range agentsToAdd {
		if err := m.addAgent(key, nil); err != nil {
			return trace.Wrap(err)
		}
	}

	m.reportStats()
	return nil
}

func tunnelsToAgentKeys(tunnels []services.ReverseTunnel) (map[agentKey]bool, error) {
	vals := make(map[agentKey]bool)
	for _, tunnel := range tunnels {
		keys, err := tunnelToAgentKeys(tunnel)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, key := range keys {
			vals[key] = true
		}
	}
	return vals, nil
}

func tunnelToAgentKeys(tunnel services.ReverseTunnel) ([]agentKey, error) {
	out := make([]agentKey, len(tunnel.GetDialAddrs()))
	for i, addr := range tunnel.GetDialAddrs() {
		netaddr, err := utils.ParseAddr(addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out[i] = agentKey{addr: *netaddr, domainName: tunnel.GetClusterName()}
	}
	return out, nil
}

func diffTunnels(existingTunnels map[agentKey][]*Agent, arrivedKeys map[agentKey]bool) ([]agentKey, []agentKey) {
	var agentsToRemove, agentsToAdd []agentKey
	for existingKey := range existingTunnels {
		if _, ok := arrivedKeys[existingKey]; !ok { // agent was removed
			agentsToRemove = append(agentsToRemove, existingKey)
		}
	}

	for arrivedKey := range arrivedKeys {
		if _, ok := existingTunnels[arrivedKey]; !ok { // agent was added
			agentsToAdd = append(agentsToAdd, arrivedKey)
		}
	}

	return agentsToAdd, agentsToRemove
}

type agentKey struct {
	domainName string
	addr       utils.NetAddr
}

func (a *agentKey) String() string {
	return fmt.Sprintf("agent(%v, %v)", a.domainName, a.addr.String())
}
