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

	"github.com/stretchr/testify/require"
)

func TestSolverIntEq(t *testing.T) {
	state := NewCachedSolver()
	x, err := state.PartialSolve("x == 7", func(s []string) any {
		return nil
	}, "x", TypeInt)

	require.NoError(t, err)
	require.Equal(t, "7", x.String())
}

func TestSolverStringExp(t *testing.T) {
	state := NewCachedSolver()
	x, err := state.PartialSolve("x == \"blah\"", func(s []string) any {
		return nil
	}, "x", TypeString)

	require.NoError(t, err)
	require.Equal(t, "\"blah\"", x.String())
}

var xRes string

func BenchmarkSolverEq(b *testing.B) {
	state := NewCachedSolver()

	for i := 0; i < b.N; i++ {
		x, err := state.PartialSolve("x == 7", func(s []string) any {
			return nil
		}, "x", TypeInt)

		if err != nil {
			b.Fatal(err)
		}

		xRes = x.String()
	}
}
