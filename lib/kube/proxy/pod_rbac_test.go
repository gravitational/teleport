/*
Copyright 2022 Gravitational, Inc.

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

package proxy

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
	testingkubemock "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
)

func TestListPodRBAC(t *testing.T) {
	t.Parallel()
	const (
		usernameWithFullAccess      = "full_user"
		usernameWithNamespaceAccess = "default_user"
		usernameWithLimitedAccess   = "limited_user"
		testPodName                 = "test"
	)
	// kubeMock is a Kubernetes API mock for the session tests.
	// Once a new session is created, this mock will write to
	// stdout and stdin (if available) the pod name, followed
	// by copying the contents of stdin into both streams.
	kubeMock, err := testingkubemock.NewKubeAPIMock()
	require.NoError(t, err)
	t.Cleanup(func() { kubeMock.Close() })

	// creates a Kubernetes service with a configured cluster pointing to mock api server
	testCtx := setupTestContext(
		context.Background(),
		t,
		testConfig{
			clusters: []kubeClusterConfig{{name: kubeCluster, apiEndpoint: kubeMock.URL}},
		},
	)
	// close tests
	t.Cleanup(func() { require.NoError(t, testCtx.Close()) })

	// create a user with full access to kubernetes Pods.
	// (kubernetes_user and kubernetes_groups specified)
	userWithFullAccess, _ := testCtx.createUserAndRole(
		testCtx.ctx,
		t,
		usernameWithFullAccess,
		roleSpec{
			name:       usernameWithFullAccess,
			kubeUsers:  roleKubeUsers,
			kubeGroups: roleKubeGroups,

			setupRoleFunc: func(r types.Role) {
				r.SetKubeResources(types.Allow, []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Name:      types.Wildcard,
						Namespace: types.Wildcard,
					},
				})
			},
		},
	)
	// create a user with full access to kubernetes Pods.
	// (kubernetes_user and kubernetes_groups specified)
	userWithNamespaceAccess, _ := testCtx.createUserAndRole(
		testCtx.ctx,
		t,
		usernameWithNamespaceAccess,
		roleSpec{
			name:       usernameWithNamespaceAccess,
			kubeUsers:  roleKubeUsers,
			kubeGroups: roleKubeGroups,
			setupRoleFunc: func(r types.Role) {
				r.SetKubeResources(types.Allow,
					[]types.KubernetesResource{
						{
							Kind:      types.KindKubePod,
							Name:      types.Wildcard,
							Namespace: metav1.NamespaceDefault,
						},
					})
			},
		},
	)

	// create a moderator user with access to kubernetes
	// (kubernetes_user and kubernetes_groups specified)
	userWithLimitedAccess, _ := testCtx.createUserAndRole(
		testCtx.ctx,
		t,
		usernameWithLimitedAccess,
		roleSpec{
			name:       usernameWithLimitedAccess,
			kubeUsers:  roleKubeUsers,
			kubeGroups: roleKubeGroups,
			setupRoleFunc: func(r types.Role) {
				r.SetKubeResources(types.Allow,
					[]types.KubernetesResource{
						{
							Kind:      types.KindKubePod,
							Name:      "nginx-*",
							Namespace: metav1.NamespaceDefault,
						},
					},
				)
			},
		},
	)

	type args struct {
		user      types.User
		namespace string
	}
	type want struct {
		listPodsResult   []string
		getTestPodResult error
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "list default namespace pods for user with full access",
			args: args{
				user:      userWithFullAccess,
				namespace: metav1.NamespaceDefault,
			},
			want: want{
				listPodsResult: []string{
					"default/nginx-1",
					"default/nginx-2",
					"default/test",
				},
			},
		},
		{
			name: "list pods in every namespace for user with full access",
			args: args{
				user:      userWithFullAccess,
				namespace: metav1.NamespaceAll,
			},
			want: want{
				listPodsResult: []string{
					"default/nginx-1",
					"default/nginx-2",
					"default/test",
					"dev/nginx-1",
					"dev/nginx-2",
				},
			},
		},
		{
			name: "list default namespace pods for user with default namespace",
			args: args{
				user:      userWithNamespaceAccess,
				namespace: metav1.NamespaceDefault,
			},
			want: want{
				listPodsResult: []string{
					"default/nginx-1",
					"default/nginx-2",
					"default/test",
				},
			},
		},
		{
			name: "list pods in every namespace for user with default namespace",
			args: args{
				user:      userWithNamespaceAccess,
				namespace: metav1.NamespaceAll,
			},
			want: want{
				listPodsResult: []string{
					"default/nginx-1",
					"default/nginx-2",
					"default/test",
				},
			},
		},
		{
			name: "list default namespace pods for user with limited access",
			args: args{
				user:      userWithLimitedAccess,
				namespace: metav1.NamespaceDefault,
			},
			want: want{
				listPodsResult: []string{
					"default/nginx-1",
					"default/nginx-2",
				},
				getTestPodResult: &errors.StatusError{
					ErrStatus: metav1.Status{
						Status:  "Failure",
						Message: "[00] access denied",
						Code:    403,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// generate a kube client with user certs for auth
			client, _ := testCtx.genTestKubeClientTLSCert(
				t,
				tt.args.user.GetName(),
				kubeCluster,
			)

			rsp, err := client.CoreV1().Pods(tt.args.namespace).List(
				testCtx.ctx,
				metav1.ListOptions{},
			)
			require.NoError(t, err)

			require.Equal(t, tt.want.listPodsResult, getPodsFromPodList(rsp.Items))

			_, err = client.CoreV1().Pods(metav1.NamespaceDefault).Get(
				testCtx.ctx,
				testPodName,
				metav1.GetOptions{},
			)
			require.Equal(t, tt.want.getTestPodResult, err)
		})
	}
}

func getPodsFromPodList(items []corev1.Pod) []string {
	pods := make([]string, 0, len(items))
	for _, item := range items {
		pods = append(pods, filepath.Join(item.Namespace, item.Name))
	}
	return pods
}
