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

package postgres

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgproto3/v2"
	"github.com/jackc/pgtype"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/utils"
)

// MakeTestClient returns Postgres client connection according to the provided
// parameters.
func MakeTestClient(ctx context.Context, config common.TestClientConfig) (*pgconn.PgConn, error) {
	// Client will be connecting directly to the multiplexer address.
	pgconnConfig, err := pgconn.ParseConfig(fmt.Sprintf("postgres://%v/?sslmode=verify-full", config.Address))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pgconnConfig.User = config.RouteToDatabase.Username
	pgconnConfig.Database = config.RouteToDatabase.Database
	pgconnConfig.TLSConfig, err = common.MakeTestClientTLSConfig(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if config.UserAgent != "" {
		pgconnConfig.RuntimeParams["application_name"] = config.UserAgent
	}
	pgConn, err := pgconn.ConnectConfig(ctx, pgconnConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pgConn, nil
}

// TestServer is a test Postgres server used in functional database
// access tests.
//
// It supports a very small subset of Postgres wire protocol that can:
//   - Accept a TLS connection from Postgres client.
//   - Reply with the same TestQueryResponse to every query the client sends.
//   - Recognize terminate messages from clients closing connections.
type TestServer struct {
	cfg       common.TestServerConfig
	listener  net.Listener
	port      string
	tlsConfig *tls.Config
	log       *slog.Logger
	// queryCount keeps track of the number of queries the server has received.
	queryCount uint32
	// parametersCh receives startup message connection parameters.
	parametersCh chan map[string]string
	// storedProcedures are the stored procedures created on the server.
	storedProcedures map[string]*storedProcedure
	// userEventsCh receives user activate/deactivate events.
	userEventsCh chan UserEvent
	// userPermissionEventsCh receives user permission change events.
	userPermissionEventsCh chan UserPermissionEvent
	// mu protects test server's shared state.
	mu sync.Mutex
	// allowedUsers list of users that can be used to connect to the server.
	allowedUsers *sync.Map

	// nextPid is a dummy variable used to assign each connection a unique fake "pid".
	// it's incremented after each new startup connection. Starts counting from 1.
	nextPid uint32
	// pids is a map of fake connection pid handles, used for cancel requests.
	pids map[uint32]*pidHandle
	// pidMu is a lock protecting nextPid and pids.
	pidMu sync.Mutex

	// mmCache caches multiMessage for reuse in benchmark
	mmCache map[string]*multiMessage
}

// pidHandle represents a fake pid handle that can cancel operations in progress.
// For test purposes, only a stub query "pg_sleep(forever)" will actually be
// cancellable.
type pidHandle struct {
	// secretKey is checked for equality when cancel request is received.
	secretKey uint32
	// cancel cancels the operation in progress, if any.
	cancel context.CancelFunc
}

// UserEvent represents a user activation/deactivation event.
type UserEvent struct {
	// Name is the user Name.
	Name string
	// Roles are the user Roles.
	Roles []string
	// Active is whether user activated or deactivated.
	Active bool
}

// UserPermissionEvent represents a user permission change event.
type UserPermissionEvent struct {
	// Name is the name of the user.
	Name string
	// Permissions are the user permissions.
	Permissions Permissions
}

// storedProcedure represents a stored procedure.
type storedProcedure struct {
	query     string
	argsCount int
}

// NewTestServer returns a new instance of a test Postgres server.
func NewTestServer(config common.TestServerConfig) (svr *TestServer, err error) {
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

	var allowedUsers sync.Map
	for _, user := range config.Users {
		allowedUsers.Store(user, struct{}{})
	}

	return &TestServer{
		cfg:       config,
		listener:  config.Listener,
		port:      port,
		tlsConfig: tlsConfig,
		log: slog.Default().With(
			teleport.ComponentKey, defaults.ProtocolPostgres,
			"name", config.Name,
		),
		parametersCh:           make(chan map[string]string, 100),
		pids:                   make(map[uint32]*pidHandle),
		storedProcedures:       make(map[string]*storedProcedure),
		userEventsCh:           make(chan UserEvent, 100),
		userPermissionEventsCh: make(chan UserPermissionEvent, 100),
		allowedUsers:           &allowedUsers,
		mmCache:                make(map[string]*multiMessage),
	}, nil
}

// Serve starts serving client connections.
func (s *TestServer) Serve() error {
	s.log.DebugContext(context.Background(), "Starting test Postgres server.", "address", s.listener.Addr())
	defer s.log.DebugContext(context.Background(), "Test Postgres server stopped.")
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if utils.IsOKNetworkError(err) {
				return nil
			}
			s.log.ErrorContext(context.Background(), "Failed to accept connection.", "error", err)
			continue
		}
		s.log.DebugContext(context.Background(), "Accepted connection.")
		go func() {
			defer s.log.DebugContext(context.Background(), "Connection done.")
			defer conn.Close()
			err = s.handleConnection(conn)
			if err != nil {
				s.log.ErrorContext(context.Background(), "Failed to handle connection.", "debug_report", trace.DebugReport(err))
			}
		}()
	}
}

