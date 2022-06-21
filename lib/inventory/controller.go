/*
Copyright 2022 Gravitational, Inc.

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

package inventory

import (
	"context"
	"time"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"

	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
)

// Auth is an interface representing the subset of the auth API that must be made available
// to the controller in order for it to be able to handle control streams.
type Auth interface {
	UpsertNode(context.Context, types.Server) (*types.KeepAlive, error)
	KeepAliveServer(context.Context, types.KeepAlive) error
}

type testEvent string

const (
	sshKeepAliveOk  testEvent = "ssh-keep-alive-ok"
	sshKeepAliveErr testEvent = "ssh-keep-alive-err"

	sshUpsertOk  testEvent = "ssh-upsert-ok"
	sshUpsertErr testEvent = "ssh-upsert-err"

	sshUpsertRetryOk  testEvent = "ssh-upsert-retry-ok"
	sshUpsertRetryErr testEvent = "ssh-upsert-retry-err"

	handlerStart = "handler-start"
	handlerClose = "handler-close"
)

type controllerOptions struct {
	serverKeepAlive  time.Duration
	testEvents       chan testEvent
	maxKeepAliveErrs int
}

func (options *controllerOptions) SetDefaults() {
	if options.serverKeepAlive == 0 {
		baseKeepAlive := apidefaults.ServerKeepAliveTTL()
		// use 1.5x standard server keep alive since we use a jitter that
		// shortens the actual average interval.
		options.serverKeepAlive = baseKeepAlive + (baseKeepAlive / 2)
	}

	if options.maxKeepAliveErrs == 0 {
		// fail on third error. this is arbitrary, but feels reasonable since if we're on our
		// third error, 3-4m have gone by, so the problem is almost certainly persistent.
		options.maxKeepAliveErrs = 2
	}
}

type ControllerOption func(c *controllerOptions)

func withServerKeepAlive(d time.Duration) ControllerOption {
	return func(opts *controllerOptions) {
		opts.serverKeepAlive = d
	}
}

func withTestEventsChannel(ch chan testEvent) ControllerOption {
	return func(opts *controllerOptions) {
		opts.testEvents = ch
	}
}

// Controller manages the inventory control streams registered with a given auth instance. Incoming
// messages are processed by invoking the appropriate methods on the Auth interface.
type Controller struct {
	store            *Store
	auth             Auth
	serverKeepAlive  time.Duration
	serverTTL        time.Duration
	maxKeepAliveErrs int
	testEvents       chan testEvent
	closeContext     context.Context
	cancel           context.CancelFunc
}

// NewController sets up a new controller instance.
func NewController(auth Auth, opts ...ControllerOption) *Controller {
	var options controllerOptions
	for _, opt := range opts {
		opt(&options)
	}
	options.SetDefaults()

	ctx, cancel := context.WithCancel(context.Background())
	return &Controller{
		store:            NewStore(),
		serverKeepAlive:  options.serverKeepAlive,
		serverTTL:        apidefaults.ServerAnnounceTTL,
		maxKeepAliveErrs: options.maxKeepAliveErrs,
		auth:             auth,
		testEvents:       options.testEvents,
		closeContext:     ctx,
		cancel:           cancel,
	}
}

// RegisterControlStream registers a new control stream with the controller.
func (c *Controller) RegisterControlStream(stream client.UpstreamInventoryControlStream, hello proto.UpstreamInventoryHello) {
	handle := newUpstreamHandle(stream, hello)
	c.store.Insert(handle)
	go c.handleControlStream(handle)
}

// GetControlStream gets a control stream for the given server ID if one exists (if multiple control streams
// exist one is selected pseudorandomly).
func (c *Controller) GetControlStream(serverID string) (handle UpstreamHandle, ok bool) {
	handle, ok = c.store.Get(serverID)
	return
}

// Iter iterates across all handles registered with this controller.
// note: if multiple handles are registered for a given server, only
// one handle is selected pseudorandomly to be observed.
func (c *Controller) Iter(fn func(UpstreamHandle)) {
	c.store.Iter(fn)
}

func (c *Controller) testEvent(event testEvent) {
	if c.testEvents == nil {
		return
	}
	c.testEvents <- event
}

// handleControlStream is the "main loop" of the upstream control stream. It handles incoming messages
// and also manages keepalives for previously heartbeated state.
func (c *Controller) handleControlStream(handle *upstreamHandle) {
	c.testEvent(handlerStart)
	defer func() {
		c.store.Remove(handle)
		handle.Close() // no effect if CloseWithError was called below
		c.testEvent(handlerClose)
	}()
	keepAliveInterval := interval.New(interval.Config{
		Duration:      c.serverKeepAlive,
		FirstDuration: utils.HalfJitter(c.serverKeepAlive),
		Jitter:        utils.NewSeventhJitter(),
	})
	defer keepAliveInterval.Stop()

	for {
		select {
		case msg := <-handle.Recv():
			switch m := msg.(type) {
			case proto.UpstreamInventoryHello:
				log.Warnf("Unexpected upstream hello on control stream of server %q.", handle.Hello().ServerID)
				handle.CloseWithError(trace.BadParameter("unexpected upstream hello"))
				return
			case proto.InventoryHeartbeat:
				if err := c.handleHeartbeat(handle, m); err != nil {
					handle.CloseWithError(err)
					return
				}
			case proto.UpstreamInventoryPong:
				c.handlePong(handle, m)
			default:
				log.Warnf("Unexpected upstream message type %T on control stream of server %q.", m, handle.Hello().ServerID)
				handle.CloseWithError(trace.BadParameter("unexpected upstream message type %T", m))
				return
			}
		case <-keepAliveInterval.Next():
			if err := c.handleKeepAlive(handle); err != nil {
				handle.CloseWithError(err)
				return
			}
		case req := <-handle.pingC:
			// pings require multiplexing, so we need to do the sending from this
			// goroutine rather than sending directly via the handle.
			if err := c.handlePingRequest(handle, req); err != nil {
				handle.CloseWithError(err)
				return
			}
		case <-handle.Done():
			return
		case <-c.closeContext.Done():
			return
		}
	}
}

func (c *Controller) handlePong(handle *upstreamHandle, msg proto.UpstreamInventoryPong) {
	pending, ok := handle.pings[msg.ID]
	if !ok {
		log.Warnf("Unexpected upstream pong from server %q (id=%d).", handle.Hello().ServerID, msg.ID)
		return
	}
	pending.rspC <- pingResponse{
		d: time.Since(pending.start),
	}
	delete(handle.pings, msg.ID)
}

func (c *Controller) handlePingRequest(handle *upstreamHandle, req pingRequest) error {
	handle.pingCounter++
	ping := proto.DownstreamInventoryPing{
		ID: handle.pingCounter,
	}
	start := time.Now()
	if err := handle.Send(c.closeContext, ping); err != nil {
		req.rspC <- pingResponse{
			err: err,
		}
		return trace.Wrap(err)
	}
	handle.pings[handle.pingCounter] = pendingPing{
		start: start,
		rspC:  req.rspC,
	}
	return nil
}

func (c *Controller) handleHeartbeat(handle *upstreamHandle, hb proto.InventoryHeartbeat) error {
	if hb.SSHServer != nil {
		if err := c.handleSSHServerHB(handle, hb.SSHServer); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *Controller) handleSSHServerHB(handle *upstreamHandle, sshServer *types.ServerV2) error {
	// the auth layer verifies that a stream's hello message matches the identity and capabilities of the
	// client cert. after that point it is our responsibility to ensure that heartbeated information is
	// consistent with the identity and capabilities claimed in the initial hello.
	if !handle.HasService(types.RoleNode) {
		return trace.AccessDenied("control stream not configured to support ssh server heartbeats")
	}
	if sshServer.GetName() != handle.Hello().ServerID {
		return trace.AccessDenied("incorrect ssh server ID (expected %q, got %q)", handle.Hello().ServerID, sshServer.GetName())
	}

	sshServer.SetExpiry(time.Now().Add(c.serverTTL).UTC())

	lease, err := c.auth.UpsertNode(c.closeContext, sshServer)
	if err == nil {
		c.testEvent(sshUpsertOk)
		// store the new lease and reset retry state
		handle.sshServerLease = lease
		handle.retrySSHServerUpsert = false
	} else {
		c.testEvent(sshUpsertErr)
		log.Warnf("Failed to upsert ssh server %q on heartbeat: %v.", handle.Hello().ServerID, err)

		// blank old lease if any and set retry state. next time handleKeepAlive is called
		// we will attempt to upsert the server again.
		handle.sshServerLease = nil
		handle.retrySSHServerUpsert = true
	}
	handle.sshServer = sshServer
	return nil
}

func (c *Controller) handleKeepAlive(handle *upstreamHandle) error {
	if handle.sshServerLease != nil {
		lease := *handle.sshServerLease
		lease.Expires = time.Now().Add(c.serverTTL).UTC()
		if err := c.auth.KeepAliveServer(c.closeContext, lease); err != nil {
			c.testEvent(sshKeepAliveErr)
			handle.sshServerKeepAliveErrs++
			shouldClose := handle.sshServerKeepAliveErrs > c.maxKeepAliveErrs

			log.Warnf("Failed to keep alive ssh server %q: %v (count=%d, closing=%v).", handle.Hello().ServerID, err, handle.sshServerKeepAliveErrs, shouldClose)

			if shouldClose {
				return trace.Errorf("failed to keep alive ssh server: %v", err)
			}
		} else {
			c.testEvent(sshKeepAliveOk)
		}
	} else if handle.retrySSHServerUpsert {
		handle.sshServer.SetExpiry(time.Now().Add(c.serverTTL).UTC())
		lease, err := c.auth.UpsertNode(c.closeContext, handle.sshServer)
		if err != nil {
			c.testEvent(sshUpsertRetryErr)
			log.Warnf("Failed to upsert ssh server %q on retry: %v.", handle.Hello().ServerID, err)
			// since this is retry-specific logic, an error here means that upsert failed twice in
			// a row. Missing upserts is more problematic than missing keepalives so we don't bother
			// attempting a third time.
			return trace.Errorf("failed to upsert ssh server on retry: %v", err)
		}
		c.testEvent(sshUpsertRetryOk)
		handle.sshServerLease = lease
		handle.retrySSHServerUpsert = false
	}

	return nil
}

// Close terminates all control streams registered with this controller. Control streams
// registered after Close() is called are closed immediately.
func (c *Controller) Close() error {
	c.cancel()
	return nil
}
