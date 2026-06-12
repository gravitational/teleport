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

package srv

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

// HeartbeatI abstracts over the basic interface of Heartbeat and HeartbeatV2. This can be removed
// once we've fully transitioned to HeartbeatV2.
type HeartbeatI interface {
	Run() error
	Close() error
}

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
	case HeartbeatModeNode, HeartbeatModeProxy, HeartbeatModeAuth, HeartbeatModeKube, HeartbeatModeApp, HeartbeatModeDB, HeartbeatModeDatabaseService, HeartbeatModeWindowsDesktopService, HeartbeatModeWindowsDesktop:
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
	case HeartbeatModeDatabaseService:
		return "DatabaseService"
	case HeartbeatModeWindowsDesktopService:
		return "WindowsDesktopService"
	case HeartbeatModeWindowsDesktop:
		return "WindowsDesktop"
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
	HeartbeatModeProxy
	// HeartbeatModeAuth sets heartbeat to auth
	// that does not support keep alives
	HeartbeatModeAuth
	// HeartbeatModeKube is a mode for kubernetes service heartbeats.
	HeartbeatModeKube
	// HeartbeatModeApp sets heartbeat to apps and will use keep alives.
	HeartbeatModeApp
	// HeartbeatModeDB sets heartbeat to db
	HeartbeatModeDB
	// HeartbeatModeDatabaseService sets heartbeat mode to DatabaseService.
	HeartbeatModeDatabaseService
	// HeartbeatModeWindowsDesktopService sets heartbeat mode to windows desktop
	// service.
	HeartbeatModeWindowsDesktopService
	// HeartbeatModeWindowsDesktop sets heartbeat mode to windows desktop.
	HeartbeatModeWindowsDesktop
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
			teleport.ComponentKey: teleport.Component(cfg.Component, "beat"),
		}),
		checkTicker: cfg.Clock.NewTicker(cfg.CheckPeriod),
		announceC:   make(chan struct{}, 1),
		sendC:       make(chan struct{}, 1),
	}
	h.Debugf("Starting %v heartbeat with announce period: %v, keep-alive period %v, poll period: %v", cfg.Mode, cfg.KeepAlivePeriod, cfg.AnnouncePeriod, cfg.CheckPeriod)
	return h, nil
}

// GetServerInfoFn is function that returns server info
type GetServerInfoFn func() (types.Resource, error)

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
	Announcer authclient.Announcer
	// GetServerInfo returns server information
	GetServerInfo GetServerInfoFn
	// ServerTTL is a server TTL used in announcements
	ServerTTL time.Duration
	// KeepAlivePeriod is a period between light-weight
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
		return trace.BadParameter("missing parameter ServerTTL")
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
	current   types.Resource
	keepAlive *types.KeepAlive
	// nextAnnounce holds time of the next scheduled announce attempt
	nextAnnounce time.Time
	// nextKeepAlive holds the time of the nex scheduled keep alive attempt
	nextKeepAlive time.Time
	// checkTicker is a ticker for state transitions
	// during which different checks are performed
	checkTicker clockwork.Ticker
	// keepAliver sends keep alive updates
	keepAliver types.KeepAliver
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
		doneSomething, err := h.fetchAndAnnounce()
		if err != nil {
			h.Warningf("Heartbeat failed %v.", err)
			h.OnHeartbeat(err)
		} else if doneSomething {
			h.OnHeartbeat(nil)
		}
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
// note that this function is equivalent of canceling
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

