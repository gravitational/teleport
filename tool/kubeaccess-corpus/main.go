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

// kubeaccess-corpus dumps a JSON corpus of Kubernetes access decisions
// for differential testing against the Lean model in ../../lean/.
//
// Section A: organic cases transcribed from TestCheckAccessToKubernetes
//            (lib/services/role_test.go:6828). Cases that use non-zero
//            AccessState (MFA/device trust) or KubernetesLabelsExpression
//            are skipped because they're out of v0 scope.
//
// Section B: synthetic combinatorial cases (added in exporter-synthetic
//            task; not present in this initial commit).
//
// Each case is a fixed-shape {name, source, roles, cluster, request,
// expected} record. The Go decision is the oracle — no manual labeling.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/services"
)

// ----------------------------------------------------------------------
// JSON schema — mirrors lean/Teleport/Types.lean.
// ----------------------------------------------------------------------

type labelsEntry struct {
	Key    string   `json:"key"`
	Values []string `json:"values"`
}

type clusterLabelsEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type kubeResourceJSON struct {
	Kind     string   `json:"kind"`
	Ns       string   `json:"ns"`
	Name     string   `json:"name"`
	APIGroup string   `json:"apiGroup"`
	Verbs    []string `json:"verbs"`
}

type roleConditionJSON struct {
	Namespaces          []string           `json:"namespaces"`
	KubernetesLabels    []labelsEntry      `json:"kubernetesLabels"`
	KubernetesResources []kubeResourceJSON `json:"kubernetesResources"`
}

type roleJSON struct {
	Name  string            `json:"name"`
	Allow roleConditionJSON `json:"allow"`
	Deny  roleConditionJSON `json:"deny"`
}

type clusterJSON struct {
	Name       string               `json:"name"`
	Labels     []clusterLabelsEntry `json:"labels"`
	TeleportNs string               `json:"teleportNs"`
}

type requestJSON struct {
	Resource      *kubeResourceJSON `json:"resource"`
	Verb          string            `json:"verb"`
	IsClusterWide bool              `json:"isClusterWide"`
}

type testCase struct {
	Name     string       `json:"name"`
	Source   string       `json:"source"`
	Roles    []roleJSON   `json:"roles"`
	Cluster  clusterJSON  `json:"cluster"`
	Request  *requestJSON `json:"request"`
	Expected string       `json:"expected"`
}

type corpus struct {
	Cases []testCase `json:"cases"`
}

// ----------------------------------------------------------------------
// Conversion helpers
// ----------------------------------------------------------------------

