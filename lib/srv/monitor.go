/*
Copyright 2019 Gravitational, Inc.

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
	"io"
	"net"
	"sync"
	"time"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// ActivityTracker is a connection activity tracker,
// it allows to update the activity on the connection
// and retrieve the time when the connection was last active
type ActivityTracker interface {
	// GetClientLastActive returns the time of the last recorded activity
	GetClientLastActive() time.Time
	// UpdateClientActivity updates the last active timestamp
	UpdateClientActivity()
}

// TrackingConn is an interface representing tracking connection
type TrackingConn interface {
	// LocalAddr returns local address
	LocalAddr() net.Addr
	// RemoteAddr returns remote address
	RemoteAddr() net.Addr
	// Close closes the connection
	Close() error
}

// MonitorConfig is a wiretap configuration
type MonitorConfig struct {
	// LockWatcher is a lock watcher.
	LockWatcher *services.LockWatcher
	// LockTargets is used to detect a lock applicable to the connection.
	LockTargets []types.LockTarget
	// LockingMode determines how to handle possibly stale lock views.
	LockingMode constants.LockingMode
	// DisconnectExpiredCert is a point in time when
	// the certificate should be disconnected
	DisconnectExpiredCert time.Time
	// ClientIdleTimeout is a timeout of inactivity
	// on the wire
	ClientIdleTimeout time.Duration
	// Clock is a clock, realtime or fixed in tests
	Clock clockwork.Clock
	// Tracker is activity tracker
	Tracker ActivityTracker
	// Conn is a connection to close
	Conn TrackingConn
	// Context is an external context to cancel the operation
	Context context.Context
	// Login is linux box login
	Login string
	// TeleportUser is a teleport user name
	TeleportUser string
	// ServerID is a session server ID
	ServerID string
	// Emitter is events emitter
	Emitter apievents.Emitter
	// Entry is a logging entry
	Entry log.FieldLogger
	// IdleTimeoutMessage is sent to the client when the idle timeout expires.
	IdleTimeoutMessage string
	// MessageWriter wraps a channel to send text messages to the client. Use
	// for disconnection messages, etc.
	MessageWriter io.StringWriter
}

// CheckAndSetDefaults checks values and sets defaults
func (m *MonitorConfig) CheckAndSetDefaults() error {
	if m.Context == nil {
		return trace.BadParameter("missing parameter Context")
	}
	if m.LockWatcher == nil {
		return trace.BadParameter("missing parameter LockWatcher")
	}
	if len(m.LockTargets) == 0 {
		return trace.BadParameter("missing parameter LockTargets")
	}
	if m.Conn == nil {
		return trace.BadParameter("missing parameter Conn")
	}
	if m.Entry == nil {
		return trace.BadParameter("missing parameter Entry")
	}
	if m.Tracker == nil {
		return trace.BadParameter("missing parameter Tracker")
	}
	if m.Emitter == nil {
		return trace.BadParameter("missing parameter Emitter")
	}
	if m.Clock == nil {
		m.Clock = clockwork.NewRealClock()
	}
	return nil
}

// StartMonitor starts a new monitor.
func StartMonitor(cfg MonitorConfig) error {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	w := &Monitor{
		MonitorConfig: cfg,
	}
	// If an applicable lock is already in force, close the connection immediately.
	if lockErr := w.LockWatcher.CheckLockInForce(w.LockingMode, w.LockTargets...); lockErr != nil {
		w.handleLockInForce(lockErr)
		return nil
	}
	lockWatch, err := w.LockWatcher.Subscribe(w.Context, w.LockTargets...)
	if err != nil {
		return trace.Wrap(err)
	}
	go w.start(lockWatch)
	return nil
}

// Monitor monitors the activity on a single connection and disconnects
// that connection if the certificate expires, if a new lock is placed
// that applies to the connection, or after periods of inactivity
type Monitor struct {
	// MonitorConfig is a connection monitor configuration
	MonitorConfig
}

// start starts monitoring connection.
func (w *Monitor) start(lockWatch types.Watcher) {
	lockWatchDoneC := lockWatch.Done()
	defer func() {
		if err := lockWatch.Close(); err != nil {
			w.Entry.WithError(err).Warn("Failed to close lock watcher subscription.")
		}
	}()

	var certTime <-chan time.Time
	if !w.DisconnectExpiredCert.IsZero() {
		discTime := w.DisconnectExpiredCert.Sub(w.Clock.Now().UTC())
		if discTime <= 0 {
			// Client cert is already expired.
			// Disconnect the client immediately.
			w.disconnectClientOnExpiredCert()
			return
		}
		t := w.Clock.NewTicker(discTime)
		defer t.Stop()
		certTime = t.Chan()
	}

	var idleTime <-chan time.Time
	if w.ClientIdleTimeout != 0 {
		idleTime = w.Clock.After(w.ClientIdleTimeout)
	}

	for {
		select {
		// Expired certificate.
		case <-certTime:
			w.disconnectClientOnExpiredCert()
			return

		// Idle timeout.
		case <-idleTime:
			clientLastActive := w.Tracker.GetClientLastActive()
			since := w.Clock.Since(clientLastActive)
			if since >= w.ClientIdleTimeout {
				reason := "client reported no activity"
				if !clientLastActive.IsZero() {
					reason = fmt.Sprintf("client is idle for %v, exceeded idle timeout of %v",
						since, w.ClientIdleTimeout)
				}
				if err := w.emitDisconnectEvent(reason); err != nil {
					w.Entry.WithError(err).Warn("Failed to emit audit event.")
				}
				if w.MessageWriter != nil && w.IdleTimeoutMessage != "" {
					if _, err := w.MessageWriter.WriteString(w.IdleTimeoutMessage); err != nil {
						w.Entry.WithError(err).Warn("Failed to send idle timeout message.")
					}
				}
				w.Entry.Debugf("Disconnecting client: %v", reason)
				if err := w.Conn.Close(); err != nil {
					w.Entry.WithError(err).Error("Failed to close connection.")
				}
				return
			}
			next := w.ClientIdleTimeout - since
			w.Entry.Debugf("Client activity detected %v ago; next check in %v", since, next)
			idleTime = w.Clock.After(next)

		// Lock in force.
		case lockEvent := <-lockWatch.Events():
			var lockErr error
			switch lockEvent.Type {
			case types.OpPut:
				lock, ok := lockEvent.Resource.(types.Lock)
				if !ok {
					w.Entry.Warnf("Skipping unexpected lock event resource type %T.", lockEvent.Resource)
				} else {
					lockErr = services.LockInForceAccessDenied(lock)
				}
			case types.OpDelete:
				// Lock deletion can be ignored.
			case types.OpUnreliable:
				if w.LockingMode == constants.LockingModeStrict {
					lockErr = services.StrictLockingModeAccessDenied
				}
			default:
				w.Entry.Warnf("Skipping unexpected lock event type %q.", lockEvent.Type)
			}
			if lockErr != nil {
				w.handleLockInForce(lockErr)
				return
			}

		case <-lockWatchDoneC:
			w.Entry.WithError(lockWatch.Error()).Warn("Lock watcher subscription was closed.")
			if w.DisconnectExpiredCert.IsZero() && w.ClientIdleTimeout == 0 {
				return
			}
			// Prevent spinning on the zero value received from closed lockWatchDoneC.
			lockWatchDoneC = nil

		case <-w.Context.Done():
			w.Entry.Debugf("Releasing associated resources - context has been closed.")
			return
		}
	}
}

func (w *Monitor) disconnectClientOnExpiredCert() {
	reason := fmt.Sprintf("client certificate expired at %v", w.Clock.Now().UTC())

	w.Entry.Debugf("Disconnecting client: %v", reason)
	if err := w.Conn.Close(); err != nil {
		w.Entry.WithError(err).Error("Failed to close connection.")
	}

	if err := w.emitDisconnectEvent(reason); err != nil {
		w.Entry.WithError(err).Warn("Failed to emit audit event.")
	}
}

func (w *Monitor) emitDisconnectEvent(reason string) error {
	event := &apievents.ClientDisconnect{
		Metadata: apievents.Metadata{
			Type: events.ClientDisconnectEvent,
			Code: events.ClientDisconnectCode,
		},
		UserMetadata: apievents.UserMetadata{
			Login: w.Login,
			User:  w.TeleportUser,
		},
		ConnectionMetadata: apievents.ConnectionMetadata{
			LocalAddr:  w.Conn.LocalAddr().String(),
			RemoteAddr: w.Conn.RemoteAddr().String(),
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerID: w.ServerID,
		},
		Reason: reason,
	}
	return trace.Wrap(w.Emitter.EmitAuditEvent(w.Context, event))
}

func (w *Monitor) handleLockInForce(lockErr error) {
	reason := lockErr.Error()
	if err := w.emitDisconnectEvent(reason); err != nil {
		w.Entry.WithError(err).Warn("Failed to emit audit event.")
	}
	if w.MessageWriter != nil {
		if _, err := w.MessageWriter.WriteString(reason); err != nil {
			w.Entry.WithError(err).Warn("Failed to send lock-in-force message.")
		}
	}
	w.Entry.Debugf("Disconnecting client: %v.", reason)
	if err := w.Conn.Close(); err != nil {
		w.Entry.WithError(err).Error("Failed to close connection.")
	}
}

type trackingChannel struct {
	ssh.Channel
	t ActivityTracker
}

func newTrackingChannel(ch ssh.Channel, t ActivityTracker) ssh.Channel {
	return trackingChannel{
		Channel: ch,
		t:       t,
	}
}

func (ch trackingChannel) Read(buf []byte) (int, error) {
	n, err := ch.Channel.Read(buf)
	ch.t.UpdateClientActivity()
	return n, err
}

func (ch trackingChannel) Write(buf []byte) (int, error) {
	n, err := ch.Channel.Write(buf)
	ch.t.UpdateClientActivity()
	return n, err
}

// TrackingReadConnConfig is a TrackingReadConn configuration.
type TrackingReadConnConfig struct {
	// Conn is a client connection.
	Conn net.Conn
	// Clock is a clock, realtime or fixed in tests.
	Clock clockwork.Clock
	// Context is an external context to cancel the operation.
	Context context.Context
	// Cancel is called whenever client context is closed.
	Cancel context.CancelFunc
}

// CheckAndSetDefaults checks and sets defaults.
func (c *TrackingReadConnConfig) CheckAndSetDefaults() error {
	if c.Conn == nil {
		return trace.BadParameter("missing parameter Conn")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.Context == nil {
		return trace.BadParameter("missing parameter Context")
	}
	if c.Cancel == nil {
		return trace.BadParameter("missing parameter Cancel")
	}
	return nil
}

// NewTrackingReadConn returns a new tracking read connection.
func NewTrackingReadConn(cfg TrackingReadConnConfig) (*TrackingReadConn, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &TrackingReadConn{
		cfg:        cfg,
		mtx:        sync.RWMutex{},
		Conn:       cfg.Conn,
		lastActive: time.Time{},
	}, nil
}

// TrackingReadConn allows to wrap net.Conn and keeps track of the latest conn read activity.
type TrackingReadConn struct {
	cfg TrackingReadConnConfig
	mtx sync.RWMutex
	net.Conn
	lastActive time.Time
}

// Read reads data from the connection.
// Read can be made to time out and return an Error with Timeout() == true
// after a fixed time limit; see SetDeadline and SetReadDeadline.
func (t *TrackingReadConn) Read(b []byte) (int, error) {
	n, err := t.Conn.Read(b)
	t.UpdateClientActivity()
	return n, trace.Wrap(err)
}

func (t *TrackingReadConn) Close() error {
	t.cfg.Cancel()
	return t.Conn.Close()
}

// GetClientLastActive returns time when client was last active
func (t *TrackingReadConn) GetClientLastActive() time.Time {
	t.mtx.RLock()
	defer t.mtx.RUnlock()
	return t.lastActive
}

// UpdateClientActivity sets last recorded client activity
func (t *TrackingReadConn) UpdateClientActivity() {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	t.lastActive = t.cfg.Clock.Now().UTC()
}
