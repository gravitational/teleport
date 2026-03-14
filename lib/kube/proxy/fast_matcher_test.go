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

func TestTryCompileFastMatcher(t *testing.T) {
	t.Parallel()

	manyRules := make([]types.KubernetesResource, maxFastMatcherRules+1)
	for i := range manyRules {
		manyRules[i] = types.KubernetesResource{
			Kind: types.KindKubePod, Namespace: fmt.Sprintf("ns-%d", i),
			Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "",
		}
	}

	tests := []struct {
		name           string
		kind           string
		verb           string
		apiGroup       string
		namespace      string
		allowed        []types.KubernetesResource
		wantNil        bool
		wantErr        bool
		wantAllowCount int
		wantDenyCount  int
	}{
		{
			name:    "returns nil for namespace kind",
			kind:    "namespaces",
			verb:    types.KubeVerbList,
			wantNil: true,
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
			name: "returns error for invalid regex",
			kind: types.KindKubePod,
			verb: types.KubeVerbList,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "^[invalid$", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "*"},
			},
			wantErr: true,
			wantNil: true,
		},
		{
			name:    "returns nil when matching rules exceed threshold",
			kind:    types.KindKubePod,
			verb:    types.KubeVerbList,
			allowed: manyRules,
			wantNil: true,
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
			fm, err := tryCompileFastMatcher(mr, tt.allowed, nil)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if tt.wantNil {
				require.Nil(t, fm)
				return
			}
			require.NotNil(t, fm)
			require.Len(t, fm.allowRules, tt.wantAllowCount)
			require.Len(t, fm.denyRules, tt.wantDenyCount)
		})
	}
}

func TestFastResourceMatcher_Match(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		kind      string
		allowed   []types.KubernetesResource
		denied    []types.KubernetesResource
		input     matchInput
		wantMatch bool
	}{
		{
			name: "wildcard allows everything",
			kind: types.KindKubePod,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "*"},
			},
			input:     matchInput{name: "nginx", namespace: "default", apiGroup: ""},
			wantMatch: true,
		},
		{
			name: "exact namespace match",
			kind: types.KindKubePod,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "default", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			input:     matchInput{name: "nginx", namespace: "default", apiGroup: ""},
			wantMatch: true,
		},
		{
			name: "namespace mismatch",
			kind: types.KindKubePod,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "default", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			input:     matchInput{name: "nginx", namespace: "kube-system", apiGroup: ""},
			wantMatch: false,
		},
		{
			name: "exact name match",
			kind: types.KindKubePod,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "nginx", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			input:     matchInput{name: "nginx", namespace: "default", apiGroup: ""},
			wantMatch: true,
		},
		{
			name: "name mismatch",
			kind: types.KindKubePod,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "nginx", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			input:     matchInput{name: "redis", namespace: "default", apiGroup: ""},
			wantMatch: false,
		},
		{
			name: "glob name pattern",
			kind: types.KindKubePod,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "nginx-*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			input:     matchInput{name: "nginx-deployment-abc123", namespace: "default", apiGroup: ""},
			wantMatch: true,
		},
		{
			name: "regex name pattern",
			kind: types.KindKubePod,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "^nginx-[a-z]+$", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			input:     matchInput{name: "nginx-deployment", namespace: "default", apiGroup: ""},
			wantMatch: true,
		},
		{
			name: "deny rule takes precedence",
			kind: types.KindKubePod,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "*"},
			},
			denied: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "kube-system", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "*"},
			},
			input:     matchInput{name: "coredns", namespace: "kube-system", apiGroup: ""},
			wantMatch: false,
		},
		{
			name: "deny rule does not affect other namespaces",
			kind: types.KindKubePod,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "*"},
			},
			denied: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "kube-system", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "*"},
			},
			input:     matchInput{name: "nginx", namespace: "default", apiGroup: ""},
			wantMatch: true,
		},
		{
			name: "multiple allow rules - first matches",
			kind: types.KindKubePod,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "default", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
				{Kind: types.KindKubePod, Namespace: "staging", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			input:     matchInput{name: "nginx", namespace: "default", apiGroup: ""},
			wantMatch: true,
		},
		{
			name: "multiple allow rules - second matches",
			kind: types.KindKubePod,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "default", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
				{Kind: types.KindKubePod, Namespace: "staging", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			input:     matchInput{name: "nginx", namespace: "staging", apiGroup: ""},
			wantMatch: true,
		},
		{
			name: "multiple allow rules - none match",
			kind: types.KindKubePod,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "default", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
				{Kind: types.KindKubePod, Namespace: "staging", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			input:     matchInput{name: "nginx", namespace: "production", apiGroup: ""},
			wantMatch: false,
		},
		{
			name: "empty namespace input with specific namespace rule",
			kind: types.KindKubePod,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "default", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			input:     matchInput{name: "nginx", namespace: "", apiGroup: ""},
			wantMatch: false,
		},
		{
			name: "empty namespace input with wildcard namespace rule",
			kind: types.KindKubePod,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
			},
			input:     matchInput{name: "nginx", namespace: "", apiGroup: ""},
			wantMatch: true,
		},
		{
			name: "apigroup matching",
			kind: types.KindKubeDeployment,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubeDeployment, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "apps"},
			},
			input:     matchInput{name: "nginx", namespace: "default", apiGroup: "apps"},
			wantMatch: true,
		},
		{
			name: "apigroup mismatch",
			kind: types.KindKubeDeployment,
			allowed: []types.KubernetesResource{
				{Kind: types.KindKubeDeployment, Namespace: "*", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: "apps"},
			},
			input:     matchInput{name: "nginx", namespace: "default", apiGroup: "extensions"},
			wantMatch: false,
		},
		{
			name:      "no rules means no access",
			kind:      types.KindKubePod,
			allowed:   []types.KubernetesResource{},
			input:     matchInput{name: "nginx", namespace: "default", apiGroup: ""},
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mr := metaResource{
				requestedResource: apiResource{resourceKind: tt.kind, apiGroup: tt.input.apiGroup},
				verb:              types.KubeVerbList,
			}
			fm, err := tryCompileFastMatcher(mr, tt.allowed, tt.denied)
			require.NoError(t, err)
			require.NotNil(t, fm)
			got, err := fm.match(tt.input.name, tt.input.namespace, tt.input.apiGroup)
			require.NoError(t, err)
			require.Equal(t, tt.wantMatch, got)
		})
	}
}

