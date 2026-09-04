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

package strings_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	tctlstrings "github.com/gravitational/teleport/tool/tctl/common/strings"
)

func TestProperty_FindMinPrefixes(t *testing.T) {
	t.Parallel()

	countUnique := func(vals []string) int {
		m := make(map[string]struct{})
		for _, v := range vals {
			m[v] = struct{}{}
		}
		return len(m)
	}

	solveMinPrefixes := func(inputs []string) []string {
		if len(inputs) == 0 {
			return nil
		}
		largest := 0
		for _, x := range inputs {
			largest = max(largest, len(x))
		}
		for l := 8; l <= largest-1; l++ {
			answer := slices.Clone(inputs)
			for i, x := range answer {
				answer[i] = x[:min(l, len(x))]
			}
			if countUnique(answer) == len(inputs) {
				return answer
			}
		}
		return inputs
	}

	rapid.Check(t, func(t *rapid.T) {
		const minLen = 0
		const maxLen = 20
		inputs := rapid.SliceOfN(rapid.String(), minLen, maxLen).Draw(t, "in")

		got := tctlstrings.FindMinPrefixes(inputs)

		// Property: length is maintained.
		require.Len(t, got, len(inputs), "got has unexpected length")

		// Property: outputs are prefixes of the inputs.
		for i := range got {
			g := got[i]
			in := inputs[i]
			require.True(t, strings.HasPrefix(in, g), "got is not a prefix of inputs")
		}

		// Property: outputs are as unique as the inputs.
		if countUnique(inputs) != len(inputs) {
			require.Equal(t, inputs, got, "got is not equal to inputs")
		} else {
			require.Equal(t, len(inputs), countUnique(got), "got is less unique than inputs")
		}

		// Property: idempotence.
		got2 := tctlstrings.FindMinPrefixes(got)
		require.Equal(t, got, got2, "got failed idempotence check")

		// Property: compare with ideal answer.
		require.Equal(t, solveMinPrefixes(inputs), got, "got differs from expected answer")
	})
}