func (s *TestServer) handleConnection(conn net.Conn) error {
	// First message we expect is SSLRequest.
	client, err := s.startTLS(conn)
	if err != nil {
		return trace.Wrap(err)
	}
	startupMessage, err := client.ReceiveStartupMessage()
	if err != nil {
		return trace.Wrap(err)
	}
	s.log.DebugContext(context.Background(), "Received.", "message", fmt.Sprintf("%#v", startupMessage))
	switch msg := startupMessage.(type) {
	case *pgproto3.StartupMessage:
		return s.handleStartup(client, msg)
	case *pgproto3.CancelRequest:
		s.handleCancelRequest(client, msg)
		// never return errors on cancel requests.
		return nil
	default:
		return trace.BadParameter("expected *pgproto3.StartupMessage or *pgproto3.CancelRequest, got: %T", msg)
	}
}

func (s *TestServer) startTLS(conn net.Conn) (*pgproto3.Backend, error) {
	client := pgproto3.NewBackend(pgproto3.NewChunkReader(conn), conn)
	startupMessage, err := client.ReceiveStartupMessage()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if _, ok := startupMessage.(*pgproto3.SSLRequest); !ok {
		return nil, trace.BadParameter("expected *pgproto3.SSLRequest, got: %#v", startupMessage)
	}
	s.log.DebugContext(context.Background(), "Received.", "message", fmt.Sprintf("%#v", startupMessage))
	// Reply with 'S' to indicate TLS support.
	if _, err := conn.Write([]byte("S")); err != nil {
		return nil, trace.Wrap(err)
	}
	// Upgrade connection to TLS.
	conn = tls.Server(conn, s.tlsConfig)
	return pgproto3.NewBackend(pgproto3.NewChunkReader(conn), conn), nil
}

