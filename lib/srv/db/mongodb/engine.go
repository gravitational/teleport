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

package mongodb

import (
	"context"
	"net"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/description"
	"go.mongodb.org/mongo-driver/x/mongo/driver"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/srv/db/mongodb/protocol"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log"
)

// NewEngine create new MongoDB engine.
func NewEngine(ec common.EngineConfig) common.Engine {
	return &Engine{
		EngineConfig:   ec,
		maxMessageSize: protocol.DefaultMaxMessageSizeBytes,
	}
}

// Engine implements the MongoDB database service that accepts client
// connections coming over reverse tunnel from the proxy and proxies
// them between the proxy and the MongoDB database instance.
//
// Implements common.Engine.
type Engine struct {
	// EngineConfig is the common database engine configuration.
	common.EngineConfig
	// clientConn is an incoming client connection.
	clientConn net.Conn
	// maxMessageSize is the max message size.
	maxMessageSize uint32
	// serverConnected specifies whether server connection has been created.
	serverConnected bool
}

// InitializeConnection initializes the client connection.
func (e *Engine) InitializeConnection(clientConn net.Conn, _ *common.Session) error {
	e.clientConn = clientConn
	return nil
}

// SendError sends an error to the connected client in MongoDB understandable format.
func (e *Engine) SendError(err error) {
	if err != nil && !utils.IsOKNetworkError(err) {
		e.replyError(e.clientConn, nil, err)
	}
}

// HandleConnection processes the connection from MongoDB proxy coming
// over reverse tunnel.
//
// It handles all necessary startup actions, authorization and acts as a
// middleman between the proxy and the database intercepting and interpreting
// all messages i.e. doing protocol parsing.
func (e *Engine) HandleConnection(ctx context.Context, sessionCtx *common.Session) error {
	observe := common.GetConnectionSetupTimeObserver(sessionCtx.Database)
	// Check that the user has access to the database.
	err := e.authorizeConnection(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err, "error authorizing database access")
	}
	// Automatically create the database user if needed.
	cancelAutoUserLease, err := e.GetUserProvisioner(e).Activate(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		err := e.GetUserProvisioner(e).Teardown(ctx, sessionCtx)
		if err != nil {
			e.Log.ErrorContext(ctx, "Failed to deactivate the user.", "error", err)
		}
	}()
	// Establish connection to the MongoDB server.
	serverConn, closeFn, err := e.connect(ctx, sessionCtx)
	if err != nil {
		cancelAutoUserLease()
		return trace.Wrap(err, "error connecting to the database")
	}
	defer closeFn()

	// Release the auto-users semaphore now that we've successfully connected.
	cancelAutoUserLease()

	e.Audit.OnSessionStart(e.Context, sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, sessionCtx)

	e.serverConnected = true
	observe()

	msgFromClient := common.GetMessagesFromClientMetric(sessionCtx.Database)
	msgFromServer := common.GetMessagesFromServerMetric(sessionCtx.Database)

	// Start reading client messages and sending them to server.
	for {
		clientMessage, err := protocol.ReadMessage(e.clientConn, e.maxMessageSize)
		if err != nil {
			return trace.Wrap(err)
		}
		err = e.handleClientMessage(ctx, sessionCtx, clientMessage, e.clientConn, serverConn, msgFromClient, msgFromServer)
		if err != nil {
			return trace.Wrap(err)
		}
	}
}

