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

package mysql

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/server"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// TestClientConn defines interface for client.Conn.
type TestClientConn interface {
	Execute(command string, args ...any) (*mysql.Result, error)
	Close() error
	UseDB(dbName string) error
	GetServerVersion() string
	Ping() error
	WritePacket(data []byte) error
}

// MakeTestClient returns MySQL client connection according to the provided
// parameters.
func MakeTestClient(config common.TestClientConfig) (TestClientConn, error) {
	tlsConfig, err := common.MakeTestClientTLSConfig(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	conn, err := client.Connect(config.Address,
		config.RouteToDatabase.Username,
		"",
		config.RouteToDatabase.Database,
		func(conn *client.Conn) error {
			conn.SetTLSConfig(tlsConfig)
			return nil
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &clientConn{
		Conn: conn,
	}, nil
}

// MakeTestClientWithoutTLS returns a MySQL client connection without setting
// TLS config to the MySQL client.
func MakeTestClientWithoutTLS(addr string, routeToDatabase tlsca.RouteToDatabase) (TestClientConn, error) {
	conn, err := client.Connect(addr,
		routeToDatabase.Username,
		"",
		routeToDatabase.Database,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &clientConn{
		Conn: conn,
	}, nil
}

// UserEvent represents a user activation/deactivation event.
type UserEvent struct {
	// TeleportUser is the Teleport username.
	TeleportUser string
	// DatabaseUser is the in-database username.
	DatabaseUser string
	// Roles are the user Roles.
	Roles []string
	// Active is whether user activated or deactivated.
	Active bool
}

// TestServer is a test MySQL server used in functional database
// access tests.
type TestServer struct {
	cfg           common.TestServerConfig
	listener      net.Listener
	port          string
	tlsConfig     *tls.Config
	log           *slog.Logger
	handler       *testHandler
	serverVersion string

	// serverConnsMtx is a mutex that guards serverConns.
	serverConnsMtx sync.Mutex
	// serverConns holds all connections created by the server.
	serverConns []*server.Conn
}

// TestServerOption allows to set test server options.
type TestServerOption func(*TestServer)

// WithServerVersion sets the test MySQL server version.
func WithServerVersion(serverVersion string) TestServerOption {
	return func(ts *TestServer) {
		ts.serverVersion = serverVersion
	}
}

// NewTestServer returns a new instance of a test MySQL server.
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

	listener := config.Listener
	if config.ListenTLS {
		listener = tls.NewListener(listener, tlsConfig)
	}

	log := utils.NewSlogLoggerForTests().With(
		teleport.ComponentKey, defaults.ProtocolMySQL,
		"name", config.Name,
	)
	server := &TestServer{
		cfg:      config,
		listener: listener,
		port:     port,
		log:      log,
		handler: &testHandler{
			log:          log,
			userEventsCh: make(chan UserEvent, 100),
			usersMapping: make(map[string]string),
		},
	}

	if !config.ListenTLS {
		server.tlsConfig = tlsConfig
	}

	for _, o := range opts {
		o(server)
	}
	return server, nil
}

// Serve starts serving client connections.
func (s *TestServer) Serve() error {
	ctx := context.Background()
	s.log.DebugContext(ctx, "Starting test MySQL server", "listen_addr", s.listener.Addr())
	defer s.log.DebugContext(ctx, "Test MySQL server stopped")
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if utils.IsOKNetworkError(err) {
				return nil
			}
			s.log.ErrorContext(ctx, "Failed to accept connection", "error", err)
			continue
		}
		s.log.DebugContext(ctx, "Accepted connection")
		go func() {
			defer s.log.DebugContext(ctx, "Connection done")
			defer conn.Close()
			err = s.handleConnection(conn)
			if err != nil {
				s.log.ErrorContext(ctx, "Failed to handle connection", "error", err)
			}
		}()
	}
}

func (s *TestServer) handleConnection(conn net.Conn) error {
	var creds server.CredentialProvider = &credentialProvider{}
	if s.cfg.AuthToken != "" {
		creds = &testCredentialProvider{
			credentials: map[string]string{
				s.cfg.AuthUser: s.cfg.AuthToken,
			},
		}
	}
	serverConn, err := server.NewCustomizedConn(
		conn,
		server.NewServer(
			s.serverVersion,
			mysql.DEFAULT_COLLATION_ID,
			mysql.AUTH_NATIVE_PASSWORD,
			nil,
			s.tlsConfig),
		creds,
		s.handler)
	if err != nil {
		return trace.Wrap(err)
	}

	s.serverConnsMtx.Lock()
	s.serverConns = append(s.serverConns, serverConn)
	s.serverConnsMtx.Unlock()

	for {
		if serverConn.Closed() {
			return nil
		}
		err = serverConn.HandleCommand()
		if err != nil {
			return trace.Wrap(err)
		}
	}
}

// testCredentialProvider is used in tests that simulate MySQL password auth
// (e.g. when using IAM auth tokens).
type testCredentialProvider struct {
	credentials map[string]string
}

// CheckUsername returns true is specified MySQL user account exists.
func (p *testCredentialProvider) CheckUsername(username string) (bool, error) {
	_, ok := p.credentials[username]
	return ok, nil
}

// GetCredential returns credentials for the specified MySQL user account.
func (p *testCredentialProvider) GetCredential(username string) (string, bool, error) {
	password, ok := p.credentials[username]
	return password, ok, nil
}

// Port returns the port server is listening on.
func (s *TestServer) Port() string {
	return s.port
}

// QueryCount returns the number of queries the server has received.
func (s *TestServer) QueryCount() uint32 {
	return atomic.LoadUint32(&s.handler.queryCount)
}

// Close closes the server listener.
func (s *TestServer) Close() error {
	return s.listener.Close()
}

// ConnsClosed returns true if all connections has been correctly closed (message COM_QUIT), false otherwise.
func (s *TestServer) ConnsClosed() bool {
	s.serverConnsMtx.Lock()
	defer s.serverConnsMtx.Unlock()

	for _, conn := range s.serverConns {
		if !conn.Closed() {
			return false
		}
	}

	return true
}

// UserEventsCh returns channel that receives user activate/deactivate events.
func (s *TestServer) UserEventsCh() <-chan UserEvent {
	return s.handler.userEventsCh
}

type testHandler struct {
	server.EmptyHandler
	log *slog.Logger
	// queryCount keeps track of the number of queries the server has received.
	queryCount uint32

	userEventsCh chan UserEvent
	// usersMapping maps in-database username to Teleport username.
	usersMapping   map[string]string
	usersMappingMu sync.Mutex
}

func (h *testHandler) HandleQuery(query string) (*mysql.Result, error) {
	h.log.DebugContext(context.Background(), "Received query", "query", query)
	atomic.AddUint32(&h.queryCount, 1)

	// When getting a "show tables" query, construct the response in a way
	// which previously caused server packets parsing logic to fail.
	if query == "show tables" {
		resultSet, err := mysql.BuildSimpleTextResultset(
			[]string{"Tables_in_test"},
			[][]any{
				// In raw bytes, this table name starts with 0x11 which used to
				// cause server packet parsing issues since it clashed with
				// COM_CHANGE_USER packet type.
				{"metadata_md_table"},
			})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &mysql.Result{
			Resultset: resultSet,
		}, nil
	}

	return newTestQueryResponse(), nil
}

func (h *testHandler) HandleStmtPrepare(prepare string) (int, int, any, error) {
	params := strings.Count(prepare, "?")
	return params, 0, nil, nil
}
func (h *testHandler) HandleStmtExecute(_ any, query string, args []any) (*mysql.Result, error) {
	h.log.DebugContext(context.Background(), "Received execute statement with args", "query", query, "args", args)
	if strings.HasPrefix(query, "CALL ") {
		return h.handleCallProcedure(query, args)
	}
	return newTestQueryResponse(), nil
}

func (h *testHandler) handleCallProcedure(query string, args []any) (*mysql.Result, error) {
	query = strings.TrimSpace(strings.TrimPrefix(query, "CALL"))
	openBracketIndex := strings.IndexByte(query, '(')
	endBracketIndex := strings.LastIndexByte(query, ')')
	if openBracketIndex < 0 || endBracketIndex < 0 {
		return nil, trace.BadParameter("invalid query: %v", query)
	}

	procedureName := query[:openBracketIndex]
	switch procedureName {
	case activateUserProcedureName:
		if len(args) != 2 {
			return nil, trace.BadParameter("invalid number of parameters: %v", args)
		}
		databaseUserBytes, ok := args[0].([]byte)
		if !ok {
			return nil, trace.BadParameter("invalid database user: %v", args[0])
		}
		detailsBytes, ok := args[1].([]byte)
		if !ok {
			return nil, trace.BadParameter("invalid details: %v", args[1])
		}
		details := activateUserDetails{}
		err := json.Unmarshal(detailsBytes, &details)
		if err != nil {
			return nil, trace.BadParameter("invalid JSON: %v", err)
		}

		// Update mapping and send event.
		databaseUser := string(databaseUserBytes)
		h.usersMappingMu.Lock()
		defer h.usersMappingMu.Unlock()
		h.usersMapping[databaseUser] = details.Attributes.User
		h.userEventsCh <- UserEvent{
			DatabaseUser: databaseUser,
			TeleportUser: h.usersMapping[databaseUser],
			Roles:        details.Roles,
			Active:       true,
		}

	case deactivateUserProcedureName, deleteUserProcedureName:
		if len(args) != 1 {
			return nil, trace.BadParameter("invalid number of parameters: %v", args)
		}
		databaseUserBytes, ok := args[0].([]byte)
		if !ok {
			return nil, trace.BadParameter("invalid database user: %v", args[0])
		}

		// Send event.
		h.usersMappingMu.Lock()
		defer h.usersMappingMu.Unlock()
		h.userEventsCh <- UserEvent{
			DatabaseUser: string(databaseUserBytes),
			TeleportUser: h.usersMapping[string(databaseUserBytes)],
			Active:       false,
		}
	}
	return newTestQueryResponse(), nil
}

// TestQueryResponse is what test MySQL server returns to every query.
var TestQueryResponse = &mysql.Result{
	InsertId:     1,
	AffectedRows: 0,
}

func newTestQueryResponse() *mysql.Result {
	return &mysql.Result{
		InsertId:     1,
		AffectedRows: 0,
	}
}
