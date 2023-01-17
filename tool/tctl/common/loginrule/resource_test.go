// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package loginrule

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

// TestUnmarshalLoginRule tests that login rules can be successfully
// unmarshalled from YAML and JSON.
func TestUnmarshalLoginRule(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		desc          string
		input         string
		errorContains string
		expected      *loginrulepb.LoginRule
	}{
		{
			desc: "traits_map yaml",
			input: `---
kind: login_rule
version: v1
metadata:
  name: test_rule
spec:
  priority: 0
  traits_map:
    logins: [ubuntu, root]
    groups:
      - "external.groups"
      - teleport
`,
			expected: &loginrulepb.LoginRule{
				Version: "v1",
				Metadata: &types.Metadata{
					Name:      "test_rule",
					Namespace: "default",
				},
				Priority: 0,
				TraitsMap: map[string]*wrappers.StringValues{
					"logins": &wrappers.StringValues{
						Values: []string{"ubuntu", "root"},
					},
					"groups": &wrappers.StringValues{
						Values: []string{"external.groups", "teleport"},
					},
				},
			},
		},
		{
			desc: "traits_map json",
			input: `{
				"kind": "login_rule",
				"version": "v1",
				"metadata": {
					"name": "test_rule"
				},
				"spec": {
					"priority": 0,
					"traits_map": {
						"logins": ["ubuntu", "root"],
						"groups": ["external.groups", "teleport"]
					}
				}
			}`,
			expected: &loginrulepb.LoginRule{
				Version: "v1",
				Metadata: &types.Metadata{
					Name:      "test_rule",
					Namespace: "default",
				},
				Priority: 0,
				TraitsMap: map[string]*wrappers.StringValues{
					"logins": &wrappers.StringValues{
						Values: []string{"ubuntu", "root"},
					},
					"groups": &wrappers.StringValues{
						Values: []string{"external.groups", "teleport"},
					},
				},
			},
		},
		{
			desc: "empty map value yaml",
			input: `---
kind: login_rule
version: v1
metadata:
  name: test_rule
spec:
  traits_map:
    logins: [ubuntu, root]
    groups:
`,
			errorContains: `traits_map has zero non-empty values for key "groups"`,
		},
		{
			desc: "empty map value json",
			input: `{
				"kind": "login_rule",
				"version": "v1",
				"metadata": {
					"name": "test_rule"
				},
				"spec": {
					"priority": 0,
					"traits_map": {
						"logins": ["ubuntu", "root"],
						"groups": []
					}
				}
			}`,
			errorContains: `traits_map has zero non-empty values for key "groups"`,
		},
		{
			desc: "traits_expression yaml",
			input: `---
kind: login_rule
version: v1
metadata:
  name: test_rule
spec:
  priority: 1
  traits_expression: external.remove_keys("test")
`,
			expected: &loginrulepb.LoginRule{
				Version: "v1",
				Metadata: &types.Metadata{
					Name:      "test_rule",
					Namespace: "default",
				},
				Priority:         1,
				TraitsExpression: `external.remove_keys("test")`,
			},
		},
		{
			desc: "traits_expression json",
			input: `{
				"kind": "login_rule",
				"version": "v1",
				"metadata": {
					"name": "test_rule"
				},
				"spec": {
					"priority": 1,
					"traits_expression": "external.remove_keys(\"test\")"
				}
			}`,
			expected: &loginrulepb.LoginRule{
				Version: "v1",
				Metadata: &types.Metadata{
					Name:      "test_rule",
					Namespace: "default",
				},
				Priority:         1,
				TraitsExpression: `external.remove_keys("test")`,
			},
		},
		{
			// Make sure yaml with a "folded scalar" (>) can be parsed
			desc: "folded traits_expression",
			input: `---
kind: login_rule
version: v1
metadata:
  name: test_rule
spec:
  priority: 1
  traits_expression: >
    external
      .remove_keys("test")
      .add_values("groups", "teleport")
`,
			expected: &loginrulepb.LoginRule{
				Version: "v1",
				Metadata: &types.Metadata{
					Name:      "test_rule",
					Namespace: "default",
				},
				Priority: 1,
				TraitsExpression: `external
  .remove_keys("test")
  .add_values("groups", "teleport")
`,
			},
		},
		{
			// Make sure yaml with a "literal scalar" (|) can be parsed
			desc: "literal traits_expression",
			input: `---
kind: login_rule
version: v1
metadata:
  name: test_rule
spec:
  priority: 1
  traits_expression: |
    external
      .remove_keys("test")
      .add_values("groups", "teleport")
`,
			expected: &loginrulepb.LoginRule{
				Version: "v1",
				Metadata: &types.Metadata{
					Name:      "test_rule",
					Namespace: "default",
				},
				Priority: 1,
				TraitsExpression: `external
  .remove_keys("test")
  .add_values("groups", "teleport")
`,
			},
		},
		{
			desc: "no map or expression yaml",
			input: `---
kind: login_rule
version: v1
metadata:
  name: test_rule
spec:
  priority: 1
`,
			errorContains: "login rule has empty traits_map and traits_expression, exactly one must be set",
		},
		{
			desc: "no map or expression json",
			input: `{
				"kind": "login_rule",
				"version": "v1",
				"metadata": {
					"name": "test_rule"
				},
				"spec": {
					"priority": 1
				}
			}`,
			errorContains: "login rule has empty traits_map and traits_expression, exactly one must be set",
		},
		{
			desc: "both map and expression yaml",
			input: `---
kind: login_rule
version: v1
metadata:
  name: test_rule
spec:
  priority: 1
  traits_map:
    logins: [root]
  traits_expression: external.remove_keys("test")
`,
			errorContains: "login rule has non-empty traits_map and traits_expression, exactly one must be set",
		},
		{
			desc: "both map and expression json",
			input: `{
				"kind": "login_rule",
				"version": "v1",
				"metadata": {
					"name": "test_rule"
				},
				"spec": {
					"priority": 1,
					"traits_map": {
						"logins": ["ubuntu", "root"]
					},
					"traits_expression": "external.remove_keys(\"test\")"
				}
			}`,
			errorContains: "login rule has non-empty traits_map and traits_expression, exactly one must be set",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// Mimic tctl resource command by using the same decoder and
			// initially unmarshalling into services.UnknownResource
			reader := strings.NewReader(tc.input)
			decoder := kyaml.NewYAMLOrJSONDecoder(reader, defaults.LookaheadBufSize)
			var raw services.UnknownResource
			err := decoder.Decode(&raw)
			require.NoError(t, err)
			require.Equal(t, "login_rule", raw.Kind)

			out, err := UnmarshalLoginRule(raw.Raw)
			if tc.errorContains != "" {
				require.ErrorContains(t, err, tc.errorContains, "error from UnmarshalLoginRule does not contain the expected string")
				return
			}
			require.NoError(t, err, "UnmarshalLoginRule returned unexpected error")

			require.Equal(t, tc.expected, out, "unmarshalled login rule does not match what was expected")
		})
	}
}
