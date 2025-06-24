package protocol

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/db/oracle/testdata"
)

func TestParse(t *testing.T) {
	testCases := []struct {
		name               string
		packetDump         string
		protocolVersion    uint16
		verify             func(t *testing.T, packet Packet)
		expectedPacketType PacketType
	}{
		{
			name:               "connect packet",
			packetDump:         testdata.ConnectPacketDump,
			expectedPacketType: CONNECT,
			verify: func(t *testing.T, packet Packet) {
				connPacket, ok := packet.(*ConnectPacket)
				require.True(t, ok)
				const wantConnString = `(DESCRIPTION=(ADDRESS=(PROTOCOL=tcps)(HOST=127.0.0.1)(PORT=54557))(CONNECT_DATA=(CID=(PROGRAM=SQLcl)(HOST=__jdbc__)(USER=marek))(SERVICE_NAME=XE)(CONNECTION_ID=MAVsTlvrTyqsibsnisguzw==)))`

				connString, err := connPacket.GetConnectionString()
				require.NoError(t, err)
				require.Equal(t, wantConnString, connString)

				svcName, err := connPacket.GetServiceName()
				require.NoError(t, err)
				require.Equal(t, "XE", svcName)
			},
		},
		{
			name:               "connect packet split",
			packetDump:         testdata.ConnectPacketDumpSplit,
			expectedPacketType: CONNECT,
			verify: func(t *testing.T, packet Packet) {
				connPacket, ok := packet.(*ConnectPacket)
				require.True(t, ok)

				// need more data, parse another packet to fill in missing details.
				require.True(t, connPacket.needMoreData())

				dataPacketConnect, err := ParseDumpToPacket(false, testdata.DataPacketForConnectDataDump)
				require.NoError(t, err)

				err = connPacket.MaybeReadMoreData(&fixedPacketReader{packet: dataPacketConnect})
				require.NoError(t, err)

				require.False(t, connPacket.needMoreData())

				// verify details
				const wantConnString = `(DESCRIPTION=(CONNECT_TIMEOUT=5)(TRANSPORT_CONNECT_TIMEOUT=3)(RETRY_COUNT=3)(CONNECT_DATA=(SERVICE_NAME=DB0528_rhb_phx.sub05280840320.tener20250528.oraclevcn.com)(CID=(PROGRAM=sqlplus@db1)(HOST=db1)(USER=oracle)))(ADDRESS=(PROTOCOL=TCP)(HOST=10.0.0.83)(PORT=1521)))`
				connString, err := connPacket.GetConnectionString()
				require.NoError(t, err)
				require.Equal(t, wantConnString, connString)

				svcName, err := connPacket.GetServiceName()
				require.NoError(t, err)
				require.Equal(t, "DB0528_rhb_phx.sub05280840320.tener20250528.oraclevcn.com", svcName)
			},
		},
		{
			name:               "resend packet",
			packetDump:         testdata.ResendPacketDump,
			expectedPacketType: RESEND,
		},
		{
			name:               "refuse packet",
			packetDump:         testdata.RefusePacketDump,
			expectedPacketType: REFUSE,
			verify: func(t *testing.T, packet Packet) {
				refuse, ok := packet.(*RefusePacket)
				require.True(t, ok)
				require.Equal(t, "(DESCRIPTION=(TMP=)(VSNNUM=352321536)(ERR=12514)(ERROR_STACK=(ERROR=(CODE=12514)(EMFI=4))))", refuse.Message)
			},
		},
		{
			name:               "data packet",
			packetDump:         testdata.DataPacketDump,
			protocolVersion:    TNSVersionMinLargeSdu,
			expectedPacketType: DATA,
		},
		{
			name:               "accept packet",
			packetDump:         testdata.AcceptPacketDump,
			expectedPacketType: ACCEPT,
			verify: func(t *testing.T, packet Packet) {
				accept, ok := packet.(*AcceptPacket)
				require.True(t, ok)
				const wantProtocolVersion = uint16(0x13e)
				require.Equal(t, wantProtocolVersion, accept.ProtocolVersion)
			},
		},
		{
			name:               "redirect packet",
			packetDump:         testdata.RedirectPacketDump,
			expectedPacketType: REDIRECT,
			verify: func(t *testing.T, packet Packet) {
				redirect, ok := packet.(*RedirectPacket)
				require.True(t, ok)
				require.True(t, redirect.needMoreData())

				dataPacketRedirect, err := ParseDumpToPacket(false, testdata.DataPacketForRedirectDump)
				require.NoError(t, err)

				err = redirect.MaybeReadMoreData(&fixedPacketReader{packet: dataPacketRedirect})
				require.NoError(t, err)

				require.False(t, redirect.needMoreData())

				addr, err := redirect.RedirectAddress()
				require.NoError(t, err)
				require.Equal(t, "(ADDRESS=(PROTOCOL=TCP)(HOST=10.0.0.52)(PORT=1521))", addr)

				const wantConnStr = `(DESCRIPTION=(CONNECT_TIMEOUT=5)(TRANSPORT_CONNECT_TIMEOUT=3)(RETRY_COUNT=3)(CONNECT_DATA=(SERVICE_NAME=DB0528_rhb_phx.sub05280840320.tener20250528.oraclevcn.com)(CID=(PROGRAM=sqlplus@db1)(HOST=db1)(USER=oracle))(SERVER=dedicated)(INSTANCE_NAME=DB05281))(ADDRESS=(PROTOCOL=TCP)(HOST=10.0.0.83)(PORT=1521)))`
				connStr, err := redirect.RedirectConnectionString()
				require.NoError(t, err)
				require.Equal(t, wantConnStr, connStr)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pkt, err := ParseDumpToPacket(tc.protocolVersion >= TNSVersionMinLargeSdu, tc.packetDump)
			require.NoError(t, err)
			require.NotNil(t, pkt)
			require.Equal(t, tc.expectedPacketType, pkt.Type())
			if tc.verify != nil {
				tc.verify(t, pkt)
			}
		})
	}
}
