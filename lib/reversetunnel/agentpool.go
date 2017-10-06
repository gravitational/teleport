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
	"golang.org/x/crypto/ssh"

	log "github.com/sirupsen/logrus"
)

// AgentPool manages the pool of outbound reverse tunnel agents.
// The agent pool watches the reverse tunnel entries created by the admin and
// connects/disconnects to added/deleted tunnels.
type AgentPool struct {
	sync.Mutex
	*log.Entry
	cfg        AgentPoolConfig
	agents     map[agentKey]*Agent
	ctx        context.Context
	cancel     context.CancelFunc
	discoveryC chan *discoveryRequest
}

// AgentPoolConfig holds configuration parameters for the agent pool
type AgentPoolConfig struct {
	// Client is client to the auth server this agent connects to recieve
	// a list of pools
	Client *auth.TunClient
	// AccessPoint is a lightweight access point
	// that can optionally cache some values
	AccessPoint auth.AccessPoint
	// HostSigners is a list of host signers this agent presents itself as
	HostSigners []ssh.Signer
	// HostUUID is a unique ID of this host
	HostUUID string
	// Context is an optional context
	Context context.Context
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
	return nil
}

// NewAgentPool returns new isntance of the agent pool
func NewAgentPool(cfg AgentPoolConfig) (*AgentPool, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(cfg.Context)
	pool := &AgentPool{
		agents:     make(map[agentKey]*Agent),
		cfg:        cfg,
		ctx:        ctx,
		cancel:     cancel,
		discoveryC: make(chan *discoveryRequest),
	}
	pool.Entry = log.WithFields(log.Fields{
		teleport.Component: teleport.ComponentReverseTunnelAgent,
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
			m.Debugf("got discovery request, following proxies are not connected %v", req.Proxies)
		}
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

func (m *AgentPool) pollAndSyncAgents() {
	ticker := time.NewTicker(defaults.ReverseTunnelAgentHeartbeatPeriod)
	defer ticker.Stop()
	m.FetchAndSyncAgents()
	for {
		select {
		case <-m.ctx.Done():
			m.Debugf("closing")
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
		m.Debugf("removing %v", &key)
		agent := m.agents[key]
		delete(m.agents, key)
		agent.Close()
	}

	for _, key := range agentsToAdd {
		m.Debugf("adding %v", &key)
		agent, err := NewAgent(AgentConfig{
			Addr:          key.addr,
			RemoteCluster: key.domainName,
			Username:      m.cfg.HostUUID,
			Signers:       m.cfg.HostSigners,
			Client:        m.cfg.Client,
			AccessPoint:   m.cfg.AccessPoint,
			Context:       m.ctx,
			DiscoveryC:    m.discoveryC,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		// start the agent in a goroutine. no need to handle Start() errors: Start() will be
		// retrying itself until the agent is closed
		go agent.Start()
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
