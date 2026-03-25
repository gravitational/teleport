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

package proxy

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
)

func TestNewMatcher(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		kind           string
		verb           string
		apiGroup       string
		namespace      string
		allowed        []types.KubernetesResource
		wantDefault    bool
		wantAllowCount int
		wantDenyCount  int
	}{
		{
			name:        "returns defaultMatcher for namespace kind",
			kind:        "namespaces",
			verb:        types.KubeVerbList,
			wantDefault: true,
		},
		{
			name: "compiles successfully for pod kind",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "default", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			wantAllowCount: 1,
		},
		{
			name: "filters out rules with non-matching kind",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "*"},
				{Kind: types.KindKubeConfigmap, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "*"},
			},
			wantAllowCount: 1,
		},
		{
			name: "includes wildcard kind rules",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.Wildcard, Namespace: "*", Name: "*", Verbs: []string{types.Wildcard}, APIGroup: "*"},
			},
			wantAllowCount: 1,
		},
		{
			name: "filters out rules with non-matching verb",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbCreate}, APIGroup: "*"},
			},
			wantAllowCount: 0,
		},
		{
			name: "wildcard verb only recognized as first element",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbGet, types.Wildcard}, APIGroup: "*"},
			},
			wantAllowCount: 0,
		},
		{
			name: "wildcard verb as first element matches any verb",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{types.Wildcard}, APIGroup: "*"},
			},
			wantAllowCount: 1,
		},
		{
			name: "empty verbs matches nothing",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{}, APIGroup: "*"},
			},
			wantAllowCount: 0,
		},
		{
			name: "returns defaultMatcher for invalid regex",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "^[invalid$", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "*"},
			},
			wantDefault: true,
		},
		{
			name:     "filters out rules with non-matching literal API group",
			kind:     types.KindKubePod,
			verb:     types.KubeVerbList,
			apiGroup: "",
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "apps"},
				{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			wantAllowCount: 1,
		},
		{
			name:     "keeps rules with wildcard API group",
			kind:     types.KindKubePod,
			verb:     types.KubeVerbList,
			apiGroup: "",
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "*"},
			},
			wantAllowCount: 1,
		},
		{
			name:     "keeps rules with regex API group",
			kind:     types.KindKubePod,
			verb:     types.KubeVerbList,
			apiGroup: "apps",
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "^apps|extensions$"},
			},
			wantAllowCount: 1,
		},
		{
			name:      "filters out rules with non-matching literal namespace",
			kind:      types.KindKubePod,
			verb:      types.KubeVerbList,
			namespace: "default",
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "staging", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
				{Kind: types.KindKubePod, Namespace: "default", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			wantAllowCount: 1,
		},
		{
			name:      "keeps all rules when request is cluster-wide",
			kind:      types.KindKubePod,
			verb:      types.KubeVerbList,
			namespace: "",
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "staging", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
				{Kind: types.KindKubePod, Namespace: "default", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			wantAllowCount: 2,
		},
		{
			name:      "keeps rules with wildcard namespace when targeting specific namespace",
			kind:      types.KindKubePod,
			verb:      types.KubeVerbList,
			namespace: "default",
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			wantAllowCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mr := metaResource{
				requestedResource: apiResource{resourceKind: tt.kind, apiGroup: tt.apiGroup, namespace: tt.namespace},
				verb:              tt.verb,
			}
			log := slog.New(slog.DiscardHandler)
			m := newMatcher(mr, tt.allowed, nil, log)
			if tt.wantDefault {
				require.IsType(t, &defaultMatcher{}, m)
				return
			}
			fm, ok := m.(*fastMatcher)
			require.True(t, ok, "expected *fastMatcher, got %T", m)
			require.Len(t, fm.allowRules, tt.wantAllowCount)
			require.Len(t, fm.denyRules, tt.wantDenyCount)
		})
	}
}

