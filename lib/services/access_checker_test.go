/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package services

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestAccessCheckerKubeResources(t *testing.T) {
	emptySet := []types.KubernetesResource{}
	kubeUsers := []string{"user1"}
	kubeAnyLabels, kubeDevLabels := types.Labels{"*": {"*"}}, types.Labels{"env": {"dev"}}
	devKubeCluster := newKubeCluster(t, "dev", map[string]string{"env": "dev"})
	prodKubeCluster := newKubeCluster(t, "prod", map[string]string{"env": "prod"})
	roleSet := NewRoleSet(
		newRole(func(rv *types.RoleV6) {
			rv.SetName("dev")
			rv.SetKubeResources(types.Allow, []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Name:      "dev",
					Namespace: "dev",
					Verbs:     []string{types.Wildcard},
				},
			})
			rv.SetKubernetesLabels(types.Allow, kubeDevLabels)
			rv.SetKubeUsers(types.Allow, kubeUsers)
		}),
		newRole(func(rv *types.RoleV6) {
			rv.SetName("any")
			rv.SetKubeResources(types.Allow, []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.Wildcard},
				},
				{
					Kind:      types.KindKubePod,
					Name:      "any1",
					Namespace: "any2",
					Verbs:     []string{types.Wildcard},
				},
			})
			rv.SetKubernetesLabels(types.Allow, kubeAnyLabels)
			rv.SetKubeUsers(types.Allow, kubeUsers)
		}),
	)
	listOnlySet := NewRoleSet(
		newRole(func(rv *types.RoleV6) {
			rv.SetName("list-only")
			rv.SetKubeResources(types.Allow, []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.KubeVerbList},
				},
				{
					Kind:      types.KindKubePod,
					Name:      "any1",
					Namespace: "any2",
					Verbs:     []string{types.KubeVerbList},
				},
			})
			rv.SetKubernetesLabels(types.Allow, kubeAnyLabels)
			rv.SetKubeUsers(types.Allow, kubeUsers)
		}),
	)
	localCluster := "cluster"
	type fields struct {
		info     *AccessInfo
		roleSet  RoleSet
		resource types.KubernetesResource
	}
	tests := []struct {
		name         string
		fields       fields
		kubeCluster  types.KubeCluster
		wantAllowed  []types.KubernetesResource
		wantDenied   []types.KubernetesResource
		assertAccess require.ErrorAssertionFunc
	}{
		{
			name:        "prod cluster",
			kubeCluster: prodKubeCluster,
			fields: fields{
				info: &AccessInfo{
					Roles: []string{"any", "dev"},
				},
				roleSet: roleSet,
				resource: types.KubernetesResource{
					Kind:      types.KindKubePod,
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.KubeVerbGet},
				},
			},
			wantAllowed: []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.Wildcard},
				},
				{
					Kind:      types.KindKubePod,
					Name:      "any1",
					Namespace: "any2",
					Verbs:     []string{types.Wildcard},
				},
			},
			wantDenied:   emptySet,
			assertAccess: require.NoError,
		},
		{
			name:        "dev cluster",
			kubeCluster: devKubeCluster,
			fields: fields{
				info: &AccessInfo{
					Roles: []string{"any", "dev"},
				},
				roleSet: roleSet,
				resource: types.KubernetesResource{
					Kind:      types.KindKubePod,
					Name:      "any1",
					Namespace: "rand",
					Verbs:     []string{types.KubeVerbGet},
				},
			},
			wantAllowed: []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.Wildcard},
				},
				{
					Kind:      types.KindKubePod,
					Name:      "any1",
					Namespace: "any2",
					Verbs:     []string{types.Wildcard},
				},
				{
					Kind:      types.KindKubePod,
					Name:      "dev",
					Namespace: "dev",
					Verbs:     []string{types.Wildcard},
				},
			},
			wantDenied:   emptySet,
			assertAccess: require.Error,
		},
		{
			name:        "dev cluster with resource access request",
			kubeCluster: devKubeCluster,
			fields: fields{
				roleSet: roleSet,
				info: &AccessInfo{
					Roles: []string{"any", "dev"},
					AllowedResourceIDs: []types.ResourceID{
						{
							Kind:        types.KindApp,
							ClusterName: localCluster,
							Name:        "devapp",
						},
						{
							Kind:            types.KindKubePod,
							ClusterName:     localCluster,
							Name:            devKubeCluster.GetName(),
							SubResourceName: "dev/dev",
						},
						{
							Kind:            types.KindKubePod,
							ClusterName:     localCluster,
							Name:            devKubeCluster.GetName(),
							SubResourceName: "test/test-3",
						},
						{
							Kind:            types.KindKubePod,
							ClusterName:     localCluster,
							Name:            prodKubeCluster.GetName(),
							SubResourceName: "prod/test-2",
						},
					},
				},
				resource: types.KubernetesResource{
					Kind:      types.KindKubePod,
					Name:      "dev",
					Namespace: "dev",
					Verbs:     []string{types.KubeVerbGet},
				},
			},
			wantAllowed: []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Name:      "dev",
					Namespace: "dev",
					Verbs:     []string{types.Wildcard},
				},
			},
			wantDenied:   emptySet,
			assertAccess: require.NoError,
		},
		{
			name:        "prod cluster with resource access request",
			kubeCluster: prodKubeCluster,
			fields: fields{
				info: &AccessInfo{
					Roles: []string{"any", "dev"},
					AllowedResourceIDs: []types.ResourceID{
						{
							Kind:        types.KindApp,
							ClusterName: localCluster,
							Name:        "devapp",
						},
						{
							Kind:            types.KindKubePod,
							ClusterName:     localCluster,
							Name:            devKubeCluster.GetName(),
							SubResourceName: "test/test-2",
						},
						{
							Kind:            types.KindKubePod,
							ClusterName:     localCluster,
							Name:            devKubeCluster.GetName(),
							SubResourceName: "test/test-3",
						},
						{
							Kind:            types.KindKubePod,
							ClusterName:     localCluster,
							Name:            prodKubeCluster.GetName(),
							SubResourceName: "prod/test-2",
						},
					},
				},
				resource: types.KubernetesResource{
					Kind:      types.KindKubePod,
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.KubeVerbGet},
				},
			},
			wantAllowed:  nil,
			wantDenied:   emptySet,
			assertAccess: require.Error,
		},
		{
			name:        "dev cluster with kube_cluster resource access request",
			kubeCluster: devKubeCluster,
			fields: fields{
				roleSet: roleSet,
				info: &AccessInfo{
					Roles: []string{"any", "dev"},
					AllowedResourceIDs: []types.ResourceID{
						{
							Kind:        types.KindApp,
							ClusterName: localCluster,
							Name:        "devapp",
						},
						{
							Kind:        types.KindKubernetesCluster,
							ClusterName: localCluster,
							Name:        devKubeCluster.GetName(),
						},
					},
				},
				resource: types.KubernetesResource{
					Kind:      types.KindKubePod,
					Name:      "dev",
					Namespace: "dev",
					Verbs:     []string{types.KubeVerbGet},
				},
			},
			wantAllowed: []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.Wildcard},
				},
				{
					Kind:      types.KindKubePod,
					Name:      "any1",
					Namespace: "any2",
					Verbs:     []string{types.Wildcard},
				},
				{
					Kind:      types.KindKubePod,
					Name:      "dev",
					Namespace: "dev",
					Verbs:     []string{types.Wildcard},
				},
			},
			wantDenied:   emptySet,
			assertAccess: require.NoError,
		},
		{
			name:        "access dev cluster with kube cluster<prodCluster> and kube pod<devCluster> resource access request",
			kubeCluster: devKubeCluster,
			fields: fields{
				roleSet: roleSet,
				info: &AccessInfo{
					Roles: []string{"any"},
					AllowedResourceIDs: []types.ResourceID{
						{
							Kind:        types.KindKubernetesCluster,
							ClusterName: localCluster,
							Name:        prodKubeCluster.GetName(),
						},
						{
							Kind:            types.KindKubePod,
							ClusterName:     localCluster,
							Name:            devKubeCluster.GetName(),
							SubResourceName: "dev/dev",
						},
					},
				},
				resource: types.KubernetesResource{
					Kind:      types.KindKubePod,
					Name:      "dev",
					Namespace: "dev",
					Verbs:     []string{types.KubeVerbGet},
				},
			},
			wantAllowed: []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Name:      "dev",
					Namespace: "dev",
					Verbs:     []string{types.Wildcard},
				},
			},
			wantDenied:   emptySet,
			assertAccess: require.NoError,
		},
		{
			name:        "access prod cluster with kube cluster<prodCluster> and kube pod<devCluster> resource access request",
			kubeCluster: prodKubeCluster,
			fields: fields{
				roleSet: roleSet,
				info: &AccessInfo{
					Roles: []string{"any"},
					AllowedResourceIDs: []types.ResourceID{
						{
							Kind:        types.KindKubernetesCluster,
							ClusterName: localCluster,
							Name:        prodKubeCluster.GetName(),
						},
						{
							Kind:            types.KindKubePod,
							ClusterName:     localCluster,
							Name:            devKubeCluster.GetName(),
							SubResourceName: "dev/dev",
						},
					},
				},
				resource: types.KubernetesResource{
					Kind:      types.KindKubePod,
					Name:      "dev",
					Namespace: "dev",
					Verbs:     []string{types.KubeVerbGet},
				},
			},
			wantAllowed: []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.Wildcard},
				},
				{
					Kind:      types.KindKubePod,
					Name:      "any1",
					Namespace: "any2",
					Verbs:     []string{types.Wildcard},
				},
			},
			wantDenied:   emptySet,
			assertAccess: require.Error,
		},
		{
			name:        "access pod outside namespace allowed by roles",
			kubeCluster: prodKubeCluster,
			fields: fields{
				roleSet: roleSet,
				info: &AccessInfo{
					Roles: []string{"any", "dev"},
					AllowedResourceIDs: []types.ResourceID{
						{
							Kind:            types.KindKubePod,
							ClusterName:     localCluster,
							Name:            prodKubeCluster.GetName(),
							SubResourceName: "wrongNamespace/wrongPodName",
						},
					},
				},
				resource: types.KubernetesResource{
					Kind:      types.KindKubePod,
					Name:      "wrongPodName",
					Namespace: "wrongNamespace",
					Verbs:     []string{types.KubeVerbGet},
				},
			},
			wantAllowed:  nil,
			wantDenied:   emptySet,
			assertAccess: require.Error,
		},
		{
			name:        "prod cluster with list verb but tries to access get",
			kubeCluster: prodKubeCluster,
			fields: fields{
				info: &AccessInfo{
					Roles: []string{"list-only"},
				},
				roleSet: listOnlySet,
				resource: types.KubernetesResource{
					Kind:      types.KindKubePod,
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.KubeVerbGet},
				},
			},
			wantAllowed: []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.KubeVerbList},
				},
				{
					Kind:      types.KindKubePod,
					Name:      "any1",
					Namespace: "any2",
					Verbs:     []string{types.KubeVerbList},
				},
			},
			wantDenied:   emptySet,
			assertAccess: require.Error,
		},
		{
			name:        "prod cluster with list verb",
			kubeCluster: prodKubeCluster,
			fields: fields{
				info: &AccessInfo{
					Roles: []string{"list-only"},
				},
				roleSet: listOnlySet,
				resource: types.KubernetesResource{
					Kind:      types.KindKubePod,
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.KubeVerbList},
				},
			},
			wantAllowed: []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.KubeVerbList},
				},
				{
					Kind:      types.KindKubePod,
					Name:      "any1",
					Namespace: "any2",
					Verbs:     []string{types.KubeVerbList},
				},
			},
			wantDenied:   emptySet,
			assertAccess: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessChecker := NewAccessCheckerWithRoleSet(tt.fields.info, localCluster, tt.fields.roleSet)
			gotAllowed, gotDenied := accessChecker.GetKubeResources(tt.kubeCluster)

			err := accessChecker.CheckAccess(
				tt.kubeCluster,
				AccessState{MFARequired: MFARequiredNever},
				// Append a matcher that validates if the Kubernetes resource is allowed
				// by the roles that satisfy the Kubernetes Cluster.
				NewKubernetesResourceMatcher(tt.fields.resource),
			)
			tt.assertAccess(t, err)
			sortKubeResourceSlice(gotAllowed)
			sortKubeResourceSlice(gotDenied)
			require.EqualValues(t, tt.wantAllowed, gotAllowed)
			require.EqualValues(t, tt.wantDenied, gotDenied)
		})
	}
}

func newKubeCluster(t *testing.T, name string, labels map[string]string) types.KubeCluster {
	cluster, err := types.NewKubernetesClusterV3(types.Metadata{
		Name:   name,
		Labels: labels,
	}, types.KubernetesClusterSpecV3{},
	)
	require.NoError(t, err)
	return cluster
}

func sortKubeResourceSlice(resources []types.KubernetesResource) {
	sort.Slice(resources, func(i, j int) bool { return resources[i].Name < resources[j].Name })
}
