/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
