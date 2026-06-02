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

package desktop

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestApplyDesktopScale(t *testing.T) {
	tests := []struct {
		name                  string
		width, height         uint32
		scale                 uint16
		wantWidth, wantHeight uint32
	}{
		{"2x", 1200, 800, 200, 2400, 1600},
		{"1x unchanged", 1200, 800, 100, 1200, 800},
		{"sub-1x unchanged", 1200, 800, 50, 1200, 800},
		{"1.5x", 1200, 800, 150, 1800, 1200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, h := applyDesktopScale(tt.width, tt.height, tt.scale)
			require.Equal(t, tt.wantWidth, w)
			require.Equal(t, tt.wantHeight, h)
		})
	}
}

func TestClampScalePercent(t *testing.T) {
	tests := []struct {
		name string
		in   uint32
		want uint16
	}{
		{"zero preserved", 0, 0},
		{"normal 2x", 200, 200},
		{"at cap", maxScalePercent, maxScalePercent},
		{"above cap clamped", maxScalePercent + 1, maxScalePercent},
		{"uint16-truncating value clamped not wrapped", 65636, maxScalePercent},
		{"huge value clamped not wrapped", 4294967295, maxScalePercent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, clampScalePercent(tt.in))
		})
	}
}

// TestApplyDesktopScaleNoOverflow checks that an oversized request saturates
// instead of wrapping uint32 to a small value that slips past the MaxRDPScreen
// bounds check.
func TestApplyDesktopScaleNoOverflow(t *testing.T) {
	// 65538*65535 overflows uint32 and, with naive math, wraps to 655 — under
	// the 8192 limit. The fix must saturate instead.
	w, h := applyDesktopScale(65538, 65538, 65535)
	require.Greater(t, w, uint32(types.MaxRDPScreenWidth),
		"scaled width must not wrap below the max-size guard")
	require.Greater(t, h, uint32(types.MaxRDPScreenHeight),
		"scaled height must not wrap below the max-size guard")
}
