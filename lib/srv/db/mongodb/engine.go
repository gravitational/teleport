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

package mongodb

import (
	"context"
	"net"

	"github.com/gravitational/trace"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/description"
	"go.mongodb.org/mongo-driver/x/mongo/driver"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/srv/db/mongodb/protocol"
	"github.com/gravitational/teleport/lib/utils"
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
	// Check that the user has access to the database.
	err := e.authorizeConnection(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err, "error authorizing database access")
	}
	// Establish connection to the MongoDB server.
	serverConn, closeFn, err := e.connect(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err, "error connecting to the database")
	}
	defer closeFn()
	e.Audit.OnSessionStart(e.Context, sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, sessionCtx)
	// Start reading client messages and sending them to server.
	for {
		clientMessage, err := protocol.ReadMessage(e.clientConn, e.maxMessageSize)
		if err != nil {
			return trace.Wrap(err)
		}
		err = e.handleClientMessage(ctx, sessionCtx, clientMessage, e.clientConn, serverConn)
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
func (e *Engine) handleClientMessage(ctx context.Context, sessionCtx *common.Session, clientMessage protocol.Message, clientConn net.Conn, serverConn driver.Connection) error {
	e.Log.Debugf("===> %v", clientMessage)
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
	e.Log.Debugf("<=== %v", serverMessage)

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
		e.Log.Debugf("<=== %v", serverMessage)
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
			e.Log.Warn("Empty MongoDB handshake response.")
			return
		}

		// Handshake messages are always the first document on a reply.
		rawMessage = bson.Raw(resp.Documents[0])
	// OP_MSG is used on modern handshake messages.
	case *protocol.MessageOpMsg:
		rawMessage = bson.Raw(resp.BodySection.Document)
	default:
		e.Log.Warn("Unabled to process MongoDB handshake response. Unexpected message type %T", respMessage)
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
	authPref, err := e.Auth.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	state := sessionCtx.GetAccessState(authPref)
	// Only the username is checked upon initial connection. MongoDB sends
	// database name with each protocol message (for query, update, etc.)
	// so it is checked when we receive a message from client.
	err = sessionCtx.Checker.CheckAccess(
		sessionCtx.Database,
		state,
		services.NewDatabaseUserMatcher(sessionCtx.Database, sessionCtx.DatabaseUser),
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
	return sessionCtx.Checker.CheckAccess(sessionCtx.Database,
		services.AccessState{MFAVerified: true},
		role.DatabaseRoleMatchers(
			sessionCtx.Database,
			sessionCtx.DatabaseUser,
			database)...)
}

func (e *Engine) replyError(clientConn net.Conn, replyTo protocol.Message, err error) {
	errSend := protocol.ReplyError(clientConn, replyTo, err)
	if errSend != nil {
		e.Log.WithError(errSend).Errorf("Failed to send error message to MongoDB client: %v.", err)
	}
}
