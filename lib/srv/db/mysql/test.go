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

package mysql

import (
	"crypto/tls"
	"encoding/json"
	"net"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/server"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// MakeTestClient returns MySQL client connection according to the provided
// parameters.
func MakeTestClient(config common.TestClientConfig) (*client.Conn, error) {
	tlsConfig, err := common.MakeTestClientTLSConfig(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	conn, err := client.Connect(config.Address,
		config.RouteToDatabase.Username,
		"",
		config.RouteToDatabase.Database,
		func(conn *client.Conn) {
			conn.SetTLSConfig(tlsConfig)
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return conn, nil
}

// MakeTestClientWithoutTLS returns a MySQL client connection without setting
// TLS config to the MySQL client.
func MakeTestClientWithoutTLS(addr string, routeToDatabase tlsca.RouteToDatabase) (*client.Conn, error) {
	conn, err := client.Connect(addr,
		routeToDatabase.Username,
		"",
		routeToDatabase.Database,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return conn, nil
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
	log           logrus.FieldLogger
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

	log := logrus.WithFields(logrus.Fields{
		trace.Component: defaults.ProtocolMySQL,
		"name":          config.Name,
	})
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
	s.log.Debugf("Starting test MySQL server on %v.", s.listener.Addr())
	defer s.log.Debug("Test MySQL server stopped.")
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
			err = s.handleConnection(conn)
			if err != nil {
				s.log.Errorf("Failed to handle connection: %v.",
					trace.DebugReport(err))
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
	log logrus.FieldLogger
	// queryCount keeps track of the number of queries the server has received.
	queryCount uint32

	userEventsCh chan UserEvent
	// usersMapping maps in-database username to Teleport username.
	usersMapping   map[string]string
	usersMappingMu sync.Mutex
}

func (h *testHandler) HandleQuery(query string) (*mysql.Result, error) {
	h.log.Debugf("Received query %q.", query)
	atomic.AddUint32(&h.queryCount, 1)

	switch {
	// When getting a "show tables" query, construct the response in a way
	// which previously caused server packets parsing logic to fail.
	case query == "show tables":
		resultSet, err := mysql.BuildSimpleTextResultset(
			[]string{"Tables_in_test"},
			[][]interface{}{
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

	case strings.HasPrefix(query, "CALL "):
		return h.handleCallProcedure(query)
	}
	return TestQueryResponse, nil
}

func (h *testHandler) handleCallProcedure(query string) (*mysql.Result, error) {
	query = strings.TrimSpace(strings.TrimPrefix(query, "CALL"))
	openBracketIndex := strings.IndexByte(query, '(')
	endBracketIndex := strings.LastIndexByte(query, ')')
	if openBracketIndex < 0 || endBracketIndex < 0 {
		return nil, trace.BadParameter("invalid query: %v", query)
	}

	procedureName := query[:openBracketIndex]
	parameters := query[openBracketIndex+1 : endBracketIndex]
	switch procedureName {
	case activateUserProcedureName:
		databaseUser, detailsJSON, ok := strings.Cut(parameters, ",")
		if !ok {
			return nil, trace.BadParameter("invalid parameters: %v", parameters)
		}
		databaseUser = strings.Trim(databaseUser, "'")

		// Trim and de-escape.
		detailsJSON = strings.ReplaceAll(strings.Trim(strings.TrimSpace(detailsJSON), "'"), "\\\\", "\\")
		details := activateUserDetails{}
		err := json.Unmarshal([]byte(detailsJSON), &details)
		if err != nil {
			return nil, trace.BadParameter("invalid JSON: %v", err)
		}

		// Update mapping and send event.
		h.usersMappingMu.Lock()
		defer h.usersMappingMu.Unlock()
		h.usersMapping[databaseUser] = details.Attributes.User
		h.userEventsCh <- UserEvent{
			DatabaseUser: databaseUser,
			TeleportUser: h.usersMapping[databaseUser],
			Roles:        details.Roles,
			Active:       true,
		}

	case deactivateUserProcedureName:
		databaseUser := strings.Trim(parameters, "'")

		// Send event.
		h.usersMappingMu.Lock()
		defer h.usersMappingMu.Unlock()
		h.userEventsCh <- UserEvent{
			DatabaseUser: databaseUser,
			TeleportUser: h.usersMapping[databaseUser],
			Active:       false,
		}
	}
	return TestQueryResponse, nil
}

// TestQueryResponse is what test MySQL server returns to every query.
var TestQueryResponse = &mysql.Result{
	InsertId:     1,
	AffectedRows: 0,
}
