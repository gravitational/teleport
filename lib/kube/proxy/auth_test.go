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
	"crypto/tls"
	"errors"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	authzapi "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryversion "k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	authztypes "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"

	"github.com/gravitational/teleport/api/types"
	testingkubemock "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
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
	// kubeMock is a Kubernetes API mock for the session tests.
	kubeMock, err := testingkubemock.NewKubeAPIMock()
	require.NoError(t, err)
	t.Cleanup(func() { kubeMock.Close() })
	targetAddr := kubeMock.Address

	rbacSupportedTypes := maps.Clone(defaultRBACResources)
	rbacSupportedTypes[allowedResourcesKey{apiGroup: "resources.teleport.dev", resourceKind: "teleportroles"}] = utils.KubeCustomResource
	rbacSupportedTypes[allowedResourcesKey{apiGroup: "resources.teleport.dev", resourceKind: "teleportroles/status"}] = utils.KubeCustomResource

	ctx := context.TODO()
	const teleClusterName = "teleport-cluster"
	dir := t.TempDir()
	kubeconfigPath := filepath.Join(dir, "kubeconfig")
	data := []byte(`
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: ` + kubeMock.URL + `
    insecure-skip-tls-verify: true
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
	err = os.WriteFile(kubeconfigPath, data, 0o600)
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
						targetAddr:      targetAddr,
						transportConfig: &transport.Config{},
						kubeClient:      &kubernetes.Clientset{},
						clientRestCfg:   &rest.Config{},
					},
					kubeCluster: mustCreateKubernetesClusterV3(t, "foo"),
					kubeClusterVersion: &apimachineryversion.Info{
						Major:      "1",
						Minor:      "20",
						GitVersion: "1.20.0",
					},
					rbacSupportedTypes: rbacSupportedTypes,
				},
				"bar": {
					kubeCreds: &staticKubeCreds{
						targetAddr:      targetAddr,
						transportConfig: &transport.Config{},
						kubeClient:      &kubernetes.Clientset{},
						clientRestCfg:   &rest.Config{},
					},
					kubeClusterVersion: &apimachineryversion.Info{
						Major:      "1",
						Minor:      "20",
						GitVersion: "1.20.0",
					},
					kubeCluster:        mustCreateKubernetesClusterV3(t, "bar"),
					rbacSupportedTypes: rbacSupportedTypes,
				},
				"baz": {
					kubeCreds: &staticKubeCreds{
						targetAddr:      targetAddr,
						transportConfig: &transport.Config{},
						kubeClient:      &kubernetes.Clientset{},
						clientRestCfg:   &rest.Config{},
					},
					kubeClusterVersion: &apimachineryversion.Info{
						Major:      "1",
						Minor:      "20",
						GitVersion: "1.20.0",
					},
					kubeCluster:        mustCreateKubernetesClusterV3(t, "baz"),
					rbacSupportedTypes: rbacSupportedTypes,
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
						targetAddr:      targetAddr,
						transportConfig: &transport.Config{},
						kubeClient:      &kubernetes.Clientset{},
						clientRestCfg:   &rest.Config{},
					},
					kubeClusterVersion: &apimachineryversion.Info{
						Major:      "1",
						Minor:      "20",
						GitVersion: "1.20.0",
					},
					kubeCluster:        mustCreateKubernetesClusterV3(t, teleClusterName),
					rbacSupportedTypes: rbacSupportedTypes,
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
						targetAddr:      targetAddr,
						transportConfig: &transport.Config{},
						kubeClient:      &kubernetes.Clientset{},
						clientRestCfg:   &rest.Config{},
					},
					kubeClusterVersion: &apimachineryversion.Info{
						Major:      "1",
						Minor:      "20",
						GitVersion: "1.20.0",
					},
					kubeCluster:        mustCreateKubernetesClusterV3(t, "foo"),
					rbacSupportedTypes: rbacSupportedTypes,
				},
				"bar": {
					kubeCreds: &staticKubeCreds{
						targetAddr:      targetAddr,
						transportConfig: &transport.Config{},
						kubeClient:      &kubernetes.Clientset{},
						clientRestCfg:   &rest.Config{},
					},
					kubeClusterVersion: &apimachineryversion.Info{
						Major:      "1",
						Minor:      "20",
						GitVersion: "1.20.0",
					},
					kubeCluster:        mustCreateKubernetesClusterV3(t, "bar"),
					rbacSupportedTypes: rbacSupportedTypes,
				},
				"baz": {
					kubeCreds: &staticKubeCreds{
						targetAddr:      targetAddr,
						transportConfig: &transport.Config{},
						kubeClient:      &kubernetes.Clientset{},
						clientRestCfg:   &rest.Config{},
					},
					kubeClusterVersion: &apimachineryversion.Info{
						Major:      "1",
						Minor:      "20",
						GitVersion: "1.20.0",
					},
					kubeCluster:        mustCreateKubernetesClusterV3(t, "baz"),
					rbacSupportedTypes: rbacSupportedTypes,
				},
			},
			assertErr: require.NoError,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			fwd := &Forwarder{
				clusterDetails: map[string]*kubeDetails{},
				cfg: ForwarderConfig{
					ClusterName:                   teleClusterName,
					KubeServiceType:               tt.serviceType,
					KubeconfigPath:                tt.kubeconfigPath,
					CheckImpersonationPermissions: tt.impersonationCheck,
					Clock:                         clockwork.NewFakeClock(),
				},
				log: utils.NewSlogLoggerForTests(),
			}
			err := fwd.getKubeDetails(ctx)
			tt.assertErr(t, err)
			if err != nil {
				return
			}
			require.Empty(t, cmp.Diff(fwd.clusterDetails, tt.want,
				cmp.AllowUnexported(staticKubeCreds{}),
				cmp.AllowUnexported(kubeDetails{}),
				cmpopts.IgnoreFields(kubeDetails{}, "rwMu", "kubeCodecs", "wg", "cancelFunc", "gvkSupportedResources"),
				cmp.Comparer(func(a, b *transport.Config) bool { return (a == nil) == (b == nil) }),
				cmp.Comparer(func(a, b *tls.Config) bool { return true }),
				cmp.Comparer(func(a, b *kubernetes.Clientset) bool { return (a == nil) == (b == nil) }),
				cmp.Comparer(func(a, b *rest.Config) bool { return (a == nil) == (b == nil) }),
				cmp.Comparer(func(a, b http.RoundTripper) bool { return true }),
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
