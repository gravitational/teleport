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
