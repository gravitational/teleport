/*
Copyright 2021 Gravitational, Inc.

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

package multiplexer

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/defaults"
	dbcommon "github.com/gravitational/teleport/lib/srv/db/dbutils"
)

// WebListenerConfig is the web listener configuration.
type WebListenerConfig struct {
	// Listener is the listener that accepts tls connections.
	Listener net.Listener
	// ReadDeadline is a connection read deadline during the TLS handshake.
	ReadDeadline time.Duration
	// Clock is a clock to override in tests.
	Clock clockwork.Clock
}

// CheckAndSetDefaults verifies configuration and sets defaults.
func (c *WebListenerConfig) CheckAndSetDefaults() error {
	if c.Listener == nil {
		return trace.BadParameter("missing parameter Listener")
	}
	if c.ReadDeadline == 0 {
		c.ReadDeadline = defaults.HandshakeReadDeadline
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

// NewWebListener returns a new web listener.
func NewWebListener(cfg WebListenerConfig) (*WebListener, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	context, cancel := context.WithCancel(context.Background())
	return &WebListener{
		log:         logrus.WithField(trace.Component, "mxweb"),
		cfg:         cfg,
		webListener: newListener(context, cfg.Listener.Addr()),
		dbListener:  newListener(context, cfg.Listener.Addr()),
		cancel:      cancel,
		context:     context,
	}, nil
}

// WebListener multiplexes tls connections between web and database listeners
// based on the client certificate.
type WebListener struct {
	log         logrus.FieldLogger
	cfg         WebListenerConfig
	webListener *Listener
	dbListener  *Listener
	cancel      context.CancelFunc
	context     context.Context
	isClosed    int32
}

// Web returns web listener.
func (l *WebListener) Web() net.Listener {
	return l.webListener
}

// DB returns database access listener.
func (l *WebListener) DB() net.Listener {
	return l.dbListener
}

// Serve starts accepting and forwarding tls connections to appropriate listeners.
func (l *WebListener) Serve() error {
	backoffTimer := time.NewTicker(5 * time.Second)
	defer backoffTimer.Stop()
	for {
		conn, err := l.cfg.Listener.Accept()
		if err != nil {
			if atomic.LoadInt32(&l.isClosed) == 1 {
				return trace.ConnectionProblem(nil, "listener is closed")
			}
			select {
			case <-backoffTimer.C:
				l.log.WithError(err).Warn("Backoff on network error.")
			case <-l.context.Done():
				return trace.ConnectionProblem(nil, "listener is closed")
			}
			continue
		}

		tlsConn, ok := conn.(*tls.Conn)
		if !ok {
			l.log.Errorf("Expected *tls.Conn, got %T.", conn)
			conn.Close()
			continue
		}

		go l.detectAndForward(tlsConn)
	}
}

func (l *WebListener) detectAndForward(conn *tls.Conn) {
	err := conn.SetReadDeadline(l.cfg.Clock.Now().Add(l.cfg.ReadDeadline))
	if err != nil {
		l.log.WithError(err).Warn("Failed to set connection read deadline.")
		conn.Close()
		return
	}

	if err := conn.Handshake(); err != nil {
		if trace.Unwrap(err) != io.EOF {
			l.log.WithError(err).Warn("Handshake failed.")
		}
		conn.Close()
		return
	}

	err = conn.SetReadDeadline(time.Time{})
	if err != nil {
		l.log.WithError(err).Warn("Failed to reset connection read deadline")
		conn.Close()
		return
	}

	// Inspect the client certificate (if there's any) and forward the
	// connection either to database access listener if identity encoded
	// in the cert indicates this is a database connection, or to a regular
	// tls listener.
	isDatabaseConnection, err := dbcommon.IsDatabaseConnection(conn.ConnectionState())
	if err != nil {
		l.log.WithError(err).Debug("Failed to check if connection is database connection.")
	}
	if isDatabaseConnection {
		select {
		case l.dbListener.connC <- conn:
		case <-l.context.Done():
			conn.Close()
		}
		return
	}

	select {
	case l.webListener.connC <- conn:
	case <-l.context.Done():
		conn.Close()
		return
	}
}

// Close closes the listener.
//
// Any blocked Accept operations will be unblocked and return errors.
func (l *WebListener) Close() error {
	defer l.cancel()
	atomic.StoreInt32(&l.isClosed, 1)
	return l.cfg.Listener.Close()
}

// Addr returns the listener's network address.
func (l *WebListener) Addr() net.Addr {
	return l.cfg.Listener.Addr()
}
