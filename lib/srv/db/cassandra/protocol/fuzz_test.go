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
	"testing"

	"github.com/datastax/go-cassandra-native-protocol/compression/lz4"
	"github.com/datastax/go-cassandra-native-protocol/segment"
	"github.com/stretchr/testify/require"
)

func encodeSegment(f *testing.F, compressor segment.PayloadCompressor, seg *segment.Segment) []byte {
	writer := &bytes.Buffer{}
	err := segment.NewCodecWithCompression(compressor).EncodeSegment(seg, writer)
	require.NoError(f, err)
	return writer.Bytes()
}

func FuzzReadPacket(f *testing.F) {
	// payload 8 byte, self-contained, uncompressed
	f.Add(encodeSegment(f, nil, &segment.Segment{
		Header:  &segment.Header{IsSelfContained: true},
		Payload: &segment.Payload{UncompressedData: []byte{1, 2, 3, 4, 5, 6, 7, 8}},
	}))
	// payload 8 byte, multi-part, uncompressed
	f.Add(encodeSegment(f, nil, &segment.Segment{
		Header:  &segment.Header{IsSelfContained: false},
		Payload: &segment.Payload{UncompressedData: []byte{1, 2, 3, 4, 5, 6, 7, 8}},
	}))
	// payload 100 bytes, self-contained, compressed
	f.Add(encodeSegment(f, lz4.Compressor{}, &segment.Segment{
		Header:  &segment.Header{IsSelfContained: true},
		Payload: &segment.Payload{UncompressedData: make([]byte, 100)},
	}))

	f.Fuzz(func(t *testing.T, body []byte) {
		require.NotPanics(t, func() {
			rawConn := &mockConn{
				buff: *bytes.NewBuffer(body),
			}
			conn := NewConn(rawConn)
			_, _ = conn.ReadPacket()
		})
	})
}
