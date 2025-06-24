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

func TestConnectPacketWithConnectionString(t *testing.T) {
	packet, err := ParseDumpToPacket(false, testdata.ConnectPacketDump)
	require.NoError(t, err)

	connect, ok := packet.(*ConnectPacket)
	require.True(t, ok)

	const connStr = "dummy connection string"
	updated, err := connect.WithConnectionString(connStr)
	require.NoError(t, err)

	connStrNew, err := updated.GetConnectionString()
	require.NoError(t, err)
	require.Equal(t, connStr, connStrNew)
}

func FuzzConnectPacketWithConnectionString(f *testing.F) {
	f.Add(testdata.ConnectPacketDump, "", "foo bar")
	f.Add(testdata.ConnectPacketDumpSplit, testdata.DataPacketForConnectDataDump, "foo bar")

	f.Fuzz(func(t *testing.T, connectPacketDump string, dataPacketDump string, connStr string) {
		packet, err := ParseDumpToPacket(false, connectPacketDump)
		if err != nil {
			return
		}

		connect, ok := packet.(*ConnectPacket)
		if !ok {
			return
		}

		_, err = connect.GetConnectionString()
		if err != nil {
			return
		}

		if connect.needMoreData() {
			packetData, err := ParseDumpToPacket(false, dataPacketDump)
			if err != nil {
				return
			}
			err = connect.MaybeReadMoreData(&fixedPacketReader{packet: packetData})
			if err != nil {
				return
			}
		}

		updated, err := connect.WithConnectionString(connStr)
		require.NoError(t, err)

		connStrNew, err := updated.GetConnectionString()
		require.NoError(t, err)

		require.Equal(t, connStr, connStrNew)
	})
}
