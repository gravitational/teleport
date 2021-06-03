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
	"io/ioutil"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/teleport/lib/utils/testlog"
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
		err := checkImpersonationPermissions(context.Background(), mock)
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

func TestGetKubeCreds(t *testing.T) {
	ctx := context.TODO()
	const teleClusterName = "teleport-cluster"

	// Permission check is tested separately above.
	TestOnlySkipSelfPermissionCheck(true)
	defer TestOnlySkipSelfPermissionCheck(false)

	tmpFile, err := ioutil.TempFile("", "teleport")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	kubeconfigPath := tmpFile.Name()
	_, err = tmpFile.Write([]byte(`
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
users:
- name: creds
current-context: foo
`))
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	tests := []struct {
		desc           string
		kubeconfigPath string
		kubeCluster    string
		newService     bool
		want           map[string]*kubeCreds
		assertErr      require.ErrorAssertionFunc
	}{
		{
			desc:       "kubernetes_service, no kube creds",
			newService: true,
			assertErr:  require.Error,
		},
		{
			desc:      "proxy_service, no kube creds",
			assertErr: require.NoError,
			want:      map[string]*kubeCreds{},
		},
		{
			desc:           "kubernetes_service, with kube creds",
			newService:     true,
			kubeconfigPath: kubeconfigPath,
			want: map[string]*kubeCreds{
				"foo": {
					targetAddr:      "example.com:3026",
					transportConfig: &transport.Config{},
					kubeClient:      &kubernetes.Clientset{},
				},
				"bar": {
					targetAddr:      "example.com:3026",
					transportConfig: &transport.Config{},
					kubeClient:      &kubernetes.Clientset{},
				},
			},
			assertErr: require.NoError,
		},
		{
			desc:           "proxy_service, with kube creds",
			kubeconfigPath: kubeconfigPath,
			want: map[string]*kubeCreds{
				teleClusterName: {
					targetAddr:      "example.com:3026",
					transportConfig: &transport.Config{},
					kubeClient:      &kubernetes.Clientset{},
				},
			},
			assertErr: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := getKubeCreds(ctx, testlog.FailureOnly(t), teleClusterName, "", tt.kubeconfigPath, tt.newService)
			tt.assertErr(t, err)
			if err != nil {
				return
			}
			require.Empty(t, cmp.Diff(got, tt.want,
				cmp.AllowUnexported(kubeCreds{}),
				cmp.Comparer(func(a, b *transport.Config) bool { return (a == nil) == (b == nil) }),
				cmp.Comparer(func(a, b *kubernetes.Clientset) bool { return (a == nil) == (b == nil) }),
			))
		})
	}
}
