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
	"net"
	"time"

	"github.com/gravitational/teleport/lib/events"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

// ActivityTracker is a connection activity tracker,
// it allows to update the activity on the connection
// and retrieve the time when the connection was last active
type ActivityTracker interface {
	// GetClientLastActive returns the time of the last recorded activity
	GetClientLastActive() time.Time
	// UpdateClient updates client activity
	UpdateClientActivity()
}

// TrackingConn is an interface representing tracking connection
type TrackingConn interface {
	// LocalAddr returns local addres
	LocalAddr() net.Addr
	// RemoteAddr returns remote address
	RemoteAddr() net.Addr
	// Close closes the connection
	Close() error
}

// MonitorConfig is a wiretap configuration
type MonitorConfig struct {
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
	// Audit is audit log
	Audit events.IAuditLog
	// Entry is a logging entry
	Entry *log.Entry
}

// CheckAndSetDefaults checks values and sets defaults
func (m *MonitorConfig) CheckAndSetDefaults() error {
	if m.Context == nil {
		return trace.BadParameter("missing parameter Context")
	}
	if m.DisconnectExpiredCert.IsZero() && m.ClientIdleTimeout == 0 {
		return trace.BadParameter("either DisconnectExpiredCert or ClientIdleTimeout should be set")
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
	if m.Audit == nil {
		return trace.BadParameter("missing parameter Audit")
	}
	if m.Clock == nil {
		m.Clock = clockwork.NewRealClock()
	}
	return nil
}

// NewMonitor returns a new monitor
func NewMonitor(cfg MonitorConfig) (*Monitor, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Monitor{
		MonitorConfig: cfg,
	}, nil
}

// Monitor moniotiors connection activity
// and disconnects connections with expired certificates
// or with periods of inactivity
type Monitor struct {
	// MonitorConfig is a connection monitori configuration
	MonitorConfig
}

// Start starts monitoring connection
func (w *Monitor) Start() {
	var certTime <-chan time.Time
	if !w.DisconnectExpiredCert.IsZero() {
		t := time.NewTimer(w.DisconnectExpiredCert.Sub(w.Clock.Now().UTC()))
		defer t.Stop()
		certTime = t.C
	}

	var idleTimer *time.Timer
	var idleTime <-chan time.Time
	if w.ClientIdleTimeout != 0 {
		idleTimer = time.NewTimer(w.ClientIdleTimeout)
		idleTime = idleTimer.C
	}

	for {
		select {
		// certificate has expired, disconnect
		case <-certTime:
			event := events.EventFields{
				events.EventType:       events.ClientDisconnectEvent,
				events.EventLogin:      w.Login,
				events.EventUser:       w.TeleportUser,
				events.LocalAddr:       w.Conn.LocalAddr().String(),
				events.RemoteAddr:      w.Conn.RemoteAddr().String(),
				events.SessionServerID: w.ServerID,
				events.Reason:          fmt.Sprintf("client certificate expired at %v", w.Clock.Now().UTC()),
			}
			w.Audit.EmitAuditEvent(events.ClientDisconnect, event)
			w.Entry.Debugf("Disconnecting client: %v", event[events.Reason])
			w.Conn.Close()
			return
		case <-idleTime:
			now := w.Clock.Now().UTC()
			clientLastActive := w.Tracker.GetClientLastActive()
			if now.Sub(clientLastActive) >= w.ClientIdleTimeout {
				event := events.EventFields{
					events.EventLogin:      w.Login,
					events.EventUser:       w.TeleportUser,
					events.LocalAddr:       w.Conn.LocalAddr().String(),
					events.RemoteAddr:      w.Conn.RemoteAddr().String(),
					events.SessionServerID: w.ServerID,
				}
				if clientLastActive.IsZero() {
					event[events.Reason] = "client reported no activity"
				} else {
					event[events.Reason] = fmt.Sprintf("client is idle for %v, exceeded idle timeout of %v",
						now.Sub(clientLastActive), w.ClientIdleTimeout)
				}
				w.Entry.Debugf("Disconnecting client: %v", event[events.Reason])
				w.Audit.EmitAuditEvent(events.ClientDisconnect, event)
				w.Conn.Close()
				return
			}
			w.Entry.Debugf("Next check in %v", w.ClientIdleTimeout-now.Sub(clientLastActive))
			idleTimer = time.NewTimer(w.ClientIdleTimeout - now.Sub(clientLastActive))
			idleTime = idleTimer.C
		case <-w.Context.Done():
			w.Entry.Debugf("Releasing associated resources - context has been closed.")
			return
		}
	}
}
