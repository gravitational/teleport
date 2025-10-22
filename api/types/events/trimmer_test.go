/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package events

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/wrappers"
)

const trimmerTestMaxFieldSize = 10

type trimmerTestSuite[T any] struct {
	source      T
	target      T
	makeTrimmer func(T, *T) fieldTrimmer

	expectNonEmptyStrs int
	expectTrimmedValue T
}

func (s *trimmerTestSuite[T]) Run(t *testing.T) {
	require.NotEmpty(t, s.target)

	trimmer := s.makeTrimmer(s.source, &s.target)
	t.Run("target should be emptied on new trimmer", func(t *testing.T) {
		require.Empty(t, s.target, "target should be empty on new trimmer")
	})

	t.Run("nonEmptyStrs", func(t *testing.T) {
		require.Equal(t, s.expectNonEmptyStrs, trimmer.nonEmptyStrs())
	})

	t.Run("trimToMaxFieldSize not trimmed", func(t *testing.T) {
		trimmer.trimToMaxFieldSize(10000)
		require.Equal(t, s.source, s.target)
	})

	t.Run("trimToMaxFieldSize not trimmed", func(t *testing.T) {
		trimmer.trimToMaxFieldSize(trimmerTestMaxFieldSize)
		require.Equal(t, s.expectTrimmedValue, s.target)
	})
}

func Test_newStrTrimmer(t *testing.T) {
	s := trimmerTestSuite[string]{
		source:             strings.Repeat("source", 10),
		target:             "some-initial-value",
		makeTrimmer:        newStrTrimmer,
		expectNonEmptyStrs: 1,
		// trimStr reserves two bytes for quotes so 10-2=8
		expectTrimmedValue: "sourceso",
	}
	s.Run(t)
}

func Test_newBytesTrimmer(t *testing.T) {
	s := trimmerTestSuite[[]byte]{
		source:             bytes.Repeat([]byte("source"), 10),
		target:             []byte("some-initial-value"),
		makeTrimmer:        newBytesTrimmer,
		expectNonEmptyStrs: 1,
		// trimStr reserves two bytes for quotes so 10-2=8
		expectTrimmedValue: []byte("sourceso"),
	}
	s.Run(t)
}

func Test_newStrSliceTrimmer(t *testing.T) {
	s := trimmerTestSuite[[]string]{
		source:             []string{strings.Repeat("a", 100), strings.Repeat("b", 100)},
		target:             []string{"some-initial-value"},
		makeTrimmer:        newStrSliceTrimmer,
		expectNonEmptyStrs: 2,
		expectTrimmedValue: []string{strings.Repeat("a", 8), strings.Repeat("b", 8)},
	}
	s.Run(t)
}

func Test_newTraitsTrimmer(t *testing.T) {
	s := trimmerTestSuite[wrappers.Traits]{
		source: wrappers.Traits{
			"a":  {strings.Repeat("a", 100)},
			"bc": {strings.Repeat("b", 100), strings.Repeat("c", 100)},
		},
		target: wrappers.Traits{
			"some": {"initial", "value"},
		},
		makeTrimmer:        newTraitsTrimmer,
		expectNonEmptyStrs: 5, // count keys and values
		expectTrimmedValue: wrappers.Traits{
			"a":  {strings.Repeat("a", 8)},
			"bc": {strings.Repeat("b", 8), strings.Repeat("c", 8)},
		},
	}
	s.Run(t)
}
