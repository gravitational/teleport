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

package protocol

import "github.com/gravitational/trace"

// StatementPreparePacket represents the COM_STMT_PREPARE command.
//
// https://dev.mysql.com/doc/internals/en/com-stmt-prepare.html
type StatementPreparePacket struct {
	packet

	// query is the query to prepare.
	query string
}

// newStatementPreparePacket parses packet bytes and returns a Packet if
// successful.
func newStatementPreparePacket(packetBytes []byte) (Packet, error) {
	if len(packetBytes) < 5 {
		return nil, trace.BadParameter("failed to parse COM_STMT_PREPARE packet: %v", packetBytes)
	}

	// 4-byte packet header + 1-byte payload header, then query text.
	return &StatementPreparePacket{
		packet: packet{bytes: packetBytes},
		query:  string(packetBytes[5:]),
	}, nil
}

// Query returns the query text.
func (p *StatementPreparePacket) Query() string {
	return p.query
}

// StatementSendLongDataPacket represents the COM_STMT_SEND_LONG_DATA command.
//
// https://dev.mysql.com/doc/internals/en/com-stmt-send-long-data.html
type StatementSendLongDataPacket struct {
	packet

	statementID string
	paramID     string
	data        string
}
