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

package peer

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

// TestServerTLS ensures that only trusted certificates with the proxy role
// are accepted by the server.
func TestServerTLS(t *testing.T) {
	ca1 := newSelfSignedCA(t)
	ca2 := newSelfSignedCA(t)

	// trusted certificates with proxy roles.
	client1 := setupClient(t, ca1, ca1, types.RoleProxy)
	_, serverDef1 := setupServer(t, "s1", ca1, ca1, types.RoleProxy)
	err := client1.updateConnections([]types.Server{serverDef1})
	require.NoError(t, err)
	stream, _, err := client1.dial([]string{"s1"}, &proto.DialRequest{})
	require.NoError(t, err)
	require.NotNil(t, stream)
	stream.Close()

	// trusted certificates with incorrect server role.
	client2 := setupClient(t, ca1, ca1, types.RoleNode)
	_, serverDef2 := setupServer(t, "s2", ca1, ca1, types.RoleProxy)
	err = client2.updateConnections([]types.Server{serverDef2})
	require.NoError(t, err) // connection succeeds but is in transient failure state
	_, _, err = client2.dial([]string{"s2"}, &proto.DialRequest{})
	require.Error(t, err)

	// certificates with correct role from different CAs
	client3 := setupClient(t, ca1, ca2, types.RoleProxy)
	_, serverDef3 := setupServer(t, "s3", ca2, ca1, types.RoleProxy)
	err = client3.updateConnections([]types.Server{serverDef3})
	require.NoError(t, err)
	stream, _, err = client3.dial([]string{"s3"}, &proto.DialRequest{})
	require.NoError(t, err)
	require.NotNil(t, stream)
	stream.Close()
}
