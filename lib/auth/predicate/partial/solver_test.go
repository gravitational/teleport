/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package predicate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSolverIntEq tests solving for a single integer equality.
func TestSolverIntEq(t *testing.T) {
	state := NewSolver()
	x, err := state.PartialSolveForAll("x == 7", func(s []string) any {
		return nil
	}, "x", TypeInt, 10*time.Second)

	require.NoError(t, err)
	require.Len(t, x, 1)
	require.Equal(t, "7", x[0].String())
}

// TestSolverStringExpMultiSolution tests solving against a string equality expression with two solutions.
func TestSolverStringExpMultiSolution(t *testing.T) {
	resolver := func(s []string) any {
		if len(s) > 0 && s[0] == "jimsName" {
			return "jims"
		}
		return nil
	}

	state := NewSolver()
	x, err := state.PartialSolveForAll("x == \"blah\" || x == \"root\" || x == jimsName", resolver, "x", TypeString, 10*time.Second)
	require.NoError(t, err)

	s := make([]string, len(x))
	for i, v := range x {
		s[i] = v.String()
	}
	require.ElementsMatch(t, []string{`"blah"`, `"root"`, `"jims"`}, s)
}

// BenchmarkSolverStringExpMultiSolution benchmarks TestSolverStringExpMultiSolution for performance monitoring.
func BenchmarkSolverStringExpMultiSolution(b *testing.B) {
	resolver := func(s []string) any {
		if len(s) > 0 && s[0] == "jimsName" {
			return "jims"
		}
		return nil
	}

	state := NewSolver()

	for i := 0; i < b.N; i++ {
		x, err := state.PartialSolveForAll("x == \"blah\" || x == \"root\" || x == jimsName", resolver, "x", TypeString, 10*time.Second)

		if err != nil {
			b.Fatal(err)
		}

		s := make([]string, len(x))
		for i, v := range x {
			s[i] = v.String()
		}
		require.ElementsMatch(b, []string{`"blah"`, `"root"`, `"jims"`}, s)
	}
}
