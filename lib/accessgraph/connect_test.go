/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package accessgraph

import (
	"context"
	"crypto/tls"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"

	"github.com/gravitational/teleport/lib/fixtures"
)

func TestGrpcCredentials(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		config      AccessGraphClientConfig
		getCreds    ClientCredentialsGetter
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config with CA",
			config: AccessGraphClientConfig{
				Addr: "localhost:50051",
				CA:   []byte(fixtures.TLSCACertPEM),
			},
			getCreds: func() (*tls.Certificate, error) {
				cert, err := tls.X509KeyPair([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
				return &cert, err
			},
			expectError: false,
		},
		{
			name: "valid config without CA",
			config: AccessGraphClientConfig{
				Addr: "localhost:50051",
			},
			getCreds: func() (*tls.Certificate, error) {
				cert, err := tls.X509KeyPair([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
				return &cert, err
			},
			expectError: false,
		},
		{
			name: "insecure config",
			config: AccessGraphClientConfig{
				Addr:     "localhost:50051",
				Insecure: true,
			},
			getCreds: func() (*tls.Certificate, error) {
				cert, err := tls.X509KeyPair([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
				return &cert, err
			},
			expectError: false,
		},
		{
			name: "missing credential getter",
			config: AccessGraphClientConfig{
				Addr: "localhost:50051",
				CA:   []byte(fixtures.TLSCACertPEM),
			},
			getCreds:    nil,
			expectError: true,
			errorMsg:    "missing credential getter",
		},
		{
			name: "invalid CA certificate",
			config: AccessGraphClientConfig{
				Addr: "localhost:50051",
				CA:   []byte("invalid ca cert"),
			},
			getCreds: func() (*tls.Certificate, error) {
				cert, err := tls.X509KeyPair([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
				return &cert, err
			},
			expectError: true,
			errorMsg:    "failed to append CA certificate to pool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt, err := grpcCredentials(tt.config, tt.getCreds)
			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					require.ErrorContains(t, err, tt.errorMsg)
				}
				return
			}
			require.NoError(t, err)
			require.NotNil(t, opt)
		})
	}
}

func TestNewAccessGraphClient(t *testing.T) {
	ctx := context.Background()

	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)

	serverCert, err := tls.X509KeyPair(
		[]byte(fixtures.TLSCACertPEM),
		[]byte(fixtures.TLSCAKeyPEM),
	)
	require.NoError(t, err)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAnyClientCert,
	}

	grpcServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsConfig)))
	t.Cleanup(func() {
		grpcServer.Stop()
	})

	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			t.Logf("Server exited with error: %v", err)
		}
	}()

	t.Run("missing credential getter", func(t *testing.T) {
		config := AccessGraphClientConfig{
			Addr: "localhost:50051",
			CA:   []byte(fixtures.TLSCACertPEM),
		}
		conn, err := NewAccessGraphClient(ctx, config)
		require.Error(t, err)
		require.ErrorContains(t, err, "missing credential getter")
		require.Nil(t, conn)
	})

	t.Run("successful connection with health check", func(t *testing.T) {
		config := AccessGraphClientConfig{
			Addr:     "bufconn",
			Insecure: true,
			ClientCredentials: func() (*tls.Certificate, error) {
				cert, err := tls.X509KeyPair([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
				return &cert, err
			},
		}

		conn, err := NewAccessGraphClient(
			ctx,
			config,
			grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return lis.DialContext(ctx)
			}),
		)
		require.NoError(t, err)
		require.NotNil(t, conn)
		t.Cleanup(func() {
			require.NoError(t, conn.Close())
		})

		healthClient := healthpb.NewHealthClient(conn)
		resp, err := healthClient.Check(ctx, &healthpb.HealthCheckRequest{})
		require.NoError(t, err)
		require.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.Status)
	})

	t.Run("successful connection with additional dial options", func(t *testing.T) {
		config := AccessGraphClientConfig{
			Addr:     "bufconn",
			Insecure: true,
			ClientCredentials: func() (*tls.Certificate, error) {
				cert, err := tls.X509KeyPair([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
				return &cert, err
			},
		}

		conn, err := NewAccessGraphClient(
			ctx,
			config,
			grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return lis.DialContext(ctx)
			}),
		)
		require.NoError(t, err)
		require.NotNil(t, conn)
		t.Cleanup(func() {
			require.NoError(t, conn.Close())
		})

		healthClient := healthpb.NewHealthClient(conn)
		resp, err := healthClient.Check(ctx, &healthpb.HealthCheckRequest{})
		require.NoError(t, err)
		require.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.Status)
	})
}
