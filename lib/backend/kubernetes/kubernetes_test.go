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

package kubernetes

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	fakecorev1 "k8s.io/client-go/kubernetes/typed/core/v1/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/gravitational/teleport/lib/backend"
)

func TestBackend_Exists(t *testing.T) {
	type fields struct {
		namespace   string
		replicaName string
		objects     []runtime.Object
	}

	tests := []struct {
		name    string
		fields  fields
		want    bool
		wantErr bool
	}{
		{
			name: "secret does not exist",
			fields: fields{
				objects:     nil,
				namespace:   "test",
				replicaName: "agent-0",
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "secret exists",
			fields: fields{
				objects: []runtime.Object{
					newSecret("agent-0-state", "test", nil),
				},
				namespace:   "test",
				replicaName: "agent-0",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "secret exists but generates an error because KUBE_NAMESPACE is not set",
			fields: fields{
				objects:     nil,
				namespace:   "",
				replicaName: "agent-0",
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "secret exists but generates an error because TELEPORT_REPLICA_NAME is not set",
			fields: fields{
				objects: []runtime.Object{
					newSecret("agent-0-state", "test", nil),
				},
				namespace:   "test",
				replicaName: "",
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// set namespace env variable
			if len(tt.fields.namespace) > 0 {
				t.Setenv(NamespaceEnv, tt.fields.namespace)
			}

			// set replicaName env variable
			if len(tt.fields.replicaName) > 0 {
				t.Setenv(teleportReplicaNameEnv, tt.fields.replicaName)
			}

			k8sClient := fake.NewSimpleClientset(tt.fields.objects...)
			b, err := NewWithClient(k8sClient)
			if err != nil && !tt.wantErr {
				require.NoError(t, err)
			} else if err != nil && tt.wantErr {
				return
			}

			require.Equal(t, tt.want, b.Exists(context.TODO()))
		})
	}
}

func newSecret(name, namespace string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: v1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: generateSecretAnnotations(namespace, ""),
		},
		Data: data,
	}
}

func TestBackend_Get(t *testing.T) {
	var (
		payloadTestData = []byte("testData")
		payloadEmpty    = []byte("")
	)

	type fields struct {
		namespace   string
		replicaName string
		objects     []runtime.Object
	}

	type args struct {
		key backend.Key
	}

	tests := []struct {
		name         string
		fields       fields
		args         args
		want         []byte
		wantNotFound bool
	}{
		{
			name: "secret does not exist",
			fields: fields{
				objects:     nil,
				namespace:   "test",
				replicaName: "agent-0",
			},
			args: args{
				key: backend.NewKey("ids", "kube", "current"),
			},
			want: nil,

			wantNotFound: true,
		},
		{
			name: "secret exists and key is present",
			args: args{
				key: backend.NewKey("ids", "kube", "current"),
			},
			fields: fields{
				objects: []runtime.Object{
					newSecret(
						"agent-0-state",
						"test",
						map[string][]byte{
							backendKeyToSecret(backend.NewKey("ids", "kube", "current")): payloadTestData,
						},
					),
				},
				namespace:   "test",
				replicaName: "agent-0",
			},
			want: payloadTestData,
		},
		{
			name: "secret exists and key is present but empty",
			args: args{
				key: backend.NewKey("ids", "kube", "current"),
			},
			fields: fields{
				objects: []runtime.Object{
					newSecret(
						"agent-0-state",
						"test",
						map[string][]byte{
							backendKeyToSecret(backend.NewKey("ids", "kube", "current")): payloadEmpty,
						},
					),
				},
				namespace:   "test",
				replicaName: "agent-0",
			},
			want:         nil,
			wantNotFound: true,
		},
		{
			name: "secret exists but key not present",
			args: args{
				key: backend.NewKey("ids", "kube", "replacement"),
			},
			fields: fields{
				objects: []runtime.Object{
					newSecret(
						"agent-0-state",
						"test",
						map[string][]byte{
							backendKeyToSecret(backend.NewKey("ids", "kube", "current")): payloadTestData,
						},
					),
				},
				namespace:   "test",
				replicaName: "agent-0",
			},
			want:         nil,
			wantNotFound: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.fields.namespace) > 0 {
				t.Setenv(NamespaceEnv, tt.fields.namespace)
			}

			if len(tt.fields.replicaName) > 0 {
				t.Setenv(teleportReplicaNameEnv, tt.fields.replicaName)
			}

			k8sClient := fake.NewSimpleClientset(tt.fields.objects...)
			b, err := NewWithClient(k8sClient)
			require.NoError(t, err)

			got, err := b.Get(context.TODO(), tt.args.key)
			if (err != nil) && (!trace.IsNotFound(err) || !tt.wantNotFound) {
				require.NoError(t, err)
				return
			} else if (err != nil) && trace.IsNotFound(err) && tt.wantNotFound {
				return
			}
			require.Equal(t, tt.want, got.Value)
		})
	}
}

