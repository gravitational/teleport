/*
Copyright 2023 Gravitational, Inc.

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

// Type defines the TNS Oracle Protocol types.
// https://github.com/oracle/python-oracledb/blob/main/src/oracledb/impl/thin/constants.pxi#L33
type Type uint8

const (
	CONNECT  Type = 1
	ACCEPT   Type = 2
	REFUSE   Type = 4
	REDIRECT Type = 5
	DATA     Type = 6
	RESEND   Type = 11
)

const (
	// PacketHeaderSize is the size in bytes of Oracle header.
	PacketHeaderSize = 8
	// TNSVersionMinLargeSdu is the 12.1 Oracle Server version.
	// https://github.com/oracle/python-oracledb/blob/main/src/oracledb/impl/thin/constants.pxi#L526
	TNSVersionMinLargeSdu = 315
)

const (
	defaultReaderCapacity = 32 * 1024
)
