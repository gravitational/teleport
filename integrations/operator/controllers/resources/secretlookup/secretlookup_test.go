/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package secretlookup

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLookupSecret(t *testing.T) {
	// Test setup: crafting fixtures
	ctx := context.Background()
	namespace := "foo"
	crName := "test-cr-name"
	secretName := "test-secret"
	keyName := "test-key-name"
	keyValue := "test-key-value"

	secretWithAnnotations := func(annotations map[string]string) v1.Secret {
		return v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        secretName,
				Namespace:   namespace,
				Annotations: annotations,
			},
			Data: map[string][]byte{
				keyName: []byte(keyValue),
			},
			StringData: nil,
			Type:       v1.SecretTypeOpaque,
		}
	}
	okAnnotations := map[string]string{AllowLookupAnnotation: strings.Join([]string{"other-cr-name", crName}, ", ")}
	okSecret := secretWithAnnotations(okAnnotations)

	// Test setup: defining test cases
	tests := []struct {
		name      string
		input     string
		secrets   v1.SecretList
		expect    string
		assertErr require.ErrorAssertionFunc
	}{
		{
			name:    "not an uri",
			input:   "secret://How do you do, fellow kids?",
			secrets: v1.SecretList{Items: []v1.Secret{okSecret}},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "parse")
			},
		},
		{
			name:    "uri does not contain secret name",
			input:   fmt.Sprintf("secret:///%s", keyName),
			secrets: v1.SecretList{Items: []v1.Secret{okSecret}},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "missing secret name")
			},
		},
		{
			name:    "uri does not contain secret key",
			input:   fmt.Sprintf("secret://%s", secretName),
			secrets: v1.SecretList{Items: []v1.Secret{okSecret}},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "missing secret key")
			},
		},
		{
			name:    "secret does not exist",
			input:   fmt.Sprintf("secret://%s/%s", secretName, keyName),
			secrets: v1.SecretList{Items: []v1.Secret{}},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "not found")
			},
		},
		{
			name:    "secret has no annotation",
			input:   fmt.Sprintf("secret://%s/%s", secretName, keyName),
			secrets: v1.SecretList{Items: []v1.Secret{secretWithAnnotations(nil)}},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "annotation")
			},
		},
		{
			name:    "secret annotations don't allow inclusion",
			input:   fmt.Sprintf("secret://%s/%s", secretName, keyName),
			secrets: v1.SecretList{Items: []v1.Secret{secretWithAnnotations(map[string]string{AllowLookupAnnotation: "not-the-right-cr"})}},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "does not contain")
			},
		},
		{
			name:  "secret is missing the key",
			input: fmt.Sprintf("secret://%s/%s", secretName, keyName),
			secrets: v1.SecretList{
				Items: []v1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:        secretName,
							Namespace:   namespace,
							Annotations: okAnnotations,
						},
						Data: map[string][]byte{
							"wrong-key-name": []byte(keyValue),
						},
						StringData: nil,
						Type:       v1.SecretTypeOpaque,
					},
				},
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "is missing key")
			},
		},
		{
			name:      "successful lookup",
			input:     fmt.Sprintf("secret://%s/%s", secretName, keyName),
			secrets:   v1.SecretList{Items: []v1.Secret{okSecret}},
			expect:    keyValue,
			assertErr: require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test setup: creating a mock with the fixtures
			clientBuilder := fake.NewClientBuilder()
			clientBuilder.WithLists(&tt.secrets)
			fakeClient := clientBuilder.Build()

			// Test execution
			result, err := Try(ctx, fakeClient, crName, namespace, tt.input)
			t.Log(err)
			tt.assertErr(t, err)
			require.Equal(t, tt.expect, result)
		})
	}
}

func Test_isInclusionAllowed(t *testing.T) {
	crName := "test-cr-name"
	tests := []struct {
		name      string
		secret    v1.Secret
		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "No annotation",
			secret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: nil,
				},
			},
			assertErr: require.Error,
		},
		{
			name: "Missing inclusion annotation",
			secret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"foo": "bar",
					},
				},
			},
			assertErr: require.Error,
		},
		{
			name: "Empty inclusion annotation",
			secret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						AllowLookupAnnotation: "",
					},
				},
			},
			assertErr: require.Error,
		},
		{
			name: "Not matching inclusion annotation",
			secret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						AllowLookupAnnotation: "foo, bar",
					},
				},
			},
			assertErr: require.Error,
		},
		{
			name: "Single-item matching annotation",
			secret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						AllowLookupAnnotation: crName,
					},
				},
			},
			assertErr: require.NoError,
		},
		{
			name: "Multi-item matching annotation",
			secret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						AllowLookupAnnotation: strings.Join([]string{"foo", "bar", crName}, ", "),
					},
				},
			},
			assertErr: require.NoError,
		},
		{
			name: "Wildcard annotation",
			secret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						AllowLookupAnnotation: "*",
					},
				},
			},
			assertErr: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isInclusionAllowed(&tt.secret, crName)
			tt.assertErr(t, result)
		})
	}
}
