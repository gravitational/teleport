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
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/srv/desktop/rdp/decoder"
	"github.com/gravitational/teleport/lib/srv/desktop/rdpstate/rdpstatetest"
)

func TestServerHello_CreatesDecoder(t *testing.T) {
	s := New()

	require.NoError(t, s.HandleMessage(encodeTDPBServerHello(t, 800, 600)))
	require.NotNil(t, s.decoder)

	img := s.Image()
	require.NotNil(t, img)

	require.Equal(t, 800, img.Bounds().Dx())
	require.Equal(t, 600, img.Bounds().Dy())
}

func TestServerHello_Resize(t *testing.T) {
	s := New()

	require.NoError(t, s.HandleMessage(encodeTDPBServerHello(t, 800, 600)))

	d := s.decoder

	require.NoError(t, s.HandleMessage(encodeTDPBServerHello(t, 1920, 1080)))

	require.Same(t, d, s.decoder)
}

func TestFastPathPDU_UpdatesImage(t *testing.T) {
	s := New()

	require.NoError(t, s.HandleMessage(encodeTDPBServerHello(t, 100, 100)))

	// Draw a 4x2 white rectangle at (10, 20).
	sendPDU(t, s, rdpstatetest.BuildBitmapPDU(10, 20, 4, 2, rdpstatetest.RGB565White))

	img := s.Image()
	require.NotNil(t, img)

	// Pixel inside the bitmap should be white.
	r, g, b, a := img.At(10, 20).RGBA()
	require.Equal(t, uint32(0xFFFF), r)
	require.Equal(t, uint32(0xFFFF), g)
	require.Equal(t, uint32(0xFFFF), b)
	require.Equal(t, uint32(0xFFFF), a)

	// Pixel outside should be untouched (black/transparent).
	r, g, b, a = img.At(99, 99).RGBA()
	require.Equal(t, uint32(0), r)
	require.Equal(t, uint32(0), g)
	require.Equal(t, uint32(0), b)
	require.Equal(t, uint32(0), a)
}

func TestFastPathPDU_MultipleBitmaps(t *testing.T) {
	s := New()

	require.NoError(t, s.HandleMessage(encodeTDPBServerHello(t, 100, 100)))

	// Red at row 0, blue at row 1.
	sendPDU(t, s, rdpstatetest.BuildBitmapPDU(0, 0, 4, 1, rdpstatetest.RGB565Red))
	sendPDU(t, s, rdpstatetest.BuildBitmapPDU(0, 1, 4, 1, rdpstatetest.RGB565Blue))

	img := s.Image()
	require.NotNil(t, img)

	// Red pixel (RGB565 0xF800 -> R>0, G=0, B=0).
	r, g, b, _ := img.At(0, 0).RGBA()
	require.NotEqual(t, uint32(0), r, "expected red channel to be non-zero")
	require.Equal(t, uint32(0), g)
	require.Equal(t, uint32(0), b)

	// Blue pixel (RGB565 0x001F -> R=0, G=0, B>0).
	r, g, b, _ = img.At(0, 1).RGBA()
	require.Equal(t, uint32(0), r)
	require.Equal(t, uint32(0), g)
	require.NotEqual(t, uint32(0), b, "expected blue channel to be non-zero")
}

func TestFastPathPDU_AfterResize(t *testing.T) {
	s := New()

	require.NoError(t, s.HandleMessage(encodeTDPBServerHello(t, 100, 100)))
	sendPDU(t, s, rdpstatetest.BuildBitmapPDU(0, 0, 4, 1, rdpstatetest.RGB565White))

	// Resize clears the framebuffer.
	require.NoError(t, s.HandleMessage(encodeTDPBServerHello(t, 200, 200)))

	img := s.Image()
	require.NotNil(t, img)
	require.Equal(t, 200, img.Bounds().Dx())
	require.Equal(t, 200, img.Bounds().Dy())

	// Old content should be gone.
	r, g, b, a := img.At(0, 0).RGBA()
	require.Equal(t, uint32(0), r)
	require.Equal(t, uint32(0), g)
	require.Equal(t, uint32(0), b)
	require.Equal(t, uint32(0), a)

	// New PDU after resize still works.
	sendPDU(t, s, rdpstatetest.BuildBitmapPDU(0, 0, 4, 1, rdpstatetest.RGB565White))
	img = s.Image()
	require.NotNil(t, img)

	r, g, b, a = img.At(0, 0).RGBA()
	require.Equal(t, uint32(0xFFFF), r)
	require.Equal(t, uint32(0xFFFF), g)
	require.Equal(t, uint32(0xFFFF), b)
	require.Equal(t, uint32(0xFFFF), a)
}

func TestLegacyTDP_ConnectionActivated(t *testing.T) {
	s := New()

	data := legacyConnectionActivated(t, 1024, 768)
	require.NoError(t, s.HandleMessage(data))
	require.NotNil(t, s.decoder)

	img := s.Image()
	require.NotNil(t, img)

	require.Equal(t, 1024, img.Bounds().Dx())
	require.Equal(t, 768, img.Bounds().Dy())
}

