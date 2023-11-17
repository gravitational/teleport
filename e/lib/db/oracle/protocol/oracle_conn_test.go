package protocol

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/db/oracle/protocol/testdata"
)

func TestConn(t *testing.T) {
	t.Parallel()
	type check func(t *testing.T, err error, packet Packet)

	hasNoErr := func() check {
		return func(t *testing.T, err error, packet Packet) {
			require.NoError(t, err)
		}
	}

	hasPacketType := func(wantType Type) check {
		return func(t *testing.T, err error, packet Packet) {
			require.Equal(t, wantType, packet.Type(), "packet type mismatch")
		}
	}

	testCases := []struct {
		name            string
		packetDump      string
		protocolVersion uint16
		checks          []check
		isServerConn    bool
	}{
		{
			name:       "connect packet",
			packetDump: testdata.ConnectPacketDump,
			checks: []check{
				hasNoErr(),
				func(t *testing.T, err error, packet Packet) {
					connPacket, ok := packet.(*ConnectPacket)
					require.True(t, ok)
					const wantConnString = `(DESCRIPTION=(ADDRESS=(PROTOCOL=tcps)(HOST=127.0.0.1)(PORT=54557))(CONNECT_DATA=(CID=(PROGRAM=SQLcl)(HOST=__jdbc__)(USER=marek))(SERVICE_NAME=XE)(CONNECTION_ID=MAVsTlvrTyqsibsnisguzw==)))`
					require.Equal(t, wantConnString, connPacket.ConnectionString)
					require.Equal(t, "XE", connPacket.ServerName)
				},
			},
		},
		{
			name:       "resend packet",
			packetDump: testdata.ResendPacketDump,
			checks: []check{
				hasNoErr(),
				hasPacketType(RESEND),
			},
		},
		{
			name:            "data packet",
			packetDump:      testdata.DataPacketDump,
			protocolVersion: TNSVersionMinLargeSdu,
			checks: []check{
				hasNoErr(),
				hasPacketType(DATA),
			},
		},
		{
			name:       "accept packet",
			packetDump: testdata.AcceptPacketDump,
			checks: []check{
				hasNoErr(),
				hasPacketType(ACCEPT),
				func(t *testing.T, err error, packet Packet) {
					accept, ok := packet.(*AcceptPacket)
					require.True(t, ok)
					const wantProtocolVersion = uint16(0x13e)
					require.Equal(t, wantProtocolVersion, accept.ProtocolVersion)
				},
			},
		},
		{
			name:            "data server param packet",
			packetDump:      testdata.DataParameters,
			protocolVersion: TNSVersionMinLargeSdu,
			isServerConn:    true,
			checks: []check{
				hasNoErr(),
				hasPacketType(DATA),
				func(t *testing.T, err error, packet Packet) {
					data, ok := packet.(*DataPacket)
					require.True(t, ok)
					require.Equal(t, "117", data.Parameters[AuthSessionIDKey])
					require.Equal(t, "xe", data.Parameters[AuthSCServiceNameKey])
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pr, pw := net.Pipe()
			defer pr.Close()
			defer pw.Close()

			conn := oracleConn{
				Conn:            pr,
				protocolVersion: tc.protocolVersion,
				isServerConn:    tc.isServerConn,
			}
			defer conn.Close()

			go func() {
				_, err := pw.Write(MustDecodePacketDump(t, tc.packetDump))
				assert.NoError(t, err)
			}()

			packet, err := conn.readPacket()
			for _, ch := range tc.checks {
				ch(t, err, packet)
			}
		})
	}
}