// announce may upsert a new heartbeat or issue a keepalive for an existing one,
// depending on the current time and the state of the heartbeat. The returned
// boolean flag will be true if successful communication with the control plane
// has occurred as part of the announce (i.e. if the actual communication wasn't
// skipped because of time or state).
func (h *Heartbeat) announce() (doneSomething bool, _ error) {
	switch h.state {
	// nothing to do in those states in terms of announce
	case HeartbeatStateInit, HeartbeatStateKeepAliveWait, HeartbeatStateAnnounceWait:
		return false, nil
	case HeartbeatStateAnnounce:
		// proxies and auth servers don't support keep alive logic yet,
		// so keep state at announce forever for proxies
		switch h.Mode {
		case HeartbeatModeProxy:
			proxy, ok := h.current.(types.Server)
			if !ok {
				return false, trace.BadParameter("expected services.Server, got %#v", h.current)
			}
			err := h.Announcer.UpsertProxy(h.cancelCtx, proxy)
			if err != nil {
				// try next announce using keep alive period,
				// that happens more frequently
				h.nextAnnounce = h.Clock.Now().UTC().Add(h.KeepAlivePeriod)
				h.setState(HeartbeatStateAnnounceWait)
				return false, trace.Wrap(err)
			}
			h.nextAnnounce = h.Clock.Now().UTC().Add(h.AnnouncePeriod)
			h.notifySend()
			h.setState(HeartbeatStateAnnounceWait)
			return true, nil
		case HeartbeatModeAuth:
			auth, ok := h.current.(types.Server)
			if !ok {
				return false, trace.BadParameter("expected services.Server, got %#v", h.current)
			}
			err := h.Announcer.UpsertAuthServer(h.cancelCtx, auth)
			if err != nil {
				h.nextAnnounce = h.Clock.Now().UTC().Add(h.KeepAlivePeriod)
				h.setState(HeartbeatStateAnnounceWait)
				return false, trace.Wrap(err)
			}
			h.nextAnnounce = h.Clock.Now().UTC().Add(h.AnnouncePeriod)
			h.notifySend()
			h.setState(HeartbeatStateAnnounceWait)
			return true, nil
		case HeartbeatModeNode:
			node, ok := h.current.(types.Server)
			if !ok {
				return false, trace.BadParameter("expected services.Server, got %#v", h.current)
			}
			keepAlive, err := h.Announcer.UpsertNode(h.cancelCtx, node)
			if err != nil {
				return false, trace.Wrap(err)
			}
			h.notifySend()
			keepAliver, err := h.Announcer.NewKeepAliver(h.cancelCtx)
			if err != nil {
				h.reset(HeartbeatStateInit)
				return false, trace.Wrap(err)
			}
			h.nextAnnounce = h.Clock.Now().UTC().Add(h.AnnouncePeriod)
			h.nextKeepAlive = h.Clock.Now().UTC().Add(h.KeepAlivePeriod)
			h.keepAlive = keepAlive
			h.keepAliver = keepAliver
			h.setState(HeartbeatStateKeepAliveWait)
			return true, nil
		case HeartbeatModeKube:
			var (
				keepAlive *types.KeepAlive
				err       error
			)

			switch current := h.current.(type) {
			case types.KubeServer:
				keepAlive, err = h.Announcer.UpsertKubernetesServer(h.cancelCtx, current)
				if err != nil {
					return false, trace.Wrap(err)
				}
			default:
				return false, trace.BadParameter("expected types.KubeServer, got %#v", h.current)
			}

			h.notifySend()
			keepAliver, err := h.Announcer.NewKeepAliver(h.cancelCtx)
			if err != nil {
				h.reset(HeartbeatStateInit)
				return false, trace.Wrap(err)
			}
			h.nextAnnounce = h.Clock.Now().UTC().Add(h.AnnouncePeriod)
			h.nextKeepAlive = h.Clock.Now().UTC().Add(h.KeepAlivePeriod)
			h.keepAlive = keepAlive
			h.keepAliver = keepAliver
			h.setState(HeartbeatStateKeepAliveWait)
			return true, nil
		case HeartbeatModeApp:
			var keepAlive *types.KeepAlive
			var err error
			switch current := h.current.(type) {
			case types.AppServer:
				keepAlive, err = h.Announcer.UpsertApplicationServer(h.cancelCtx, current)
			default:
				return false, trace.BadParameter("expected types.AppServer, got %#v", h.current)
			}
			if err != nil {
				return false, trace.Wrap(err)
			}
			h.notifySend()
			keepAliver, err := h.Announcer.NewKeepAliver(h.cancelCtx)
			if err != nil {
				h.reset(HeartbeatStateInit)
				return false, trace.Wrap(err)
			}
			h.nextAnnounce = h.Clock.Now().UTC().Add(h.AnnouncePeriod)
			h.nextKeepAlive = h.Clock.Now().UTC().Add(h.KeepAlivePeriod)
			h.keepAlive = keepAlive
			h.keepAliver = keepAliver
			h.setState(HeartbeatStateKeepAliveWait)
			return true, nil
		case HeartbeatModeDB:
			db, ok := h.current.(types.DatabaseServer)
			if !ok {
				return false, trace.BadParameter("expected services.DatabaseServer, got %#v", h.current)
			}
			keepAlive, err := h.Announcer.UpsertDatabaseServer(h.cancelCtx, db)
			if err != nil {
				return false, trace.Wrap(err)
			}
			h.notifySend()
			keepAliver, err := h.Announcer.NewKeepAliver(h.cancelCtx)
			if err != nil {
				h.reset(HeartbeatStateInit)
				return false, trace.Wrap(err)
			}
			h.nextAnnounce = h.Clock.Now().UTC().Add(h.AnnouncePeriod)
			h.nextKeepAlive = h.Clock.Now().UTC().Add(h.KeepAlivePeriod)
			h.keepAlive = keepAlive
			h.keepAliver = keepAliver
			h.setState(HeartbeatStateKeepAliveWait)
			return true, nil
		case HeartbeatModeWindowsDesktopService:
			wd, ok := h.current.(types.WindowsDesktopService)
			if !ok {
				return false, trace.BadParameter("expected services.WindowsDesktopService, got %#v", h.current)
			}
			keepAlive, err := h.Announcer.UpsertWindowsDesktopService(h.cancelCtx, wd)
			if err != nil {
				return false, trace.Wrap(err)
			}
			h.notifySend()
			keepAliver, err := h.Announcer.NewKeepAliver(h.cancelCtx)
			if err != nil {
				h.reset(HeartbeatStateInit)
				return false, trace.Wrap(err)
			}
			h.nextAnnounce = h.Clock.Now().UTC().Add(h.AnnouncePeriod)
			h.nextKeepAlive = h.Clock.Now().UTC().Add(h.KeepAlivePeriod)
			h.keepAlive = keepAlive
			h.keepAliver = keepAliver
			h.setState(HeartbeatStateKeepAliveWait)
			return true, nil
		case HeartbeatModeWindowsDesktop:
			desktop, ok := h.current.(types.WindowsDesktop)
			if !ok {
				return false, trace.BadParameter("expected types.WindowsDesktop, got %#v", h.current)
			}
			err := h.Announcer.UpsertWindowsDesktop(h.cancelCtx, desktop)
			if err != nil {
				h.nextAnnounce = h.Clock.Now().UTC().Add(h.KeepAlivePeriod)
				h.setState(HeartbeatStateAnnounceWait)
				return false, trace.Wrap(err)
			}
			h.nextAnnounce = h.Clock.Now().UTC().Add(h.AnnouncePeriod)
			h.notifySend()
			h.setState(HeartbeatStateAnnounceWait)
			return true, nil
		case HeartbeatModeDatabaseService:
			dbService, ok := h.current.(types.DatabaseService)
			if !ok {
				return false, trace.BadParameter("expected services.DatabaseService, got %#v", h.current)
			}
			keepAlive, err := h.Announcer.UpsertDatabaseService(h.cancelCtx, dbService)
			if err != nil {
				return false, trace.Wrap(err)
			}
			h.notifySend()
			keepAliver, err := h.Announcer.NewKeepAliver(h.cancelCtx)
			if err != nil {
				h.reset(HeartbeatStateInit)
				return false, trace.Wrap(err)
			}
			h.nextAnnounce = h.Clock.Now().UTC().Add(h.AnnouncePeriod)
			h.nextKeepAlive = h.Clock.Now().UTC().Add(h.KeepAlivePeriod)
			h.keepAlive = keepAlive
			h.keepAliver = keepAliver
			h.setState(HeartbeatStateKeepAliveWait)
			return true, nil
		default:
			return false, trace.BadParameter("unknown mode %q", h.Mode)
		}
	case HeartbeatStateKeepAlive:
		keepAlive := *h.keepAlive
		keepAlive.Expires = h.Clock.Now().UTC().Add(h.ServerTTL)
		timeout := time.NewTimer(h.KeepAlivePeriod)
		defer timeout.Stop()
		select {
		case <-h.cancelCtx.Done():
			return false, nil
		case <-timeout.C:
			h.Warningf("Blocked on keep alive send, going to reset.")
			h.reset(HeartbeatStateInit)
			return false, trace.ConnectionProblem(nil, "timeout sending keep alive")
		case h.keepAliver.KeepAlives() <- keepAlive:
			h.notifySend()
			h.nextKeepAlive = h.Clock.Now().UTC().Add(h.KeepAlivePeriod)
			h.setState(HeartbeatStateKeepAliveWait)
			return true, nil
		case <-h.keepAliver.Done():
			h.Warningf("Keep alive has failed: %v.", h.keepAliver.Error())
			err := h.keepAliver.Error()
			h.reset(HeartbeatStateInit)
			return false, trace.ConnectionProblem(err, "keep alive channel closed")
		}
	default:
		return false, trace.BadParameter("unsupported state: %v", h.state)
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
func (h *Heartbeat) fetchAndAnnounce() (doneSomething bool, _ error) {
	if err := h.fetch(); err != nil {
		return false, trace.Wrap(err)
	}
	return h.announce()
}
