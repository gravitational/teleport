package oracle

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gravitational/teleport/e/lib/db/oracle/protocol"
)

// servicesRequestTCPS is an SNS request payload requesting services for TCPS login.
// We will send that to the server during TCPS auth flow.
var servicesRequestTCPS = decodeHexDumpOrPanic(`
00000000  de ad be ef 00 a2 15 c3  95 fa 00 04 00 00 04 00  |................|
00000010  03 00 00 00 00 00 04 00  05 15 c3 95 fa 00 08 00  |................|
00000020  01 09 09 09 09 09 09 09  09 00 12 00 01 de ad be  |................|
00000030  ef 00 03 00 00 00 04 00  04 00 01 00 02 00 03 00  |................|
00000040  01 00 05 00 00 00 00 00  04 00 05 15 c3 95 fa 00  |................|
00000050  02 00 03 e0 e1 00 02 00  06 fc ff 00 01 00 02 02  |................|
00000060  00 04 00 00 74 63 70 73  00 02 00 02 00 00 00 00  |....tcps........|
00000070  00 04 00 05 15 c3 95 fa  00 0c 00 01 00 01 08 0a  |................|
00000080  06 03 02 0b 0c 0f 10 11  00 03 00 02 00 00 00 00  |................|
00000090  00 04 00 05 15 c3 95 fa  00 06 00 01 00 01 03 04  |................|
000000a0  05 06                                             |..|`)

// servicesResponseNoServices is an SNS response payload from server that enables no services.
// We will send that to all clients as part of the auth flow, irrespective of the server auth mechanism (TCPS/Kerberos).
var servicesResponseNoServices = decodeHexDumpOrPanic(`
00000000  de ad be ef 00 75 00 00  00 00 00 04 00 00 04 00  |.....u..........|
00000010  03 00 00 00 00 00 04 00  05 15 00 00 00 00 02 00  |................|
00000020  06 00 1f 00 0e 00 01 de  ad be ef 00 03 00 00 00  |................|
00000030  02 00 04 00 01 00 01 00  02 00 00 00 00 00 04 00  |................|
00000040  05 15 00 10 00 00 02 00  06 fb ff 00 02 00 02 00  |................|
00000050  00 00 00 00 04 00 05 15  00 10 00 00 01 00 02 00  |................|
00000060  00 03 00 02 00 00 00 00  00 04 00 05 15 00 10 00  |................|
00000070  00 01 00 02 00                                    |.....|`)

// servicesRequestKerberos is an SNS request payload requesting services for Kerberos login.
// We will send that to the server during Kerberos auth flow.
var servicesRequestKerberos = decodeHexDumpOrPanic(`
00000000  de ad be ef 00 a9 0b 20  02 00 00 04 00 00 04 00  |....... ........|
00000010  03 00 00 00 00 00 04 00  05 0b 20 02 00 00 08 00  |.......... .....|
00000020  01 00 00 10 1c 66 ec 28  ea 00 12 00 01 de ad be  |.....f.(........|
00000030  ef 00 03 00 00 00 04 00  04 00 01 00 02 00 03 00  |................|
00000040  01 00 05 00 00 00 00 00  04 00 05 0b 20 02 00 00  |............ ...|
00000050  02 00 03 e0 e1 00 02 00  06 fc ff 00 01 00 02 01  |................|
00000060  00 09 00 00 4b 45 52 42  45 52 4f 53 35 00 02 00  |....KERBEROS5...|
00000070  03 00 00 00 00 00 04 00  05 0b 20 02 00 00 09 00  |.......... .....|
00000080  01 00 01 08 0a 06 02 0f  10 11 00 01 00 02 01 00  |................|
00000090  03 00 02 00 00 00 00 00  04 00 05 0b 20 02 00 00  |............ ...|
000000a0  06 00 01 00 01 03 04 05  06                       |.........|`)

// servicesResponseKerberosExpected19 is the expected Oracle server ver 19 response to the servicesRequestKerberos payload.
// We will use that for comparison and warn if there is mismatch.
var servicesResponseKerberosExpected19 = decodeHexDumpOrPanic(`
00000000  de ad be ef 00 9f 00 00  00 00 00 04 00 00 04 00  |................|
00000010  03 00 00 00 00 00 04 00  05 13 00 00 00 00 02 00  |................|
00000020  06 00 1f 00 0e 00 01 de  ad be ef 00 03 00 00 00  |................|
00000030  02 00 04 00 01 00 01 00  07 00 00 00 00 00 04 00  |................|
00000040  05 13 00 10 00 00 02 00  06 fa ff 00 01 00 02 01  |................|
00000050  00 09 00 00 4b 45 52 42  45 52 4f 53 35 00 04 00  |....KERBEROS5...|
00000060  05 13 00 00 00 00 04 00  04 00 00 00 09 00 04 00  |................|
00000070  04 00 00 00 02 00 02 00  02 00 00 00 00 00 04 00  |................|
00000080  05 13 00 10 00 00 01 00  02 00 00 03 00 02 00 00  |................|
00000090  00 00 00 04 00 05 13 00  10 00 00 01 00 02 00     |...............|
`)

// matchServicesResponseKerberosRegexp is version independent check for packets similar to servicesResponseKerberosExpected19. It replaces version-specific 0513 with 05.. regexp.
// Values seen so far, corresponding to the major server version: 0x13=19, 0x15=21, 0x17=23.
var matchServicesResponseKerberosRegexp = regexp.MustCompile(strings.ReplaceAll(fmt.Sprintf("%x", servicesResponseKerberosExpected19), "0513", "05.."))

func matchServicesResponseKerberosExpected(payload []byte) bool {
	formatted := fmt.Sprintf("%x", payload)
	return matchServicesResponseKerberosRegexp.MatchString(formatted)
}

// acknowledgeServicesKerberos is a payload to acknowledge the receipt of services and move to the next phase.
var acknowledgeServicesKerberos = decodeHexDumpOrPanic(`
00000000  de ad be ef 00 32 0b 20  02 00 00 01 00 00 01 00  |.....2. ........|
00000010  04 00 00 00 00 00 04 00  05 0b 20 02 00 00 04 00  |.......... .....|
00000020  04 00 00 00 09 00 04 00  04 00 00 00 02 00 01 00  |................|
00000030  02 01                                             |..|`)

// acknowledgeFinalKerberos is a payload to acknowledge the auth flow is finished.
var acknowledgeFinalKerberos = decodeHexDumpOrPanic(`
00000000  de ad be ef 00 19 0b 20  02 00 00 01 00 00 01 00  |....... ........|
00000010  01 00 00 00 00 00 00 00  01                       |.........|`)

func decodeHexDumpOrPanic(data string) []byte {
	out, err := protocol.DecodeHexDump(data)
	if err != nil {
		panic(err)
	}
	return out
}
