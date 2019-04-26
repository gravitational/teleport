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
	"net"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
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
	// KubeDialAddr is an address of a kubernetes proxy
	KubeDialAddr utils.NetAddr
	// Server is a SSH server that can handle a connection (perform a handshake
	// then process). Only set with the agent is running within a node.
	Server ServerHandler
	// Component is the Teleport component this agent pool is running in. It can
	// either be proxy (trusted clusters) or node (dial back).
	Component string
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
		if len(m.agents[*matchKey]) == 0 {
			delete(m.agents, *matchKey)
		}
		return
	}
	for key, agents := range m.agents {
		m.agents[key] = filterAndClose(agents, matchAgent)
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
			m.Debugf("Closing.")
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
	// If the component connecting is a proxy, get the cluster name from the
	// tunnelID (where it is the name of the remote cluster). If it's a node, get
	// the cluster name from the agent pool configuration itself (where it is
	// the name of the local cluster).
	clusterName := key.tunnelID
	if key.tunnelType == string(services.NodeTunnel) {
		clusterName = m.cfg.Cluster
	}

	agent, err := NewAgent(AgentConfig{
		Addr:            key.addr,
		ClusterName:     clusterName,
		Username:        m.cfg.HostUUID,
		Signers:         m.cfg.HostSigners,
		Client:          m.cfg.Client,
		AccessPoint:     m.cfg.AccessPoint,
		Context:         m.ctx,
		DiscoveryC:      m.discoveryC,
		DiscoverProxies: discoverProxies,
		KubeDialAddr:    m.cfg.KubeDialAddr,
		Server:          m.cfg.Server,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	m.Debugf("Adding %v.", agent)
	// start the agent in a goroutine. no need to handle Start() errors: Start() will be
	// retrying itself until the agent is closed
	go agent.Start()
	agents, _ := m.agents[key]
	agents = append(agents, agent)
	m.agents[key] = agents
	return nil
}

// Counts returns a count of the number of proxies a outbound tunnel is
// connected to. Used in tests to determine if a proxy has been found and/or
// removed.
func (m *AgentPool) Counts() map[string]int {
	out := make(map[string]int)

	for key, agents := range m.agents {
		out[key.tunnelID] += len(agents)
	}

	return out
}

// reportStats submits report about agents state once in a while at info
// level. Always logs more detailed information at debug level.
func (m *AgentPool) reportStats() {
	var logReport bool
	if m.cfg.Clock.Now().Sub(m.lastReport) > defaults.ReportingPeriod {
		m.lastReport = m.cfg.Clock.Now()
		logReport = true
	}

	for key, agents := range m.agents {
		m.Debugf("Outbound tunnel for %v connected to %v proxies.", key.tunnelID, len(agents))

		countPerState := map[string]int{
			agentStateConnecting:   0,
			agentStateDiscovering:  0,
			agentStateConnected:    0,
			agentStateDiscovered:   0,
			agentStateDisconnected: 0,
		}
		for _, a := range agents {
			countPerState[a.getState()]++
		}
		for state, count := range countPerState {
			gauge, err := trustedClustersStats.GetMetricWithLabelValues(key.tunnelID, state)
			if err != nil {
				m.Warningf("Failed to get gauge: %v.", err)
				continue
			}
			gauge.Set(float64(count))
		}
		if logReport {
			m.WithFields(log.Fields{"target": key.tunnelID, "stats": countPerState}).Info("Outbound tunnel stats.")
		}
	}
}

func (m *AgentPool) syncAgents(tunnels []services.ReverseTunnel) error {
	m.Lock()
	defer m.Unlock()

	// Filter out tunnels based off if the AgentPool is running in the proxy
	// or node.
	//
	// For proxies, get all tunnels of type proxy.
	//
	// For nodes, get all tunnels of type node and with the same UUID as the host.
	// For nodes, because the AgentPool is running in the host, this ensures it
	// only picks up tunnels for itself.
	filtered := make([]services.ReverseTunnel, 0, len(tunnels))
	switch m.cfg.Component {
	case teleport.ComponentProxy:
		for _, t := range tunnels {
			if t.GetType() == services.ProxyTunnel {
				filtered = append(filtered, t)
			}
		}
	case teleport.ComponentNode:
		for _, t := range tunnels {
			if t.GetType() == services.NodeTunnel && t.GetName() == m.cfg.HostUUID {
				filtered = append(filtered, t)
			}
		}
	}

	keys, err := tunnelsToAgentKeys(filtered)
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

	// Remove disconnected agents from the list of agents.
	m.removeDisconnected()

	// Report tunnel statistics.
	m.reportStats()

	return nil
}

// removeDisconnected removes disconnected agents from the list of agents.
// This function should be called under a lock.
func (m *AgentPool) removeDisconnected() {
	for agentKey, agentSlice := range m.agents {
		// Filter and close all disconnected agents.
		validAgents := filterAndClose(agentSlice, func(agent *Agent) bool {
			if agent.getState() == agentStateDisconnected {
				return true
			}
			return false
		})

		// Update (or delete) agent key with filter applied.
		if len(validAgents) > 0 {
			m.agents[agentKey] = validAgents
		} else {
			delete(m.agents, agentKey)
		}
	}
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
		out[i] = agentKey{addr: *netaddr, tunnelType: string(tunnel.GetType()), tunnelID: tunnel.GetClusterName()}
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

// agentKey is used to uniquely identify agents.
type agentKey struct {
	// tunnelID identifies who the tunnel is connected to. For trusted clusters,
	// the tunnelID is the name of the remote cluster (like example.com). For
	// nodes, it is the nodeID (like 4a050852-23b5-4d6d-a45f-bed02792d453.example.com).
	tunnelID string

	// tunnelType is the type of tunnel, is either node or proxy.
	tunnelType string

	// addr is the address this tunnel is agent is connected to. For example:
	// proxy.example.com:3024.
	addr utils.NetAddr
}

func (a *agentKey) String() string {
	return fmt.Sprintf("agentKey(tunnelID=%v, type=%v, addr=%v)", a.tunnelID, a.tunnelType, a.addr.String())
}
