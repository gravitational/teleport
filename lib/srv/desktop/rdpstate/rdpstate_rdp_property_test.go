//go:build desktop_access_rdp || rust_rdp_decoder

/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/srv/desktop/rdpstate/rdpstatetest"
	"github.com/gravitational/teleport/lib/utils/testutils"
)

func TestProperty_RDPState_ServerHelloMatchesDimensions(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		w := genScreenDim(t, "w")
		h := genScreenDim(t, "h")

		s := New()
		defer s.Release()

		evt, err := rdpstatetest.EncodeTDPBServerHello(w, h)
		require.NoError(t, err)
		require.NoError(t, s.HandleMessage(evt))

		img := s.Image()
		require.NotNil(t, img, "decoder should be created for w=%d h=%d", w, h)
		require.Equal(t, int(w), img.Bounds().Dx(), "framebuffer width mismatch")
		require.Equal(t, int(h), img.Bounds().Dy(), "framebuffer height mismatch")
	})
}

func TestRDPState_ServerHelloAtMaxDimension(t *testing.T) {
	cases := []struct {
		name string
		w, h uint32
	}{
		{"max width", types.MaxRDPScreenWidth, 16},
		{"max height", 16, types.MaxRDPScreenHeight},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := New()
			defer s.Release()

			evt, err := rdpstatetest.EncodeTDPBServerHello(tc.w, tc.h)
			require.NoError(t, err)
			require.NoError(t, s.HandleMessage(evt))

			img := s.Image()
			require.NotNil(t, img)
			require.Equal(t, int(tc.w), img.Bounds().Dx())
			require.Equal(t, int(tc.h), img.Bounds().Dy())
		})
	}
}

func TestProperty_RDPState_BitmapPDUNeverPanics(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := New()
		defer s.Release()

		evt, err := rdpstatetest.EncodeTDPBServerHello(100, 100)
		require.NoError(t, err)
		require.NoError(t, s.HandleMessage(evt))

		left := rapid.IntRange(0, 96).Draw(t, "left")
		top := rapid.IntRange(0, 99).Draw(t, "top")
		w := rapid.IntRange(4, 100-left).Draw(t, "w")
		h := rapid.IntRange(1, 100-top).Draw(t, "h")

		colorIdx := rapid.IntRange(0, 2).Draw(t, "color")
		colors := []uint16{rdpstatetest.RGB565White, rdpstatetest.RGB565Red, rdpstatetest.RGB565Blue}

		testutils.RunWithTimeout(t, 500*time.Millisecond, func() {
			pdu := rdpstatetest.BuildBitmapPDU(left, top, w, h, colors[colorIdx])
			pduEvt, err := rdpstatetest.EncodeTDPBFastPathPDU(pdu)
			require.NoError(t, err)

			_ = s.HandleMessage(pduEvt)
			_ = s.Image()
		})
	})
}

func TestProperty_RDPState_PointerPositionUpdatesCursor(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := New()
		defer s.Release()

		require.NoError(t, s.HandleMessage(mustServerHello(t, 200, 200)))
		require.NoError(t, s.HandleMessage(mustFastPath(t,
			rdpstatetest.BuildNewPointerPDU(2, 2, 0, 0, rdpstatetest.BGRARed))))

		x := rapid.IntRange(0, 199).Draw(t, "x")
		y := rapid.IntRange(0, 199).Draw(t, "y")
		require.NoError(t, s.HandleMessage(mustFastPath(t,
			rdpstatetest.BuildPointerPositionPDU(x, y))))

		cs := s.CursorState()
		require.Equal(t, uint16(x), cs.X)
		require.Equal(t, uint16(y), cs.Y)
		require.True(t, cs.Visible, "cursor should be visible after Position update")
	})
}

func TestProperty_RDPState_PointerHiddenThenDefaultRestoresVisibility(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := New()
		defer s.Release()

		require.NoError(t, s.HandleMessage(mustServerHello(t, 100, 100)))
		require.NoError(t, s.HandleMessage(mustFastPath(t,
			rdpstatetest.BuildNewPointerPDU(2, 2, 0, 0, rdpstatetest.BGRARed))))

		actions := rapid.SliceOfN(rapid.Bool(), 1, 8).Draw(t, "toggles")
		for _, hide := range actions {
			if hide {
				require.NoError(t, s.HandleMessage(mustFastPath(t, rdpstatetest.BuildPointerHiddenPDU())))
			} else {
				require.NoError(t, s.HandleMessage(mustFastPath(t, rdpstatetest.BuildPointerDefaultPDU())))
			}
		}

		expected := !actions[len(actions)-1] // hide=true -> invisible; hide=false -> visible
		require.Equal(t, expected, s.CursorState().Visible,
			"final visibility mismatch after actions=%v", actions)
	})
}

