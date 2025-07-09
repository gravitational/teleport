/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport/api/types"
	testingkubemock "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
)

// pathRoutedKubeClient uses the given rest.Config to build a Kubernetes client
// using path-based routing derived from the provided Teleport and Kubernetes
// cluster names.
func pathRoutedKubeClient(t *testing.T, restConfig *rest.Config, teleportCluster, kubeCluster string) *kubernetes.Clientset {
	t.Helper()

	restConfig = rest.CopyConfig(restConfig)
	encTeleportCluster := base64.RawURLEncoding.EncodeToString([]byte(teleportCluster))
	encKubeCluster := base64.RawURLEncoding.EncodeToString([]byte(kubeCluster))
	restConfig.Host += fmt.Sprintf("/v1/teleport/%s/%s", encTeleportCluster, encKubeCluster)

	client, err := kubernetes.NewForConfig(restConfig)
	require.NoError(t, err)

	return client
}

func TestSingleCertRouting(t *testing.T) {
	kubeMockA, err := testingkubemock.NewKubeAPIMock()
	require.NoError(t, err)
	t.Cleanup(func() { kubeMockA.Close() })

	kubeMockB, err := testingkubemock.NewKubeAPIMock(
		// This endpoint returns a known mock error so we can determine from the
		// response which cluster the request was routed to.
		testingkubemock.WithGetPodError(
			metav1.Status{
				Status:  metav1.StatusFailure,
				Message: "cluster b error",
				Reason:  metav1.StatusReasonInternalError,
				Code:    http.StatusInternalServerError,
			},
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { kubeMockB.Close() })

	defaultRoleSpec := RoleSpec{
		Name:       roleName,
		KubeUsers:  roleKubeUsers,
		KubeGroups: roleKubeGroups,
	}

	const clusterName = "root.example.com"

	tests := []struct {
		name string

		roleSpec               RoleSpec
		assert                 func(t *testing.T, restConfig *rest.Config)
		genKubeCertificateOpts []GenTestKubeClientTLSCertOptions
	}{
		{
			name:     "successful path routing to multiple clusters",
			roleSpec: defaultRoleSpec,
			assert: func(t *testing.T, restConfig *rest.Config) {
				clientB := pathRoutedKubeClient(t, restConfig, clusterName, "b")
				_, err = clientB.CoreV1().Pods(metav1.NamespaceDefault).List(context.Background(), metav1.ListOptions{})
				require.NoError(t, err)

				clientA := pathRoutedKubeClient(t, restConfig, clusterName, "a")
				_, err := clientA.CoreV1().Pods(metav1.NamespaceDefault).List(context.Background(), metav1.ListOptions{})
				require.NoError(t, err)

			},
		},
		{
			name:     "cannot access nonexistent cluster",
			roleSpec: defaultRoleSpec,
			assert: func(t *testing.T, restConfig *rest.Config) {
				client := pathRoutedKubeClient(t, restConfig, clusterName, "c")
				_, err = client.CoreV1().Pods(metav1.NamespaceDefault).List(context.Background(), metav1.ListOptions{})
				require.ErrorContains(t, err, "not found")
			},
		},
		{
			name: "cannot access cluster denied by roles",
			roleSpec: RoleSpec{
				Name:       roleName,
				KubeUsers:  roleKubeUsers,
				KubeGroups: roleKubeGroups,
				SetupRoleFunc: func(r types.Role) {
					r.SetKubeResources(types.Deny, []types.KubernetesResource{{Kind: types.KindKubePod, Name: types.Wildcard, Namespace: types.Wildcard, Verbs: []string{types.Wildcard}}})
				},
			},
			assert: func(t *testing.T, restConfig *rest.Config) {
				client := pathRoutedKubeClient(t, restConfig, clusterName, "a")
				_, err = client.CoreV1().Pods(metav1.NamespaceDefault).List(context.Background(), metav1.ListOptions{})
				require.ErrorContains(t, err, "cannot list resource")
			},
		},
		{
			name:     "path route cannot override identity",
			roleSpec: defaultRoleSpec,
			genKubeCertificateOpts: []GenTestKubeClientTLSCertOptions{
				WithIdentityRoute(clusterName, "a"),
			},
			assert: func(t *testing.T, restConfig *rest.Config) {
				client := pathRoutedKubeClient(t, restConfig, clusterName, "b")
				_, err = client.CoreV1().Pods(metav1.NamespaceDefault).Get(context.Background(), "foo", metav1.GetOptions{})
				require.ErrorContains(t, err, "existing route in identity may not be overwritten")
			},
		},
		{
			name: "access is denied with per-session MFA enabled and no verification flag",
			roleSpec: RoleSpec{
				Name:       roleName,
				KubeUsers:  roleKubeUsers,
				KubeGroups: roleKubeGroups,
				SetupRoleFunc: func(r types.Role) {
					r.SetKubeResources(types.Allow, []types.KubernetesResource{{Kind: types.KindKubePod, Name: types.Wildcard, Namespace: types.Wildcard, Verbs: []string{types.Wildcard}}})
					r.SetOptions(types.RoleOptions{
						RequireMFAType: types.RequireMFAType_SESSION,
					})
				},
			},
			genKubeCertificateOpts: []GenTestKubeClientTLSCertOptions{
				WithIdentityRoute("", ""),
			},
			assert: func(t *testing.T, restConfig *rest.Config) {
				client := pathRoutedKubeClient(t, restConfig, clusterName, "a")
				_, err = client.CoreV1().Pods(metav1.NamespaceDefault).Get(context.Background(), "foo", metav1.GetOptions{})
				require.ErrorContains(t, err, "kubernetes cluster \"a\" not found")
			},
		},
		{
			name: "requires routing parameters when per-session MFA is enabled",
			roleSpec: RoleSpec{
				Name:       roleName,
				KubeUsers:  roleKubeUsers,
				KubeGroups: roleKubeGroups,
				SetupRoleFunc: func(r types.Role) {
					r.SetKubeResources(types.Allow, []types.KubernetesResource{{Kind: types.KindKubePod, Name: types.Wildcard, Namespace: types.Wildcard, Verbs: []string{types.Wildcard}}})
					r.SetOptions(types.RoleOptions{
						RequireMFAType: types.RequireMFAType_SESSION,
					})
				},
			},
			genKubeCertificateOpts: []GenTestKubeClientTLSCertOptions{
				WithIdentityRoute("", ""),
				WithMFAVerified(),
			},
			assert: func(t *testing.T, restConfig *rest.Config) {
				// If a user somehow manages to get auth to issue an MFA cert
				// with no routing parameters, we should refuse to route the
				// request arbitrarily.

				client := pathRoutedKubeClient(t, restConfig, clusterName, "a")
				_, err = client.CoreV1().Pods(metav1.NamespaceDefault).Get(context.Background(), "foo", metav1.GetOptions{})
				require.ErrorContains(t, err, "identity routing parameters are required")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCtx := SetupTestContext(
				context.Background(),
				t,
				TestConfig{
					Clusters: []KubeClusterConfig{
						{Name: "a", APIEndpoint: kubeMockA.URL},
						{Name: "b", APIEndpoint: kubeMockB.URL},
					},
				},
			)
			t.Cleanup(func() { require.NoError(t, testCtx.Close()) })

			_, _ = testCtx.CreateUserAndRole(
				testCtx.Context,
				t,
				username,
				tt.roleSpec)

			_, restConfig := testCtx.GenTestKubeClientTLSCert(
				t,
				username,
				"",
				tt.genKubeCertificateOpts...,
			)

			tt.assert(t, restConfig)
		})
	}

}
