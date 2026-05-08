// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package decoder

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFitDimensions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		srcW, srcH, maxW, maxH uint16
		wantW, wantH           uint16
	}{
		{"no upscale: source fits", 800, 600, 1024, 768, 800, 600},
		{"exact fit", 800, 600, 800, 600, 800, 600},
		{"landscape proportional downscale", 1920, 1080, 960, 540, 960, 540},
		{"portrait proportional downscale", 1080, 1920, 540, 960, 540, 960},
		{"landscape clamped by width", 1920, 1080, 800, 600, 800, 450},
		{"portrait clamped by height", 1080, 1920, 800, 600, 338, 600},
		{"extreme aspect ratio scales to 1 (tall)", 1, 10000, 100, 100, 1, 100},
		{"extreme aspect ratio scales to 1 (wide)", 10000, 1, 100, 100, 100, 1},
		{"round-down to fit", 3, 3, 2, 2, 2, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotW, gotH := fitDimensions(tt.srcW, tt.srcH, tt.maxW, tt.maxH)
			require.Equal(t, tt.wantW, gotW, "width")
			require.Equal(t, tt.wantH, gotH, "height")
		})
	}
}

func TestFitDimensions_MinimumIsOnePixel(t *testing.T) {
	t.Parallel()

	w, h := fitDimensions(1000, 1, 10, 10)
	require.GreaterOrEqual(t, w, uint16(1))
	require.GreaterOrEqual(t, h, uint16(1))
}