func labelsToEntries(l types.Labels) []labelsEntry {
	if len(l) == 0 {
		return []labelsEntry{}
	}
	out := make([]labelsEntry, 0, len(l))
	for k, vs := range l {
		copied := append([]string(nil), vs...)
		sort.Strings(copied)
		out = append(out, labelsEntry{Key: k, Values: copied})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func kubeResourcesToJSON(in []types.KubernetesResource) []kubeResourceJSON {
	out := make([]kubeResourceJSON, 0, len(in))
	for _, r := range in {
		verbs := append([]string(nil), r.Verbs...)
		sort.Strings(verbs)
		out = append(out, kubeResourceJSON{
			Kind:     r.Kind,
			Ns:       r.Namespace,
			Name:     r.Name,
			APIGroup: r.APIGroup,
			Verbs:    verbs,
		})
	}
	return out
}

func roleConditionToJSON(c types.RoleConditions) roleConditionJSON {
	namespaces := append([]string(nil), c.Namespaces...)
	if namespaces == nil {
		namespaces = []string{}
	}
	sort.Strings(namespaces)
	return roleConditionJSON{
		Namespaces:          namespaces,
		KubernetesLabels:    labelsToEntries(c.KubernetesLabels),
		KubernetesResources: kubeResourcesToJSON(c.KubernetesResources),
	}
}

func roleToJSON(r *types.RoleV6) roleJSON {
	return roleJSON{
		Name:  r.GetName(),
		Allow: roleConditionToJSON(r.GetRoleConditions(types.Allow)),
		Deny:  roleConditionToJSON(r.GetRoleConditions(types.Deny)),
	}
}

func clusterToJSON(name string, staticLabels map[string]string,
	dynamicLabels map[string]types.CommandLabelV2) clusterJSON {
	labels := make([]clusterLabelsEntry, 0, len(staticLabels)+len(dynamicLabels))
	for k, v := range staticLabels {
		labels = append(labels, clusterLabelsEntry{Key: k, Value: v})
	}
	for k, dl := range dynamicLabels {
		labels = append(labels, clusterLabelsEntry{Key: k, Value: dl.Result})
	}
	sort.Slice(labels, func(i, j int) bool { return labels[i].Key < labels[j].Key })
	return clusterJSON{
		Name:       name,
		Labels:     labels,
		TeleportNs: apidefaults.Namespace,
	}
}

// ----------------------------------------------------------------------
// Oracle: run Go's CheckAccess, return "allow" or "deny".
// ----------------------------------------------------------------------

func makeAccessChecker(roles []*types.RoleV6) services.AccessChecker {
	roleSet := make(services.RoleSet, len(roles))
	roleNames := make([]string, len(roles))
	for i, r := range roles {
		roleSet[i] = r
		roleNames[i] = r.GetName()
	}
	info := &services.AccessInfo{
		Username:                 "alice",
		Roles:                    roleNames,
		Traits:                   nil,
		AllowedResourceAccessIDs: nil,
	}
	return services.NewAccessCheckerWithRoleSet(info, "clustername", roleSet)
}

func runCheckAccess(roles []*types.RoleV6, cluster *types.KubernetesCluster) (string, error) {
	k8sV3, err := types.NewKubernetesClusterV3FromLegacyCluster(apidefaults.Namespace, cluster)
	if err != nil {
		return "", trace.Wrap(err)
	}
	checker := makeAccessChecker(roles)
	err = checker.CheckAccess(k8sV3, services.AccessState{})
	if err == nil {
		return "allow", nil
	}
	if trace.IsAccessDenied(err) {
		return "deny", nil
	}
	return "", trace.Wrap(err)
}

// ----------------------------------------------------------------------
// Fixtures — transcribed from TestCheckAccessToKubernetes
// (lib/services/role_test.go:6828). Inline in the test, so duplicated here.
// ----------------------------------------------------------------------

func fixtures() (
	clusterNoLabels, clusterWithLabels *types.KubernetesCluster,
	wildcardRole, matchingLabelsRole, noLabelsRole, mismatchingLabelsRole *types.RoleV6,
) {
	clusterNoLabels = &types.KubernetesCluster{Name: "no-labels"}
	clusterWithLabels = &types.KubernetesCluster{
		Name:          "with-labels",
		StaticLabels:  map[string]string{"foo": "bar"},
		DynamicLabels: map[string]types.CommandLabelV2{"baz": {Result: "qux"}},
	}
	wildcardRole = &types.RoleV6{
		Metadata: types.Metadata{Name: "wildcard-labels", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:       []string{apidefaults.Namespace},
				KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			},
		},
	}
	matchingLabelsRole = &types.RoleV6{
		Metadata: types.Metadata{Name: "matching-labels", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
				KubernetesLabels: types.Labels{
					"foo": apiutils.Strings{"bar"},
					"baz": apiutils.Strings{"qux"},
				},
			},
		},
	}
	noLabelsRole = &types.RoleV6{
		Metadata: types.Metadata{Name: "no-labels", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{Namespaces: []string{apidefaults.Namespace}},
		},
	}
	mismatchingLabelsRole = &types.RoleV6{
		Metadata: types.Metadata{Name: "mismatching-labels", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
				KubernetesLabels: types.Labels{
					"qux": apiutils.Strings{"baz"},
					"bar": apiutils.Strings{"foo"},
				},
			},
		},
	}
	// Normalize every role so serialized state matches post-CheckAndSetDefaults shape.
	for _, r := range []*types.RoleV6{wildcardRole, matchingLabelsRole, noLabelsRole, mismatchingLabelsRole} {
		if err := r.CheckAndSetDefaults(); err != nil {
			panic(err)
		}
	}
	return
}

