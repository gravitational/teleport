/*
Copyright 2023 Gravitational, Inc.

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

package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
)

func TestDestinationKubernetesSecret(t *testing.T) {
	t.Setenv("POD_NAMESPACE", "test-namespace")

	// Hack a reactor into the Kubernetes client-go fake client set as it
	// doesn't currently support Apply :)
	// https://github.com/kubernetes/kubernetes/issues/99953
	fakeClientSet := func(objects ...runtime.Object) *fake.Clientset {
		f := fake.NewSimpleClientset(objects...)
		f.PrependReactor("patch", "secrets", func(action core.Action) (handled bool, ret runtime.Object, err error) {
			pa := action.(core.PatchAction)
			if pa.GetPatchType() == types.ApplyPatchType {
				react := core.ObjectReaction(f.Tracker())
				_, _, err := react(
					core.NewGetAction(pa.GetResource(), pa.GetNamespace(), pa.GetName()),
				)
				if kubeerrors.IsNotFound(err) {
					_, _, err = react(
						core.NewCreateAction(pa.GetResource(), pa.GetNamespace(), &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      pa.GetName(),
								Namespace: pa.GetNamespace(),
							},
						}),
					)
					if err != nil {
						return false, nil, err
					}
				}
				return react(action)
			}
			return false, nil, nil
		})
		return f
	}

	tests := []struct {
		name string
		dest *DestinationKubernetesSecret

		wantErr string
	}{
		{
			name: "no existing secret",
			dest: &DestinationKubernetesSecret{
				Name: "my-secret",
				k8s:  fakeClientSet(),
			},
		},
		{
			name: "existing secret",
			dest: &DestinationKubernetesSecret{
				Name: "my-secret",
				k8s: fakeClientSet(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-secret",
						Namespace: "test-namespace",
					},
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			require.NoError(t, tt.dest.Init(ctx, []string{}))
			require.NoError(t, tt.dest.Write(ctx, "artifact-a", []byte("data-a")))
			require.NoError(t, tt.dest.Write(ctx, "artifact-b", []byte("data-b")))
			aData, err := tt.dest.Read(ctx, "artifact-a")
			require.NoError(t, err)
			require.Equal(t, []byte("data-a"), aData)
			bData, err := tt.dest.Read(ctx, "artifact-b")
			require.NoError(t, err)
			require.Equal(t, []byte("data-b"), bData)
		})
	}
}

func TestDestinationKubernetesSecret_CheckAndSetDefaults(t *testing.T) {
	tests := []testCheckAndSetDefaultsCase[*DestinationKubernetesSecret]{
		{
			name: "valid",
			in: func() *DestinationKubernetesSecret {
				return &DestinationKubernetesSecret{
					Name: "my-secret",
				}
			},
		},
		{
			name: "missing name",
			in: func() *DestinationKubernetesSecret {
				return &DestinationKubernetesSecret{
					Name: "",
				}
			},
			wantErr: "name must not be empty",
		},
	}
	testCheckAndSetDefaults(t, tests)
}

func TestDestinationKubernetesSecret_YAML(t *testing.T) {
	tests := []testYAMLCase[*DestinationKubernetesSecret]{
		{
			name: "full",
			in: &DestinationKubernetesSecret{
				Name: "my-secret",
			},
		},
	}
	testYAML(t, tests)
}

func TestDestinationKubernetesSecret_String(t *testing.T) {
	require.Equal(t, "kubernetes_secret: my-secret", (&DestinationKubernetesSecret{Name: "my-secret"}).String())
}
