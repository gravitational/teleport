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
	"strings"
	"time"

	"github.com/datastax/go-cassandra-native-protocol/client"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/gocql/gocql"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// Session alias for easier use.
type Session = gocql.Session

// ClientOptionsParams is a struct for client configuration options.
type ClientOptionsParams struct {
	skipPing bool
}

// ClientOptions allows setting test client options.
type ClientOptions func(*ClientOptionsParams)

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
		//EnableHostVerification: true,
		Config: tlsConfig,
	}
	cluster.DisableInitialHostLookup = true
	cluster.ConnectTimeout = 50 * time.Second
	cluster.Timeout = 50 * time.Second
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
	listener  net.Listener
	port      string
	tlsConfig *tls.Config
	log       logrus.FieldLogger
	server    *client.CqlServer
}

// NewTestServer returns a new instance of a test Snowflake server.
func NewTestServer(config common.TestServerConfig, opts ...TestServerOption) (*TestServer, error) {
	address := "localhost:0"
	if config.Address != "" {
		address = config.Address
	}
	tlsConfig, err := common.MakeTestServerTLSConfig(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig.InsecureSkipVerify = true

	listener, err := tls.Listen("tcp", address, tlsConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	//TODO(jakule): Hacky way to get a free port.
	addr := listener.Addr()
	listener.Close()

	server := client.NewCqlServer(addr.String(), &client.AuthCredentials{
		Password: "cassandra",
		Username: "cassandra",
	})

	server.TLSConfig = tlsConfig
	if err := server.Start(context.Background()); err != nil {
		return nil, trace.Wrap(err)
	}

	testServer := &TestServer{
		cfg:       config,
		listener:  listener,
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
	err := s.processConnection(s.server)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *TestServer) processConnection(server *client.CqlServer) error {
	for {
		conn, err := server.AcceptAny()
		if err != nil {
			return trace.Wrap(err)
		}

		if err := conn.AcceptHandshake(); err != nil {
			return trace.Wrap(err)
		}

		go func() {
			if err := s.processRequest(conn); err != nil {
				s.log.WithError(err).Error("failed to process request")
			}
		}()
	}
}

func (s *TestServer) processRequest(conn *client.CqlServerConnection) error {
	codec := frame.NewRawCodec()
	for {
		recvFrame, err := conn.Receive()
		if err != nil {
			return trace.Wrap(err)
		}

		var responseFrame *frame.Frame

		switch recvFrame.Header.OpCode {
		case primitive.OpCodeRegister:
			responseFrame = frame.NewFrame(recvFrame.Header.Version, recvFrame.Header.StreamId, &message.Ready{})
		case primitive.OpCodeOptions:
			responseFrame = frame.NewFrame(recvFrame.Header.Version, recvFrame.Header.StreamId, &message.Supported{})
		case primitive.OpCodePrepare:
			prepare, ok := recvFrame.Body.Message.(*message.Prepare)
			if !ok {
				return trace.BadParameter("failed to cast, expected *message.Prepare, got %T", recvFrame.Body.Message)
			}
			lQuery := strings.ToLower(prepare.Query)
			lQuery = strings.TrimSpace(lQuery)
			switch lQuery {
			case "select * from system_schema.keyspaces": // query ID: 211, 78, 99, 137, 52, 114, 28, 59, 205, 105, 147, 63, 153, 42, 0, 203
				rawFrame := &frame.RawFrame{
					Header: &frame.Header{
						IsResponse: true,
						Version:    recvFrame.Header.Version,
						StreamId:   recvFrame.Header.StreamId,
						OpCode:     primitive.OpCodeResult,
						BodyLength: 122,
					},
					Body: []byte{0, 0, 0, 4, 0, 16, 211, 78, 99, 137, 52, 114, 28, 59, 205, 105, 147, 63, 153, 42, 0, 203, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 3, 0, 13, 115, 121, 115, 116, 101, 109, 95, 115, 99, 104, 101, 109, 97, 0, 9, 107, 101, 121, 115, 112, 97, 99, 101, 115, 0, 13, 107, 101, 121, 115, 112, 97, 99, 101, 95, 110, 97, 109, 101, 0, 13, 0, 14, 100, 117, 114, 97, 98, 108, 101, 95, 119, 114, 105, 116, 101, 115, 0, 4, 0, 11, 114, 101, 112, 108, 105, 99, 97, 116, 105, 111, 110, 0, 33, 0, 13, 0, 13},
				}
				responseFrame, err = codec.ConvertFromRawFrame(rawFrame)
				if err != nil {
					return trace.Wrap(err)
				}
			case "select cluster_name from system.local":
				rawFrame := &frame.RawFrame{
					Header: &frame.Header{
						IsResponse: true,
						Version:    recvFrame.Header.Version,
						StreamId:   recvFrame.Header.StreamId,
						OpCode:     primitive.OpCodeResult,
						BodyLength: 73,
					},
					Body: []byte{0, 0, 0, 4, 0, 16, 48, 60, 203, 12, 80, 82, 198, 204, 96, 125, 128, 97, 211, 209, 122, 35, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 1, 0, 6, 115, 121, 115, 116, 101, 109, 0, 5, 108, 111, 99, 97, 108, 0, 12, 99, 108, 117, 115, 116, 101, 114, 95, 110, 97, 109, 101, 0, 13},
				}
				responseFrame, err = codec.ConvertFromRawFrame(rawFrame)
				if err != nil {
					return trace.Wrap(err)
				}
			}
		case primitive.OpCodeStartup:
			if _, ok := recvFrame.Body.Message.(*message.Startup); !ok {
				return trace.BadParameter("failed to cast, expected *message.Startup, got %T", recvFrame.Body.Message)
			}
			responseFrame = frame.NewFrame(recvFrame.Header.Version, recvFrame.Header.StreamId, message.NewStartup())
		case primitive.OpCodeQuery:
			query, ok := recvFrame.Body.Message.(*message.Query)
			if !ok {
				return trace.BadParameter("failed to cast, expected *message.Query, got %T", recvFrame.Body.Message)
			}
			lQuery := strings.ToLower(query.Query)
			lQuery = strings.TrimSpace(lQuery)

			switch lQuery {
			case "select * from system.local where key='local'":
				rawFrame := &frame.RawFrame{
					Header: &frame.Header{
						IsResponse: true,
						Version:    recvFrame.Header.Version,
						StreamId:   recvFrame.Header.StreamId,
						OpCode:     primitive.OpCodeResult,
						BodyLength: 1057,
					},
					Body: []byte{0, 0, 0, 2, 0, 0, 0, 1, 0, 0, 0, 20, 0, 6, 115, 121, 115, 116, 101, 109, 0, 5, 108, 111, 99, 97, 108, 0, 3, 107, 101, 121, 0, 13, 0, 12, 98, 111, 111, 116, 115, 116, 114, 97, 112, 112, 101, 100, 0, 13, 0, 17, 98, 114, 111, 97, 100, 99, 97, 115, 116, 95, 97, 100, 100, 114, 101, 115, 115, 0, 16, 0, 14, 98, 114, 111, 97, 100, 99, 97, 115, 116, 95, 112, 111, 114, 116, 0, 9, 0, 12, 99, 108, 117, 115, 116, 101, 114, 95, 110, 97, 109, 101, 0, 13, 0, 11, 99, 113, 108, 95, 118, 101, 114, 115, 105, 111, 110, 0, 13, 0, 11, 100, 97, 116, 97, 95, 99, 101, 110, 116, 101, 114, 0, 13, 0, 17, 103, 111, 115, 115, 105, 112, 95, 103, 101, 110, 101, 114, 97, 116, 105, 111, 110, 0, 9, 0, 7, 104, 111, 115, 116, 95, 105, 100, 0, 12, 0, 14, 108, 105, 115, 116, 101, 110, 95, 97, 100, 100, 114, 101, 115, 115, 0, 16, 0, 11, 108, 105, 115, 116, 101, 110, 95, 112, 111, 114, 116, 0, 9, 0, 23, 110, 97, 116, 105, 118, 101, 95, 112, 114, 111, 116, 111, 99, 111, 108, 95, 118, 101, 114, 115, 105, 111, 110, 0, 13, 0, 11, 112, 97, 114, 116, 105, 116, 105, 111, 110, 101, 114, 0, 13, 0, 4, 114, 97, 99, 107, 0, 13, 0, 15, 114, 101, 108, 101, 97, 115, 101, 95, 118, 101, 114, 115, 105, 111, 110, 0, 13, 0, 11, 114, 112, 99, 95, 97, 100, 100, 114, 101, 115, 115, 0, 16, 0, 8, 114, 112, 99, 95, 112, 111, 114, 116, 0, 9, 0, 14, 115, 99, 104, 101, 109, 97, 95, 118, 101, 114, 115, 105, 111, 110, 0, 12, 0, 6, 116, 111, 107, 101, 110, 115, 0, 34, 0, 13, 0, 12, 116, 114, 117, 110, 99, 97, 116, 101, 100, 95, 97, 116, 0, 33, 0, 12, 0, 3, 0, 0, 0, 1, 0, 0, 0, 5, 108, 111, 99, 97, 108, 0, 0, 0, 9, 67, 79, 77, 80, 76, 69, 84, 69, 68, 0, 0, 0, 4, 192, 168, 0, 185, 0, 0, 0, 4, 0, 0, 27, 88, 0, 0, 0, 12, 84, 101, 115, 116, 32, 67, 108, 117, 115, 116, 101, 114, 0, 0, 0, 5, 51, 46, 52, 46, 53, 0, 0, 0, 11, 100, 97, 116, 97, 99, 101, 110, 116, 101, 114, 49, 0, 0, 0, 4, 98, 131, 225, 57, 0, 0, 0, 16, 115, 186, 31, 38, 108, 6, 70, 99, 137, 7, 214, 86, 107, 238, 190, 67, 0, 0, 0, 4, 192, 168, 0, 185, 0, 0, 0, 4, 0, 0, 27, 88, 0, 0, 0, 1, 53, 0, 0, 0, 43, 111, 114, 103, 46, 97, 112, 97, 99, 104, 101, 46, 99, 97, 115, 115, 97, 110, 100, 114, 97, 46, 100, 104, 116, 46, 77, 117, 114, 109, 117, 114, 51, 80, 97, 114, 116, 105, 116, 105, 111, 110, 101, 114, 0, 0, 0, 5, 114, 97, 99, 107, 49, 0, 0, 0, 5, 52, 46, 48, 46, 49, 0, 0, 0, 4, 0, 0, 0, 0, 0, 0, 0, 4, 0, 0, 35, 82, 0, 0, 0, 16, 34, 7, 194, 169, 245, 152, 57, 113, 152, 107, 41, 38, 224, 158, 35, 157, 0, 0, 1, 123, 0, 0, 0, 16, 0, 0, 0, 20, 45, 49, 55, 50, 48, 51, 53, 55, 51, 57, 49, 55, 53, 48, 54, 50, 51, 53, 48, 54, 0, 0, 0, 20, 45, 50, 57, 54, 51, 56, 55, 55, 51, 57, 55, 52, 51, 49, 50, 49, 53, 52, 56, 48, 0, 0, 0, 20, 45, 52, 48, 52, 51, 50, 53, 48, 50, 48, 52, 56, 55, 56, 56, 49, 55, 49, 51, 55, 0, 0, 0, 20, 45, 52, 55, 55, 57, 49, 54, 53, 56, 57, 55, 53, 56, 57, 55, 49, 48, 55, 55, 56, 0, 0, 0, 20, 45, 53, 56, 53, 52, 57, 50, 49, 55, 55, 56, 56, 49, 53, 50, 57, 48, 51, 49, 56, 0, 0, 0, 20, 45, 54, 54, 55, 50, 53, 57, 51, 55, 49, 49, 52, 50, 55, 54, 52, 49, 49, 49, 50, 0, 0, 0, 19, 45, 55, 53, 50, 57, 57, 51, 49, 52, 56, 54, 55, 50, 48, 56, 56, 48, 48, 56, 0, 0, 0, 20, 45, 55, 54, 57, 48, 52, 52, 53, 49, 49, 54, 49, 55, 48, 48, 54, 56, 49, 50, 56, 0, 0, 0, 20, 45, 56, 55, 54, 49, 57, 52, 53, 55, 51, 50, 53, 49, 48, 49, 48, 48, 57, 52, 49, 0, 0, 0, 19, 50, 52, 53, 54, 48, 51, 52, 54, 57, 57, 53, 51, 56, 56, 53, 51, 57, 50, 54, 0, 0, 0, 19, 51, 52, 51, 52, 49, 56, 48, 49, 53, 50, 48, 53, 52, 57, 54, 56, 48, 48, 50, 0, 0, 0, 19, 53, 50, 49, 50, 56, 49, 52, 52, 53, 50, 56, 55, 56, 53, 53, 53, 49, 51, 48, 0, 0, 0, 19, 54, 51, 48, 56, 56, 53, 54, 51, 53, 51, 49, 49, 51, 52, 57, 51, 52, 49, 57, 0, 0, 0, 19, 55, 52, 48, 48, 49, 53, 51, 52, 54, 51, 56, 48, 53, 55, 48, 53, 54, 55, 56, 0, 0, 0, 19, 56, 50, 52, 52, 53, 51, 48, 49, 49, 49, 50, 57, 54, 56, 54, 55, 52, 51, 54, 0, 0, 0, 18, 56, 51, 55, 48, 49, 49, 52, 48, 51, 56, 52, 49, 56, 49, 48, 54, 50, 57, 0, 0, 0, 92, 0, 0, 0, 2, 0, 0, 0, 16, 23, 108, 57, 205, 185, 61, 51, 165, 162, 24, 142, 176, 106, 86, 246, 110, 0, 0, 0, 20, 0, 0, 1, 128, 211, 39, 197, 153, 0, 0, 0, 28, 0, 0, 1, 128, 211, 39, 200, 217, 0, 0, 0, 16, 97, 143, 129, 123, 0, 95, 54, 120, 184, 164, 83, 243, 147, 11, 142, 134, 0, 0, 0, 20, 0, 0, 1, 128, 211, 39, 197, 153, 0, 0, 0, 28, 0, 0, 1, 128, 211, 39, 200, 44},
				}
				responseFrame, err = codec.ConvertFromRawFrame(rawFrame)
				if err != nil {
					return trace.Wrap(err)
				}
			case "select * from system.peers":
				rawFrame := &frame.RawFrame{
					Header: &frame.Header{
						IsResponse: true,
						Version:    recvFrame.Header.Version,
						StreamId:   recvFrame.Header.StreamId,
						OpCode:     primitive.OpCodeResult,
						BodyLength: 153,
					},
					Body: []byte{0, 0, 0, 2, 0, 0, 0, 1, 0, 0, 0, 9, 0, 6, 115, 121, 115, 116, 101, 109, 0, 5, 112, 101, 101, 114, 115, 0, 4, 112, 101, 101, 114, 0, 16, 0, 11, 100, 97, 116, 97, 95, 99, 101, 110, 116, 101, 114, 0, 13, 0, 7, 104, 111, 115, 116, 95, 105, 100, 0, 12, 0, 12, 112, 114, 101, 102, 101, 114, 114, 101, 100, 95, 105, 112, 0, 16, 0, 4, 114, 97, 99, 107, 0, 13, 0, 15, 114, 101, 108, 101, 97, 115, 101, 95, 118, 101, 114, 115, 105, 111, 110, 0, 13, 0, 11, 114, 112, 99, 95, 97, 100, 100, 114, 101, 115, 115, 0, 16, 0, 14, 115, 99, 104, 101, 109, 97, 95, 118, 101, 114, 115, 105, 111, 110, 0, 12, 0, 6, 116, 111, 107, 101, 110, 115, 0, 34, 0, 13, 0, 0, 0, 0},
				}
				responseFrame, err = codec.ConvertFromRawFrame(rawFrame)
				if err != nil {
					return trace.Wrap(err)
				}
			}
		case primitive.OpCodeExecute:
			execute, ok := recvFrame.Body.Message.(*message.Execute)
			if !ok {
				return trace.BadParameter("failed to cast, expected *message.Execute, got %T", recvFrame.Body.Message)
			}
			switch {
			case bytes.Equal(execute.QueryId, []byte{211, 78, 99, 137, 52, 114, 28, 59, 205, 105, 147, 63, 153, 42, 0, 203}):
				rawFrame := &frame.RawFrame{
					Header: &frame.Header{
						IsResponse: true,
						Version:    recvFrame.Header.Version,
						StreamId:   recvFrame.Header.StreamId,
						OpCode:     primitive.OpCodeResult,
						BodyLength: 521,
					},
					Body: []byte{0, 0, 0, 2, 0, 0, 0, 5, 0, 0, 0, 3, 0, 0, 0, 5, 0, 0, 0, 11, 115, 121, 115, 116, 101, 109, 95, 97, 117, 116, 104, 0, 0, 0, 1, 1, 0, 0, 0, 87, 0, 0, 0, 2, 0, 0, 0, 5, 99, 108, 97, 115, 115, 0, 0, 0, 43, 111, 114, 103, 46, 97, 112, 97, 99, 104, 101, 46, 99, 97, 115, 115, 97, 110, 100, 114, 97, 46, 108, 111, 99, 97, 116, 111, 114, 46, 83, 105, 109, 112, 108, 101, 83, 116, 114, 97, 116, 101, 103, 121, 0, 0, 0, 18, 114, 101, 112, 108, 105, 99, 97, 116, 105, 111, 110, 95, 102, 97, 99, 116, 111, 114, 0, 0, 0, 1, 49, 0, 0, 0, 13, 115, 121, 115, 116, 101, 109, 95, 115, 99, 104, 101, 109, 97, 0, 0, 0, 1, 1, 0, 0, 0, 59, 0, 0, 0, 1, 0, 0, 0, 5, 99, 108, 97, 115, 115, 0, 0, 0, 42, 111, 114, 103, 46, 97, 112, 97, 99, 104, 101, 46, 99, 97, 115, 115, 97, 110, 100, 114, 97, 46, 108, 111, 99, 97, 116, 111, 114, 46, 76, 111, 99, 97, 108, 83, 116, 114, 97, 116, 101, 103, 121, 0, 0, 0, 18, 115, 121, 115, 116, 101, 109, 95, 100, 105, 115, 116, 114, 105, 98, 117, 116, 101, 100, 0, 0, 0, 1, 1, 0, 0, 0, 87, 0, 0, 0, 2, 0, 0, 0, 5, 99, 108, 97, 115, 115, 0, 0, 0, 43, 111, 114, 103, 46, 97, 112, 97, 99, 104, 101, 46, 99, 97, 115, 115, 97, 110, 100, 114, 97, 46, 108, 111, 99, 97, 116, 111, 114, 46, 83, 105, 109, 112, 108, 101, 83, 116, 114, 97, 116, 101, 103, 121, 0, 0, 0, 18, 114, 101, 112, 108, 105, 99, 97, 116, 105, 111, 110, 95, 102, 97, 99, 116, 111, 114, 0, 0, 0, 1, 51, 0, 0, 0, 6, 115, 121, 115, 116, 101, 109, 0, 0, 0, 1, 1, 0, 0, 0, 59, 0, 0, 0, 1, 0, 0, 0, 5, 99, 108, 97, 115, 115, 0, 0, 0, 42, 111, 114, 103, 46, 97, 112, 97, 99, 104, 101, 46, 99, 97, 115, 115, 97, 110, 100, 114, 97, 46, 108, 111, 99, 97, 116, 111, 114, 46, 76, 111, 99, 97, 108, 83, 116, 114, 97, 116, 101, 103, 121, 0, 0, 0, 13, 115, 121, 115, 116, 101, 109, 95, 116, 114, 97, 99, 101, 115, 0, 0, 0, 1, 1, 0, 0, 0, 87, 0, 0, 0, 2, 0, 0, 0, 5, 99, 108, 97, 115, 115, 0, 0, 0, 43, 111, 114, 103, 46, 97, 112, 97, 99, 104, 101, 46, 99, 97, 115, 115, 97, 110, 100, 114, 97, 46, 108, 111, 99, 97, 116, 111, 114, 46, 83, 105, 109, 112, 108, 101, 83, 116, 114, 97, 116, 101, 103, 121, 0, 0, 0, 18, 114, 101, 112, 108, 105, 99, 97, 116, 105, 111, 110, 95, 102, 97, 99, 116, 111, 114, 0, 0, 0, 1, 50},
				}
				responseFrame, err = codec.ConvertFromRawFrame(rawFrame)
				if err != nil {
					return trace.Wrap(err)
				}
			case bytes.Equal(execute.QueryId, []byte{48, 60, 203, 12, 80, 82, 198, 204, 96, 125, 128, 97, 211, 209, 122, 35}): // query ID: 48, 60, 203, 12, 80, 82, 198, 204, 96, 125, 128, 97, 211, 209, 122, 35
				rawFrame := &frame.RawFrame{
					Header: &frame.Header{
						IsResponse: true,
						Version:    recvFrame.Header.Version,
						StreamId:   recvFrame.Header.StreamId,
						OpCode:     primitive.OpCodeResult,
						BodyLength: 32,
					},
					Body: []byte{0, 0, 0, 2, 0, 0, 0, 5, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 12, 84, 101, 115, 116, 32, 67, 108, 117, 115, 116, 101, 114},
				}
				responseFrame, err = codec.ConvertFromRawFrame(rawFrame)
				if err != nil {
					return trace.Wrap(err)
				}
			}
		}

		if responseFrame == nil {
			return trace.NotImplemented("request is not implemented")
		}

		if err := conn.Send(responseFrame); err != nil {
			return trace.Wrap(err)
		}
	}
}

// Close closes the server.
func (s *TestServer) Close() error {
	return s.listener.Close()
}

func (s *TestServer) Port() string {
	return s.port
}
