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

// Package reversetunnel sets up persistent reverse tunnel
// between remote site and teleport proxy, when site agents
// dial to teleport proxy's socket and teleport proxy can connect
// to any server through this tunnel.
package reversetunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/constants"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/reversetunnel/track"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

type AgentState string

const (
	// AgentInitial is the state of an agent when first created.
	AgentInitial AgentState = "initial"
	// AgentConnecting is the state when an agent is starting but not yet connected.
	AgentConnecting AgentState = "connecting"
	// AgentConnected is the state of an agent when is successfully connects
	// to a server and sends its first heartbeat.
	AgentConnected AgentState = "connected"
	// AgentClosed is the state of an agent when the connection and all other
	// resources are cleaned up.
	AgentClosed AgentState = "closed"
)

// AgentStateCallback is called when an agent's state changes.
type AgentStateCallback func(AgentState)

// transportHandler handles the creation of new transports over ssh.
type transportHandler interface {
	// handleTransport runs the receiver of a teleport-transport channel.
	handleTransport(context.Context, ssh.Channel, <-chan *ssh.Request, sshutils.Conn)
}

// sshDialer is an ssh dialer that returns an SSHClient
type sshDialer interface {
	// DialContext dials the given address and creates a new SSHClient.
	DialContext(context.Context, utils.NetAddr) (SSHClient, error)
}

// versionGetter gets the connected auth server version.
type versionGetter interface {
	getVersion(context.Context) (string, error)
}

// SSHClient is a client for an ssh connection.
type SSHClient interface {
	ssh.ConnMetadata
	io.Closer
	Wait() error
	OpenChannel(ctx context.Context, name string, data []byte) (*tracessh.Channel, <-chan *ssh.Request, error)
	SendRequest(ctx context.Context, name string, wantReply bool, payload []byte) (bool, []byte, error)
	Principals() []string
	GlobalRequests() <-chan *ssh.Request
	HandleChannelOpen(channelType string) <-chan ssh.NewChannel
	Reply(*ssh.Request, bool, []byte) error
}

// agentConfig represents an agent configuration.
type agentConfig struct {
	// addr is the target address to dial.
	addr utils.NetAddr
	// keepAlive is the interval at which the agent will send heartbeats.
	keepAlive time.Duration
	// stateCallback is called each time the state changes.
	stateCallback AgentStateCallback
	// sshDialer creates a new ssh connection.
	sshDialer sshDialer
	// transportHandler handles teleport-transport channels.
	transportHandler transportHandler
	// versionGetter gets the connected auth server version.
	versionGetter versionGetter
	// tracker tracks existing proxies.
	tracker *track.Tracker
	// lease gives the agent an exclusive claim to connect to a proxy.
	lease *track.Lease
	// clock is use to get the current time. Mock clocks can be used for
	// testing.
	clock clockwork.Clock
	// logger is an optional logger.
	logger *slog.Logger
	// localAuthAddresses is a list of auth servers to use when dialing back to
	// the local cluster.
	localAuthAddresses []string
	// proxySigner is used to sign PROXY headers for securely propagating client IP address
	proxySigner multiplexer.PROXYHeaderSigner
}

// checkAndSetDefaults ensures an agentConfig contains required parameters.
func (c *agentConfig) checkAndSetDefaults() error {
	if c.addr.IsEmpty() {
		return trace.BadParameter("missing parameter addr")
	}
	if c.sshDialer == nil {
		return trace.BadParameter("missing parameter sshDialer")
	}
	if c.transportHandler == nil {
		return trace.BadParameter("missing parameter transportHandler")
	}
	if c.versionGetter == nil {
		return trace.BadParameter("missing parameter versionGetter")
	}
	if c.tracker == nil {
		return trace.BadParameter("missing parameter tracker")
	}
	if c.lease == nil {
		return trace.BadParameter("missing parameter lease")
	}
	if c.clock == nil {
		c.clock = clockwork.NewRealClock()
	}
	if c.logger == nil {
		c.logger = slog.Default()
	}
	c.logger = c.logger.With(
		"lease_id", c.lease.ID(),
		"target", c.addr.String(),
	)

	return nil
}

