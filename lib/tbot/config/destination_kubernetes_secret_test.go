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
			name: "labels",
			dest: &DestinationKubernetesSecret{
				Name: "my-secret",
				Labels: map[string]string{
					"key": "value",
					"bar": "baz",
				},
				k8s: fakeClientSet(),
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

			// Test individual write
			require.NoError(t, tt.dest.Write(ctx, "artifact-a", []byte("data-a")))
			require.NoError(t, tt.dest.Write(ctx, "artifact-b", []byte("data-b")))
			aData, err := tt.dest.Read(ctx, "artifact-a")
			require.NoError(t, err)
			require.Equal(t, []byte("data-a"), aData)
			bData, err := tt.dest.Read(ctx, "artifact-b")
			require.NoError(t, err)
			require.Equal(t, []byte("data-b"), bData)

			// Test write many
			require.NoError(t, tt.dest.WriteMany(ctx, map[string][]byte{
				"artifact-a": []byte("data-c"),
				"artifact-b": []byte("data-d"),
			}))
			aData, err = tt.dest.Read(ctx, "artifact-a")
			require.NoError(t, err)
			require.Equal(t, []byte("data-c"), aData)
			bData, err = tt.dest.Read(ctx, "artifact-b")
			require.NoError(t, err)
			require.Equal(t, []byte("data-d"), bData)

			// Check labels have been set
			secret, err := tt.dest.k8s.CoreV1().Secrets("test-namespace").Get(ctx, tt.dest.Name, metav1.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, tt.dest.Labels, secret.Labels)
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
