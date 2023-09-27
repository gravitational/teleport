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

package x11

import (
	"bytes"
	"encoding/hex"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzParseDisplay(f *testing.F) {
	for _, tc := range parseDisplayTestCases {
		f.Add(tc.displayString)
	}

	f.Fuzz(func(t *testing.T, displayString string) {
		require.NotPanics(t, func() {
			ParseDisplay(displayString)
		})
	})
}

func makeSpoofedXAuthEntryForPacket(packet []byte) *XAuthEntry {
	result := &XAuthEntry{}
	r := bytes.NewReader(packet)

	// we need to parse some of the packet to pass validation with the spoofedXAuthEntry
	// we will return the result as soon as we run into any issue
	initBuf := make([]byte, xauthPacketInitBufSize)
	if _, err := io.ReadFull(r, initBuf); err != nil {
		return result
	}
	protoLen, dataLen, err := readXauthPacketInitBuf(initBuf)
	if err != nil {
		return result
	}
	authPacketSize := protoLen + (4-protoLen%4)%4 + dataLen
	authPacket := make([]byte, authPacketSize)
	if _, err := io.ReadFull(r, authPacket); err != nil {
		return result
	}

	result.Proto = string(authPacket[:protoLen])
	result.Cookie = hex.EncodeToString(authPacket[len(authPacket)-dataLen:])
	return result
}

func FuzzReadAndRewriteXAuthPacket(f *testing.F) {
	f.Add([]byte{0x6c, 0x0, 0x0, 0x0, 0x0, 0x0, 0x12, 0x0, 0x10, 0x0, 0x0, 0x0})
	f.Add([]byte{0x6c, 0x0, 0x0, 0x0, 0x0, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
	f.Add([]byte{0x6c, 0x0, 0x0, 0x0, 0x0, 0x0, 0x12, 0x0, 0x10, 0x0, 0x0, 0x0, 0x4d, 0x49, 0x54,
		0x2d, 0x4d, 0x41, 0x47, 0x49, 0x43, 0x2d, 0x43, 0x4f, 0x4f, 0x4b, 0x49, 0x45,
		0x2d, 0x31, 0x0, 0x0, 0xcc, 0xa2, 0xc6, 0x39, 0x19, 0xb9, 0xaa, 0xc1, 0x69, 0x73,
		0x85, 0x38, 0xc8, 0xbb, 0x52, 0x7d})
	f.Add([]byte{0x6c, 0x0, 0x0, 0x0, 0x0, 0x0, 0x12, 0x0, 0x10, 0x0, 0x0, 0x0, 0x4d, 0x49, 0x54,
		0x2d, 0x4d, 0x41, 0x47, 0x49, 0x43, 0x2d, 0x43, 0x4f, 0x4f, 0x4b, 0x49, 0x45,
		0x2d, 0x31, 0x0, 0x0, 0xa4, 0x1d, 0x8a, 0x34, 0xfe, 0xc1, 0xcf, 0x17, 0xa3, 0x83,
		0xcc, 0x74, 0x30, 0xc8, 0x95, 0xb6})

	f.Fuzz(func(t *testing.T, packet []byte) {
		require.NotPanics(t, func() {
			r := bytes.NewReader(packet)
			spoofedXAuthEntry := makeSpoofedXAuthEntryForPacket(packet)
			realXAuthEntry := spoofedXAuthEntry

			_, _ = ReadAndRewriteXAuthPacket(r, spoofedXAuthEntry, realXAuthEntry)
		})
	})
}