// handleClientMessage implements the client message's roundtrip which can go
// down a few different ways:
//  1. If the client's command is not allowed by user's role, we do not pass it
//     to the server and return an error to the client.
//  2. In the most common case, we send client message to the server, read its
//     reply and send it back to the client.
//  3. Some client commands do not receive a reply in which case we just return
//     after sending message to the server and wait for next client message.
//  4. Server can also send multiple messages in a row in which case we exhaust
//     them before returning to listen for next client message.
func (e *Engine) handleClientMessage(ctx context.Context, sessionCtx *common.Session, clientMessage protocol.Message, clientConn net.Conn, serverConn driver.Connection, msgFromClient prometheus.Counter, msgFromServer prometheus.Counter) error {
	msgFromClient.Inc()

	// First check the client command against user's role and log in the audit.
	err := e.authorizeClientMessage(sessionCtx, clientMessage)
	if err != nil {
		return protocol.ReplyError(clientConn, clientMessage, err)
	}
	// If RBAC is ok, pass the message to the server.
	err = serverConn.WriteWireMessage(ctx, clientMessage.GetBytes())
	if err != nil {
		return trace.Wrap(err)
	}
	// Some client messages will not receive a reply.
	if clientMessage.MoreToCome(nil) {
		return nil
	}
	// Otherwise read the server's reply...
	serverMessage, err := protocol.ReadServerMessage(ctx, serverConn, e.maxMessageSize)
	if err != nil {
		return trace.Wrap(err)
	}
	msgFromServer.Inc()

	// Intercept handshake server response to proper configure the engine.
	if protocol.IsHandshake(clientMessage) {
		e.processHandshakeResponse(ctx, serverMessage)
	}

	// ... and pass it back to the client.
	_, err = clientConn.Write(serverMessage.GetBytes())
	if err != nil {
		return trace.Wrap(err)
	}
	// Keep reading if server indicated it has more to send.
	for serverMessage.MoreToCome(clientMessage) {
		serverMessage, err = protocol.ReadServerMessage(ctx, serverConn, e.maxMessageSize)
		if err != nil {
			return trace.Wrap(err)
		}
		msgFromServer.Inc()
		_, err = clientConn.Write(serverMessage.GetBytes())
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// processHandshakeResponse process handshake message and set engine values.
func (e *Engine) processHandshakeResponse(ctx context.Context, respMessage protocol.Message) {
	var rawMessage bson.Raw
	switch resp := respMessage.(type) {
	// OP_REPLY is used on legacy handshake messages (deprecated on MongoDB 5.0)
	case *protocol.MessageOpReply:
		if len(resp.Documents) == 0 {
			e.Log.WarnContext(ctx, "Empty MongoDB handshake response.")
			return
		}

		// Handshake messages are always the first document on a reply.
		rawMessage = bson.Raw(resp.Documents[0])
	// OP_MSG is used on modern handshake messages.
	case *protocol.MessageOpMsg:
		rawMessage = bson.Raw(resp.BodySection.Document)
	default:
		e.Log.WarnContext(ctx, "Unable to process MongoDB handshake response. Unexpected message type.", "message_type", log.TypeAttr(respMessage))
		return
	}

	// Use the description server to parse the handshake message. The address is
	// not validated and won't be used by the engine.
	serverDescription := description.NewServer("", rawMessage)

	// Only overwrite engine configuration if handshake has value set.
	if serverDescription.MaxMessageSize > 0 {
		e.maxMessageSize = serverDescription.MaxMessageSize
	}
}

// authorizeConnection does authorization check for MongoDB connection about
// to be established.
func (e *Engine) authorizeConnection(ctx context.Context, sessionCtx *common.Session) error {
	if err := sessionCtx.CheckUsernameForAutoUserProvisioning(); err != nil {
		return trace.Wrap(err)
	}

	authPref, err := e.Auth.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	state := sessionCtx.GetAccessState(authPref)
	dbRoleMatchers := role.GetDatabaseRoleMatchers(role.RoleMatchersConfig{
		Database:     sessionCtx.Database,
		DatabaseUser: sessionCtx.DatabaseUser,
		// Only the username is checked upon initial connection. MongoDB sends
		// database name with each protocol message (for query, update, etc.) so it
		// is checked when we receive a message from client.
		DisableDatabaseNameMatcher: true,
		AutoCreateUser:             sessionCtx.AutoCreateUserMode.IsEnabled(),
	})
	err = sessionCtx.Checker.CheckAccess(
		sessionCtx.Database,
		state,
		dbRoleMatchers...,
	)
	if err != nil {
		e.Audit.OnSessionStart(e.Context, sessionCtx, err)
		return trace.Wrap(err)
	}
	return nil
}

// authorizeClientMessage checks if the user can run the provided MongoDB command.
//
// Each MongoDB command contains information about the database it's run in
// so we check it against allowed databases in the user's role.
func (e *Engine) authorizeClientMessage(sessionCtx *common.Session, message protocol.Message) error {
	// Each client message should have database information in it.
	database, err := message.GetDatabase()
	if err != nil {
		return trace.Wrap(err)
	}
	err = e.checkClientMessage(sessionCtx, message, database)
	defer e.Audit.OnQuery(e.Context, sessionCtx, common.Query{
		Database: database,
		Query:    message.String(),
		Error:    err,
	})
	return trace.Wrap(err)
}

func (e *Engine) checkClientMessage(sessionCtx *common.Session, message protocol.Message, database string) error {
	// Legacy OP_KILL_CURSORS command doesn't contain database information.
	if _, ok := message.(*protocol.MessageOpKillCursors); ok {
		return sessionCtx.Checker.CheckAccess(sessionCtx.Database,
			services.AccessState{MFAVerified: true},
			services.NewDatabaseUserMatcher(sessionCtx.Database, sessionCtx.DatabaseUser),
		)
	}
	// Do not allow certain commands that deal with authentication.
	command, err := message.GetCommand()
	if err != nil {
		return trace.Wrap(err)
	}
	switch command {
	case "authenticate", "saslStart", "saslContinue", "logout":
		return trace.AccessDenied("access denied")
	}

	// Otherwise authorize the command against allowed databases.
	return sessionCtx.Checker.CheckAccess(
		sessionCtx.Database,
		services.AccessState{MFAVerified: true},
		role.GetDatabaseRoleMatchers(role.RoleMatchersConfig{
			Database:       sessionCtx.Database,
			DatabaseUser:   sessionCtx.DatabaseUser,
			DatabaseName:   database,
			AutoCreateUser: sessionCtx.AutoCreateUserMode.IsEnabled(),
		})...,
	)
}

func (e *Engine) waitForAnyClientMessage(clientConn net.Conn) protocol.Message {
	clientMessage, err := protocol.ReadMessage(clientConn, e.maxMessageSize)
	if err != nil {
		e.Log.WarnContext(e.Context, "Failed to read a message for reply.", "error", err)
	}
	return clientMessage
}

// replyError sends the error to client. It is currently assumed that this
// function will only be called when HandleConnection terminates.
func (e *Engine) replyError(clientConn net.Conn, replyTo protocol.Message, err error) {
	// If an error happens during server connection, wait for a client message
	// before replying to ensure the client can interpret the reply.
	// The first message is usually the isMaster hello message.
	if replyTo == nil && !e.serverConnected {
		waitChan := make(chan protocol.Message, 1)
		go func() {
			waitChan <- e.waitForAnyClientMessage(clientConn)
		}()

		select {
		case clientMessage := <-waitChan:
			replyTo = clientMessage
		case <-e.Clock.After(common.DefaultMongoDBServerSelectionTimeout):
			e.Log.WarnContext(e.Context, "Timed out waiting for client message to reply err.", "error", err)
			// Make sure the connection is closed so waitForAnyClientMessage
			// doesn't get stuck.
			defer clientConn.Close()
		}
	}

	errSend := protocol.ReplyError(clientConn, replyTo, err)
	if errSend != nil {
		e.Log.ErrorContext(e.Context, "Failed to send error message to MongoDB client.", "send_error", errSend, "orig_error", err)
	}
}
