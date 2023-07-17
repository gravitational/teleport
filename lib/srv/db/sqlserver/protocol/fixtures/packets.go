/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package fixtures

import (
	"encoding/binary"
	"unicode/utf16"
)

const (
	// packetStatusFinalMessage packet status value used to indicate the message
	// does not contain more chunks.
	packetStatusFinalMessage = 0x01
	// packetStatusNotFinalMessage packet status value used to indicate the
	// message is not the final one.  The documentation doesn't mention a proper
	// value. Here we're using the same status used by Azure Data Studio (0x04).
	packetStatusNotFinalMessage = 0x04
	// packetTypeSQLBatch is the packet type for SQL Batch.
	packetTypeSQLBatch = 0x01
	// PacketTypeSQLBatch is the packet type for RPC Call.
	packetTypeRPCCall = 0x03
	// procIDExecuteSQL is the RPC ID for Sp_ExecuteSQL.
	procIDExecuteSQL = 10
	// nvarcharType is the flag that represents the type NVARCHAR.
	nvarcharType = 0xef
	// ntextType is the flag that represents the type NTEXT.
	ntextType = 0x63
	// intnType is the flag that represents the type INTN.
	intnType = 0x26
	// intnTinyType is the flag that indicates the integer type tiny int.
	intnTinyType = 0x01
	// statusFlags consists 3 flag bits + 5 reserved bits.
	statusFlags = 0x00
)

var (
	// PreLogin is an example Pre-Login request packet from the protocol spec:
	//
	// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-tds/9420b4a3-eb9f-4f5e-90bd-3160444aa5a7
	PreLogin = []byte{
		0x12, 0x01, 0x00, 0x2F, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x1A, 0x00, 0x06, 0x01, 0x00, 0x20,
		0x00, 0x01, 0x02, 0x00, 0x21, 0x00, 0x01, 0x03, 0x00, 0x22, 0x00, 0x04, 0x04, 0x00, 0x26, 0x00,
		0x01, 0xFF, 0x09, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0xB8, 0x0D, 0x00, 0x00, 0x01,
	}

	// Login7 is an example Login7 request packet from the protocol spec:
	//
	// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-tds/ce5ad23f-6bf8-4fa5-9426-6b0d36e14da2
	Login7 = []byte{
		0x10, 0x01, 0x00, 0x90, 0x00, 0x00, 0x01, 0x00, 0x88, 0x00, 0x00, 0x00, 0x02, 0x00, 0x09, 0x72,
		0x00, 0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x07, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xE0, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x09, 0x04, 0x00, 0x00, 0x5E, 0x00, 0x08, 0x00,
		0x6E, 0x00, 0x02, 0x00, 0x72, 0x00, 0x00, 0x00, 0x72, 0x00, 0x07, 0x00, 0x80, 0x00, 0x00, 0x00,
		0x80, 0x00, 0x00, 0x00, 0x80, 0x00, 0x04, 0x00, 0x88, 0x00, 0x00, 0x00, 0x88, 0x00, 0x00, 0x00,
		0x00, 0x50, 0x8B, 0xE2, 0xB7, 0x8F, 0x88, 0x00, 0x00, 0x00, 0x88, 0x00, 0x00, 0x00, 0x88, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x73, 0x00, 0x6B, 0x00, 0x6F, 0x00, 0x73, 0x00, 0x74, 0x00,
		0x6F, 0x00, 0x76, 0x00, 0x31, 0x00, 0x73, 0x00, 0x61, 0x00, 0x4F, 0x00, 0x53, 0x00, 0x51, 0x00,
		0x4C, 0x00, 0x2D, 0x00, 0x33, 0x00, 0x32, 0x00, 0x4F, 0x00, 0x44, 0x00, 0x42, 0x00, 0x43, 0x00,
	}

	// MalformedPacketTest is an RPC Request malformed packet.
	MalformedPacketTest = []byte{
		0x03, 0x01, 0x00, 0x90, 0x00, 0x00, 0x02, 0x00, 0x72, 0x00, 0x61, 0x00, 0x6d, 0x00, 0x5f, 0x00,
		0x31, 0x00, 0x20, 0x00, 0x6e, 0x00, 0x76, 0x00, 0x61, 0x00, 0x72, 0x00, 0x63, 0x00, 0x68, 0x00,
		0x61, 0x00, 0x72, 0x00, 0x28, 0x00, 0x34, 0x00, 0x30, 0x00, 0x30, 0x00, 0x30, 0x00, 0x29, 0x00,
		0x0b, 0x40, 0x00, 0x5f, 0x00, 0x6d, 0x00, 0x73, 0x00, 0x70, 0x00, 0x61, 0x00, 0x72, 0x00, 0x61,
		0x00, 0x6d, 0x00, 0x5f, 0x00, 0x30, 0x00, 0x00, 0xe7, 0x40, 0x1f, 0x09, 0x04, 0xd0, 0x00, 0x34,
		0x16, 0x00, 0x73, 0x00, 0x70, 0x00, 0x74, 0x00, 0x5f, 0x00, 0x6d, 0x00, 0x6f, 0x00, 0x6e, 0x00,
		0x69, 0x00, 0x74, 0x00, 0x6f, 0x00, 0x72, 0x00, 0x0b, 0x40, 0x00, 0x5f, 0x00, 0x6d, 0x00, 0x73,
		0x00, 0x70, 0x00, 0x61, 0x00, 0x72, 0x00, 0x61, 0x00, 0x6d, 0x00, 0x5f, 0x00, 0x31, 0x00, 0x00,
		0xe7, 0x40, 0x1f, 0x09, 0x04, 0xd0, 0x00, 0x34, 0x06, 0x00, 0x64, 0x00, 0x62, 0x00, 0x6f, 0x00,
	}

	// AllHeadersSliceWithTransactionDescriptor is a ALL_HEADERS data stream
	// header containing the TransactionDescriptor data. It is required for
	// SQLBatch and RPC packets.
	//
	// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-tds/e17e54ae-0fac-48b7-b8a8-c267be297923
	AllHeadersSliceWithTransactionDescriptor = []byte{
		0x16, 0x00, 0x00, 0x00, // Total length
		0x12, 0x00, 0x00, 0x00, // Header length
		0x02, 0x00, // Header type: Transaction descriptor
		// BEGIN Transaction description: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-tds/4257dd95-ef6c-4621-b75d-270738487d68
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // TransactionDescriptor
		0x00, 0x00, 0x00, 0x00, // OutstandingRequestCount
		// End transaction description.
	}

	// FieldCollation defintion for data parameters. Using "raw collation" is ok
	// for testing because the server is not processing it.
	//
	// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-tds/3d29e8dc-218a-42c6-9ba4-947ebca9fd7e
	FieldCollation = []byte{0x00, 0x00, 0x00, 0x00, 0x00}
)

