/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
