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
	"io"
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
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

// ConnectionMonitorConfig contains dependencies required by
// the ConnectionMonitor.
type ConnectionMonitorConfig struct {
	// AccessPoint is used to retrieve cluster configuration.
	AccessPoint AccessPoint
	// LockWatcher ensures lock information is up to date.
	LockWatcher *services.LockWatcher
	// Clock is a clock, realtime or fixed in tests.
	Clock clockwork.Clock
	// ServerID is the host UUID of the server receiving connections.
	ServerID string
	// Emitter allows events to be emitted.
	Emitter apievents.Emitter
	// EmitterContext is long-lived context suitable to be used with Emitter
	EmitterContext context.Context
	// Logger is a logging entry.
	Logger log.FieldLogger
	// MonitorCloseChannel will be signaled when the monitor closes a connection.
	// Used only for testing. Optional.
	MonitorCloseChannel chan struct{}
}

// CheckAndSetDefaults checks values and sets defaults
func (c *ConnectionMonitorConfig) CheckAndSetDefaults() error {
	if c.AccessPoint == nil {
		return trace.BadParameter("missing parameter AccessPoint")
	}
	if c.LockWatcher == nil {
		return trace.BadParameter("missing parameter LockWatcher")
	}
	if c.Logger == nil {
		return trace.BadParameter("missing parameter Logger")
	}
	if c.Emitter == nil {
		return trace.BadParameter("missing parameter Emitter")
	}
	if c.EmitterContext == nil {
		return trace.BadParameter("missing parameter EmitterContext")
	}
	if c.ServerID == "" {
		return trace.BadParameter("missing parameter ServerID")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

// ConnectionMonitor monitors the activity of connections and disconnects
// them if the certificate expires, if a new lock is placed
// that applies to the connection, or after periods of inactivity
type ConnectionMonitor struct {
	cfg ConnectionMonitorConfig
}

// NewConnectionMonitor returns a ConnectionMonitor that can be used to monitor
// connection activity and terminate connections based on various cluster conditions.
func NewConnectionMonitor(cfg ConnectionMonitorConfig) (*ConnectionMonitor, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &ConnectionMonitor{cfg: cfg}, nil
}

func getTrackingReadConn(conn net.Conn) (*TrackingReadConn, bool) {
	type netConn interface {
		NetConn() net.Conn
	}

	for {
		if tconn, ok := conn.(*TrackingReadConn); ok {
			return tconn, true
		}

		connGetter, ok := conn.(netConn)
		if !ok {
			return nil, false
		}
		conn = connGetter.NetConn()
	}
}

// MonitorConn ensures that the provided [net.Conn] is allowed per cluster configuration
// and security controls. If at any point during the lifetime of the connection the
// cluster controls dictate that the connection is not permitted it will be closed and the
// returned [context.Context] will be canceled.
func (c *ConnectionMonitor) MonitorConn(ctx context.Context, authzCtx *authz.Context, conn net.Conn) (context.Context, net.Conn, error) {
	authPref, err := c.cfg.AccessPoint.GetAuthPreference(ctx)
	if err != nil {
		return ctx, conn, trace.Wrap(err)
	}
	netConfig, err := c.cfg.AccessPoint.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return ctx, conn, trace.Wrap(err)
	}

	identity := authzCtx.Identity.GetIdentity()
	checker := authzCtx.Checker

	idleTimeout := checker.AdjustClientIdleTimeout(netConfig.GetClientIdleTimeout())

	tconn, ok := getTrackingReadConn(conn)
	if !ok {
		tctx, cancel := context.WithCancelCause(ctx)
		tconn, err = NewTrackingReadConn(TrackingReadConnConfig{
			Conn:    conn,
			Clock:   c.cfg.Clock,
			Context: tctx,
			Cancel:  cancel,
		})
		if err != nil {
			return ctx, conn, trace.Wrap(err)
		}
	}

	// Start monitoring client connection. When client connection is closed the monitor goroutine exits.
	if err := StartMonitor(MonitorConfig{
		LockWatcher:           c.cfg.LockWatcher,
		LockTargets:           authzCtx.LockTargets(),
		LockingMode:           authzCtx.Checker.LockingMode(authPref.GetLockingMode()),
		DisconnectExpiredCert: authzCtx.GetDisconnectCertExpiry(authPref),
		ClientIdleTimeout:     idleTimeout,
		Conn:                  tconn,
		Tracker:               tconn,
		Context:               ctx,
		Clock:                 c.cfg.Clock,
		ServerID:              c.cfg.ServerID,
		TeleportUser:          identity.Username,
		Emitter:               c.cfg.Emitter,
		EmitterContext:        c.cfg.EmitterContext,
		Entry:                 c.cfg.Logger,
		IdleTimeoutMessage:    netConfig.GetClientIdleTimeoutMessage(),
		MonitorCloseChannel:   c.cfg.MonitorCloseChannel,
	}); err != nil {
		return ctx, conn, trace.Wrap(err)
	}

	return tconn.cfg.Context, tconn, nil
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
	// Context is an external context. To reliably close the monitor and ensure no goroutine leak,
	// make sure to pass a context which will be canceled on time.
	Context context.Context
	// Login is linux box login
	Login string
	// TeleportUser is a teleport user name
	TeleportUser string
	// ServerID is a session server ID
	ServerID string
	// Emitter is events emitter
	Emitter apievents.Emitter
	// EmitterContext is long-lived context suitable to be used with Emitter. Typically, a server exit context will be used here.
	EmitterContext context.Context
	// Entry is a logging entry
	Entry log.FieldLogger
	// IdleTimeoutMessage is sent to the client when the idle timeout expires.
	IdleTimeoutMessage string
	// CertificateExpiredMessage is sent to the client when the certificate expires.
	CertificateExpiredMessage string
	// MessageWriter wraps a channel to send text messages to the client. Use
	// for disconnection messages, etc.
	MessageWriter io.StringWriter
	// MonitorCloseChannel will be signaled when the monitor closes a connection.
	// Used only for testing. Optional.
	MonitorCloseChannel chan struct{}
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
	if m.EmitterContext == nil {
		return trace.BadParameter("missing parameter EmitterContext")
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
	go func() {
		w.start(lockWatch)
		if w.MonitorCloseChannel != nil {
			// Non blocking send to the close channel.
			select {
			case w.MonitorCloseChannel <- struct{}{}:
			default:
			}
		}
	}()
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
				reason := "Client reported no activity"
				if !clientLastActive.IsZero() {
					reason = fmt.Sprintf("Client exceeded idle timeout of %v", w.ClientIdleTimeout)
				}
				if w.MessageWriter != nil {
					msg := w.IdleTimeoutMessage
					if msg == "" {
						msg = reason
					}
					if _, err := w.MessageWriter.WriteString(msg); err != nil {
						w.Entry.WithError(err).Warn("Failed to send idle timeout message.")
					}
				}
				w.disconnectClient(reason)
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
			return
		}
	}
}