func TestBackend_Put(t *testing.T) {
	payloadTestData := []byte("testData")

	type fields struct {
		namespace   string
		replicaName string
		objects     []runtime.Object
	}

	type args struct {
		item backend.Item
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   *corev1.Secret
	}{
		{
			name: "secret does not exist and should be created",
			fields: fields{
				objects:     nil,
				namespace:   "test",
				replicaName: "agent-0",
			},
			args: args{
				item: backend.Item{
					Key:   backend.NewKey("ids", "kube", "current"),
					Value: payloadTestData,
				},
			},
			want: newSecret(
				"agent-0-state",
				"test",
				map[string][]byte{
					backendKeyToSecret(backend.NewKey("ids", "kube", "current")): payloadTestData,
				},
			),
		},
		{
			name: "secret exists and has keys",
			args: args{
				item: backend.Item{
					Key:   backend.NewKey("ids", "kube", "current2"),
					Value: payloadTestData,
				},
			},
			fields: fields{
				objects: []runtime.Object{
					newSecret(
						"agent-0-state",
						"test",
						map[string][]byte{
							backendKeyToSecret(backend.NewKey("ids", "kube", "current")): payloadTestData,
						},
					),
				},
				namespace:   "test",
				replicaName: "agent-0",
			},
			want: newSecret(
				"agent-0-state",
				"test",
				map[string][]byte{
					backendKeyToSecret(backend.NewKey("ids", "kube", "current")):  payloadTestData,
					backendKeyToSecret(backend.NewKey("ids", "kube", "current2")): payloadTestData,
				},
			),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// set namespace env var
			if len(tt.fields.namespace) > 0 {
				t.Setenv(NamespaceEnv, tt.fields.namespace)
			}

			// set replicaName env var
			if len(tt.fields.replicaName) > 0 {
				t.Setenv(teleportReplicaNameEnv, tt.fields.replicaName)
			}

			k8sClient := fake.NewSimpleClientset(tt.fields.objects...)

			b, err := NewWithClient(k8sClient)
			require.NoError(t, err)

			// k8s fake client does not support apply operations,
			// so we need to install a reactor to handle them.
			// https://github.com/kubernetes/kubernetes/issues/99953
			k8sClient.CoreV1().(*fakecorev1.FakeCoreV1).Fake.PrependReactor(
				"patch",
				"secrets",
				func(action k8stesting.Action) (bool, runtime.Object, error) {
					secretResourceVersion := corev1.SchemeGroupVersion.WithResource("secrets")
					applyAction := action.(k8stesting.PatchActionImpl)

					var secret *corev1.Secret

					// FIXME(tigrato): in the future merge  applyAction.Patch into the data
					// it requires unmarshal + merge into the structue with the given rules
					// for now it just grabs the Item from the test and sets the value.
					if obj, err := k8sClient.Tracker().Get(
						secretResourceVersion,
						applyAction.Namespace,
						applyAction.Name,
					); err == nil {
						secret = obj.(*corev1.Secret)
						secret.Data[backendKeyToSecret(tt.args.item.Key)] = tt.args.item.Value
						k8sClient.Tracker().Update(
							secretResourceVersion,
							secret,
							applyAction.Namespace,
						)
						return true, secret, nil
					}

					secret = newSecret(
						applyAction.Name,
						action.GetNamespace(),
						map[string][]byte{
							backendKeyToSecret(tt.args.item.Key): tt.args.item.Value,
						},
					)
					k8sClient.Tracker().Add(secret)

					return true, secret, nil
				},
			)
			// Put upserts the content in the secret
			_, err = b.Put(context.TODO(), tt.args.item)
			require.NoError(t, err)

			// get secret loads the kubernetes secret to compare.
			got, err := b.getSecret(context.TODO())
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
