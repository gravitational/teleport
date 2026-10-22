/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package events

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"io"
	"math"
	"math/rand/v2"
	"testing"

	"filippo.io/age"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/utils"
)

func TestReadOversizedMessage(t *testing.T) {
	const maxMessageSize = uint32(len(ProtoReader{}.messageBytes))

	makePart := func(t *testing.T, messageSize uint32) []byte {
		t.Helper()

		var gz bytes.Buffer
		w := gzip.NewWriter(&gz)
		var recordHeader [Int32Size]byte
		binary.BigEndian.PutUint32(recordHeader[:], messageSize)
		_, err := w.Write(recordHeader[:])
		require.NoError(t, err)
		require.NoError(t, w.Close())

		header := PartHeader{
			ProtoVersion: ProtoStreamV1,
			PartSize:     uint64(gz.Len()),
		}
		return append(header.Bytes(), gz.Bytes()...)
	}

	tests := []struct {
		name        string
		messageSize uint32
	}{
		{name: "just over the limit", messageSize: maxMessageSize + 1},
		{name: "max uint32", messageSize: math.MaxUint32},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := NewProtoReader(bytes.NewReader(makePart(t, tc.messageSize)), nil /* decrypter */)
			defer reader.Close()

			_, err := reader.Read(t.Context())
			require.ErrorIs(t, err, trace.BadParameter("unexpected message size %d", tc.messageSize))
		})
	}
}

// TestSliceCloseReserve drives a single slice to the point where shouldUpload
// cuts it and checks that closing never pushes it past MinUploadBytes, with
// and without encryption. The event sizes and the threshold vary the
// alignment of the gzip and age buffers at the cut.
func TestSliceCloseReserve(t *testing.T) {
	const (
		// flushWindow is the most input the gzip writer, or output the age
		// writer, holds back before a close.
		flushWindow = 64 * 1024
		sweepStep   = 8 * 1024
		// protoOverhead keeps a SessionPrint event with this payload under
		// the event size cap.
		protoOverhead = 512
	)

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	random := rand.NewChaCha8([32]byte{})
	for _, encrypted := range []bool{false, true} {
		// Without encryption the slice can grow past the threshold by the gzip
		// window and the crossing event, with encryption by one age chunk more.
		minOvershoot := int64(96 * 1024)
		if encrypted {
			minOvershoot = 160 * 1024
		}
		var maxOvershoot int64
		// A compressible event shifts how much input the gzip writer holds
		// back at the cut, an incompressible one shifts how much output the
		// age writer holds back, and the threshold offset shifts where the
		// cut falls within an age chunk.
		for _, offset := range []int64{0, sweepStep} {
			proto := &ProtoStream{
				cfg: ProtoStreamConfig{
					MinUploadBytes: MinUploadPartSizeBytes + offset,
					SlicePool:      utils.NewSliceSyncPool(constants.MaxProtoMessageSizeBytes + ProtoStreamV1RecordHeaderSize),
				},
				completeCtx: context.Background(),
			}
			for compressible := 1; compressible < flushWindow; compressible += sweepStep {
				for incompressible := 1; incompressible < flushWindow; incompressible += sweepStep {
					buffer := &bytes.Buffer{}
					var writer io.WriteCloser = &bufferCloser{Buffer: buffer}
					headerSize := ProtoStreamV1PartHeaderSize
					if encrypted {
						writer = NewTestAgeWriter(writer, identity.Recipient())
						headerSize = ProtoStreamV2PartHeaderSize
					}
					buffer.Write(make([]byte, headerSize))
					s := &slice{proto: proto, buffer: buffer, writer: newGzipWriter(writer), encrypted: encrypted}

					require.NoError(t, s.recordEvent(sessionPrintEvent(t, make([]byte, compressible))))
					data := make([]byte, incompressible)
					_, err := random.Read(data)
					require.NoError(t, err)
					require.NoError(t, s.recordEvent(sessionPrintEvent(t, data)))
					for !s.shouldUpload() {
						filler := make([]byte, constants.MaxProtoMessageSizeBytes-protoOverhead)
						_, err := random.Read(filler)
						require.NoError(t, err)
						require.NoError(t, s.recordEvent(sessionPrintEvent(t, filler)))
					}
					_, err = s.reader()
					require.NoError(t, err)
					part := s.buffer.Bytes()
					header, err := ParsePartHeader(bytes.NewReader(part))
					require.NoError(t, err)
					wroteBytes := int64(len(part)) - int64(header.PaddingSize)
					require.LessOrEqual(t, wroteBytes, proto.cfg.MinUploadBytes, "encrypted=%v offset=%d compressible=%d incompressible=%d", encrypted, offset, compressible, incompressible)
					maxOvershoot = max(maxOvershoot, wroteBytes-(proto.cfg.MinUploadBytes-sliceCloseReserveBytes))
				}
			}
		}
		// The sweep has to get close to the bound that sliceCloseReserveBytes
		// documents to prove it exercises the reserve rather than a lucky
		// alignment.
		require.Greater(t, maxOvershoot, minOvershoot, "encrypted=%v", encrypted)
		t.Logf("encrypted=%v: largest slice before padding is %d bytes past the cut", encrypted, maxOvershoot)
	}
}

func sessionPrintEvent(t *testing.T, data []byte) protoEvent {
	t.Helper()
	oneof, err := apievents.ToOneOf(&apievents.SessionPrint{
		Metadata: apievents.Metadata{Type: SessionPrintEvent},
		Data:     data,
	})
	require.NoError(t, err)
	return protoEvent{oneof: oneof}
}
