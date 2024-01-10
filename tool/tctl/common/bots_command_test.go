/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetsEqual(t *testing.T) {
	tests := []struct {
		name   string
		a      map[string]struct{}
		b      map[string]struct{}
		assert require.BoolAssertionFunc
	}{
		{
			name:   "out of order true",
			a:      arrayToSet([]string{"a", "b", "c", "d"}),
			b:      arrayToSet([]string{"d", "b", "c", "a"}),
			assert: require.True,
		},
		{
			name:   "length mismatch",
			a:      arrayToSet([]string{"a", "b", "c"}),
			b:      arrayToSet([]string{"d", "b", "c", "a"}),
			assert: require.False,
		},
		{
			name:   "simple false",
			a:      arrayToSet([]string{"a", "b", "c", "d"}),
			b:      arrayToSet([]string{"d", "b", "c", "e"}),
			assert: require.False,
		},
		{
			name:   "duplicates ignored",
			a:      arrayToSet([]string{"a", "b", "c", "d"}),
			b:      arrayToSet([]string{"d", "b", "c", "a", "a", "b", "c"}),
			assert: require.True,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.assert(t, setsEqual(tt.a, tt.b))
		})
	}
}

func TestArrayToSetRoundtrip(t *testing.T) {
	a := []string{"a", "b", "c", "d"}
	require.ElementsMatch(t, a, setToArray(arrayToSet(a)))

	// It should also remove duplicates
	require.ElementsMatch(t, setToArray(arrayToSet(append(a, a...))), a)
}

func TestSetUnion(t *testing.T) {
	a := []string{"a", "b", "c", "d"}
	b := []string{"c", "d", "e", "f"}

	// Self union clones
	require.ElementsMatch(
		t,
		setToArray(setUnion(arrayToSet(a), arrayToSet(a), arrayToSet(a))),
		a,
	)

	require.ElementsMatch(
		t,
		setToArray(setUnion(arrayToSet(a), arrayToSet(b))),
		[]string{"a", "b", "c", "d", "e", "f"},
	)
}
