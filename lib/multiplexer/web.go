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

package multiplexer

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	dbcommon "github.com/gravitational/teleport/lib/srv/db/dbutils"
	"github.com/gravitational/teleport/lib/utils"
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
		log:         logrus.WithField(teleport.ComponentKey, "mxweb"),
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
	for {
		conn, err := l.cfg.Listener.Accept()
		if err != nil {
			if utils.IsUseOfClosedNetworkError(err) {
				<-l.context.Done()
				return trace.Wrap(err, "listener is closed")
			}
			select {
			case <-l.context.Done():
				return trace.Wrap(net.ErrClosed, "listener is closed")
			case <-time.After(5 * time.Second):
				l.log.WithError(err).Warn("Backoff on accept error.")
			}
			continue
		}

		tlsConn, ok := conn.(*tls.Conn)
		if !ok {
			l.log.WithFields(logrus.Fields{
				"src_addr": conn.RemoteAddr(),
				"dst_addr": conn.LocalAddr(),
			}).Errorf("Expected *tls.Conn, got %T.", conn)
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

	if err := conn.HandshakeContext(l.context); err != nil {
		if !errors.Is(trace.Unwrap(err), io.EOF) {
			l.log.WithFields(logrus.Fields{
				"src_addr": conn.RemoteAddr(),
				"dst_addr": conn.LocalAddr(),
			}).WithError(err).Warn("Handshake failed.")
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
		l.log.WithFields(logrus.Fields{
			"src_addr": conn.RemoteAddr(),
			"dst_addr": conn.LocalAddr(),
		}).WithError(err).Debug("Failed to check if connection is database connection.")
	}
	if isDatabaseConnection {
		l.dbListener.HandleConnection(l.context, conn)
		return
	}

	l.webListener.HandleConnection(l.context, conn)
}

// Close closes the listener.
//
// Any blocked Accept operations will be unblocked and return errors.
func (l *WebListener) Close() error {
	defer l.cancel()
	return l.cfg.Listener.Close()
}

// Addr returns the listener's network address.
func (l *WebListener) Addr() net.Addr {
	return l.cfg.Listener.Addr()
}
