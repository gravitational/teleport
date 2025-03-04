package protocol

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/db/oracle/testdata"
)

func TestConnectPacket_ProtocolVersions(t *testing.T) {
	packet, err := ParseDumpToPacket(false, testdata.ConnectPacketDump)
	require.NoError(t, err)

	connect, ok := packet.(*ConnectPacket)
	require.True(t, ok)

	version, err := connect.GetProtocolVersion()
	require.NoError(t, err)
	require.Equal(t, 318, int(version))

	require.NoError(t, connect.SetProtocolVersion(314))

	version, err = connect.GetProtocolVersion()
	require.NoError(t, err)
	require.Equal(t, 314, int(version))
}
