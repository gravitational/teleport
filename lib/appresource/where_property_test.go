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

package appresource

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// drawEnv draws a valid environment with a canonical method.
func drawEnv(t *rapid.T) Env {
	str := rapid.StringMatching(`[a-zA-Z0-9_\-]{0,8}`)
	return Env{
		Request: Request{Method: rapid.SampledFrom(validMethods).Draw(t, "method")},
		Identity: Identity{
			Name:   str.Draw(t, "name"),
			Roles:  rapid.SliceOfN(str, 0, 3).Draw(t, "roles"),
			Traits: rapid.MapOfN(str, rapid.SliceOfN(str, 0, 3), 0, 3).Draw(t, "traits"),
		},
	}
}

// drawClause draws a well-formed where clause of bounded depth.
func drawClause(t *rapid.T, depth int) string {
	if depth == 0 || rapid.Bool().Draw(t, "leaf") {
		str := rapid.StringMatching(`[a-zA-Z0-9_\-]{0,8}`)
		switch rapid.IntRange(0, 5).Draw(t, "kind") {
		case 0:
			return `true`
		case 1:
			return `false`
		case 2:
			return fmt.Sprintf(`user.name == %q`, str.Draw(t, "literal"))
		case 3:
			return fmt.Sprintf(`contains(user.roles, %q)`, str.Draw(t, "literal"))
		case 4:
			return fmt.Sprintf(`request.method == %q`, rapid.SampledFrom(validMethods).Draw(t, "m"))
		default:
			return fmt.Sprintf(`has_prefix(user.name, %q)`, str.Draw(t, "literal"))
		}
	}
	switch rapid.IntRange(0, 2).Draw(t, "op") {
	case 0:
		return fmt.Sprintf("(%s && %s)", drawClause(t, depth-1), drawClause(t, depth-1))
	case 1:
		return fmt.Sprintf("(%s || %s)", drawClause(t, depth-1), drawClause(t, depth-1))
	default:
		return fmt.Sprintf("!(%s)", drawClause(t, depth-1))
	}
}

// evaluateProp compiles and evaluates a generated clause. A well-formed
// clause over a valid environment must neither fail to compile nor fail
// to evaluate.
func evaluateProp(t *rapid.T, expr string, env Env) bool {
	t.Helper()
	where, err := CompileWhere(expr)
	if err != nil {
		t.Fatalf("CompileWhere(%q): %v", expr, err)
	}
	got, err := where.Evaluate(env)
	if err != nil {
		t.Fatalf("Evaluate(%q): %v", expr, err)
	}
	return got
}

func TestCompileNeverPanics(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		expr := rapid.String().Draw(t, "expr")
		where, err := CompileWhere(expr)
		if err == nil && where == nil {
			t.Fatalf("CompileWhere(%q) returned neither a value nor an error", expr)
		}
	})
}

func TestNegationDuality(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		clause := drawClause(t, 2)
		env := drawEnv(t)
		got := evaluateProp(t, clause, env)
		negated := evaluateProp(t, "!("+clause+")", env)
		if got == negated {
			t.Fatalf("clause %q and its negation both evaluate to %v", clause, got)
		}
	})
}

func TestInvalidMethodDenied(t *testing.T) {
	where, err := CompileWhere(`true`)
	require.NoError(t, err)
	rapid.Check(t, func(t *rapid.T) {
		method := rapid.String().Filter(func(s string) bool {
			return !slices.Contains(validMethods, s)
		}).Draw(t, "method")
		got, err := where.Evaluate(Env{Request: Request{Method: method}})
		if err == nil || got {
			t.Fatalf("Evaluate with method %q = %v, %v, want false and an error", method, got, err)
		}
	})
}

func TestContainsMatchesSlicesContains(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		str := rapid.StringMatching(`[a-zA-Z0-9_\-]{0,8}`)
		items := rapid.SliceOfN(str, 0, 4).Draw(t, "items")
		item := str.Draw(t, "item")
		quoted := make([]string, len(items))
		for i, s := range items {
			quoted[i] = strconv.Quote(s)
		}
		expr := fmt.Sprintf("contains(set(%s), %s)", strings.Join(quoted, ", "), strconv.Quote(item))
		got := evaluateProp(t, expr, drawEnv(t))
		if want := slices.Contains(items, item); got != want {
			t.Fatalf("%s = %v, want %v", expr, got, want)
		}
	})
}

func TestEvaluateDeterministic(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		clause := drawClause(t, 2)
		env := drawEnv(t)
		first := evaluateProp(t, clause, env)
		second := evaluateProp(t, clause, env)
		if first != second {
			t.Fatalf("clause %q evaluated to %v then %v for the same environment", clause, first, second)
		}
	})
}
