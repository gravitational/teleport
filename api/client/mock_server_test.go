/*
Copyright 2023 Gravitational, Inc.

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

package client

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/testhelpers/mtls"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
)

// mockServer mocks an Auth Server.
type mockServer struct {
	addr       string
	grpc       *grpc.Server
	mtlsConfig *mtls.Config
}

func newMockServer(t *testing.T, addr string, service proto.AuthServiceServer) *mockServer {
	t.Helper()
	m := &mockServer{
		addr:       addr,
		mtlsConfig: mtls.NewConfig(t),
	}

	m.grpc = grpc.NewServer(
		grpc.Creds(credentials.NewTLS(m.mtlsConfig.ServerTLS)),
		grpc.UnaryInterceptor(interceptors.GRPCServerUnaryErrorInterceptor),
		grpc.StreamInterceptor(interceptors.GRPCServerStreamErrorInterceptor),
	)

	proto.RegisterAuthServiceServer(m.grpc, service)
	return m
}

// startMockServer starts a new mock server. Parallel tests cannot use the same addr.
func startMockServer(t *testing.T, service proto.AuthServiceServer) *mockServer {
	l, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err)
	srv := newMockServer(t, l.Addr().String(), service)
	srv.serve(t, l)
	return srv
}

func (m *mockServer) serve(t *testing.T, l net.Listener) {
	errCh := make(chan error, 1)
	go func() {
		errCh <- m.grpc.Serve(l)
	}()

	t.Cleanup(func() {
		m.grpc.Stop()
		require.NoError(t, <-errCh, "mockServer gRPC server exited with unexpected error")
	})
}

func (m *mockServer) clientCfg() Config {
	return Config{
		// Reduce dial timeout for tests.
		DialTimeout: time.Second,
		Addrs:       []string{m.addr},
		Credentials: []Credentials{
			LoadTLS(m.mtlsConfig.ClientTLS),
		},
	}
}
