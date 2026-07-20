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
	"testing"

	"github.com/stretchr/testify/require"
)

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

func TestIsAllowAllOnly(t *testing.T) {
	require.True(t, AppResource{AllowAll: true}.IsAllowAllOnly())
	require.False(t, AppResource{}.IsAllowAllOnly())
	// Unknown fields from a newer auth server must not grant access.
	withUnknownField := AppResource{AllowAll: true, XXX_unrecognized: []byte{0x0a, 0x01, 0x2f}}
	require.False(t, withUnknownField.IsAllowAllOnly())
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
