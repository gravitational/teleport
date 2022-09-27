/*
Copyright 2022 Gravitational, Inc.

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

package cassandra

import (
	"bytes"
	"context"
	"crypto/tls"
	"net"
	"reflect"
	"strings"
	"time"
	"unsafe"

	"github.com/datastax/go-cassandra-native-protocol/client"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/gocql/gocql"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
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
	log       logrus.FieldLogger
	server    *client.CqlServer
}

// unsafeGetServerListener is a hack to get the listener from the server.
// Allows to start server on port random port and obtain the port number from
// private client.CqlServer field.
func unsafeGetServerListener(server *client.CqlServer) net.Listener {
	v := reflect.ValueOf(server)
	ve := reflect.Indirect(v)
	lf := ve.FieldByName("listener")
	ptr := reflect.NewAt(lf.Type(), unsafe.Pointer(lf.UnsafeAddr())).Elem().Interface()
	return ptr.(net.Listener)
}

// NewTestServer returns a new instance of a test Snowflake server.
func NewTestServer(config common.TestServerConfig, opts ...TestServerOption) (*TestServer, error) {
	address := "localhost:0"
	tlsConfig, err := common.MakeTestServerTLSConfig(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig.InsecureSkipVerify = true

	server := client.NewCqlServer(address, &client.AuthCredentials{
		Password: "cassandra",
		Username: "cassandra",
	})

	server.RequestHandlers = []client.RequestHandler{
		client.HandshakeHandler,
		handleMessageOption,
		handleMessageQuery,
		handleMessagePrepare,
		handleMessageExecute,
		handleMessageBatch,
		handleMessageRegister,
		func(request *frame.Frame, conn *client.CqlServerConnection, ctx client.RequestHandlerContext) (response *frame.Frame) {
			return nil
		},
	}

	server.TLSConfig = tlsConfig
	if err := server.Start(context.Background()); err != nil {
		return nil, trace.Wrap(err)
	}

	_, port, err := net.SplitHostPort(unsafeGetServerListener(server).Addr().String())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	testServer := &TestServer{
		cfg:       config,
		port:      port,
		tlsConfig: tlsConfig,
		server:    server,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: defaults.ProtocolCassandra,
			"name":          config.Name,
		}),
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
							//{Keyspace: "system", Table: "local", Name: "gossip_generation", Index: 0, Type: datatype.Int},
							//{Keyspace: "system", Table: "local", Name: "host_id", Index: 0, Type: datatype.Uuid},
							//{Keyspace: "system", Table: "local", Name: "listen_address", Index: 0, Type: datatype.Inet},
							//{Keyspace: "system", Table: "local", Name: "listen_port", Index: 0, Type: datatype.Int},
							//{Keyspace: "system", Table: "local", Name: "native_protocol_version", Index: 0, Type: datatype.Varchar},
							//{Keyspace: "system", Table: "local", Name: "partitioner", Index: 0, Type: datatype.Varchar},
							//{Keyspace: "system", Table: "local", Name: "rack", Index: 0, Type: datatype.Varchar},
							//{Keyspace: "system", Table: "local", Name: "release_version", Index: 0, Type: datatype.Varchar},
							//{Keyspace: "system", Table: "local", Name: "rpc_address", Index: 0, Type: datatype.Inet},
							//{Keyspace: "system", Table: "local", Name: "rpc_port", Index: 0, Type: datatype.Int},
							//{Keyspace: "system", Table: "local", Name: "schema_version", Index: 0, Type: datatype.Uuid},
							//{Keyspace: "system", Table: "local", Name: "tokens", Index: 0, Type: datatype.Varchar},
							//{Keyspace: "system", Table: "local", Name: "truncated_at", Index: 0, Type: &datatype.Map{KeyType: datatype.Uuid, ValueType: datatype.Blob}},
						},
					},
					Data: message.RowSet{
						{
							[]byte("local"),          // 0
							[]byte("COMPLETED"),      // 1
							[]byte{192, 168, 0, 185}, // 2
							[]byte{0, 0, 27, 88},     // 3
							[]byte("Test Cluster"),   // 4
							[]byte("3.4.5"),          // 5
							[]byte("datacenter1"),
							//[]byte{98, 131, 225, 57},
							//[]byte{115, 186, 31, 38, 108, 6, 70, 99, 137, 7, 214, 86, 107, 238, 190, 67},
							//[]byte{192, 168, 0, 185},
							//[]byte{0, 0, 27, 88},
							//[]byte{53},
							//[]byte("org.apache.cassandra.dht.Murmur3Partitioner"),
							//[]byte("rack1"),
							//[]byte("4.0.1"),
							//[]byte{0, 0, 0, 0},
							//[]byte{0, 0, 35, 82},
							//[]byte{34, 7, 194, 169, 245, 152, 57, 113, 152, 57, 113, 152, 107, 41, 38, 224, 158, 35, 157},
							//[]byte{0, 0, 0, 16, 0, 0, 0, 20, 45, 49, 55, 50, 48, 51, 53, 55, 51, 57, 49, 55, 53, 48, 54, 50, 51, 53, 48, 54, 0, 0, 0, 20, 45, 50, 57, 54, 51, 56, 55, 55, 51, 57, 55, 52, 51, 49, 50, 49, 53, 52, 56, 48, 0, 0, 0, 20, 45, 52, 48, 52, 51, 50, 53, 48, 50, 48, 52, 56, 55, 56, 56, 49, 55, 49, 51, 55, 0, 0, 0, 20, 45, 52, 55, 55, 57, 49, 54, 53, 56, 57, 55, 53, 56, 57, 55, 49, 48, 55, 55, 56, 0, 0, 0, 20, 45, 53, 56, 53, 52, 57, 50, 49, 55, 55, 56, 56, 49, 53, 50, 57, 48, 51, 49, 56, 0, 0, 0, 20, 45, 54, 54, 55, 50, 53, 57, 51, 55, 49, 49, 52, 50, 55, 54, 52, 49, 49, 49, 50, 0, 0, 0, 19, 45, 55, 53, 50, 57, 57, 51, 49, 52, 56, 54, 55, 50, 48, 56, 56, 48, 48, 56, 0, 0, 0, 20, 45, 55, 54, 57, 48, 52, 52, 53, 49, 49, 54, 49, 55, 48, 48, 54, 56, 49, 50, 56, 0, 0, 0, 20, 45, 56, 55, 54, 49, 57, 52, 53, 55, 51, 50, 53, 49, 48, 49, 48, 48, 57, 52, 49, 0, 0, 0, 19, 50, 52, 53, 54, 48, 51, 52, 54, 57, 57, 53, 51, 56, 56, 53, 51, 57, 50, 54, 0, 0, 0, 19, 51, 52, 51, 52, 49, 56, 48, 49, 53, 50, 48, 53, 52, 57, 54, 56, 48, 48, 50, 0, 0, 0, 19, 53, 50, 49, 50, 56, 49, 52, 52, 53, 50, 56, 55, 56, 53, 53, 53, 49, 51, 48, 0, 0, 0, 19, 54, 51, 48, 56, 56, 53, 54, 51, 53, 51, 49, 49, 51, 52, 57, 51, 52, 49, 57, 0, 0, 0, 19, 55, 52, 48, 48, 49, 53, 51, 52, 54, 51, 56, 48, 53, 55, 48, 53, 54, 55, 56, 0, 0, 0, 19, 56, 50, 52, 52, 53, 51, 48, 49, 49, 49, 50, 57, 54, 56, 54, 55, 52, 51, 54, 0, 0, 0, 18, 56, 51, 55, 48, 49, 49, 52, 48, 51, 56, 52, 49, 56, 49, 48, 54, 50, 57},
							//[]byte{0, 0, 0, 2, 0, 0, 0, 16, 23, 108, 57, 205, 185, 61, 51, 165, 162, 24, 142, 176, 106, 86, 246, 110, 0, 0, 0, 20, 0, 0, 1, 128, 211, 39, 197, 153, 0, 0, 0, 28, 0, 0, 1, 128, 211, 39, 200, 217, 0, 0, 0, 16, 97, 143, 129, 123, 0, 95, 54, 120, 184, 164, 83, 243, 147, 11, 142, 134, 0, 0, 0, 20, 0, 0, 1, 128, 211, 39, 197, 153, 0, 0, 0, 28, 0, 0, 1, 128, 211, 39, 200, 44},
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
			logrus.Errorf("Error converting raw frame to frame: %v", err)
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
