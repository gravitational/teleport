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
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnel/track"
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
	cfg          AgentPoolConfig
	agents       map[agentKey][]*Agent
	proxyTracker *track.Tracker
	ctx          context.Context
	cancel       context.CancelFunc
	// lastReport is the last time the agent has reported the stats
	lastReport time.Time
	// spawnLimiter limits agent spawn rate
	spawnLimiter utils.Retry
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
	// ReverseTunnelServer holds all reverse tunnel connections.
	ReverseTunnelServer Server
	// ProxyAddr if set, points to the address of the ssh proxy
	ProxyAddr string
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
	retry, err := utils.NewLinear(utils.LinearConfig{
		Step:      time.Second,
		Max:       time.Second * 8,
		Jitter:    utils.NewJitter(),
		AutoReset: 4,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(cfg.Context)
	pool := &AgentPool{
		agents:       make(map[agentKey][]*Agent),
		proxyTracker: track.New(ctx, track.Config{}),
		cfg:          cfg,
		ctx:          ctx,
		cancel:       cancel,
		spawnLimiter: retry,
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
	m.Debugf("Starting agent pool %s.%s...", m.cfg.HostUUID, m.cfg.Cluster)
	go m.pollAndSyncAgents()
	go m.processSeekEvents()
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

func (m *AgentPool) processSeekEvents() {
	limiter := m.spawnLimiter.Clone()
	for {
		select {
		case <-m.ctx.Done():
			m.Debugf("Halting seek event processing (pool closing)")
			return
		case lease := <-m.proxyTracker.Acquire():
			m.Debugf("Seeking: %+v.", lease.Key())
			m.withLock(func() {
				if err := m.addAgent(lease); err != nil {
					m.WithError(err).Errorf("Failed to add agent.")
				}
			})
		}
		select {
		case <-m.ctx.Done():
			m.Debugf("Halting seek event processing (pool closing)")
			return
		case <-limiter.After():
			limiter.Inc()
		}
	}
}

// FetchAndSyncAgents executes one time fetch and sync request
// (used in tests instead of polling)
func (m *AgentPool) FetchAndSyncAgents() error {
	tunnels, err := m.getReverseTunnels()
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
	ticker := time.NewTicker(defaults.ResyncInterval)
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
				m.Warningf("Failed to get reverse tunnels: %v.", err)
				continue
			}
		}
	}
}

func (m *AgentPool) addAgent(lease track.Lease) error {
	// If the component connecting is a proxy, get the cluster name from the
	// clusterName (where it is the name of the remote cluster). If it's a node, get
	// the cluster name from the agent pool configuration itself (where it is
	// the name of the local cluster).
	key := keyFromLease(lease)
	clusterName := key.clusterName
	if key.tunnelType == string(services.NodeTunnel) {
		clusterName = m.cfg.Cluster
	}
	agent, err := NewAgent(AgentConfig{
		Addr:                key.addr,
		ClusterName:         clusterName,
		Username:            m.cfg.HostUUID,
		Signers:             m.cfg.HostSigners,
		Client:              m.cfg.Client,
		AccessPoint:         m.cfg.AccessPoint,
		Context:             m.ctx,
		KubeDialAddr:        m.cfg.KubeDialAddr,
		Server:              m.cfg.Server,
		ReverseTunnelServer: m.cfg.ReverseTunnelServer,
		LocalClusterName:    m.cfg.Cluster,
		Tracker:             m.proxyTracker,
		Lease:               lease,
	})
	if err != nil {
		// ensure that lease has been released; OK to call multiple times.
		lease.Release()
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
	m.Lock()
	defer m.Unlock()
	out := make(map[string]int)

	for key, agents := range m.agents {
		count := 0
		for _, agent := range agents {
			if agent.getState() == agentStateConnected {
				count++
			}
		}
		out[key.clusterName] += count
	}

	return out
}

// getReverseTunnels always returns a builtin node tunnel
// that has to be established
func (m *AgentPool) getReverseTunnels() ([]services.ReverseTunnel, error) {
	// Proxy uses reverse tunnels for bookkeeping
	// purposes - to communicate that trusted cluster has been
	// deleted and all agents have to be closed.
	// Nodes do not have this need as the agent pool should
	// exist as long as the node is running.
	switch m.cfg.Component {
	case teleport.ComponentProxy:
		return m.cfg.AccessPoint.GetReverseTunnels()
	case teleport.ComponentNode:
		reverseTunnel := services.NewReverseTunnel(
			m.cfg.Cluster,
			[]string{m.cfg.ProxyAddr},
		)
		reverseTunnel.SetType(services.NodeTunnel)
		return []services.ReverseTunnel{reverseTunnel}, nil
	default:
		return nil, trace.BadParameter("unsupported component %q", m.cfg.Component)
	}
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
		countPerState := map[string]int{
			agentStateConnecting:   0,
			agentStateConnected:    0,
			agentStateDisconnected: 0,
		}
		for _, a := range agents {
			countPerState[a.getState()]++
		}
		for state, count := range countPerState {
			gauge, err := trustedClustersStats.GetMetricWithLabelValues(key.clusterName, state)
			if err != nil {
				m.Warningf("Failed to get gauge: %v.", err)
				continue
			}
			gauge.Set(float64(count))
		}
		if logReport {
			m.WithFields(log.Fields{"target": key.clusterName, "stats": countPerState}).Info("Outbound tunnel stats.")
		} else {
			m.WithFields(log.Fields{"target": key.clusterName, "stats": countPerState}).Debug("Outbound tunnel stats.")
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
		m.proxyTracker.Stop(agentToTrackingKey(key))
		m.closeAgents(&key)
	}
	// add agents from added reverse tunnels
	for _, key := range agentsToAdd {
		m.proxyTracker.Start(agentToTrackingKey(key))
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
		out[i] = agentKey{addr: *netaddr, tunnelType: string(tunnel.GetType()), clusterName: tunnel.GetClusterName()}
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
	// clusterName is a cluster name of this agent
	clusterName string

	// tunnelType is the type of tunnel, is either node or proxy.
	tunnelType string

	// addr is the address this tunnel is agent is connected to. For example:
	// proxy.example.com:3024.
	addr utils.NetAddr
}

func keyFromLease(lease track.Lease) agentKey {
	key := lease.Key().(track.Key)
	return agentKey{
		clusterName: key.Cluster,
		tunnelType:  key.Type,
		addr:        key.Addr,
	}
}

func agentToTrackingKey(key agentKey) track.Key {
	return track.Key{
		Cluster: key.clusterName,
		Type:    key.tunnelType,
		Addr:    key.addr,
	}
}

func (a *agentKey) String() string {
	return fmt.Sprintf("agentKey(cluster=%v, type=%v, addr=%v)", a.clusterName, a.tunnelType, a.addr.String())
}
