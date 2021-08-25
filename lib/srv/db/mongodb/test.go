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
	"crypto/tls"
	"net"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/mongodb/protocol"
	"github.com/gravitational/teleport/lib/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// MakeTestClient returns MongoDB client connection according to the provided
// parameters.
func MakeTestClient(ctx context.Context, config common.TestClientConfig) (*mongo.Client, error) {
	tlsConfig, err := common.MakeTestClientTLSConfig(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client, err := mongo.Connect(ctx, options.Client().
		ApplyURI("mongodb://"+config.Address).
		SetTLSConfig(tlsConfig).
		// Mongo client connects in background so set a short server selection
		// timeout so access errors are returned to the client quicker.
		SetServerSelectionTimeout(5*time.Second))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Mongo client connects in background so do a ping to make sure it
	// can connect successfully.
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client, nil
}

// TestServer is a test MongoDB server used in functional database access tests.
type TestServer struct {
	cfg      common.TestServerConfig
	listener net.Listener
	port     string
	log      logrus.FieldLogger
}

// NewTestServer returns a new instance of a test MongoDB server.
func NewTestServer(config common.TestServerConfig) (*TestServer, error) {
	address := "localhost:0"
	if config.Address != "" {
		address = config.Address
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig, err := common.MakeTestServerTLSConfig(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := logrus.WithFields(logrus.Fields{
		trace.Component: defaults.ProtocolMongoDB,
		"name":          config.Name,
	})
	return &TestServer{
		cfg: config,
		// MongoDB uses regular TLS handshake so standard TLS listener will work.
		listener: tls.NewListener(listener, tlsConfig),
		port:     port,
		log:      log,
	}, nil
}

// Serve starts serving client connections.
func (s *TestServer) Serve() error {
	s.log.Debugf("Starting test MongoDB server on %v.", s.listener.Addr())
	defer s.log.Debug("Test MongoDB server stopped.")
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if utils.IsOKNetworkError(err) {
				return nil
			}
			s.log.WithError(err).Error("Failed to accept connection.")
			continue
		}
		s.log.Debug("Accepted connection.")
		go func() {
			defer s.log.Debug("Connection done.")
			defer conn.Close()
			if err := s.handleConnection(conn); err != nil {
				if !utils.IsOKNetworkError(err) {
					s.log.Errorf("Failed to handle connection: %v.",
						trace.DebugReport(err))
				}
			}
		}()
	}
}

// handleConnection receives Mongo wire messages from the client connection
// and sends back the response messages.
func (s *TestServer) handleConnection(conn net.Conn) error {
	// Read client messages and reply to them - test server supports a very
	// basic set of commands: "isMaster", "authenticate", "ping" and "find".
	for {
		message, err := protocol.ReadMessage(conn)
		if err != nil {
			return trace.Wrap(err)
		}
		reply, err := s.handleMessage(message)
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = conn.Write(reply.ToWire(message.GetHeader().RequestID))
		if err != nil {
			return trace.Wrap(err)
		}
	}
}

// handleMessage makes response for the provided command received from client.
func (s *TestServer) handleMessage(message protocol.Message) (protocol.Message, error) {
	switch m := message.(type) {
	case *protocol.MessageOpQuery:
		switch getCommandType(m.Query) {
		case commandIsMaster: // isMaster command can come as both OP_QUERY and OP_MSG
			return s.handleIsMaster(message)
		case commandAuth:
			return s.handleAuth(m)
		}
	case *protocol.MessageOpMsg:
		document, err := m.GetDocument()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		switch getCommandType(document) {
		case commandIsMaster: // isMaster command can come as both OP_QUERY and OP_MSG
			return s.handleIsMaster(m)
		case commandPing:
			return s.handlePing(m)
		case commandFind:
			return s.handleFind(m)
		}
	}
	return nil, trace.NotImplemented("unsupported message: %v", message)
}

// handleAuth makes response to the client's "authenticate" command.
func (s *TestServer) handleAuth(message protocol.Message) (protocol.Message, error) {
	authCmd, ok := message.(*protocol.MessageOpQuery)
	if !ok {
		return nil, trace.BadParameter("expected OP_MSG message, got: %v", message)
	}
	s.log.Debugf("Authenticate: %v.", authCmd.Query.String())
	if getCommandType(authCmd.Query) != commandAuth {
		return nil, trace.BadParameter("expected authenticate command, got: %v", authCmd.Query)
	}
	authReply, err := makeOKReply()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return protocol.MakeOpMsg(authReply), nil
}

// handleIsMaster makes response to the client's "isMaster" command.
//
// isMaster command is used as a handshake by the client to determine the
// cluster topology.
func (s *TestServer) handleIsMaster(message protocol.Message) (protocol.Message, error) {
	isMasterReply, err := makeIsMasterReply()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch message.(type) {
	case *protocol.MessageOpQuery:
		return protocol.MakeOpReply(isMasterReply), nil
	case *protocol.MessageOpMsg:
		return protocol.MakeOpMsg(isMasterReply), nil
	}
	return nil, trace.NotImplemented("unsupported message: %v", message)
}

// handlePing makes response to the client's "ping" command.
//
// ping command is usually used by client to test connectivity to the server.
func (s *TestServer) handlePing(message protocol.Message) (protocol.Message, error) {
	pingReply, err := makeOKReply()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return protocol.MakeOpMsg(pingReply), nil
}

// handleFind makes response to the client's "find" command.
//
// Test server always responds with the same test result set to each find command.
func (s *TestServer) handleFind(message protocol.Message) (protocol.Message, error) {
	findReply, err := makeFindReply([]bson.M{{"k": "v"}})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return protocol.MakeOpMsg(findReply), nil
}

// Port returns the port server is listening on.
func (s *TestServer) Port() string {
	return s.port
}

// Close closes the server listener.
func (s *TestServer) Close() error {
	return s.listener.Close()
}

const (
	commandAuth     = "auth"
	commandIsMaster = "isMaster"
	commandPing     = "ping"
	commandFind     = "find"
)

// getCommandType determines which Mongo command this is.
func getCommandType(document bsoncore.Document) string {
	auth, ok := document.Lookup("authenticate").Int32OK()
	if ok && auth == 1 {
		return commandAuth
	}
	isMaster, ok := document.Lookup("isMaster").Int32OK()
	if ok && isMaster == 1 {
		return commandIsMaster
	}
	ping, ok := document.Lookup("ping").Int32OK()
	if ok && ping == 1 {
		return commandPing
	}
	find, ok := document.Lookup("find").StringValueOK()
	if ok && find != "" {
		return commandFind
	}
	return ""
}

// makeOKReply builds a generic document used to indicate success e.g.
// for "ping" and "authenticate" command replies.
func makeOKReply() ([]byte, error) {
	return bson.Marshal(bson.M{
		"ok": 1,
	})
}

// makeIsMasterReply builds a document used as a "isMaster" command reply.
func makeIsMasterReply() ([]byte, error) {
	return bson.Marshal(bson.M{
		"ok":             1,
		"maxWireVersion": 9, // Latest MongoDB server sends maxWireVersion=9.
	})
}

// makeFindReply builds a document used as a "find" command reply.
func makeFindReply(result interface{}) ([]byte, error) {
	return bson.Marshal(bson.M{
		"ok": 1,
		"cursor": bson.M{
			"firstBatch": result,
		},
	})
}
