// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	t.Run("emptyTarget", func(t *testing.T) {
		trimmer.emptyTarget()
		require.Empty(t, s.target)
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
