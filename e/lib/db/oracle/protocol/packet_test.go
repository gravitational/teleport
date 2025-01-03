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
		expectedPacketType Type
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
