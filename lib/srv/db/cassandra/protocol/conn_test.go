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

package protocol

import (
	"bytes"
	"net"
	"testing"

	"github.com/datastax/go-cassandra-native-protocol/client"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/datastax/go-cassandra-native-protocol/segment"
	"github.com/stretchr/testify/require"
)

// TestV5ReadSelfContained checks that self-contained frames are read correctly
// in v5 cassandra protocol version.
func TestV5ReadSelfContained(t *testing.T) {
	rawConn := &mockConn{}
	conn := NewConn(rawConn)
	conn.modernLayoutRead = true
	conn.modernLayoutWrite = true
	conn.frameCodec = frame.NewRawCodecWithCompression(client.NewBodyCompressor(primitive.CompressionNone))
	conn.segmentCode = segment.NewCodecWithCompression(client.NewPayloadCompressor(primitive.CompressionNone))

	var buff bytes.Buffer
	fr1 := frame.NewFrame(primitive.ProtocolVersion5, 1, &message.Query{
		Query: "select * from query1;",
	})
	fr2 := frame.NewFrame(primitive.ProtocolVersion5, 1, &message.Query{
		Query: "select * from query2;",
	})

	err := conn.writeFrame(fr1, &buff)
	require.NoError(t, err)

	err = conn.writeFrame(fr2, &buff)
	require.NoError(t, err)

	// Create a self-contained segment that contains multiple frames.
	seg := &segment.Segment{
		Header:  &segment.Header{IsSelfContained: true},
		Payload: &segment.Payload{UncompressedData: buff.Bytes()},
	}
	err = conn.segmentCode.EncodeSegment(seg, rawConn)
	require.NoError(t, err)

	firstPacket, err := conn.ReadPacket()
	require.NoError(t, err)
	gotFrame1, ok := firstPacket.frame.Body.Message.(*message.Query)
	require.True(t, ok)
	require.Equal(t, "select * from query1;", gotFrame1.Query)

	secondPacket, err := conn.ReadPacket()
	require.NoError(t, err)

	gotFrame2, ok := secondPacket.frame.Body.Message.(*message.Query)
	require.True(t, ok)
	require.Equal(t, "select * from query2;", gotFrame2.Query)
}

// mockConn implements net.Conn interface and is used to mock a connection to the server.
type mockConn struct {
	net.Conn
	buff bytes.Buffer
}

func (m *mockConn) Write(p []byte) (n int, err error) {
	return m.buff.Write(p)
}
func (m *mockConn) Read(p []byte) (n int, err error) {
	return m.buff.Read(p)
}
