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

package sqlserver

import (
	"context"
	"io"
	"log/slog"
	"net"
	"strconv"

	"github.com/gravitational/trace"
	mssql "github.com/microsoft/go-mssqldb"
	"github.com/microsoft/go-mssqldb/msdsn"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/sqlserver/protocol"
	"github.com/gravitational/teleport/lib/utils"
)

// MakeTestClient returns SQL Server client used in tests.
func MakeTestClient(ctx context.Context, config common.TestClientConfig) (*mssql.Conn, error) {
	host, port, err := net.SplitHostPort(config.Address)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	portI, err := strconv.ParseUint(port, 10, 64)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connector := mssql.NewConnectorConfig(msdsn.Config{
		Host:       host,
		Port:       portI,
		User:       config.RouteToDatabase.Username,
		Database:   config.RouteToDatabase.Database,
		Encryption: msdsn.EncryptionDisabled,
		Protocols:  []string{"tcp"},
	}, nil)

	conn, err := connector.Connect(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	mssqlConn, ok := conn.(*mssql.Conn)
	if !ok {
		return nil, trace.BadParameter("expected *mssql.Conn, got: %T", conn)
	}

	return mssqlConn, nil
}

// TestConnector is used in tests to mock connections to SQL Server.
type TestConnector struct{}

// Connect simulates successful connection to a SQL Server.
func (c *TestConnector) Connect(ctx context.Context, sessionCtx *common.Session, loginPacket *protocol.Login7Packet) (io.ReadWriteCloser, []mssql.Token, error) {
	host, port, err := net.SplitHostPort(sessionCtx.Database.GetURI())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	portI, err := strconv.ParseUint(port, 10, 64)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Pass all login options from the client to the server.
	options := msdsn.LoginOptions{
		OptionFlags1: loginPacket.OptionFlags1(),
		OptionFlags2: loginPacket.OptionFlags2(),
		TypeFlags:    loginPacket.TypeFlags(),
	}

	connector := mssql.NewConnectorConfig(msdsn.Config{
		Host:         host,
		Port:         portI,
		LoginOptions: options,
		Protocols:    []string{"tcp"},
	}, nil)

	conn, err := connector.Connect(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	mssqlConn, ok := conn.(*mssql.Conn)
	if !ok {
		return nil, nil, trace.BadParameter("expected *mssql.Conn, got: %T", conn)
	}

	return mssqlConn.GetUnderlyingConn(), mssqlConn.GetLoginFlags(), nil
}

// TestServer is a test MSServer server used in functional database
// access tests.
type TestServer struct {
	cfg      common.TestServerConfig
	listener net.Listener
	port     string
	log      *slog.Logger
}

// NewTestServer returns a new instance of a test MSServer.
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
	log := utils.NewSlogLoggerForTests().With(
		teleport.ComponentKey, defaults.ProtocolSQLServer,
		"name", config.Name,
	)
	server := &TestServer{
		cfg:      config,
		listener: config.Listener,
		port:     port,
		log:      log,
	}
	return server, nil
}

// Port returns SQL Server port.
func (s *TestServer) Port() string {
	return s.port
}

// Serve starts serving client connections.
func (s *TestServer) Serve() error {
	ctx := context.Background()
	s.log.DebugContext(ctx, "Starting test MSServer server", "listen_addr", s.listener.Addr())
	defer s.log.DebugContext(ctx, "Test MSServer server stopped")
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
	if err := handleLogin(conn); err != nil {
		return trace.Wrap(err)
	}
	for {
		basicPacket, err := protocol.ReadPacket(conn)
		if err != nil {
			return trace.Wrap(err)
		}
		packet, err := protocol.ToSQLPacket(basicPacket)
		if err != nil {
			return trace.Wrap(err)
		}

		switch packet.(type) {
		case *protocol.SQLBatch:
			if _, err = conn.Write(mockSQLBatchServerResp); err != nil {
				return trace.Wrap(err)
			}
		}
	}
}

func handleLogin(conn net.Conn) error {
	if _, err := protocol.ReadPreLoginPacket(conn); err != nil {
		return trace.Wrap(err)
	}
	if err := protocol.WritePreLoginResponse(conn); err != nil {
		return trace.Wrap(err)
	}
	if _, err := protocol.ReadLogin7Packet(conn); err != nil {
		return trace.Wrap(err)
	}
	if _, err := conn.Write(mockLoginServerResp); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Close closes the server listener.
func (s *TestServer) Close() error {
	return s.listener.Close()
}

var (
	mockLoginServerResp = []byte{
		0x04, 0x01, 0x00, 0xa7, 0x00, 0x00, 0x00, 0x00, 0xe3, 0x1b, 0x00, 0x01, 0x06, 0x6d, 0x00, 0x61,
		0x00, 0x73, 0x00, 0x74, 0x00, 0x65, 0x00, 0x72, 0x00, 0x06, 0x6d, 0x00, 0x61, 0x00, 0x73, 0x00,
		0x74, 0x00, 0x65, 0x00, 0x72, 0x00, 0xe3, 0x08, 0x00, 0x07, 0x05, 0x09, 0x04, 0xd0, 0x00, 0x34,
		0x00, 0xe3, 0x17, 0x00, 0x02, 0x0a, 0x75, 0x00, 0x73, 0x00, 0x5f, 0x00, 0x65, 0x00, 0x6e, 0x00,
		0x67, 0x00, 0x6c, 0x00, 0x69, 0x00, 0x73, 0x00, 0x68, 0x00, 0x00, 0xad, 0x36, 0x00, 0x01, 0x74,
		0x00, 0x00, 0x04, 0x16, 0x4d, 0x00, 0x69, 0x00, 0x63, 0x00, 0x72, 0x00, 0x6f, 0x00, 0x73, 0x00,
		0x6f, 0x00, 0x66, 0x00, 0x74, 0x00, 0x20, 0x00, 0x53, 0x00, 0x51, 0x00, 0x4c, 0x00, 0x20, 0x00,
		0x53, 0x00, 0x65, 0x00, 0x72, 0x00, 0x76, 0x00, 0x65, 0x00, 0x72, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x0f, 0x00, 0x0f, 0xe9, 0xe3, 0x13, 0x00, 0x04, 0x04, 0x34, 0x00, 0x30, 0x00, 0x39, 0x00, 0x36,
		0x00, 0x04, 0x34, 0x00, 0x30, 0x00, 0x39, 0x00, 0x36, 0x00, 0xfd, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	mockSQLBatchServerResp = []byte{
		0x04, 0x01, 0x00, 0x2b, 0x00, 0x4a, 0x01, 0x00, 0xe3, 0x03, 0x00, 0x12, 0x00, 0x00, 0x81, 0x01,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x20, 0x00, 0x38, 0x00, 0xd1, 0x01, 0x00, 0x00, 0x00, 0xfd, 0x10,
		0x00, 0xc1, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
)