// organicCases produces the Section-A corpus. Skips cases using
// non-zero AccessState or KubernetesLabelsExpression (per workspace
// research notes).
func organicCases() ([]testCase, error) {
	clusterNoLabels, clusterWithLabels, wildcardRole, matchingLabelsRole, noLabelsRole, mismatchingLabelsRole := fixtures()

	type entry struct {
		name    string
		roles   []*types.RoleV6
		cluster *types.KubernetesCluster
	}
	entries := []entry{
		{"empty-role-set-no-access", nil, clusterNoLabels},
		{"role-with-no-labels-no-access", []*types.RoleV6{noLabelsRole}, clusterNoLabels},
		{"wildcard-role-matches-no-labels-cluster", []*types.RoleV6{wildcardRole}, clusterNoLabels},
		{"wildcard-role-matches-with-labels-cluster", []*types.RoleV6{wildcardRole}, clusterWithLabels},
		{"labels-role-no-match-on-no-labels-cluster", []*types.RoleV6{matchingLabelsRole}, clusterNoLabels},
		{"labels-role-match-on-with-labels-cluster", []*types.RoleV6{matchingLabelsRole}, clusterWithLabels},
		{"mismatched-labels-no-match", []*types.RoleV6{mismatchingLabelsRole}, clusterWithLabels},
		{"one-role-in-set-matches", []*types.RoleV6{mismatchingLabelsRole, noLabelsRole, matchingLabelsRole}, clusterWithLabels},
	}

	out := make([]testCase, 0, len(entries))
	for _, e := range entries {
		expected, err := runCheckAccess(e.roles, e.cluster)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.name, err)
		}
		roles := make([]roleJSON, 0, len(e.roles))
		for _, r := range e.roles {
			roles = append(roles, roleToJSON(r))
		}
		out = append(out, testCase{
			Name:     "organic/" + e.name,
			Source:   "organic",
			Roles:    roles,
			Cluster:  clusterToJSON(e.cluster.Name, e.cluster.StaticLabels, e.cluster.DynamicLabels),
			Request:  nil,
			Expected: expected,
		})
	}
	return out, nil
}

// ----------------------------------------------------------------------
// Synthetic corpus — combinatorial coverage via parameterized role +
// cluster templates. Go's CheckAccess computes the expected decision.
// ----------------------------------------------------------------------

// mkRole builds a V6 role with explicit allow/deny conditions and
// runs CheckAndSetDefaults so serialized state matches post-normalization.
func mkRole(name string, allow, deny types.RoleConditions) *types.RoleV6 {
	r := &types.RoleV6{
		Metadata: types.Metadata{Name: name, Namespace: apidefaults.Namespace},
		Spec:     types.RoleSpecV6{Allow: allow, Deny: deny},
	}
	if err := r.CheckAndSetDefaults(); err != nil {
		panic(err)
	}
	return r
}

// Role template builders. Each takes a unique name suffix so multiple
// instances can coexist in a single role set.
func emptyCondRole(name string) *types.RoleV6 {
	return mkRole(name,
		types.RoleConditions{Namespaces: []string{apidefaults.Namespace}},
		types.RoleConditions{Namespaces: []string{apidefaults.Namespace}})
}

func wildcardAllowRole(name string) *types.RoleV6 {
	return mkRole(name,
		types.RoleConditions{
			Namespaces:       []string{apidefaults.Namespace},
			KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
		},
		types.RoleConditions{Namespaces: []string{apidefaults.Namespace}})
}

func singleLabelAllowRole(name, key, value string) *types.RoleV6 {
	return mkRole(name,
		types.RoleConditions{
			Namespaces:       []string{apidefaults.Namespace},
			KubernetesLabels: types.Labels{key: apiutils.Strings{value}},
		},
		types.RoleConditions{Namespaces: []string{apidefaults.Namespace}})
}

func multiLabelAllowRole(name string) *types.RoleV6 {
	return mkRole(name,
		types.RoleConditions{
			Namespaces: []string{apidefaults.Namespace},
			KubernetesLabels: types.Labels{
				"env":  apiutils.Strings{"prod"},
				"team": apiutils.Strings{"sre"},
			},
		},
		types.RoleConditions{Namespaces: []string{apidefaults.Namespace}})
}

func multiValueAllowRole(name, key string, values ...string) *types.RoleV6 {
	return mkRole(name,
		types.RoleConditions{
			Namespaces:       []string{apidefaults.Namespace},
			KubernetesLabels: types.Labels{key: apiutils.Strings(values)},
		},
		types.RoleConditions{Namespaces: []string{apidefaults.Namespace}})
}

func globValueAllowRole(name, key, pattern string) *types.RoleV6 {
	return mkRole(name,
		types.RoleConditions{
			Namespaces:       []string{apidefaults.Namespace},
			KubernetesLabels: types.Labels{key: apiutils.Strings{pattern}},
		},
		types.RoleConditions{Namespaces: []string{apidefaults.Namespace}})
}

func denyOnLabelRole(name, key, value string) *types.RoleV6 {
	return mkRole(name,
		types.RoleConditions{
			Namespaces:       []string{apidefaults.Namespace},
			KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
		},
		types.RoleConditions{
			Namespaces:       []string{apidefaults.Namespace},
			KubernetesLabels: types.Labels{key: apiutils.Strings{value}},
		})
}

