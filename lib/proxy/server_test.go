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

package proxy

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/api/types"

	"github.com/stretchr/testify/require"
)

// TestServerTLS ensures that only trusted certificates with the proxy role
// are accepted by the server.
func TestServerTLS(t *testing.T) {
	ca1 := newSelfSignedCA(t)
	ca2 := newSelfSignedCA(t)

	// trusted certificates with proxy roles.
	client1, _ := setupClient(t, ca1, ca1, types.RoleProxy)
	server1, _ := setupServer(t, ca1, ca1, types.RoleProxy)
	stream, _, err := client1.dial(context.TODO(), server1.config.Listener.Addr().String())
	require.NoError(t, err)
	require.NotNil(t, stream)
	require.NoError(t, sendMsg(stream))
	stream.CloseSend()

	// trusted certificates with incorrect server role.
	client2, _ := setupClient(t, ca1, ca1, types.RoleAdmin)
	server2, _ := setupServer(t, ca1, ca1, types.RoleProxy)
	_, _, err = client2.dial(context.TODO(), server2.config.Listener.Addr().String())
	require.Error(t, err)

	// certificates with correct role from different CAs
	client3, _ := setupClient(t, ca1, ca2, types.RoleProxy)
	server3, _ := setupServer(t, ca2, ca1, types.RoleProxy)
	stream, _, err = client3.dial(context.TODO(), server3.config.Listener.Addr().String())
	require.NoError(t, err)
	require.NotNil(t, stream)
	require.NoError(t, sendMsg(stream))
	stream.CloseSend()
}