// TestNamespaceKindFallback verifies that namespace kind requests go through defaultMatcher
// and produce correct results for all the special cases in KubeResourceMatchesRegex:
// - pod rule in namespace "foo" → user can list namespace "foo"
// - explicit namespace rule → matched by name
// - wildcard kind with namespace swapping
// If someone removes the namespace guard from newMatcher, these will fail.
func TestNamespaceKindFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		verb    string
		allowed []types.KubernetesResource
		denied  []types.KubernetesResource
		input   types.KubernetesResource
	}{
		{
			name: "pod rule grants read-only namespace visibility",
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "production", Name: "*", Verbs: []string{types.Wildcard}},
			},
			input: types.KubernetesResource{Kind: "namespaces", Name: "production", Verbs: []string{types.KubeVerbList}},
		},
		{
			name: "pod rule does not grant visibility to other namespaces",
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "production", Name: "*", Verbs: []string{types.Wildcard}},
			},
			input: types.KubernetesResource{Kind: "namespaces", Name: "staging", Verbs: []string{types.KubeVerbList}},
		},
		{
			name: "explicit namespace rule matched by name",
			verb: types.KubeVerbGet,
			allowed: []types.KubernetesResource{
				{Kind: "namespaces", Name: "default", Verbs: []string{types.KubeVerbGet}},
			},
			input: types.KubernetesResource{Kind: "namespaces", Name: "default", Verbs: []string{types.KubeVerbGet}},
		},
		{
			name: "explicit namespace rule does not match different name",
			verb: types.KubeVerbGet,
			allowed: []types.KubernetesResource{
				{Kind: "namespaces", Name: "default", Verbs: []string{types.KubeVerbGet}},
			},
			input: types.KubernetesResource{Kind: "namespaces", Name: "staging", Verbs: []string{types.KubeVerbGet}},
		},
		{
			name: "wildcard kind with wildcard namespace uses name as target",
			verb: types.KubeVerbGet,
			allowed: []types.KubernetesResource{
				{Kind: types.Wildcard, APIGroup: types.Wildcard, Namespace: types.Wildcard, Name: types.Wildcard, Verbs: []string{types.Wildcard}},
			},
			input: types.KubernetesResource{Kind: "namespaces", Name: "anything", Verbs: []string{types.KubeVerbGet}},
		},
		{
			name: "deny rule blocks namespace visibility",
			verb: types.KubeVerbGet,
			allowed: []types.KubernetesResource{
				{Kind: types.Wildcard, APIGroup: types.Wildcard, Namespace: types.Wildcard, Name: types.Wildcard, Verbs: []string{types.Wildcard}},
			},
			denied: []types.KubernetesResource{
				{Kind: types.Wildcard, APIGroup: types.Wildcard, Namespace: types.Wildcard, Name: types.Wildcard, Verbs: []string{types.Wildcard}},
			},
			input: types.KubernetesResource{Kind: "namespaces", Name: "default", Verbs: []string{types.KubeVerbGet}},
		},
		{
			name: "pod rule does not grant write access to namespace",
			verb: types.KubeVerbUpdate,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "production", Name: "*", Verbs: []string{types.Wildcard}},
			},
			input: types.KubernetesResource{Kind: "namespaces", Name: "production", Verbs: []string{types.KubeVerbUpdate}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mr := metaResource{
				requestedResource: apiResource{resourceKind: "namespaces"},
				verb:              tt.verb,
			}
			log := slog.New(slog.DiscardHandler)
			m := newMatcher(mr, tt.allowed, tt.denied, log)
			require.IsType(t, &defaultMatcher{}, m)

			expected, err := matchKubernetesResource(tt.input, true, tt.allowed, tt.denied)
			require.NoError(t, err)

			got, err := m.match(tt.input.Name, tt.input.Namespace, tt.input.APIGroup)
			require.NoError(t, err)
			require.Equal(t, expected, got)
		})
	}
}

