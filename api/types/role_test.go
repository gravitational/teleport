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
	const labelsExpression = "contains(user.spec.traits[\"groups\"], \"prod\")"

	type args struct {
		version          string
		labels           Labels
		labelsExpression string
		resources        []KubernetesResource
	}
	tests := []struct {
		name                string
		args                args
		wantAllow           []KubernetesResource
		wantDeny            []KubernetesResource
		assertErrorCreation require.ErrorAssertionFunc
	}{
		{
			name: "v8 with unknown kind",
			args: args{
				version: V8,
				labels:  kubeLabels,
				resources: []KubernetesResource{
					{
						Kind:      "unknown kind",
						Namespace: "test",
						Name:      "test",
						APIGroup:  Wildcard,
					},
				},
			},
			wantAllow: []KubernetesResource{
				{
					Kind:      "unknown kind",
					Namespace: "test",
					Name:      "test",
					APIGroup:  Wildcard,
				},
			},
			assertErrorCreation: require.NoError,
		},
		{
			name: "v8 pods without group",
			args: args{
				version: V8,
				labels:  kubeLabels,
				resources: []KubernetesResource{
					{
						Kind:      "pods",
						Namespace: "test",
						Name:      "test",
					},
				},
			},
			wantAllow: []KubernetesResource{
				{
					Kind:      "pods",
					Namespace: "test",
					Name:      "test",
					APIGroup:  "",
				},
			},
			assertErrorCreation: require.NoError,
		},
		{
			name: "v8 deployments without group",
			args: args{
				version: V8,
				labels:  kubeLabels,
				resources: []KubernetesResource{
					{
						Kind:      "deployements",
						Namespace: "test",
						Name:      "test",
					},
				},
			},
			assertErrorCreation: require.Error,
		},
		{
			name: "v8 deny",
			args: args{
				version: V8,
				labels:  kubeLabels,
				resources: []KubernetesResource{
					{
						Kind:      "crontabs",
						Namespace: "test",
						Name:      "test",
						Verbs:     []string{Wildcard},
						APIGroup:  "stable.example.com",
					},
				},
			},
			wantDeny: []KubernetesResource{
				{
					Kind:      "crontabs",
					Namespace: "test",
					Name:      "test",
					Verbs:     []string{Wildcard},
					APIGroup:  "stable.example.com",
				},
			},
			assertErrorCreation: require.NoError,
		},
		{
			name: "v7 deny",
			args: args{
				version: V7,
				labels:  kubeLabels,
				resources: []KubernetesResource{
					{
						Kind:      KindKubePod,
						Namespace: "test",
						Name:      "test",
						Verbs:     []string{Wildcard},
					},
					{
						Kind:      KindKubeJob,
						Namespace: "test",
						Name:      "test",
						Verbs:     []string{Wildcard},
					},
				},
			},
			wantDeny: []KubernetesResource{
				{
					Kind:      "pods",
					Namespace: "test",
					Name:      "test",
					Verbs:     []string{Wildcard},
					APIGroup:  "",
				},
				{
					Kind:      "jobs",
					Namespace: "test",
					Name:      "test",
					Verbs:     []string{Wildcard},
					APIGroup:  "batch",
				},
			},
			assertErrorCreation: require.NoError,
		},
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
			name: "v7 without group",
			args: args{
				version: V7,
				labels:  kubeLabels,
				resources: []KubernetesResource{
					{
						Kind:      KindKubePod,
						Namespace: "test",
						Name:      "test",
					},
					{
						Kind:      KindKubeDeployment,
						Namespace: "test",
						Name:      "test",
					},
				},
			},
			assertErrorCreation: require.NoError,
			wantAllow: []KubernetesResource{
				{
					Kind:      "pods",
					Namespace: "test",
					Name:      "test",
					APIGroup:  "",
				},
				{
					Kind:      "deployments",
					Namespace: "test",
					Name:      "test",
					APIGroup:  "apps",
				},
			},
		},
		{
			name: "v7 with group",
			args: args{
				version: V7,
				labels:  kubeLabels,
				resources: []KubernetesResource{
					{
						Kind:      KindKubePod,
						Namespace: "test",
						Name:      "test",
						APIGroup:  "apps",
					},
				},
			},
			assertErrorCreation: require.Error,
		},
		{
			name: "v7 with label expression",
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
			wantAllow: []KubernetesResource{
				{
					Kind:      "pods",
					Namespace: "test",
					Name:      "test",
					APIGroup:  "",
				},
			},
		},
		{
			name: "v7 with allow wildcard kind",
			args: args{
				version: V7,
				labels:  kubeLabels,
				resources: []KubernetesResource{
					{
						// rolev7 ignored the namespace field for global resources.
						Kind:      Wildcard,
						Namespace: "default",
						Name:      Wildcard,
						Verbs:     []string{Wildcard},
					},
				},
			},
			assertErrorCreation: require.NoError,
			wantAllow: []KubernetesResource{
				// Expect the main resource to match namespaced resources.
				{
					Kind:      Wildcard,
					Namespace: "default",
					Name:      Wildcard,
					Verbs:     []string{Wildcard},
					APIGroup:  Wildcard,
				},
				// Expect injected global resources to maintain v7 behavior.
				{
					Kind:     "nodes",
					Name:     Wildcard,
					Verbs:    []string{Wildcard},
					APIGroup: Wildcard,
				},
				{
					Kind:     "persistentvolumes",
					Name:     Wildcard,
					Verbs:    []string{Wildcard},
					APIGroup: Wildcard,
				},
				{
					Kind:     "clusterroles",
					Name:     Wildcard,
					Verbs:    []string{Wildcard},
					APIGroup: Wildcard,
				},
				{
					Kind:     "clusterrolebindings",
					Name:     Wildcard,
					Verbs:    []string{Wildcard},
					APIGroup: Wildcard,
				},
				{
					Kind:     "certificatesigningrequests",
					Name:     Wildcard,
					Verbs:    []string{Wildcard},
					APIGroup: Wildcard,
				},
			},
		},
		{
			name: "v7 with deny wildcard kind",
			args: args{
				version: V7,
				labels:  kubeLabels,
				resources: []KubernetesResource{
					{
						// rolev7 ignored the namespace field for global resources.
						Kind:      Wildcard,
						Namespace: "default",
						Name:      Wildcard,
						Verbs:     []string{Wildcard},
					},
				},
			},
			assertErrorCreation: require.NoError,
			wantDeny: []KubernetesResource{
				// Expect the main resource to match namespaced resources.
				{
					Kind:      Wildcard,
					Namespace: "default",
					Name:      Wildcard,
					Verbs:     []string{Wildcard},
					APIGroup:  Wildcard,
				},
				// Expect injected global resources to maintain v7 behavior.
				{
					Kind:     "nodes",
					Name:     Wildcard,
					Verbs:    []string{Wildcard},
					APIGroup: Wildcard,
				},
				{
					Kind:     "persistentvolumes",
					Name:     Wildcard,
					Verbs:    []string{Wildcard},
					APIGroup: Wildcard,
				},
				{
					Kind:     "clusterroles",
					Name:     Wildcard,
					Verbs:    []string{Wildcard},
					APIGroup: Wildcard,
				},
				{
					Kind:     "clusterrolebindings",
					Name:     Wildcard,
					Verbs:    []string{Wildcard},
					APIGroup: Wildcard,
				},
				{
					Kind:     "certificatesigningrequests",
					Name:     Wildcard,
					Verbs:    []string{Wildcard},
					APIGroup: Wildcard,
				},
			},
		},
		{
			name: "v7 with allow namespace",
			args: args{
				version: V7,
				labels:  kubeLabels,
				resources: []KubernetesResource{
					{
						Kind:  KindKubeNamespace,
						Name:  "default",
						Verbs: []string{Wildcard},
					},
				},
			},
			assertErrorCreation: require.NoError,
			wantAllow: []KubernetesResource{
				{
					Kind:      Wildcard,
					Namespace: "default",
					Name:      Wildcard,
					Verbs:     []string{Wildcard},
					APIGroup:  Wildcard,
				},
				{
					Kind:  "namespaces",
					Name:  "default",
					Verbs: []string{Wildcard},
				},
			},
		},

		{
			name: "v7 with deny namespace",
			args: args{
				version: V7,
				labels:  kubeLabels,
				resources: []KubernetesResource{
					{
						Kind:  KindKubeNamespace,
						Name:  "default",
						Verbs: []string{Wildcard},
					},
				},
			},
			assertErrorCreation: require.NoError,
			wantDeny: []KubernetesResource{
				{
					Kind:      Wildcard,
					Namespace: "default",
					Name:      Wildcard,
					Verbs:     []string{Wildcard},
					APIGroup:  Wildcard,
				},
				{
					Kind:  "namespaces",
					Name:  "default",
					Verbs: []string{Wildcard},
				},
			},
		},

		{
			name: "v6 without wildcard; labels expression",
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
			wantAllow: append([]KubernetesResource{
				{
					Kind:      "pods",
					Namespace: "test",
					Name:      "test",
					Verbs:     []string{Wildcard},
					APIGroup:  "",
				},
			},
				appendV7KubeResources()...),
		},
		{
			name: "v6 with wildcard",
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
			wantAllow: []KubernetesResource{
				{
					Kind:      Wildcard,
					Namespace: Wildcard,
					Name:      Wildcard,
					Verbs:     []string{Wildcard},
					APIGroup:  Wildcard,
				},
			},
		},
		{
			name: "v6 without wildcard",
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
			wantAllow: append([]KubernetesResource{
				{
					Kind:      "pods",
					Namespace: "test",
					Name:      "test",
					Verbs:     []string{Wildcard},
					APIGroup:  "",
				},
			},
				appendV7KubeResources()...),
		},
		{
			name: "v5: populate with defaults.",
			args: args{
				version:   V5,
				labels:    kubeLabels,
				resources: nil,
			},
			assertErrorCreation: require.NoError,
			wantAllow: []KubernetesResource{
				{
					Kind:      Wildcard,
					Namespace: Wildcard,
					Name:      Wildcard,
					Verbs:     []string{Wildcard},
					APIGroup:  Wildcard,
				},
			},
		},
		{
			name: "v5 without kube labels",
			args: args{
				version:   V5,
				resources: nil,
			},
			assertErrorCreation: require.NoError,
			wantAllow:           nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := RoleConditions{
				Namespaces:                 []string{"default"},
				KubernetesLabels:           tt.args.labels,
				KubernetesResources:        tt.args.resources,
				KubernetesLabelsExpression: tt.args.labelsExpression,
			}
			spec := RoleSpecV6{}
			if tt.wantDeny == nil {
				spec.Allow = cond
			} else {
				spec.Deny = cond
			}

			r, err := NewRoleWithVersion(
				"test",
				tt.args.version,
				spec,
			)
			tt.assertErrorCreation(t, err)
			if err != nil {
				return
			}
			if tt.wantDeny == nil {
				got := r.GetKubeResources(Allow)
				tt.wantAllow = append(tt.wantAllow, KubernetesResourceSelfSubjectAccessReview)
				require.Equal(t, tt.wantAllow, got)
			}
			got := r.GetKubeResources(Deny)
			require.Equal(t, tt.wantDeny, got)
		})
	}
}