// RPCClientVariableLength returns a RPCCLientRequest packet containing a
// partially Length-prefixed Bytes request, as described here: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-tds/3f983fde-0509-485a-8c40-a9fa6679a828
func RPCClientPartiallyLength(procName string, length uint64, chunks uint64) []byte {
	params := []byte{
		0x00,         // Parameter name (B_VARCHAR)
		statusFlags,  // Status flags
		nvarcharType, // NVARCHARYTYPE
		0xff, 0xff,   // NULL length (this indicates it is a PLP parameter)
	}
	params = append(params, FieldCollation...)

	// Since we're not encoding the string into UC2, here we force it to have a
	// valid size.
	if length%2 != 0 {
		length += 1
	}

	// PLP_BODY length is ULONGLONGLEN (64-bit).
	params = binary.LittleEndian.AppendUint64(params, length)

	if length > 0 && chunks > 1 {
		chunkSize := length / chunks
		rem := length
		for i := uint64(0); i < chunks-1; i++ {
			// PLP_CHUNK length size is ULONGLEN (32-bit).
			params = binary.LittleEndian.AppendUint32(params, uint32(chunkSize))
			data := make([]byte, chunkSize)
			params = append(params, data...)
			rem -= chunkSize
		}

		// Last chunk will contain the remaining data.
		params = binary.LittleEndian.AppendUint32(params, uint32(rem))
		data := make([]byte, rem)
		params = append(params, data...)
	}

	// PLP_TERMINATOR
	params = append(params, []byte{0x00, 0x00, 0x00, 0x00}...)
	return generateRPCCallPacket(packetStatusFinalMessage, true, 1, rpcProcName(procName), params)
}

