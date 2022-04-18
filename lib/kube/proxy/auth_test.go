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
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/stretchr/testify/require"
	"gopkg.in/check.v1"
	authzapi "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	authztypes "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/transport"
)

type AuthSuite struct{}

var _ = check.Suite(AuthSuite{})

func (s AuthSuite) TestCheckImpersonationPermissions(c *check.C) {
	tests := []struct {
		desc             string
		sarErr           error
		allowedVerbs     []string
		allowedResources []string

		wantErr bool
	}{
		{
			desc:    "request failure",
			sarErr:  errors.New("uh oh"),
			wantErr: true,
		},
		{
			desc:             "all permissions granted",
			allowedVerbs:     []string{"impersonate"},
			allowedResources: []string{"users", "groups", "serviceaccounts"},
			wantErr:          false,
		},
		{
			desc:             "missing some permissions",
			allowedVerbs:     []string{"impersonate"},
			allowedResources: []string{"users"},
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		c.Log(tt.desc)
		mock := &mockSARClient{
			err:              tt.sarErr,
			allowedVerbs:     tt.allowedVerbs,
			allowedResources: tt.allowedResources,
		}
		err := checkImpersonationPermissions(context.Background(), "test", mock)
		if tt.wantErr {
			c.Assert(err, check.NotNil)
		} else {
			c.Assert(err, check.IsNil)
		}
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

func failsForCluster(clusterName string) ImpersonationPermissionsChecker {
	return func(ctx context.Context, cluster string, a authztypes.SelfSubjectAccessReviewInterface) error {
		if cluster == clusterName {
			return errors.New("Kaboom")
		}
		return nil
	}
}

func TestGetKubeCreds(t *testing.T) {
	ctx := context.TODO()
	const teleClusterName = "teleport-cluster"
	testDir := t.TempDir()

	cert, err := utils.GenerateSelfSignedCert([]string{"localhost"})
	require.NoError(t, err)

	certFilePath := filepath.Join(testDir, "certfile")
	require.NoError(t, os.WriteFile(certFilePath, cert.Cert, fs.ModePerm))
	keyFilePath := filepath.Join(testDir, "keyfile")
	require.NoError(t, os.WriteFile(keyFilePath, cert.PrivateKey, fs.ModePerm))
	tlsConfig, err := utils.CreateTLSConfiguration(certFilePath, keyFilePath, utils.DefaultCipherSuites())
	require.NoError(t, err)

	kubeconfigPath := filepath.Join(testDir, "kubeconf")
	require.NoError(t, os.WriteFile(kubeconfigPath, []byte(fmt.Sprintf(`
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
  user:
    client-certificate: %s
    client-key: %s
current-context: foo
`, certFilePath, keyFilePath)), fs.ModePerm))

	kubeconfigbadPath := filepath.Join(testDir, "kubeconfbad")
	require.NoError(t, os.WriteFile(kubeconfigbadPath, []byte(`
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
`), fs.ModePerm))

	logger := utils.NewLoggerForTests()
	buf := bytes.NewBuffer([]byte{})
	logger.SetOutput(buf)
	sc := bufio.NewScanner(buf)

	tests := []struct {
		desc               string
		kubeconfigPath     string
		kubeCluster        string
		serviceType        KubeServiceType
		impersonationCheck ImpersonationPermissionsChecker
		want               map[string]*kubeCreds
		assertErr          require.ErrorAssertionFunc
	}{
		{
			desc:               "kubernetes_service, no kube creds",
			serviceType:        KubeService,
			impersonationCheck: alwaysSucceeds,
			assertErr:          require.Error,
		}, {
			desc:               "proxy_service, no kube creds",
			serviceType:        ProxyService,
			impersonationCheck: alwaysSucceeds,
			assertErr:          require.NoError,
			want:               map[string]*kubeCreds{},
		}, {
			desc:               "legacy proxy_service, no kube creds",
			serviceType:        ProxyService,
			impersonationCheck: alwaysSucceeds,
			assertErr:          require.NoError,
			want:               map[string]*kubeCreds{},
		}, {
			desc:               "kubernetes_service, with kube creds",
			serviceType:        KubeService,
			kubeconfigPath:     kubeconfigPath,
			impersonationCheck: alwaysSucceeds,
			want: map[string]*kubeCreds{
				"foo": {
					tlsConfig:       tlsConfig,
					targetAddr:      "example.com:3026",
					transportConfig: &transport.Config{},
					kubeClient:      &kubernetes.Clientset{},
				},
				"bar": {
					tlsConfig:       tlsConfig,
					targetAddr:      "example.com:3026",
					transportConfig: &transport.Config{},
					kubeClient:      &kubernetes.Clientset{},
				},
				"baz": {
					tlsConfig:       tlsConfig,
					targetAddr:      "example.com:3026",
					transportConfig: &transport.Config{},
					kubeClient:      &kubernetes.Clientset{},
				},
			},
			assertErr: require.NoError,
		}, {
			desc:               "proxy_service, with kube creds",
			kubeconfigPath:     kubeconfigPath,
			serviceType:        ProxyService,
			impersonationCheck: alwaysSucceeds,
			want:               map[string]*kubeCreds{},
			assertErr:          require.NoError,
		}, {
			desc:               "legacy proxy_service, with kube creds",
			kubeconfigPath:     kubeconfigPath,
			serviceType:        LegacyProxyService,
			impersonationCheck: alwaysSucceeds,
			want: map[string]*kubeCreds{
				teleClusterName: {
					tlsConfig:       tlsConfig,
					targetAddr:      "example.com:3026",
					transportConfig: &transport.Config{},
					kubeClient:      &kubernetes.Clientset{},
				},
			},
			assertErr: require.NoError,
		}, {
			desc:               "Missing cluster does not fail operation",
			kubeconfigPath:     kubeconfigPath,
			serviceType:        KubeService,
			impersonationCheck: failsForCluster("bar"),
			want: map[string]*kubeCreds{
				"foo": {
					tlsConfig:       tlsConfig,
					targetAddr:      "example.com:3026",
					transportConfig: &transport.Config{},
					kubeClient:      &kubernetes.Clientset{},
				},
				"bar": {
					tlsConfig:       tlsConfig,
					targetAddr:      "example.com:3026",
					transportConfig: &transport.Config{},
					kubeClient:      &kubernetes.Clientset{},
				},
				"baz": {
					tlsConfig:       tlsConfig,
					targetAddr:      "example.com:3026",
					transportConfig: &transport.Config{},
					kubeClient:      &kubernetes.Clientset{},
				},
			},
			assertErr: require.NoError,
		}, {
			desc:               "kubernetes_service, bad kube creds",
			serviceType:        KubeService,
			kubeconfigPath:     kubeconfigbadPath,
			impersonationCheck: alwaysSucceeds,
			assertErr: func(tt require.TestingT, err error, i ...interface{}) {
				findErr := "failed to generate TLS config from kubeConfig. clientConfig"
				for sc.Scan() {
					if strings.Contains(sc.Text(), findErr) {
						return
					}
				}
				t.Fatalf("Failed to find error %q in the logs", findErr)
			},
			want: map[string]*kubeCreds{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := getKubeCreds(ctx, logger, teleClusterName, "", tt.kubeconfigPath, tt.serviceType, tt.impersonationCheck)
			tt.assertErr(t, err)
			if err != nil {
				return
			}
			require.Empty(t, cmp.Diff(got, tt.want,
				cmp.AllowUnexported(kubeCreds{}),
				cmp.Comparer(func(a, b *transport.Config) bool { return (a == nil) == (b == nil) }),
				cmp.Comparer(func(a, b *kubernetes.Clientset) bool { return (a == nil) == (b == nil) }),
				cmp.Comparer(func(a, b *tls.Config) bool { return (a == nil) == (b == nil) }),
			))
		})
	}
}
