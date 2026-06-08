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
	"image"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/srv/desktop/rdpstate/rdpstatetest"
)

func TestDimensions_BeforeServerHello(t *testing.T) {
	t.Parallel()

	s := New()
	t.Cleanup(s.Release)
	w, h := s.Dimensions()
	require.Equal(t, uint16(0), w)
	require.Equal(t, uint16(0), h)
}

func TestDimensions_AfterServerHello(t *testing.T) {
	t.Parallel()

	s := New()
	t.Cleanup(s.Release)
	require.NoError(t, s.HandleMessage(encodeTDPBServerHello(t, 1920, 1080)))

	w, h := s.Dimensions()
	require.Equal(t, uint16(1920), w)
	require.Equal(t, uint16(1080), h)
}

func TestDimensions_AfterResize(t *testing.T) {
	t.Parallel()

	s := New()
	t.Cleanup(s.Release)
	require.NoError(t, s.HandleMessage(encodeTDPBServerHello(t, 800, 600)))
	require.NoError(t, s.HandleMessage(encodeTDPBServerHello(t, 1920, 1080)))

	w, h := s.Dimensions()
	require.Equal(t, uint16(1920), w)
	require.Equal(t, uint16(1080), h)
}

func TestResizeCrop_BeforeServerHello(t *testing.T) {
	t.Parallel()

	s := New()
	t.Cleanup(s.Release)
	img, err := s.ResizeCrop(0, 0, 100, 100, 50, 50)
	require.Error(t, err)
	require.Nil(t, img)
}

func TestResizeCrop_ReturnsCorrectDimensions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		cropX, cropY uint16
		cropW, cropH uint16
		outW, outH   uint16
	}{
		{"identity full-screen", 0, 0, 800, 600, 800, 600},
		{"half-resolution crop of full screen", 0, 0, 800, 600, 400, 300},
		{"upscale a tiny crop", 100, 100, 50, 50, 500, 500},
		{"non-square crop and out", 200, 100, 400, 200, 100, 50},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := New()
			t.Cleanup(s.Release)
			require.NoError(t, s.HandleMessage(encodeTDPBServerHello(t, 800, 600)))

			img, err := s.ResizeCrop(tc.cropX, tc.cropY, tc.cropW, tc.cropH, tc.outW, tc.outH)
			require.NoError(t, err)
			require.NotNil(t, img)
			require.Equal(t, image.Rect(0, 0, int(tc.outW), int(tc.outH)), img.Bounds())
			require.Len(t, img.Pix, int(tc.outW)*int(tc.outH)*4)
		})
	}
}

func TestResizeCrop_PreservesSolidColor(t *testing.T) {
	t.Parallel()

	s := New()
	t.Cleanup(s.Release)
	require.NoError(t, s.HandleMessage(encodeTDPBServerHello(t, 100, 100)))

	// Fill the whole screen with red.
	sendPDU(t, s, rdpstatetest.BuildBitmapPDU(0, 0, 100, 100, rdpstatetest.RGB565Red))

	// Crop a 10x10 region from the middle and scale up; every output pixel must still be red.
	img, err := s.ResizeCrop(45, 45, 10, 10, 50, 50)
	require.NoError(t, err)
	require.NotNil(t, img)
	require.Equal(t, image.Rect(0, 0, 50, 50), img.Bounds())

	for i := 0; i < len(img.Pix); i += 4 {
		require.Equal(t, uint8(0xFF), img.Pix[i], "R at offset %d", i)
		require.Equal(t, uint8(0x00), img.Pix[i+1], "G at offset %d", i)
		require.Equal(t, uint8(0x00), img.Pix[i+2], "B at offset %d", i)
		require.Equal(t, uint8(0xFF), img.Pix[i+3], "A at offset %d", i)
	}
}

func TestResizeCrop_RejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		cropX, cropY uint16
		cropW, cropH uint16
		outW, outH   uint16
	}{
		{"crop extends past width", 90, 0, 20, 10, 10, 10},
		{"crop extends past height", 0, 95, 10, 10, 10, 10},
		{"zero crop width", 0, 0, 0, 10, 10, 10},
		{"zero crop height", 0, 0, 10, 0, 10, 10},
		{"zero output width", 0, 0, 10, 10, 0, 10},
		{"zero output height", 0, 0, 10, 10, 10, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := New()
			t.Cleanup(s.Release)
			require.NoError(t, s.HandleMessage(encodeTDPBServerHello(t, 100, 100)))

			img, err := s.ResizeCrop(tc.cropX, tc.cropY, tc.cropW, tc.cropH, tc.outW, tc.outH)
			require.Error(t, err)
			require.Nil(t, img)
		})
	}
}

func TestSampleHash_BeforeServerHello(t *testing.T) {
	t.Parallel()

	s := New()
	t.Cleanup(s.Release)
	require.Equal(t, uint64(0), s.SampleHash(64))
}

func TestSampleHash_ZeroSampleCount(t *testing.T) {
	t.Parallel()

	s := New()
	t.Cleanup(s.Release)
	require.NoError(t, s.HandleMessage(encodeTDPBServerHello(t, 100, 100)))

	require.Equal(t, uint64(0), s.SampleHash(0))
}

func TestSampleHash_DeterministicForSameFrame(t *testing.T) {
	t.Parallel()

	s := New()
	t.Cleanup(s.Release)
	require.NoError(t, s.HandleMessage(encodeTDPBServerHello(t, 100, 100)))
	sendPDU(t, s, rdpstatetest.BuildBitmapPDU(0, 0, 100, 100, rdpstatetest.RGB565Red))

	h1 := s.SampleHash(64)
	h2 := s.SampleHash(64)

	require.NotZero(t, h1)
	require.Equal(t, h1, h2)
}

func TestSampleHash_ChangesWhenFrameChanges(t *testing.T) {
	t.Parallel()

	s := New()
	t.Cleanup(s.Release)
	require.NoError(t, s.HandleMessage(encodeTDPBServerHello(t, 100, 100)))

	sendPDU(t, s, rdpstatetest.BuildBitmapPDU(0, 0, 100, 100, rdpstatetest.RGB565Red))
	red := s.SampleHash(64)

	sendPDU(t, s, rdpstatetest.BuildBitmapPDU(0, 0, 100, 100, rdpstatetest.RGB565Blue))
	blue := s.SampleHash(64)

	require.NotEqual(t, red, blue)
}
