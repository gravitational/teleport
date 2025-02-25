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

package cassandra

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"io"
	"net"

	"github.com/datastax/go-cassandra-native-protocol/client"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/db/cassandra/protocol"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/utils"
)

// NewEngine create new Cassandra engine.
func NewEngine(ec common.EngineConfig) common.Engine {
	return &Engine{
		EngineConfig: ec,
	}
}

// Engine implements common.Engine.
type Engine struct {
	// EngineConfig is the common database engine configuration.
	common.EngineConfig
	// clientConn is a client connection.
	clientConn *protocol.Conn
	// sessionCtx is current session context.
	sessionCtx *common.Session
	// handshakeTriggered is set to true if handshake was triggered and
	// used to indicated that custom errors should be sent to the client.
	// Cassandra wire protocol relies on streamID to that needs to match the request value
	// so sending a custom error to the client requires reading the first message sent by the client.
	handshakeTriggered bool
}

// SendError send a Cassandra ServerError to  error to the client if handshake was not yet initialized by the client.
// Cassandra wire protocol relies on streamID to that are set by the client and server response needs to
// set the correct streamID in order to get streamID SendError reads a first message send by the client.
func (e *Engine) SendError(sErr error) {
	if utils.IsOKNetworkError(sErr) || sErr == nil {
		return
	}
	e.Log.DebugContext(e.Context, "Cassandra connection error.", "error", sErr)
	// Errors from Cassandra engine can be sent to the client only if handshake is triggered.
	if e.handshakeTriggered {
		return
	}

	eh := failedHandshake{error: sErr}
	if err := eh.handshake(e.clientConn, nil); err != nil {
		e.Log.WarnContext(e.Context, "Cassandra handshake error.", "error", sErr)
	}
}

// InitializeConnection initializes the database connection.
func (e *Engine) InitializeConnection(clientConn net.Conn, sessionCtx *common.Session) error {
	e.clientConn = protocol.NewConn(clientConn)
	e.sessionCtx = sessionCtx
	return nil
}

