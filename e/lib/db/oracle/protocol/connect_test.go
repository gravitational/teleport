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

	version, err = connect.GetCompatProtocolVersion()
	require.NoError(t, err)
	require.Equal(t, 300, int(version))

	require.NoError(t, connect.SetProtocolVersion(314))
	require.NoError(t, connect.SetCompatProtocolVersion(299))

	version, err = connect.GetProtocolVersion()
	require.NoError(t, err)
	require.Equal(t, 314, int(version))

	version, err = connect.GetCompatProtocolVersion()
	require.NoError(t, err)
	require.Equal(t, 299, int(version))
}
