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
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// ruleFromYAML unmarshals a single rule from YAML. The test surface is YAML on
// purpose: the cases below are written in the exact form an author would put
// under app_resources, so the test doubles as a worked example.
func ruleFromYAML(t *testing.T, doc string) Rule {
	t.Helper()
	var r Rule
	require.NoError(t, yaml.Unmarshal([]byte(doc), &r))
	return r
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
			// trailing empty literal that matches the trailing empty segment a
			// request path produces, so the slashed pattern matches only the
			// slashed request, not the bare one.
			name: "trailing slash path",
			sugared: `
paths: ["/api/v4/health/"]
methods: [GET]
`,
			desugared: `
pred: |
  path.match(literal("api/v4/health/")) &&
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
			// a single empty segment that matches only the root request "/".
			name: "bare root path",
			sugared: `
paths: ["/"]
methods: [GET]
`,
			desugared: `
pred: |
  path.match(literal("")) &&
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
			desugared, err := ruleFromYAML(t, sc.desugared).Compile()
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

// TestRuleSetUnion pins the additive OR-union: a request is allowed if any
// rule matches, and captures come from the matching rule.
func TestRuleSetUnion(t *testing.T) {
	rules := []Rule{
		ruleFromYAML(t, `
paths: ["/api/v4/projects/{project}/repository/**"]
methods: [GET]
`),
		ruleFromYAML(t, `
paths: ["/api/v4/user"]
methods: [GET]
`),
	}
	set, err := CompileRules(rules)
	require.NoError(t, err)

	roles := []string{"reader", "self"}
	cases := []probe{
		{method: "GET", path: "/api/v4/projects/x/repository/tree", allow: true, vars: map[string]string{"project": "x"}},
		{method: "GET", path: "/api/v4/user", allow: true, vars: map[string]string{}},
		{method: "GET", path: "/api/v4/groups/y", allow: false},
		{method: "DELETE", path: "/api/v4/projects/x/repository/tree", allow: false},
	}
	for _, c := range cases {
		got, err := set.Evaluate(Request{Method: c.method, Path: c.path}, c.identity, roles)
		require.NoError(t, err)
		require.Equal(t, c.allow, got.Allowed, "%s %s", c.method, c.path)
		// Every decision carries the evaluated roles, and a deny that reached
		// the rules carries the not-allowed reason rather than invalid-request.
		require.Equal(t, roles, got.EvaluatedRoles, "%s %s", c.method, c.path)
		if c.allow {
			require.Equal(t, c.vars, got.Allow.Vars)
			require.Nil(t, got.Deny, "%s %s", c.method, c.path)
		} else {
			require.Equal(t, DenyNotAllowed, got.Deny.Kind, "%s %s", c.method, c.path)
		}
	}
}

// TestRuleSetInvalidRequest pins that a malformed or unsafe path is denied with
// DenyInvalidRequest before any rule runs, distinct from a well-formed request
// that simply matches no rule.
func TestRuleSetInvalidRequest(t *testing.T) {
	rules := []Rule{
		ruleFromYAML(t, `
paths: ["/api/v4/user"]
methods: [GET]
`),
	}
	set, err := CompileRules(rules)
	require.NoError(t, err)

	roles := []string{"self"}
	for _, path := range []string{"/api/v4/../secret", "/api/v4//user", "/api/v4/user/\x00"} {
		got, err := set.Evaluate(Request{Method: "GET", Path: path}, Identity{}, roles)
		require.NoError(t, err)
		require.False(t, got.Allowed, path)
		require.Equal(t, DenyInvalidRequest, got.Deny.Kind, path)
		require.Equal(t, roles, got.EvaluatedRoles, path)
	}

	// A well-formed path that no rule matches is not-allowed, not invalid.
	got, err := set.Evaluate(Request{Method: "GET", Path: "/api/v4/groups"}, Identity{}, roles)
	require.NoError(t, err)
	require.Equal(t, DenyNotAllowed, got.Deny.Kind)
}

// TestRuleSetMisconfiguredDefaultDeny pins that an empty EvaluatedRoles on a
// deny marks the case where no role carried any app_resources, as opposed to a
// request a granting role did not match.
func TestRuleSetMisconfiguredDefaultDeny(t *testing.T) {
	set := RuleSet{}
	got, err := set.Evaluate(Request{Method: "GET", Path: "/api/v4/user"}, Identity{}, nil)
	require.NoError(t, err)
	require.False(t, got.Allowed)
	require.Equal(t, DenyNotAllowed, got.Deny.Kind)
	require.Empty(t, got.EvaluatedRoles)
}

// TestURLDecodingFromYAML pins that the url_decoding knob parses from YAML and
// changes how a percent-encoded path tokenizes. The default rejects percent
// bytes; allow_percent plus a decode pass admits and decodes them.
func TestURLDecodingFromYAML(t *testing.T) {
	// GitLab keeps an encoded slash as one project-id segment: decode 0,
	// percent allowed. The capture binds the whole encoded id.
	gitlab := ruleFromYAML(t, `
paths: ["/api/v4/projects/{project}/**"]
methods: [GET]
url_decoding:
  allow_percent: true
  decode_iterations: 0
`)
	compiled, err := gitlab.Compile()
	require.NoError(t, err)
	got, err := compiled.Evaluate(Request{
		Method: "GET",
		Path:   "/api/v4/projects/group%2Frepo/repository/tree",
	}, Identity{})
	require.NoError(t, err)
	require.True(t, got.Allowed)
	require.Equal(t, "group%2Frepo", got.Allow.Vars["project"], "encoded slash stays one segment")

	// The strict default rejects the same request: a percent byte is not
	// admitted, so the path cannot tokenize and nothing matches.
	strict := ruleFromYAML(t, `
paths: ["/api/v4/projects/{project}/**"]
methods: [GET]
`)
	compiledStrict, err := strict.Compile()
	require.NoError(t, err)
	gotStrict, err := compiledStrict.Evaluate(Request{
		Method: "GET",
		Path:   "/api/v4/projects/group%2Frepo/repository/tree",
	}, Identity{})
	require.NoError(t, err)
	require.False(t, gotStrict.Allowed, "strict default rejects percent-encoding")
}

func TestCompileRejectsBothSurfaces(t *testing.T) {
	_, err := Rule{
		Paths: []string{"/api/**"},
		Pred:  `path.match(greedy())`,
	}.Compile()
	require.Error(t, err)
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
