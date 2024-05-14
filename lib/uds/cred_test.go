//go:build darwin || linux

/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package uds

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/peer"

	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/lib/utils"
)

func TestGetCreds(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	l, err := net.Listen("unix", sockPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, l.Close())
	})

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := net.Dial("unix", sockPath)
		assert.NoError(t, err)
		t.Cleanup(func() {
			_ = conn.Close()
		})
	}()

	conn, err := l.Accept()
	require.NoError(t, err)

	// Wait for the connection to be established.
	wg.Wait()

	creds, err := GetCreds(conn)
	require.NoError(t, err)

	// Compare to the current process.
	assert.Equal(t, os.Getpid(), creds.PID)
	assert.Equal(t, os.Getuid(), creds.UID)
	assert.Equal(t, os.Getgid(), creds.GID)
}

type service struct {
	*machineidv1.UnimplementedBotServiceServer
	lastCalledCreds *Creds
}

func (s *service) GetBot(
	ctx context.Context, _ *machineidv1.GetBotRequest,
) (*machineidv1.Bot, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, trace.BadParameter("peer not found in context")
	}
	authInfo, ok := p.AuthInfo.(AuthInfo)
	if !ok {
		return nil, trace.BadParameter("auth info not found in peer")
	}

	s.lastCalledCreds = authInfo.Creds
	return &machineidv1.Bot{}, nil
}

func TestTransportCredentials(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	l, err := net.Listen("unix", sockPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := l.Close()
		if err != nil && !utils.IsUseOfClosedNetworkError(err) {
			assert.NoError(t, err)
		}
	})

	svc := &service{}
	grpcSrv := grpc.NewServer(grpc.Creds(NewTransportCredentials(insecure.NewCredentials())))
	machineidv1.RegisterBotServiceServer(grpcSrv, svc)
	t.Cleanup(func() {
		grpcSrv.Stop()
	})

	go func() {
		assert.NoError(t, grpcSrv.Serve(l))
	}()

	grpcConn, err := grpc.Dial(
		"unix:///"+sockPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, grpcConn.Close())
	})

	client := machineidv1.NewBotServiceClient(grpcConn)
	_, err = client.GetBot(context.Background(), &machineidv1.GetBotRequest{})
	require.NoError(t, err)

	assert.NotNil(t, svc.lastCalledCreds)
	assert.Equal(t, os.Getpid(), svc.lastCalledCreds.PID)
	assert.Equal(t, os.Getuid(), svc.lastCalledCreds.UID)
	assert.Equal(t, os.Getgid(), svc.lastCalledCreds.GID)
}