// GeneratePacketHeader generates a packet header based on the specified parameters.
func GeneratePacketHeader(packetType byte, packetStatus byte, length int, seq int) []byte {
	header := []byte{
		packetType,   // Type: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-tds/9b4a463c-2634-4a4b-ac35-bebfff2fb0f7
		packetStatus, // Status: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-tds/ce398f9a-7d47-4ede-8f36-9dd6fc21ca43
		0x00, 0x00,   // Packet length (placeholder). https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-tds/c1cddd03-b448-470a-946a-9b1b908f27a7
		0x00, 0x00, // Sever process ID (SPID). This is only sent by the server. https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-tds/fcfc00d0-6df1-42c8-8d34-93007b9a80f0
		byte(seq), // PacketID (currently ignored). https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-tds/ec9e8663-191c-4dd1-baa8-48bbfba5ed7e
		0x00,      // Window (currently ignored). https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-tds/fbc2f523-92e6-4316-aaed-3a6966e548ad
	}
	binary.BigEndian.PutUint16(header[2:], uint16(length+len(header)))
	return header
}

// GenerateBatchQueryPacket generates a final SQLBatch with the provided query.
func GenerateBatchQueryPacket(query string) []byte {
	return generateBatchQueryPacket(0x01, 1, true, query)
}

func generateBatchQueryPacket(packetStatus byte, seq int, withAllHeaders bool, query string) []byte {
	var packet []byte
	if withAllHeaders {
		packet = append(packet, AllHeadersSliceWithTransactionDescriptor...)
	}
	packet = append(packet, encodeString(query)...)
	return append(GeneratePacketHeader(packetTypeSQLBatch, packetStatus, len(packet), seq), packet...)
}

// GenerateBatchQueryChunkedPacket split a batch query into multiple network packets.
func GenerateBatchQueryChunkedPacket(chunks int, query string) [][]byte {
	queryLen := len(query)
	chunkSize := queryLen / chunks

	packets := [][]byte{generateBatchQueryPacket(0x04, 1, true, query[0:chunkSize])}
	for i := 1; i < chunks-1; i++ {
		// Sequence packets must not have the all headers information.
		packets = append(packets, generateBatchQueryPacket(packetStatusNotFinalMessage, i+2, false, query[chunkSize*i:chunkSize*(i+1)]))
	}

	// Last packet must indicate the final message.
	return append(packets, generateBatchQueryPacket(packetStatusFinalMessage, chunks, false, query[chunkSize*(chunks-1):]))
}

// GenerateExecuteSQLRPCChunkedPacket slipt a RPC Call into multiple network
// packets.
func GenerateExecuteSQLRPCChunkedPacket(chunks int, query string) [][]byte {
	rpcProcName := rpcProcID(procIDExecuteSQL)
	queryLen := len(query)
	chunkSize := queryLen / chunks

	packets := [][]byte{generateRPCCallPacket(0x04, true, 1, rpcProcName, generateNVARCHARParam(query[0:chunkSize], len(encodeString(query))))}
	for i := 1; i < chunks-1; i++ {
		packetData := encodeString(query[chunkSize*i : chunkSize*(i+1)])
		packets = append(packets, append(GeneratePacketHeader(0x03, packetStatusNotFinalMessage, len(packetData), i+1), packetData...))
	}

	// Last packet must indicate the final message.
	packetData := encodeString(query[chunkSize*(chunks-1):])
	return append(packets, append(GeneratePacketHeader(packetTypeRPCCall, packetStatusFinalMessage, len(packetData), chunks), packetData...))
}

// GenerateCustomRPCCallPacket generates a packet containing a custom RPC call
// with an empty integer parameter.
func GenerateCustomRPCCallPacket(procName string) []byte {
	params := []byte{
		0x00,         // Parameter name (B_VARCHAR). Here we're passing a length 0 (BYTELEN) which means empty name.
		statusFlags,  // Status flags.
		intnType,     // Parameter type. INTNTYPE
		intnTinyType, // Integer type.
		0x00,         // Actual data length. Providing length 0 zero we don't need to encode the integer.
	}

	return generateRPCCallPacket(packetStatusFinalMessage, true, 1, rpcProcName(procName), params)
}

