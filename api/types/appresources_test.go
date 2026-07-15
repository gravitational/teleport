/*
Copyright 2026 Gravitational, Inc.

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

package types

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func appErrContains(substr string) require.ErrorAssertionFunc {
	return func(t require.TestingT, err error, _ ...any) {
		require.ErrorContains(t, err, substr)
	}
}

func manyPaths(n int) []string {
	paths := make([]string, n)
	for i := range paths {
		paths[i] = "/health"
	}
	return paths
}

func manyMethods(n int) []string {
	methods := make([]string, n)
	for i := range methods {
		methods[i] = "GET"
	}
	return methods
}

func TestRoleAppResourcesValidation(t *testing.T) {
	manyRules := make([]AppResource, maxAppRulesPerRole+1)
	for i := range manyRules {
		manyRules[i] = AppResource{UnsafeAllowAll: true}
	}

	tests := []struct {
		name      string
		version   string
		allow     RoleConditions
		deny      RoleConditions
		assertErr require.ErrorAssertionFunc
	}{
		{
			name:      "v9 unsafe_allow_all alone",
			version:   V9,
			allow:     RoleConditions{AppResources: []AppResource{{UnsafeAllowAll: true}}},
			assertErr: require.NoError,
		},
		{
			name:      "v9 paths not yet supported",
			version:   V9,
			allow:     RoleConditions{AppResources: []AppResource{{Paths: []string{"/health"}}}},
			assertErr: appErrContains("only unsafe_allow_all is supported"),
		},
		{
			name:    "v9 full rule not yet supported",
			version: V9,
			allow: RoleConditions{AppResources: []AppResource{{
				Paths:          []string{"/api/v4/projects/{project}/**"},
				Methods:        []string{"GET", "HEAD"},
				Where:          `contains(user.traits["allowed_projects"], vars.project)`,
				AllowCode:      "repo_read",
				AllowReason:    "Read access to the repository API",
				DenyCodeHint:   "project_not_allowed",
				DenyReasonHint: "Project is not in the caller's allowlist",
			}}},
			assertErr: appErrContains("only unsafe_allow_all is supported"),
		},
		{
			name:      "v9 expressions not yet supported",
			version:   V9,
			allow:     RoleConditions{AppResourcesExpressions: []string{`path.match(literal("health"))`}},
			assertErr: appErrContains("not yet supported"),
		},
		{
			// unsafe_allow_all with another field is rejected by the
			// not-yet-supported gate before the combine rule runs.
			name:      "unsafe_allow_all combined with paths",
			version:   V9,
			allow:     RoleConditions{AppResources: []AppResource{{UnsafeAllowAll: true, Paths: []string{"/health"}}}},
			assertErr: appErrContains("only unsafe_allow_all is supported"),
		},
		{
			name:      "over the per-role rule cap",
			version:   V9,
			allow:     RoleConditions{AppResources: manyRules},
			assertErr: appErrContains("at most"),
		},
		{
			name:      "app_resources under deny",
			version:   V9,
			deny:      RoleConditions{AppResources: []AppResource{{Paths: []string{"/health"}}}},
			assertErr: appErrContains("not allowed under deny"),
		},
		{
			name:      "app_resources_expressions under deny",
			version:   V9,
			deny:      RoleConditions{AppResourcesExpressions: []string{`path.match(literal("health"))`}},
			assertErr: appErrContains("not allowed under deny"),
		},
		{
			name:      "app_resources on v8 role",
			version:   V8,
			allow:     RoleConditions{AppResources: []AppResource{{Paths: []string{"/health"}}}},
			assertErr: appErrContains("require role version"),
		},
		{
			name:      "app_resources_expressions on v8 role",
			version:   V8,
			allow:     RoleConditions{AppResourcesExpressions: []string{`path.match(literal("health"))`}},
			assertErr: appErrContains("require role version"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			role := &RoleV6{
				Metadata: Metadata{Name: "test"},
				Version:  test.version,
				Spec:     RoleSpecV6{Allow: test.allow, Deny: test.deny},
			}
			test.assertErr(t, role.CheckAndSetDefaults())
		})
	}
}

// TestValidateAppResources exercises the full structural validation directly.
// CheckAndSetDefaults cannot reach most of these cases yet, since the
// not-yet-supported check rejects every field except unsafe_allow_all first.
// The cases below become reachable as enforcement lands and the supported
// check is relaxed.
func TestValidateAppResources(t *testing.T) {
	tests := []struct {
		name        string
		rules       []AppResource
		expressions []string
		assertErr   require.ErrorAssertionFunc
	}{
		{
			name:      "paths only",
			rules:     []AppResource{{Paths: []string{"/health"}}},
			assertErr: require.NoError,
		},
		{
			name: "full rule with codes and hints",
			rules: []AppResource{{
				Paths:          []string{"/api/v4/projects/{project}/**"},
				Methods:        []string{"GET", "HEAD"},
				Where:          `contains(user.traits["allowed_projects"], vars.project)`,
				AllowCode:      "repo_read",
				AllowReason:    "Read access to the repository API",
				DenyCodeHint:   "project_not_allowed",
				DenyReasonHint: "Project is not in the caller's allowlist",
			}},
			assertErr: require.NoError,
		},
		{
			name:        "expressions only",
			expressions: []string{`path.match(literal("health"))`},
			assertErr:   require.NoError,
		},
		{
			name:      "rule missing paths and unsafe_allow_all",
			rules:     []AppResource{{Methods: []string{"GET"}}},
			assertErr: appErrContains("must set paths or unsafe_allow_all"),
		},
		{
			name:      "unsafe_allow_all combined with paths",
			rules:     []AppResource{{UnsafeAllowAll: true, Paths: []string{"/health"}}},
			assertErr: appErrContains("unsafe_allow_all cannot be combined"),
		},
		{
			name:      "allow_reason without allow_code",
			rules:     []AppResource{{Paths: []string{"/health"}, AllowReason: "why"}},
			assertErr: appErrContains("allow_reason set without allow_code"),
		},
		{
			name:      "deny_reason_hint without deny_code_hint",
			rules:     []AppResource{{Paths: []string{"/health"}, Where: "user.name == \"a\"", DenyReasonHint: "why"}},
			assertErr: appErrContains("deny_reason_hint set without deny_code_hint"),
		},
		{
			name:      "deny_code_hint without where",
			rules:     []AppResource{{Paths: []string{"/health"}, DenyCodeHint: "nope"}},
			assertErr: appErrContains("deny_code_hint set without a where clause"),
		},
		{
			name:      "allow_code with reserved prefix",
			rules:     []AppResource{{Paths: []string{"/health"}, AllowCode: "teleport_read"}},
			assertErr: appErrContains("reserved teleport_ prefix"),
		},
		{
			name:      "deny_code_hint with reserved prefix",
			rules:     []AppResource{{Paths: []string{"/health"}, Where: "user.name == \"a\"", DenyCodeHint: "teleport_nope"}},
			assertErr: appErrContains("reserved teleport_ prefix"),
		},
		{
			name:      "where over the byte cap",
			rules:     []AppResource{{Paths: []string{"/health"}, Where: strings.Repeat("a", maxAppWhereBytes+1)}},
			assertErr: appErrContains("where clause is"),
		},
		{
			name:        "expression over the byte cap",
			expressions: []string{strings.Repeat("a", maxAppExpressionBytes+1)},
			assertErr:   appErrContains("app_resources_expressions[0] is"),
		},
		{
			name:      "over the per-rule path cap",
			rules:     []AppResource{{Paths: manyPaths(maxAppPathsPerRule + 1)}},
			assertErr: appErrContains("path cap"),
		},
		{
			name:      "path over the byte cap",
			rules:     []AppResource{{Paths: []string{strings.Repeat("a", maxAppPathBytes+1)}}},
			assertErr: appErrContains("paths[0] is"),
		},
		{
			name:      "over the per-rule method cap",
			rules:     []AppResource{{Paths: []string{"/health"}, Methods: manyMethods(maxAppMethodsPerRule + 1)}},
			assertErr: appErrContains("method cap"),
		},
		{
			name:      "allow_encoded with an unsupported value",
			rules:     []AppResource{{Paths: []string{"/health"}, AllowEncoded: []string{"?"}}},
			assertErr: appErrContains("only supports the encoded slash"),
		},
		{
			name:      "allow_encoded lists the slash twice",
			rules:     []AppResource{{Paths: []string{"/health"}, AllowEncoded: []string{"/", "/"}}},
			assertErr: appErrContains("more than once"),
		},
		{
			name:      "allow_reason over the byte cap",
			rules:     []AppResource{{Paths: []string{"/health"}, AllowCode: "read", AllowReason: strings.Repeat("a", maxAppReasonBytes+1)}},
			assertErr: appErrContains("allow_reason is"),
		},
		{
			name:      "deny_reason_hint over the byte cap",
			rules:     []AppResource{{Paths: []string{"/health"}, Where: "user.name == \"a\"", DenyCodeHint: "nope", DenyReasonHint: strings.Repeat("a", maxAppReasonBytes+1)}},
			assertErr: appErrContains("deny_reason_hint is"),
		},
		{
			name:      "allow_code over the byte cap",
			rules:     []AppResource{{Paths: []string{"/health"}, AllowCode: strings.Repeat("a", maxAppCodeBytes+1)}},
			assertErr: appErrContains("byte cap"),
		},
		{
			name:      "allow_code with characters outside the charset",
			rules:     []AppResource{{Paths: []string{"/health"}, AllowCode: "Repo_Read"}},
			assertErr: appErrContains("lowercase letters, digits, and underscores"),
		},
		{
			name:      "deny_code_hint with a case-variant reserved prefix",
			rules:     []AppResource{{Paths: []string{"/health"}, Where: "user.name == \"a\"", DenyCodeHint: "Teleport_nope"}},
			assertErr: appErrContains("lowercase letters, digits, and underscores"),
		},
		{
			name:      "allow_reason with control characters",
			rules:     []AppResource{{Paths: []string{"/health"}, AllowCode: "read", AllowReason: "why\x1b[31m"}},
			assertErr: appErrContains("control, format, or separator characters"),
		},
		{
			name:      "allow_reason with a bidi override",
			rules:     []AppResource{{Paths: []string{"/health"}, AllowCode: "read", AllowReason: "why\u202e"}},
			assertErr: appErrContains("control, format, or separator characters"),
		},
		{
			name:      "allow_reason with a line separator",
			rules:     []AppResource{{Paths: []string{"/health"}, AllowCode: "read", AllowReason: "why\u2028"}},
			assertErr: appErrContains("control, format, or separator characters"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.assertErr(t, validateAppResources(test.rules, test.expressions))
		})
	}
}
