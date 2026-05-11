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
	"testing"

	"github.com/stretchr/testify/require"
)

func allowPolicy(name, path string, methods ...string) Policy {
	rule := Rule{Paths: []*PathMatcher{MustCompilePath(path)}, Methods: methods, ReasonCode: name, Reason: name}
	return Policy{Name: name, Allow: []Rule{rule}}
}

func denyPolicy(name, path string, methods ...string) Policy {
	rule := Rule{Paths: []*PathMatcher{MustCompilePath(path)}, Methods: methods, ReasonCode: name, Reason: name}
	return Policy{Name: name, Deny: []Rule{rule}}
}

func defaultDenyPolicy(name, path string) Policy {
	p, err := Compile(Spec{
		Name: name,
		Deny: []RuleSpec{{Paths: []string{path}}},
	})
	if err != nil {
		panic(err)
	}
	return p
}

func TestEvaluate_TruthTable(t *testing.T) {
	allow := allowPolicy("allow-foo", "/foo")
	deny := denyPolicy("deny-foo", "/foo")
	tests := []struct {
		name       string
		policies   []Policy
		path       string
		wantAllow  bool
		wantReason string
	}{
		{name: "none", policies: nil, path: "/foo", wantAllow: true},
		{name: "only deny, match", policies: []Policy{deny}, path: "/foo", wantAllow: false, wantReason: "deny-foo"},
		{name: "only deny, no match", policies: []Policy{deny}, path: "/bar", wantAllow: true},
		{name: "only allow, match", policies: []Policy{allow}, path: "/foo", wantAllow: true, wantReason: "allow-foo"},
		{name: "only allow, no match", policies: []Policy{allow}, path: "/bar", wantAllow: false, wantReason: ReasonNoMatchingAllow},
		{name: "deny first, both match", policies: []Policy{deny, allow}, path: "/foo", wantAllow: false, wantReason: "deny-foo"},
		// Deny wins even when placed after a matching allow.
		{name: "allow first, both match", policies: []Policy{allow, deny}, path: "/foo", wantAllow: false, wantReason: "deny-foo"},
		{
			name: "deny no match, allow match",
			policies: []Policy{
				denyPolicy("deny-bar", "/bar"),
				allow,
			},
			path: "/foo", wantAllow: true, wantReason: "allow-foo",
		},
		{
			name: "deny no match, allow no match",
			policies: []Policy{
				denyPolicy("deny-bar", "/bar"),
				allow,
			},
			path: "/baz", wantAllow: false, wantReason: ReasonNoMatchingAllow,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := Evaluate(tc.policies, Request{Method: "GET", Path: tc.path})
			require.Equal(t, tc.wantAllow, d.Allow, "decision: %+v", d)
			if tc.wantReason != "" {
				require.Equal(t, tc.wantReason, d.ReasonCode)
			}
		})
	}
}

func TestEvaluate_MixedPolicyAllowAndDeny(t *testing.T) {
	p, err := Compile(Spec{
		Name: "mixed",
		Allow: []RuleSpec{{
			Paths: []string{"/api/**"},
		}},
		Deny: []RuleSpec{{
			Paths: []string{"/api/admin/**"},
		}},
	})
	require.NoError(t, err)

	t.Run("allow path matches", func(t *testing.T) {
		d := Evaluate([]Policy{p}, Request{Method: "GET", Path: "/api/users"})
		require.True(t, d.Allow)
	})
	t.Run("deny wins over allow", func(t *testing.T) {
		d := Evaluate([]Policy{p}, Request{Method: "GET", Path: "/api/admin/users"})
		require.False(t, d.Allow)
		require.Equal(t, ReasonExplicitDeny, d.ReasonCode)
	})
}

func TestEvaluate_DefaultDenyReason(t *testing.T) {
	deny := defaultDenyPolicy("no-admin", "/admin")
	d := Evaluate([]Policy{deny}, Request{Method: "GET", Path: "/admin"})
	require.False(t, d.Allow)
	require.Equal(t, ReasonExplicitDeny, d.ReasonCode)
}

func TestEvaluate_PoliciesEvaluatedOrder(t *testing.T) {
	policies := []Policy{
		denyPolicy("z-deny", "/never"),
		allowPolicy("a-allow", "/yes"),
	}
	d := Evaluate(policies, Request{Method: "GET", Path: "/yes"})
	require.Equal(t, []string{"z-deny", "a-allow"}, d.EvalSummary.PoliciesEvaluated)
}