// TestMatcherEquivalence verifies that fastMatcher and defaultMatcher produce the same results.
// The original function is the source of truth, no hardcoded expected values.
func TestMatcherEquivalence(t *testing.T) {
	t.Parallel()

	type input struct {
		resource              types.KubernetesResource
		isClusterWideResource bool
	}
	pod := func(ns, name string) input {
		return input{resource: types.KubernetesResource{Kind: types.KindKubePod, Namespace: ns, Name: name, Verbs: []string{types.KubeVerbList}, APIGroup: ""}}
	}
	deploy := func(ns, name, apiGroup string) input {
		return input{resource: types.KubernetesResource{Kind: types.KindKubeDeployment, Namespace: ns, Name: name, Verbs: []string{types.KubeVerbList}, APIGroup: apiGroup}}
	}

	tests := []struct {
		name     string
		kind     string
		verb     string
		apiGroup string
		allowed  []types.KubernetesResource
		denied   []types.KubernetesResource
		inputs   []input
	}{
		// -- allow rule patterns --
		{
			name: "wildcard allows everything",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "*"},
			},
			inputs: []input{
				pod("default", "nginx"),
				pod("kube-system", "coredns"),
				pod("", "orphan"),
			},
		},
		{
			name: "exact namespace and name matching",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "default", Name: "nginx", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			inputs: []input{
				pod("default", "nginx"),
				pod("default", "redis"),
				pod("kube-system", "nginx"),
			},
		},
		{
			name: "glob name pattern",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "nginx-*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			inputs: []input{
				pod("default", "nginx-deployment-abc123"),
				pod("default", "redis"),
			},
		},
		{
			name: "regex name pattern",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "^nginx-[a-z]+$", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			inputs: []input{
				pod("default", "nginx-deployment"),
				pod("default", "nginx-123"),
				pod("default", "redis"),
			},
		},
		{
			name: "multiple allow rules",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "default", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
				{Kind: types.KindKubePod, Namespace: "staging", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			inputs: []input{
				pod("default", "nginx"),
				pod("staging", "nginx"),
				pod("production", "nginx"),
			},
		},
		{
			name:    "no allow rules means no access",
			kind:    types.KindKubePod,
			verb:    types.KubeVerbList,
			allowed: []types.KubernetesResource{},
			inputs: []input{
				pod("default", "nginx"),
			},
		},
		{
			name: "empty namespace input with specific and wildcard rules",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "default", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
				{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			inputs: []input{
				pod("", "nginx"),
				pod("default", "nginx"),
			},
		},
		{
			name:     "apigroup match",
			kind:     types.KindKubeDeployment,
			verb:     types.KubeVerbList,
			apiGroup: "apps",
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubeDeployment, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "apps"},
			},
			inputs: []input{
				deploy("default", "nginx", "apps"),
			},
		},
		{
			name:     "apigroup mismatch",
			kind:     types.KindKubeDeployment,
			verb:     types.KubeVerbList,
			apiGroup: "extensions",
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubeDeployment, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "apps"},
			},
			inputs: []input{
				deploy("default", "nginx", "extensions"),
			},
		},
		// -- glob and exact patterns with deny --
		{
			name: "glob and exact patterns with deny",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "default", Name: "nginx-*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
				{Kind: types.KindKubePod, Namespace: "staging", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
				{Kind: types.Wildcard, Namespace: "monitoring", Name: "*", Verbs: []string{types.Wildcard}, APIGroup: "*"},
			},
			denied: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "staging", Name: "secret-*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			inputs: []input{
				pod("default", "nginx-abc"),
				pod("default", "redis"),
				pod("staging", "app"),
				pod("staging", "secret-data"),
				pod("monitoring", "prometheus"),
				pod("production", "app"),
				pod("", "orphan"),
				{resource: types.KubernetesResource{Kind: types.KindKubePod, Namespace: "", Name: "cluster-wide-pod", Verbs: []string{types.KubeVerbList}, APIGroup: ""}, isClusterWideResource: true},
			},
		},
		// -- deny rule patterns --
		{
			name: "deny with exact namespace",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "*"},
			},
			denied: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "kube-system", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "*"},
			},
			inputs: []input{
				pod("kube-system", "coredns"),
				pod("default", "nginx"),
			},
		},
		{
			name: "deny with regex namespace pattern",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			denied: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "^kube-.*$", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			inputs: []input{
				pod("kube-system", "coredns"),
				pod("kube-public", "info"),
				pod("default", "nginx"),
			},
		},
		{
			name: "deny with regex name pattern",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			denied: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "^(secret|internal)-.*$", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			inputs: []input{
				pod("default", "secret-data"),
				pod("default", "internal-api"),
				pod("default", "nginx"),
				pod("default", "secret"),
			},
		},
		{
			name: "deny with glob name pattern",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "*"},
			},
			denied: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "secret-*", Verbs: []string{types.KubeVerbList}, APIGroup: "*"},
			},
			inputs: []input{
				pod("default", "secret-data"),
				pod("default", "nginx"),
			},
		},
		{
			name: "deny with exact name",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "*"},
			},
			denied: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "secret-data", Verbs: []string{types.KubeVerbList}, APIGroup: "*"},
			},
			inputs: []input{
				pod("default", "secret-data"),
				pod("default", "secret-other"),
				pod("default", "nginx"),
			},
		},
		{
			name: "multiple deny rules with mixed patterns",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			denied: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "kube-system", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
				{Kind: types.KindKubePod, Namespace: "*", Name: "secret-*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			inputs: []input{
				pod("kube-system", "coredns"),
				pod("default", "secret-data"),
				pod("default", "nginx"),
				pod("kube-system", "secret-data"),
			},
		},
		{
			name: "deny with glob namespace pattern",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			denied: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "kube-*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			inputs: []input{
				pod("kube-system", "coredns"),
				pod("kube-public", "info"),
				pod("default", "nginx"),
			},
		},
		{
			name:     "deny with regex apigroup",
			kind:     types.KindKubeDeployment,
			verb:     types.KubeVerbList,
			apiGroup: "apps",
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubeDeployment, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "*"},
			},
			denied: []types.KubernetesResource{
				{Kind: types.KindKubeDeployment, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "^apps|extensions$"},
			},
			inputs: []input{
				deploy("default", "nginx", "apps"),
				deploy("staging", "redis", "apps"),
			},
		},
		{
			name: "narrow allow with overlapping deny",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "default", Name: "nginx-*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			denied: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "nginx-secret", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			inputs: []input{
				pod("default", "nginx-web"),
				pod("default", "nginx-secret"),
				pod("default", "redis"),
				pod("staging", "nginx-web"),
			},
		},
		// -- namespace edge cases --
		{
			name: "allow with empty namespace rule",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			inputs: []input{
				pod("default", "nginx"),
				pod("", "orphan"),
				{resource: types.KubernetesResource{Kind: types.KindKubePod, Namespace: "", Name: "orphan", Verbs: []string{types.KubeVerbList}, APIGroup: ""}, isClusterWideResource: true},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mr := metaResource{
				requestedResource: apiResource{resourceKind: tc.kind, apiGroup: tc.apiGroup},
				verb:              tc.verb,
			}

			log := slog.New(slog.DiscardHandler)
			fm := newMatcher(mr, tc.allowed, tc.denied, log)
			require.IsType(t, &fastMatcher{}, fm)

			dm := &defaultMatcher{
				kind:             tc.kind,
				verb:             tc.verb,
				allowedResources: tc.allowed,
				deniedResources:  tc.denied,
			}

			for _, tt := range tc.inputs {
				t.Run(fmt.Sprintf("%s/%s", tt.resource.Namespace, tt.resource.Name), func(t *testing.T) {
					t.Parallel()

					// Use the original matchKubernetesResource as source of truth.
					expected, err := matchKubernetesResource(tt.resource, tt.isClusterWideResource, tc.allowed, tc.denied)
					require.NoError(t, err)

					fastResult, err := fm.match(tt.resource.Name, tt.resource.Namespace, tt.resource.APIGroup)
					require.NoError(t, err)
					require.Equal(t, expected, fastResult, "fastMatcher mismatch for %s/%s", tt.resource.Namespace, tt.resource.Name)

					defaultResult, err := dm.match(tt.resource.Name, tt.resource.Namespace, tt.resource.APIGroup)
					require.NoError(t, err)
					require.Equal(t, expected, defaultResult, "defaultMatcher mismatch for %s/%s", tt.resource.Namespace, tt.resource.Name)
				})
			}
		})
	}
}

