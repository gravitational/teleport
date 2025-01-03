package oracle

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/lib/db/oracle/protocol"
)

// requestSNSTCPS is an SNS request from a client performing TCPS login.
//
// Transparent Network Substrate Protocol
//
//	Packet Length: 172
//	Packet Type: Data (6)
//	Reserved Byte: 20
//	Header Checksum: 0x0000
//	Data
//	    Data Flag: 0x0000
//	    Data ID: Secure Network Services (0xdeadbeef)
//	    Data Length: 162 bytes
//	    Client Version: 21.12.9.5.250
//	    Services: 4
//	    Data (162 bytes)
//	        Data […]: deadbeef00a215c395fa00040000040003000000000004000515c395fa00080001090909090909090900120001deadbeef000300000004000400010002000300010005000000000004000515c395fa00020003e0e100020006fcff000100020200040000746370730002000200000000000
//	        [Length: 162]
var requestSNSTCPS = parsePacketOrPanic(true, `
0000   00 00 00 ac 06 20 00 00 00 00 de ad be ef 00 a2   ..... ..........
0010   15 c3 95 fa 00 04 00 00 04 00 03 00 00 00 00 00   ................
0020   04 00 05 15 c3 95 fa 00 08 00 01 09 09 09 09 09   ................
0030   09 09 09 00 12 00 01 de ad be ef 00 03 00 00 00   ................
0040   04 00 04 00 01 00 02 00 03 00 01 00 05 00 00 00   ................
0050   00 00 04 00 05 15 c3 95 fa 00 02 00 03 e0 e1 00   ................
0060   02 00 06 fc ff 00 01 00 02 02 00 04 00 00 74 63   ..............tc
0070   70 73 00 02 00 02 00 00 00 00 00 04 00 05 15 c3   ps..............
0080   95 fa 00 0c 00 01 00 01 08 0a 06 03 02 0b 0c 0f   ................
0090   10 11 00 03 00 02 00 00 00 00 00 04 00 05 15 c3   ................
00a0   95 fa 00 06 00 01 00 01 03 04 05 06               ............`)

var requestSNSTCPS314 = parsePacketOrPanic(false, `
0000   00 ac 00 00 06 00 00 00 00 00 de ad be ef 00 a2   ..... ..........
0010   15 c3 95 fa 00 04 00 00 04 00 03 00 00 00 00 00   ................
0020   04 00 05 15 c3 95 fa 00 08 00 01 09 09 09 09 09   ................
0030   09 09 09 00 12 00 01 de ad be ef 00 03 00 00 00   ................
0040   04 00 04 00 01 00 02 00 03 00 01 00 05 00 00 00   ................
0050   00 00 04 00 05 15 c3 95 fa 00 02 00 03 e0 e1 00   ................
0060   02 00 06 fc ff 00 01 00 02 02 00 04 00 00 74 63   ..............tc
0070   70 73 00 02 00 02 00 00 00 00 00 04 00 05 15 c3   ps..............
0080   95 fa 00 0c 00 01 00 01 08 0a 06 03 02 0b 0c 0f   ................
0090   10 11 00 03 00 02 00 00 00 00 00 04 00 05 15 c3   ................
00a0   95 fa 00 06 00 01 00 01 03 04 05 06               ............`)

// responseSNSPassword is a response to requestSNSPassword.
//
// Transparent Network Substrate Protocol
//
//	Packet Length: 127
//	Packet Type: Data (6)
//	Reserved Byte: 20
//	Header Checksum: 0x0000
//	Data
//	    Data Flag: 0x0000
//	        .... .... .... ...0 = Send Token: False
//	        .... .... .... ..0. = Request Confirmation: False
//	        .... .... .... .0.. = Confirmation: False
//	        .... .... .... 0... = Reserved: False
//	        .... .... ..0. .... = More Data to Come: False
//	        .... .... .0.. .... = End of File: False
//	        .... .... 0... .... = Do Immediate Confirmation: False
//	        .... ...0 .... .... = Request To Send: False
//	        .... ..0. .... .... = Send NT Trailer: False
//	    Data ID: Secure Network Services (0xdeadbeef)
//	    Data Length: 117 bytes
//	    Client Version: 0.0.0.0.0
//	    Services: 4
//	    Data (117 bytes)
//	        Data […]: deadbeef0075000000000004000004000300000000000400051500000000020006001f000e0001deadbeef000300000002000400010001000200000000000400051500100000020006fbff00020002000000000004000515001000000100020000030002000000000004000515001000000
//	        [Length: 117]
var responseSNSPassword = parsePacketOrPanic(true, `
0000   00 00 00 7f 06 20 00 00 00 00 de ad be ef 00 75   ..... .........u
0010   00 00 00 00 00 04 00 00 04 00 03 00 00 00 00 00   ................
0020   04 00 05 15 00 00 00 00 02 00 06 00 1f 00 0e 00   ................
0030   01 de ad be ef 00 03 00 00 00 02 00 04 00 01 00   ................
0040   01 00 02 00 00 00 00 00 04 00 05 15 00 10 00 00   ................
0050   02 00 06 fb ff 00 02 00 02 00 00 00 00 00 04 00   ................
0060   05 15 00 10 00 00 01 00 02 00 00 03 00 02 00 00   ................
0070   00 00 00 04 00 05 15 00 10 00 00 01 00 02 00      ...............`)

var responseSNSPassword314 = parsePacketOrPanic(false, `
0000   00 7f 00 00 06 00 00 00 00 00 de ad be ef 00 75   ...............u
0010   00 00 00 00 00 04 00 00 04 00 03 00 00 00 00 00   ................
0020   04 00 05 15 00 00 00 00 02 00 06 00 1f 00 0e 00   ................
0030   01 de ad be ef 00 03 00 00 00 02 00 04 00 01 00   ................
0040   01 00 02 00 00 00 00 00 04 00 05 15 00 10 00 00   ................
0050   02 00 06 fb ff 00 02 00 02 00 00 00 00 00 04 00   ................
0060   05 15 00 10 00 00 01 00 02 00 00 03 00 02 00 00   ................
0070   00 00 00 04 00 05 15 00 10 00 00 01 00 02 00      ...............`)

func parsePacketOrPanic(largeSDU bool, dump string) *protocol.DataPacket {
	// all hardcoded packets are using large SDU
	packet, err := protocol.ParseDumpToPacket(largeSDU, dump)
	if err != nil {
		panic(trace.Wrap(err, "failed to parse packet"))
	}

	// expect DATA packet.
	dataPacket, ok := packet.(*protocol.DataPacket)
	if !ok {
		panic(trace.BadParameter("unexpected packet type: %T (%v)", packet, packet.Type()))
	}

	if dataPacket == nil {
		panic(trace.BadParameter("dataPacket is nil"))
	}

	return dataPacket
}