func TestLegacyTDP_Resize(t *testing.T) {
	s := New()

	require.NoError(t, s.HandleMessage(legacyConnectionActivated(t, 800, 600)))
	d := s.decoder

	require.NoError(t, s.HandleMessage(legacyConnectionActivated(t, 1920, 1080)))
	require.Same(t, d, s.decoder)

	img := s.Image()
	require.NotNil(t, img)
	require.Equal(t, 1920, img.Bounds().Dx())
	require.Equal(t, 1080, img.Bounds().Dy())
}

func TestCursorState_DefaultHidden(t *testing.T) {
	s := New()

	require.NoError(t, s.HandleMessage(encodeTDPBServerHello(t, 800, 600)))

	cs := s.CursorState()
	require.False(t, cs.Visible)
	require.Equal(t, uint16(0), cs.X)
	require.Equal(t, uint16(0), cs.Y)
}

func TestCursorState_BeforeServerHello(t *testing.T) {
	s := New()

	cs := s.CursorState()
	require.False(t, cs.Visible)
	require.Equal(t, uint16(0), cs.X)
	require.Equal(t, uint16(0), cs.Y)
}

func TestCursorState_PointerPosition(t *testing.T) {
	s := New()

	require.NoError(t, s.HandleMessage(encodeTDPBServerHello(t, 100, 100)))

	sendPDU(t, s, rdpstatetest.BuildNewPointerPDU(2, 2, 0, 0, rdpstatetest.BGRARed))
	sendPDU(t, s, rdpstatetest.BuildPointerPositionPDU(50, 60))

	cs := s.CursorState()
	require.Equal(t, decoder.CursorState{Visible: true, X: 50, Y: 60}, cs)
}

func TestCursorState_PointerHidden(t *testing.T) {
	s := New()

	require.NoError(t, s.HandleMessage(encodeTDPBServerHello(t, 100, 100)))

	sendPDU(t, s, rdpstatetest.BuildNewPointerPDU(2, 2, 0, 0, rdpstatetest.BGRARed))
	require.True(t, s.CursorState().Visible)

	sendPDU(t, s, rdpstatetest.BuildPointerHiddenPDU())
	require.False(t, s.CursorState().Visible)
}

func TestCursorState_PointerDefaultRestoresVisibility(t *testing.T) {
	s := New()

	require.NoError(t, s.HandleMessage(encodeTDPBServerHello(t, 100, 100)))

	sendPDU(t, s, rdpstatetest.BuildNewPointerPDU(2, 2, 0, 0, rdpstatetest.BGRARed))
	sendPDU(t, s, rdpstatetest.BuildPointerHiddenPDU())
	require.False(t, s.CursorState().Visible)

	sendPDU(t, s, rdpstatetest.BuildPointerDefaultPDU())
	require.True(t, s.CursorState().Visible)
}

func TestLegacyCursorState_PointerPosition(t *testing.T) {
	s := New()

	require.NoError(t, s.HandleMessage(legacyConnectionActivated(t, 100, 100)))

	sendLegacyPDU(t, s, rdpstatetest.BuildNewPointerPDU(2, 2, 0, 0, rdpstatetest.BGRARed))
	sendLegacyPDU(t, s, rdpstatetest.BuildPointerPositionPDU(50, 60))

	cs := s.CursorState()
	require.Equal(t, decoder.CursorState{Visible: true, X: 50, Y: 60}, cs)
}

func TestLegacyCursorState_PointerHidden(t *testing.T) {
	s := New()

	require.NoError(t, s.HandleMessage(legacyConnectionActivated(t, 100, 100)))

	sendLegacyPDU(t, s, rdpstatetest.BuildNewPointerPDU(2, 2, 0, 0, rdpstatetest.BGRARed))
	require.True(t, s.CursorState().Visible)

	sendLegacyPDU(t, s, rdpstatetest.BuildPointerHiddenPDU())
	require.False(t, s.CursorState().Visible)
}

func TestLegacyCursorState_PointerDefaultRestoresVisibility(t *testing.T) {
	s := New()

	require.NoError(t, s.HandleMessage(legacyConnectionActivated(t, 100, 100)))

	sendLegacyPDU(t, s, rdpstatetest.BuildNewPointerPDU(2, 2, 0, 0, rdpstatetest.BGRARed))
	sendLegacyPDU(t, s, rdpstatetest.BuildPointerHiddenPDU())
	require.False(t, s.CursorState().Visible)

	sendLegacyPDU(t, s, rdpstatetest.BuildPointerDefaultPDU())
	require.True(t, s.CursorState().Visible)
}

func sendPDU(t *testing.T, s *RDPState, pdu []byte) {
	t.Helper()

	require.NoError(t, s.HandleMessage(encodeTDPBFastPathPDU(t, pdu)))
}

func sendLegacyPDU(t *testing.T, s *RDPState, pdu []byte) {
	t.Helper()

	// Encode as legacy RDPFastPathPDU: type byte (29) + uint32 length + data.
	data := make([]byte, 1+4+len(pdu))
	data[0] = 29 // TypeRDPFastPathPDU
	binary.BigEndian.PutUint32(data[1:5], uint32(len(pdu)))
	copy(data[5:], pdu)
	require.NoError(t, s.HandleMessage(rdpstatetest.LegacyEvent(data)))
}
