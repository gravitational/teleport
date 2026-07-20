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
	"strconv"
	"sync"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func evaluate(t *testing.T, expr string, request Request, identity Identity) bool {
	t.Helper()
	where, err := CompileWhere(expr)
	require.NoError(t, err)
	env := Env{Request: request, Identity: identity}
	got, err := where.Evaluate(env)
	require.NoError(t, err)
	return got
}

func TestEnvBindings(t *testing.T) {
	identity := Identity{
		Name:   "alice",
		Roles:  []string{"dev", "access"},
		Traits: map[string][]string{"allowed_projects": {"acme", "widgets"}},
	}
	tests := []struct {
		expr string
		want bool
	}{
		{`true`, true},
		{`false`, false},
		{`user.name == "alice"`, true},
		{`user.name == "bob"`, false},
		{`contains(user.roles, "dev")`, true},
		{`contains(user.roles, "admin")`, false},
		{`contains(user.traits["allowed_projects"], "acme")`, true},
		{`contains(user.traits["allowed_projects"], "secret")`, false},
		{`contains(user.traits["missing"], "acme")`, false},
		{`contains(user.traits.allowed_projects, "acme")`, true},
		{`request.method == "GET"`, true},
		{`request.method != "GET"`, false},
		// A method comparison is case-sensitive.
		{`request.method == "get"`, false},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			require.Equal(t, tt.want, evaluate(t, tt.expr, Request{Method: "GET"}, identity))
		})
	}
}

// TestMethodValidated checks that evaluation fails for any method
// outside the canonical HTTP method list, even when the clause does not
// read the method.
func TestMethodValidated(t *testing.T) {
	require.True(t, evaluate(t, `request.method == "DELETE"`, Request{Method: "DELETE"}, Identity{}))
	for _, expr := range []string{`request.method == "DELETE"`, `true`} {
		where, err := CompileWhere(expr)
		require.NoError(t, err)
		for _, method := range []string{"delete", "DeLeTe", "YOLO", ""} {
			t.Run(expr+"/"+method, func(t *testing.T) {
				got, err := where.Evaluate(Env{Request: Request{Method: method}})
				require.Error(t, err)
				require.False(t, got)
			})
		}
	}
}

func TestSet(t *testing.T) {
	identity := Identity{Name: "alice"}
	require.True(t, evaluate(t, `contains(set("alice", "bob"), user.name)`, Request{Method: "GET"}, identity))
	require.False(t, evaluate(t, `contains(set(), user.name)`, Request{Method: "GET"}, identity))
}

func TestContainsOnStringBinding(t *testing.T) {
	identity := Identity{Name: "alice"}
	require.True(t, evaluate(t, `contains(user.name, "alice")`, Request{Method: "GET"}, identity))
	require.False(t, evaluate(t, `contains(user.name, "ali")`, Request{Method: "GET"}, identity))
	require.True(t, evaluate(t, `has_substring(user.name, "ali")`, Request{Method: "GET"}, identity))
}

func TestLowerUpper(t *testing.T) {
	identity := Identity{Name: "alice"}
	require.True(t, evaluate(t, `lower(request.method) == "get"`, Request{Method: "GET"}, identity))
	require.True(t, evaluate(t, `upper(user.name) == "ALICE"`, Request{Method: "GET"}, identity))
}

func TestSubstringFuncs(t *testing.T) {
	identity := Identity{Name: "svc-ci-runner"}
	tests := []struct {
		expr string
		want bool
	}{
		{`has_prefix(user.name, "svc-")`, true},
		{`has_prefix(user.name, "usr-")`, false},
		{`has_suffix(user.name, "-runner")`, true},
		{`has_suffix(user.name, "-admin")`, false},
		{`has_substring(user.name, "-ci-")`, true},
		{`has_substring(user.name, "-qa-")`, false},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			require.Equal(t, tt.want, evaluate(t, tt.expr, Request{Method: "GET"}, identity))
		})
	}
}

func TestCombinations(t *testing.T) {
	identity := Identity{Name: "alice", Traits: map[string][]string{"allowed_projects": {"acme"}}}
	require.True(t, evaluate(t, `contains(user.traits["allowed_projects"], lower("ACME")) && request.method == "GET"`, Request{Method: "GET"}, identity))
	require.False(t, evaluate(t, `has_prefix(lower(user.name), "bob") || contains(set("dev"), user.name)`, Request{Method: "GET"}, identity))
	require.True(t, evaluate(t, `!contains(set("dev"), user.name)`, Request{Method: "GET"}, identity))
}

// TestInvalidWhereClause checks that an unknown binding, an unknown
// function, and the empty clause all fail at compile.
func TestInvalidWhereClause(t *testing.T) {
	for _, expr := range []string{
		// Bindings the where environment does not provide. Paths are
		// matched by the paths field, not in where.
		`request.path == "/api"`,
		`user.role == "dev"`,
		// Names outside the function set. Regular expressions are
		// excluded deliberately.
		`regex_match("a.*", user.name)`,
		`equals(user.name, "alice")`,
		// The empty clause never stands for allow-everything.
		``,
	} {
		t.Run(expr, func(t *testing.T) {
			_, err := CompileWhere(expr)
			require.ErrorContains(t, err, strconv.Quote(expr))
			require.True(t, trace.IsBadParameter(err))
		})
	}
}

// TestConcurrentEvaluate evaluates one Where from many goroutines so the
// race detector checks that it is concurrency safe.
func TestConcurrentEvaluate(t *testing.T) {
	where, err := CompileWhere(`contains(user.roles, "dev") && request.method == "GET"`)
	require.NoError(t, err)
	env := Env{
		Request:  Request{Method: "GET"},
		Identity: Identity{Roles: []string{"dev"}},
	}
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			got, err := where.Evaluate(env)
			assert.NoError(t, err)
			assert.True(t, got)
		})
	}
	wg.Wait()
}
