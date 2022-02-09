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

import "bytes"

// StatementPreparePacket represents the COM_STMT_PREPARE command.
//
// https://dev.mysql.com/doc/internals/en/com-stmt-prepare.html
type StatementPreparePacket struct {
	packet

	// query is the query to prepare.
	query string
}

// Query returns the query text.
func (p *StatementPreparePacket) Query() string {
	return p.query
}

// statementIDPacket represents a common statement packet where statement ID is
// right after packet type.
type statementIDPacket struct {
	packet

	statementID uint32
}

// StatementID returns the statement ID.
func (p *statementIDPacket) StatementID() uint32 {
	return p.statementID
}

// StatementSendLongDataPacket represents the COM_STMT_SEND_LONG_DATA command.
//
// https://dev.mysql.com/doc/internals/en/com-stmt-send-long-data.html
type StatementSendLongDataPacket struct {
	statementIDPacket

	paramID uint16
	data    string
}

// ParamID returns the parameter ID.
func (p *StatementSendLongDataPacket) ParamID() uint16 {
	return p.paramID
}

// Data returns the data in string.
func (p *StatementSendLongDataPacket) Data() string {
	return p.data
}

// StatementExecutePacket represents the COM_STMT_EXECUTE command.
//
// https://dev.mysql.com/doc/internals/en/com-stmt-execute.html
type StatementExecutePacket struct {
	statementIDPacket

	iterations        uint32
	paramsStartingPos int
}

// Parameters returns a slice of formatted parameters
func (p *StatementExecutePacket) Parameters(n int) []string {
	// TODO(greedy52) number of parameters is required in order to parse
	// paremeters out of the packet. Number of parameters can be obtained from
	// the response of COM_STMT_PREPARE.
	return nil
}

// StatementClosePacket represents the COM_STMT_CLOSE command.
//
// https://dev.mysql.com/doc/internals/en/com-stmt-close.html
type StatementClosePacket struct {
	statementIDPacket
}

// StatementResetPacket represents the COM_STMT_RESET command.
//
// https://dev.mysql.com/doc/internals/en/com-stmt-reset.html
type StatementResetPacket struct {
	statementIDPacket
}

// parseStatementPreparePacket parses packet bytes and returns a Packet if
// successful.
func parseStatementPreparePacket(rawPacket packet) (Packet, bool) {
	unread, ok := skipHeaderAndType(rawPacket.bytes)
	if !ok {
		return nil, false
	}

	return &StatementPreparePacket{
		packet: rawPacket,
		query:  readString(unread),
	}, true
}

// parseStatementIDPacket parses packet bytes and returns a statementIDPacket.
func parseStatementIDPacket(rawPacket packet) (statementIDPacket, []byte, bool) {
	unread, ok := skipHeaderAndType(rawPacket.bytes)
	if !ok {
		return statementIDPacket{}, nil, false
	}

	unread, statementID, ok := readUint32(unread)
	if !ok {
		return statementIDPacket{}, nil, false
	}

	return statementIDPacket{
		packet:      rawPacket,
		statementID: statementID,
	}, unread, true
}

// parseStatementSendLongDataPacket parses packet bytes and returns a Packet if
// successful.
func parseStatementSendLongDataPacket(rawPacket packet) (Packet, bool) {
	parent, unread, ok := parseStatementIDPacket(rawPacket)
	if !ok {
		return nil, false
	}

	unread, paramID, ok := readUint16(unread)
	if !ok {
		return nil, false
	}

	return &StatementSendLongDataPacket{
		statementIDPacket: parent,
		paramID:           paramID,
		data:              readString(unread),
	}, true
}

// parseStatementExecutePacket parses packet bytes and returns a Packet if
// successful.
func parseStatementExecutePacket(rawPacket packet) (Packet, bool) {
	parent, unread, ok := parseStatementIDPacket(rawPacket)
	if !ok {
		return nil, false
	}

	// Skip cursor flag.
	if unread, ok = skipBytes(unread, 1); !ok {
		return nil, false
	}

	unread, iterations, ok := readUint32(unread)
	if !ok {
		return nil, false
	}

	// Read through null bitmap and find new-params-bound flag. If bound flag
	// is 0, there is no more data after. If bound flag is 1, rest of the bytes
	// are parameters.
	paramsStartingPos := 0
	flagPos := bytes.IndexByte(unread, 0x01)
	if flagPos > 0 {
		paramsStartingPos = len(rawPacket.bytes) - len(unread[flagPos:]) + 1
	}

	return &StatementExecutePacket{
		statementIDPacket: parent,
		iterations:        iterations,
		paramsStartingPos: paramsStartingPos,
	}, true
}

// parseStatementClosePacket parses packet bytes and returns a Packet if
// successful.
func parseStatementClosePacket(rawPacket packet) (Packet, bool) {
	parent, _, ok := parseStatementIDPacket(rawPacket)
	if !ok {
		return nil, false
	}
	return &StatementClosePacket{
		statementIDPacket: parent,
	}, true
}

// parseStatementResetPacket parses packet bytes and returns a Packet if
// successful.
func parseStatementResetPacket(rawPacket packet) (Packet, bool) {
	parent, _, ok := parseStatementIDPacket(rawPacket)
	if !ok {
		return nil, false
	}
	return &StatementResetPacket{
		statementIDPacket: parent,
	}, true
}
