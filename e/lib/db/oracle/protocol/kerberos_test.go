package protocol

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildKerberosTokenPayload(t *testing.T) {
	var token []byte
	for range 30 {
		token = append(token, 0xAA, 0xBB, 0xCC, 0xDD)
	}

	payload, err := BuildKerberosTokenPayload(token)
	require.NoError(t, err)
	require.NotNil(t, payload)

	payloadHex := hex.Dump(payload)

	const expected = `00000000  de ad be ef 00 a7 0b 20  02 00 00 01 00 00 01 00  |....... ........|
00000010  04 00 00 00 00 00 02 00  03 00 02 00 04 00 04 00  |................|
00000020  00 00 04 00 04 00 01 7f  00 00 01 00 78 00 01 aa  |............x...|
00000030  bb cc dd aa bb cc dd aa  bb cc dd aa bb cc dd aa  |................|
00000040  bb cc dd aa bb cc dd aa  bb cc dd aa bb cc dd aa  |................|
00000050  bb cc dd aa bb cc dd aa  bb cc dd aa bb cc dd aa  |................|
00000060  bb cc dd aa bb cc dd aa  bb cc dd aa bb cc dd aa  |................|
00000070  bb cc dd aa bb cc dd aa  bb cc dd aa bb cc dd aa  |................|
00000080  bb cc dd aa bb cc dd aa  bb cc dd aa bb cc dd aa  |................|
00000090  bb cc dd aa bb cc dd aa  bb cc dd aa bb cc dd aa  |................|
000000a0  bb cc dd aa bb cc dd                              |.......|
`
	require.Equal(t, expected, payloadHex)

	dp, err := DataPacketFromPayload(true, payload)
	require.NoError(t, err)

	require.True(t, dp.HasSecureNetworkServices())
	require.NoError(t, VerifySNSPacket(dp))
}

func TestParseKerberosAuthParams(t *testing.T) {
	const ticketData = `
00000000  de ad be ef 00 3d 00 00  00 00 00 01 00 00 01 00  |.....=..........|
00000010  02 00 00 00 00 00 14 00  00 64 62 2d 6c 78 35 64  |.........db-lx5d|
00000020  35 6d 75 7a 35 63 7a 74  6d 77 36 6d 71 00 0c 00  |5muz5cztmw6mq...|
00000030  00 64 62 2e 6f 72 61 61  64 2e 63 6f 6d           |.db.oraad.com|
 `

	ticketDataBytes, err := DecodeHexDump(ticketData)
	require.NoError(t, err)

	dp, err := DataPacketFromPayload(true, ticketDataBytes)
	require.NoError(t, err)

	data, err := ParseKerberosAuthParams(dp)
	require.NoError(t, err)
	require.NotNil(t, data)

	require.Equal(t, "db-lx5d5muz5cztmw6mq", data.ServiceClass)
	require.Equal(t, "db.oraad.com", data.ServerInstance)
}
