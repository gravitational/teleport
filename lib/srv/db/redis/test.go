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

type Client = redis.Client

func MakeTestClient(ctx context.Context, config common.TestClientConfig, opts ...*redis.Options) (*Client, error) {
	tlsConfig, err := common.MakeTestClientTLSConfig(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client := redis.NewClient(&redis.Options{
		Addr:      config.Address,
		TLSConfig: tlsConfig,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, trace.Wrap(err)
	}

	return client, nil
}

type TestServer struct {
	cfg    common.TestServerConfig
	server *miniredis.Miniredis
	log    logrus.FieldLogger
}

// NewTestServer returns a new instance of a test Redis server.
func NewTestServer(t *testing.T, config common.TestServerConfig) (*TestServer, error) {
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

	s := miniredis.NewMiniRedis()
	err = s.StartTLS(tlsConfig)
	require.NoError(t, err)

	t.Cleanup(s.Close)

	server.server = s

	return server, nil
}

func (s *TestServer) Port() string {
	return s.server.Port()
}
