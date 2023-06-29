/*
Copyright 2020 Gravitational, Inc.

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
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	authzapi "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	authztypes "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

func TestCheckImpersonationPermissions(t *testing.T) {
	tests := []struct {
		desc             string
		sarErr           error
		allowedVerbs     []string
		allowedResources []string
		errAssertion     require.ErrorAssertionFunc
	}{
		{
			desc:         "request failure",
			sarErr:       errors.New("uh oh"),
			errAssertion: require.Error,
		},
		{
			desc:             "all permissions granted",
			allowedVerbs:     []string{"impersonate"},
			allowedResources: []string{"users", "groups", "serviceaccounts"},
			errAssertion:     require.NoError,
		},
		{
			desc:             "missing some permissions",
			allowedVerbs:     []string{"impersonate"},
			allowedResources: []string{"users"},
			errAssertion:     require.Error,
		},
	}

	for _, tt := range tests {
		mock := &mockSARClient{
			err:              tt.sarErr,
			allowedVerbs:     tt.allowedVerbs,
			allowedResources: tt.allowedResources,
		}
		err := checkImpersonationPermissions(context.Background(), "test", mock)
		tt.errAssertion(t, err)
	}
}

type mockSARClient struct {
	authztypes.SelfSubjectAccessReviewInterface

	err              error
	allowedVerbs     []string
	allowedResources []string
}

func (c *mockSARClient) Create(_ context.Context, sar *authzapi.SelfSubjectAccessReview, _ metav1.CreateOptions) (*authzapi.SelfSubjectAccessReview, error) {
	if c.err != nil {
		return nil, c.err
	}

	var verbAllowed, resourceAllowed bool
	for _, v := range c.allowedVerbs {
		if v == sar.Spec.ResourceAttributes.Verb {
			verbAllowed = true
			break
		}
	}
	for _, r := range c.allowedResources {
		if r == sar.Spec.ResourceAttributes.Resource {
			resourceAllowed = true
			break
		}
	}

	sar.Status.Allowed = verbAllowed && resourceAllowed
	return sar, nil
}

func alwaysSucceeds(context.Context, string, authztypes.SelfSubjectAccessReviewInterface) error {
	return nil
}

func failsForCluster(clusterName string) servicecfg.ImpersonationPermissionsChecker {
	return func(ctx context.Context, cluster string, a authztypes.SelfSubjectAccessReviewInterface) error {
		if cluster == clusterName {
			return errors.New("Kaboom")
		}
		return nil
	}
}

func TestGetKubeCreds(t *testing.T) {
	t.Parallel()
	logger := utils.NewLoggerForTests()
	ctx := context.TODO()
	const teleClusterName = "teleport-cluster"
	dir := t.TempDir()
	kubeconfigPath := filepath.Join(dir, "kubeconfig")
	data := []byte(`
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://example.com:3026
  name: example
contexts:
- context:
    cluster: example
    user: creds
  name: foo
- context:
    cluster: example
    user: creds
  name: bar
- context:
    cluster: example
    user: creds
  name: baz
users:
- name: creds
current-context: foo
`)
	err := os.WriteFile(kubeconfigPath, data, 0o600)
	require.NoError(t, err)

	tests := []struct {
		desc               string
		kubeconfigPath     string
		kubeCluster        string
		serviceType        KubeServiceType
		impersonationCheck servicecfg.ImpersonationPermissionsChecker
		want               map[string]*kubeDetails
		assertErr          require.ErrorAssertionFunc
	}{
		{
			desc:               "kubernetes_service, no kube creds",
			serviceType:        KubeService,
			impersonationCheck: alwaysSucceeds,
			assertErr:          require.Error,
			want:               map[string]*kubeDetails{},
		}, {
			desc:               "proxy_service, no kube creds",
			serviceType:        ProxyService,
			impersonationCheck: alwaysSucceeds,
			assertErr:          require.NoError,
			want:               map[string]*kubeDetails{},
		}, {
			desc:               "legacy proxy_service, no kube creds",
			serviceType:        ProxyService,
			impersonationCheck: alwaysSucceeds,
			assertErr:          require.NoError,
			want:               map[string]*kubeDetails{},
		}, {
			desc:               "kubernetes_service, with kube creds",
			serviceType:        KubeService,
			kubeconfigPath:     kubeconfigPath,
			impersonationCheck: alwaysSucceeds,
			want: map[string]*kubeDetails{
				"foo": {
					kubeCreds: &staticKubeCreds{
						targetAddr:      "example.com:3026",
						transportConfig: &transport.Config{},
						kubeClient:      &kubernetes.Clientset{},
						clientRestCfg:   &rest.Config{},
					},
					kubeCluster: mustCreateKubernetesClusterV3(t, "foo"),
				},
				"bar": {
					kubeCreds: &staticKubeCreds{
						targetAddr:      "example.com:3026",
						transportConfig: &transport.Config{},
						kubeClient:      &kubernetes.Clientset{},
						clientRestCfg:   &rest.Config{},
					},
					kubeCluster: mustCreateKubernetesClusterV3(t, "bar"),
				},
				"baz": {
					kubeCreds: &staticKubeCreds{
						targetAddr:      "example.com:3026",
						transportConfig: &transport.Config{},
						kubeClient:      &kubernetes.Clientset{},
						clientRestCfg:   &rest.Config{},
					},
					kubeCluster: mustCreateKubernetesClusterV3(t, "baz"),
				},
			},
			assertErr: require.NoError,
		}, {
			desc:               "proxy_service, with kube creds",
			kubeconfigPath:     kubeconfigPath,
			serviceType:        ProxyService,
			impersonationCheck: alwaysSucceeds,
			want:               map[string]*kubeDetails{},
			assertErr:          require.NoError,
		}, {
			desc:               "legacy proxy_service, with kube creds",
			kubeconfigPath:     kubeconfigPath,
			serviceType:        LegacyProxyService,
			impersonationCheck: alwaysSucceeds,
			want: map[string]*kubeDetails{
				teleClusterName: {
					kubeCreds: &staticKubeCreds{
						targetAddr:      "example.com:3026",
						transportConfig: &transport.Config{},
						kubeClient:      &kubernetes.Clientset{},
						clientRestCfg:   &rest.Config{},
					},
					kubeCluster: mustCreateKubernetesClusterV3(t, teleClusterName),
				},
			},
			assertErr: require.NoError,
		}, {
			desc:               "Missing cluster does not fail operation",
			kubeconfigPath:     kubeconfigPath,
			serviceType:        KubeService,
			impersonationCheck: failsForCluster("bar"),
			want: map[string]*kubeDetails{
				"foo": {
					kubeCreds: &staticKubeCreds{
						targetAddr:      "example.com:3026",
						transportConfig: &transport.Config{},
						kubeClient:      &kubernetes.Clientset{},
						clientRestCfg:   &rest.Config{},
					},
					kubeCluster: mustCreateKubernetesClusterV3(t, "foo"),
				},
				"bar": {
					kubeCreds: &staticKubeCreds{
						targetAddr:      "example.com:3026",
						transportConfig: &transport.Config{},
						kubeClient:      &kubernetes.Clientset{},
						clientRestCfg:   &rest.Config{},
					},
					kubeCluster: mustCreateKubernetesClusterV3(t, "bar"),
				},
				"baz": {
					kubeCreds: &staticKubeCreds{
						targetAddr:      "example.com:3026",
						transportConfig: &transport.Config{},
						kubeClient:      &kubernetes.Clientset{},
						clientRestCfg:   &rest.Config{},
					},
					kubeCluster: mustCreateKubernetesClusterV3(t, "baz"),
				},
			},
			assertErr: require.NoError,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			got, err := getKubeDetails(ctx, logger, teleClusterName, "", tt.kubeconfigPath, tt.serviceType, tt.impersonationCheck)
			tt.assertErr(t, err)
			if err != nil {
				return
			}
			require.Empty(t, cmp.Diff(got, tt.want,
				cmp.AllowUnexported(staticKubeCreds{}),
				cmp.AllowUnexported(kubeDetails{}),
				cmp.AllowUnexported(httpTransport{}),
				cmp.Comparer(func(a, b *transport.Config) bool { return (a == nil) == (b == nil) }),
				cmp.Comparer(func(a, b *kubernetes.Clientset) bool { return (a == nil) == (b == nil) }),
				cmp.Comparer(func(a, b *rest.Config) bool { return (a == nil) == (b == nil) }),
				cmp.Comparer(func(a, b httpTransport) bool { return true }),
			))
		})
	}
}

func mustCreateKubernetesClusterV3(t *testing.T, name string) *types.KubernetesClusterV3 {
	kubeCluster, err := types.NewKubernetesClusterV3(types.Metadata{
		Name: name,
	}, types.KubernetesClusterSpecV3{})
	require.NoError(t, err)
	return kubeCluster
}
