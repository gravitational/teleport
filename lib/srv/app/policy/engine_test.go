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

package policy

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// mustCompile compiles a path pattern for a test rule and fails the test
// if the pattern is malformed.
func mustCompile(t *testing.T, patterns ...string) []*Matcher {
	t.Helper()
	out := make([]*Matcher, len(patterns))
	for i, p := range patterns {
		m, err := Compile(p)
		require.NoError(t, err)
		out[i] = m
	}
	return out
}

// TestEvaluate pins the decision model. The three named seed cases
// (conformance cases 01, 04, 05) are reproduced here in their resolved
// form: the rule provenance and attached-policy list are what the loader
// will hand the engine, so these rows exercise the engine and its JSON
// projection without depending on the loader. The remaining rows cover
// the no-policies-attached branch in both RequirePolicy states.
func TestEvaluate(t *testing.T) {
	// inlineHealth is the resolved form of cases 01 and 05: one inline
	// policy "health" with a single GET /api/v4/health allow rule.
	inlineHealth := Input{
		AttachedPolicies: []string{"health"},
		Rules: []Rule{{
			Paths:     mustCompile(t, "/api/v4/health"),
			Methods:   []string{"GET"},
			PolicyRef: "health",
		}},
	}
	// boundRepoRead is the resolved form of case 04: a standalone policy
	// reached through the "gitlab" binding, with an audit_code and a
	// capture.
	boundRepoRead := Input{
		AttachedPolicies: []string{"gitlab/gitlab-repo-read"},
		Rules: []Rule{{
			Paths:     mustCompile(t, "/api/v4/projects/{project}/repository/**"),
			Methods:   []string{"GET", "HEAD"},
			PolicyRef: "gitlab/gitlab-repo-read/repo_read",
			AuditCode: "repo_read",
			Binding:   "gitlab",
		}},
	}

	withRequest := func(base Input, app, method, path string) Input {
		base.Request = Request{App: app, Method: method, Path: path}
		return base
	}

	tests := []struct {
		name string
		in   Input
		want DecisionJSON
	}{
		// Case 01: exact path match, GET allowed, no audit_code.
		{
			name: "01/exact path allow",
			in:   withRequest(inlineHealth, "gitlab", "GET", "/api/v4/health"),
			want: DecisionJSON{
				Event:            EventAllow,
				Method:           "GET",
				Path:             "/api/v4/health",
				App:              "gitlab",
				PolicyRef:        "health",
				PathCaptures:     map[string]string{},
				AttachedPolicies: []string{"health"},
			},
		},
		{
			name: "01/method mismatch deny",
			in:   withRequest(inlineHealth, "gitlab", "POST", "/api/v4/health"),
			want: DecisionJSON{
				Event:            EventDeny,
				Method:           "POST",
				Path:             "/api/v4/health",
				App:              "gitlab",
				ReasonCode:       ReasonNoMatchingAllow,
				Reason:           DefaultDenyReason,
				PolicyRef:        ReasonNoMatchingAllow,
				PathCaptures:     map[string]string{},
				AttachedPolicies: []string{"health"},
			},
		},
		{
			name: "01/sibling path deny",
			in:   withRequest(inlineHealth, "gitlab", "GET", "/api/v4/health/details"),
			want: DecisionJSON{
				Event:            EventDeny,
				Method:           "GET",
				Path:             "/api/v4/health/details",
				App:              "gitlab",
				ReasonCode:       ReasonNoMatchingAllow,
				Reason:           DefaultDenyReason,
				PolicyRef:        ReasonNoMatchingAllow,
				PathCaptures:     map[string]string{},
				AttachedPolicies: []string{"health"},
			},
		},
		// Case 04: binding-resolved standalone policy, multiple methods,
		// capture, audit_code, method mismatch.
		{
			name: "04/GET allow with capture and binding",
			in:   withRequest(boundRepoRead, "gitlab", "GET", "/api/v4/projects/123/repository/files/README"),
			want: DecisionJSON{
				Event:            EventAllow,
				Method:           "GET",
				Path:             "/api/v4/projects/123/repository/files/README",
				App:              "gitlab",
				Binding:          "gitlab",
				AuditCode:        "repo_read",
				PolicyRef:        "gitlab/gitlab-repo-read/repo_read",
				PathCaptures:     map[string]string{"project": "123"},
				AttachedPolicies: []string{"gitlab/gitlab-repo-read"},
			},
		},
		{
			name: "04/HEAD allow",
			in:   withRequest(boundRepoRead, "gitlab", "HEAD", "/api/v4/projects/123/repository/files/README"),
			want: DecisionJSON{
				Event:            EventAllow,
				Method:           "HEAD",
				Path:             "/api/v4/projects/123/repository/files/README",
				App:              "gitlab",
				Binding:          "gitlab",
				AuditCode:        "repo_read",
				PolicyRef:        "gitlab/gitlab-repo-read/repo_read",
				PathCaptures:     map[string]string{"project": "123"},
				AttachedPolicies: []string{"gitlab/gitlab-repo-read"},
			},
		},
		{
			name: "04/DELETE method mismatch deny",
			in:   withRequest(boundRepoRead, "gitlab", "DELETE", "/api/v4/projects/123/repository/files/README"),
			want: DecisionJSON{
				Event:            EventDeny,
				Method:           "DELETE",
				Path:             "/api/v4/projects/123/repository/files/README",
				App:              "gitlab",
				ReasonCode:       ReasonNoMatchingAllow,
				Reason:           DefaultDenyReason,
				PolicyRef:        ReasonNoMatchingAllow,
				PathCaptures:     map[string]string{},
				AttachedPolicies: []string{"gitlab/gitlab-repo-read"},
			},
		},
		// Case 05: same inline policy denies a non-matching path and
		// allows the matching one (default-deny once a policy attaches).
		{
			name: "05/non-matching path deny",
			in:   withRequest(inlineHealth, "gitlab", "GET", "/api/v4/admin/users"),
			want: DecisionJSON{
				Event:            EventDeny,
				Method:           "GET",
				Path:             "/api/v4/admin/users",
				App:              "gitlab",
				ReasonCode:       ReasonNoMatchingAllow,
				Reason:           DefaultDenyReason,
				PolicyRef:        ReasonNoMatchingAllow,
				PathCaptures:     map[string]string{},
				AttachedPolicies: []string{"health"},
			},
		},
		{
			name: "05/matching path allow",
			in:   withRequest(inlineHealth, "gitlab", "GET", "/api/v4/health"),
			want: DecisionJSON{
				Event:            EventAllow,
				Method:           "GET",
				Path:             "/api/v4/health",
				App:              "gitlab",
				PolicyRef:        "health",
				PathCaptures:     map[string]string{},
				AttachedPolicies: []string{"health"},
			},
		},
		// A lowercase method does not match an upper-case stored method:
		// the loader uppercases, so matching is case-sensitive.
		{
			name: "lowercase method denies",
			in:   withRequest(inlineHealth, "gitlab", "get", "/api/v4/health"),
			want: DecisionJSON{
				Event:            EventDeny,
				Method:           "get",
				Path:             "/api/v4/health",
				App:              "gitlab",
				ReasonCode:       ReasonNoMatchingAllow,
				Reason:           DefaultDenyReason,
				PolicyRef:        ReasonNoMatchingAllow,
				PathCaptures:     map[string]string{},
				AttachedPolicies: []string{"health"},
			},
		},
		// An empty method filter matches any method.
		{
			name: "empty methods matches any method",
			in: Input{
				Request:          Request{App: "gitlab", Method: "PATCH", Path: "/api/v4/health"},
				AttachedPolicies: []string{"any-method"},
				Rules: []Rule{{
					Paths:     mustCompile(t, "/api/v4/health"),
					PolicyRef: "any-method",
				}},
			},
			want: DecisionJSON{
				Event:            EventAllow,
				Method:           "PATCH",
				Path:             "/api/v4/health",
				App:              "gitlab",
				PolicyRef:        "any-method",
				PathCaptures:     map[string]string{},
				AttachedPolicies: []string{"any-method"},
			},
		},
		// An empty path list matches any path.
		{
			name: "empty paths matches any path",
			in: Input{
				Request:          Request{App: "gitlab", Method: "GET", Path: "/anything/at/all"},
				AttachedPolicies: []string{"any-path"},
				Rules: []Rule{{
					Methods:   []string{"GET"},
					PolicyRef: "any-path",
				}},
			},
			want: DecisionJSON{
				Event:            EventAllow,
				Method:           "GET",
				Path:             "/anything/at/all",
				App:              "gitlab",
				PolicyRef:        "any-path",
				PathCaptures:     map[string]string{},
				AttachedPolicies: []string{"any-path"},
			},
		},
		// A rule with several path patterns matches when a later one
		// matches, not only the first.
		{
			name: "second path pattern in a rule matches",
			in: Input{
				Request:          Request{App: "gitlab", Method: "GET", Path: "/api/v4/version"},
				AttachedPolicies: []string{"multi"},
				Rules: []Rule{{
					Paths:     mustCompile(t, "/api/v4/health", "/api/v4/version"),
					Methods:   []string{"GET"},
					PolicyRef: "multi",
				}},
			},
			want: DecisionJSON{
				Event:            EventAllow,
				Method:           "GET",
				Path:             "/api/v4/version",
				App:              "gitlab",
				PolicyRef:        "multi",
				PathCaptures:     map[string]string{},
				AttachedPolicies: []string{"multi"},
			},
		},
		// An attached policy that contributes no rules still default-denies:
		// the no-match branch keys off the attached set, not the rule count.
		{
			name: "attached policy with no usable rules denies",
			in: Input{
				Request:          Request{App: "gitlab", Method: "GET", Path: "/anything"},
				AttachedPolicies: []string{"empty-policy"},
			},
			want: DecisionJSON{
				Event:            EventDeny,
				Method:           "GET",
				Path:             "/anything",
				App:              "gitlab",
				ReasonCode:       ReasonNoMatchingAllow,
				Reason:           DefaultDenyReason,
				PolicyRef:        ReasonNoMatchingAllow,
				PathCaptures:     map[string]string{},
				AttachedPolicies: []string{"empty-policy"},
			},
		},
		// No policies attached: permissive today, deny under enforcement.
		{
			name: "no policies permissive allow",
			in: Input{
				Request: Request{App: "gitlab", Method: "GET", Path: "/anything"},
			},
			want: DecisionJSON{
				Event:            EventAllow,
				Method:           "GET",
				Path:             "/anything",
				App:              "gitlab",
				PathCaptures:     map[string]string{},
				AttachedPolicies: []string{},
			},
		},
		{
			name: "no policies require-policy deny",
			in: Input{
				Request:       Request{App: "gitlab", Method: "GET", Path: "/anything"},
				RequirePolicy: true,
			},
			want: DecisionJSON{
				Event:            EventDeny,
				Method:           "GET",
				Path:             "/anything",
				App:              "gitlab",
				ReasonCode:       ReasonNoPolicyAttached,
				Reason:           DefaultDenyReason,
				PolicyRef:        ReasonNoPolicyAttached,
				PathCaptures:     map[string]string{},
				AttachedPolicies: []string{},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Evaluate(tc.in).JSON()
			require.Equal(t, tc.want, got)
		})
	}
}

