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

import "encoding/binary"

func skipHeaderAndType(input []byte) (unread []byte, ok bool) {
	return skipBytes(input, headerAndTypeSize)
}

func skipBytes(input []byte, n int) (unread []byte, ok bool) {
	if len(input) < n {
		return nil, false
	}
	return input[n:], true
}

func readUint32(input []byte) (unread []byte, read uint32, ok bool) {
	if len(input) < 4 {
		return nil, 0, false
	}
	return input[4:], binary.LittleEndian.Uint32(input[:4]), true
}

func readUint16(input []byte) (unread []byte, read uint16, ok bool) {
	if len(input) < 2 {
		return nil, 0, false
	}
	return input[2:], binary.LittleEndian.Uint16(input[:2]), true
}

func readString(input []byte) string {
	return string(input)
}
