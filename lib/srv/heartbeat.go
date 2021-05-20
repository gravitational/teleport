/*
Copyright 2018 Gravitational, Inc.

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

package srv

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

// KeepAliveState represents state of the heartbeat
type KeepAliveState int

func (k KeepAliveState) String() string {
	switch k {
	case HeartbeatStateInit:
		return "init"
	case HeartbeatStateAnnounce:
		return "announce"
	case HeartbeatStateAnnounceWait:
		return "announce-wait"
	case HeartbeatStateKeepAlive:
		return "keepalive"
	case HeartbeatStateKeepAliveWait:
		return "keepalive-wait"
	default:
		return fmt.Sprintf("unknown state %v", int(k))
	}
}

const (
	// HeartbeatStateInit is set when
	// the state has not been collected yet,
	// or the state is not fetched
	HeartbeatStateInit KeepAliveState = iota
	// HeartbeatStateAnnounce is set when full
	// state has to be announced back to the auth server
	HeartbeatStateAnnounce
	// HeartbeatStateAnnounceWait is set after successful
	// announce, heartbeat will wait until server updates
	// information, or time for next announce comes
	HeartbeatStateAnnounceWait
	// HeartbeatStateKeepAlive is set when
	// only sending keep alives is necessary
	HeartbeatStateKeepAlive
	// HeartbeatStateKeepAliveWait is set when
	// heartbeat will waiting until it's time to send keep alive
	HeartbeatStateKeepAliveWait
)

// HeartbeatMode represents the mode of the heartbeat
// node, proxy or auth server
type HeartbeatMode int

// CheckAndSetDefaults checks values and sets defaults
func (h HeartbeatMode) CheckAndSetDefaults() error {
	switch h {
	case HeartbeatModeNode, HeartbeatModeProxy, HeartbeatModeAuth, HeartbeatModeKube, HeartbeatModeApp, HeartbeatModeDB:
		return nil
	default:
		return trace.BadParameter("unrecognized mode")
	}
}

// String returns user-friendly representation of the mode
func (h HeartbeatMode) String() string {
	switch h {
	case HeartbeatModeNode:
		return "Node"
	case HeartbeatModeProxy:
		return "Proxy"
	case HeartbeatModeAuth:
		return "Auth"
	case HeartbeatModeKube:
		return "Kube"
	case HeartbeatModeApp:
		return "App"
	case HeartbeatModeDB:
		return "Database"
	default:
		return fmt.Sprintf("<unknown: %v>", int(h))
	}
}

const (
	// HeartbeatModeNode sets heartbeat to node
	// updates that support keep alives
	HeartbeatModeNode HeartbeatMode = iota
	// HeartbeatModeProxy sets heartbeat to proxy
	// that does not support keep alives
	HeartbeatModeProxy HeartbeatMode = iota
	// HeartbeatModeAuth sets heartbeat to auth
	// that does not support keep alives
	HeartbeatModeAuth HeartbeatMode = iota
	// HeartbeatModeKube is a mode for kubernetes service heartbeats.
	HeartbeatModeKube HeartbeatMode = iota
	// HeartbeatModeApp sets heartbeat to apps and will use keep alives.
	HeartbeatModeApp HeartbeatMode = iota
	// HeartbeatModeDB sets heatbeat to db
	HeartbeatModeDB HeartbeatMode = iota
)

// NewHeartbeat returns a new instance of heartbeat
func NewHeartbeat(cfg HeartbeatConfig) (*Heartbeat, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(cfg.Context)
	h := &Heartbeat{
		cancelCtx:       ctx,
		cancel:          cancel,
		HeartbeatConfig: cfg,
		Entry: log.WithFields(log.Fields{
			trace.Component: teleport.Component(cfg.Component, "beat"),
		}),
		checkTicker: cfg.Clock.NewTicker(cfg.CheckPeriod),
		announceC:   make(chan struct{}, 1),
		sendC:       make(chan struct{}, 1),
	}
	h.Debugf("Starting %v heartbeat with announce period: %v, keep-alive period %v, poll period: %v", cfg.Mode, cfg.KeepAlivePeriod, cfg.AnnouncePeriod, cfg.CheckPeriod)
	return h, nil
}

// GetServerInfoFn is function that returns server info
type GetServerInfoFn func() (services.Resource, error)

// HeartbeatConfig is a heartbeat configuration
type HeartbeatConfig struct {
	// Mode sets one of the proxy, auth or node modes.
	Mode HeartbeatMode
	// Context is parent context that signals
	// heartbeat cancel
	Context context.Context
	// Component is a name of component used in logs
	Component string
	// Announcer is used to announce presence
	Announcer auth.Announcer
	// GetServerInfo returns server information
	GetServerInfo GetServerInfoFn
	// ServerTTL is a server TTL used in announcements
	ServerTTL time.Duration
	// KeepAlivePeriod is a period between lights weight
	// keep alive calls, that only update TTLs and don't consume
	// bandwidh, also is used to derive time between
	// failed attempts as well for auth and proxy modes
	KeepAlivePeriod time.Duration
	// AnnouncePeriod is a period between announce calls,
	// when client sends full server specification
	// to the presence service
	AnnouncePeriod time.Duration
	// CheckPeriod is a period to check for updates
	CheckPeriod time.Duration
	// Clock is a clock used to override time in tests
	Clock clockwork.Clock
	// OnHeartbeat is called after every heartbeat. A non-nil error is passed
	// when a heartbeat fails.
	OnHeartbeat func(error)
}

// CheckAndSetDefaults checks and sets default values
func (cfg *HeartbeatConfig) CheckAndSetDefaults() error {
	if err := cfg.Mode.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if cfg.Context == nil {
		return trace.BadParameter("missing parameter Context")
	}
	if cfg.Announcer == nil {
		return trace.BadParameter("missing parameter Announcer")
	}
	if cfg.Component == "" {
		return trace.BadParameter("missing parameter Component")
	}
	if cfg.CheckPeriod == 0 {
		return trace.BadParameter("missing parameter CheckPeriod")
	}
	if cfg.KeepAlivePeriod == 0 {
		return trace.BadParameter("missing parameter KeepAlivePeriod")
	}
	if cfg.AnnouncePeriod == 0 {
		return trace.BadParameter("missing parameter AnnouncePeriod")
	}
	if cfg.ServerTTL == 0 {
		return trace.BadParameter("missing parmeter ServerTTL")
	}
	if cfg.GetServerInfo == nil {
		return trace.BadParameter("missing parameter GetServerInfo")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.OnHeartbeat == nil {
		// Blackhole callback if none was specified.
		cfg.OnHeartbeat = func(error) {}
	}

	return nil
}

// Heartbeat keeps heartbeat state, it is implemented
// according to actor model - all interactions with it are to be done
// with signals
type Heartbeat struct {
	HeartbeatConfig
	cancelCtx context.Context
	cancel    context.CancelFunc
	*log.Entry
	state     KeepAliveState
	current   services.Resource
	keepAlive *types.KeepAlive
	// nextAnnounce holds time of the next scheduled announce attempt
	nextAnnounce time.Time
	// nextKeepAlive holds the time of the nex scheduled keep alive attempt
	nextKeepAlive time.Time
	// checkTicker is a ticker for state transitions
	// during which different checks are performed
	checkTicker clockwork.Ticker
	// keepAliver sends keep alive updates
	keepAliver services.KeepAliver
	// announceC is event receives an event
	// whenever new announce has been sent, used in tests
	announceC chan struct{}
	// sendC is event channel used to trigger
	// new announces
	sendC chan struct{}
}

// Run periodically calls to announce presence,
// should be called explicitly in a separate goroutine
func (h *Heartbeat) Run() error {
	defer func() {
		h.reset(HeartbeatStateInit)
		h.checkTicker.Stop()
	}()
	for {
		err := h.fetchAndAnnounce()
		if err != nil {
			h.Warningf("Heartbeat failed %v.", err)
		}
		h.OnHeartbeat(err)
		select {
		case <-h.checkTicker.Chan():
		case <-h.sendC:
			h.Debugf("Asked check out of cycle")
		case <-h.cancelCtx.Done():
			h.Debugf("Heartbeat exited.")
			return nil
		}
	}
}

// Close closes all timers and goroutines,
// note that this function is equivalent of cancelling
// of the context passed in configuration and can be
// used interchangeably
func (h *Heartbeat) Close() error {
	// note that close does not clean up resources,
	// because it is unaware of heartbeat actual state,
	// Run() could may as well be creating new keep aliver
	// while this function attempts to close it,
	// so instead it relies on Run() loop to clean up after itself
	h.cancel()
	return nil
}

// setState is used to debug state transitions
// as it logs in addition to setting state
func (h *Heartbeat) setState(state KeepAliveState) {
	h.state = state
}

// reset resets keep alive state
// and sends the state back to the initial state
// of sending full update
func (h *Heartbeat) reset(state KeepAliveState) {
	h.setState(state)
	h.nextAnnounce = time.Time{}
	h.nextKeepAlive = time.Time{}
	h.keepAlive = nil
	if h.keepAliver != nil {
		if err := h.keepAliver.Close(); err != nil {
			h.Warningf("Failed to close keep aliver: %v", err)
		}
		h.keepAliver = nil
	}
}

// fetch, if succeeded updates or sets current server
// to the last received server
func (h *Heartbeat) fetch() error {
	// failed to fetch server info?
	// reset to init state regardless of the current state
	server, err := h.GetServerInfo()
	if err != nil {
		h.reset(HeartbeatStateInit)
		return trace.Wrap(err)
	}
	switch h.state {
	// in case of successful state fetch, move to announce from init
	case HeartbeatStateInit:
		h.current = server
		h.reset(HeartbeatStateAnnounce)
		return nil
		// nothing to do in announce state
	case HeartbeatStateAnnounce:
		return nil
	case HeartbeatStateAnnounceWait:
		// time to announce
		if h.Clock.Now().UTC().After(h.nextAnnounce) {
			h.current = server
			h.reset(HeartbeatStateAnnounce)
			return nil
		}
		result := services.CompareServers(h.current, server)
		// server update happened, time to announce
		if result == services.Different {
			h.current = server
			h.reset(HeartbeatStateAnnounce)
		}
		return nil
		// nothing to do in keep alive state
	case HeartbeatStateKeepAlive:
		return nil
		// Stay in keep alive state in case
		// if there are no changes
	case HeartbeatStateKeepAliveWait:
		// time to send a new keep alive
		if h.Clock.Now().UTC().After(h.nextKeepAlive) {
			h.setState(HeartbeatStateKeepAlive)
			return nil
		}
		result := services.CompareServers(h.current, server)
		// server update happened, move to announce
		if result == services.Different {
			h.current = server
			h.reset(HeartbeatStateAnnounce)
		}
		return nil
	default:
		return trace.BadParameter("unsupported state: %v", h.state)
	}
}

func (h *Heartbeat) announce() error {
	switch h.state {
	// nothing to do in those states in terms of announce
	case HeartbeatStateInit, HeartbeatStateKeepAliveWait, HeartbeatStateAnnounceWait:
		return nil
	case HeartbeatStateAnnounce:
		// proxies and auth servers don't support keep alive logic yet,
		// so keep state at announce forever for proxies
		switch h.Mode {
		case HeartbeatModeProxy:
			proxy, ok := h.current.(services.Server)
			if !ok {
				return trace.BadParameter("expected services.Server, got %#v", h.current)
			}
			err := h.Announcer.UpsertProxy(proxy)
			if err != nil {
				// try next announce using keep alive period,
				// that happens more frequently
				h.nextAnnounce = h.Clock.Now().UTC().Add(h.KeepAlivePeriod)
				h.setState(HeartbeatStateAnnounceWait)
				return trace.Wrap(err)
			}
			h.nextAnnounce = h.Clock.Now().UTC().Add(h.AnnouncePeriod)
			h.notifySend()
			h.setState(HeartbeatStateAnnounceWait)
			return nil
		case HeartbeatModeAuth:
			auth, ok := h.current.(services.Server)
			if !ok {
				return trace.BadParameter("expected services.Server, got %#v", h.current)
			}
			err := h.Announcer.UpsertAuthServer(auth)
			if err != nil {
				h.nextAnnounce = h.Clock.Now().UTC().Add(h.KeepAlivePeriod)
				h.setState(HeartbeatStateAnnounceWait)
				return trace.Wrap(err)
			}
			h.nextAnnounce = h.Clock.Now().UTC().Add(h.AnnouncePeriod)
			h.notifySend()
			h.setState(HeartbeatStateAnnounceWait)
			return nil
		case HeartbeatModeNode:
			node, ok := h.current.(services.Server)
			if !ok {
				return trace.BadParameter("expected services.Server, got %#v", h.current)
			}
			keepAlive, err := h.Announcer.UpsertNode(h.cancelCtx, node)
			if err != nil {
				return trace.Wrap(err)
			}
			h.notifySend()
			keepAliver, err := h.Announcer.NewKeepAliver(h.cancelCtx)
			if err != nil {
				h.reset(HeartbeatStateInit)
				return trace.Wrap(err)
			}
			h.nextAnnounce = h.Clock.Now().UTC().Add(h.AnnouncePeriod)
			h.nextKeepAlive = h.Clock.Now().UTC().Add(h.KeepAlivePeriod)
			h.keepAlive = keepAlive
			h.keepAliver = keepAliver
			h.setState(HeartbeatStateKeepAliveWait)
			return nil
		case HeartbeatModeKube:
			kube, ok := h.current.(services.Server)
			if !ok {
				return trace.BadParameter("expected services.Server, got %#v", h.current)
			}
			err := h.Announcer.UpsertKubeService(context.TODO(), kube)
			if err != nil {
				h.nextAnnounce = h.Clock.Now().UTC().Add(h.KeepAlivePeriod)
				h.setState(HeartbeatStateAnnounceWait)
				return trace.Wrap(err)
			}
			h.nextAnnounce = h.Clock.Now().UTC().Add(h.AnnouncePeriod)
			h.notifySend()
			h.setState(HeartbeatStateAnnounceWait)
			return nil
		case HeartbeatModeApp:
			app, ok := h.current.(services.Server)
			if !ok {
				return trace.BadParameter("expected services.Server, got %#v", h.current)
			}
			keepAlive, err := h.Announcer.UpsertAppServer(h.cancelCtx, app)
			if err != nil {
				return trace.Wrap(err)
			}
			h.notifySend()
			keepAliver, err := h.Announcer.NewKeepAliver(h.cancelCtx)
			if err != nil {
				h.reset(HeartbeatStateInit)
				return trace.Wrap(err)
			}
			h.nextAnnounce = h.Clock.Now().UTC().Add(h.AnnouncePeriod)
			h.nextKeepAlive = h.Clock.Now().UTC().Add(h.KeepAlivePeriod)
			h.keepAlive = keepAlive
			h.keepAliver = keepAliver
			h.setState(HeartbeatStateKeepAliveWait)
			return nil
		case HeartbeatModeDB:
			db, ok := h.current.(types.DatabaseServer)
			if !ok {
				return trace.BadParameter("expected services.DatabaseServer, got %#v", h.current)
			}
			keepAlive, err := h.Announcer.UpsertDatabaseServer(h.cancelCtx, db)
			if err != nil {
				return trace.Wrap(err)
			}
			h.notifySend()
			keepAliver, err := h.Announcer.NewKeepAliver(h.cancelCtx)
			if err != nil {
				h.reset(HeartbeatStateInit)
				return trace.Wrap(err)
			}
			h.nextAnnounce = h.Clock.Now().UTC().Add(h.AnnouncePeriod)
			h.nextKeepAlive = h.Clock.Now().UTC().Add(h.KeepAlivePeriod)
			h.keepAlive = keepAlive
			h.keepAliver = keepAliver
			h.setState(HeartbeatStateKeepAliveWait)
			return nil
		default:
			return trace.BadParameter("unknown mode %q", h.Mode)
		}
	case HeartbeatStateKeepAlive:
		keepAlive := *h.keepAlive
		keepAlive.Expires = h.Clock.Now().UTC().Add(h.ServerTTL)
		timeout := time.NewTimer(h.KeepAlivePeriod)
		defer timeout.Stop()
		select {
		case <-h.cancelCtx.Done():
			return nil
		case <-timeout.C:
			h.Warningf("Blocked on keep alive send, going to reset.")
			h.reset(HeartbeatStateInit)
			return trace.ConnectionProblem(nil, "timeout sending keep alive")
		case h.keepAliver.KeepAlives() <- keepAlive:
			h.notifySend()
			h.nextKeepAlive = h.Clock.Now().UTC().Add(h.KeepAlivePeriod)
			h.setState(HeartbeatStateKeepAliveWait)
			return nil
		case <-h.keepAliver.Done():
			h.Warningf("Keep alive has failed: %v.", h.keepAliver.Error())
			err := h.keepAliver.Error()
			h.reset(HeartbeatStateInit)
			return trace.ConnectionProblem(err, "keep alive channel closed")
		}
	default:
		return trace.BadParameter("unsupported state: %v", h.state)
	}
}

func (h *Heartbeat) notifySend() {
	select {
	case h.announceC <- struct{}{}:
		return
	default:
	}
}

// fetchAndAnnounce fetches data about server
// and announces it to the server
func (h *Heartbeat) fetchAndAnnounce() error {
	if err := h.fetch(); err != nil {
		return trace.Wrap(err)
	}
	if err := h.announce(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ForceSend forces send cycle, used in tests, returns
// nil in case of success, error otherwise
func (h *Heartbeat) ForceSend(timeout time.Duration) error {
	timeoutC := time.After(timeout)
	select {
	case h.sendC <- struct{}{}:
	case <-timeoutC:
		return trace.ConnectionProblem(nil, "timeout waiting for send")
	}
	select {
	case <-h.announceC:
		return nil
	case <-timeoutC:
		return trace.ConnectionProblem(nil, "timeout waiting for announce to be sent")
	}
}
