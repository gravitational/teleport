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

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/srv/db/sqlserver/protocol/fixtures"
)

func FuzzMSSQLLogin(f *testing.F) {
	f.Add([]byte("\x100\x00x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"))
	f.Add([]byte("\x100\x00x00000000000000000000000000000000000000000000\xff\xff0\x800000000000000000000000000\x00 \x000000000000000000000000000000000000000000"))
	f.Add([]byte("\x100\x00x00000000000000000000000000000000000000000000 \x00 \x800000000000000000000000000\x00\xff\xff0000000000000000000000000000000000000000"))
	f.Add([]byte("\x100\x00x000000000000000000000000000000000000000000000\x00 \x8000000000000000000000000000000000000000000000000000000000000000000000"))
	f.Add([]byte("\x100\x00x000000000000000000000000000000000000000000000\x00 \x800000000000000000000000000\x00\xff\xff0000000000000000000000000000000000000000"))
	f.Add([]byte("\x100\x00x000000000000000000000000000000000000000000000\x00\xff\xff0000000000000000000000000\x00 \x000000000000000000000000000000000000000000"))
	f.Add([]byte("\x100\x00x000000000000000000000000000000000000000000000\x00\x00\x800000000000000000000000000\x00\xff\xff0000000000000000000000000000000000000000"))

	f.Fuzz(func(t *testing.T, packet []byte) {
		reader := bytes.NewReader(packet)

		require.NotPanics(t, func() {
			_, _ = ReadLogin7Packet(reader)
		})
	})
}

func FuzzRPCClientPartialLength(f *testing.F) {
	f.Fuzz(func(t *testing.T, length uint64, chunks uint64) {
		packet, err := ReadPacket(bytes.NewReader(fixtures.RPCClientPartiallyLength("foo3", length, chunks)))
		require.NoError(t, err)
		require.Equal(t, PacketTypeRPCRequest, packet.Type())

		// Given that `ToSQLPacket` recovers from panics when reading the packet,
		// we just need to ensure the function doesn't return error.
		_, err = ToSQLPacket(packet)
		require.NoError(t, err)
	})
}
