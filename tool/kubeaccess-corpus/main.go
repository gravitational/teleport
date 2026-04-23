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
// Entry point
// ----------------------------------------------------------------------

func run() error {
	organic, err := organicCases()
	if err != nil {
		return err
	}
	c := corpus{Cases: organic}
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