func TestEvaluate_WorkedExample(t *testing.T) {
	gitlabNoAdmin := Policy{
		Name: "gitlab-no-project-admin",
		Deny: []Rule{{
			Paths: []*PathMatcher{
				MustCompilePath("/api/v4/projects/*/variables/**"),
				MustCompilePath("/api/v4/projects/*/hooks/**"),
			},
			ReasonCode: "project_admin_blocked",
			Reason:     "Project admin endpoints are platform-team only.",
		}},
	}
	gitlabRepoRead := Policy{
		Name: "gitlab-repo-read",
		Allow: []Rule{{
			Paths:      []*PathMatcher{MustCompilePath("/api/v4/projects/*/repository/**")},
			Methods:    []string{"GET", "HEAD"},
			ReasonCode: "gitlab-repo-read",
			Reason:     "gitlab-repo-read",
		}},
	}
	gitlabMR := Policy{
		Name: "gitlab-mr-collaborate",
		Allow: []Rule{{
			Paths:      []*PathMatcher{MustCompilePath("/api/v4/projects/*/merge_requests/**")},
			Methods:    []string{"GET", "HEAD", "POST", "PUT"},
			ReasonCode: "gitlab-mr-collaborate",
			Reason:     "gitlab-mr-collaborate",
		}},
	}
	policies := []Policy{gitlabNoAdmin, gitlabRepoRead, gitlabMR}

	t.Run("encoded slash deny", func(t *testing.T) {
		_, err := Normalize("/api/v4/projects/org%2Fapi/repository/files/README.md")
		require.Error(t, err)
		code, ok := IsNormalizeError(err)
		require.True(t, ok)
		require.Equal(t, ReasonEncodedSlashInSegment, code)
	})
	t.Run("allow via repo-read", func(t *testing.T) {
		path, err := Normalize("/api/v4/projects/123/repository/files/README.md")
		require.NoError(t, err)
		d := Evaluate(policies, Request{Method: "GET", Path: path})
		require.True(t, d.Allow)
		require.Equal(t, "gitlab-repo-read", d.ReasonCode)
	})
	t.Run("deny via project-admin", func(t *testing.T) {
		path, err := Normalize("/api/v4/projects/123/hooks/4")
		require.NoError(t, err)
		d := Evaluate(policies, Request{Method: "DELETE", Path: path})
		require.False(t, d.Allow)
		require.Equal(t, "project_admin_blocked", d.ReasonCode)
	})
	t.Run("deny via no allow match", func(t *testing.T) {
		path, err := Normalize("/api/v4/projects/789/repository/files/README.md")
		require.NoError(t, err)
		d := Evaluate(policies, Request{Method: "POST", Path: path})
		require.False(t, d.Allow)
		require.Equal(t, ReasonNoMatchingAllow, d.ReasonCode)
	})
}

func TestEvaluate_Where(t *testing.T) {
	pred, err := CompilePredicate(`path.username == user.name`)
	require.NoError(t, err)
	p := Policy{
		Name: "own-subtree",
		Allow: []Rule{{
			Paths:      []*PathMatcher{MustCompilePath("/api/users/{username}/**")},
			Where:      pred,
			ReasonCode: "own-subtree",
			Reason:     "own-subtree",
		}},
	}
	d := Evaluate([]Policy{p}, Request{
		Method:   "GET",
		Path:     "/api/users/alice/profile",
		UserName: "alice",
	})
	require.True(t, d.Allow)
	require.Equal(t, "alice", d.BoundVars["username"])

	// Path captures bob but user is alice; the allow rule must not match.
	d = Evaluate([]Policy{p}, Request{
		Method:   "POST",
		Path:     "/api/users/bob/data",
		UserName: "alice",
	})
	require.False(t, d.Allow)
	require.Equal(t, ReasonNoMatchingAllow, d.ReasonCode)
}

func TestEvaluate_DenyPredicateError(t *testing.T) {
	deny := Policy{
		Name: "errs",
		Deny: []Rule{{
			Paths: []*PathMatcher{MustCompilePath("/x/**")},
			Where: errPredicate{},
		}},
	}
	d := Evaluate([]Policy{deny}, Request{Method: "GET", Path: "/x/1"})
	require.False(t, d.Allow)
	require.Equal(t, ReasonPredicateError, d.ReasonCode)
}

func TestEvaluate_AllowPredicateError(t *testing.T) {
	allow := Policy{
		Name: "errs",
		Allow: []Rule{{
			Paths: []*PathMatcher{MustCompilePath("/x/**")},
			Where: errPredicate{},
		}},
	}
	d := Evaluate([]Policy{allow}, Request{Method: "GET", Path: "/x/1"})
	require.False(t, d.Allow)
	require.Equal(t, ReasonNoMatchingAllow, d.ReasonCode)
}

func TestEvaluate_NormalizeFailureSurfaces(t *testing.T) {
	d := Evaluate(nil, Request{
		Method:        "GET",
		Path:          "",
		NormalizeCode: ReasonEncodedSlashInSegment,
		NormalizeErr:  "encoded path separator decoded into a new segment",
	})
	require.False(t, d.Allow)
	require.Equal(t, ReasonEncodedSlashInSegment, d.ReasonCode)
}

type errPredicate struct{}

func (errPredicate) Evaluate(Env) (bool, error) {
	return false, errBoom
}

var errBoom = stringErr("boom")

type stringErr string

func (s stringErr) Error() string { return string(s) }
