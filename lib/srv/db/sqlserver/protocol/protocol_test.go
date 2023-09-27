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
	require.Equal(t, "", packet.Database())
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
	require.Equal(t, r.Type(), PacketTypeSQLBatch)
	p, ok := r.(*SQLBatch)
	require.True(t, ok)
	require.Equal(t, "\nselect 'foo' as 'bar'\n        ", p.SQLText)
}

// TestRPCClientRequestParam verifies RPC Request with param packet parsing.
func TestRPCClientRequestParam(t *testing.T) {
	packet, err := ReadPacket(bytes.NewReader(fixtures.GenerateExecuteSQLRPCPacket("select @@version")))
	require.NoError(t, err)
	require.Equal(t, packet.Type(), PacketTypeRPCRequest)
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
	require.Equal(t, packet.Type(), PacketTypeRPCRequest)
	r, err := ToSQLPacket(packet)
	require.NoError(t, err)
	p, ok := r.(*RPCRequest)
	require.True(t, ok)
	require.Equal(t, "foo3", p.ProcName)
}

func TestRPCClientRequestPartialLength(t *testing.T) {
	packet, err := ReadPacket(bytes.NewReader(fixtures.RPCClientPartiallyLength("foo3", 32, 4)))
	require.NoError(t, err)
	require.Equal(t, packet.Type(), PacketTypeRPCRequest)

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
	require.Equal(t, packet.Type(), PacketTypeRPCRequest)
	r, err := ToSQLPacket(packet)
	require.NoError(t, err)
	p, ok := r.(*RPCRequest)
	require.True(t, ok)
	require.Equal(t, "select @@version", p.Parameters[0])
}