func TestProperty_RDPState_RandomPDUNeverPanicsThroughRustDecoder(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := New()
		defer s.Release()
		require.NoError(t, s.HandleMessage(mustServerHello(t, 200, 200)))

		pdu := rapid.SliceOfN(rapid.Byte(), 0, 2048).Draw(t, "pdu_bytes")

		testutils.RunWithTimeout(t, 500*time.Millisecond, func() {
			evt, err := rdpstatetest.EncodeTDPBFastPathPDU(pdu)
			require.NoError(t, err)
			_ = s.HandleMessage(evt)

			_ = s.CursorState()
			_ = s.Image()
			_ = s.UpdatedRegions()
		})
	})
}

func TestProperty_RDPState_RandomFastPathUpdateNeverPanics(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := New()
		defer s.Release()
		require.NoError(t, s.HandleMessage(mustServerHello(t, 200, 200)))

		updateCode := byte(rapid.IntRange(0, 255).Draw(t, "update_code"))
		payload := rapid.SliceOfN(rapid.Byte(), 0, 1024).Draw(t, "payload")
		pdu := rdpstatetest.WrapFastPathUpdate(updateCode, payload)

		testutils.RunWithTimeout(t, 500*time.Millisecond, func() {
			evt, err := rdpstatetest.EncodeTDPBFastPathPDU(pdu)
			require.NoError(t, err)
			_ = s.HandleMessage(evt)
		})
	})
}

func TestProperty_RDPState_TruncatedThenValidPDURecovers(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := New()
		defer s.Release()
		require.NoError(t, s.HandleMessage(mustServerHello(t, 200, 200)))

		full := rdpstatetest.BuildBitmapPDU(10, 10, 8, 8, rdpstatetest.RGB565White)
		cut := rapid.IntRange(0, len(full)).Draw(t, "cut")
		truncated := full[:cut]

		testutils.RunWithTimeout(t, 1*time.Second, func() {
			_ = s.HandleMessage(mustFastPath(t, truncated))
			_ = s.HandleMessage(mustFastPath(t, full))
		})

		img := s.Image()
		require.NotNil(t, img)

		r, g, b, a := img.At(10, 10).RGBA()
		require.Equal(t, uint32(0xFFFF), r, "valid frame after a truncated one must still paint")
		require.Equal(t, uint32(0xFFFF), g)
		require.Equal(t, uint32(0xFFFF), b)
		require.Equal(t, uint32(0xFFFF), a)
	})
}

func TestProperty_RDPState_MixedValidGarbageSequenceNeverPanics(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := New()
		defer s.Release()

		require.NoError(t, s.HandleMessage(mustServerHello(t, 300, 300)))

		count := rapid.IntRange(0, 15).Draw(t, "count")
		events := make([]*apievents.DesktopRecording, 0, count)
		for i := 0; i < count; i++ {
			kind := rapid.IntRange(0, 3).Draw(t, "kind")
			var pdu []byte
			switch kind {
			case 0:
				// Real bitmap. width >= 4 so it decodes; IronRDP bounds-checks narrower widths.
				l := rapid.IntRange(0, 290).Draw(t, "l")
				t2 := rapid.IntRange(0, 290).Draw(t, "t")
				w := rapid.IntRange(4, 300-l).Draw(t, "w")
				h := rapid.IntRange(1, 300-t2).Draw(t, "h")
				pdu = rdpstatetest.BuildBitmapPDU(l, t2, w, h, rdpstatetest.RGB565White)

			case 1:
				// Real pointer position.
				pdu = rdpstatetest.BuildPointerPositionPDU(
					rapid.IntRange(0, 299).Draw(t, "px"),
					rapid.IntRange(0, 299).Draw(t, "py"),
				)

			case 2:
				// Garbage with valid framing.
				payload := rapid.SliceOfN(rapid.Byte(), 0, 256).Draw(t, "garbage")
				pdu = rdpstatetest.WrapFastPathUpdate(0x99, payload)

			case 3:
				// Pure random bytes (no valid framing).
				pdu = rapid.SliceOfN(rapid.Byte(), 0, 256).Draw(t, "raw")
			}

			evt, err := rdpstatetest.EncodeTDPBFastPathPDU(pdu)
			require.NoError(t, err)
			events = append(events, evt)
		}

		testutils.RunWithTimeout(t, 2*time.Second, func() {
			for _, evt := range events {
				_ = s.HandleMessage(evt)
			}

			_ = s.CursorState()
			_ = s.Image()
		})
	})
}

func mustServerHello(t *rapid.T, w, h uint32) *apievents.DesktopRecording {
	t.Helper()

	evt, err := rdpstatetest.EncodeTDPBServerHello(w, h)
	require.NoError(t, err)

	return evt
}

func mustFastPath(t *rapid.T, pdu []byte) *apievents.DesktopRecording {
	t.Helper()

	evt, err := rdpstatetest.EncodeTDPBFastPathPDU(pdu)
	require.NoError(t, err)

	return evt
}

func genScreenDim(t *rapid.T, label string) uint32 {
	t.Helper()

	return uint32(rapid.OneOf(
		rapid.Just(1),
		rapid.Just(2),
		rapid.IntRange(64, 1920),
	).Draw(t, label))
}
