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

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
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
					Kind:      "pods",
					Name:      "dev",
					Namespace: "dev",
					Verbs:     []string{types.Wildcard},
					APIGroup:  types.Wildcard,
				},
			})
			rv.SetKubernetesLabels(types.Allow, kubeDevLabels)
			rv.SetKubeUsers(types.Allow, kubeUsers)
		}),
		newRole(func(rv *types.RoleV6) {
			rv.SetName("any")
			rv.SetKubeResources(types.Allow, []types.KubernetesResource{
				{
					Kind:      "pods",
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.Wildcard},
					APIGroup:  types.Wildcard,
				},
				{
					Kind:      "pods",
					Name:      "any1",
					Namespace: "any2",
					Verbs:     []string{types.Wildcard},
					APIGroup:  types.Wildcard,
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
					Kind:      "pods",
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.KubeVerbList},
					APIGroup:  types.Wildcard,
				},
				{
					Kind:      "pods",
					Name:      "any1",
					Namespace: "any2",
					Verbs:     []string{types.KubeVerbList},
					APIGroup:  types.Wildcard,
				},
			})
			rv.SetKubernetesLabels(types.Allow, kubeAnyLabels)
			rv.SetKubeUsers(types.Allow, kubeUsers)
		}),
	)
	localCluster := "cluster"
	type fields struct {
		info          *AccessInfo
		roleSet       RoleSet
		resource      types.KubernetesResource
		isClusterWide bool
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
					Kind:      "pods",
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.KubeVerbGet},
				},
			},
			wantAllowed: []types.KubernetesResource{
				{
					Kind:      "pods",
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.Wildcard},
					APIGroup:  types.Wildcard,
				},
				{
					Kind:      "pods",
					Name:      "any1",
					Namespace: "any2",
					Verbs:     []string{types.Wildcard},
					APIGroup:  types.Wildcard,
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
					Kind:      "pods",
					Name:      "any1",
					Namespace: "rand",
					Verbs:     []string{types.KubeVerbGet},
				},
			},
			wantAllowed: []types.KubernetesResource{
				{
					Kind:      "pods",
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.Wildcard},
					APIGroup:  types.Wildcard,
				},
				{
					Kind:      "pods",
					Name:      "any1",
					Namespace: "any2",
					Verbs:     []string{types.Wildcard},
					APIGroup:  types.Wildcard,
				},
				{
					Kind:      "pods",
					Name:      "dev",
					Namespace: "dev",
					Verbs:     []string{types.Wildcard},
					APIGroup:  types.Wildcard,
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
					Kind:      "pods",
					Name:      "dev",
					Namespace: "dev",
					Verbs:     []string{types.KubeVerbGet},
					APIGroup:  "*",
				},
			},
			wantAllowed: []types.KubernetesResource{
				{
					Kind:      "pods",
					Name:      "dev",
					Namespace: "dev",
					Verbs:     []string{types.Wildcard},
					APIGroup:  "*",
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
					Kind:      "pods",
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.KubeVerbGet},
					APIGroup:  "",
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
					Kind:      "pods",
					Name:      "dev",
					Namespace: "dev",
					Verbs:     []string{types.KubeVerbGet},
					APIGroup:  types.Wildcard,
				},
			},
			wantAllowed: []types.KubernetesResource{
				{
					Kind:      "pods",
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.Wildcard},
					APIGroup:  types.Wildcard,
				},
				{
					Kind:      "pods",
					Name:      "any1",
					Namespace: "any2",
					Verbs:     []string{types.Wildcard},
					APIGroup:  types.Wildcard,
				},
				{
					Kind:      "pods",
					Name:      "dev",
					Namespace: "dev",
					Verbs:     []string{types.Wildcard},
					APIGroup:  types.Wildcard,
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
					Kind:      "pods",
					Name:      "dev",
					Namespace: "dev",
					Verbs:     []string{types.KubeVerbGet},
					APIGroup:  "",
				},
			},
			wantAllowed: []types.KubernetesResource{
				{
					Kind:      "pods",
					Name:      "dev",
					Namespace: "dev",
					Verbs:     []string{types.Wildcard},
					APIGroup:  types.Wildcard,
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
					Kind:      "pods",
					Name:      "dev",
					Namespace: "dev",
					Verbs:     []string{types.KubeVerbGet},
					APIGroup:  "",
				},
			},
			wantAllowed: []types.KubernetesResource{
				{
					Kind:      "pods",
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.Wildcard},
					APIGroup:  types.Wildcard,
				},
				{
					Kind:      "pods",
					Name:      "any1",
					Namespace: "any2",
					Verbs:     []string{types.Wildcard},
					APIGroup:  types.Wildcard,
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
							Kind:            "pods",
							ClusterName:     localCluster,
							Name:            prodKubeCluster.GetName(),
							SubResourceName: "wrongNamespace/wrongPodName",
						},
					},
				},
				resource: types.KubernetesResource{
					Kind:      "pods",
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
					Kind:      "pods",
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.KubeVerbGet},
				},
			},
			wantAllowed: []types.KubernetesResource{
				{
					Kind:      "pods",
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.KubeVerbList},
					APIGroup:  types.Wildcard,
				},
				{
					Kind:      "pods",
					Name:      "any1",
					Namespace: "any2",
					Verbs:     []string{types.KubeVerbList},
					APIGroup:  types.Wildcard,
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
					Kind:      "pods",
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.KubeVerbList},
				},
			},
			wantAllowed: []types.KubernetesResource{
				{
					Kind:      "pods",
					Name:      "any1",
					Namespace: "any1",
					Verbs:     []string{types.KubeVerbList},
					APIGroup:  types.Wildcard,
				},
				{
					Kind:      "pods",
					Name:      "any1",
					Namespace: "any2",
					Verbs:     []string{types.KubeVerbList},
					APIGroup:  types.Wildcard,
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
				NewKubernetesResourceMatcher(tt.fields.resource, tt.fields.isClusterWide),
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

func TestAccessCheckerHostUsersShell(t *testing.T) {
	anyLabels := types.Labels{"*": {"*"}}
	expectedShell := "bash"
	secondaryShell := "zsh"
	localCluster := "cluster"

	roleSet := NewRoleSet(
		newRole(func(rv *types.RoleV6) {
			rv.SetName("any")
			rv.SetOptions(types.RoleOptions{
				CreateHostUserDefaultShell: expectedShell,
				CreateHostUserMode:         types.CreateHostUserMode_HOST_USER_MODE_KEEP,
			})
			rv.SetNodeLabels(types.Allow, anyLabels)
		}),
		newRole(func(rv *types.RoleV6) {
			rv.SetName("any")
			rv.SetOptions(types.RoleOptions{
				CreateHostUserDefaultShell: secondaryShell,
				CreateHostUserMode:         types.CreateHostUserMode_HOST_USER_MODE_KEEP,
			})
			rv.SetNodeLabels(types.Allow, anyLabels)
		}),
	)

	accessInfo := &AccessInfo{
		Roles: []string{"default-shell"},
	}

	accessChecker := NewAccessCheckerWithRoleSet(accessInfo, localCluster, roleSet)
	hui, err := accessChecker.HostUsers(serverStub{})
	require.NoError(t, err)

	// the first value for shell encountered while checking roles should be used, which means
	// secondaryShell should never be the result here
	require.Equal(t, expectedShell, hui.Shell)
}

func TestSSHPortForwarding(t *testing.T) {
	anyLabels := types.Labels{"*": {"*"}}
	localCluster := "cluster"

	allAllow := newRole(func(rv *types.RoleV6) {
		rv.SetName("all-allow")
		rv.SetOptions(types.RoleOptions{
			PortForwarding: types.NewBoolOption(true),
			SSHPortForwarding: &types.SSHPortForwarding{
				Remote: &types.SSHRemotePortForwarding{Enabled: types.NewBoolOption(true)},
				Local:  &types.SSHLocalPortForwarding{Enabled: types.NewBoolOption(true)},
			},
		})
		rv.SetNodeLabels(types.Allow, anyLabels)
	})

	allDeny := newRole(func(rv *types.RoleV6) {
		rv.SetName("all-deny")
		rv.SetOptions(types.RoleOptions{
			SSHPortForwarding: &types.SSHPortForwarding{
				Remote: &types.SSHRemotePortForwarding{Enabled: types.NewBoolOption(false)},
				Local:  &types.SSHLocalPortForwarding{Enabled: types.NewBoolOption(false)},
			},
		})
		rv.SetNodeLabels(types.Allow, anyLabels)
	})

	allow := newRole(func(rv *types.RoleV6) {
		rv.SetName("allow")
		rv.SetOptions(types.RoleOptions{
			SSHPortForwarding: &types.SSHPortForwarding{
				Remote: &types.SSHRemotePortForwarding{Enabled: types.NewBoolOption(true)},
				Local:  &types.SSHLocalPortForwarding{Enabled: types.NewBoolOption(true)},
			},
		})
		rv.SetNodeLabels(types.Allow, anyLabels)
	})

	deny := newRole(func(rv *types.RoleV6) {
		rv.SetName("deny")
		rv.SetOptions(types.RoleOptions{
			SSHPortForwarding: &types.SSHPortForwarding{
				Remote: &types.SSHRemotePortForwarding{Enabled: types.NewBoolOption(false)},
				Local:  &types.SSHLocalPortForwarding{Enabled: types.NewBoolOption(false)},
			},
		})
		rv.SetNodeLabels(types.Allow, anyLabels)
	})

	legacyAllow := newRole(func(rv *types.RoleV6) {
		rv.SetName("legacy-allow")
		rv.SetOptions(types.RoleOptions{
			PortForwarding: types.NewBoolOption(true),
		})
		rv.SetNodeLabels(types.Allow, anyLabels)
	})

	legacyDeny := newRole(func(rv *types.RoleV6) {
		rv.SetName("legacy-deny")
		rv.SetOptions(types.RoleOptions{
			PortForwarding: types.NewBoolOption(false),
		})
		rv.SetNodeLabels(types.Allow, anyLabels)
	})

	remoteAllow := newRole(func(rv *types.RoleV6) {
		rv.SetName("remote-allow")
		rv.SetOptions(types.RoleOptions{
			SSHPortForwarding: &types.SSHPortForwarding{
				Remote: &types.SSHRemotePortForwarding{Enabled: types.NewBoolOption(true)},
			},
		})
		rv.SetNodeLabels(types.Allow, anyLabels)
	})

	remoteDeny := newRole(func(rv *types.RoleV6) {
		rv.SetName("remote-deny")
		rv.SetOptions(types.RoleOptions{
			SSHPortForwarding: &types.SSHPortForwarding{
				Remote: &types.SSHRemotePortForwarding{Enabled: types.NewBoolOption(false)},
			},
		})
		rv.SetNodeLabels(types.Allow, anyLabels)
	})

	localAllow := newRole(func(rv *types.RoleV6) {
		rv.SetName("local-allow")
		rv.SetOptions(types.RoleOptions{
			SSHPortForwarding: &types.SSHPortForwarding{
				Local: &types.SSHLocalPortForwarding{Enabled: types.NewBoolOption(true)},
			},
		})
		rv.SetNodeLabels(types.Allow, anyLabels)
	})

	localDeny := newRole(func(rv *types.RoleV6) {
		rv.SetName("local-deny")
		rv.SetOptions(types.RoleOptions{
			SSHPortForwarding: &types.SSHPortForwarding{
				Local: &types.SSHLocalPortForwarding{Enabled: types.NewBoolOption(false)},
			},
		})
		rv.SetNodeLabels(types.Allow, anyLabels)
	})

	implicitAllow := newRole(func(rv *types.RoleV6) {
		rv.SetName("implicit-allow")
		rv.SetNodeLabels(types.Allow, anyLabels)
	})

	testCases := []struct {
		name         string
		roleSet      RoleSet
		expectedMode decisionpb.SSHPortForwardMode
	}{
		{
			name:         "allow all",
			roleSet:      NewRoleSet(allAllow),
			expectedMode: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_ON,
		},
		{
			name:         "deny all",
			roleSet:      NewRoleSet(allDeny),
			expectedMode: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_OFF,
		},
		{
			name:         "allow remote and local",
			roleSet:      NewRoleSet(allow),
			expectedMode: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_ON,
		},
		{
			name:         "deny remote and local",
			roleSet:      NewRoleSet(deny),
			expectedMode: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_OFF,
		},
		{
			name:         "legacy allow",
			roleSet:      NewRoleSet(legacyAllow),
			expectedMode: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_ON,
		},
		{
			name:         "legacy deny",
			roleSet:      NewRoleSet(legacyDeny),
			expectedMode: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_OFF,
		},
		{
			name:         "remote allow",
			roleSet:      NewRoleSet(remoteAllow),
			expectedMode: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_ON,
		},
		{
			name:         "remote deny",
			roleSet:      NewRoleSet(remoteDeny),
			expectedMode: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_LOCAL,
		},
		{
			name:         "local allow",
			roleSet:      NewRoleSet(localAllow),
			expectedMode: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_ON,
		},
		{
			name:         "local deny",
			roleSet:      NewRoleSet(localDeny),
			expectedMode: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_REMOTE,
		},
		{
			name:         "implicit allow",
			roleSet:      NewRoleSet(implicitAllow),
			expectedMode: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_ON,
		},
		{
			name:         "conflicting roles: allow all with remote deny",
			roleSet:      NewRoleSet(allow, remoteDeny),
			expectedMode: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_LOCAL,
		},
		{
			name:         "conflicting roles: allow all with local deny",
			roleSet:      NewRoleSet(allow, localDeny),
			expectedMode: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_REMOTE,
		},
		{
			// legacy behavior prefers explicit allow, so make sure we respect that if one is given
			name:         "conflicting roles: deny all with legacy allow",
			roleSet:      NewRoleSet(deny, legacyAllow),
			expectedMode: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_ON,
		},
		{
			// legacy behavior prioritizes explicit allow, so make sure we respect that if another role would allow access
			name:         "conflicting roles: allow all with legacy deny",
			roleSet:      NewRoleSet(allow, legacyDeny),
			expectedMode: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_ON,
		},
		{
			name:         "conflicting roles implicit allow explicit deny",
			roleSet:      NewRoleSet(implicitAllow, deny),
			expectedMode: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_OFF,
		},
	}

	for _, c := range testCases {
		t.Run(c.name, func(t *testing.T) {
			accessChecker := NewAccessCheckerWithRoleSet(&AccessInfo{}, localCluster, c.roleSet)
			require.Equal(t, c.expectedMode, accessChecker.SSHPortForwardMode())
		})
	}
}

type serverStub struct {
	types.Server
}

func (serverStub) GetKind() string {
	return types.KindNode
}

func TestAccessCheckerWorkloadIdentity(t *testing.T) {
	localCluster := "cluster"

	noLabelsWI := &workloadidentityv1pb.WorkloadIdentity{
		Kind: types.KindWorkloadIdentity,
		Metadata: &headerv1.Metadata{
			Name: "no-labels",
		},
	}
	fooLabeledWI := &workloadidentityv1pb.WorkloadIdentity{
		Kind: types.KindWorkloadIdentity,
		Metadata: &headerv1.Metadata{
			Name: "foo-labeled",
			Labels: map[string]string{
				"foo": "bar",
			},
		},
	}

	roleNoLabels := newRole(func(rv *types.RoleV6) {})
	roleWildcard := newRole(func(rv *types.RoleV6) {
		rv.Spec.Allow.WorkloadIdentityLabels = types.Labels{types.Wildcard: []string{types.Wildcard}}
	})
	roleFooLabel := newRole(func(rv *types.RoleV6) {
		rv.Spec.Allow.WorkloadIdentityLabels = types.Labels{"foo": {"bar"}}
	})
	tests := []struct {
		name         string
		roleSet      RoleSet
		resource     *workloadidentityv1pb.WorkloadIdentity
		requireError require.ErrorAssertionFunc
	}{
		{
			name: "wildcard role, no labels wi",
			roleSet: NewRoleSet(
				roleWildcard,
			),
			resource:     noLabelsWI,
			requireError: require.NoError,
		},
		{
			name: "no labels role, no labels wi",
			roleSet: NewRoleSet(
				roleNoLabels,
			),
			resource:     noLabelsWI,
			requireError: require.Error,
		},
		{
			name: "labels role, no labels wi",
			roleSet: NewRoleSet(
				roleFooLabel,
			),
			resource:     noLabelsWI,
			requireError: require.Error,
		},
		{
			name: "wildcard role, labels wi",
			roleSet: NewRoleSet(
				roleWildcard,
			),
			resource:     fooLabeledWI,
			requireError: require.NoError,
		},
		{
			name: "no labels role, labels wi",
			roleSet: NewRoleSet(
				roleNoLabels,
			),
			resource:     fooLabeledWI,
			requireError: require.Error,
		},
		{
			name: "labels role, labels wi",
			roleSet: NewRoleSet(
				roleFooLabel,
			),
			resource:     fooLabeledWI,
			requireError: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessChecker := NewAccessCheckerWithRoleSet(&AccessInfo{}, localCluster, tt.roleSet)
			err := accessChecker.CheckAccess(
				types.Resource153ToResourceWithLabels(tt.resource),
				AccessState{},
			)
			tt.requireError(t, err)
		})
	}
}

func TestIdentityCenterAccountAccessRequestMatcher(t *testing.T) {
	const localCluster = "cluster"

	tests := []struct {
		info         *AccessInfo
		name         string
		resource     types.AppServerV3
		assertAccess require.ErrorAssertionFunc
	}{
		{
			name: "matches kind and subkind",
			info: &AccessInfo{
				AllowedResourceIDs: []types.ResourceID{
					{
						Kind:        types.KindIdentityCenterAccount,
						ClusterName: localCluster,
						Name:        "aws-dev",
					},
				},
			},
			resource: types.AppServerV3{
				Kind:    types.KindApp,
				SubKind: types.KindIdentityCenterAccount,
				Metadata: types.Metadata{
					Name: "aws-dev",
				},
			},
			assertAccess: require.NoError,
		},
		{
			name: "unmatched subkind",
			info: &AccessInfo{
				AllowedResourceIDs: []types.ResourceID{
					{
						Kind:        types.KindIdentityCenterAccount,
						ClusterName: localCluster,
						Name:        "aws-dev",
					},
				},
			},
			resource: types.AppServerV3{
				Kind: types.KindApp,
				Metadata: types.Metadata{
					Name: "aws-dev",
				},
			},
			assertAccess: func(t require.TestingT, err error, _ ...interface{}) {
				require.ErrorContains(t, err, "not in allowed resource IDs")
			},
		},
		{
			name: "unmatched kind",
			info: &AccessInfo{
				AllowedResourceIDs: []types.ResourceID{
					{
						Kind:        types.KindIdentityCenterAccount,
						ClusterName: localCluster,
						Name:        "aws-dev",
					},
				},
			},
			resource: types.AppServerV3{
				Kind:    types.KindAppSession,
				SubKind: types.KindIdentityCenterAccount,
				Metadata: types.Metadata{
					Name: "aws-dev",
				},
			},
			assertAccess: func(t require.TestingT, err error, _ ...interface{}) {
				require.ErrorContains(t, err, "not in allowed resource IDs")
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			accessChecker := NewAccessCheckerWithRoleSet(tc.info, localCluster, NewRoleSet(newRole(func(rv *types.RoleV6) {})))
			tc.assertAccess(t, accessChecker.CheckAccess(
				&tc.resource,
				AccessState{MFARequired: MFARequiredNever},
			))
		})
	}
}
