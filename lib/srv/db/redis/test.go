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

package redis

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gravitational/trace"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	rediscontainer "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

// Client alias for easier use.
type Client = redis.Client

// ClientOptionsParams is a struct for client configuration options.
type ClientOptionsParams struct {
	skipPing bool
	timeout  time.Duration
	password string
}

// ClientOptions allows setting test client options.
type ClientOptions func(*ClientOptionsParams)

// SkipPing skips Redis server ping right after the connection is established.
func SkipPing(skip bool) ClientOptions {
	return func(ts *ClientOptionsParams) {
		ts.skipPing = skip
	}
}

// WithTimeout overrides test client's default timeout.
func WithTimeout(timeout time.Duration) ClientOptions {
	return func(ts *ClientOptionsParams) {
		ts.timeout = timeout
	}
}

// MakeTestClient returns Redis client connection according to the provided
// parameters.
func MakeTestClient(ctx context.Context, config common.TestClientConfig, opts ...ClientOptions) (*Client, error) {
	tlsConfig, err := common.MakeTestClientTLSConfig(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientOptions := &ClientOptionsParams{
		// set default timeout to 10 seconds for test clients.
		timeout: 10 * time.Second,
	}

	for _, opt := range opts {
		opt(clientOptions)
	}

	client := redis.NewClient(&redis.Options{
		Addr:             config.Address,
		TLSConfig:        tlsConfig,
		DialTimeout:      clientOptions.timeout,
		ReadTimeout:      clientOptions.timeout,
		WriteTimeout:     clientOptions.timeout,
		Protocol:         protocolV2,
		DisableIndentity: true,
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
	cfg       common.TestServerConfig
	server    *miniredis.Miniredis
	container *rediscontainer.RedisContainer
	log       logrus.FieldLogger

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
func NewTestServer(t testing.TB, config common.TestServerConfig, opts ...TestServerOption) (*TestServer, error) {
	log := logrus.WithFields(logrus.Fields{
		teleport.ComponentKey: defaults.ProtocolRedis,
		"name":                config.Name,
	})
	server := &TestServer{
		cfg: config,
		log: log,
	}

	for _, opt := range opts {
		opt(server)
	}

	if os.Getenv("TELEPORT_TESTCONTAINERS") != "" {
		if err := server.initTestContainer(t); err != nil {
			return nil, trace.Wrap(err)
		}
		return server, nil
	}

	tlsConfig, err := common.MakeTestServerTLSConfig(config)
	if err != nil {
		return nil, trace.Wrap(err)
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
func (s *TestServer) Uri(t testing.TB, ctx context.Context) string {
	if s.server != nil {
		return fmt.Sprintf("rediss://%s", net.JoinHostPort("localhost", s.server.Port()))
	}

	mappedPort, err := s.container.MappedPort(ctx, "6379")
	require.NoError(t, err)

	hostIP, err := s.container.Host(ctx)
	require.NoError(t, err)

	return fmt.Sprintf("rediss://%s:%s", hostIP, mappedPort.Port())
}

type redisPaths struct {
	common.TLSConfigPaths
}

func (p redisPaths) redisConfPath() string {
	return filepath.Join(p.DirPath, "redis.conf")
}

func (p redisPaths) mountTo(target string) (redisPaths, testcontainers.ContainerMount) {
	source := p.DirPath
	p.DirPath = target
	return p, testcontainers.BindMount(source, testcontainers.ContainerMountTarget(target))
}

func (s *TestServer) writeRedisConf(localPaths, containerPaths redisPaths) error {
	content := fmt.Sprintf(`
tls-port 6379
port 0
tls-ca-cert-file %s
tls-cert-file %s
tls-key-file %s
tls-protocols "TLSv1.2 TLSv1.3"
`,
		containerPaths.CAPath(),
		containerPaths.CertPath(),
		containerPaths.KeyPath(),
	)
	if s.password != "" {
		content += fmt.Sprintf(`requirepass "%s"
`, s.password)
	}
	return trace.ConvertSystemError(os.WriteFile(localPaths.redisConfPath(), []byte(content), 0o644))
}

func (s *TestServer) initTestContainer(t testing.TB) error {
	tlsPaths, err := s.cfg.SaveCertsTo(t.TempDir())
	if err != nil {
		return trace.Wrap(err)
	}

	localPaths := redisPaths{*tlsPaths}
	containerPaths, containerMount := localPaths.mountTo("/opt/teleport")

	if err := s.writeRedisConf(localPaths, containerPaths); err != nil {
		return trace.Wrap(err)
	}

	container, err := rediscontainer.RunContainer(
		context.Background(),
		testcontainers.CustomizeRequestOption(func(req *testcontainers.GenericContainerRequest) error {
			req.Mounts = append(req.Mounts, containerMount)
			return nil
		}),
		rediscontainer.WithConfigFile(localPaths.redisConfPath()),
		// Make it based on testing verbose
		rediscontainer.WithLogLevel(rediscontainer.LogLevelDebug),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	s.container = container

	t.Cleanup(func() {
		logs, err := container.Logs(context.Background())
		if err == nil {
			logsRead, err := io.ReadAll(logs)
			if err == nil {
				t.Log("=== container log: ", string(logsRead))
			}
		}

		if err := container.Terminate(context.Background()); err != nil {
			t.Log(err)
		}
	})

	return nil
}
