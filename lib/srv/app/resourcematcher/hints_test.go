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

package resourcematcher

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAllowCode pins that a matching rule carries its allow_code and
// allow_reason on the decision, and that neither appears on a deny.
func TestAllowCode(t *testing.T) {
	rule, err := ruleFromYAML(t, `
paths: ["/api/v4/projects/{project}/repository/**"]
methods: [GET]
allow_code: allowed_project
allow_reason: "User has access to project"
`).Compile()
	require.NoError(t, err)

	got, err := rule.Evaluate(Request{Method: "GET", Path: "/api/v4/projects/x/repository/tree"}, Identity{})
	require.NoError(t, err)
	require.True(t, got.Allowed)
	require.Equal(t, "allowed_project", got.Allow.Code)
	require.Equal(t, "User has access to project", got.Allow.Reason)

	denied, err := rule.Evaluate(Request{Method: "POST", Path: "/api/v4/projects/x/repository/tree"}, Identity{})
	require.NoError(t, err)
	require.False(t, denied.Allowed)
	require.Nil(t, denied.Allow)
}

func TestAllowCodeValidation(t *testing.T) {
	for _, code := range []string{"Project_Read", "teleport_internal", "has space", ""} {
		// The empty string means "no code", which is allowed; the others are
		// invalid charset or reserved prefix.
		_, err := ruleFromYAML(t, `
paths: ["/api/health"]
allow_code: `+strconv.Quote(code)+`
`).Compile()
		if code == "" {
			require.NoError(t, err, code)
			continue
		}
		require.Error(t, err, code)
	}
}

// TestDenyHintDefaultOn pins the default-on rule: with no on, a hint fires on a
// deny exactly when the path and method matched but where failed.
func TestDenyHintDefaultOn(t *testing.T) {
	rule, err := ruleFromYAML(t, `
paths: ["/api/v4/projects/{project}/**"]
methods: [GET]
where: contains(user.traits["allowed_projects"], vars.project)
allow_code: project_read
deny_hint:
  - deny_code: not_in_allowlist
    deny_reason: "Project is not in your allowlist."
`).Compile()
	require.NoError(t, err)

	allowed := Identity{Traits: map[string][]string{"allowed_projects": {"ok"}}}

	// Allow: path, method, and where all hold. The hint does not fire.
	got, err := rule.Evaluate(Request{Method: "GET", Path: "/api/v4/projects/ok/issues"}, allowed)
	require.NoError(t, err)
	require.True(t, got.Allowed)
	require.Equal(t, "project_read", got.Allow.Code)
	require.Nil(t, got.Deny)

	// Near miss: path and method match, where fails. The default-on hint fires.
	got, err = rule.Evaluate(Request{Method: "GET", Path: "/api/v4/projects/secret/issues"}, allowed)
	require.NoError(t, err)
	require.False(t, got.Allowed)
	require.Equal(t, []Hint{{Code: "not_in_allowlist", Reason: "Project is not in your allowlist."}}, got.Deny.Hints)

	// Method miss: the path matches but the method does not, so the default-on
	// hint, which requires both, does not fire.
	got, err = rule.Evaluate(Request{Method: "POST", Path: "/api/v4/projects/secret/issues"}, allowed)
	require.NoError(t, err)
	require.False(t, got.Allowed)
	require.Empty(t, got.Deny.Hints)

	// Path miss: outside the rule's territory entirely. No hint.
	got, err = rule.Evaluate(Request{Method: "GET", Path: "/api/v4/groups/secret"}, allowed)
	require.NoError(t, err)
	require.False(t, got.Allowed)
	require.Empty(t, got.Deny.Hints)
}

// TestDenyHintSugaredEqualsDesugared pins that the declarative default-on hint
// and its explicit desugared equivalent fire identically. The desugared form
// must spell out on as the path and method clauses, because a predicate-form
// rule has no separate path and method to default from.
func TestDenyHintSugaredEqualsDesugared(t *testing.T) {
	sugared, err := ruleFromYAML(t, `
paths: ["/api/v4/projects/{project}/**"]
methods: [GET]
where: contains(user.traits["allowed_projects"], vars.project)
deny_hint:
  - deny_code: not_in_allowlist
    deny_reason: "Project is not in your allowlist."
`).Compile()
	require.NoError(t, err)

	desugared, err := ruleFromYAML(t, `
pred: |
  path.match(literal("api/v4/projects", capture("project", greedy()))) &&
  contains(set("GET"), request.method) &&
  contains(user.traits["allowed_projects"], vars.project)
deny_hint:
  - on: |
      path.match(literal("api/v4/projects", capture("project", greedy()))) &&
      contains(set("GET"), request.method)
    deny_code: not_in_allowlist
    deny_reason: "Project is not in your allowlist."
`).Compile()
	require.NoError(t, err)

	id := Identity{Traits: map[string][]string{"allowed_projects": {"ok"}}}
	for _, req := range []Request{
		{Method: "GET", Path: "/api/v4/projects/ok/issues"},     // allow
		{Method: "GET", Path: "/api/v4/projects/secret/issues"}, // near miss, hint fires
		{Method: "POST", Path: "/api/v4/projects/secret/x"},     // method miss, no hint
		{Method: "GET", Path: "/api/v4/groups/x"},               // path miss, no hint
	} {
		gotS, err := sugared.Evaluate(req, id)
		require.NoError(t, err)
		gotD, err := desugared.Evaluate(req, id)
		require.NoError(t, err)
		require.Equal(t, gotS.Allowed, gotD.Allowed, "%s %s", req.Method, req.Path)
		require.Equal(t, gotS.Deny, gotD.Deny, "%s %s", req.Method, req.Path)
	}
}

// TestDenyHintRequiresOnInPredicateForm pins that a predicate-form rule cannot
// default the hint territory and must set on explicitly.
func TestDenyHintRequiresOnInPredicateForm(t *testing.T) {
	_, err := ruleFromYAML(t, `
pred: path.match(literal("api", greedy()))
deny_hint:
  - deny_code: nope
    deny_reason: "no on, no territory"
`).Compile()
	require.Error(t, err)
}

// TestDenyHintExplicitOn pins that an explicit on fires the hint when, and only
// when, the on predicate holds on a deny. Several matching hints all fire.
func TestDenyHintExplicitOn(t *testing.T) {
	rule, err := ruleFromYAML(t, `
paths: ["/api/v4/projects/{project}/**"]
methods: [GET, POST]
where: contains(user.traits["allowed_projects"], vars.project)
deny_hint:
  - on: contains(set("POST"), request.method)
    deny_code: writes_need_review
    deny_reason: "Writes require a review."
  - on: path.match(literal("api/v4/projects", capture("project", greedy())))
    deny_code: project_scope
    deny_reason: "Check the project scope."
`).Compile()
	require.NoError(t, err)

	// A POST to an in-territory path that fails where fires both hints: the
	// method-specific one and the path-territory one.
	got, err := rule.Evaluate(Request{Method: "POST", Path: "/api/v4/projects/secret/x"}, Identity{})
	require.NoError(t, err)
	require.False(t, got.Allowed)
	require.Equal(t, []Hint{
		{Code: "writes_need_review", Reason: "Writes require a review."},
		{Code: "project_scope", Reason: "Check the project scope."},
	}, got.Deny.Hints)
}