func (w *Monitor) disconnectClientOnExpiredCert() {
	reason := fmt.Sprintf("client certificate expired at %v", w.Clock.Now().UTC())
	if w.MessageWriter != nil {
		msg := w.CertificateExpiredMessage
		if msg == "" {
			msg = reason
		}
		if _, err := w.MessageWriter.WriteString(msg); err != nil {
			w.Entry.WithError(err).Warn("Failed to send certificate expiration message")
		}
	}
	w.disconnectClient(reason)
}

type withCauseCloser interface {
	CloseWithCause(cause error) error
}

func (w *Monitor) disconnectClient(reason string) {
	w.Entry.Debugf("Disconnecting client: %v", reason)

	if connWithCauseCloser, ok := w.Conn.(withCauseCloser); ok {
		if err := connWithCauseCloser.CloseWithCause(trace.AccessDenied(reason)); err != nil {
			w.Entry.WithError(err).Error("Failed to close connection.")
		}
	} else {
		if err := w.Conn.Close(); err != nil {
			w.Entry.WithError(err).Error("Failed to close connection.")
		}
	}

	// emit audit event after client has been disconnected.
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
			ServerVersion: teleport.Version,
			ServerID:      w.ServerID,
		},
		Reason: reason,
	}
	return trace.Wrap(w.Emitter.EmitAuditEvent(w.EmitterContext, event))
}

func (w *Monitor) handleLockInForce(lockErr error) {
	reason := lockErr.Error()
	if w.MessageWriter != nil {
		if _, err := w.MessageWriter.WriteString(reason); err != nil {
			w.Entry.WithError(err).Warn("Failed to send lock-in-force message.")
		}
	}
	w.disconnectClient(reason)
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
	Cancel context.CancelCauseFunc
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

	// This has to use the original error type or else utilities using the connection
	// (like io.Copy, which is used by the oxy forwarder) may incorrectly categorize
	// the error produced by this and terminate the connection unnecessarily.
	return n, err
}

// Close cancels the context with io.EOF and closes the underlying connection.
func (t *TrackingReadConn) Close() error {
	t.cfg.Cancel(io.EOF)
	return t.Conn.Close()
}

// CloseWithCause cancels the context with provided cause and closes the
// underlying connection.
func (t *TrackingReadConn) CloseWithCause(cause error) error {
	t.cfg.Cancel(cause)
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

// See GetDisconnectExpiredCertFromIdentity
func getDisconnectExpiredCertFromIdentityContext(
	checker services.AccessChecker,
	authPref types.AuthPreference,
	identity *IdentityContext,
) time.Time {
	// In the case where both disconnect_expired_cert and require_session_mfa are enabled,
	// the PreviousIdentityExpires value of the certificate will be used, which is the
	// expiry of the certificate used to issue the short lived MFA verified certificate.
	//
	// See https://github.com/gravitational/teleport/issues/18544

	// If the session doesn't need to be disconnected on cert expiry just return the default value.
	if !checker.AdjustDisconnectExpiredCert(authPref.GetDisconnectExpiredCert()) {
		return time.Time{}
	}

	if !identity.PreviousIdentityExpires.IsZero() {
		// If this is a short-lived mfa verified cert, return the certificate extension
		// that holds its' issuing cert's expiry value.
		return identity.PreviousIdentityExpires
	}

	// Otherwise just return the current cert's expiration
	return identity.CertValidBefore
}
