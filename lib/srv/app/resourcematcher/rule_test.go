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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// ruleFromYAML unmarshals a single sugared rule from YAML. The test surface is
// YAML on purpose: the cases below are written in the exact form an author would
// put under app_resources, so the test doubles as a worked example.
func ruleFromYAML(t *testing.T, doc string) Rule {
	t.Helper()
	var r Rule
	require.NoError(t, yaml.Unmarshal([]byte(doc), &r))
	return r
}

// exprFromYAML reads a bare predicate from a YAML "pred:" key and compiles it as
// an app_resources_expressions entry. It mirrors how a desugared rule is written
// as one predicate string, the parallel of node_labels_expression.
func exprFromYAML(t *testing.T, doc string) (*CompiledRule, error) {
	t.Helper()
	var d struct {
		Pred string `yaml:"pred"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(doc), &d))
	return compileExpression(d.Pred)
}

// probe is one request evaluated against a rule, with the decision expected.
type probe struct {
	method   string
	path     string
	identity Identity
	allow    bool
	vars     map[string]string
}

// TestSugaredEqualsDesugared is the central rule-level test. Each scenario
// gives the same rule twice: once in the declarative (sugared) YAML form, and
// once in the bare predicate (desugared) YAML form. Both compile through one
// engine, and every probe must return the identical decision from both, which
// is what "the sugar is exactly the desugared form" means in practice.
func TestSugaredEqualsDesugared(t *testing.T) {
	scenarios := []struct {
		name      string
		sugared   string
		desugared string
		probes    []probe
	}{
		{
			name: "method-scoped path",
			sugared: `
paths: ["/api/v4/projects/{project}/repository/**"]
methods: [GET, HEAD]
`,
			desugared: `
pred: |
  path.match(
    literal("api", literal("v4", literal("projects",
      capture("project",
        literal("repository", greedy())))))) &&
  contains(set("GET", "HEAD"), request.method)
`,
			probes: []probe{
				{
					method: "GET",
					path:   "/api/v4/projects/myproj/repository/tree",
					allow:  true,
					vars:   map[string]string{"project": "myproj"},
				},
				{
					method: "POST",
					path:   "/api/v4/projects/myproj/repository/tree",
					allow:  false,
				},
				{
					method: "GET",
					path:   "/api/v4/projects/myproj/settings",
					allow:  false,
				},
			},
		},
		{
			name: "capture checked against a trait",
			sugared: `
paths: ["/api/v4/projects/{project}/**"]
methods: [GET]
where: contains(user.traits["allowed_projects"], vars.project)
`,
			desugared: `
pred: |
  path.match(
    literal("api", literal("v4", literal("projects",
      capture("project", greedy()))))) &&
  contains(set("GET"), request.method) &&
  (contains(user.traits["allowed_projects"], vars.project))
`,
			probes: []probe{
				{
					method:   "GET",
					path:     "/api/v4/projects/allowed-one/issues",
					identity: Identity{Traits: map[string][]string{"allowed_projects": {"allowed-one", "allowed-two"}}},
					allow:    true,
					vars:     map[string]string{"project": "allowed-one"},
				},
				{
					method:   "GET",
					path:     "/api/v4/projects/secret/issues",
					identity: Identity{Traits: map[string][]string{"allowed_projects": {"allowed-one"}}},
					allow:    false,
				},
				{
					method:   "GET",
					path:     "/api/v4/projects/allowed-one/issues",
					identity: Identity{}, // no trait at all reads as empty: deny.
					allow:    false,
				},
			},
		},
		{
			// A glob in a non-final position desugars to a glob node that
			// carries the following segments as children. The renderer must
			// emit glob(child, ...) with no leading comma, since glob takes no
			// name argument the way literal and capture do.
			name: "glob before a capture",
			sugared: `
paths: ["/api/v4/projects/*/{project}/**"]
methods: [GET]
`,
			desugared: `
pred: |
  path.match(
    literal("api", literal("v4", literal("projects",
      glob(capture("project", greedy())))))) &&
  contains(set("GET"), request.method)
`,
			probes: []probe{
				{
					method: "GET",
					path:   "/api/v4/projects/anyteam/myproj/issues",
					allow:  true,
					vars:   map[string]string{"project": "myproj"},
				},
				{
					// The glob and the capture each require a segment, so a
					// path that ends at the glob position leaves the capture
					// with nothing to bind and does not match.
					method: "GET",
					path:   "/api/v4/projects/onlyteam",
					allow:  false,
				},
			},
		},
		{
			// A trailing slash in a pattern is significant. It desugars to a
			// slash() node that matches the trailing empty segment a request
			// path produces, so the slashed pattern matches only the slashed
			// request, not the bare one.
			name: "trailing slash path",
			sugared: `
paths: ["/api/v4/health/"]
methods: [GET]
`,
			desugared: `
pred: |
  path.match(literal("api/v4/health", slash())) &&
  contains(set("GET"), request.method)
`,
			probes: []probe{
				{
					method: "GET",
					path:   "/api/v4/health/",
					allow:  true,
				},
				{
					method: "GET",
					path:   "/api/v4/health",
					allow:  false,
				},
			},
		},
		{
			// The bare root "/" is the trailing-slash rule taken to its limit:
			// a single slash() node that matches only the root request "/".
			name: "bare root path",
			sugared: `
paths: ["/"]
methods: [GET]
`,
			desugared: `
pred: |
  path.match(slash()) &&
  contains(set("GET"), request.method)
`,
			probes: []probe{
				{
					method: "GET",
					path:   "/",
					allow:  true,
				},
				{
					method: "GET",
					path:   "/foo",
					allow:  false,
				},
			},
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			sugared, err := ruleFromYAML(t, sc.sugared).Compile()
			require.NoError(t, err)
			desugared, err := exprFromYAML(t, sc.desugared)
			require.NoError(t, err)

			for _, p := range sc.probes {
				req := Request{Method: p.method, Path: p.path}

				gotSugared, err := sugared.Evaluate(req, p.identity)
				require.NoError(t, err)
				gotDesugared, err := desugared.Evaluate(req, p.identity)
				require.NoError(t, err)

				// The two surfaces must agree, and both must match the
				// expectation.
				require.Equal(t, gotSugared, gotDesugared,
					"sugared and desugared disagree for %s %s", p.method, p.path)
				require.Equal(t, p.allow, gotSugared.Allowed,
					"unexpected decision for %s %s", p.method, p.path)
				if p.allow && p.vars != nil {
					require.Equal(t, p.vars, gotSugared.Allow.Vars)
				}
			}
		})
	}
}

// TestRoleSetUnion pins the additive OR-union within one role: a request is
// allowed if any of the role's rules matches, and captures come from the
// matching rule.
func TestRoleSetUnion(t *testing.T) {
	set, err := CompileRoles([]Role{{
		Name: "reader",
		Resources: []Rule{
			ruleFromYAML(t, `
paths: ["/api/v4/projects/{project}/repository/**"]
methods: [GET]
`),
			ruleFromYAML(t, `
paths: ["/api/v4/user"]
methods: [GET]
`),
		},
	}})
	require.NoError(t, err)

	cases := []probe{
		{method: "GET", path: "/api/v4/projects/x/repository/tree", allow: true, vars: map[string]string{"project": "x"}},
		{method: "GET", path: "/api/v4/user", allow: true, vars: map[string]string{}},
		{method: "GET", path: "/api/v4/groups/y", allow: false},
		{method: "DELETE", path: "/api/v4/projects/x/repository/tree", allow: false},
	}
	for _, c := range cases {
		got, err := set.Evaluate(Request{Method: c.method, Path: c.path}, c.identity)
		require.NoError(t, err)
		require.Equal(t, c.allow, got.Allowed, "%s %s", c.method, c.path)
		require.Equal(t, []string{"reader"}, got.EvaluatedRoles, "%s %s", c.method, c.path)
		if c.allow {
			require.Equal(t, c.vars, got.Allow.Vars)
			require.Nil(t, got.Deny, "%s %s", c.method, c.path)
		} else {
			require.Equal(t, DenyNotAllowed, got.Deny.Kind, "%s %s", c.method, c.path)
		}
	}
}

// TestRoleSetUnionAcrossRoles pins the union over several roles: a request is
// allowed if any rule in any role matches, the matching role's allow_code
// surfaces, and every evaluated role is reported regardless of which one
// matched.
func TestRoleSetUnionAcrossRoles(t *testing.T) {
	set, err := CompileRoles([]Role{
		{
			Name: "reader",
			Resources: []Rule{ruleFromYAML(t, `
paths: ["/api/v4/projects/{project}/repository/**"]
methods: [GET]
allow_code: reader_grant
`)},
		},
		{
			Name: "writer",
			Resources: []Rule{ruleFromYAML(t, `
paths: ["/api/v4/projects/{project}/repository/**"]
methods: [POST]
allow_code: writer_grant
`)},
		},
	})
	require.NoError(t, err)

	const path = "/api/v4/projects/x/repository/tree"
	roles := []string{"reader", "writer"}

	// A GET matches the reader role; the reader's allow_code surfaces.
	got, err := set.Evaluate(Request{Method: "GET", Path: path}, Identity{})
	require.NoError(t, err)
	require.True(t, got.Allowed)
	require.Equal(t, "reader_grant", got.Allow.Code)
	require.Equal(t, roles, got.EvaluatedRoles)

	// A POST matches the writer role; the writer's allow_code surfaces.
	got, err = set.Evaluate(Request{Method: "POST", Path: path}, Identity{})
	require.NoError(t, err)
	require.True(t, got.Allowed)
	require.Equal(t, "writer_grant", got.Allow.Code)
	require.Equal(t, roles, got.EvaluatedRoles)

	// A DELETE matches no role: a not-allowed deny that still reports both
	// evaluated roles.
	got, err = set.Evaluate(Request{Method: "DELETE", Path: path}, Identity{})
	require.NoError(t, err)
	require.False(t, got.Allowed)
	require.Equal(t, DenyNotAllowed, got.Deny.Kind)
	require.Equal(t, roles, got.EvaluatedRoles)
}

// TestRoleSetInvalidRequest pins that a malformed or unsafe path is denied with
// DenyInvalidRequest before any rule runs, distinct from a well-formed request
// that simply matches no rule.
func TestRoleSetInvalidRequest(t *testing.T) {
	set, err := CompileRoles([]Role{{
		Name: "self",
		Resources: []Rule{ruleFromYAML(t, `
paths: ["/api/v4/user"]
methods: [GET]
`)},
	}})
	require.NoError(t, err)

	roles := []string{"self"}
	for _, path := range []string{"/api/v4/../secret", "/api/v4//user", "/api/v4/user/\x00"} {
		got, err := set.Evaluate(Request{Method: "GET", Path: path}, Identity{})
		require.NoError(t, err)
		require.False(t, got.Allowed, path)
		require.Equal(t, DenyInvalidRequest, got.Deny.Kind, path)
		require.Equal(t, roles, got.EvaluatedRoles, path)
	}

	// A well-formed path that no rule matches is not-allowed, not invalid.
	got, err := set.Evaluate(Request{Method: "GET", Path: "/api/v4/groups"}, Identity{})
	require.NoError(t, err)
	require.Equal(t, DenyNotAllowed, got.Deny.Kind)
}

// TestRoleSetMisconfiguredDefaultDeny pins that an empty EvaluatedRoles on a
// deny marks the case where no role carried any app_resources, as opposed to a
// request a granting role did not match.
func TestRoleSetMisconfiguredDefaultDeny(t *testing.T) {
	set := RoleSet{}
	got, err := set.Evaluate(Request{Method: "GET", Path: "/api/v4/user"}, Identity{})
	require.NoError(t, err)
	require.False(t, got.Allowed)
	require.Equal(t, DenyNotAllowed, got.Deny.Kind)
	require.Empty(t, got.EvaluatedRoles)
}

// TestEncodedSlashCapture pins the GitLab shape: capture_encoded binds an
// encoded project id as one raw segment, while a plain capture rejects the same
// encoded segment and a plain glob never spans it.
func TestEncodedSlashCapture(t *testing.T) {
	// capture_encoded, the sole per-segment opt-in, admits both a plain id and
	// an encoded id, binding the decoded value either way.
	c, err := compileExpression(`path.match(literal("api/v4/projects", capture_encoded("project", set("/"), greedy())))`)
	require.NoError(t, err)

	got, err := c.Evaluate(Request{Method: "GET", Path: "/api/v4/projects/mygroup%2Fmyproject/issues"}, Identity{})
	require.NoError(t, err)
	require.True(t, got.Allowed)
	require.Equal(t, "mygroup/myproject", got.Allow.Vars["project"], "encoded id binds the decoded value as one segment")

	got, err = c.Evaluate(Request{Method: "GET", Path: "/api/v4/projects/123/issues"}, Identity{})
	require.NoError(t, err)
	require.True(t, got.Allowed)
	require.Equal(t, "123", got.Allow.Vars["project"], "plain id matches the same node")

	// A plain capture is safe-only: it rejects a token carrying a percent byte,
	// so the encoded id does not match and the rule denies rather than binding
	// the encoded value. Only an encoded node admits it.
	cp, err := compileExpression(`path.match(literal("api/v4/projects", capture("project", greedy())))`)
	require.NoError(t, err)
	gotPlain, err := cp.Evaluate(Request{Method: "GET", Path: "/api/v4/projects/mygroup%2Fmyproject/issues"}, Identity{})
	require.NoError(t, err)
	require.False(t, gotPlain.Allowed, "a plain capture rejects an encoded slash")
}

// TestDoubleEncodedSlashIsInvalid pins that a double-encoded slash (%252F) is
// rejected at tokenize, since its first escape is %25, not the encoded
// separator, so it never reaches a matcher even one that admits an encoded
// slash.
func TestDoubleEncodedSlashIsInvalid(t *testing.T) {
	set, err := CompileRoles([]Role{{
		Name: "developer",
		Expressions: []string{
			`path.match(literal("api/v4/projects", capture_encoded("project", set("/"), greedy())))`,
		},
	}})
	require.NoError(t, err)
	got, err := set.Evaluate(Request{Method: "GET", Path: "/api/v4/projects/a%252Fb/issues"}, Identity{})
	require.NoError(t, err)
	require.False(t, got.Allowed)
	require.Equal(t, DenyInvalidRequest, got.Deny.Kind)
}

// TestResourcesAndExpressionsCoexist pins that a role may carry both
// app_resources and app_resources_expressions, the parallel of node_labels and
// node_labels_expression. The two are an additive union: a request that matches
// either field is allowed.
func TestResourcesAndExpressionsCoexist(t *testing.T) {
	set, err := CompileRoles([]Role{{
		Name: "mixed",
		Resources: []Rule{ruleFromYAML(t, `
paths: ["/api/v4/user"]
methods: [GET]
`)},
		Expressions: []string{
			`path.match(literal("api/v4/projects", capture_encoded("project", set("/"), greedy())))`,
		},
	}})
	require.NoError(t, err)

	// The sugared rule grants the user endpoint.
	got, err := set.Evaluate(Request{Method: "GET", Path: "/api/v4/user"}, Identity{})
	require.NoError(t, err)
	require.True(t, got.Allowed)

	// The expression rule grants an encoded project id the sugar cannot express.
	got, err = set.Evaluate(Request{Method: "GET", Path: "/api/v4/projects/mygroup%2Fmyproject/issues"}, Identity{})
	require.NoError(t, err)
	require.True(t, got.Allowed)
	require.Equal(t, "mygroup/myproject", got.Allow.Vars["project"])

	// A path neither field grants is denied.
	got, err = set.Evaluate(Request{Method: "GET", Path: "/api/v4/groups"}, Identity{})
	require.NoError(t, err)
	require.False(t, got.Allowed)
}

func TestCompileRejectsEmptyRule(t *testing.T) {
	_, err := Rule{}.Compile()
	require.Error(t, err)
}

// TestNodeToSourceContractsLiterals checks that a run of single-child literals
// renders as one slash-joined literal, while globs, captures, and branches
// stop the contraction. The Literal constructor splits the text back on "/", so
// the contracted source parses to the same tree.
func TestNodeToSourceContractsLiterals(t *testing.T) {
	for _, tc := range []struct {
		name string
		node *Node
		want string
	}{
		{
			name: "literal chain contracts",
			node: Literal("api", Literal("v4", Literal("health"))),
			want: `literal("api/v4/health")`,
		},
		{
			name: "trailing greedy stays a child",
			node: Literal("api", Literal("v4", Greedy())),
			want: `literal("api/v4", greedy())`,
		},
		{
			name: "capture ends the run",
			node: Literal("api", Literal("v4", Capture("project", Greedy()))),
			want: `literal("api/v4", capture("project", greedy()))`,
		},
		{
			name: "glob is not contracted",
			node: Literal("data", Glob(Capture("letter", Greedy()))),
			want: `literal("data", glob(capture("letter", greedy())))`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, nodeToSource(tc.node))
		})
	}
}

// TestWhereByteCap pins the 1 KiB cap on a sugared where clause. A clause at the
// cap compiles; one byte over is a load error, so an unbounded condition cannot
// ride into the evaluator or the audit log.
func TestWhereByteCap(t *testing.T) {
	// A where of exactly maxWhereBytes built from a long disjunction of identity
	// reads compiles, and one byte longer does not. The clause is padded with
	// spaces, which the parser tolerates, so only the length is under test.
	atCap := "user.name == \"x\"" + strings.Repeat(" ", maxWhereBytes-len("user.name == \"x\""))
	require.Len(t, atCap, maxWhereBytes)
	_, err := Rule{Paths: []string{"/api/**"}, Where: atCap}.Compile()
	require.NoError(t, err, "a where at the cap compiles")

	overCap := atCap + " "
	_, err = Rule{Paths: []string{"/api/**"}, Where: overCap}.Compile()
	require.Error(t, err)
	require.Contains(t, err.Error(), "over the")
}

// TestExpressionByteCap pins the 4 KiB cap on one app_resources_expressions
// entry. An entry at the cap compiles; one byte over is a load error.
func TestExpressionByteCap(t *testing.T) {
	atCap := "path.match(literal(\"api\", greedy()))"
	atCap += strings.Repeat(" ", maxExpressionBytes-len(atCap))
	require.Len(t, atCap, maxExpressionBytes)
	_, err := compileExpression(atCap)
	require.NoError(t, err, "an expression at the cap compiles")

	_, err = compileExpression(atCap + " ")
	require.Error(t, err)
	require.Contains(t, err.Error(), "over the")
}

// TestPercentInPlainLiteralRejected pins that a "%" in a plain literal is a load
// error on both surfaces: the predicate literal() and the sugared paths string.
// A plain literal pins exact bytes and can never match an encoded char, so a
// silent dead rule is rejected and the author is steered to encoded_literal or
// the {name:/} sugar.
func TestPercentInPlainLiteralRejected(t *testing.T) {
	_, err := compileExpression(`path.match(literal("a%2Fb"))`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "contains %")

	_, err = Rule{Paths: []string{"/a%2Fb"}}.Compile()
	require.Error(t, err)
	require.Contains(t, err.Error(), "contains %")

	// encoded_literal, the steer, takes the decoded value and compiles.
	_, err = compileExpression(`path.match(encoded_literal("a/b", set("/")))`)
	require.NoError(t, err)
}

// TestAllowReasonNeedsCode pins that an allow_reason with no allow_code is a
// load error, symmetric to the deny side rejecting a reason hint with no code.
func TestAllowReasonNeedsCode(t *testing.T) {
	_, err := Rule{Paths: []string{"/api/**"}, AllowReason: "public"}.Compile()
	require.Error(t, err)
	require.Contains(t, err.Error(), "allow_reason set without allow_code")

	// A reason paired with a code compiles.
	_, err = Rule{Paths: []string{"/api/**"}, AllowCode: "public_api", AllowReason: "public"}.Compile()
	require.NoError(t, err)
}

// TestRolesSortedByName pins the deterministic role order: CompileRoles sorts
// contributing roles by name, so the evaluated-role order and the recorded
// allow_code do not depend on the input slice order. Two roles grant the same
// path with different codes; whichever sorts first by name wins, no matter how
// the caller ordered them.
func TestRolesSortedByName(t *testing.T) {
	zeta := Role{Name: "zeta", Resources: []Rule{{Paths: []string{"/api/**"}, AllowCode: "from_zeta"}}}
	alpha := Role{Name: "alpha", Resources: []Rule{{Paths: []string{"/api/**"}, AllowCode: "from_alpha"}}}

	for _, order := range [][]Role{{zeta, alpha}, {alpha, zeta}} {
		set, err := CompileRoles(order)
		require.NoError(t, err)
		require.Equal(t, []string{"alpha", "zeta"}, set.EvaluatedRoles(),
			"evaluated roles are sorted by name regardless of input order")

		got, err := set.Evaluate(Request{Method: "GET", Path: "/api/v4/health"}, Identity{})
		require.NoError(t, err)
		require.True(t, got.Allowed)
		require.Equal(t, "from_alpha", got.Allow.Code,
			"the lexicographically first role's allow_code wins, deterministically")
	}
}
