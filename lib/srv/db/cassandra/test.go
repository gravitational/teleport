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

package cassandra

import (
	"bytes"
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/datastax/go-cassandra-native-protocol/client"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/gocql/gocql"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/utils"
)

// Session alias for easier use.
type Session = gocql.Session

// ClientOptionsParams is a struct for client configuration options.
type ClientOptionsParams struct {
	Username string
}

// ClientOptions allows setting test client options.
type ClientOptions func(*ClientOptionsParams)

// WithCassandraUsername set the username used during cassandra login.
func WithCassandraUsername(username string) ClientOptions {
	return func(params *ClientOptionsParams) {
		params.Username = username
	}
}

// MakeTestClient returns Cassandra client connection according to the provided
// parameters.
func MakeTestClient(_ context.Context, config common.TestClientConfig, opts ...ClientOptions) (*Session, error) {
	tlsConfig, err := common.MakeTestClientTLSConfig(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clientOptions := &ClientOptionsParams{}
	for _, opt := range opts {
		opt(clientOptions)
	}

	cluster := gocql.NewCluster(config.Address)
	cluster.SslOpts = &gocql.SslOptions{
		Config: tlsConfig,
	}
	cluster.DisableInitialHostLookup = true
	cluster.ConnectTimeout = 5 * time.Second
	cluster.Timeout = 5 * time.Second
	cluster.ProtoVersion = 4
	cluster.Authenticator = gocql.PasswordAuthenticator{
		Username: "cassandra",
		Password: "cassandra",
	}
	session, err := cluster.CreateSession()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

// TestServerOption allows setting test server options.
type TestServerOption func(*TestServer)

type TestServer struct {
	cfg       common.TestServerConfig
	port      string
	tlsConfig *tls.Config
	logger    *slog.Logger
	server    *client.CqlServer
}

// NewTestServer returns a new instance of a test Snowflake server.
func NewTestServer(config common.TestServerConfig, opts ...TestServerOption) (*TestServer, error) {
	address := "localhost:0"
	tlsConfig, err := common.MakeTestServerTLSConfig(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	server := client.NewCqlServer(address, &client.AuthCredentials{
		Password: "cassandra",
		Username: "cassandra",
	})
	if config.Listener != nil {
		server.Listener = tls.NewListener(config.Listener, tlsConfig)
	}

	server.RequestHandlers = []client.RequestHandler{
		client.HandshakeHandler,
		handleMessageOption,
		handleMessageQuery,
		handleMessagePrepare,
		handleMessageExecute,
		handleMessageBatch,
		handleMessageRegister,
	}

	server.TLSConfig = tlsConfig
	if err := server.Start(context.Background()); err != nil {
		return nil, trace.Wrap(err)
	}

	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	testServer := &TestServer{
		cfg:       config,
		port:      port,
		tlsConfig: tlsConfig,
		server:    server,
		logger: utils.NewSlogLoggerForTests().With(
			teleport.ComponentKey, defaults.ProtocolCassandra,
			"name", config.Name,
		),
	}
	for _, opt := range opts {
		opt(testServer)
	}
	return testServer, nil
}

// Serve starts serving client connections.
func (s *TestServer) Serve() error {
	return s.server.Start(context.Background())
}

// Close closes the server.
func (s *TestServer) Close() error {
	return s.server.Close()
}

func (s *TestServer) Port() string {
	return s.port
}

func handleMessageQuery(request *frame.Frame, conn *client.CqlServerConnection, ctx client.RequestHandlerContext) *frame.Frame {
	switch msg := request.Body.Message.(type) {
	case *message.Query:
		lQuery := strings.TrimSpace(strings.ToLower(msg.Query))
		switch lQuery {
		case "select * from system.local where key='local'":
			return frame.NewFrame(
				request.Header.Version,
				request.Header.StreamId,
				&message.RowsResult{
					Metadata: &message.RowsMetadata{
						ColumnCount: 7,
						Columns: []*message.ColumnMetadata{
							{Keyspace: "system", Table: "local", Name: "key", Index: 0, Type: datatype.Varchar},
							{Keyspace: "system", Table: "local", Name: "bootstrapped", Index: 0, Type: datatype.Varchar},
							{Keyspace: "system", Table: "local", Name: "broadcast_address", Index: 0, Type: datatype.Inet},
							{Keyspace: "system", Table: "local", Name: "broadcast_port", Index: 0, Type: datatype.Inet},
							{Keyspace: "system", Table: "local", Name: "cluster_name", Index: 0, Type: datatype.Varchar},
							{Keyspace: "system", Table: "local", Name: "cql_version", Index: 0, Type: datatype.Varchar},
							{Keyspace: "system", Table: "local", Name: "data_center", Index: 0, Type: datatype.Varchar},
						},
					},
					Data: message.RowSet{
						{
							[]byte("local"),
							[]byte("COMPLETED"),
							[]byte{192, 168, 0, 185},
							[]byte{0, 0, 27, 88},
							[]byte("Test Cluster"),
							[]byte("3.4.5"),
							[]byte("datacenter1"),
						},
					},
				},
			)
		}
	}
	return nil
}

func handleMessagePrepare(request *frame.Frame, conn *client.CqlServerConnection, ctx client.RequestHandlerContext) *frame.Frame {
	switch msg := request.Body.Message.(type) {
	case *message.Prepare:
		lQuery := strings.TrimSpace(strings.ToLower(msg.Query))
		switch lQuery {
		case "select * from system_schema.keyspaces":
			return frame.NewFrame(
				request.Header.Version,
				request.Header.StreamId,
				&message.PreparedResult{
					PreparedQueryId: []byte{211, 78, 99, 137, 52, 114, 28, 59, 205, 105, 147, 63, 153, 42, 0, 203},
					ResultMetadata: &message.RowsMetadata{
						ColumnCount: 3,
					},
				})
		case "select cluster_name from system.local":
			return frame.NewFrame(
				request.Header.Version,
				request.Header.StreamId,
				&message.PreparedResult{
					PreparedQueryId: []byte{48, 60, 203, 12, 80, 82, 198, 204, 96, 125, 128, 97, 211, 209, 122, 35},
					ResultMetadata: &message.RowsMetadata{
						ColumnCount: 1,
						Columns: []*message.ColumnMetadata{
							{
								Keyspace: "system",
								Table:    "local",
								Name:     "cluster_name",
								Index:    0,
								Type:     datatype.Varchar,
							},
						},
					},
				})
		}
	}
	return nil
}

func handleMessageExecute(request *frame.Frame, conn *client.CqlServerConnection, ctx client.RequestHandlerContext) *frame.Frame {
	switch msg := request.Body.Message.(type) {
	case *message.Execute:
		switch {
		case bytes.Equal(msg.QueryId, []byte{211, 78, 99, 137, 52, 114, 28, 59, 205, 105, 147, 63, 153, 42, 0, 203}):
			return frame.NewFrame(
				request.Header.Version,
				request.Header.StreamId,
				&message.RowsResult{
					Metadata: &message.RowsMetadata{
						ColumnCount: 3,
					},
					Data: message.RowSet{
						{
							[]byte("system_auth"),
							[]byte("org.apache.cassandra.locator.SimpleStrategy"),
							[]byte("1"),
						},
						{
							[]byte("system_schema"),
							[]byte("org.apache.cassandra.locator.LocalStrategy"),
							[]byte("1"),
						},
						{
							[]byte("system_distributed"),
							[]byte("org.apache.cassandra.locator.SimpleStrategy"),
							[]byte("3"),
						},
						{
							[]byte("system"),
							[]byte("org.apache.cassandra.locator.LocalStrategy"),
							[]byte("1"),
						},
						{
							[]byte("system_traces"),
							[]byte("org.apache.cassandra.locator.SimpleStrategy"),
							[]byte("2"),
						},
					},
				},
			)
		case bytes.Equal(msg.QueryId, []byte{48, 60, 203, 12, 80, 82, 198, 204, 96, 125, 128, 97, 211, 209, 122, 35}):
			return frame.NewFrame(
				request.Header.Version,
				request.Header.StreamId,
				&message.RowsResult{
					Metadata: &message.RowsMetadata{
						ColumnCount: 1,
					},
					Data: message.RowSet{
						{
							[]byte("Test Cluster"),
						},
					},
				},
			)
		}
	}
	return nil
}

func handleMessageRegister(request *frame.Frame, conn *client.CqlServerConnection, ctx client.RequestHandlerContext) *frame.Frame {
	switch request.Body.Message.(type) {
	case *message.Register:
		return frame.NewFrame(
			request.Header.Version,
			request.Header.StreamId,
			&message.Ready{},
		)
	default:
		return nil
	}
}

func handleMessageBatch(request *frame.Frame, conn *client.CqlServerConnection, ctx client.RequestHandlerContext) *frame.Frame {
	codec := frame.NewRawCodec()
	switch request.Body.Message.(type) {
	case *message.Batch:
		resp := &frame.RawFrame{
			Header: &frame.Header{
				IsResponse: true,
				Version:    request.Header.Version,
				StreamId:   request.Header.StreamId,
				OpCode:     primitive.OpCodeResult,
				BodyLength: 4,
			},
			Body: []byte{0, 0, 0, 1},
		}
		responseFrame, err := codec.ConvertFromRawFrame(resp)
		if err != nil {
			slog.ErrorContext(context.Background(), "Error converting raw frame to frame", "error", err)
			return nil
		}
		return responseFrame
	default:
		return nil
	}
}

func handleMessageOption(request *frame.Frame, conn *client.CqlServerConnection, ctx client.RequestHandlerContext) *frame.Frame {
	if _, ok := request.Body.Message.(*message.Options); ok {
		return frame.NewFrame(
			request.Header.Version,
			request.Header.StreamId,
			&message.Supported{},
		)
	}
	return nil
}
