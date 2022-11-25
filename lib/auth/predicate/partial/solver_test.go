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

func TestSolverIntEq(t *testing.T) {
	state := NewCachedSolver()
	x, err := state.PartialSolveForAll("x == 7", func(s []string) any {
		return nil
	}, "x", TypeInt, 10*time.Second)

	require.NoError(t, err)
	require.Len(t, x, 1)
	require.Equal(t, "7", x[0].String())
}

func TestSolverStringExpMultiSolution(t *testing.T) {
	state := NewCachedSolver()
	x, err := state.PartialSolveForAll("x == \"blah\" || x == \"root\"", func(s []string) any {
		return nil
	}, "x", TypeString, 10*time.Second)

	require.NoError(t, err)
	require.Len(t, x, 2)
	require.Equal(t, "\"blah\"", x[0].String())
	require.Equal(t, "\"root\"", x[1].String())
}

func BenchmarkSolverStringExpMultiSolution(b *testing.B) {
	state := NewCachedSolver()

	for i := 0; i < b.N; i++ {
		x, err := state.PartialSolveForAll("x == \"blah\" || x == \"root\"", func(s []string) any {
			return nil
		}, "x", TypeString, 10*time.Second)

		if err != nil {
			b.Fatal(err)
		}

		require.Equal(b, "\"blah\"", x[0].String())
		require.Equal(b, "\"root\"", x[1].String())
	}
}