// TestMatcherEquivalenceMatrix generates a cross product of rule patterns and resource inputs.
// Then verifies fastMatcher, defaultMatcher and the original matchKubernetesResource all agree.
// This catches regressions that hand-picked cases might miss.
func TestMatcherEquivalenceMatrix(t *testing.T) {
	t.Parallel()

	// Rule patterns covering literals, globs, regexes, wildcards, and empty strings.
	namePatterns := []string{"nginx", "nginx-*", "^nginx-[a-z]+$", "*", ""}
	nsPatterns := []string{"default", "kube-*", "^staging-.*$", "*", ""}
	apiGroupPatterns := []string{"", "apps", "*"}

	// Resources to match against each rule set.
	names := []string{"nginx", "nginx-web", "nginx-123", "redis", ""}
	namespaces := []string{"default", "kube-system", "staging-prod", "other", ""}
	apiGroups := []string{"", "apps"}

	kind := types.KindKubePod
	verb := types.KubeVerbList

	for _, nameP := range namePatterns {
		for _, nsP := range nsPatterns {
			for _, agP := range apiGroupPatterns {
				allowed := []types.KubernetesResource{
					{Kind: kind, Namespace: nsP, Name: nameP, Verbs: []string{verb}, APIGroup: agP},
				}

				for _, inputName := range names {
					for _, inputNS := range namespaces {
						for _, inputAG := range apiGroups {
							mr := metaResource{
								requestedResource: apiResource{resourceKind: kind, apiGroup: inputAG},
								verb:              verb,
							}
							log := slog.New(slog.DiscardHandler)
							m := newMatcher(mr, allowed, nil, log)
							fm, ok := m.(*fastMatcher)
							if !ok {
								// Invalid regex — fell back to defaultMatcher. Skip.
								continue
							}

							resource := types.KubernetesResource{
								Kind: kind, Namespace: inputNS, Name: inputName,
								Verbs: []string{verb}, APIGroup: inputAG,
							}

							expected, err := matchKubernetesResource(resource, false, allowed, nil)
							if err != nil {
								continue
							}

							fastResult, err := fm.match(inputName, inputNS, inputAG)
							if err != nil {
								continue
							}

							dm := &defaultMatcher{
								kind: kind, verb: verb,
								allowedResources: allowed,
							}
							defaultResult, err := dm.match(inputName, inputNS, inputAG)
							if err != nil {
								continue
							}

							label := fmt.Sprintf("allow[ns=%q,name=%q,ag=%q]/input[ns=%q,name=%q,ag=%q]",
								nsP, nameP, agP, inputNS, inputName, inputAG)

							if fastResult != expected {
								t.Errorf("fastMatcher mismatch: %s: got %v, want %v", label, fastResult, expected)
							}
							if defaultResult != expected {
								t.Errorf("defaultMatcher mismatch: %s: got %v, want %v", label, defaultResult, expected)
							}
						}
					}
				}
			}
		}
	}
}

