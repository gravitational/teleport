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
	"strings"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/mongodb/protocol"
	"github.com/gravitational/teleport/lib/utils"

	"go.mongodb.org/mongo-driver/x/mongo/driver"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

// Engine implements the MongoDB database service that accepts client
// connections coming over reverse tunnel from the proxy and proxies
// them between the proxy and the MongoDB database instance.
//
// Implements common.Engine.
type Engine struct {
	// Auth handles database access authentication.
	Auth common.Auth
	// Audit emits database access audit events.
	Audit common.Audit
	// Context is the database server close context.
	Context context.Context
	// Clock is the clock interface.
	Clock clockwork.Clock
	// Log is used for logging.
	Log logrus.FieldLogger
}

// HandleConnection processes the connection from MongoDB proxy coming
// over reverse tunnel.
//
// It handles all necessary startup actions, authorization and acts as a
// middleman between the proxy and the database intercepting and interpreting
// all messages i.e. doing protocol parsing.
func (e *Engine) HandleConnection(ctx context.Context, sessionCtx *common.Session, clientConn net.Conn) (err error) {
	defer func() {
		if err != nil && !utils.IsOKNetworkError(err) {
			e.replyError(clientConn, nil, err)
		}
	}()
	// Check that the user has access to the database.
	err = e.authorizeConnection(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err, "error authorizing database access")
	}
	// Establish connection to the MongoDB server.
	serverConn, err := e.connect(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err, "error connecting to the database")
	}
	defer func() {
		err := serverConn.Close()
		if err != nil {
			e.Log.WithError(err).Error("Failed to close server connection.")
		}
	}()
	e.Audit.OnSessionStart(e.Context, sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, sessionCtx)
	// Start reading client messages and sending them to server.
	for {
		clientMessage, err := protocol.ReadMessage(clientConn)
		if err != nil {
			return trace.Wrap(err)
		}
		err = e.handleClientMessage(ctx, sessionCtx, clientMessage, clientConn, serverConn)
		if err != nil {
			return trace.Wrap(err)
		}
	}
}

// handleClientMessage implements the client message's roundtrip which can go
// down a few different ways:
// 1. If the client's command is not allowed by user's role, we do not pass it
//    to the server and return an error to the client.
// 2. In the most common case, we send client message to the server, read its
//    reply and send it back to the client.
// 3. Some client commands do not receive a reply in which case we just return
//    after sending message to the server and wait for next client message.
// 4. Server can also send multiple messages in a row in which case we exhaust
//    them before returning to listen for next client message.
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
	serverMessage, err := protocol.ReadServerMessage(ctx, serverConn)
	if err != nil {
		return trace.Wrap(err)
	}
	e.Log.Debugf("<=== %v", serverMessage)
	// ... and pass it back to the client.
	_, err = clientConn.Write(serverMessage.GetBytes())
	if err != nil {
		return trace.Wrap(err)
	}
	// Keep reading if server indicated it has more to send.
	for serverMessage.MoreToCome(clientMessage) {
		serverMessage, err = protocol.ReadServerMessage(ctx, serverConn)
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

// authorizeConnection does authorization check for MongoDB connection about
// to be established.
func (e *Engine) authorizeConnection(ctx context.Context, sessionCtx *common.Session) error {
	ap, err := e.Auth.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	mfaParams := services.AccessMFAParams{
		Verified:       sessionCtx.Identity.MFAVerified != "",
		AlwaysRequired: ap.GetRequireSessionMFA(),
	}
	// Only the username is checked upon initial connection. MongoDB sends
	// database name with each protocol message (for query, update, etc.)
	// so it is checked when we receive a message from client.
	err = sessionCtx.Checker.CheckAccessToDatabase(sessionCtx.Database, mfaParams,
		&services.DatabaseLabelsMatcher{Labels: sessionCtx.Database.GetAllLabels()},
		&services.DatabaseUserMatcher{User: sessionCtx.DatabaseUser})
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
	// Mongo uses OP_MSG for most operations now.
	msg, ok := message.(*protocol.MessageOpMsg)
	if !ok {
		return nil
	}
	// Each message has a database information in it.
	database := msg.GetDatabase()
	if database == "" {
		e.Log.Warnf("No database info in message: %v.", message)
		return nil
	}
	err := sessionCtx.Checker.CheckAccessToDatabase(sessionCtx.Database,
		services.AccessMFAParams{Verified: true},
		&services.DatabaseLabelsMatcher{Labels: sessionCtx.Database.GetAllLabels()},
		&services.DatabaseUserMatcher{User: sessionCtx.DatabaseUser},
		&services.DatabaseNameMatcher{Name: database})
	e.Audit.OnQuery(e.Context, sessionCtx, common.Query{
		Database: msg.GetDatabase(),
		// Commands may consist of multiple bson documents.
		Query: strings.Join(msg.GetDocumentsAsStrings(), ", "),
		Error: err,
	})
	return trace.Wrap(err)
}

func (e *Engine) replyError(clientConn net.Conn, replyTo protocol.Message, err error) {
	errSend := protocol.ReplyError(clientConn, replyTo, err)
	if errSend != nil {
		e.Log.WithError(errSend).Errorf("Failed to send error message to MongoDB client: %v.", err)
	}
}