// agent creates and manages a reverse tunnel to a remote proxy server.
type agent struct {
	agentConfig
	// client is a client for the agent's ssh connection.
	client SSHClient
	// state is the internal state of an agent. Use GetState for threadsafe access.
	state AgentState
	// once ensures doneConnecting is closed exactly once.
	once sync.Once
	// mu manages concurrent access to agent state.
	mu sync.RWMutex
	// doneConnecting is used to synchronize access to fields initialized while
	// an agent is connecting and protects wait groups from being waited on early.
	doneConnecting chan struct{}
	// hbChannel is the channel heartbeats are sent over.
	hbChannel *tracessh.Channel
	// hbRequests are requests going over the heartbeat channel.
	hbRequests <-chan *ssh.Request
	// discoveryC receives new discovery channels.
	discoveryC <-chan ssh.NewChannel
	// transportC receives new tranport channels.
	transportC <-chan ssh.NewChannel
	// ctx is the internal context used to release resources used by  the agent.
	ctx context.Context
	// cancel cancels the internal context.
	cancel context.CancelFunc
	// wg ensures that all concurrent operations finish.
	wg sync.WaitGroup
	// drainCtx is used to release resourced that must be stopped to drain the agent.
	drainCtx context.Context
	// drainCancel cancels the drain context.
	drainCancel context.CancelFunc
	// drainWG tracks transports and other concurrent operations required
	// to drain a connection are finished.
	drainWG sync.WaitGroup
	// proxySigner is used to sign PROXY headers for securely propagating client IP address
	proxySigner multiplexer.PROXYHeaderSigner
}

// newAgent intializes a reverse tunnel agent.
func newAgent(config agentConfig) (*agent, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	noop := func() {}
	return &agent{
		agentConfig:    config,
		state:          AgentInitial,
		cancel:         noop,
		drainCancel:    noop,
		doneConnecting: make(chan struct{}),
		proxySigner:    config.proxySigner,
	}, nil
}

// String returns the string representation of an agent.
func (a *agent) String() string {
	return fmt.Sprintf("agent(leaseID=%d,state=%s) -> %s", a.lease.ID(), a.GetState(), a.addr.String())
}

// GetState returns the current state of the agent.
func (a *agent) GetState() AgentState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// GetProxyID returns the proxy id of the proxy the agent is connected to.
func (a *agent) GetProxyID() (string, bool) {
	if a.client == nil {
		return "", false
	}
	return proxyIDFromPrincipals(a.client.Principals())
}

// proxyIDFromPrincipals gets the proxy id from a list of principals.
func proxyIDFromPrincipals(principals []string) (string, bool) {
	if len(principals) == 0 {
		return "", false
	}

	// The proxy id will always be the first principal.
	id := principals[0]

	// Return the uuid from the format "<uuid>.<cluster-name>".
	split := strings.Split(id, ".")
	if len(split) == 0 {
		return "", false
	}

	return split[0], true
}

// updateState updates the internal state of the agent returning
// the state of the agent before the update and an error if the
// state transition is not valid.
func (a *agent) updateState(state AgentState) (AgentState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	errMsg := "invalid state transition: %s -> %s"

	// Once closed no state transitions are allowed.
	if a.state == AgentClosed {
		return a.state, trace.Errorf(errMsg, a.state, state)
	}

	// A state must not transition to itself.
	if a.state == state {
		return a.state, trace.Errorf(errMsg, a.state, state)
	}

	// A state must never transition back to initial.
	if state == AgentInitial {
		return a.state, trace.Errorf(errMsg, a.state, state)
	}

	// Connecting must transition from initial.
	if state == AgentConnecting && a.state != AgentInitial {
		return a.state, trace.Errorf(errMsg, a.state, state)
	}

	// Connected must transition from connecting.
	if state == AgentConnected && a.state != AgentConnecting {
		return a.state, trace.Errorf(errMsg, a.state, state)
	}

	prevState := a.state
	a.state = state
	a.logger.DebugContext(a.ctx, "Agent state updated",
		"previous_state", prevState,
		"current_state", state,
	)

	if a.agentConfig.stateCallback != nil {
		go a.agentConfig.stateCallback(a.state)
	}

	return prevState, nil
}