// TestEvaluateFirstMatchWins checks that the earliest rule in evaluation
// order decides, and that its provenance, not a later rule's, is the one
// reported.
func TestEvaluateFirstMatchWins(t *testing.T) {
	in := Input{
		Request:          Request{App: "gitlab", Method: "GET", Path: "/api/v4/health"},
		AttachedPolicies: []string{"first", "second"},
		Rules: []Rule{
			{Paths: mustCompile(t, "/api/v4/health"), Methods: []string{"GET"}, PolicyRef: "first", AuditCode: "first_code"},
			{Paths: mustCompile(t, "/api/v4/**"), Methods: []string{"GET"}, PolicyRef: "second", AuditCode: "second_code"},
		},
	}

	dec := Evaluate(in)
	require.True(t, dec.Allow)
	require.Equal(t, "first", dec.PolicyRef)
	require.Equal(t, "first_code", dec.AuditCode)
}

// TestDecisionJSONNormalizesEmptyCollections checks that the projection
// renders empty captures and an empty attached set as a JSON object and
// array rather than null, since the conformance output and audit payload
// both rely on path_captures being {} when a decision has no captures.
func TestDecisionJSONNormalizesEmptyCollections(t *testing.T) {
	out, err := json.Marshal(Decision{Allow: true}.JSON())
	require.NoError(t, err)

	require.Contains(t, string(out), `"path_captures":{}`)
	require.Contains(t, string(out), `"attached_policies":[]`)
	// Fields the decision does not set are omitted, not emitted empty.
	require.NotContains(t, string(out), `"binding"`)
	require.NotContains(t, string(out), `"audit_code"`)
	require.NotContains(t, string(out), `"reason_code"`)

	// A deny emits reason_code and reason.
	denyOut, err := json.Marshal(deny(Decision{}, ReasonNoMatchingAllow).JSON())
	require.NoError(t, err)
	require.Contains(t, string(denyOut), `"reason_code":"`+ReasonNoMatchingAllow+`"`)
	require.Contains(t, string(denyOut), `"reason":"`+DefaultDenyReason+`"`)
}
