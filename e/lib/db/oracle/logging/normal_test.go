package logging

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/db/oracle/protocol"
	"github.com/gravitational/teleport/e/lib/db/oracle/testdata"
	"github.com/gravitational/teleport/lib/srv/db/common/packetcapture"
)

func mustParseDumpToPacket[T protocol.Packet](t *testing.T, dump string) T {
	packet, err := protocol.ParseDumpToPacket(false, dump)
	require.NoError(t, err)
	packetTyped, ok := packet.(T)
	require.True(t, ok)
	return packetTyped
}

func TestNormalLogger(t *testing.T) {
	tmp := t.TempDir()
	log := filepath.Join(tmp, "normal.log")
	clock := clockwork.NewFakeClockAt(time.Date(2024, time.December, 4, 0, 0, 0, 0, time.UTC))
	logger := newNormalLogger(log, clock)
	connect := mustParseDumpToPacket[*protocol.ConnectPacket](t, testdata.ConnectPacketDump)
	refuse := mustParseDumpToPacket[*protocol.RefusePacket](t, testdata.RefusePacketDump)

	logger.LogHeader(packetcapture.ClientToTeleport, connect.Header())
	logger.LogPacket(packetcapture.ClientToTeleport, connect)
	logger.LogPacket(packetcapture.TeleportToServer, connect)
	logger.LogPacket(packetcapture.ServerToTeleport, refuse)
	logger.LogPacket(packetcapture.TeleportToClient, refuse)

	logger.Close()

	contents, err := os.ReadFile(log)
	require.NoError(t, err)

	expected := `Timestamp: 2024-12-04 00:00:00 +0000 UTC
Direction: Client->Teleport

00000000  01 05 00 00 01 00 00 00  01 3e 01 2c 0c 41 20 00  |.........>.,.A .|
00000010  ff ff 4f 98 00 00 00 01  00 bb 00 4a 00 00 00 00  |..O........J....|
00000020  81 81 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000030  00 00 00 00 00 00 00 00  00 00 00 00 20 00 00 20  |............ .. |
00000040  00 00 00 00 00 00 00 00  00 01 28 44 45 53 43 52  |..........(DESCR|
00000050  49 50 54 49 4f 4e 3d 28  41 44 44 52 45 53 53 3d  |IPTION=(ADDRESS=|
00000060  28 50 52 4f 54 4f 43 4f  4c 3d 74 63 70 73 29 28  |(PROTOCOL=tcps)(|
00000070  48 4f 53 54 3d 31 32 37  2e 30 2e 30 2e 31 29 28  |HOST=127.0.0.1)(|
00000080  50 4f 52 54 3d 35 34 35  35 37 29 29 28 43 4f 4e  |PORT=54557))(CON|
00000090  4e 45 43 54 5f 44 41 54  41 3d 28 43 49 44 3d 28  |NECT_DATA=(CID=(|
000000a0  50 52 4f 47 52 41 4d 3d  53 51 4c 63 6c 29 28 48  |PROGRAM=SQLcl)(H|
000000b0  4f 53 54 3d 5f 5f 6a 64  62 63 5f 5f 29 28 55 53  |OST=__jdbc__)(US|
000000c0  45 52 3d 6d 61 72 65 6b  29 29 28 53 45 52 56 49  |ER=marek))(SERVI|
000000d0  43 45 5f 4e 41 4d 45 3d  58 45 29 28 43 4f 4e 4e  |CE_NAME=XE)(CONN|
000000e0  45 43 54 49 4f 4e 5f 49  44 3d 4d 41 56 73 54 6c  |ECTION_ID=MAVsTl|
000000f0  76 72 54 79 71 73 69 62  73 6e 69 73 67 75 7a 77  |vrTyqsibsnisguzw|
00000100  3d 3d 29 29 29                                    |==)))|


Timestamp: 2024-12-04 00:00:00 +0000 UTC
Direction: Teleport->Server

00000000  01 05 00 00 01 00 00 00  01 3e 01 2c 0c 41 20 00  |.........>.,.A .|
00000010  ff ff 4f 98 00 00 00 01  00 bb 00 4a 00 00 00 00  |..O........J....|
00000020  81 81 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000030  00 00 00 00 00 00 00 00  00 00 00 00 20 00 00 20  |............ .. |
00000040  00 00 00 00 00 00 00 00  00 01 28 44 45 53 43 52  |..........(DESCR|
00000050  49 50 54 49 4f 4e 3d 28  41 44 44 52 45 53 53 3d  |IPTION=(ADDRESS=|
00000060  28 50 52 4f 54 4f 43 4f  4c 3d 74 63 70 73 29 28  |(PROTOCOL=tcps)(|
00000070  48 4f 53 54 3d 31 32 37  2e 30 2e 30 2e 31 29 28  |HOST=127.0.0.1)(|
00000080  50 4f 52 54 3d 35 34 35  35 37 29 29 28 43 4f 4e  |PORT=54557))(CON|
00000090  4e 45 43 54 5f 44 41 54  41 3d 28 43 49 44 3d 28  |NECT_DATA=(CID=(|
000000a0  50 52 4f 47 52 41 4d 3d  53 51 4c 63 6c 29 28 48  |PROGRAM=SQLcl)(H|
000000b0  4f 53 54 3d 5f 5f 6a 64  62 63 5f 5f 29 28 55 53  |OST=__jdbc__)(US|
000000c0  45 52 3d 6d 61 72 65 6b  29 29 28 53 45 52 56 49  |ER=marek))(SERVI|
000000d0  43 45 5f 4e 41 4d 45 3d  58 45 29 28 43 4f 4e 4e  |CE_NAME=XE)(CONN|
000000e0  45 43 54 49 4f 4e 5f 49  44 3d 4d 41 56 73 54 6c  |ECTION_ID=MAVsTl|
000000f0  76 72 54 79 71 73 69 62  73 6e 69 73 67 75 7a 77  |vrTyqsibsnisguzw|
00000100  3d 3d 29 29 29                                    |==)))|


Timestamp: 2024-12-04 00:00:00 +0000 UTC
Direction: Server->Teleport

00000000  00 67 00 00 04 00 00 00  22 00 00 5b 28 44 45 53  |.g......"..[(DES|
00000010  43 52 49 50 54 49 4f 4e  3d 28 54 4d 50 3d 29 28  |CRIPTION=(TMP=)(|
00000020  56 53 4e 4e 55 4d 3d 33  35 32 33 32 31 35 33 36  |VSNNUM=352321536|
00000030  29 28 45 52 52 3d 31 32  35 31 34 29 28 45 52 52  |)(ERR=12514)(ERR|
00000040  4f 52 5f 53 54 41 43 4b  3d 28 45 52 52 4f 52 3d  |OR_STACK=(ERROR=|
00000050  28 43 4f 44 45 3d 31 32  35 31 34 29 28 45 4d 46  |(CODE=12514)(EMF|
00000060  49 3d 34 29 29 29 29                              |I=4))))|


Timestamp: 2024-12-04 00:00:00 +0000 UTC
Direction: Teleport->Client

00000000  00 67 00 00 04 00 00 00  22 00 00 5b 28 44 45 53  |.g......"..[(DES|
00000010  43 52 49 50 54 49 4f 4e  3d 28 54 4d 50 3d 29 28  |CRIPTION=(TMP=)(|
00000020  56 53 4e 4e 55 4d 3d 33  35 32 33 32 31 35 33 36  |VSNNUM=352321536|
00000030  29 28 45 52 52 3d 31 32  35 31 34 29 28 45 52 52  |)(ERR=12514)(ERR|
00000040  4f 52 5f 53 54 41 43 4b  3d 28 45 52 52 4f 52 3d  |OR_STACK=(ERROR=|
00000050  28 43 4f 44 45 3d 31 32  35 31 34 29 28 45 4d 46  |(CODE=12514)(EMF|
00000060  49 3d 34 29 29 29 29                              |I=4))))|


`

	require.Equal(t, expected, string(contents))
}
