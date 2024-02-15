/*
Copyright 2023 Gravitational, Inc.

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
	"encoding/json"
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/gravitational/teleport/api/types/wrappers"
)

func TestAccessRequestConditionsIsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		arc      AccessRequestConditions
		expected bool
	}{
		{
			name:     "empty",
			arc:      AccessRequestConditions{},
			expected: true,
		},
		{
			name: "annotations",
			arc: AccessRequestConditions{
				Annotations: wrappers.Traits{
					"test": []string{"test"},
				},
			},
			expected: false,
		},
		{
			name: "claims to roles",
			arc: AccessRequestConditions{
				ClaimsToRoles: []ClaimMapping{
					{},
				},
			},
			expected: false,
		},
		{
			name: "roles",
			arc: AccessRequestConditions{
				Roles: []string{"test"},
			},
			expected: false,
		},
		{
			name: "search as roles",
			arc: AccessRequestConditions{
				SearchAsRoles: []string{"test"},
			},
			expected: false,
		},
		{
			name: "suggested reviewers",
			arc: AccessRequestConditions{
				SuggestedReviewers: []string{"test"},
			},
			expected: false,
		},
		{
			name: "thresholds",
			arc: AccessRequestConditions{
				Thresholds: []AccessReviewThreshold{
					{
						Name: "test",
					},
				},
			},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, test.arc.IsEmpty())
		})
	}
}

func TestAccessReviewConditionsIsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		arc      AccessReviewConditions
		expected bool
	}{
		{
			name:     "empty",
			arc:      AccessReviewConditions{},
			expected: true,
		},
		{
			name: "claims to roles",
			arc: AccessReviewConditions{
				ClaimsToRoles: []ClaimMapping{
					{},
				},
			},
			expected: false,
		},
		{
			name: "preview as roles",
			arc: AccessReviewConditions{
				PreviewAsRoles: []string{"test"},
			},
			expected: false,
		},
		{
			name: "roles",
			arc: AccessReviewConditions{
				Roles: []string{"test"},
			},
			expected: false,
		},
		{
			name: "where",
			arc: AccessReviewConditions{
				Where: "test",
			},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, test.arc.IsEmpty())
		})
	}
}

func TestRole_GetKubeResources(t *testing.T) {
	kubeLabels := Labels{
		Wildcard: {Wildcard},
	}
	labelsExpression := "contains(user.spec.traits[\"groups\"], \"prod\")"
	type args struct {
		version          string
		labels           Labels
		labelsExpression string
		resources        []KubernetesResource
	}
	tests := []struct {
		name                string
		args                args
		want                []KubernetesResource
		assertErrorCreation require.ErrorAssertionFunc
	}{
		{
			name: "v7 with error",
			args: args{
				version: V7,
				labels:  kubeLabels,
				resources: []KubernetesResource{
					{
						Kind:      "invalid resource",
						Namespace: "test",
						Name:      "test",
					},
				},
			},
			assertErrorCreation: require.Error,
		},
		{
			name: "v7",
			args: args{
				version: V7,
				labels:  kubeLabels,
				resources: []KubernetesResource{
					{
						Kind:      KindKubePod,
						Namespace: "test",
						Name:      "test",
					},
				},
			},
			assertErrorCreation: require.NoError,
			want: []KubernetesResource{
				{
					Kind:      KindKubePod,
					Namespace: "test",
					Name:      "test",
				},
			},
		},
		{
			name: "v7 with labels expression",
			args: args{
				version:          V7,
				labelsExpression: labelsExpression,
				resources: []KubernetesResource{
					{
						Kind:      KindKubePod,
						Namespace: "test",
						Name:      "test",
					},
				},
			},
			assertErrorCreation: require.NoError,
			want: []KubernetesResource{
				{
					Kind:      KindKubePod,
					Namespace: "test",
					Name:      "test",
				},
			},
		},
		{
			name: "v6 to v7 without wildcard; labels expression",
			args: args{
				version:          V6,
				labelsExpression: labelsExpression,
				resources: []KubernetesResource{
					{
						Kind:      KindKubePod,
						Namespace: "test",
						Name:      "test",
					},
				},
			},
			assertErrorCreation: require.NoError,
			want: append([]KubernetesResource{
				{
					Kind:      KindKubePod,
					Namespace: "test",
					Name:      "test",
					Verbs:     []string{Wildcard},
				},
			},
				appendV7KubeResources()...),
		},
		{
			name: "v6 to v7 with wildcard",
			args: args{
				version: V6,
				labels:  kubeLabels,
				resources: []KubernetesResource{
					{
						Kind:      KindKubePod,
						Namespace: Wildcard,
						Name:      Wildcard,
					},
				},
			},
			assertErrorCreation: require.NoError,
			want: []KubernetesResource{
				{
					Kind:      Wildcard,
					Namespace: Wildcard,
					Name:      Wildcard,
					Verbs:     []string{Wildcard},
				},
			},
		},
		{
			name: "v6 to v7 without wildcard",
			args: args{
				version: V6,
				labels:  kubeLabels,
				resources: []KubernetesResource{
					{
						Kind:      KindKubePod,
						Namespace: "test",
						Name:      "test",
					},
				},
			},
			assertErrorCreation: require.NoError,
			want: append([]KubernetesResource{
				{
					Kind:      KindKubePod,
					Namespace: "test",
					Name:      "test",
					Verbs:     []string{Wildcard},
				},
			},
				appendV7KubeResources()...),
		},
		{
			name: "v5 to v7: populate with defaults.",
			args: args{
				version:   V5,
				labels:    kubeLabels,
				resources: nil,
			},
			assertErrorCreation: require.NoError,
			want: []KubernetesResource{
				{
					Kind:      Wildcard,
					Namespace: Wildcard,
					Name:      Wildcard,
					Verbs:     []string{Wildcard},
				},
			},
		},
		{
			name: "v5 to v7 without kube labels",
			args: args{
				version:   V5,
				resources: nil,
			},
			assertErrorCreation: require.NoError,
			want:                nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := NewRoleWithVersion(
				"test",
				tt.args.version,
				RoleSpecV6{
					Allow: RoleConditions{
						Namespaces:                 []string{"default"},
						KubernetesLabels:           tt.args.labels,
						KubernetesResources:        tt.args.resources,
						KubernetesLabelsExpression: tt.args.labelsExpression,
					},
				},
			)
			tt.assertErrorCreation(t, err)
			if err != nil {
				return
			}
			got := r.GetKubeResources(Allow)
			require.Equal(t, tt.want, got)
			got = r.GetKubeResources(Deny)
			require.Empty(t, got)
		})
	}
}

func appendV7KubeResources() []KubernetesResource {
	resources := []KubernetesResource{}
	// append other kubernetes resources
	for _, resource := range KubernetesResourcesKinds {
		if resource == KindKubePod || resource == KindKubeNamespace {
			continue
		}
		resources = append(resources, KubernetesResource{
			Kind:      resource,
			Namespace: Wildcard,
			Name:      Wildcard,
			Verbs:     []string{Wildcard},
		},
		)
	}
	return resources
}

func TestMarshallCreateHostUserModeJSON(t *testing.T) {
	for _, tc := range []struct {
		input    CreateHostUserMode
		expected string
	}{
		{input: CreateHostUserMode_HOST_USER_MODE_OFF, expected: "off"},
		{input: CreateHostUserMode_HOST_USER_MODE_UNSPECIFIED, expected: ""},
		{input: CreateHostUserMode_HOST_USER_MODE_KEEP, expected: "keep"},
		{input: CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP, expected: "insecure-drop"},
	} {
		got, err := json.Marshal(&tc.input)
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("%q", tc.expected), string(got))
	}
}

func TestMarshallCreateHostUserModeYAML(t *testing.T) {
	for _, tc := range []struct {
		input    CreateHostUserMode
		expected string
	}{
		{input: CreateHostUserMode_HOST_USER_MODE_OFF, expected: "\"off\""},
		{input: CreateHostUserMode_HOST_USER_MODE_UNSPECIFIED, expected: "\"\""},
		{input: CreateHostUserMode_HOST_USER_MODE_KEEP, expected: "keep"},
		{input: CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP, expected: "insecure-drop"},
	} {
		got, err := yaml.Marshal(&tc.input)
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("%s\n", tc.expected), string(got))
	}
}

func TestUnmarshallCreateHostUserModeJSON(t *testing.T) {
	for _, tc := range []struct {
		expected CreateHostUserMode
		input    any
	}{
		{expected: CreateHostUserMode_HOST_USER_MODE_OFF, input: "\"off\""},
		{expected: CreateHostUserMode_HOST_USER_MODE_UNSPECIFIED, input: "\"\""},
		{expected: CreateHostUserMode_HOST_USER_MODE_KEEP, input: "\"keep\""},
		{expected: CreateHostUserMode_HOST_USER_MODE_KEEP, input: 3},
		{expected: CreateHostUserMode_HOST_USER_MODE_OFF, input: 1},
		{expected: CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP, input: 4},
	} {
		var got CreateHostUserMode
		err := json.Unmarshal([]byte(fmt.Sprintf("%v", tc.input)), &got)
		require.NoError(t, err)
		require.Equal(t, tc.expected, got)
	}
}

func TestUnmarshallCreateHostUserModeYAML(t *testing.T) {
	for _, tc := range []struct {
		expected CreateHostUserMode
		input    string
	}{
		{expected: CreateHostUserMode_HOST_USER_MODE_OFF, input: "\"off\""},
		{expected: CreateHostUserMode_HOST_USER_MODE_OFF, input: "off"},
		{expected: CreateHostUserMode_HOST_USER_MODE_UNSPECIFIED, input: "\"\""},
		{expected: CreateHostUserMode_HOST_USER_MODE_KEEP, input: "keep"},
		{expected: CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP, input: "insecure-drop"},
	} {
		var got CreateHostUserMode
		err := yaml.Unmarshal([]byte(tc.input), &got)
		require.NoError(t, err)
		require.Equal(t, tc.expected, got)
	}
}

func TestRoleV6_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()
	requireBadParameterContains := func(contains string) require.ErrorAssertionFunc {
		return func(t require.TestingT, err error, msgAndArgs ...interface{}) {
			require.True(t, trace.IsBadParameter(err))
			require.ErrorContains(t, err, contains)
		}
	}
	newRole := func(t *testing.T, spec RoleSpecV6) *RoleV6 {
		return &RoleV6{
			Metadata: Metadata{
				Name: "test",
			},
			Spec: spec,
		}
	}

	tests := []struct {
		name         string
		role         *RoleV6
		requireError require.ErrorAssertionFunc
	}{
		{
			name: "spiffe: valid",
			role: newRole(t, RoleSpecV6{
				Allow: RoleConditions{
					SPIFFE: []*SPIFFERoleCondition{{Path: "/test"}},
				},
			}),
			requireError: require.NoError,
		},
		{
			name: "spiffe: valid regex path",
			role: newRole(t, RoleSpecV6{
				Allow: RoleConditions{
					SPIFFE: []*SPIFFERoleCondition{{Path: `^\/svc\/foo\/.*\/bar$`}},
				},
			}),
			requireError: require.NoError,
		},
		{
			name: "spiffe: missing path",
			role: newRole(t, RoleSpecV6{
				Allow: RoleConditions{
					SPIFFE: []*SPIFFERoleCondition{{Path: ""}},
				},
			}),
			requireError: requireBadParameterContains("path: should be non-empty"),
		},
		{
			name: "spiffe: path not prepended",
			role: newRole(t, RoleSpecV6{
				Allow: RoleConditions{
					SPIFFE: []*SPIFFERoleCondition{{Path: "foo"}},
				},
			}),
			requireError: requireBadParameterContains("path: should start with /"),
		},
		{
			name: "spiffe: invalid ip cidr",
			role: newRole(t, RoleSpecV6{
				Allow: RoleConditions{
					SPIFFE: []*SPIFFERoleCondition{
						{
							Path: "/foo",
							IPSANs: []string{
								"10.0.0.1/24",
								"llama",
							},
						},
					},
				},
			}),
			requireError: requireBadParameterContains("validating ip_sans[1]: invalid CIDR address: llama"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.role.CheckAndSetDefaults()
			tt.requireError(t, err)
		})
	}
}
