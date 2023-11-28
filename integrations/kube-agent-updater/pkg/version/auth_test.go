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

package version

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAuthVersionGetter_Get(t *testing.T) {
	// Test setup: generating and loading fixtures
	ctx := context.Background()
	namespace := "bar"

	fixtures := &v1.SecretList{Items: []v1.Secret{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "no-key-shared-state", Namespace: namespace},
			Data:       map[string][]byte{"foo": []byte("bar")},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "invalid-json-shared-state", Namespace: namespace},
			Data:       map[string][]byte{authVersionKeyName: []byte(`{"foo": "bar"}`)},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "invalid-auth-version-shared-state", Namespace: namespace},
			Data:       map[string][]byte{authVersionKeyName: []byte(".13")},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "valid-auth-version-shared-state", Namespace: namespace},
			Data:       map[string][]byte{authVersionKeyName: []byte("13.4.5")},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "prefix-shared-state", Namespace: namespace},
			Data:       map[string][]byte{authVersionKeyName: []byte("v13.4.5")},
		},
	}}

	clientBuilder := fake.NewClientBuilder()
	clientBuilder.WithLists(fixtures)
	client := clientBuilder.Build()

	tests := []struct {
		name      string
		object    kclient.Object
		want      string
		assertErr require.ErrorAssertionFunc
	}{
		{
			name:      "no secret",
			object:    &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "not-found", Namespace: namespace}},
			assertErr: require.Error,
		},
		{
			name:      "secret no key",
			object:    &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "no-key", Namespace: namespace}},
			assertErr: require.Error,
		},
		{
			name:      "secret invalid JSON",
			object:    &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "invalid-json", Namespace: namespace}},
			assertErr: require.Error,
		},
		{
			name:      "secret invalid auth version",
			object:    &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "invalid-auth-version", Namespace: namespace}},
			assertErr: require.Error,
		},
		{
			name:      "valid auth version",
			object:    &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "valid-auth-version", Namespace: namespace}},
			want:      "v13.4.5",
			assertErr: require.NoError,
		},
		{
			name:      "prefix auth version",
			object:    &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "prefix", Namespace: namespace}},
			want:      "v13.4.5",
			assertErr: require.NoError,
		},
	}
	// Doing the real test
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authVersionGetter := NewAuthVersionGetter(client)
			version, err := authVersionGetter.Get(ctx, tt.object)
			tt.assertErr(t, err)
			require.Equal(t, tt.want, version)
		})
	}
}
