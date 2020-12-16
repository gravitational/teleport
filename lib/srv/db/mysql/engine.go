/*
Copyright 2020 Gravitational, Inc.

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

package mysql

import (
	"context"
	"net"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/db/auth"
	"github.com/gravitational/teleport/lib/srv/db/mysql/protocol"
	"github.com/gravitational/teleport/lib/srv/db/session"

	"github.com/siddontang/go-mysql/client"
	"github.com/siddontang/go-mysql/mysql"
	"github.com/siddontang/go-mysql/packet"
	"github.com/siddontang/go-mysql/server"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

// Engine implements the MySQL database service that accepts client
// connections coming over reverse tunnel from the proxy and proxies
// them between the proxy and the MySQL database instance.
//
// Implements db.DatabaseEngine.
type Engine struct {
	// Auth handles database access authentication.
	Auth *auth.Authenticator
	// StreamWriter is the async audit logger.
	StreamWriter events.StreamWriter
	// OnSessionStart is called upon successful connection to the database.
	OnSessionStart func(session.Context, error) error
	// OnSessionEnd is called upon disconnection from the database.
	OnSessionEnd func(session.Context) error
	// OnQuery is called when an SQL query is executed on the connection.
	OnQuery func(session.Context, string) error
	// Clock is the clock interface.
	Clock clockwork.Clock
	// Log is used for logging.
	Log logrus.FieldLogger
}

// HandleConnection processes the connection from MySQL proxy coming
// over reverse tunnel.
//
// It handles all necessary startup actions, authorization and acts as a
// middleman between the proxy and the database intercepting and interpreting
// all messages i.e. doing protocol parsing.
func (e *Engine) HandleConnection(ctx context.Context, sessionCtx *session.Context, clientConn net.Conn) (err error) {
	// Make server conn to get access to protocol's WriteOK/WriteError methods.
	proxyConn := server.Conn{Conn: packet.NewConn(clientConn)}
	defer func() {
		if err != nil {
			if err := proxyConn.WriteError(err); err != nil {
				e.Log.WithError(err).Error("Failed to send error to client.")
			}
		}
	}()
	// Perform authorization checks.
	err = e.checkAccess(sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	// Establish connection to the MySQL server.
	serverConn, err := e.connect(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		err := serverConn.Close()
		if err != nil {
			e.Log.WithError(err).Error("Failed to close connection to MySQL server.")
		}
	}()
	// Send back OK packet to indicate auth/connect success. At this point
	// the original client should consider the connection phase completed.
	err = proxyConn.WriteOK(nil)
	if err != nil {
		return trace.Wrap(err)
	}
	err = e.OnSessionStart(*sessionCtx, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		err := e.OnSessionEnd(*sessionCtx)
		if err != nil {
			e.Log.WithError(err).Error("Failed to emit audit event.")
		}
	}()
	// Copy between the connections.
	clientErrCh := make(chan error, 1)
	serverErrCh := make(chan error, 1)
	go e.receiveFromClient(clientConn, serverConn, clientErrCh, sessionCtx)
	go e.receiveFromServer(serverConn, clientConn, serverErrCh)
	select {
	case err := <-clientErrCh:
		e.Log.WithError(err).Debug("Client done.")
	case err := <-serverErrCh:
		e.Log.WithError(err).Debug("Server done.")
	case <-ctx.Done():
		e.Log.Debug("Context canceled.")
	}
	return nil
}

func (e *Engine) checkAccess(sessionCtx *session.Context) error {
	err := sessionCtx.Checker.CheckAccessToDatabase(sessionCtx.Server,
		sessionCtx.DatabaseName, sessionCtx.DatabaseUser)
	if err != nil {
		if err := e.OnSessionStart(*sessionCtx, err); err != nil {
			e.Log.WithError(err).Error("Failed to emit audit event.")
		}
		return trace.Wrap(err)
	}
	return nil
}

func (e *Engine) connect(ctx context.Context, sessionCtx *session.Context) (*client.Conn, error) {
	tlsConfig, err := e.Auth.GetTLSConfig(ctx, sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var password string
	if sessionCtx.Server.IsAWS() {
		password, err = e.Auth.GetAWSAuthToken(sessionCtx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	conn, err := client.Connect(sessionCtx.Server.GetURI(),
		sessionCtx.DatabaseUser,
		password,
		sessionCtx.DatabaseName,
		func(conn *client.Conn) {
			conn.SetTLSConfig(tlsConfig)
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return conn, nil
}

func (e *Engine) receiveFromClient(clientConn, serverConn net.Conn, clientErrCh chan<- error, sessionCtx *session.Context) {
	log := e.Log.WithField("from", "client")
	defer log.Debug("Stop receiving from client.")
	for {
		packet, err := protocol.ReadPacket(clientConn)
		if err != nil {
			log.WithError(err).Error("Failed to read client packet.")
			clientErrCh <- err
			return
		}
		log.Debugf("Client packet: %s.", packet)
		switch packet[4] {
		case mysql.COM_QUERY:
			err := e.OnQuery(*sessionCtx, string(packet[5:]))
			if err != nil {
				log.WithError(err).Error("Failed to emit audit event.")
			}
		case mysql.COM_QUIT:
			clientErrCh <- nil
			return
		}
		_, err = protocol.WritePacket(packet, serverConn)
		if err != nil {
			log.WithError(err).Error("Failed to write server packet.")
			clientErrCh <- err
			return
		}
	}
}

func (e *Engine) receiveFromServer(serverConn, clientConn net.Conn, serverErrCh chan<- error) {
	log := e.Log.WithField("from", "server")
	defer log.Debug("Stop receiving from server.")
	for {
		packet, err := protocol.ReadPacket(serverConn)
		if err != nil {
			if strings.Contains(err.Error(), teleport.UseOfClosedNetworkConnection) {
				log.Debug("Server connection closed.")
				serverErrCh <- nil
				return
			}
			log.WithError(err).Error("Failed to read server packet.")
			serverErrCh <- err
			return
		}
		log.Debugf("Server packet: %s.", packet)
		_, err = protocol.WritePacket(packet, clientConn)
		if err != nil {
			log.WithError(err).Error("Failed to write client packet.")
			serverErrCh <- err
			return
		}
	}
}