// generateRPCCallPacket generates a RPC call packet.
func generateRPCCallPacket(packetStatus byte, withAllHeaders bool, seq int, rpcProcName []byte, params []byte) []byte {
	var packet []byte
	if withAllHeaders {
		packet = append(packet, AllHeadersSliceWithTransactionDescriptor...)
	}
	// Proc name
	packet = append(packet, rpcProcName...)
	// Option flags: 3 flag bits + 13 reserved bits.
	packet = append(packet, []byte{0x00, 0x00}...)
	// Parameters
	packet = append(packet, params...)
	return append(GeneratePacketHeader(packetTypeRPCCall, packetStatus, len(packet), seq), packet...)
}

// GenerateExecuteSQLRPCPacket generates a RPC call packet containing a
// single parameter (NVARCHARTYPE).
func GenerateExecuteSQLRPCPacket(query string) []byte {
	return generateRPCCallPacket(packetStatusFinalMessage, true, 1, rpcProcID(procIDExecuteSQL), generateNVARCHARParam(query, 0))
}

// GenerateExecuteSQLRPCPacketNTEXT generates a RPC call packet containing a
// single parameter (NTEXT).
func GenerateExecuteSQLRPCPacketNTEXT(query string) []byte {
	return generateRPCCallPacket(packetStatusFinalMessage, true, 1, rpcProcID(procIDExecuteSQL), generateNTEXTParam(query))
}

// generateNVARCHARParam generates a NVARCHARTYPE parameter.
func generateNVARCHARParam(contents string, totalLength int) []byte {
	encodedContents := encodeString(contents)
	length := len(encodedContents)
	if totalLength > 0 {
		length = totalLength
	}

	// Parameter length (USHORTLEN_TYPE for NVARCHARTYPE).
	encodedLength := binary.LittleEndian.AppendUint16([]byte{}, uint16(length))
	packet := []byte{
		0x00,                               // Parameter name (B_VARCHAR). Here we're passing a length 0 (BYTELEN) which means empty name.
		statusFlags,                        // Status flags.
		nvarcharType,                       // Parameter type: NVARCHARTYPE
		encodedLength[0], encodedLength[1], // Param length
	}
	// Data collation flags.
	packet = append(packet, FieldCollation...)
	// Param data also has the parameter length (same encoding).
	packet = append(packet, encodedLength...)
	return append(packet, encodedContents...)
}

// generateNTEXTParam generates a NTEXT parameter.
//
// The parameter format is based on the official documentataion and compared
// with requests generated by Azure Data Studio.
func generateNTEXTParam(contents string) []byte {
	encodedContents := encodeString(contents)
	// Parameter length (LONGLEN_TYPE for NTEXT).
	encodedLength := binary.LittleEndian.AppendUint32([]byte{}, uint32(len(encodedContents)))
	packet := append([]byte{
		0x00,        // Parameter name (B_VARCHAR). Here we're passing a length 0 (BYTELEN) which means empty name.
		statusFlags, // Status flags.
		ntextType,   // Parameter type: NTEXT
	}, encodedLength...)
	// Data collation flags.
	packet = append(packet, FieldCollation...)
	// Param data also has the parameter length (same encoding).
	packet = append(packet, encodedLength...)
	return append(packet, encodedContents...)
}

// rpcProcName returns PROC NAME field used on RPC calls.
func rpcProcName(name string) []byte {
	var packet []byte
	packet = binary.LittleEndian.AppendUint16(packet, uint16(len(name)))
	return append(packet, encodeString(name)...)
}

// rpcProcID returns the PROC ID field used on RPC calls.
func rpcProcID(id uint16) []byte {
	packet := []byte{0xff, 0xff}
	return binary.LittleEndian.AppendUint16(packet, id)
}

// encodeString encodes the string into UTF-16 LittleEndian.
func encodeString(s string) []byte {
	var encodedString []byte
	for _, r := range utf16.Encode([]rune(s)) {
		encodedString = binary.LittleEndian.AppendUint16(encodedString, r)
	}

	return encodedString
}
