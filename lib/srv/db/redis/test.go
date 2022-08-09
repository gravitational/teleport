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

package redis

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// Client alias for easier use.
type Client = redis.Client

// ClientOptionsParams is a struct for client configuration options.
type ClientOptionsParams struct {
	skipPing bool
}

// ClientOptions allows setting test client options.
type ClientOptions func(*ClientOptionsParams)

// SkipPing skips Redis server ping right after the connection is established.
func SkipPing(skip bool) ClientOptions {
	return func(ts *ClientOptionsParams) {
		ts.skipPing = skip
	}
}

// MakeTestClient returns Redis client connection according to the provided
// parameters.
func MakeTestClient(ctx context.Context, config common.TestClientConfig, opts ...ClientOptions) (*Client, error) {
	tlsConfig, err := common.MakeTestClientTLSConfig(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientOptions := &ClientOptionsParams{}

	for _, opt := range opts {
		opt(clientOptions)
	}

	client := redis.NewClient(&redis.Options{
		Addr:      config.Address,
		TLSConfig: tlsConfig,
	})

	if !clientOptions.skipPing {
		if err := client.Ping(ctx).Err(); err != nil {
			_ = client.Close()
			return nil, trace.Wrap(err)
		}
	}

	return client, nil
}

// TestServer is a test Redis server used in functional database
// access tests. Internally is uses github.com/alicebob/miniredis to
// simulate Redis server behavior.
type TestServer struct {
	cfg    common.TestServerConfig
	server *miniredis.Miniredis
	log    logrus.FieldLogger

	// password is the default user password.
	// If set, AUTH must be sent first to get access to the server.
	password string
}

// TestServerOption allows setting test server options.
type TestServerOption func(*TestServer)

// TestServerPassword sets the test Redis server password for default user.
func TestServerPassword(password string) TestServerOption {
	return func(ts *TestServer) {
		ts.password = password
	}
}

// NewTestServer returns a new instance of a test Redis server.
func NewTestServer(t *testing.T, config common.TestServerConfig, opts ...TestServerOption) (*TestServer, error) {
	tlsConfig, err := common.MakeTestServerTLSConfig(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := logrus.WithFields(logrus.Fields{
		trace.Component: defaults.ProtocolRedis,
		"name":          config.Name,
	})
	server := &TestServer{
		cfg: config,
		log: log,
	}

	for _, opt := range opts {
		opt(server)
	}

	// Create a new test Redis instance.
	s := miniredis.NewMiniRedis()
	if server.password != "" {
		s.RequireAuth(server.password)
	}

	err = s.StartTLS(tlsConfig)
	require.NoError(t, err)

	t.Cleanup(s.Close)

	server.server = s

	return server, nil
}

// Port returns a port that test Redis instance is listening on.
func (s *TestServer) Port() string {
	return s.server.Port()
}
