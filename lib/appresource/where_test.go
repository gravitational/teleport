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
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// evaluate compiles expr and evaluates it against request and identity,
// returning the boolean result and the recorded evaluation state.
func evaluate(t *testing.T, expr string, request Request, identity Identity) (bool, *evalState) {
	t.Helper()
	pred, err := compilePredicate(expr)
	require.NoError(t, err)
	e := newEnv(request, identity)
	got, err := pred.Evaluate(e)
	require.NoError(t, err)
	return got, e.state
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
		{`equals(user.name, "alice")`, true},
		{`equals(user.name, "bob")`, false},
		{`contains(user.roles, "dev")`, true},
		{`contains(user.roles, "admin")`, false},
		{`contains(user.traits["allowed_projects"], "acme")`, true},
		{`contains(user.traits["allowed_projects"], "secret")`, false},
		{`contains(user.traits["missing"], "acme")`, false},
		{`request.method == "GET"`, true},
		{`request.method != "GET"`, false},
		{`user.name == "alice" && request.method == "GET"`, true},
		{`user.name == "bob" || contains(user.roles, "access")`, true},
		{`!contains(user.roles, "admin")`, true},
		{`contains(set("alice", "bob"), user.name)`, true},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got, _ := evaluate(t, tt.expr, Request{Method: "GET"}, identity)
			require.Equal(t, tt.want, got)
		})
	}
}

// TestInvalidPredicateRejected pins that an identifier outside the where
// environment, and a malformed expression, both fail at compile so a
// typo cannot silently evaluate.
func TestInvalidPredicateRejected(t *testing.T) {
	for _, expr := range []string{
		`request.path == "/api"`,
		`vars.project == "acme"`,
		`user.nope == "x"`,
		`regex_match("a.*", user.name)`,
		`user.name ==`,
		`&& true`,
		`contains(user.roles, )`,
	} {
		t.Run(expr, func(t *testing.T) {
			_, err := compilePredicate(expr)
			require.Error(t, err)
		})
	}
}

func TestFreshStatePerEvaluation(t *testing.T) {
	pred, err := compilePredicate(`allow_code("dev_read", "Dev access.", contains(user.roles, "dev"))`)
	require.NoError(t, err)

	e := newEnv(Request{Method: "GET"}, Identity{Roles: []string{"dev"}})
	got, err := pred.Evaluate(e)
	require.NoError(t, err)
	require.True(t, got)
	require.Equal(t, "dev_read", e.state.allowCode)

	e = newEnv(Request{Method: "GET"}, Identity{})
	got, err = pred.Evaluate(e)
	require.NoError(t, err)
	require.False(t, got)
	require.Empty(t, e.state.allowCode)
}

// TestLowerUpper pins the lower and upper string helpers, used to compare
// case-insensitively in a where clause.
func TestLowerUpper(t *testing.T) {
	const lowered = `contains(set("get", "head"), lower(request.method))`
	const uppered = `contains(set("GET", "HEAD"), upper(request.method))`

	tests := []struct {
		method string
		want   bool
	}{
		{"GET", true},
		{"get", true},
		{"GeT", true},
		{"DELETE", false},
		{"delete", false},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			got, _ := evaluate(t, lowered, Request{Method: tt.method}, Identity{})
			require.Equal(t, tt.want, got)

			got, _ = evaluate(t, uppered, Request{Method: tt.method}, Identity{})
			require.Equal(t, tt.want, got)
		})
	}

	// The lower function folds case the way strings.ToLower does, beyond ASCII.
	got, _ := evaluate(t, `lower(user.name) == "münchen"`, Request{}, Identity{Name: "MÜNCHEN"})
	require.True(t, got)
}

// TestSubstringFuncs pins has_prefix, has_suffix, and has_substring, the
// string helpers for a where clause. has_substring is substring search.
// contains is list membership.
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
			got, _ := evaluate(t, tt.expr, Request{Method: "GET"}, identity)
			require.Equal(t, tt.want, got)
		})
	}
}

// TestConcurrentEvaluationIsolatesState pins that concurrent evaluations
// of one compiled predicate never share recorded codes or hints. Run
// under -race, it also catches a data race on the shared expression tree.
func TestConcurrentEvaluationIsolatesState(t *testing.T) {
	pred, err := compilePredicate(`allow_code("has_dev", "Dev.", contains(user.roles, "dev")) || ` +
		`deny_hint("needs_dev", "Need dev.", contains(user.roles, "dev"))`)
	require.NoError(t, err)

	const n = 50
	type result struct {
		got   bool
		state *evalState
		err   error
	}
	results := make([]result, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var roles []string
			if i%2 == 0 {
				roles = []string{"dev"}
			}
			e := newEnv(Request{Method: "GET"}, Identity{Roles: roles})
			got, err := pred.Evaluate(e)
			results[i] = result{got: got, state: e.state, err: err}
		}(i)
	}
	wg.Wait()

	for i, r := range results {
		require.NoError(t, r.err)
		if i%2 == 0 {
			require.True(t, r.got)
			require.Equal(t, "has_dev", r.state.allowCode)
			require.Empty(t, r.state.denyHints)
		} else {
			require.False(t, r.got)
			require.Empty(t, r.state.allowCode)
			require.Equal(t, []Hint{{Code: "needs_dev", Reason: "Need dev."}}, r.state.denyHints)
		}
	}
}
