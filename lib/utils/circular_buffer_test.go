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

package utils

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
)

func TestNewCircularBuffer(t *testing.T) {
	t.Parallel()

	buff, err := NewCircularBuffer(-1)
	require.Error(t, err)
	require.Nil(t, buff)

	buff, err = NewCircularBuffer(5)
	require.NoError(t, err)
	require.NotNil(t, buff)
	require.Len(t, buff.buf, 5)
}

func TestCircularBuffer_Data(t *testing.T) {
	t.Parallel()

	buff, err := NewCircularBuffer(5)
	require.NoError(t, err)

	expectData := func(expected []float64) {
		for i := range 15 {
			e := expected
			if i <= len(expected) {
				e = expected[len(expected)-i:]
			}
			require.Empty(t, cmp.Diff(e, buff.Data(i), cmpopts.EquateEmpty()), "i = %v", i)
		}
	}

	expectData(nil)

	buff.Add(1)
	expectData([]float64{1})

	buff.Add(2)
	buff.Add(3)
	buff.Add(4)
	expectData([]float64{1, 2, 3, 4})

	buff.Add(5)
	buff.Add(6)
	buff.Add(7)
	expectData([]float64{3, 4, 5, 6, 7})
}