// mkCluster builds a cluster with just static labels.
func mkCluster(name string, staticLabels map[string]string) *types.KubernetesCluster {
	return &types.KubernetesCluster{Name: name, StaticLabels: staticLabels}
}

func syntheticCases() ([]testCase, error) {
	// Cluster pool.
	clusters := []*types.KubernetesCluster{
		mkCluster("c-empty", nil),
		mkCluster("c-env-prod", map[string]string{"env": "prod"}),
		mkCluster("c-env-staging", map[string]string{"env": "staging"}),
		mkCluster("c-env-production-literal", map[string]string{"env": "production"}),
		mkCluster("c-env-prod-team-sre", map[string]string{"env": "prod", "team": "sre"}),
		mkCluster("c-team-sre", map[string]string{"team": "sre"}),
	}

	// Single-role scenarios. Each one-role set is exercised against every cluster.
	type singleRole struct {
		name    string
		builder func() *types.RoleV6
	}
	singles := []singleRole{
		{"empty", func() *types.RoleV6 { return emptyCondRole("r-empty") }},
		{"wildcard", func() *types.RoleV6 { return wildcardAllowRole("r-wildcard") }},
		{"env-prod-allow", func() *types.RoleV6 { return singleLabelAllowRole("r-env-prod", "env", "prod") }},
		{"multi-label-allow", func() *types.RoleV6 { return multiLabelAllowRole("r-multi") }},
		{"env-multi-value-allow", func() *types.RoleV6 { return multiValueAllowRole("r-multi-val", "env", "prod", "staging") }},
		{"env-glob-allow", func() *types.RoleV6 { return globValueAllowRole("r-glob", "env", "prod*") }},
		{"deny-env-prod", func() *types.RoleV6 { return denyOnLabelRole("r-deny", "env", "prod") }},
	}

	// Two-role scenarios — deliberately chosen to exercise interaction:
	// wildcard+deny, two allows, mismatch+match, etc.
	type pairRole struct {
		name    string
		builder func() []*types.RoleV6
	}
	pairs := []pairRole{
		{"wildcard-plus-deny-env-prod", func() []*types.RoleV6 {
			return []*types.RoleV6{
				wildcardAllowRole("r-w"),
				denyOnLabelRole("r-d", "env", "prod"),
			}
		}},
		{"env-prod-allow-plus-env-staging-allow", func() []*types.RoleV6 {
			return []*types.RoleV6{
				singleLabelAllowRole("r-p", "env", "prod"),
				singleLabelAllowRole("r-s", "env", "staging"),
			}
		}},
		{"mismatch-plus-wildcard", func() []*types.RoleV6 {
			return []*types.RoleV6{
				singleLabelAllowRole("r-m", "nosuch", "value"),
				wildcardAllowRole("r-w"),
			}
		}},
		{"two-empty-roles", func() []*types.RoleV6 {
			return []*types.RoleV6{
				emptyCondRole("r-e1"),
				emptyCondRole("r-e2"),
			}
		}},
		{"wildcard-plus-wildcard", func() []*types.RoleV6 {
			return []*types.RoleV6{
				wildcardAllowRole("r-w1"),
				wildcardAllowRole("r-w2"),
			}
		}},
		{"env-prod-with-deny-team-sre", func() []*types.RoleV6 {
			return []*types.RoleV6{
				singleLabelAllowRole("r-p", "env", "prod"),
				denyOnLabelRole("r-d", "team", "sre"),
			}
		}},
	}

	// Three-role scenarios — variety with ordering.
	triples := []pairRole{
		{"three-disjoint-allows", func() []*types.RoleV6 {
			return []*types.RoleV6{
				singleLabelAllowRole("r-a", "env", "prod"),
				singleLabelAllowRole("r-b", "team", "sre"),
				singleLabelAllowRole("r-c", "env", "staging"),
			}
		}},
		{"wildcard-with-two-denies", func() []*types.RoleV6 {
			return []*types.RoleV6{
				wildcardAllowRole("r-w"),
				denyOnLabelRole("r-d1", "env", "prod"),
				denyOnLabelRole("r-d2", "team", "sre"),
			}
		}},
		{"deny-first-then-allows", func() []*types.RoleV6 {
			return []*types.RoleV6{
				denyOnLabelRole("r-d", "env", "staging"),
				singleLabelAllowRole("r-a", "env", "prod"),
				wildcardAllowRole("r-w"),
			}
		}},
	}

	out := make([]testCase, 0, 200)

	emit := func(name string, roles []*types.RoleV6, cluster *types.KubernetesCluster) error {
		expected, err := runCheckAccess(roles, cluster)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		jroles := make([]roleJSON, 0, len(roles))
		for _, r := range roles {
			jroles = append(jroles, roleToJSON(r))
		}
		out = append(out, testCase{
			Name:     "synthetic/" + name,
			Source:   "synthetic",
			Roles:    jroles,
			Cluster:  clusterToJSON(cluster.Name, cluster.StaticLabels, cluster.DynamicLabels),
			Request:  nil,
			Expected: expected,
		})
		return nil
	}

	for _, s := range singles {
		for _, c := range clusters {
			if err := emit(s.name+"_on_"+c.Name, []*types.RoleV6{s.builder()}, c); err != nil {
				return nil, err
			}
		}
	}
	for _, p := range pairs {
		for _, c := range clusters {
			if err := emit(p.name+"_on_"+c.Name, p.builder(), c); err != nil {
				return nil, err
			}
		}
	}
	for _, t := range triples {
		for _, c := range clusters {
			if err := emit(t.name+"_on_"+c.Name, t.builder(), c); err != nil {
				return nil, err
			}
		}
	}

	// Deterministic sort by case name.
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// edgeCases exercises specific behaviors that combinatorial coverage in
// `syntheticCases` misses: implicit-wildcard deny injection and glob
// literal-`.` semantics. Namespace-mismatch is omitted because RoleV6's
// CheckAndSetDefaults rejects any namespaces value other than "default".
func edgeCases() ([]testCase, error) {
	// Case 1 — implicit-wildcard deny: role has empty deny.kubernetesLabels
	// but non-empty deny.kubernetesResources. Go's getKubeLabelMatchers
	// injects {*:*}, so the deny matches any cluster labels. T8 covers the
	// Lean side; this case validates cross-language agreement.
	denyResourcesOnlyRole := mkRole("r-deny-resources-only",
		types.RoleConditions{
			Namespaces:       []string{apidefaults.Namespace},
			KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
		},
		types.RoleConditions{
			Namespaces: []string{apidefaults.Namespace},
			KubernetesResources: []types.KubernetesResource{
				{Kind: "pods", Namespace: "*", Name: "*", APIGroup: "*", Verbs: []string{"*"}},
			},
		})

	// Case 2 — cluster label `version=1.2.3` against role pattern
	// `version=1.2.*`. QuoteMeta + glob expansion means `.` is literal;
	// this verifies Lean's globMatch agrees on non-trivial literal chars.
	versionedCluster := mkCluster("c-version-dotted", map[string]string{"version": "1.2.3"})
	globVersionRole := globValueAllowRole("r-version-glob", "version", "1.2.*")
	exactMismatchRole := singleLabelAllowRole("r-version-exact", "version", "1.2")

	entries := []struct {
		name    string
		roles   []*types.RoleV6
		cluster *types.KubernetesCluster
	}{
		{"implicit-wildcard-deny-injection", []*types.RoleV6{denyResourcesOnlyRole}, mkCluster("c-any", map[string]string{"any": "value"})},
		{"glob-literal-dot-matches", []*types.RoleV6{globVersionRole}, versionedCluster},
		{"glob-literal-dot-requires-suffix", []*types.RoleV6{exactMismatchRole}, versionedCluster},
	}

	out := make([]testCase, 0, len(entries))
	for _, e := range entries {
		expected, err := runCheckAccess(e.roles, e.cluster)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.name, err)
		}
		roles := make([]roleJSON, 0, len(e.roles))
		for _, r := range e.roles {
			roles = append(roles, roleToJSON(r))
		}
		out = append(out, testCase{
			Name:     "edge/" + e.name,
			Source:   "edge",
			Roles:    roles,
			Cluster:  clusterToJSON(e.cluster.Name, e.cluster.StaticLabels, e.cluster.DynamicLabels),
			Request:  nil,
			Expected: expected,
		})
	}
	return out, nil
}

// ----------------------------------------------------------------------
// Entry point
// ----------------------------------------------------------------------

func run() error {
	organic, err := organicCases()
	if err != nil {
		return err
	}
	synthetic, err := syntheticCases()
	if err != nil {
		return err
	}
	edge, err := edgeCases()
	if err != nil {
		return err
	}
	all := append(organic, synthetic...)
	all = append(all, edge...)
	c := corpus{Cases: all}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(c)
}

func main() {
	if err := run(); err != nil {
		if errors.Is(err, trace.Unwrap(err)) || trace.Unwrap(err) == nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "%s\n", trace.DebugReport(err))
		}
		os.Exit(1)
	}
}
