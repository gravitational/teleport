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
	"crypto/rand"
	"crypto/tls"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/mongodb/protocol"
	"github.com/gravitational/teleport/lib/utils"
)

// MakeTestClient returns MongoDB client connection according to the provided
// parameters.
func MakeTestClient(ctx context.Context, config common.TestClientConfig, opts ...*options.ClientOptions) (*mongo.Client, error) {
	tlsConfig, err := common.MakeTestClientTLSConfig(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client, err := mongo.Connect(ctx, append(
		[]*options.ClientOptions{
			options.Client().
				ApplyURI("mongodb://" + config.Address).
				SetTLSConfig(tlsConfig).
				// Mongo client connects in background so set a short heartbeat
				// interval and server selection timeout so access errors are
				// returned to the client quicker.
				SetHeartbeatInterval(500 * time.Millisecond).
				// Setting load balancer disables the topology selection logic.
				SetLoadBalanced(true),
		}, opts...)...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Mongo client connects in background so do a ping to make sure it
	// can connect successfully.
	err = client.Ping(ctx, nil)
	if err != nil {
		return client, trace.Wrap(err)
	}
	return client, nil
}

// TestServer is a test MongoDB server used in functional database access tests.
type TestServer struct {
	cfg      common.TestServerConfig
	listener net.Listener
	port     string
	log      logrus.FieldLogger

	wireVersion      int
	activeConnection int32

	// conversationIdx conversion ID control number. It is increased every time
	// a SASL conversation is started.
	conversationIdx int32

	// saslConversionTracker map to track which SASL mechanism is being used by
	// the conversion ID.
	saslConversationTracker sync.Map
}

// TestServerOption allows to set test server options.
type TestServerOption func(*TestServer)

// TestServerWireVersion sets the test MongoDB server wire protocol version.
func TestServerWireVersion(wireVersion int) TestServerOption {
	return func(ts *TestServer) {
		ts.wireVersion = wireVersion
	}
}

// NewTestServer returns a new instance of a test MongoDB server.
func NewTestServer(config common.TestServerConfig, opts ...TestServerOption) (svr *TestServer, err error) {
	err = config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer config.CloseOnError(&err)

	port, err := config.Port()
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
	server := &TestServer{
		cfg: config,
		// MongoDB uses regular TLS handshake so standard TLS listener will work.
		listener: tls.NewListener(config.Listener, tlsConfig),
		port:     port,
		log:      log,
	}
	for _, o := range opts {
		o(server)
	}
	return server, nil
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
			atomic.AddInt32(&s.activeConnection, 1)
			defer atomic.AddInt32(&s.activeConnection, -1)
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
	command, err := message.GetCommand()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch command {
	case commandIsMaster:
		return s.handleIsMaster(message)
	case commandAuth:
		return s.handleAuth(message)
	case commandPing:
		return s.handlePing(message)
	case commandFind:
		return s.handleFind(message)
	case commandSaslStart:
		return s.handleSaslStart(message)
	case commandSaslContinue:
		return s.handleSaslContinue(message)
	}
	return nil, trace.NotImplemented("unsupported message: %v", message)
}

// handleAuth makes response to the client's "authenticate" command.
func (s *TestServer) handleAuth(message protocol.Message) (protocol.Message, error) {
	// If authentication token is set on the server, it should only use SASL.
	// This avoid false positives where Teleport uses the wrong authentication
	// method.
	if s.cfg.AuthUser != "" || s.cfg.AuthToken != "" {
		return nil, trace.BadParameter("expected SASL authentication but got certificate")
	}

	command, err := message.GetCommand()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.log.Debugf("Authenticate: %s.", message)
	if command != commandAuth {
		return nil, trace.BadParameter("expected authenticate command, got: %s", message)
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
	isMasterReply, err := makeIsMasterReply(s.getWireVersion())
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

// handleSaslStart makes response to the client's "saslStart" command.
func (s *TestServer) handleSaslStart(message protocol.Message) (protocol.Message, error) {
	opmsg, ok := message.(*protocol.MessageOpQuery)
	if !ok {
		return nil, trace.BadParameter("expected message type *protocol.MessageOpQuery but got %T", message)
	}

	mechanism := opmsg.Query.Lookup("mechanism").StringValue()
	conversationID := atomic.AddInt32(&s.conversationIdx, 1)
	s.saslConversationTracker.Store(conversationID, mechanism)

	switch mechanism {
	case authMechanismAWS:
		return s.handleAWSIAMSaslStart(conversationID, opmsg)
	default:
		return nil, trace.NotImplemented("authentication mechanism %q not supported", mechanism)
	}
}

// handleSaslContinue makes response to the client's "saslContinue" command.
// It expects a conversion to be present at `saslConversationTracker`,
// otherwise it won't be able to define which authentication mechanism to use.
func (s *TestServer) handleSaslContinue(message protocol.Message) (protocol.Message, error) {
	opmsg, ok := message.(*protocol.MessageOpQuery)
	if !ok {
		return nil, trace.BadParameter("expected message type *protocol.MessageOpQuery but got %T", message)
	}

	conversationID := opmsg.Query.Lookup("conversationId").Int32()
	mechanism, ok := s.saslConversationTracker.Load(conversationID)
	if !ok {
		return nil, trace.NotFound("conversationID not found")
	}

	switch mechanism {
	case authMechanismAWS:
		return s.handleAWSIAMSaslContinue(conversationID, opmsg)
	default:
		return nil, trace.NotImplemented("authentication mechanism %q not supported", mechanism)
	}
}

// handleAWSIAMSaslStart handles the "saslStart" command for "MONGODB-AWS"
// authentication.
func (s *TestServer) handleAWSIAMSaslStart(conversationID int32, opmsg *protocol.MessageOpQuery) (protocol.Message, error) {
	_, userPass := opmsg.Query.Lookup("payload").Binary()
	doc, _, ok := bsoncore.ReadDocument(userPass)
	if !ok {
		return nil, trace.BadParameter("invalid payload")
	}

	// Append server "Nonce" to client "Nonce".
	_, clientNonce := doc.Lookup("r").Binary()
	serverNonce := make([]byte, 32)
	_, _ = rand.Read(serverNonce)

	firstResponse, err := bson.Marshal(struct {
		Nonce primitive.Binary `bson:"s"`
		Host  string           `bson:"h"`
	}{
		Nonce: primitive.Binary{
			Subtype: 0x00,
			Data:    append(clientNonce, serverNonce...),
		},
		Host: "sts.amazonaws.com",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authReply, err := makeSaslReply(conversationID, firstResponse)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return protocol.MakeOpMsg(authReply), nil
}

// handleAWSIAMSaslContinue handles the "saslStart" command for "MONGODB-AWS"
// authentication.
func (s *TestServer) handleAWSIAMSaslContinue(conversationID int32, opmsg *protocol.MessageOpQuery) (protocol.Message, error) {
	_, awsSaslPayload := opmsg.Query.Lookup("payload").Binary()
	doc, _, ok := bsoncore.ReadDocument(awsSaslPayload)
	if !ok {
		return nil, trace.BadParameter("invalid payload")
	}

	if !strings.Contains(doc.Lookup("a").StringValue(), s.cfg.AuthUser) {
		return nil, trace.AccessDenied("invalid username")
	}

	if s.cfg.AuthToken != doc.Lookup("t").StringValue() {
		return nil, trace.AccessDenied("invalid session token")
	}

	authReply, err := makeSaslReply(conversationID, []byte{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return protocol.MakeOpMsg(authReply), nil
}

// getWireVersion returns the server's wire protocol version.
func (s *TestServer) getWireVersion() int {
	if s.wireVersion != 0 {
		return s.wireVersion
	}
	return 9 // Latest MongoDB server sends maxWireVersion=9.
}

// Port returns the port server is listening on.
func (s *TestServer) Port() string {
	return s.port
}

// GetActiveConnectionsCount returns the current value of activeConnection counter.
func (s *TestServer) GetActiveConnectionsCount() int32 {
	return atomic.LoadInt32(&s.activeConnection)
}

// Close closes the server listener.
func (s *TestServer) Close() error {
	return s.listener.Close()
}

const (
	authMechanismAWS = "MONGODB-AWS"

	commandAuth         = "authenticate"
	commandIsMaster     = "isMaster"
	commandPing         = "ping"
	commandFind         = "find"
	commandSaslStart    = "saslStart"
	commandSaslContinue = "saslContinue"
)

// makeOKReply builds a generic document used to indicate success e.g.
// for "ping" and "authenticate" command replies.
func makeOKReply() ([]byte, error) {
	return bson.Marshal(bson.M{
		"ok": 1,
	})
}

// makeIsMasterReply builds a document used as a "isMaster" command reply.
func makeIsMasterReply(wireVersion int) ([]byte, error) {
	return bson.Marshal(bson.M{
		"ok":             1,
		"maxWireVersion": wireVersion,
		"compression":    []string{"zlib"},
		"serviceId":      primitive.NewObjectID(),
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

// makeSaslReply builds a document used as reply for "saslStart" and "saslContinue"
// commands.
func makeSaslReply(conversationID int32, payload []byte) ([]byte, error) {
	return bson.Marshal(bson.M{
		"ok":             1,
		"conversationId": conversationID,
		"done":           true,
		"payload":        payload,
	})
}
