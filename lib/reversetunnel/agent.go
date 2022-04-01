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

// Package reversetunnel sets up persistent reverse tunnel
// between remote site and teleport proxy, when site agents
// dial to teleport proxy's socket and teleport proxy can connect
// to any server through this tunnel.
package reversetunnel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/reversetunnel/track"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

type AgentState string

const (
	// AgentInitial is the state of an agent when first created.
	AgentInitial AgentState = "initial"
	// AgentConnected is the state of an agent when is successfully connects
	// to a server and sends its first heartbeat.
	AgentConnected AgentState = "connected"
	// AgentClosed is the state of an agent when the connection and all other
	// resources are cleaned up.
	AgentClosed AgentState = "closed"
)

// AgentStateCallback is called when an agent's state changes.
type AgentStateCallback func(*Agent)

// transporter handles the creation of new transports over ssh.
type transporter interface {
	// Transport creates a new transport.
	transport(context.Context, ssh.Channel, <-chan *ssh.Request, ssh.Conn) *transport
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
	// transporter creates a new transport.
	transporter transporter
	// versionGetter gets the connected auth server version.
	versionGetter versionGetter
	// tracker tracks existing proxies.
	tracker *track.Tracker
	// lease gives the agent an exclusive claim to connect to a proxy.
	lease track.Lease
	// clock is use to get the current time. Mock clocks can be used for
	// testing.
	clock clockwork.Clock
	// log is an optional logger.
	log logrus.FieldLogger
}

// checkAndSetDefaults ensures an agentConfig contains required parameters.
func (c *agentConfig) checkAndSetDefaults() error {
	if c.addr.IsEmpty() {
		return trace.BadParameter("missing parameter addr")
	}
	if c.sshDialer == nil {
		return trace.BadParameter("missing parameter sshDialer")
	}
	if c.transporter == nil {
		return trace.BadParameter("missing parameter transporter")
	}
	if c.versionGetter == nil {
		return trace.BadParameter("missing parameter versionGetter")
	}
	if c.tracker == nil {
		return trace.BadParameter("missing parameter tracker")
	}
	if c.clock == nil {
		c.clock = clockwork.NewRealClock()
	}
	if c.log == nil {
		c.log = logrus.New()
	}
	if !c.lease.IsZero() {
		c.log = c.log.WithField("leaseID", c.lease.ID())
	}

	c.log = c.log.WithField("target", c.addr.String())

	return nil
}

// Agent creates and manages a reverse tunnel to a remote proxy server.
type Agent struct {
	*agentConfig
	// client is a client for the agent's ssh connection.
	client SSHClient
	// state is the internal state of an agent. Use GetState for threadsafe access.
	state AgentState
	// mu manages concurrent access to agent state.
	mu sync.RWMutex
	// hbChannel is the channel heartbeats are sent over.
	hbChannel ssh.Channel
	// hbRequests are requests going over the heartbeat channel.
	hbRequests <-chan *ssh.Request
	// unclaim releases the claim to the proxy in the tracker.
	unclaim func()
	// stopOnce ensure that cleanup when stopping an agent runs once.
	stopOnce sync.Once
	// ctx is the internal context used to release resources used by  the agent.
	ctx context.Context
	// cancel cancels the internal context.
	cancel context.CancelFunc
	// wg ensures that all concurrent operations finish.
	wg          sync.WaitGroup
	transportWG sync.WaitGroup
}

// NewAgent intializes a reverse tunnel agent.
func NewAgent(config *agentConfig) (*Agent, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Agent{
		agentConfig: config,
		state:       AgentInitial,
	}, nil
}

// String returns the string representation of an agent.
func (a *Agent) String() string {
	return fmt.Sprintf("agent(leaseID=%d,state=%s) -> %s", a.lease.ID(), a.GetState(), a.addr.String())
}

// GetState returns the current state of the agent.
func (a *Agent) GetState() AgentState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// updateState updates the internal state of the agent.
func (a *Agent) updateState(state AgentState) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.state != state {
		a.log.Debugf("Changing state %s -> %s.", a.state, state)
	}

	a.state = state
	if a.stateCallback != nil {
		go a.stateCallback(a)
	}
}

