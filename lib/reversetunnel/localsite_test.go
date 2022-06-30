// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package reversetunnel

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestLocalSiteOverlap(t *testing.T) {
	t.Parallel()

	// to stop (*localSite).periodicFunctions()
	ctx, ctxCancel := context.WithCancel(context.Background())
	ctxCancel()

	srv := &server{
		ctx: ctx,
		newAccessPoint: func(clt auth.ClientI, _ []string) (auth.RemoteProxyAccessPoint, error) {
			return clt, nil
		},
	}

	site, err := newlocalSite(srv, "clustername", nil /* authServers */, &mockLocalSiteClient{}, nil /* peerClient */)
	require.NoError(t, err)

	nodeID := uuid.NewString()
	connType := types.NodeTunnel
	dreq := &sshutils.DialReq{
		ServerID: nodeID,
		ConnType: connType,
	}

	conn1, err := site.addConn(nodeID, connType, mockRemoteConnConn{}, nil)
	require.NoError(t, err)

	conn2, err := site.addConn(nodeID, connType, mockRemoteConnConn{}, nil)
	require.NoError(t, err)

	c, err := site.getRemoteConn(dreq)
	require.True(t, trace.IsNotFound(err))
	require.Nil(t, c)

	conn1.setLastHeartbeat(time.Now())
	c, err = site.getRemoteConn(dreq)
	require.NoError(t, err)
	require.Equal(t, conn1, c)

	conn2.setLastHeartbeat(time.Now())
	c, err = site.getRemoteConn(dreq)
	require.NoError(t, err)
	require.Equal(t, conn2, c)

	conn2.markInvalid(nil)
	c, err = site.getRemoteConn(dreq)
	require.NoError(t, err)
	require.Equal(t, conn1, c)

	conn1.markInvalid(nil)
	c, err = site.getRemoteConn(dreq)
	require.True(t, trace.IsNotFound(err))
	require.Nil(t, c)
}

type mockLocalSiteClient struct {
	auth.Client
}

// called by (*localSite).sshTunnelStats() as part of (*localSite).periodicFunctions()
func (mockLocalSiteClient) GetNodes(_ context.Context, _ string) ([]types.Server, error) {
	return nil, nil
}

type mockRemoteConnConn struct {
	net.Conn
}

// called for logging by (*remoteConn).markInvalid()
func (mockRemoteConnConn) RemoteAddr() net.Addr { return nil }
