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

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/srv/db/sqlserver/protocol/fixtures"
)

// TestReadPreLogin verifies Pre-Login packet parsing.
func TestReadPreLogin(t *testing.T) {
	_, err := ReadPreLoginPacket(bytes.NewBuffer(fixtures.PreLogin))
	require.NoError(t, err)
}

// TestWritePreLoginResponse verifies Pre-Login response written to the client.
func TestWritePreLoginResponse(t *testing.T) {
	b := &bytes.Buffer{}

	err := WritePreLoginResponse(b)
	require.NoError(t, err)

	packet, err := ReadPacket(b)
	require.NoError(t, err)
	require.Equal(t, PacketTypeResponse, packet.Type())
}

// TestReadLogin7 verifies Login7 packet parsing.
func TestReadLogin7(t *testing.T) {
	packet, err := ReadLogin7Packet(bytes.NewBuffer(fixtures.Login7))
	require.NoError(t, err)
	require.Equal(t, "sa", packet.Username())
	require.Empty(t, packet.Database())
}

// TestErrorResponse verifies writing error response.
func TestErrorResponse(t *testing.T) {
	b := &bytes.Buffer{}

	err := WriteErrorResponse(b, trace.AccessDenied("access denied"))
	require.NoError(t, err)
}

// TestSQLBatch verifies SQLPatch packet parsing.
func TestSQLBatch(t *testing.T) {
	packet, err := ReadPacket(bytes.NewReader(fixtures.GenerateBatchQueryPacket("\nselect 'foo' as 'bar'\n        ")))
	require.NoError(t, err)
	r, err := ToSQLPacket(packet)
	require.NoError(t, err)
	require.Equal(t, PacketTypeSQLBatch, r.Type())
	p, ok := r.(*SQLBatch)
	require.True(t, ok)
	require.Equal(t, "\nselect 'foo' as 'bar'\n        ", p.SQLText)
}

// TestRPCClientRequestParam verifies RPC Request with param packet parsing.
func TestRPCClientRequestParam(t *testing.T) {
	packet, err := ReadPacket(bytes.NewReader(fixtures.GenerateExecuteSQLRPCPacket("select @@version")))
	require.NoError(t, err)
	require.Equal(t, PacketTypeRPCRequest, packet.Type())
	r, err := ToSQLPacket(packet)
	require.NoError(t, err)
	p, ok := r.(*RPCRequest)
	require.True(t, ok)
	require.Equal(t, "select @@version", p.Parameters[0])
}

// TestRPCClientRequest verifies rpc request packet parsing.
func TestRPCClientRequest(t *testing.T) {
	packet, err := ReadPacket(bytes.NewReader(fixtures.GenerateCustomRPCCallPacket("foo3")))
	require.NoError(t, err)
	require.Equal(t, PacketTypeRPCRequest, packet.Type())
	r, err := ToSQLPacket(packet)
	require.NoError(t, err)
	p, ok := r.(*RPCRequest)
	require.True(t, ok)
	require.Equal(t, "foo3", p.ProcName)
}

func TestRPCClientRequestPartialLength(t *testing.T) {
	packet, err := ReadPacket(bytes.NewReader(fixtures.RPCClientPartiallyLength("foo3", 32, 4)))
	require.NoError(t, err)
	require.Equal(t, PacketTypeRPCRequest, packet.Type())

	r, err := ToSQLPacket(packet)
	require.NoError(t, err)

	p, ok := r.(*RPCRequest)
	require.True(t, ok)
	require.Equal(t, "foo3", p.ProcName)
	require.NoError(t, err)
}

func TestRPCClientRequestParamNTEXT(t *testing.T) {
	// Currently the ReadTypeInfo is not parsing the NTEXT contents correctly,
	// giving a invalid memory access.
	//
	// TODO(gabrielcorado): validate this use case and ensure the parameter is
	// correctly parsed on the driver.
	t.Skip()

	packet, err := ReadPacket(bytes.NewReader(fixtures.GenerateExecuteSQLRPCPacketNTEXT("select @@version")))
	require.NoError(t, err)
	require.Equal(t, PacketTypeRPCRequest, packet.Type())
	r, err := ToSQLPacket(packet)
	require.NoError(t, err)
	p, ok := r.(*RPCRequest)
	require.True(t, ok)
	require.Equal(t, "select @@version", p.Parameters[0])
}
