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

package session

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClampTTYSize(t *testing.T) {
	tests := []struct {
		name  string
		w, h  int
		wantW int
		wantH int
	}{
		{name: "normal 80x24", w: 80, h: 24, wantW: 80, wantH: 24},
		{name: "at upper bound", w: MaxTTYCols, h: MaxTTYRows, wantW: MaxTTYCols, wantH: MaxTTYRows},
		{name: "over upper bound", w: 99999, h: 99999, wantW: MaxTTYCols, wantH: MaxTTYRows},
		{name: "max int", w: math.MaxInt, h: math.MaxInt, wantW: MaxTTYCols, wantH: MaxTTYRows},
		{name: "zero", w: 0, h: 0, wantW: 1, wantH: 1},
		{name: "negative", w: -1, h: -1, wantW: 1, wantH: 1},
		{name: "mixed", w: 5000, h: 10, wantW: MaxTTYCols, wantH: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotW, gotH := ClampTTYSize(tt.w, tt.h)
			require.Equal(t, tt.wantW, gotW)
			require.Equal(t, tt.wantH, gotH)
		})
	}
}