// buildUnstructuredList creates an unstructured list with the given items for benchmarks.
func buildUnstructuredList(listKind, apiVersion string, items []map[string]any) *unstructured.Unstructured {
	anyItems := make([]any, len(items))
	for i, item := range items {
		anyItems[i] = item
	}
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": apiVersion,
			"kind":       listKind,
			"metadata":   map[string]any{"resourceVersion": ""},
			"items":      anyItems,
		},
	}
}

// BenchmarkFilterObj benchmarks the full FilterObj path through newResourceFilterer,
// comparing fast matcher vs fallback at different item and rule counts.
func BenchmarkFilterObj(b *testing.B) {
	log := slog.New(slog.DiscardHandler)

	mr := metaResource{
		requestedResource: apiResource{
			resourceKind: types.KindKubePod,
			apiGroup:     "",
		},
		resourceDefinition: &metav1.APIResource{Namespaced: true},
		verb:               types.KubeVerbList,
	}

	namespaces := []string{"default", "staging", "monitoring", "kube-system", "production"}

	buildRules := func(ruleCount int) (allowed, denied []types.KubernetesResource) {
		for i := range ruleCount {
			allowed = append(allowed, types.KubernetesResource{
				Kind:      types.KindKubePod,
				Namespace: namespaces[i%len(namespaces)],
				Name:      fmt.Sprintf("app-%d-*", i),
				Verbs:     []string{types.KubeVerbList},
				APIGroup:  "",
			})
		}
		denied = []types.KubernetesResource{
			{Kind: types.KindKubePod, Namespace: "default", Name: "secret-*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
		}
		return allowed, denied
	}

	buildItems := func(count int) ([]map[string]any, []any) {
		items := make([]map[string]any, count)
		for i := range items {
			items[i] = map[string]any{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      fmt.Sprintf("pod-%d", i),
					"namespace": namespaces[i%len(namespaces)],
				},
			}
		}
		saved := make([]any, len(items))
		for i, item := range items {
			saved[i] = item
		}
		return items, saved
	}

	newFilterer := func(b *testing.B, items []map[string]any, allowed, denied []types.KubernetesResource, useFastMatcher bool) (*resourceFilterer, *unstructured.Unstructured) {
		b.Helper()
		wrapper := newResourceFilterer(mr, &globalKubeCodecs, newMatcher(mr, allowed, denied, log), log)
		filter, err := wrapper(responsewriters.DefaultContentType, 200)
		require.NoError(b, err)
		rf := filter.(*resourceFilterer)
		if useFastMatcher {
			require.IsType(b, &fastMatcher{}, rf.matcher)
		} else {
			rf.matcher = &defaultMatcher{
				kind:             mr.requestedResource.resourceKind,
				verb:             mr.verb,
				isClusterWide:    mr.isClusterWideResource(),
				allowedResources: allowed,
				deniedResources:  denied,
			}
		}
		obj := buildUnstructuredList("PodList", "v1", items)
		return rf, obj
	}

	for _, ruleCount := range []int{4, 50, 150, 4000} {
		allowed, denied := buildRules(ruleCount)

		for _, itemCount := range []int{500, 5000} {
			items, savedItems := buildItems(itemCount)
			prefix := fmt.Sprintf("%d_rules/%d_items", ruleCount, itemCount)

			b.Run(prefix+"/fast_matcher", func(b *testing.B) {
				rf, obj := newFilterer(b, items, allowed, denied, true)
				require.IsType(b, &fastMatcher{}, rf.matcher)
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					obj.Object["items"] = savedItems
					rf.FilterObj(obj)
				}
			})

			b.Run(prefix+"/default_matcher", func(b *testing.B) {
				rf, obj := newFilterer(b, items, allowed, denied, false)
				require.IsType(b, &defaultMatcher{}, rf.matcher)
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					obj.Object["items"] = savedItems
					rf.FilterObj(obj)
				}
			})
		}
	}
}
