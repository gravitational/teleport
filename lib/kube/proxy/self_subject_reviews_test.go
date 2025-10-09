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

package proxy

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
	testingkubemock "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
)

func TestSelfSubjectAccessReviewsRBAC(t *testing.T) {
	t.Parallel()
	// kubeMock is a Kubernetes API mock for the session tests.
	// Once a new session is created, this mock will write to
	// stdout and stdin (if available) the pod name, followed
	// by copying the contents of stdin into both streams.
	kubeMock, err := testingkubemock.NewKubeAPIMock()
	require.NoError(t, err)
	t.Cleanup(func() { kubeMock.Close() })

	// creates a Kubernetes service with a configured cluster pointing to mock api server
	testCtx := SetupTestContext(
		context.Background(),
		t,
		TestConfig{
			Clusters: []KubeClusterConfig{{Name: kubeCluster, APIEndpoint: kubeMock.URL}},
		},
	)
	// close tests
	t.Cleanup(func() { require.NoError(t, testCtx.Close()) })

	type args struct {
		name      string
		namespace string
		kind      string
		apiGroup  string
		resources []types.KubernetesResource
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "user with full access to kubernetes resources",
			args: args{
				name:      "",
				namespace: "",
				kind:      "pods",
				resources: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: types.Wildcard,
						Name:      types.Wildcard,
						Verbs:     []string{types.Wildcard},
					},
				},
			},
			want: true,
		},
		{
			name: "user with full access to kubernetes resources to namespace=namespace-1, pod=pod-1",
			args: args{
				name:      "pod-1",
				namespace: "namespace-1",
				kind:      "pods",
				resources: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: types.Wildcard,
						Name:      types.Wildcard,
						Verbs:     []string{types.Wildcard},
					},
				},
			},
			want: true,
		},
		{
			name: "user with full access to kubernetes resources to pod=pod-1",
			args: args{
				name:      "pod-1",
				namespace: "",
				kind:      "pods",
				resources: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: types.Wildcard,
						Name:      types.Wildcard,
						Verbs:     []string{types.Wildcard},
					},
				},
			},
			want: true,
		},
		{
			name: "user with no access to kubernetes resources to namespace=namespace-1, pod=pod-1",
			args: args{
				name:      "pod-1",
				namespace: "namespace-1",
				kind:      "pods",
				resources: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Name:      "pod-2",
						Namespace: "namespace-1",
						Verbs:     []string{types.Wildcard},
					},
				},
			},
			want: false,
		},
		{
			name: "user with access to kubernetes resources to namespace=namespace-1, pod=pod-1",
			args: args{
				name:      "pod-1",
				namespace: "namespace-1",
				kind:      "pods",
				resources: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Name:      "pod-2",
						Namespace: "namespace-1",
						Verbs:     []string{types.Wildcard},
					},
					{
						Kind:      types.KindKubePod,
						Name:      "pod-1",
						Namespace: "namespace-1",
						Verbs:     []string{types.Wildcard},
					},
				},
			},
			want: true,
		},
		{
			name: "user with access to kubernetes resources to namespace=namespace-1, pod=pod-2",
			args: args{
				name:      "",
				namespace: "namespace-1",
				kind:      "pods",
				resources: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Name:      "pod-2",
						Namespace: "namespace-1",
						Verbs:     []string{types.Wildcard},
					},
				},
			},
			want: true,
		},
		{
			name: "user without access to kubernetes resources to namespace=namespace-2",
			args: args{
				name:      "",
				namespace: "namespace-2",
				kind:      "pods",
				resources: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Name:      "pod-2",
						Namespace: "namespace-1",
						Verbs:     []string{types.Wildcard},
					},
				},
			},
			want: false,
		},
		{
			name: "user with namespace access to namespace=namespace-2",
			args: args{
				name: "namespace-2",
				kind: "namespaces",
				resources: []types.KubernetesResource{
					{
						Kind:  types.KindKubeNamespace,
						Name:  "namespace-2",
						Verbs: []string{types.Wildcard},
					},
				},
			},
			want: true,
		},
		{
			name: "user without namespace access to namespace=namespace-2",
			args: args{
				name: "namespace-2",
				kind: "namespaces",
				resources: []types.KubernetesResource{
					{
						Kind:  types.KindKubeNamespace,
						Name:  "namespace",
						Verbs: []string{types.Wildcard},
					},
				},
			},
			want: false,
		},
		{
			name: "user with namespace access to pods in namespace=namespace-2",
			args: args{
				namespace: "namespace-2",
				kind:      "pods",
				resources: []types.KubernetesResource{
					{
						Kind:  types.KindKubeNamespace,
						Name:  "namespace-2",
						Verbs: []string{types.Wildcard},
					},
				},
			},
			want: true,
		},
		{
			name: "user with namespace access to custom resource in namespace=namespace-2",
			args: args{
				namespace: "namespace-2",
				kind:      "teleportroles",
				apiGroup:  "resources.teleport.dev",
				resources: []types.KubernetesResource{
					{
						Kind:  types.KindKubeNamespace,
						Name:  "namespace-2",
						Verbs: []string{types.Wildcard},
					},
				},
			},
			want: true,
		},
		{
			name: "user without namespace access to custom resource in namespace=namespace",
			args: args{
				namespace: "namespace",
				kind:      "teleportroles",
				apiGroup:  "resources.teleport.dev",
				resources: []types.KubernetesResource{
					{
						Kind:  types.KindKubeNamespace,
						Name:  "namespace-2",
						Verbs: []string{types.Wildcard},
					},
				},
			},
			want: false,
		},
		{
			name: "user without clusterrole access",
			args: args{
				name:     "role",
				kind:     "clusterroles",
				apiGroup: "rbac.authorization.k8s.io",
				resources: []types.KubernetesResource{
					{
						Kind:  types.KindKubeNamespace,
						Name:  "namespace-2",
						Verbs: []string{types.Wildcard},
					},
				},
			},
			want: false,
		},
		{
			name: "user with clusterrole access",
			args: args{
				name:     "role",
				kind:     "clusterroles",
				apiGroup: "rbac.authorization.k8s.io",
				resources: []types.KubernetesResource{
					{
						Kind:  types.KindKubeClusterRole,
						Name:  "role",
						Verbs: []string{types.Wildcard},
					},
				},
			},
			want: true,
		},
		{
			name: "user without access to clusterrole role2",
			args: args{
				name:     "role2",
				kind:     "clusterroles",
				apiGroup: "rbac.authorization.k8s.io",
				resources: []types.KubernetesResource{
					{
						Kind:  types.KindKubeClusterRole,
						Name:  "role",
						Verbs: []string{types.Wildcard},
					},
				},
			},
			want: false,
		},
		{
			name: "user check clusterrole access with empty role name",
			args: args{
				name:     "",
				kind:     "clusterroles",
				apiGroup: "rbac.authorization.k8s.io",
				resources: []types.KubernetesResource{
					{
						Kind:  types.KindKubeClusterRole,
						Name:  "role",
						Verbs: []string{types.Wildcard},
					},
				},
			},
			want: true,
		},
		{
			name: "user misses the role",
			args: args{
				name:     "",
				kind:     "clusterroles",
				apiGroup: "rbac.authorization.k8s.io",
				resources: []types.KubernetesResource{
					{
						Kind:  types.KindKubeClusterRole,
						Name:  "role",
						Verbs: []string{"get"},
					},
				},
			},
			want: false,
		},
		{
			name: "user tries to check a specific pod when he holds a namespace resource",
			args: args{
				name:      "pod-1",
				kind:      "pods",
				namespace: "namespace-1",
				resources: []types.KubernetesResource{
					{
						Kind:  types.KindKubeNamespace,
						Name:  "namespace-1",
						Verbs: []string{types.Wildcard},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// create a user with full access to kubernetes Pods.
			// (kubernetes_user and kubernetes_groups specified)
			userID := uuid.New().String()
			user, _ := testCtx.CreateUserAndRole(
				testCtx.Context,
				t,
				userID,
				RoleSpec{
					Name:       userID,
					KubeUsers:  roleKubeUsers,
					KubeGroups: roleKubeGroups,

					SetupRoleFunc: func(r types.Role) {
						r.SetKubeResources(types.Allow, tt.args.resources)
					},
				},
			)
			// generate a kube client with user certs for auth
			client, _ := testCtx.GenTestKubeClientTLSCert(
				t,
				user.GetName(),
				kubeCluster,
			)

			rsp, err := client.AuthorizationV1().SelfSubjectAccessReviews().Create(
				context.TODO(),
				&authv1.SelfSubjectAccessReview{
					Spec: authv1.SelfSubjectAccessReviewSpec{
						ResourceAttributes: &authv1.ResourceAttributes{
							Resource:  tt.args.kind,
							Group:     tt.args.apiGroup,
							Name:      tt.args.name,
							Namespace: tt.args.namespace,
							Verb:      "list",
						},
					},
				},
				metav1.CreateOptions{},
			)
			require.NoError(t, err)
			require.Equal(t, tt.want, rsp.Status.Allowed)
		})
	}
}