func (s *TestServer) handleStartup(client *pgproto3.Backend, startupMessage *pgproto3.StartupMessage) error {
	// Push connect parameters into the channel so tests can consume them.
	s.parametersCh <- startupMessage.Parameters

	// Perform authentication.
	switch {
	case s.cfg.AuthToken != "":
		// If auth token is specified, used it for password authentication, this
		// simulates cloud provider IAM auth.
		if err := s.handlePasswordAuth(client); err != nil {
			if trace.IsAccessDenied(err) {
				if err := client.Send(&pgproto3.ErrorResponse{Code: pgerrcode.InvalidPassword, Message: err.Error()}); err != nil {
					return trace.Wrap(err)
				}
			}
			return trace.Wrap(err)
		}
	case !s.cfg.AllowAnyUser:
		if _, ok := s.allowedUsers.Load(startupMessage.Parameters[userParameterName]); !ok {
			return trace.AccessDenied("invalid username")
		}
	}

	// Accept auth and send ready for query.
	if err := client.Send(&pgproto3.AuthenticationOk{}); err != nil {
		return trace.Wrap(err)
	}

	pid := s.newPid()
	defer s.cleanupPid(pid)

	err := client.Send(&pgproto3.BackendKeyData{
		ProcessID: pid,
		SecretKey: testSecretKey,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := client.Send(&pgproto3.ReadyForQuery{}); err != nil {
		return trace.Wrap(err)
	}
	// Enter the loop replying to client messages.
	for {
		message, err := s.receiveFrontendMessage(client)
		if err != nil {
			return trace.Wrap(err)
		}
		switch msg := message.(type) {
		case *pgproto3.Query:
			if err := s.handleQuery(client, msg.String, pid); err != nil {
				s.log.ErrorContext(context.Background(), "Failed to handle query.", "error", err)
			}
		// Following messages are for handling Postgres extended query
		// protocol flow used by prepared statements.
		case *pgproto3.Parse:
			schema, procName, argsCount, ok := processProcedureCall(msg.Query)
			if ok {
				if !s.hasProcedure(pid, schema, procName, argsCount) {
					return trace.BadParameter("procedure %q on schema %q wasn't created before the call for PID %d", procName, schema, pid)
				}

				switch procName {
				case activateProcName:
					if err := s.handleActivateUser(client); err != nil {
						s.log.ErrorContext(context.Background(), "Failed to handle user activation.", "error", err)
					}
				case deleteProcName:
					if err := s.handleDeactivateUser(client, true); err != nil {
						s.log.ErrorContext(context.Background(), "Failed to handle user deletion.", "error", err)
					}
				case deactivateProcName:
					if err := s.handleDeactivateUser(client, false); err != nil {
						s.log.ErrorContext(context.Background(), "Failed to handle user deactivation.", "error", err)
					}
				case updatePermissionsProcName:
					if err := s.handleUpdatePermissions(client); err != nil {
						s.log.ErrorContext(context.Background(), "Failed to handle user permissions update.", "error", err)
					}
				}

				continue
			}

			switch msg.Query {
			case schemaInfoQuery:
				if err := s.handleSchemaInfo(client); err != nil {
					s.log.ErrorContext(context.Background(), "Failed to handle schema info query.", "error", err)
				}
			default:
				s.log.WarnContext(context.Background(), "Ignoring PARSE message", "query", msg.Query)
			}
		case *pgproto3.Bind:
		case *pgproto3.Describe:
		case *pgproto3.Sync:
			if err := s.handleSync(client); err != nil {
				s.log.ErrorContext(context.Background(), "Failed to handle sync.", "error", err)
			}
		case *pgproto3.Execute:
			// Execute executes prepared statement.
			if err := s.handleQuery(client, "", pid); err != nil {
				s.log.ErrorContext(context.Background(), "Failed to handle query.", "error", err)
			}
		case *pgproto3.Terminate:
			return nil
		default:
			return trace.BadParameter("unsupported message %#v", message)
		}
	}
}

func (s *TestServer) handleCancelRequest(client *pgproto3.Backend, req *pgproto3.CancelRequest) {
	s.pidMu.Lock()
	defer s.pidMu.Unlock()
	p, ok := s.pids[req.ProcessID]
	if ok && p != nil && p.secretKey == req.SecretKey && p.cancel != nil {
		p.cancel()
	}
}

func (s *TestServer) handlePasswordAuth(client *pgproto3.Backend) error {
	// Request cleartext password.
	if err := client.Send(&pgproto3.AuthenticationCleartextPassword{}); err != nil {
		return trace.Wrap(err)
	}
	// Wait for response which should be PasswordMessage.
	message, err := client.Receive()
	if err != nil {
		return trace.Wrap(err)
	}
	passwordMessage, ok := message.(*pgproto3.PasswordMessage)
	if !ok {
		return trace.BadParameter("expected *pgproto3.PasswordMessage, got: %#v", message)
	}
	// Verify the token.
	if passwordMessage.Password != s.cfg.AuthToken {
		// Logging auth tokens just for the tests debugging convenience...
		return trace.AccessDenied("invalid auth token: got %q, want %q", passwordMessage.Password, s.cfg.AuthToken)
	}
	return nil
}

func (s *TestServer) handleQuery(client *pgproto3.Backend, query string, pid uint32) error {
	atomic.AddUint32(&s.queryCount, 1)
	if query == TestLongRunningQuery {
		return trace.Wrap(s.fakeLongRunningQuery(client, pid))
	}
	if strings.Contains(strings.ToUpper(query), "CREATE OR REPLACE PROCEDURE") {
		if err := s.handleCreateStoredProcedure(query, pid); err != nil {
			return trace.Wrap(err)
		}
	}
	if selectBenchmarkRe.MatchString(query) {
		return trace.Wrap(s.handleBenchmarkQuery(query, client))
	}
	if query == TestErrorQuery {
		return trace.Wrap(s.handleQueryWithError(client))
	}

	messages := []pgproto3.BackendMessage{
		&pgproto3.RowDescription{Fields: TestQueryResponse.FieldDescriptions},
		&pgproto3.DataRow{Values: TestQueryResponse.Rows[0]},
		&pgproto3.CommandComplete{CommandTag: TestQueryResponse.CommandTag},
		&pgproto3.ReadyForQuery{},
	}
	for _, message := range messages {
		s.log.DebugContext(context.Background(), "Sending.", "message", fmt.Sprintf("%#v", message))
		err := client.Send(message)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (s *TestServer) handleQueryWithError(client *pgproto3.Backend) error {
	for _, message := range []pgproto3.BackendMessage{
		&pgproto3.ErrorResponse{Severity: "ERROR", Code: "42703", Message: "error"},
		&pgproto3.ReadyForQuery{},
	} {
		s.log.DebugContext(context.Background(), "Sending.", "message", fmt.Sprintf("%#v", message))
		err := client.Send(message)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (s *TestServer) handleCreateStoredProcedure(query string, pid uint32) error {
	match := storedProcedureRe.FindStringSubmatch(query)
	if match == nil {
		return trace.BadParameter("failed to extract stored procedure name from query")
	}

	if _, ok := procs[match[storedProcedureRe.SubexpIndex("ProcName")]]; !ok {
		return trace.BadParameter("test server doesn't support stored procedure %q", match[1])
	}

	procName := storedProcedureName(pid, match[storedProcedureRe.SubexpIndex("Schema")], match[storedProcedureRe.SubexpIndex("ProcName")])
	var argsCount int
	args := strings.SplitSeq(match[storedProcedureRe.SubexpIndex("Args")], ",")
	for arg := range args {
		// Skip arguments that have a default value.
		if !strings.Contains(strings.ToLower(arg), "default") {
			argsCount++
		}
	}

	s.log.DebugContext(context.Background(), "Created stored procedure.", "procedure", procName)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.storedProcedures[procName] = &storedProcedure{query: query, argsCount: argsCount}
	return nil
}

func (s *TestServer) hasProcedure(pid uint32, schema, procName string, argsCount int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	storedProcedure, ok := s.storedProcedures[storedProcedureName(pid, schema, procName)]
	if !ok {
		s.log.ErrorContext(context.Background(), "Procedure not found", "procedure", procName, "schema", schema)
		return false
	}

	if argsCount != storedProcedure.argsCount {
		s.log.ErrorContext(context.Background(), "Wrong number of arguments for procedure call", "procedure", procName, "expected_args", storedProcedure.argsCount, "args_provided", argsCount)
		return false
	}

	return true
}

func storedProcedureName(pid uint32, schema, procName string) string {
	var name string
	switch strings.ToLower(schema) {
	case "pg_temp":
		name = fmt.Sprintf("%d.%s", pid, procName)
	case "":
		name = procName
	default:
		name = fmt.Sprintf("%s.%s", schema, procName)
	}

	return strings.ToLower(name)
}

// multiMessage wraps *pgproto3.DataRow and implements pgproto3.BackendMessage by writing multiple copies of this message in Encode.
type multiMessage struct {
	singleMessage *pgproto3.DataRow
	payload       []byte
}

func newMultiMessage(rowSize, repeats int) (*multiMessage, error) {
	buf := make([]byte, rowSize)
	_, err := rand.Read(buf)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	message := &pgproto3.DataRow{Values: [][]byte{buf}}
	encoded, err := message.Encode(nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	payload := bytes.Repeat(encoded, repeats)
	return &multiMessage{
		singleMessage: message,
		payload:       payload,
	}, nil
}

func (m *multiMessage) Decode(_ []byte) error {
	return trace.NotImplemented("Decode is not implemented for multiMessage")
}

func (m *multiMessage) Encode(dst []byte) ([]byte, error) {
	return append(dst, m.payload...), nil
}

func (m *multiMessage) Backend() {
}

var _ pgproto3.BackendMessage = (*multiMessage)(nil)

func (s *TestServer) getMultiMessage(rowSize, repeats int) (*multiMessage, error) {
	key := fmt.Sprintf("%v/%v", rowSize, repeats)
	if mm, ok := s.mmCache[key]; ok {
		return mm, nil
	}
	mm, err := newMultiMessage(rowSize, repeats)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.mmCache[key] = mm
	return mm, nil
}

// handleBenchmarkQuery handles the query used for read benchmark. It will send a stream of messages of requested size and number.
func (s *TestServer) handleBenchmarkQuery(query string, client *pgproto3.Backend) error {
	// parse benchmark parameters
	matches := selectBenchmarkRe.FindStringSubmatch(query)

	messageSize, err := strconv.Atoi(matches[1])
	if err != nil {
		return trace.Wrap(err)
	}
	// minimum message size is 11, corresponding to empty buffer transferred in a DataRow
	if messageSize < 11 {
		return trace.BadParameter("bad message size, must be at least 11, got %v", messageSize)
	}

	repeats, err := strconv.Atoi(matches[2])
	if err != nil {
		return trace.Wrap(err)
	}

	mm, err := s.getMultiMessage(messageSize-11, repeats)
	if err != nil {
		return trace.Wrap(err)
	}

	s.log.DebugContext(context.Background(), "Responding to query", "query", query, "repeat", repeats, "length", len(mm.payload))

	// preamble
	err = client.Send(&pgproto3.RowDescription{Fields: []pgproto3.FieldDescription{{Name: []byte("dummy")}}})
	if err != nil {
		return trace.Wrap(err)
	}

	// send messages in bulk, which is fast.
	err = client.Send(mm)
	if err != nil {
		return trace.Wrap(err)
	}

	// epilogue
	err = client.Send(&pgproto3.CommandComplete{CommandTag: []byte("SELECT 100")})
	if err != nil {
		return trace.Wrap(err)
	}

	err = client.Send(&pgproto3.ReadyForQuery{})
	if err != nil {
		return trace.Wrap(err)
	}

	s.log.DebugContext(context.Background(), "Finished handling query", "query", query)

	return nil
}

func (s *TestServer) handleActivateUser(client *pgproto3.Backend) error {
	// Expect Describe message.
	_, err := s.receiveDescribeMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Expect Sync message.
	_, err = s.receiveSyncMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Respond to Parse message.
	err = s.sendMessages(client,
		&pgproto3.ParseComplete{},
		&pgproto3.ParameterDescription{ParameterOIDs: []uint32{pgtype.VarcharOID, pgtype.VarcharArrayOID}},
		&pgproto3.NoData{},
		&pgproto3.ReadyForQuery{})
	if err != nil {
		return trace.Wrap(err)
	}
	// Expect Bind message.
	bind, err := s.receiveBindMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Extract user name.
	name, err := getVarchar(bind.ParameterFormatCodes[0], bind.Parameters[0])
	if err != nil {
		return trace.Wrap(err)
	}
	// Extract role names.
	roles, err := getVarcharArray(bind.ParameterFormatCodes[1], bind.Parameters[1])
	if err != nil {
		return trace.Wrap(err)
	}
	// Expect Describe message.
	_, err = s.receiveDescribeMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Expect Execute message.
	_, err = s.receiveExecuteMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Expect Sync message.
	_, err = s.receiveSyncMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Respond to Bind message.
	err = s.sendMessages(client,
		&pgproto3.BindComplete{},
		&pgproto3.NoData{},
		&pgproto3.CommandComplete{},
		&pgproto3.ReadyForQuery{})
	if err != nil {
		return trace.Wrap(err)
	}
	// Mark the user as active.
	s.log.DebugContext(context.Background(), "Activated user.", "user", name, "roles", roles)
	s.userEventsCh <- UserEvent{Name: name, Roles: roles, Active: true}
	s.allowedUsers.Store(name, struct{}{})
	return nil
}

func (s *TestServer) handleDeactivateUser(client *pgproto3.Backend, sendDeleteResponse bool) error {
	// Expect Describe message.
	_, err := s.receiveDescribeMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Expect Sync message.
	_, err = s.receiveSyncMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Respond to Parse message.
	err = s.sendMessages(client,
		&pgproto3.ParseComplete{},
		&pgproto3.ParameterDescription{ParameterOIDs: []uint32{pgtype.VarcharOID}},
		&pgproto3.NoData{},
		&pgproto3.ReadyForQuery{})
	if err != nil {
		return trace.Wrap(err)
	}
	// Expect Bind message.
	bind, err := s.receiveBindMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Extract user name.
	name, err := getVarchar(bind.ParameterFormatCodes[0], bind.Parameters[0])
	if err != nil {
		return trace.Wrap(err)
	}
	// Expect Describe message.
	_, err = s.receiveDescribeMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Expect Execute message.
	_, err = s.receiveExecuteMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Expect Sync message.
	_, err = s.receiveSyncMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Respond to Bind message.
	messages := []pgproto3.BackendMessage{
		&pgproto3.BindComplete{},
	}
	if sendDeleteResponse {
		messages = append(messages,
			&pgproto3.RowDescription{Fields: TestDeleteUserResponse.FieldDescriptions},
			&pgproto3.DataRow{Values: TestDeleteUserResponse.Rows[0]},
		)
	} else {
		messages = append(messages, &pgproto3.NoData{})
	}
	messages = append(messages,
		&pgproto3.CommandComplete{},
		&pgproto3.ReadyForQuery{},
	)

	err = s.sendMessages(client, messages...)
	if err != nil {
		return trace.Wrap(err)
	}
	// Mark the user as active.
	s.log.DebugContext(context.Background(), "Deactivated user.", "user", name)
	s.userEventsCh <- UserEvent{Name: name, Active: false}
	s.allowedUsers.Delete(name)
	return nil
}

func (s *TestServer) handleUpdatePermissions(client *pgproto3.Backend) error {
	// Expect Describe message.
	_, err := s.receiveDescribeMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Expect Sync message.
	_, err = s.receiveSyncMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Respond to Parse message.
	err = s.sendMessages(client,
		&pgproto3.ParseComplete{},
		&pgproto3.ParameterDescription{ParameterOIDs: []uint32{pgtype.VarcharOID, pgtype.JSONBOID}},
		&pgproto3.NoData{},
		&pgproto3.ReadyForQuery{})
	if err != nil {
		return trace.Wrap(err)
	}
	// Expect Bind message.
	bind, err := s.receiveBindMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Extract user name.
	name, err := getVarchar(bind.ParameterFormatCodes[0], bind.Parameters[0])
	if err != nil {
		return trace.Wrap(err)
	}
	// Extract role names.
	perms, err := getJSONB[Permissions](bind.ParameterFormatCodes[1], bind.Parameters[1])
	if err != nil {
		return trace.Wrap(err)
	}
	// Expect Describe message.
	_, err = s.receiveDescribeMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Expect Execute message.
	_, err = s.receiveExecuteMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Expect Sync message.
	_, err = s.receiveSyncMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Respond to Bind message.
	err = s.sendMessages(client,
		&pgproto3.BindComplete{},
		&pgproto3.NoData{},
		&pgproto3.CommandComplete{},
		&pgproto3.ReadyForQuery{})
	if err != nil {
		return trace.Wrap(err)
	}
	// Mark the user as active.
	s.log.DebugContext(context.Background(), "Updated permissions for user.", "user", name, "permissions", fmt.Sprintf("%#v", perms))
	s.userPermissionEventsCh <- UserPermissionEvent{Name: name, Permissions: perms}
	return nil
}

func (s *TestServer) handleSchemaInfo(client *pgproto3.Backend) error {
	// Expect Describe message.
	_, err := s.receiveDescribeMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Expect Sync message.
	_, err = s.receiveSyncMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Respond to Parse message.
	err = s.sendMessages(client,
		&pgproto3.ParseComplete{},
		&pgproto3.ParameterDescription{ParameterOIDs: []uint32{}},
		&pgproto3.RowDescription{Fields: []pgproto3.FieldDescription{{Name: []byte("schemaname")}, {Name: []byte("tablename")}}},
		&pgproto3.ReadyForQuery{})
	if err != nil {
		return trace.Wrap(err)
	}
	// Expect Bind message.
	_, err = s.receiveBindMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Expect Describe message.
	_, err = s.receiveDescribeMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Expect Execute message.
	_, err = s.receiveExecuteMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Expect Sync message.
	_, err = s.receiveSyncMessage(client)
	if err != nil {
		return trace.Wrap(err)
	}

	// Respond to Bind message.
	err = s.sendMessages(client,
		&pgproto3.BindComplete{},
		&pgproto3.RowDescription{Fields: []pgproto3.FieldDescription{{Name: []byte("schemaname")}, {Name: []byte("tablename")}}},
	)
	if err != nil {
		return trace.Wrap(err)
	}

	// Send over results
	fixedSchema := map[string][]string{
		"public": {"employees", "orders", "departments", "projects", "customers"},
		"hr":     {"salaries"},
	}
	for schemaName, tables := range fixedSchema {
		for _, tableName := range tables {
			err = s.sendMessages(client, &pgproto3.DataRow{Values: [][]byte{[]byte(schemaName), []byte(tableName)}})
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}

	// Notify the client we are finished.
	err = s.sendMessages(client,
		&pgproto3.CommandComplete{},
		&pgproto3.ReadyForQuery{})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *TestServer) receiveDescribeMessage(client *pgproto3.Backend) (*pgproto3.Describe, error) {
	message, err := s.receiveFrontendMessage(client)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	describe, ok := message.(*pgproto3.Describe)
	if !ok {
		return nil, trace.BadParameter("expected *pgproto3.Describe, got %#v", message)
	}
	return describe, nil
}

func (s *TestServer) receiveSyncMessage(client *pgproto3.Backend) (*pgproto3.Sync, error) {
	message, err := s.receiveFrontendMessage(client)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sync, ok := message.(*pgproto3.Sync)
	if !ok {
		return nil, trace.BadParameter("expected *pgproto3.Sync, got %#v", message)
	}
	return sync, nil
}

func (s *TestServer) receiveBindMessage(client *pgproto3.Backend) (*pgproto3.Bind, error) {
	message, err := s.receiveFrontendMessage(client)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	bind, ok := message.(*pgproto3.Bind)
	if !ok {
		return nil, trace.BadParameter("expected *pgproto3.Bind, got %#v", message)
	}
	return bind, nil
}

func (s *TestServer) receiveExecuteMessage(client *pgproto3.Backend) (*pgproto3.Execute, error) {
	message, err := s.receiveFrontendMessage(client)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	execute, ok := message.(*pgproto3.Execute)
	if !ok {
		return nil, trace.BadParameter("expected *pgproto3.Execute, got %#v", message)
	}
	return execute, nil
}

func (s *TestServer) receiveFrontendMessage(client *pgproto3.Backend) (pgproto3.FrontendMessage, error) {
	message, err := client.Receive()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.log.DebugContext(context.Background(), "Received.", "message", fmt.Sprintf("%#v", message))
	return message, nil
}

func getVarchar(formatCode int16, src []byte) (string, error) {
	var dst any
	err := pgtype.NewConnInfo().Scan(pgtype.VarcharOID, formatCode, src, &dst)
	if err != nil {
		return "", trace.Wrap(err)
	}
	str, ok := dst.(string)
	if !ok {
		return "", trace.BadParameter("expected string, got %#v", dst)
	}
	return str, nil
}

func getVarcharArray(formatCode int16, src []byte) ([]string, error) {
	var dst any
	err := pgtype.NewConnInfo().Scan(pgtype.VarcharArrayOID, formatCode, src, &dst)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	arr, ok := dst.(pgtype.VarcharArray)
	if !ok {
		return nil, trace.BadParameter("expected string array, got %#v", dst)
	}
	var strs []string
	for _, el := range arr.Elements {
		strs = append(strs, el.String)
	}
	return strs, nil
}

// getJSONB parses the incoming JSONB data into target type. To get raw data, pass pgtype.JSONB as T.
func getJSONB[T any](formatCode int16, src []byte) (T, error) {
	var dst T
	err := pgtype.NewConnInfo().Scan(pgtype.JSONBOID, formatCode, src, &dst)
	if err != nil {
		return dst, trace.Wrap(err)
	}
	return dst, nil
}

func (s *TestServer) sendMessages(client *pgproto3.Backend, messages ...pgproto3.BackendMessage) error {
	for _, message := range messages {
		s.log.DebugContext(context.Background(), "Sending.", "message", fmt.Sprintf("%#v", message))
		err := client.Send(message)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (s *TestServer) fakeLongRunningQuery(client *pgproto3.Backend, pid uint32) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.registerCancel(pid, cancel); err != nil {
		return trace.Wrap(err)
	}
	<-ctx.Done()
	messages := []pgproto3.BackendMessage{
		&pgproto3.ErrorResponse{
			Code:    pgerrcode.QueryCanceled,
			Message: "canceling statement due to user request",
		},
		&pgproto3.ReadyForQuery{},
	}
	for _, message := range messages {
		s.log.DebugContext(context.Background(), "Sending.", "message", fmt.Sprintf("%#v", message))
		err := client.Send(message)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (s *TestServer) handleSync(client *pgproto3.Backend) error {
	message := &pgproto3.ReadyForQuery{}
	s.log.DebugContext(context.Background(), "Sending.", "message", fmt.Sprintf("%#v", message))
	err := client.Send(message)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Port returns the port server is listening on.
func (s *TestServer) Port() string {
	return s.port
}

// QueryCount returns the number of queries the server has received.
func (s *TestServer) QueryCount() uint32 {
	return atomic.LoadUint32(&s.queryCount)
}

// ParametersCh returns channel that receives startup message parameters.
func (s *TestServer) ParametersCh() chan map[string]string {
	return s.parametersCh
}

// UserEventsCh returns channel that receives user activate/deactivate events.
func (s *TestServer) UserEventsCh() <-chan UserEvent {
	return s.userEventsCh
}

// UserPermissionsCh returns channel that receives user permission events.
func (s *TestServer) UserPermissionsCh() <-chan UserPermissionEvent {
	return s.userPermissionEventsCh
}

// Close closes the server listener.
func (s *TestServer) Close() error {
	closeErr := s.listener.Close()
	s.cleanupPids()
	return trace.Wrap(closeErr)
}

// newPid makes a unique PID and inserts it into the server's pid map.
// For test purposes, every pid will have the same stub secret key.
func (s *TestServer) newPid() uint32 {
	s.pidMu.Lock()
	defer s.pidMu.Unlock()
	s.nextPid++
	s.pids[s.nextPid] = &pidHandle{secretKey: testSecretKey}
	return s.nextPid
}

// cleanupPid cleans up a pid by calling its cancel func and
// deleting it from the pid map.
func (s *TestServer) cleanupPid(pid uint32) {
	s.pidMu.Lock()
	defer s.pidMu.Unlock()
	if entry, ok := s.pids[pid]; ok && entry != nil && entry.cancel != nil {
		entry.cancel()
		delete(s.pids, pid)
	}
}

func (s *TestServer) cleanupPids() {
	s.pidMu.Lock()
	defer s.pidMu.Unlock()
	for pid, entry := range s.pids {
		if entry != nil && entry.cancel != nil {
			entry.cancel()
		}
		delete(s.pids, pid)
	}
}

// registerCancel registers a cancel func for a given pid and returns a context.
func (s *TestServer) registerCancel(pid uint32, cancel context.CancelFunc) error {
	s.pidMu.Lock()
	defer s.pidMu.Unlock()
	entry, ok := s.pids[pid]
	if !ok || entry == nil {
		return trace.BadParameter("expected registered info for pid %v", pid)
	}
	entry.cancel = cancel
	return nil
}

// TestQueryResponse is the response test Postgres server sends to every success
// query.
var TestQueryResponse = &pgconn.Result{
	FieldDescriptions: []pgproto3.FieldDescription{{Name: []byte("test-field")}},
	Rows:              [][][]byte{{[]byte("test-value")}},
	CommandTag:        pgconn.CommandTag("select 1"),
}

// TestDeleteUserResponse is the response test Postgres server sends to every
// query that calls the auto user deletion procedure.
var TestDeleteUserResponse = &pgconn.Result{
	FieldDescriptions: []pgproto3.FieldDescription{{Name: []byte("state")}},
	Rows:              [][][]byte{{[]byte("TP003")}},
}

// TestLongRunningQuery is a stub SQL query clients can use to simulate a long
// running query that can be only be stopped by a cancel request.
const TestLongRunningQuery = "pg_sleep(forever)"

// TestErrorQuery is a stub SQL query clients can use to simulate a query that
// returns error.
const TestErrorQuery = "select err"

// testSecretKey is the secret key stub for all connections, used for cancel requests.
const testSecretKey = 1234

// userParameterName is the parameter name that contains the username used to
// connect.
const userParameterName = "user"

// storedProcedureRe is the regex for capturing stored procedure name from its
// creation query.
var storedProcedureRe = regexp.MustCompile(`(?i)create or replace procedure (?:(?P<Schema>\w+)\.)?(?P<ProcName>.+)\((?P<Args>.+)?\)`)

// selectBenchmarkRe is the regex for capturing the parameters from the select query used for read benchmark.
var selectBenchmarkRe = regexp.MustCompile(`SELECT \* FROM bench\_(\d+) LIMIT (\d+)`)

// callProcedureRe is the regex for caputuring the schema name, and procedure
// name from the procedure call query.
// Examples:
// - call pg_temp.hello($1)
// - call pg_temp.hello1()
// - call pg_temp.hello2($1, $2, $3)
// - call hello3($1::jsonb)
var callProcedureRe = regexp.MustCompile(`(?i)^call (?:(?P<Schema>\w+)\.)?(?P<ProcName>\w+)\((?P<Args>.+)?\)`)

// processProcedureCall parses a query and returns the information about the
// the procedure call.
// Examples:
// - create or replace procedure teleport_procedure(username varchar, inout state varchar default 'TP003')
// - create or replace procedure pg_temp.teleport_procedure(username varchar, inout state varchar default 'TP003')
// - create or replace procedure pg_temp.teleport_procedure()
// - create or replace procedure pg_temp.teleport_procedure(permissions_ JSONB)
func processProcedureCall(query string) (schema string, procName string, argsCount int, ok bool) {
	procMatches := callProcedureRe.FindStringSubmatch(query)
	if procMatches == nil {
		return
	}

	ok = true
	schema = procMatches[callProcedureRe.SubexpIndex("Schema")]
	procName = procMatches[callProcedureRe.SubexpIndex("ProcName")]
	argsCount = len(strings.Split(procMatches[callProcedureRe.SubexpIndex("Args")], ","))
	return
}
