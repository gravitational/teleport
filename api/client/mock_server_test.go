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
	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/testhelpers/mtls"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
)

// mockServer mocks an Auth Server.
type mockServer struct {
	addr       string
	grpc       *grpc.Server
	mtlsConfig *mtls.Config
}

type mockServices struct {
	auth                proto.AuthServiceServer
	recordingEncryption recordingencryptionv1.RecordingEncryptionServiceServer
}

func newMockServer(t *testing.T, addr string, services mockServices) *mockServer {
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

	if services.auth != nil {
		proto.RegisterAuthServiceServer(m.grpc, services.auth)
	}

	if services.recordingEncryption != nil {
		recordingencryptionv1.RegisterRecordingEncryptionServiceServer(m.grpc, services.recordingEncryption)
	}
	return m
}

// startMockServer starts a new mock server. Parallel tests cannot use the same addr.
func startMockServer(t *testing.T, services mockServices) *mockServer {
	l, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err)
	srv := newMockServer(t, l.Addr().String(), services)
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
