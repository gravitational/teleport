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
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAppResourceFields fails when a field is added to or removed from
// AppResource, so unreserving a proto field number cannot slip past the
// checks that decide whether a rule is unrestricted. Any new field
// restricts the rule, so IsAllowAllOnly and the server-side write
// validation must account for it before this list changes.
func TestAppResourceFields(t *testing.T) {
	var fields []string
	for f := range reflect.TypeFor[AppResource]().Fields() {
		if !strings.HasPrefix(f.Name, "XXX_") {
			fields = append(fields, f.Name)
		}
	}
	want := []string{
		"Paths", "Methods", "Where", "AllowEncoded", "AllowAll",
		"AllowCode", "AllowReason", "DenyCodeHint", "DenyReasonHint",
	}
	require.Equal(t, want, fields)
}

// TestIsAllowAllOnly checks that every declared field besides allow_all
// disqualifies a rule from counting as unrestricted. A rule a newer
// version wrote with a restricting field must deny, never widen to
// allow_all, on a version that does not enforce the field.
func TestIsAllowAllOnly(t *testing.T) {
	require.True(t, AppResource{AllowAll: true}.IsAllowAllOnly())
	require.False(t, AppResource{}.IsAllowAllOnly())

	for name, rule := range map[string]AppResource{
		"paths":            {AllowAll: true, Paths: []string{"/api/**"}},
		"methods":          {AllowAll: true, Methods: []string{"GET"}},
		"where":            {AllowAll: true, Where: "true"},
		"allow_encoded":    {AllowAll: true, AllowEncoded: []string{"/"}},
		"allow_code":       {AllowAll: true, AllowCode: "all"},
		"allow_reason":     {AllowAll: true, AllowReason: "All."},
		"deny_code_hint":   {AllowAll: true, DenyCodeHint: "no"},
		"deny_reason_hint": {AllowAll: true, DenyReasonHint: "No."},
		"unknown field":    {AllowAll: true, XXX_unrecognized: []byte{0x50, 0x01}},
	} {
		t.Run(name, func(t *testing.T) {
			require.False(t, rule.IsAllowAllOnly())
		})
	}
}

// TestAppResourcesRequireV9 covers the read-path check in
// CheckAndSetDefaults, which rejects app_resources below role version v9.
// The write-path checks are covered by TestValidateRoleAppResources.
func TestAppResourcesRequireV9(t *testing.T) {
	errContains := func(substr string) require.ErrorAssertionFunc {
		return func(t require.TestingT, err error, _ ...any) {
			require.ErrorContains(t, err, substr)
		}
	}

	tests := []struct {
		name      string
		version   string
		allow     RoleConditions
		deny      RoleConditions
		assertErr require.ErrorAssertionFunc
	}{
		{
			name:      "v9 allow_all",
			version:   V9,
			allow:     RoleConditions{AppResources: []AppResource{{AllowAll: true}}},
			assertErr: require.NoError,
		},
		{
			name:      "v9 default-deny role without app_resources",
			version:   V9,
			allow:     RoleConditions{AppLabels: Labels{"env": []string{"dev"}}},
			assertErr: require.NoError,
		},
		{
			// CheckAndSetDefaults accepts any rule content for forward
			// compatibility.
			name:      "v9 rule without allow_all passes read validation",
			version:   V9,
			allow:     RoleConditions{AppResources: []AppResource{{}}},
			assertErr: require.NoError,
		},
		{
			name:      "app_resources on v8 role",
			version:   V8,
			allow:     RoleConditions{AppResources: []AppResource{{AllowAll: true}}},
			assertErr: errContains("requires role version"),
		},
		{
			name:      "app_resources under deny on v8 role",
			version:   V8,
			deny:      RoleConditions{AppResources: []AppResource{{AllowAll: true}}},
			assertErr: errContains("requires role version"),
		},
		{
			name:      "app_resources_expressions on v8 role",
			version:   V8,
			allow:     RoleConditions{AppResourcesExpressions: []string{"true"}},
			assertErr: errContains("requires role version"),
		},
		{
			name:      "app_resources_expressions under deny on v8 role",
			version:   V8,
			deny:      RoleConditions{AppResourcesExpressions: []string{"true"}},
			assertErr: errContains("requires role version"),
		},
		{
			name:      "v9 app_resources_expressions passes read validation",
			version:   V9,
			allow:     RoleConditions{AppResourcesExpressions: []string{"true"}},
			assertErr: require.NoError,
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

func TestAppResourcesAllowAll(t *testing.T) {
	allowAll := AppResource{AllowAll: true}
	require.True(t, AppResourcesAllowAll([]AppResource{allowAll}, nil))
	require.False(t, AppResourcesAllowAll(nil, nil))
	require.False(t, AppResourcesAllowAll([]AppResource{{}}, nil))
	// Multiple allow rules and deny-side rules can only come from newer
	// versions and fail closed.
	require.False(t, AppResourcesAllowAll([]AppResource{{}, allowAll}, nil))
	require.False(t, AppResourcesAllowAll([]AppResource{allowAll, allowAll}, nil))
	require.False(t, AppResourcesAllowAll([]AppResource{allowAll}, []AppResource{{}}))
}
