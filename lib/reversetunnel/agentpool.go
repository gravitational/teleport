package reversetunnel

import (
	"fmt"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	log "github.com/Sirupsen/logrus"
)

// AgentPool manages the pool of outbound reverse tunnel agents
// it watches the reverse tunnels entries created by admin and
// establishes shuts down the tunnels based on records
type AgentPool struct {
	sync.Mutex
	*log.Entry
	cfg            AgentPoolConfig
	agents         map[agentKey]*Agent
	closeBroadcast *utils.CloseBroadcaster
}

// AgentPoolConfig is a configuration parameters for agent pool
type AgentPoolConfig struct {
	// Client is client to the auth server this agent connects to recieve
	// a list of pools
	Client *auth.TunClient
	// EventLog is optional event log recording all events
	EventLog events.Log
	// HostSigners is a list of host signers this agent presents itself as
	HostSigners []ssh.Signer
	// HostUUID is a unique ID of this host
	HostUUID string
}

// NewAgentPool returns new isntance of the agent pool
func NewAgentPool(cfg AgentPoolConfig) (*AgentPool, error) {
	if cfg.Client == nil {
		return nil, trace.BadParameter("missing 'Client' parameter")
	}
	if len(cfg.HostSigners) == 0 {
		return nil, trace.BadParameter("missing 'HostSigners' parameter")
	}
	if len(cfg.HostUUID) == 0 {
		return nil, trace.BadParameter("missing 'HostUUID' parameter")
	}
	if cfg.EventLog == nil {
		cfg.EventLog = events.NullEventLogger
	}
	pool := &AgentPool{
		agents:         make(map[agentKey]*Agent),
		cfg:            cfg,
		closeBroadcast: utils.NewCloseBroadcaster(),
	}
	pool.Entry = log.WithFields(log.Fields{
		teleport.Component: teleport.ComponentReverseTunnel,
		teleport.ComponentFields: map[string]interface{}{
			"side": "agent",
			"mode": "agentpool",
		},
	})
	return pool, nil
}

// Start starts the agent pool
func (m *AgentPool) Start() error {
	go m.pollAndSyncAgents()
	return nil
}

// Stop stops the agent pool
func (m *AgentPool) Stop() {
	m.closeBroadcast.Close()
}

// Wait returns when agent pool is closed
func (m *AgentPool) Wait() error {
	select {
	case <-m.closeBroadcast.C:
		break
	}
	return nil
}

// FetchAndSyncAgents executes one time fetch and sync request
// (used in tests instead of polling)
func (m *AgentPool) FetchAndSyncAgents() error {
	tunnels, err := m.cfg.Client.GetReverseTunnels()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := m.syncAgents(tunnels); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (m *AgentPool) pollAndSyncAgents() {
	ticker := time.NewTicker(defaults.ReverseTunnelAgentHeartbeatPeriod)
	defer ticker.Stop()
	m.FetchAndSyncAgents()
	for {
		select {
		case <-m.closeBroadcast.C:
			m.Infof("closing")
			m.Lock()
			defer m.Unlock()
			for _, a := range m.agents {
				a.Close()
			}
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

func (m *AgentPool) syncAgents(tunnels []services.ReverseTunnel) error {
	m.Lock()
	defer m.Unlock()

	keys, err := tunnelsToAgentKeys(tunnels)
	if err != nil {
		return trace.Wrap(err)
	}
	agentsToAdd, agentsToRemove := diffTunnels(m.agents, keys)
	for _, key := range agentsToRemove {
		m.Infof("removing %v", &key)
		agent := m.agents[key]
		delete(m.agents, key)
		agent.Close()
	}

	for _, key := range agentsToAdd {
		m.Infof("adding %v", &key)
		agent, err := NewAgent(key.addr, m.cfg.HostUUID, m.cfg.HostSigners, m.cfg.Client, SetEventLogger(m.cfg.EventLog))
		if err != nil {
			return trace.Wrap(err)
		}
		go func() {
			if err := agent.Start(); err != nil {
				m.Warningf("%v failed to start", agent)
			}
		}()
		m.agents[key] = agent
	}
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
	out := make([]agentKey, len(tunnel.DialAddrs))
	for i, addr := range tunnel.DialAddrs {
		netaddr, err := utils.ParseAddr(addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out[i] = agentKey{addr: *netaddr, domainName: tunnel.DomainName}
	}
	return out, nil
}

func diffTunnels(existingTunnels map[agentKey]*Agent, arrivedKeys map[agentKey]bool) ([]agentKey, []agentKey) {
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