func TestRole_AllowRequestKubernetesResource(t *testing.T) {
	type args struct {
		version   string
		resources []RequestKubernetesResource
	}
	tests := []struct {
		name                string
		args                args
		want                []RequestKubernetesResource
		assertErrorCreation require.ErrorAssertionFunc
	}{
		{
			name: "valid single value",
			args: args{
				version: V7,
				resources: []RequestKubernetesResource{
					{
						Kind: KindKubePod,
					},
				},
			},
			assertErrorCreation: require.NoError,
			want: []RequestKubernetesResource{
				{
					Kind: KindKubePod,
				},
			},
		},
		{
			name: "valid no values",
			args: args{
				version: V7,
			},
			assertErrorCreation: require.NoError,
		},
		{
			name: "valid wildcard value",
			args: args{
				version: V7,
				resources: []RequestKubernetesResource{
					{
						Kind: Wildcard,
					},
				},
			},
			assertErrorCreation: require.NoError,
			want: []RequestKubernetesResource{
				{
					Kind: Wildcard,
				},
			},
		},
		{
			name: "valid multi values",
			args: args{
				version: V7,
				resources: []RequestKubernetesResource{
					{
						Kind: KindKubeNamespace,
					},
					{
						Kind: KindKubePod,
					},
					{
						Kind: KindKubeSecret,
					},
				},
			},
			assertErrorCreation: require.NoError,
			want: []RequestKubernetesResource{
				{
					Kind: KindKubeNamespace,
				},
				{
					Kind: KindKubePod,
				},
				{
					Kind: KindKubeSecret,
				},
			},
		},
		{
			name: "valid multi values with wildcard",
			args: args{
				version: V7,
				resources: []RequestKubernetesResource{
					{
						Kind: KindKubeNamespace,
					},
					{
						Kind: Wildcard,
					},
				},
			},
			assertErrorCreation: require.NoError,
			want: []RequestKubernetesResource{
				{
					Kind: KindKubeNamespace,
				},
				{
					Kind: Wildcard,
				},
			},
		},
		{
			name: "invalid kind (kube_cluster is not part of Kubernetes subresources)",
			args: args{
				version: V7,
				resources: []RequestKubernetesResource{
					{
						Kind: KindKubernetesCluster,
					},
				},
			},
			assertErrorCreation: require.Error,
		},
		{
			name: "invalid multi value",
			args: args{
				version: V7,
				resources: []RequestKubernetesResource{
					{
						Kind: Wildcard,
					},
					{
						Kind: KindKubeNamespace,
					},
					{
						Kind: KindKubernetesCluster,
					},
				},
			},
			assertErrorCreation: require.Error,
		},
		{
			name: "invalid kinds not supported for v6",
			args: args{
				version: V6,
				resources: []RequestKubernetesResource{
					{
						Kind: Wildcard,
					},
				},
			},
			assertErrorCreation: require.Error,
		},
		{
			name: "invalid kinds not supported for v5",
			args: args{
				version: V6,
				resources: []RequestKubernetesResource{
					{
						Kind: Wildcard,
					},
				},
			},
			assertErrorCreation: require.Error,
		},
		{
			name: "invalid kinds not supported for v4",
			args: args{
				version: V6,
				resources: []RequestKubernetesResource{
					{
						Kind: Wildcard,
					},
				},
			},
			assertErrorCreation: require.Error,
		},
		{
			name: "invalid kinds not supported for v3",
			args: args{
				version: V6,
				resources: []RequestKubernetesResource{
					{
						Kind: Wildcard,
					},
				},
			},
			assertErrorCreation: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := NewRoleWithVersion(
				"test",
				tt.args.version,
				RoleSpecV6{
					Allow: RoleConditions{
						Request: &AccessRequestConditions{
							KubernetesResources: tt.args.resources,
						},
					},
				},
			)
			tt.assertErrorCreation(t, err)
			if err != nil {
				return
			}
			got := r.GetRoleConditions(Allow).Request.KubernetesResources
			require.Equal(t, tt.want, got)
		})
	}
}

