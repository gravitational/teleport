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

package k8s

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDestinationKubernetesSecret(t *testing.T) {
	defaultNamespace := "test-namespace"
	t.Setenv("POD_NAMESPACE", defaultNamespace)

	tests := []struct {
		name          string
		dest          *SecretDestination
		wantNamespace string

		wantErr string
	}{
		{
			name: "no existing secret",
			dest: &SecretDestination{
				Name: "my-secret",
				k8s:  fake.NewClientset(),
			},
			wantNamespace: defaultNamespace,
		},
		{
			name: "no existing secret with explicit namespace",
			dest: &SecretDestination{
				Name:      "my-secret",
				Namespace: "my-other-namespace",
				k8s:       fake.NewClientset(),
			},
			wantNamespace: "my-other-namespace",
		},
		{
			name: "labels",
			dest: &SecretDestination{
				Name: "my-secret",
				Labels: map[string]string{
					"key": "value",
					"bar": "baz",
				},
				k8s: fake.NewClientset(),
			},
			wantNamespace: defaultNamespace,
		},
		{
			name: "existing secret",
			dest: &SecretDestination{
				Name: "my-secret",
				k8s: fake.NewClientset(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-secret",
						Namespace: "test-namespace",
					},
				}),
			},
			wantNamespace: defaultNamespace,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

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
			secret, err := tt.dest.k8s.CoreV1().
				Secrets(tt.wantNamespace).
				Get(ctx, tt.dest.Name, metav1.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, tt.dest.Labels, secret.Labels)
		})
	}
}

func TestDestinationKubernetesSecret_CheckAndSetDefaults(t *testing.T) {
	tests := []testCheckAndSetDefaultsCase[*SecretDestination]{
		{
			name: "valid",
			in: func() *SecretDestination {
				return &SecretDestination{
					Name: "my-secret",
				}
			},
		},
		{
			name: "missing name",
			in: func() *SecretDestination {
				return &SecretDestination{}
			},
			wantErr: "name must not be empty",
		},
	}
	testCheckAndSetDefaults(t, tests)
}

func TestDestinationKubernetesSecret_YAML(t *testing.T) {
	tests := []testYAMLCase[*SecretDestination]{
		{
			name: "full",
			in: &SecretDestination{
				Name:      "my-secret",
				Namespace: "my-namespace",
				Labels: map[string]string{
					"key": "value",
				},
			},
		},
	}
	testYAML(t, tests)
}

func TestDestinationKubernetesSecret_String(t *testing.T) {
	require.Equal(
		t,
		"kubernetes_secret: foo/my-secret",
		(&SecretDestination{Namespace: "foo", Name: "my-secret"}).String(),
	)
}
