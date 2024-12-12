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
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/mongodb/protocol"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
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
	usersTracker

	cfg      common.TestServerConfig
	listener net.Listener
	port     string
	logger   *slog.Logger

	serverVersion    string
	wireVersion      int
	activeConnection int32
	maxMessageSize   uint32

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

// TestServerMaxMessageSize sets the test MongoDB server max message size.
func TestServerMaxMessageSize(maxMessageSize uint32) TestServerOption {
	return func(ts *TestServer) {
		ts.maxMessageSize = maxMessageSize
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
	log := utils.NewSlogLoggerForTests().With(
		teleport.ComponentKey, defaults.ProtocolMongoDB,
		"name", config.Name,
	)
	server := &TestServer{
		cfg: config,
		// MongoDB uses regular TLS handshake so standard TLS listener will work.
		listener:      tls.NewListener(config.Listener, tlsConfig),
		port:          port,
		logger:        log,
		serverVersion: "7.0.0",
		usersTracker: usersTracker{
			userEventsCh: make(chan UserEvent, 100),
			users:        make(map[string]userWithTracking),
		},
	}
	for _, o := range opts {
		o(server)
	}
	return server, nil
}

// Serve starts serving client connections.
func (s *TestServer) Serve() error {
	ctx := context.Background()
	s.logger.DebugContext(ctx, "Starting test MongoDB server", "listen_addr", s.listener.Addr())
	defer s.logger.DebugContext(ctx, "Test MongoDB server stopped")
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if utils.IsOKNetworkError(err) {
				return nil
			}
			s.logger.ErrorContext(ctx, "Failed to accept connection", "error", err)
			continue
		}
		s.logger.DebugContext(ctx, "Accepted connection")
		go func() {
			defer s.logger.DebugContext(ctx, "Connection done")
			defer conn.Close()
			atomic.AddInt32(&s.activeConnection, 1)
			defer atomic.AddInt32(&s.activeConnection, -1)
			if err := s.handleConnection(conn); err != nil {
				if !utils.IsOKNetworkError(err) {
					s.logger.ErrorContext(ctx, "Failed to handle connection", "error", err)
				}
			}
		}()
	}
}

