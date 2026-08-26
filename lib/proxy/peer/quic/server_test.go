// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package quic

import (
	"encoding/binary"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	quicpeeringv1a "github.com/gravitational/teleport/gen/proto/go/teleport/quicpeering/v1alpha"
)

func TestDialResponseOKEncoding(t *testing.T) {
	resp := &quicpeeringv1a.DialResponse{}
	require.NoError(t, status.FromProto(resp.GetStatus()).Err())

	b, err := proto.Marshal(resp)
	require.NoError(t, err)
	require.Empty(t, b)

	b = append(binary.LittleEndian.AppendUint32(nil, 0), b...)
	require.Equal(t, dialResponseOK, string(b))

	sizedResp, err := marshalSized(&quicpeeringv1a.DialResponse{})
	require.NoError(t, err)
	require.Equal(t, dialResponseOK, string(sizedResp))
}

func TestSessionTicketRefresher(t *testing.T) {
	synctest.Test(t, testSessionTicketRefresher)
}
func testSessionTicketRefresher(t *testing.T) {
	str := new(sessionTicketRefresher)

	t1 := time.Now()
	k1 := str.getSessionTicketKeys()
	require.Len(t, k1, 1)
	requireSameSlice(t, k1, str.getSessionTicketKeys())

	time.Sleep(time.Hour)

	// the first key is still valid, no keys are added

	requireSameSlice(t, k1, str.getSessionTicketKeys())

	requireSameSlice(t, k1, str.state.Load().tickets)
	require.True(t, t1.Equal(str.state.Load().created[0]))

	time.Sleep(24 * time.Hour)

	// the first key is valid but old, a new fresh key is generated

	t2 := time.Now()
	k2 := str.getSessionTicketKeys()

	require.Len(t, k2, 2)
	requireSameSlice(t, k2, str.state.Load().tickets)
	require.True(t, t2.Equal(str.state.Load().created[0]))
	require.True(t, t1.Equal(str.state.Load().created[1]))

	time.Sleep(6 * 24 * time.Hour)

	// the first key should be rotated out

	t3 := time.Now()
	k3 := str.getSessionTicketKeys()

	require.Len(t, k3, 2)
	requireSameSlice(t, k3, str.state.Load().tickets)
	require.True(t, t3.Equal(str.state.Load().created[0]))
	require.True(t, t2.Equal(str.state.Load().created[1]))
}

func requireSameSlice[S ~[]T, T any](t *testing.T, expected, actual S) {
	require.Len(t, actual, len(expected))
	require.Same(t, &expected[0], &actual[0])
}