// Start starts an agent returning after successfully connecting and sending
// the first heatbeat.
func (a *Agent) Start(ctx context.Context) error {
	a.log.Debugf("Starting agent %v", a.addr)
	a.ctx, a.cancel = context.WithCancel(ctx)

	err := a.connect()
	if err != nil {
		a.Stop()
		return trace.Wrap(err)
	}

	// Update state before starting handlers. This ensures a future call to
	// to stop will see the connection was started and update the state.
	a.updateState(AgentConnected)

	// Start handing global requests again.
	a.wg.Add(1)
	go func() {
		err := a.handleGlobalRequests(a.ctx, a.client.GlobalRequests())
		if err != nil {
			a.log.WithError(err).Debug("Failed to handle global requests.")
		}
		a.wg.Done()
		a.Stop()
	}()

	a.wg.Add(1)
	go func() {
		a.handleChannels()
		if err != nil {
			a.log.WithError(err).Debug("Failed to handle channels.")
		}
		a.wg.Done()
		a.Stop()
	}()

	return nil
}

// connect connects to the server and finishes setting up the agent.
func (a *Agent) connect() error {
	client, err := a.sshDialer.DialContext(a.ctx, a.addr)
	if err != nil {
		return trace.Wrap(err)
	}
	a.client = client

	unclaim, ok := a.tracker.Claim(a.client.Principals()...)
	if !ok {
		a.client.Close()
		return trace.Errorf("Failed to claim proxy: %v claimed by another agent", a.client.Principals())
	}
	a.unclaim = unclaim

	startupCtx, cancel := context.WithCancel(a.ctx)
	done := make(chan struct{})

	// Temporarily reply to global requests during startup. This is necessary
	// due to the server sending a version request when we connect.
	go func() {
		a.handleGlobalRequests(startupCtx, a.client.GlobalRequests())
		close(done)
	}()

	// Ensure we stop handling global requests before returning.
	defer func() {
		cancel()
		<-done
	}()

	err = a.sendFirstHeartbeat(a.ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// sendFirstHeartbeat opens the heartbeat channel and sends the first
// heartbeat.
func (a *Agent) sendFirstHeartbeat(ctx context.Context) error {
	channel, requests, err := a.client.OpenChannel(chanHeartbeat, nil)
	if err != nil {
		return trace.Wrap(err)
	}

	a.hbChannel = channel
	a.hbRequests = requests

	// Send the first ping right away.
	if _, err := a.hbChannel.SendRequest("ping", false, nil); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Stop stops the agent ensuring the cleanup runs exactly once.
func (a *Agent) Stop() error {
	a.stopOnce.Do(func() {
		a.stop()
	})
	return nil
}

// stop closes the connection gracefully. This should only be called once.
func (a *Agent) stop() error {
	// Only update state if the agent was started.
	if a.GetState() != AgentInitial {
		defer a.updateState(AgentClosed)
	}

	// Wait for open tranports to close before closing the connection.
	a.transportWG.Wait()

	a.cancel()
	if a.client != nil {
		a.client.Close()
	}

	a.wg.Wait()

	a.lease.Release()
	if a.unclaim != nil {
		a.unclaim()
	}

	return nil
}

// handleGlobalRequests processes global requests from the server.
func (a *Agent) handleGlobalRequests(ctx context.Context, requests <-chan *ssh.Request) error {
	for {
		select {
		case r := <-requests:
			// When the channel is closing, nil is returned.
			if r == nil {
				return trace.Errorf("global request channel is closing")
			}

			switch r.Type {
			case versionRequest:
				version, err := a.versionGetter.getVersion(ctx)
				if err != nil {
					a.log.WithError(err).Warnf("Failed to retrieve auth version in response to %v request.", r.Type)
					if err := a.client.Reply(r, false, []byte("Failed to retrieve auth version")); err != nil {
						a.log.Debugf("Failed to reply to %v request: %v.", r.Type, err)
						continue
					}
				}

				if err := a.client.Reply(r, true, []byte(version)); err != nil {
					a.log.Debugf("Failed to reply to %v request: %v.", r.Type, err)
					continue
				}
			default:
				// This handles keep-alive messages and matches the behaviour of OpenSSH.
				err := a.client.Reply(r, false, nil)
				if err != nil {
					a.log.Debugf("Failed to reply to %v request: %v.", r.Type, err)
					continue
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// handleChannels handles tranports, discoveries, and heartbeats.
func (a *Agent) handleChannels() error {
	ticker := time.NewTicker(a.keepAlive)
	defer ticker.Stop()

	newTransportC := a.client.HandleChannelOpen(constants.ChanTransport)
	newDiscoveryC := a.client.HandleChannelOpen(chanDiscovery)

	for {
		select {
		// need to exit:
		case <-a.ctx.Done():
			return trace.ConnectionProblem(nil, "heartbeat: agent is stopped")
		// time to ping:
		case <-ticker.C:
			bytes, _ := a.clock.Now().UTC().MarshalText()
			_, err := a.hbChannel.SendRequest("ping", false, bytes)
			if err != nil {
				a.log.Error(err)
				return trace.Wrap(err)
			}
			a.log.Debugf("Ping -> %v.", a.client.RemoteAddr())
		// ssh channel closed:
		case req := <-a.hbRequests:
			if req == nil {
				return trace.ConnectionProblem(nil, "heartbeat: connection closed")
			}
		// Handle transport requests.
		case nch := <-newTransportC:
			if nch == nil {
				continue
			}
			a.log.Debugf("Transport request: %v.", nch.ChannelType())
			ch, req, err := nch.Accept()
			if err != nil {
				a.log.Warningf("Failed to accept transport request: %v.", err)
				continue
			}

			t := a.transporter.transport(a.ctx, ch, req, a.client)

			a.transportWG.Add(1)
			go func() {
				t.start()
				a.transportWG.Done()
			}()

		// new discovery request channel
		case nch := <-newDiscoveryC:
			if nch == nil {
				continue
			}
			a.log.Debugf("Discovery request channel opened: %v.", nch.ChannelType())
			ch, req, err := nch.Accept()
			if err != nil {
				a.log.Warningf("Failed to accept discovery channel request: %v.", err)
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
func (a *Agent) handleDiscovery(ch ssh.Channel, reqC <-chan *ssh.Request) {
	a.log.Debugf("handleDiscovery requests channel.")
	defer func() {
		if err := ch.Close(); err != nil {
			a.log.Warnf("Failed to close discovery channel:: %v", err)
		}
	}()

	for {
		var req *ssh.Request
		select {
		case <-a.ctx.Done():
			return
		case req = <-reqC:
			if req == nil {
				a.log.Infof("Connection closed, returning")
				return
			}
			r, err := unmarshalDiscoveryRequest(req.Payload)
			if err != nil {
				a.log.Warningf("Bad payload: %v.", err)
				return
			}

			for _, proxy := range r.Proxies {
				a.tracker.TrackExpected(proxy.GetName())
			}
		}
	}
}

const (
	chanHeartbeat    = "teleport-heartbeat"
	chanDiscovery    = "teleport-discovery"
	chanDiscoveryReq = "discovery"
)

const (
	// LocalNode is a special non-resolvable address that indicates the request
	// wants to connect to a dialed back node.
	LocalNode = "@local-node"
	// RemoteAuthServer is a special non-resolvable address that indicates client
	// requests a connection to the remote auth server.
	RemoteAuthServer = "@remote-auth-server"
	// LocalKubernetes is a special non-resolvable address that indicates that clients
	// requests a connection to the kubernetes endpoint of the local proxy.
	// This has to be a valid domain name, so it lacks @
	LocalKubernetes = "remote.kube.proxy." + constants.APIDomain
	// LocalWindowsDesktop is a special non-resolvable address that indicates
	// that clients requests a connection to the windows service endpoint of
	// the local proxy.
	// This has to be a valid domain name, so it lacks @
	LocalWindowsDesktop = "remote.windows_desktop.proxy." + constants.APIDomain
)
