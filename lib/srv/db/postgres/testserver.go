/*
Copyright 2020 Gravitational, Inc.

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

package postgres

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgproto3/v2"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// TestClientConfig combines parameters for a test Postgres client.
type TestClientConfig struct {
	// AuthClient will be used to retrieve trusted CA.
	AuthClient auth.ClientI
	// AuthServer will be used to generate database access certificate for a user.
	AuthServer *auth.Server
	// Address is the address to connect to (web proxy).
	Address string
	// Cluster is the Teleport cluster name.
	Cluster string
	// Username is the Teleport user name.
	Username string
	// RouteToDatabase contains database routing information.
	RouteToDatabase tlsca.RouteToDatabase
}

// MakeTestClient returns Postgres client connection according to the provided
// parameters.
func MakeTestClient(ctx context.Context, config TestClientConfig) (*pgconn.PgConn, error) {
	// Client will be connecting directly to the multiplexer address.
	pgconnConfig, err := pgconn.ParseConfig(fmt.Sprintf("postgres://%v@%v/?database=%v",
		config.RouteToDatabase.Username, config.Address, config.RouteToDatabase.Database))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key, err := client.NewKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Generate client certificate for the Teleport user.
	cert, err := config.AuthServer.GenerateDatabaseTestCert(
		auth.DatabaseTestCertRequest{
			PublicKey:       key.Pub,
			Cluster:         config.Cluster,
			Username:        config.Username,
			RouteToDatabase: config.RouteToDatabase,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCert, err := tls.X509KeyPair(cert, key.Priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := config.AuthClient.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: config.Cluster,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pool, err := services.CertPool(ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pgconnConfig.TLSConfig = &tls.Config{
		RootCAs:            pool,
		Certificates:       []tls.Certificate{tlsCert},
		InsecureSkipVerify: true,
	}
	pgConn, err := pgconn.ConnectConfig(ctx, pgconnConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pgConn, nil
}

// MakeTestServer returns a new configured and unstarted test Postgres server
// for the provided cluster client.
func MakeTestServer(authClient *auth.Client, name, address string) (*TestServer, error) {
	privateKey, _, err := testauthority.New().GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	csr, err := tlsca.GenerateCertificateRequestPEM(pkix.Name{
		CommonName: "localhost",
	}, privateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := authClient.GenerateDatabaseCert(context.Background(),
		&proto.DatabaseCertRequest{
			CSR:        csr,
			ServerName: "localhost",
			TTL:        proto.Duration(time.Hour),
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := tls.X509KeyPair(resp.Cert, privateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pool := x509.NewCertPool()
	for _, ca := range resp.CACerts {
		if ok := pool.AppendCertsFromPEM(ca); !ok {
			return nil, trace.BadParameter("failed to append certificate pem")
		}
	}
	return NewTestServer(TestServerConfig{
		Name: name,
		TLSConfig: &tls.Config{
			ClientCAs:    pool,
			Certificates: []tls.Certificate{cert},
		},
		Address: address,
	})
}

// TestServerConfig is the test Postgres server configuration.
type TestServerConfig struct {
	// Name is used for identification purposes in the logs.
	Name string
	// TLSConfig is the server TLS config.
	TLSConfig *tls.Config
	// Address is the optional listen address.
	Address string
}

// CheckAndSetDefaults validates the config and sets default values.
func (c *TestServerConfig) CheckAndSetDefaults() error {
	if c.Name == "" {
		return trace.BadParameter("missing Name")
	}
	if c.TLSConfig == nil {
		return trace.BadParameter("missing TLSConfig")
	}
	if c.Address == "" {
		c.Address = "localhost:0"
	}
	return nil
}

// TestServer is a test Postgres server used in functional database
// access tests.
//
// It supports a very small subset of Postgres wire protocol that can:
//   - Accept a TLS connection from Postgres client.
//   - Reply with the same TestQueryResponse to every query the client sends.
//   - Recognize terminate messages from clients closing connections.
type TestServer struct {
	listener  net.Listener
	port      string
	tlsConfig *tls.Config
	log       logrus.FieldLogger
	// queryCount keeps track of the number of queries the server has received.
	queryCount uint32
}

// NewTestServer returns a new instance of a test Postgres server.
func NewTestServer(config TestServerConfig) (*TestServer, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	listener, err := net.Listen("tcp", config.Address)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &TestServer{
		listener:  listener,
		port:      port,
		tlsConfig: config.TLSConfig,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: "postgres",
			"name":          config.Name,
		}),
	}, nil
}

// Serve starts serving client connections.
func (s *TestServer) Serve() error {
	s.log.Debugf("Starting test Postgres server on %v.", s.listener.Addr())
	defer s.log.Debug("Test Postgres server stopped.")
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "use of closed network connection") {
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
	// First message we expect is SSLRequest.
	client, err := s.startTLS(conn)
	if err != nil {
		return trace.Wrap(err)
	}
	// Next should come StartupMessage.
	err = s.handleStartup(client)
	if err != nil {
		return trace.Wrap(err)
	}
	// Enter the loop replying to client messages.
	for {
		message, err := client.Receive()
		if err != nil {
			return trace.Wrap(err)
		}
		s.log.Debugf("Received %#v.", message)
		switch msg := message.(type) {
		case *pgproto3.Query:
			err := s.handleQuery(client, msg)
			if err != nil {
				s.log.WithError(err).Error("Failed to handle query.")
			}
		case *pgproto3.Terminate:
			return nil
		default:
			return trace.BadParameter("unsupported message %#v", msg)
		}
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
	s.log.Debugf("Received %#v.", startupMessage)
	// Reply with 'S' to indicate TLS support.
	if _, err := conn.Write([]byte("S")); err != nil {
		return nil, trace.Wrap(err)
	}
	// Upgrade connection to TLS.
	conn = tls.Server(conn, s.tlsConfig)
	return pgproto3.NewBackend(pgproto3.NewChunkReader(conn), conn), nil
}

func (s *TestServer) handleStartup(client *pgproto3.Backend) error {
	startupMessage, err := client.ReceiveStartupMessage()
	if err != nil {
		return trace.Wrap(err)
	}
	if _, ok := startupMessage.(*pgproto3.StartupMessage); !ok {
		return trace.BadParameter("expected *pgproto3.StartupMessage, got: %#v", startupMessage)
	}
	s.log.Debugf("Received %#v.", startupMessage)
	// Accept auth and send ready for query.
	if err := client.Send(&pgproto3.AuthenticationOk{}); err != nil {
		return trace.Wrap(err)
	}
	if err := client.Send(&pgproto3.ReadyForQuery{}); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *TestServer) handleQuery(client *pgproto3.Backend, query *pgproto3.Query) error {
	atomic.AddUint32(&s.queryCount, 1)
	messages := []pgproto3.BackendMessage{
		&pgproto3.RowDescription{Fields: TestQueryResponse.FieldDescriptions},
		&pgproto3.DataRow{Values: TestQueryResponse.Rows[0]},
		&pgproto3.CommandComplete{CommandTag: TestQueryResponse.CommandTag},
		&pgproto3.ReadyForQuery{},
	}
	for _, message := range messages {
		s.log.Debugf("Sending %#v.", message)
		err := client.Send(message)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Port returns the port server is listening on.
func (s *TestServer) Port() string {
	return s.port
}

// QueryCount returns the number of queries the server has received.
func (s *TestServer) QueryCount() uint32 {
	return s.queryCount
}

// Close closes the server listener.
func (s *TestServer) Close() error {
	return s.listener.Close()
}

// TestQueryResponse is the response test Postgres server sends to every query.
var TestQueryResponse = &pgconn.Result{
	FieldDescriptions: []pgproto3.FieldDescription{{Name: []byte("test-field")}},
	Rows:              [][][]byte{{[]byte("test-value")}},
	CommandTag:        pgconn.CommandTag("select 1"),
}