func appendV7KubeResources() []KubernetesResource {
	resources := []KubernetesResource{}
	// append other kubernetes resources
	for _, resource := range KubernetesResourcesKinds {
		group := KubernetesResourcesV7KindGroups[resource]
		resource = KubernetesResourcesKindsPlurals[resource]
		if resource == "pods" || resource == "namespaces" {
			continue
		}
		resources = append(resources, KubernetesResource{
			Kind:      resource,
			Namespace: Wildcard,
			Name:      Wildcard,
			Verbs:     []string{Wildcard},
			APIGroup:  group,
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
		{input: CreateHostUserMode_HOST_USER_MODE_OFF, expected: `"off"`},
		{input: CreateHostUserMode_HOST_USER_MODE_UNSPECIFIED, expected: `""`},
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
		input    any
		expected CreateHostUserMode
	}{
		{input: `"off"`, expected: CreateHostUserMode_HOST_USER_MODE_OFF},
		{input: `""`, expected: CreateHostUserMode_HOST_USER_MODE_UNSPECIFIED},
		{input: `"keep"`, expected: CreateHostUserMode_HOST_USER_MODE_KEEP},
		{input: 3, expected: CreateHostUserMode_HOST_USER_MODE_KEEP},
		{input: 1, expected: CreateHostUserMode_HOST_USER_MODE_OFF},
		{input: 4, expected: CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP},
	} {
		var got CreateHostUserMode
		err := json.Unmarshal([]byte(fmt.Sprintf("%v", tc.input)), &got)
		require.NoError(t, err)
		require.Equal(t, tc.expected, got)
	}
}

func TestUnmarshallCreateHostUserModeYAML(t *testing.T) {
	for _, tc := range []struct {
		input    string
		expected CreateHostUserMode
	}{
		{input: `"off"`, expected: CreateHostUserMode_HOST_USER_MODE_OFF},
		{input: "off", expected: CreateHostUserMode_HOST_USER_MODE_OFF},
		{input: `""`, expected: CreateHostUserMode_HOST_USER_MODE_UNSPECIFIED},
		{input: "keep", expected: CreateHostUserMode_HOST_USER_MODE_KEEP},
		{input: "insecure-drop", expected: CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP},
	} {
		var got CreateHostUserMode
		err := yaml.Unmarshal([]byte(tc.input), &got)
		require.NoError(t, err)
		require.Equal(t, tc.expected, got)
	}
}

func TestUnmarshallCreateDatabaseUserModeJSON(t *testing.T) {
	for _, tc := range []struct {
		input    any
		expected CreateDatabaseUserMode
	}{
		{input: `""`, expected: CreateDatabaseUserMode_DB_USER_MODE_UNSPECIFIED},
		{input: `"off"`, expected: CreateDatabaseUserMode_DB_USER_MODE_OFF},
		{input: `"keep"`, expected: CreateDatabaseUserMode_DB_USER_MODE_KEEP},
		{input: `"best_effort_drop"`, expected: CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_DROP},
		{input: 0, expected: CreateDatabaseUserMode_DB_USER_MODE_UNSPECIFIED},
		{input: 1, expected: CreateDatabaseUserMode_DB_USER_MODE_OFF},
		{input: 2, expected: CreateDatabaseUserMode_DB_USER_MODE_KEEP},
		{input: 3, expected: CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_DROP},
	} {
		var got CreateDatabaseUserMode
		err := json.Unmarshal([]byte(fmt.Sprintf("%v", tc.input)), &got)
		require.NoError(t, err)
		require.Equalf(t, tc.expected, got, "for input: %v", tc.input)
	}
}

func TestUnmarshallCreateDatabaseUserModeYAML(t *testing.T) {
	for _, tc := range []struct {
		input    any
		expected CreateDatabaseUserMode
	}{
		{input: `""`, expected: CreateDatabaseUserMode_DB_USER_MODE_UNSPECIFIED},
		{input: `"off"`, expected: CreateDatabaseUserMode_DB_USER_MODE_OFF},
		{input: "off", expected: CreateDatabaseUserMode_DB_USER_MODE_OFF},
		{input: `"keep"`, expected: CreateDatabaseUserMode_DB_USER_MODE_KEEP},
		{input: `"best_effort_drop"`, expected: CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_DROP},
		{input: 0, expected: CreateDatabaseUserMode_DB_USER_MODE_UNSPECIFIED},
		{input: 1, expected: CreateDatabaseUserMode_DB_USER_MODE_OFF},
		{input: 2, expected: CreateDatabaseUserMode_DB_USER_MODE_KEEP},
		{input: 3, expected: CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_DROP},
	} {
		var got CreateDatabaseUserMode
		err := yaml.Unmarshal([]byte(fmt.Sprintf("%v", tc.input)), &got)
		require.NoError(t, err)
		require.Equalf(t, tc.expected, got, "for input: %v", tc.input)
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

func TestRoleV6_KubernetesResourcesCheckAndSetDefaults(t *testing.T) {
	t.Parallel()
	requireBadParameterContains := func(contains string) require.ErrorAssertionFunc {
		return func(t require.TestingT, err error, args ...any) {
			require.True(t, trace.IsBadParameter(err), args...)
			require.ErrorContains(t, err, contains, args...)
		}
	}
	newRole := func(t *testing.T, version string, spec RoleSpecV6) *RoleV6 {
		return &RoleV6{
			Version: version,
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
		expect       *RoleV6
	}{
		{
			name: "valid empty v7",
			role: newRole(t, V7, RoleSpecV6{
				Allow: RoleConditions{
					KubernetesLabels: Labels{
						"env": []string{"prod"},
					},
				},
			}),
			requireError: require.NoError,
			expect: newRole(t, V7, RoleSpecV6{
				Allow: RoleConditions{
					KubernetesLabels: Labels{
						"env": []string{"prod"},
					},
					KubernetesResources: []KubernetesResource{
						{
							Kind:      Wildcard,
							Namespace: Wildcard,
							Name:      Wildcard,
							Verbs:     []string{Wildcard},
						},
					},
				},
			}),
		},
		{
			name: "valid empty v8",
			role: newRole(t, V8, RoleSpecV6{
				Allow: RoleConditions{
					KubernetesLabels: Labels{
						"env": []string{"prod"},
					},
				},
			}),
			requireError: require.NoError,
			expect: newRole(t, V8, RoleSpecV6{
				Allow: RoleConditions{
					KubernetesLabels: Labels{
						"env": []string{"prod"},
					},
					KubernetesResources: []KubernetesResource{
						{
							Kind:      Wildcard,
							Namespace: Wildcard,
							Name:      Wildcard,
							Verbs:     []string{Wildcard},
							APIGroup:  Wildcard,
						},
					},
				},
			}),
		},
		{
			name: "invalid v7 api group set",
			role: newRole(t, V7, RoleSpecV6{
				Allow: RoleConditions{
					KubernetesResources: []KubernetesResource{
						{
							Kind:      Wildcard,
							Namespace: Wildcard,
							Name:      Wildcard,
							Verbs:     []string{Wildcard},
							APIGroup:  Wildcard,
						},
					},
				},
			}),
			requireError: requireBadParameterContains("Group \"*\" is not supported in role version \"v7\""),
		},
		{
			name: "invalid v8 missing api group",
			role: newRole(t, V8, RoleSpecV6{
				Allow: RoleConditions{
					KubernetesResources: []KubernetesResource{
						{
							Kind:      Wildcard,
							Namespace: Wildcard,
							Name:      Wildcard,
							Verbs:     []string{Wildcard},
						},
					},
				},
			}),
			requireError: requireBadParameterContains("KubernetesResource api_group is required for resource \"*\" in role version \"v8\""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.role.CheckAndSetDefaults()
			tt.requireError(t, err)
			if err == nil {
				require.Equal(t, tt.expect.Spec.Allow.KubernetesResources, tt.role.Spec.Allow.KubernetesResources)
			}
		})
	}
}

func TestRoleV6CheckAndSetDefaults_SAMLIdPOptions(t *testing.T) {
	t.Parallel()

	newRole := func(t *testing.T, spec RoleSpecV6, version string) *RoleV6 {
		t.Helper()
		return &RoleV6{
			Metadata: Metadata{
				Name: "test",
			},
			Spec:    spec,
			Version: version,
		}
	}

	tests := []struct {
		name              string
		role              *RoleV6
		expectedIdpOption *IdPOptions
		requireError      require.ErrorAssertionFunc
	}{
		{
			name:              "samlidp: idp option not enabled in role version 8",
			role:              newRole(t, RoleSpecV6{}, V8),
			expectedIdpOption: nil,
			requireError:      require.NoError,
		},
		{
			name: "samlidp: idp option enabled in role version v7 and below",
			role: newRole(t, RoleSpecV6{}, V7),
			expectedIdpOption: &IdPOptions{
				SAML: &IdPSAMLOptions{
					Enabled: NewBoolOption(true),
				},
			},
			requireError: require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.role.CheckAndSetDefaults()
			tt.requireError(t, err)
			require.Equal(t, tt.expectedIdpOption, tt.role.GetOptions().IDP)
		})
	}
}

func TestRoleFilterMatch(t *testing.T) {
	regularRole := RoleV6{
		Metadata: Metadata{
			Name: "request-approver",
		},
	}
	systemRole := RoleV6{
		Metadata: Metadata{
			Name: "bot",
			Labels: map[string]string{
				TeleportInternalResourceType: SystemResource,
			},
		},
	}

	tests := []struct {
		name        string
		role        *RoleV6
		filter      *RoleFilter
		shouldMatch bool
	}{
		{
			name:        "empty filter should match everything",
			role:        &regularRole,
			filter:      &RoleFilter{},
			shouldMatch: true,
		},
		{
			name:        "correct search keyword should match the regular role",
			role:        &regularRole,
			filter:      &RoleFilter{SearchKeywords: []string{"appr"}},
			shouldMatch: true,
		},
		{
			name:        "correct search keyword should match the system role",
			role:        &systemRole,
			filter:      &RoleFilter{SearchKeywords: []string{"bot"}},
			shouldMatch: true,
		},
		{
			name:        "incorrect search keyword shouldn't match the role",
			role:        &regularRole,
			filter:      &RoleFilter{SearchKeywords: []string{"xyz"}},
			shouldMatch: false,
		},
		{
			name:        "skip system roles filter shouldn't match the system role",
			role:        &systemRole,
			filter:      &RoleFilter{SkipSystemRoles: true},
			shouldMatch: false,
		},
		{
			name:        "skip system roles filter should match the regular role",
			role:        &regularRole,
			filter:      &RoleFilter{SkipSystemRoles: true},
			shouldMatch: true,
		},
		{
			name:        "skip system roles filter and incorrect search keywords shouldn't match the regular role",
			role:        &regularRole,
			filter:      &RoleFilter{SkipSystemRoles: true, SearchKeywords: []string{"xyz"}},
			shouldMatch: false,
		},
		{
			name:        "skip system roles filter and correct search keywords shouldn't match the system role",
			role:        &systemRole,
			filter:      &RoleFilter{SkipSystemRoles: true, SearchKeywords: []string{"bot"}},
			shouldMatch: false,
		},
		{
			name:        "skip system roles filter and correct search keywords should match the regular role",
			role:        &regularRole,
			filter:      &RoleFilter{SkipSystemRoles: true, SearchKeywords: []string{"appr"}},
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.role.CheckAndSetDefaults()
			require.NoError(t, err)
			require.Equal(t, tt.shouldMatch, tt.filter.Match(tt.role))
		})
	}
}

func TestRoleGitHubPermissions(t *testing.T) {
	role, err := NewRole("github-my-org", RoleSpecV6{
		Allow: RoleConditions{
			GitHubPermissions: []GitHubPermission{{
				Organizations: []string{"my-org"},
			}},
		},
		Deny: RoleConditions{
			GitHubPermissions: []GitHubPermission{{
				Organizations: []string{"jedi", "night-watch"},
			}},
		},
	})
	require.NoError(t, err)

	allowMatchers, err := role.GetLabelMatchers(Allow, KindGitServer)
	require.NoError(t, err)
	require.Equal(t, LabelMatchers{Labels: Labels{
		GitHubOrgLabel: []string{"my-org"},
	}}, allowMatchers)

	denyMatchers, err := role.GetLabelMatchers(Deny, KindGitServer)
	require.NoError(t, err)
	require.Equal(t, LabelMatchers{Labels: Labels{
		GitHubOrgLabel: []string{"jedi", "night-watch"},
	}}, denyMatchers)
}
