/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
package grpcconn_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/gravitational/teleport/lib/tbot/grpcconn"
)

func TestUninitialized(t *testing.T) {
	conn := new(grpcconn.ClientConn)
	client := healthpb.NewHealthClient(conn)

	_, err := client.Check(context.Background(), &healthpb.HealthCheckRequest{})
	require.ErrorContains(t, err, "connection is uninitialized")
	require.Equal(t, codes.Unavailable.String(), status.Code(err).String())

	_, err = client.Watch(context.Background(), &healthpb.HealthCheckRequest{})
	require.ErrorContains(t, err, "connection is uninitialized")
	require.Equal(t, codes.Unavailable.String(), status.Code(err).String())
}

func TestWithError(t *testing.T) {
	conn := grpcconn.WithError(errors.New("KABOOM"))
	client := healthpb.NewHealthClient(conn)

	_, err := client.Check(context.Background(), &healthpb.HealthCheckRequest{})
	require.ErrorContains(t, err, "KABOOM")

	_, err = client.Watch(context.Background(), &healthpb.HealthCheckRequest{})
	require.ErrorContains(t, err, "KABOOM")
}

func TestWithConnection(t *testing.T) {
	conn := grpcconn.WithConnection(dialRealConnection(t))
	client := healthpb.NewHealthClient(conn)

	_, err := client.Check(context.Background(), &healthpb.HealthCheckRequest{})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := client.Watch(ctx, &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, stream.CloseSend()) })
}

func TestEndToEnd(t *testing.T) {
	conn := new(grpcconn.ClientConn)
	client := healthpb.NewHealthClient(conn)

	conn.SetError(errors.New("KABOOM"))

	_, err := client.Check(context.Background(), &healthpb.HealthCheckRequest{})
	require.ErrorContains(t, err, "KABOOM")

	realConn := dialRealConnection(t)
	conn.SetConnection(realConn)

	_, err = client.Check(context.Background(), &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
}

func dialRealConnection(t *testing.T) *grpc.ClientConn {
	t.Helper()

	lis := bufconn.Listen(100)
	t.Cleanup(func() {
		assert.NoError(t, lis.Close())
	})

	srv := grpc.NewServer()
	healthpb.RegisterHealthServer(srv, testHealthService{})

	t.Cleanup(srv.Stop)
	go func() { require.NoError(t, srv.Serve(lis)) }()

	realConn, err := grpc.NewClient(
		"127.0.0.0", // Avoid DNS resolution.
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		realConn.Close()
	})

	return realConn
}

type testHealthService struct {
	healthpb.UnimplementedHealthServer
}

func (testHealthService) Check(context.Context, *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{
		Status: healthpb.HealthCheckResponse_SERVING,
	}, nil
}

func (testHealthService) Watch(*healthpb.HealthCheckRequest, grpc.ServerStreamingServer[healthpb.HealthCheckResponse]) error {
	return nil
}