// HandleConnection processes the connection from Cassandra proxy coming
// over reverse tunnel.
func (e *Engine) HandleConnection(ctx context.Context, sessionCtx *common.Session) error {
	observe := common.GetConnectionSetupTimeObserver(sessionCtx.Database)
	err := e.authorizeConnection(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	serverConn, err := e.connect(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer serverConn.Close()

	e.Audit.OnSessionStart(e.Context, sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, sessionCtx)

	observe()

	if err := e.handshake(sessionCtx, e.clientConn, serverConn); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(e.handleClientServerConn(ctx, e.clientConn, serverConn))
}

func (e *Engine) handleClientServerConn(ctx context.Context, clientConn *protocol.Conn, serverConn net.Conn) error {
	errC := make(chan error, 2)
	go func() {
		err := e.handleClientConnectionWithAudit(clientConn, serverConn)
		errC <- trace.Wrap(err, "client done")
	}()
	go func() {
		err := e.handleServerConnection(serverConn)
		errC <- trace.Wrap(err, "server done")
	}()

	var errs []error
	for i := 0; i < 2; i++ {
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case err := <-errC:
			if err != nil && !utils.IsOKNetworkError(errors.Unwrap(err)) && !errors.Is(err, io.EOF) {
				errs = append(errs, err)
			}
		}
	}
	return trace.NewAggregate(errs...)

}

func (e *Engine) handleClientConnectionWithAudit(clientConn *protocol.Conn, serverConn net.Conn) error {
	defer serverConn.Close()
	msgFromClient := common.GetMessagesFromClientMetric(e.sessionCtx.Database)

	for {
		packet, err := clientConn.ReadPacket()
		if err != nil {
			return trace.Wrap(err)
		}

		msgFromClient.Inc()

		if err := e.processPacket(packet); err != nil {
			return trace.Wrap(err)
		}
		if _, err := serverConn.Write(packet.Raw()); err != nil {
			return trace.Wrap(err)
		}
	}
}

func (e *Engine) handleServerConnection(serverConn net.Conn) error {
	// We cannot increase the db_messages_from_server metric because we pass the data from the server as-is, so we don't know the number of messages transferred.
	defer e.clientConn.Close()
	if _, err := io.Copy(e.clientConn, serverConn); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// validateUsername checks if provided cassandra username matches the database session username.
func validateUsername(ses *common.Session, msg *message.AuthResponse) error {
	var userCredentials client.AuthCredentials
	if err := userCredentials.Unmarshal(msg.Token); err != nil {
		return trace.AccessDenied("invalid credentials format")
	}
	if ses.DatabaseUser != userCredentials.Username {
		return trace.AccessDenied("cassandra user %q doesn't match db session username %q  ", userCredentials.Username, ses.DatabaseUser)
	}
	return nil
}

func (e *Engine) processPacket(packet *protocol.Packet) error {
	body := packet.FrameBody()
	switch msg := body.Message.(type) {
	case *message.Options:
		// Cassandra client sends options message to the server to negotiate protocol version.
		// Skip audit for this message.
	case *message.Startup:
		// Startup message is sent by the client to initialize the cassandra handshake.
		// Skip audit for this message.
	case *message.AuthResponse:
		if err := validateUsername(e.sessionCtx, msg); err != nil {
			return trace.Wrap(err)
		}
	case *message.Query:
		e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{
			Query: msg.String(),
		})
	case *message.Prepare:
		e.Audit.EmitEvent(e.Context, &events.CassandraPrepare{
			Metadata: common.MakeEventMetadata(e.sessionCtx,
				libevents.DatabaseSessionCassandraPrepareEvent,
				libevents.CassandraPrepareEventCode,
			),
			UserMetadata:     common.MakeUserMetadata(e.sessionCtx),
			SessionMetadata:  common.MakeSessionMetadata(e.sessionCtx),
			DatabaseMetadata: common.MakeDatabaseMetadata(e.sessionCtx),
			Query:            msg.Query,
			Keyspace:         msg.Keyspace,
		})
	case *message.Execute:
		e.Audit.EmitEvent(e.Context, &events.CassandraExecute{
			Metadata: common.MakeEventMetadata(e.sessionCtx,
				libevents.DatabaseSessionCassandraExecuteEvent,
				libevents.CassandraExecuteEventCode,
			),
			UserMetadata:     common.MakeUserMetadata(e.sessionCtx),
			SessionMetadata:  common.MakeSessionMetadata(e.sessionCtx),
			DatabaseMetadata: common.MakeDatabaseMetadata(e.sessionCtx),
			QueryId:          hex.EncodeToString(msg.QueryId),
		})
	case *message.Batch:
		e.Audit.EmitEvent(e.Context, &events.CassandraBatch{
			Metadata: common.MakeEventMetadata(e.sessionCtx,
				libevents.DatabaseSessionCassandraBatchEvent,
				libevents.CassandraBatchEventCode,
			),
			UserMetadata:     common.MakeUserMetadata(e.sessionCtx),
			SessionMetadata:  common.MakeSessionMetadata(e.sessionCtx),
			DatabaseMetadata: common.MakeDatabaseMetadata(e.sessionCtx),
			Consistency:      msg.Consistency.String(),
			Keyspace:         msg.Keyspace,
			BatchType:        msg.Type.String(),
			Children:         batchChildToProto(msg.Children),
		})
	case *message.Register:
		e.Audit.EmitEvent(e.Context, &events.CassandraRegister{
			Metadata: common.MakeEventMetadata(e.sessionCtx,
				libevents.DatabaseSessionCassandraRegisterEvent,
				libevents.CassandraRegisterEventCode,
			),
			UserMetadata:     common.MakeUserMetadata(e.sessionCtx),
			SessionMetadata:  common.MakeSessionMetadata(e.sessionCtx),
			DatabaseMetadata: common.MakeDatabaseMetadata(e.sessionCtx),
			EventTypes:       eventTypesToString(msg.EventTypes),
		})
	case *message.Revise:
		// Revise message is support by DSE (DataStax Enterprise) only.
		// Skip audit for this message.
		e.Log.DebugContext(e.Context, "Skip audit for revise message.", "message", msg)
	default:
		return trace.BadParameter("received a message with unexpected type %T", body.Message)
	}

	return nil
}

// authorizeConnection does authorization check for Cassandra connection about
// to be established.
func (e *Engine) authorizeConnection(ctx context.Context) error {
	authPref, err := e.Auth.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	state := e.sessionCtx.GetAccessState(authPref)

	dbRoleMatchers := role.GetDatabaseRoleMatchers(role.RoleMatchersConfig{
		Database:     e.sessionCtx.Database,
		DatabaseUser: e.sessionCtx.DatabaseUser,
		DatabaseName: e.sessionCtx.DatabaseName,
	})
	err = e.sessionCtx.Checker.CheckAccess(
		e.sessionCtx.Database,
		state,
		dbRoleMatchers...,
	)
	if err != nil {
		e.Audit.OnSessionStart(e.Context, e.sessionCtx, err)
		return trace.Wrap(err)
	}
	return nil
}

func (e *Engine) connect(ctx context.Context, sessionCtx *common.Session) (*protocol.Conn, error) {
	config, err := e.Auth.GetTLSConfig(ctx, sessionCtx.GetExpiry(), sessionCtx.Database, sessionCtx.DatabaseUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsDialer := tls.Dialer{Config: config}
	serverConn, err := tlsDialer.DialContext(ctx, "tcp", sessionCtx.Database.GetURI())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return protocol.NewConn(serverConn), nil
}

func (e *Engine) handshake(sessionCtx *common.Session, clientConn, serverConn *protocol.Conn) error {
	auth, err := e.getAuth(sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	e.handshakeTriggered = true
	return auth.handleHandshake(e.Context, clientConn, serverConn)
}

func (e *Engine) getAuth(sessionCtx *common.Session) (handshakeHandler, error) {
	switch {
	case sessionCtx.Database.IsAWSHosted():
		return &authAWSSigV4Auth{
			ses:       sessionCtx,
			awsConfig: e.AWSConfigProvider,
		}, nil
	default:
		return &basicHandshake{ses: sessionCtx}, nil
	}
}