type matchInput struct {
	name      string
	namespace string
	apiGroup  string
}

// TestFastMatcherEquivalence verifies that the fast matcher produces the same
// results as matchKubernetesResource for the common (non-namespace) case.
func TestFastMatcherEquivalence(t *testing.T) {
	t.Parallel()

	allowed := []types.KubernetesResource{
		{Kind: types.KindKubePod, Namespace: "default", Name: "nginx-*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
		{Kind: types.KindKubePod, Namespace: "staging", Name: "*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
		{Kind: types.Wildcard, Namespace: "monitoring", Name: "*", Verbs: []string{types.Wildcard}, APIGroup: "*"},
	}
	denied := []types.KubernetesResource{
		{Kind: types.KindKubePod, Namespace: "staging", Name: "secret-*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
	}

	mr := metaResource{
		requestedResource: apiResource{resourceKind: types.KindKubePod},
		verb:              types.KubeVerbList,
	}
	fm, err := tryCompileFastMatcher(mr, allowed, denied)
	require.NoError(t, err)
	require.NotNil(t, fm)

	inputs := []struct {
		resource              types.KubernetesResource
		isClusterWideResource bool
	}{
		{resource: types.KubernetesResource{Kind: types.KindKubePod, Namespace: "default", Name: "nginx-abc", Verbs: []string{types.KubeVerbList}, APIGroup: ""}},
		{resource: types.KubernetesResource{Kind: types.KindKubePod, Namespace: "default", Name: "redis", Verbs: []string{types.KubeVerbList}, APIGroup: ""}},
		{resource: types.KubernetesResource{Kind: types.KindKubePod, Namespace: "staging", Name: "app", Verbs: []string{types.KubeVerbList}, APIGroup: ""}},
		{resource: types.KubernetesResource{Kind: types.KindKubePod, Namespace: "staging", Name: "secret-data", Verbs: []string{types.KubeVerbList}, APIGroup: ""}},
		{resource: types.KubernetesResource{Kind: types.KindKubePod, Namespace: "monitoring", Name: "prometheus", Verbs: []string{types.KubeVerbList}, APIGroup: ""}},
		{resource: types.KubernetesResource{Kind: types.KindKubePod, Namespace: "production", Name: "app", Verbs: []string{types.KubeVerbList}, APIGroup: ""}},
		{resource: types.KubernetesResource{Kind: types.KindKubePod, Namespace: "", Name: "orphan", Verbs: []string{types.KubeVerbList}, APIGroup: ""}},
		{resource: types.KubernetesResource{Kind: types.KindKubePod, Namespace: "", Name: "cluster-wide-pod", Verbs: []string{types.KubeVerbList}, APIGroup: ""}, isClusterWideResource: true},
	}

	for _, tt := range inputs {
		t.Run(fmt.Sprintf("%s/%s", tt.resource.Namespace, tt.resource.Name), func(t *testing.T) {
			t.Parallel()
			expected, err := matchKubernetesResource(tt.resource, tt.isClusterWideResource, allowed, denied)
			require.NoError(t, err)
			got, err := fm.match(tt.resource.Name, tt.resource.Namespace, tt.resource.APIGroup)
			require.NoError(t, err)
			require.Equal(t, expected, got,
				"mismatch for %s/%s: matchKubernetesResource=%v, fastMatcher=%v",
				tt.resource.Namespace, tt.resource.Name, expected, got,
			)
		})
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
		wrapper := newResourceFilterer(mr, &globalKubeCodecs, allowed, denied, log)
		filter, err := wrapper(responsewriters.DefaultContentType, 200)
		require.NoError(b, err)
		rf := filter.(*resourceFilterer)
		if useFastMatcher {
			if _, ok := rf.matcher.(*fastResourceMatcher); !ok {
				// For high rule counts that exceed maxFastMatcherRules, compile the fast
				// matcher directly so we can benchmark it against the fallback path.
				filteredAllowed := filterRules(mr, allowed)
				filteredDenied := filterRules(mr, denied)
				fm, err := compileFastMatcher(filteredAllowed, filteredDenied)
				require.NoError(b, err)
				rf.matcher = fm
			}
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
				require.IsType(b, &fastResourceMatcher{}, rf.matcher)
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
