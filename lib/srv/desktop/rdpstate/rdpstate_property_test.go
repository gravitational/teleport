/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

package rdpstate

import (
	"encoding/binary"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/utils/testutils"
)

func TestProperty_RDPState_NeverPanicsOnRandomLegacyMessage(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		data := genLegacyMessage(t)
		testutils.RunWithTimeout(t, 1*time.Second, func() {
			s := New()
			defer s.Release()

			_ = s.HandleMessage(&apievents.DesktopRecording{Message: data})
		})
	})
}

func TestProperty_RDPState_NeverPanicsOnRandomTDPBMessage(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		data := rapid.SliceOfN(rapid.Byte(), 0, 4096).Draw(t, "data")
		testutils.RunWithTimeout(t, 1*time.Second, func() {
			s := New()
			defer s.Release()

			_ = s.HandleMessage(&apievents.DesktopRecording{TDPBMessage: data})
		})
	})
}

func TestProperty_RDPState_LegacyFastPathLengthGuard(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		declared := rapid.Uint32Range(maxMessageLength, ^uint32(0)).Draw(t, "declared_len")
		buf := make([]byte, 1+4)
		buf[0] = legacyTypeRDPFastPathPDU
		binary.BigEndian.PutUint32(buf[1:], declared)

		s := New()
		defer s.Release()

		err := s.HandleMessage(&apievents.DesktopRecording{Message: buf})
		require.Error(t, err, "expected error for declared length %d", declared)
	})
}

func TestProperty_RDPState_TDPBLengthGuard(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		declared := rapid.Uint32Range(maxMessageLength, ^uint32(0)).Draw(t, "declared_len")
		buf := make([]byte, tdpbHeaderLength)
		binary.BigEndian.PutUint32(buf, declared)

		s := New()
		defer s.Release()

		err := s.HandleMessage(&apievents.DesktopRecording{TDPBMessage: buf})
		require.Error(t, err, "expected error for declared length %d", declared)
	})
}

func TestProperty_RDPState_AfterReleaseNeverPanics(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := New()
		s.Release()

		// Double-release and read-after-release must not panic.
		testutils.RunWithTimeout(t, 1*time.Second, func() {
			s.Release()
			_ = s.CursorState()
			_ = s.Image()
			_ = s.UpdatedRegions()

			s.ResetUpdatedRegions()
			img, cs := s.ImageWithCursor()
			_ = img
			_ = cs
		})
	})
}

func TestProperty_RDPState_FastPathBeforeServerHelloRejected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		payloadLen := rapid.IntRange(0, 128).Draw(t, "payload_len")
		payload := rapid.SliceOfN(rapid.Byte(), payloadLen, payloadLen).Draw(t, "payload")
		buf := make([]byte, 1+4+payloadLen)
		buf[0] = legacyTypeRDPFastPathPDU
		binary.BigEndian.PutUint32(buf[1:5], uint32(payloadLen))
		copy(buf[5:], payload)

		s := New()
		defer s.Release()

		err := s.HandleMessage(&apievents.DesktopRecording{Message: buf})
		if payloadLen > 0 {
			require.Error(t, err, "expected error for FastPathPDU before ServerHello")
		}
	})
}

func TestProperty_RDPState_RandomEventSequenceNeverPanics(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		count := rapid.IntRange(0, 20).Draw(t, "count")
		events := make([]*apievents.DesktopRecording, count)
		for i := range events {
			useTDPB := rapid.Bool().Draw(t, "tdpb")
			data := rapid.SliceOfN(rapid.Byte(), 0, 256).Draw(t, "data")
			evt := &apievents.DesktopRecording{}
			if useTDPB {
				evt.TDPBMessage = data
			} else {
				boundLegacyScreenDims(t, data)
				evt.Message = data
			}

			events[i] = evt
		}

		testutils.RunWithTimeout(t, 1*time.Second, func() {
			s := New()
			defer s.Release()

			for _, evt := range events {
				_ = s.HandleMessage(evt)
				_ = s.CursorState()
				_ = s.Image()
				_ = s.UpdatedRegions()
			}
		})
	})
}

func TestProperty_RDPState_OversizedServerHelloRejected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		w := rapid.IntRange(1, math.MaxUint16).Draw(t, "w")
		h := rapid.IntRange(1, math.MaxUint16).Draw(t, "h")
		if rapid.Bool().Draw(t, "wide") {
			w = rapid.IntRange(int(types.MaxRDPScreenWidth)+1, math.MaxUint16).Draw(t, "w_over")
		} else {
			h = rapid.IntRange(int(types.MaxRDPScreenHeight)+1, math.MaxUint16).Draw(t, "h_over")
		}

		buf := make([]byte, 1+8)
		buf[0] = legacyTypeConnectionActivated
		binary.BigEndian.PutUint16(buf[5:7], uint16(w))
		binary.BigEndian.PutUint16(buf[7:9], uint16(h))

		s := New()
		defer s.Release()

		require.Error(t, s.HandleMessage(&apievents.DesktopRecording{Message: buf}),
			"expected rejection for %dx%d", w, h)
	})
}

func genLegacyMessage(t *rapid.T) []byte {
	body := rapid.SliceOfN(rapid.Byte(), 0, 4096).Draw(t, "body")
	first := rapid.OneOf(
		rapid.Just(byte(legacyTypeConnectionActivated)),
		rapid.Just(byte(legacyTypeRDPFastPathPDU)),
		rapid.Just(byte(legacyTypeMouseMove)),
		rapid.Byte(),
	).Draw(t, "first")

	msg := append([]byte{first}, body...)
	boundLegacyScreenDims(t, msg)

	return msg
}

// maxFuzzScreenDim bounds fuzzed screen dimensions to avoid huge framebuffer allocations.
const maxFuzzScreenDim = 512

// boundLegacyScreenDims bounds the dimensions of a randomly-formed legacy ConnectionActivated.
func boundLegacyScreenDims(t *rapid.T, data []byte) {
	if len(data) >= 9 && data[0] == legacyTypeConnectionActivated {
		binary.BigEndian.PutUint16(data[5:7], uint16(rapid.IntRange(0, maxFuzzScreenDim).Draw(t, "screen_w")))
		binary.BigEndian.PutUint16(data[7:9], uint16(rapid.IntRange(0, maxFuzzScreenDim).Draw(t, "screen_h")))
	}
}
