package oracle

import (
	"context"
	"encoding/hex"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/db/oracle/connection"
	"github.com/gravitational/teleport/e/lib/db/oracle/protocol"
)

func Test_performKerberosAuth(t *testing.T) {
	// we use fixed client and server payloads;
	// - for client, the data is the entire packet (including header), and is used for comparison.
	// - for server payloads, this data is sent to the client for consumption.

	// ask for Kerberos auth
	const clientExpectedPacket1 = `
00000000  00 b3 00 00 06 00 00 00  00 00 de ad be ef 00 a9  |................|
00000010  0b 20 02 00 00 04 00 00  04 00 03 00 00 00 00 00  |. ..............|
00000020  04 00 05 0b 20 02 00 00  08 00 01 00 00 10 1c 66  |.... ..........f|
00000030  ec 28 ea 00 12 00 01 de  ad be ef 00 03 00 00 00  |.(..............|
00000040  04 00 04 00 01 00 02 00  03 00 01 00 05 00 00 00  |................|
00000050  00 00 04 00 05 0b 20 02  00 00 02 00 03 e0 e1 00  |...... .........|
00000060  02 00 06 fc ff 00 01 00  02 01 00 09 00 00 4b 45  |..............KE|
00000070  52 42 45 52 4f 53 35 00  02 00 03 00 00 00 00 00  |RBEROS5.........|
00000080  04 00 05 0b 20 02 00 00  09 00 01 00 01 08 0a 06  |.... ...........|
00000090  02 0f 10 11 00 01 00 02  01 00 03 00 02 00 00 00  |................|
000000a0  00 00 04 00 05 0b 20 02  00 00 06 00 01 00 01 03  |...... .........|
000000b0  04 05 06                                          |...|
`

	// ack services
	const clientExpectedPacket2 = `
00000000  00 3c 00 00 06 00 00 00  00 00 de ad be ef 00 32  |.<.............2|
00000010  0b 20 02 00 00 01 00 00  01 00 04 00 00 00 00 00  |. ..............|
00000020  04 00 05 0b 20 02 00 00  04 00 04 00 00 00 09 00  |.... ...........|
00000030  04 00 04 00 00 00 02 00  01 00 02 01              |............|
`

	// send ticket data
	const clientExpectedPacket3 = `
00000000  00 45 00 00 06 00 00 00  00 00 de ad be ef 00 3b  |.E.............;|
00000010  0b 20 02 00 00 01 00 00  01 00 04 00 00 00 00 00  |. ..............|
00000020  02 00 03 00 02 00 04 00  04 00 00 00 04 00 04 00  |................|
00000030  01 7f 00 00 01 00 0c 00  01 64 75 6d 6d 79 20 74  |.........dummy t|
00000040  69 63 6b 65 74                                    |icket|
`

	// ack auth flow done
	const clientExpectedPacket4 = `
00000000  00 23 00 00 06 00 00 00  00 00 de ad be ef 00 19  |.#..............|
00000010  0b 20 02 00 00 01 00 00  01 00 01 00 00 00 00 00  |. ..............|
00000020  00 00 01                                          |...|
`

	// reply with just Kerberos auth service enabled
	const serverPayload1 = `
00000000  de ad be ef 00 9f 00 00  00 00 00 04 00 00 04 00  |................|
00000010  03 00 00 00 00 00 04 00  05 15 00 00 00 00 02 00  |................|
00000020  06 00 1f 00 0e 00 01 de  ad be ef 00 03 00 00 00  |................|
00000030  02 00 04 00 01 00 01 00  07 00 00 00 00 00 04 00  |................|
00000040  05 15 00 10 00 00 02 00  06 fa ff 00 01 00 02 01  |................|
00000050  00 09 00 00 4b 45 52 42  45 52 4f 53 35 00 04 00  |....KERBEROS5...|
00000060  05 15 00 00 00 00 04 00  04 00 00 00 09 00 04 00  |................|
00000070  04 00 00 00 02 00 02 00  02 00 00 00 00 00 04 00  |................|
00000080  05 15 00 10 00 00 01 00  02 00 00 03 00 02 00 00  |................|
00000090  00 00 00 04 00 05 15 00  10 00 00 01 00 02 00     |...............|
`

	// SPN details
	const serverPayload2 = `
00000000  de ad be ef 00 3d 00 00  00 00 00 01 00 00 01 00  |.....=..........|
00000010  02 00 00 00 00 00 14 00  00 64 62 2d 6c 78 35 64  |.........db-lx5d|
00000020  35 6d 75 7a 35 63 7a 74  6d 77 36 6d 71 00 0c 00  |5muz5cztmw6mq...|
00000030  00 64 62 2e 6f 72 61 61  64 2e 63 6f 6d           |.db.oraad.com|
`

	// final ack
	const serverPayload3 = `
00000000  de ad be ef 00 1e 00 00  00 00 00 01 00 00 01 00  |................|
00000010  02 00 00 00 00 00 01 00  02 00 00 00 00 01        |..............|
`

	sendPayload := func(conn *connection.OracleConn, hexPayload string) error {
		payload, err := protocol.DecodeHexDump(hexPayload)
		if err != nil {
			return trace.Wrap(err)
		}

		dp, err := protocol.DataPacketFromPayload(conn.LargeSDU(), payload)
		if err != nil {
			return trace.Wrap(err)
		}

		return conn.WritePacket(dp)
	}

	serverEnd, clientEnd := net.Pipe()
	t.Cleanup(func() {
		_ = serverEnd.Close()
		_ = clientEnd.Close()
	})

	serverHandler := func() error {
		clientConn, err := connection.NewConn(serverEnd)
		if err != nil {
			return trace.Wrap(err)
		}

		payloads := []struct{ clientPacket, serverPayload string }{
			{clientPacket: clientExpectedPacket1, serverPayload: serverPayload1},
			{clientPacket: clientExpectedPacket2, serverPayload: serverPayload2},
			{clientPacket: clientExpectedPacket3, serverPayload: serverPayload3},
			{clientPacket: clientExpectedPacket4, serverPayload: ""},
		}

		for _, pair := range payloads {
			// receive client packet and verify contents
			dp, err := clientConn.ReadPacket()
			if err != nil {
				return trace.Wrap(err)
			}
			expected := strings.TrimSpace(pair.clientPacket)
			actual := strings.TrimSpace(hex.Dump(dp.Payload()))
			if !assert.Equal(t, expected, actual) {
				return trace.BadParameter("payload mismatch")
			}

			// send response
			if pair.serverPayload != "" {
				err = sendPayload(clientConn, pair.serverPayload)
				if err != nil {
					return trace.Wrap(err)
				}
			}
		}
		return nil
	}
	chErr := make(chan error)
	go func() {
		err := serverHandler()
		chErr <- err
	}()

	serverConn, err := connection.NewConn(clientEnd)
	require.NoError(t, err)

	go func() {
		authenticate := func(username string, params protocol.KerberosAuthParams) ([]byte, error) {
			if params.ServerInstance != "db.oraad.com" {
				return nil, trace.BadParameter("unexpected server instance %q", params.ServerInstance)
			}
			if params.ServiceClass != "db-lx5d5muz5cztmw6mq" {
				return nil, trace.BadParameter("unexpected service class %q", params.ServiceClass)
			}

			t.Logf("instance=%v, class=%v", params.ServerInstance, params.ServiceClass)
			response := "dummy ticket"
			return []byte(response), nil
		}
		chErr <- performKerberosAuth(context.Background(), slog.Default(), "", authenticate, serverConn)
	}()

	for range 2 {
		select {
		case err = <-chErr:
			require.NoError(t, err)
		case <-time.After(time.Second * 3):
			t.Fatalf("timed out waiting for test goroutine to finish")
		}
	}
}