// Start starts an agent returning after successfully connecting and sending
// the first heartbeat.
func (a *agent) Start(ctx context.Context) error {
	a.logger.DebugContext(ctx, "Starting agent", "addr", a.addr.FullAddress())

	var err error
	defer func() {
		a.once.Do(func() {
			close(a.doneConnecting)
		})
		if err != nil {
			a.Stop()
		}
	}()

	_, err = a.updateState(AgentConnecting)
	if err != nil {
		return trace.Wrap(err)
	}

	a.ctx, a.cancel = context.WithCancel(ctx)
	a.drainCtx, a.drainCancel = context.WithCancel(a.ctx)

	err = a.connect()
	if err != nil {
		return trace.Wrap(err)
	}

	// Start handing global requests again.
	a.wg.Add(1)
	go func() {
		if err := a.handleGlobalRequests(a.ctx, a.client.GlobalRequests()); err != nil {
			a.logger.DebugContext(a.ctx, "Failed to handle global requests", "error", err)
		}
		a.wg.Done()
		a.Stop()
	}()

	// drainWG.Done will be called from handleDrainChannels.
	a.drainWG.Add(1)
	a.wg.Add(1)
	go func() {
		if err := a.handleDrainChannels(); err != nil {
			a.logger.DebugContext(a.ctx, "Failed to handle drainable channels", "error", err)
		}
		a.wg.Done()
		a.Stop()
	}()

	a.wg.Add(1)
	go func() {
		if err := a.handleChannels(); err != nil {
			a.logger.DebugContext(a.ctx, "Failed to handle channels", "error", err)
		}
		a.wg.Done()
		a.Stop()
	}()

	_, err = a.updateState(AgentConnected)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// connect connects to the server and finishes setting up the agent.
func (a *agent) connect() error {
	client, err := a.sshDialer.DialContext(a.ctx, a.addr)
	if err != nil {
		return trace.Wrap(err)
	}
	a.client = client

	if !a.lease.Claim(a.client.Principals()...) {
		a.client.Close()
		// the error message must end with [proxyAlreadyClaimedError] to be
		// recognized by [isProxyAlreadyClaimed]
		return trace.Errorf("failed to claim proxy %v: "+proxyAlreadyClaimedError, a.client.Principals())
	}

	startupCtx, cancel := context.WithCancel(a.ctx)

	// Add channel handlers immediately to avoid rejecting a channel.
	a.discoveryC = a.client.HandleChannelOpen(chanDiscovery)
	a.transportC = a.client.HandleChannelOpen(constants.ChanTransport)

	// Temporarily reply to global requests during startup. This is necessary
	// due to the server sending a version request when we connect.
	go func() {
		a.handleGlobalRequests(startupCtx, a.client.GlobalRequests())
	}()

	// Stop handling global requests before returning.
	defer func() {
		cancel()
	}()

	err = a.sendFirstHeartbeat(a.ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// sendFirstHeartbeat opens the heartbeat channel and sends the first
// heartbeat.
func (a *agent) sendFirstHeartbeat(ctx context.Context) error {
	channel, requests, err := a.client.OpenChannel(ctx, chanHeartbeat, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	sshutils.DiscardChannelData(channel)

	a.hbChannel = channel
	a.hbRequests = requests

	// Send the first ping right away.
	if _, err := a.hbChannel.SendRequest(ctx, "ping", false, nil); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Stop stops the agent ensuring the cleanup runs exactly once.
func (a *agent) Stop() error {
	prevState, err := a.updateState(AgentClosed)
	if err != nil {
		return trace.Wrap(err)
	}

	// Wait for agent to finish connecting.
	if prevState == AgentConnecting {
		<-a.doneConnecting
	}

	a.drainCancel()
	a.lease.Release()

	// Wait for open tranports to close before closing the connection.
	a.drainWG.Wait()

	a.cancel()
	if a.client != nil {
		a.client.Close()
	}

	a.wg.Wait()
	return nil
}

// handleGlobalRequests processes global requests from the server.
func (a *agent) handleGlobalRequests(ctx context.Context, requests <-chan *ssh.Request) error {
	for {
		select {
		case r := <-requests:
			// The request will be nil when the request channel is closing.
			if r == nil {
				return trace.Errorf("global request channel is closing")
			}

			switch r.Type {
			case versionRequest:
				version, err := a.versionGetter.getVersion(ctx)
				if err != nil {
					a.logger.WarnContext(ctx, "Failed to retrieve auth version in response to x-teleport-version request", "error", err)
					if err := a.client.Reply(r, false, []byte("Failed to retrieve auth version")); err != nil {
						a.logger.DebugContext(ctx, "Failed to reply to x-teleport-version request", "error", err)
						continue
					}
				}

				if err := a.client.Reply(r, true, []byte(version)); err != nil {
					a.logger.DebugContext(ctx, "Failed to reply to x-teleport-version request", "error", err)
					continue
				}
			case reconnectRequest:
				a.logger.DebugContext(ctx, "Received reconnect advisory request from proxy")
				if r.WantReply {
					err := a.client.Reply(r, true, nil)
					if err != nil {
						a.logger.DebugContext(ctx, "Failed to reply to reconnect@goteleport.com request", "error", err)
					}
				}

				// Fire off stop but continue to handle global requests until the
				// context is canceled to allow the agent to drain.
				go a.Stop()
			default:
				// This handles keep-alive messages and matches the behavior of OpenSSH.
				err := a.client.Reply(r, false, nil)
				if err != nil {
					a.logger.DebugContext(ctx, "Failed to reply to global request", "request_type", r.Type, "error", err)
					continue
				}
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (a *agent) isDraining() bool {
	return a.drainCtx.Err() != nil
}

// signalDraining will signal one time when the draining context is canceled.
func (a *agent) signalDraining() <-chan struct{} {
	c := make(chan struct{})
	a.wg.Add(1)
	go func() {
		<-a.drainCtx.Done()
		close(c)
		a.wg.Done()
	}()

	return c
}

// handleDrainChannels handles channels that should be stopped when the agent is draining.
func (a *agent) handleDrainChannels() error {
	ticker := time.NewTicker(a.keepAlive)
	defer ticker.Stop()

	// once ensures drainWG.Done() is called one more time
	// after no more transports will be created.
	once := &sync.Once{}
	drainWGDone := func() {
		once.Do(func() {
			a.drainWG.Done()
		})
	}
	defer drainWGDone()
	drainSignal := a.signalDraining()

	for {
		if a.isDraining() {
			drainWGDone()
		}

		select {
		case <-a.ctx.Done():
			return nil
		// Signal once when the drain context is canceled to ensure we unblock
		// to call drainWG.Done().
		case <-drainSignal:
			continue
		// Handle closed heartbeat channel.
		case req := <-a.hbRequests:
			if req == nil {
				return trace.ConnectionProblem(nil, "heartbeat: connection closed")
			}
		// Send ping over heartbeat channel.
		case <-ticker.C:
			if a.isDraining() {
				continue
			}
			bytes, _ := a.clock.Now().UTC().MarshalText()
			_, err := a.hbChannel.SendRequest(a.ctx, "ping", false, bytes)
			if err != nil {
				a.logger.ErrorContext(a.ctx, "failed to send ping request", "error", err)
				return trace.Wrap(err)
			}
			a.logger.DebugContext(a.ctx, "Sent ping request", "target_addr", logutils.StringerAttr(a.client.RemoteAddr()))
		// Handle transport requests.
		case nch := <-a.transportC:
			if nch == nil {
				continue
			}
			if a.isDraining() {
				err := nch.Reject(ssh.ConnectionFailed, "agent connection is draining")
				if err != nil {
					a.logger.WarnContext(a.ctx, "Failed to reject transport channel", "error", err)
				}
				continue
			}

			a.logger.DebugContext(a.ctx, "Received transport request", "channel_type", nch.ChannelType())
			ch, req, err := nch.Accept()
			if err != nil {
				a.logger.WarnContext(a.ctx, "Failed to accept transport request", "error", err)
				continue
			}

			a.drainWG.Add(1)
			go func() {
				defer a.drainWG.Done()
				a.transportHandler.handleTransport(a.ctx, ch, req, a.client)
			}()

		}
	}
}

// handleChannels handles channels that should run for the entire lifetime of the agent.
func (a *agent) handleChannels() error {
	for {
		select {
		// need to exit:
		case <-a.ctx.Done():
			return nil
		// new discovery request channel
		case nch := <-a.discoveryC:
			if nch == nil {
				continue
			}
			a.logger.DebugContext(a.ctx, "Discovery request channel opened", "channel_type", nch.ChannelType())
			ch, req, err := nch.Accept()
			if err != nil {
				a.logger.WarnContext(a.ctx, "Failed to accept discovery channel request", "error", err)
				continue
			}

			a.wg.Add(1)
			go func() {
				a.handleDiscovery(ch, req)
				a.wg.Done()
			}()
		}
	}
}

// handleDiscovery receives discovery requests from the reverse tunnel
// server, that informs agent about proxies registered in the remote
// cluster and the reverse tunnels already established
//
// ch   : SSH channel which received "teleport-transport" out-of-band request
// reqC : request payload
func (a *agent) handleDiscovery(ch ssh.Channel, reqC <-chan *ssh.Request) {
	a.logger.DebugContext(a.ctx, "handleDiscovery requests channel")
	sshutils.DiscardChannelData(ch)
	defer func() {
		if err := ch.Close(); err != nil {
			a.logger.WarnContext(a.ctx, "Failed to close discovery channel", "error", err)
		}
	}()

	for {
		var req *ssh.Request
		select {
		case <-a.ctx.Done():
			return
		case req = <-reqC:
			if req == nil {
				a.logger.InfoContext(a.ctx, "Connection closed, returning")
				return
			}

			var r discoveryRequest
			if err := json.Unmarshal(req.Payload, &r); err != nil {
				a.logger.WarnContext(a.ctx, "Received discovery request with bad payload", "error", err)
				return
			}

			a.logger.DebugContext(a.ctx, "Received discovery request", "discovery_request", logutils.StringerAttr(&r))
			a.tracker.TrackExpected(r.TrackProxies()...)
		}
	}
}

const (
	chanHeartbeat    = "teleport-heartbeat"
	chanDiscovery    = "teleport-discovery"
	chanDiscoveryReq = "discovery"
	reconnectRequest = "reconnect@goteleport.com"
)
