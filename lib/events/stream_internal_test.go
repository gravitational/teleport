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
	"encoding/binary"
	"math"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
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