// handleConnection receives Mongo wire messages from the client connection
// and sends back the response messages.
func (s *TestServer) handleConnection(conn net.Conn) error {
	release, err := s.trackUserConnection(conn)
	if err != nil {
		return trace.Wrap(err)
	}
	defer release()

	// Read client messages and reply to them - test server supports a very
	// basic set of commands: "isMaster", "authenticate", "ping" and "find".
	for {
		message, err := protocol.ReadMessage(conn, s.getMaxMessageSize())
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
	case commandHello:
		return s.handleHello(message)
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
	case commandEndSessions:
		return sendOKReply()
	case commandBuildInfo:
		return s.handleBuildInfo(message)

	// Auto-user provisioning related commands.
	case commandCurrentOp:
		return s.handleCurrentOp(message)
	case commandCreateUser:
		return s.handleCreateUser(message)
	case commandUsersInfo:
		return s.handleUsersInfo(message)
	case commandUpdateUser:
		return s.handleUpdateUser(message)
	case commandDropUser:
		return s.handleDropUser(message)
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
	s.logger.DebugContext(context.Background(), "Authenticate", "message", logutils.StringerAttr(message))
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
// cluster topology. Replaced by hello command in newer versions.
func (s *TestServer) handleIsMaster(message protocol.Message) (protocol.Message, error) {
	isMasterReply, err := makeIsMasterReply(s.getWireVersion(), s.getMaxMessageSize())
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

// handleHello makes response to the client's "hello" command.
//
// hello command is used as a handshake by the client to determine the
// cluster topology.
func (s *TestServer) handleHello(message protocol.Message) (protocol.Message, error) {
	reply, err := bson.Marshal(bson.M{
		"ok":                  1,
		"isWritablePrimary":   true,
		"maxMessageSizeBytes": s.getMaxMessageSize(),
		"maxWireVersion":      s.getWireVersion(),
		"readOnly":            false,
		"compression":         []string{"zlib"},
		// `serviceId` is required for LoadBalanced mode. Reference:
		// https://github.com/mongodb/specifications/blob/master/source/load-balancers/load-balancers.rst
		"serviceId": primitive.NewObjectID(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return protocol.MakeOpMsg(reply), nil
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
	opmsg, ok := message.(*protocol.MessageOpMsg)
	if !ok {
		return nil, trace.BadParameter("expected message type *protocol.MessageOpMsg but got %T", message)
	}

	mechanism := opmsg.BodySection.Document.Lookup("mechanism").StringValue()
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
	opmsg, ok := message.(*protocol.MessageOpMsg)
	if !ok {
		return nil, trace.BadParameter("expected message type *protocol.MessageOpMsg but got %T", message)
	}

	conversationID := opmsg.BodySection.Document.Lookup("conversationId").Int32()
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
func (s *TestServer) handleAWSIAMSaslStart(conversationID int32, opmsg *protocol.MessageOpMsg) (protocol.Message, error) {
	_, userPass := opmsg.BodySection.Document.Lookup("payload").Binary()
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
func (s *TestServer) handleAWSIAMSaslContinue(conversationID int32, opmsg *protocol.MessageOpMsg) (protocol.Message, error) {
	_, awsSaslPayload := opmsg.BodySection.Document.Lookup("payload").Binary()
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

func (s *TestServer) handleBuildInfo(msg protocol.Message) (protocol.Message, error) {
	return sendOKReply(withReplyKeyValue("version", s.serverVersion))
}

func (s *TestServer) handleCurrentOp(msg protocol.Message) (protocol.Message, error) {
	opmsg, ok := msg.(*protocol.MessageOpMsg)
	if !ok {
		return nil, trace.BadParameter("invalid msg")
	}

	req := struct {
		EffectiveUsers struct {
			Match struct {
				User string `bson:"user"`
			} `bson:"$elemMatch"`
		} `bson:"effectiveUsers"`
	}{}
	if err := bson.Unmarshal([]byte(opmsg.BodySection.Document), &req); err != nil {
		return nil, trace.Wrap(err)
	}

	// We are cheating here as the Engine only counts the number of inprog.
	conns := s.usersTracker.getUserRemoteConnections(req.EffectiveUsers.Match.User)
	return sendOKReply(withReplyKeyValue("inprog", conns))
}

func (s *TestServer) handleCreateUser(msg protocol.Message) (protocol.Message, error) {
	username, user, err := getUsernameAndUserFromMessage(msg, commandCreateUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.usersTracker.createUser(username, user); err != nil {
		return nil, trace.Wrap(err)
	}
	return sendOKReply()
}

func (s *TestServer) handleUsersInfo(msg protocol.Message) (protocol.Message, error) {
	username, err := getUsernameFromMessage(msg, commandUsersInfo)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	users := []user{}
	if user, found := s.usersTracker.getUser(username); found {
		users = append(users, user)
	}
	return sendOKReply(withReplyKeyValue("users", users))
}

func (s *TestServer) handleUpdateUser(msg protocol.Message) (protocol.Message, error) {
	username, user, err := getUsernameAndUserFromMessage(msg, commandUpdateUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.usersTracker.updateUser(username, user); err != nil {
		return nil, trace.Wrap(err)
	}
	return sendOKReply()
}

func (s *TestServer) handleDropUser(msg protocol.Message) (protocol.Message, error) {
	username, err := getUsernameFromMessage(msg, commandDropUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.usersTracker.deleteUser(username); err != nil {
		return nil, trace.Wrap(err)
	}
	return sendOKReply()
}

// getWireVersion returns the server's wire protocol version.
func (s *TestServer) getWireVersion() int {
	if s.wireVersion != 0 {
		return s.wireVersion
	}
	return 9 // Latest MongoDB server sends maxWireVersion=9.
}

// getMaxMessageSize returns the server's max message size.
func (s *TestServer) getMaxMessageSize() uint32 {
	if s.maxMessageSize != 0 {
		return s.maxMessageSize
	}
	return protocol.DefaultMaxMessageSizeBytes
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

func withReplyKeyValue(key string, value any) func(bson.M) {
	return func(reply bson.M) {
		reply[key] = value
	}
}

func sendOKReply(opts ...func(bson.M)) (protocol.Message, error) {
	reply := bson.M{
		"ok": 1,
	}
	for _, applyOpt := range opts {
		applyOpt(reply)
	}
	replyBytes, err := bson.Marshal(reply)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return protocol.MakeOpMsg(replyBytes), nil
}

func getUsernameFromMessage(msg protocol.Message, usernameKey string) (string, error) {
	opmsg, ok := msg.(*protocol.MessageOpMsg)
	if !ok {
		return "", trace.BadParameter("invalid msg")
	}
	username := opmsg.BodySection.Document.Lookup(usernameKey)
	if !strings.HasPrefix(username.StringValue(), "CN=") {
		return "", trace.BadParameter("invalid username %v", username)
	}
	return username.StringValue(), nil
}

func getUsernameAndUserFromMessage(msg protocol.Message, usernameKey string) (string, user, error) {
	var user user
	username, err := getUsernameFromMessage(msg, usernameKey)
	if err != nil {
		return "", user, trace.Wrap(err)
	}

	err = bson.Unmarshal([]byte(msg.(*protocol.MessageOpMsg).BodySection.Document), &user)
	return username, user, trace.Wrap(err)
}

// UserEventType defines the type of the UserEventType.
type UserEventType int

const (
	UserEventActivate UserEventType = iota
	UserEventDeactivate
	UserEventDelete
)

// UserEvent represents a user activation/deactivation event.
type UserEvent struct {
	// DatabaseUser is the in-database username.
	DatabaseUser string
	// Roles are the user Roles.
	Roles []string
	// Type defines the type of the UserEventType.
	Type UserEventType
}

type userWithTracking struct {
	user              user
	activeConnections map[net.Conn]struct{}
}

type usersTracker struct {
	userEventsCh chan UserEvent
	users        map[string]userWithTracking
	usersMu      sync.Mutex
}

// UserEventsCh returns channel that receives user activate/deactivate events.
func (t *usersTracker) UserEventsCh() <-chan UserEvent {
	return t.userEventsCh
}

func (t *usersTracker) trackUserConnection(conn net.Conn) (func(), error) {
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return func() {}, nil
	}

	if err := tlsConn.Handshake(); err != nil {
		return nil, trace.Wrap(err)
	} else if len(tlsConn.ConnectionState().PeerCertificates) == 0 {
		return func() {}, nil
	}

	username := "CN=" + tlsConn.ConnectionState().PeerCertificates[0].Subject.CommonName

	t.usersMu.Lock()
	defer t.usersMu.Unlock()
	if user, found := t.users[username]; found {
		user.activeConnections[conn] = struct{}{}
	}

	return func() {
		// Untrack per-user active connections.
		t.usersMu.Lock()
		defer t.usersMu.Unlock()
		if user, found := t.users[username]; found {
			delete(user.activeConnections, conn)
		}
	}, nil
}

func (t *usersTracker) sendUserEvent(username string, user user, eventType UserEventType) {
	event := UserEvent{
		DatabaseUser: username,
		Type:         eventType,
	}
	for _, role := range user.Roles {
		event.Roles = append(event.Roles, fmt.Sprintf("%s@%s", role.Rolename, role.Database))
	}
	t.userEventsCh <- event
}

func (t *usersTracker) getUser(username string) (user, bool) {
	t.usersMu.Lock()
	defer t.usersMu.Unlock()

	user, ok := t.users[username]
	return user.user, ok
}

func (t *usersTracker) getUserRemoteConnections(username string) (conns []string) {
	t.usersMu.Lock()
	defer t.usersMu.Unlock()

	user, ok := t.users[username]
	if !ok {
		return conns
	}

	for conn := range user.activeConnections {
		conns = append(conns, conn.RemoteAddr().String())
	}
	return conns
}

func (t *usersTracker) createUser(username string, user user) error {
	t.usersMu.Lock()
	defer t.usersMu.Unlock()

	_, found := t.users[username]
	if found {
		return trace.AlreadyExists("user %q already exists", username)
	}

	t.users[username] = userWithTracking{
		user:              user,
		activeConnections: make(map[net.Conn]struct{}),
	}
	go t.sendUserEvent(username, user, UserEventActivate)
	return nil
}

func (t *usersTracker) updateUser(username string, user user) error {
	t.usersMu.Lock()
	defer t.usersMu.Unlock()

	existingUser, found := t.users[username]
	if !found {
		return trace.NotFound("user %q not found", username)
	}

	existingUser.user = user
	if user.isLocked() {
		go t.sendUserEvent(username, user, UserEventDeactivate)
	} else {
		go t.sendUserEvent(username, user, UserEventActivate)
	}
	return nil
}

func (t *usersTracker) deleteUser(username string) error {
	t.usersMu.Lock()
	defer t.usersMu.Unlock()

	existingUser, found := t.users[username]
	if !found {
		return trace.NotFound("user %q not found", username)
	} else if len(existingUser.activeConnections) > 0 {
		return trace.CompareFailed("user %q has active connections", username)
	}

	delete(t.users, username)
	go t.sendUserEvent(username, existingUser.user, UserEventDelete)
	return nil
}

const (
	authMechanismAWS = "MONGODB-AWS"

	commandAuth         = "authenticate"
	commandIsMaster     = "isMaster"
	commandHello        = "hello"
	commandPing         = "ping"
	commandFind         = "find"
	commandSaslStart    = "saslStart"
	commandSaslContinue = "saslContinue"
	commandEndSessions  = "endSessions"
	commandBuildInfo    = "buildInfo"

	commandCurrentOp  = "currentOp"
	commandCreateUser = "createUser"
	commandUsersInfo  = "usersInfo"
	commandUpdateUser = "updateUser"
	commandDropUser   = "dropUser"
)

// makeOKReply builds a generic document used to indicate success e.g.
// for "ping" and "authenticate" command replies.
func makeOKReply() ([]byte, error) {
	return bson.Marshal(bson.M{
		"ok": 1,
	})
}

// makeIsMasterReply builds a document used as a "isMaster" command reply.
func makeIsMasterReply(wireVersion int, maxMessageSize uint32) ([]byte, error) {
	return bson.Marshal(bson.M{
		"ok":              1,
		"maxWireVersion":  wireVersion,
		"maxMessageBytes": maxMessageSize,
		"compression":     []string{"zlib"},
		// `serviceId` is required for LoadBalanced mode. Reference:
		// https://github.com/mongodb/specifications/blob/master/source/load-balancers/load-balancers.rst
		"serviceId": primitive.NewObjectID(),
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
