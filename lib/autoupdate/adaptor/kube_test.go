/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package adaptor

import (
	"github.com/google/uuid"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend/kubernetes"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"
	"testing"
)

func TestCreateKubeUpdaterID(t *testing.T) {
	releaseName := "test-release"
	releaseNamespace := "test-namespace"
	t.Setenv(kubernetes.ReleaseNameEnv, releaseName)
	t.Setenv(kubernetes.NamespaceEnv, releaseNamespace)
	secretName := releaseName + "-shared-state"
	existingUUID := uuid.New()

	tests := []struct {
		name      string
		fixtures  []runtime.Object
		assertID  func(t *testing.T, id uuid.UUID)
		assertErr require.ErrorAssertionFunc
	}{
		{
			name:      "no secret",
			fixtures:  []runtime.Object{},
			assertErr: require.NoError,
			assertID: func(t *testing.T, id uuid.UUID) {
				require.NotEqual(t, uuid.Nil, id)
			},
		},
		{
			name: "secret exists but has a different key",
			fixtures: []runtime.Object{
				&corev1.Secret{
					Type: corev1.SecretTypeOpaque,
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretName,
						Namespace: releaseNamespace,
					},
					Data: map[string][]byte{
						"some key": []byte("some value"),
					},
				},
			},
			assertErr: require.NoError,
			assertID: func(t *testing.T, id uuid.UUID) {
				require.NotEqual(t, uuid.Nil, id)
			},
		},
		{
			name: "secret exists and has the key",
			fixtures: []runtime.Object{
				&corev1.Secret{
					Type: corev1.SecretTypeOpaque,
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretName,
						Namespace: releaseNamespace,
					},
					Data: map[string][]byte{
						"." + teleport.UpdaterIDKubeBackendKey: []byte(existingUUID.String()),
					},
				},
			},
			assertErr: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsRetryError(err))
			},
			assertID: func(t *testing.T, id uuid.UUID) {
				require.Equal(t, existingUUID, id)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := fake.NewClientset(test.fixtures...)
			backend, err := kubernetes.NewSharedWithClient(client)
			require.NoError(t, err)

			err = CreateKubeUpdaterID(t.Context(), backend)
			test.assertErr(t, err)
			id, err := LookupKubeUpdaterID(t.Context(), backend)
			require.NoError(t, err)
			test.assertID(t, id)
		})
	}
}

func TestLookupKubeUpdaterID(t *testing.T) {
	releaseName := "test-release"
	releaseNamespace := "test-namespace"
	t.Setenv(kubernetes.ReleaseNameEnv, releaseName)
	t.Setenv(kubernetes.NamespaceEnv, releaseNamespace)
	secretName := releaseName + "-shared-state"
	existingUUID := uuid.New()

	tests := []struct {
		name      string
		fixtures  []runtime.Object
		assertID  func(t *testing.T, id uuid.UUID)
		assertErr require.ErrorAssertionFunc
	}{
		{
			name:     "no secret",
			fixtures: []runtime.Object{},
			assertErr: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsNotFound(err))
			},
			assertID: func(t *testing.T, id uuid.UUID) {},
		},
		{
			name: "secret exists but has a different key",
			fixtures: []runtime.Object{
				&corev1.Secret{
					Type: corev1.SecretTypeOpaque,
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretName,
						Namespace: releaseNamespace,
					},
					Data: map[string][]byte{
						"some key": []byte("some value"),
					},
				},
			},
			assertErr: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsNotFound(err))
			},
			assertID: func(t *testing.T, id uuid.UUID) {},
		},
		{
			name: "secret exists and has the key",
			fixtures: []runtime.Object{
				&corev1.Secret{
					Type: corev1.SecretTypeOpaque,
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretName,
						Namespace: releaseNamespace,
					},
					Data: map[string][]byte{
						"." + teleport.UpdaterIDKubeBackendKey: []byte(existingUUID.String()),
					},
				},
			},
			assertErr: require.NoError,
			assertID: func(t *testing.T, id uuid.UUID) {
				require.Equal(t, existingUUID, id)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := fake.NewClientset(test.fixtures...)
			backend, err := kubernetes.NewSharedWithClient(client)
			require.NoError(t, err)

			id, err := LookupKubeUpdaterID(t.Context(), backend)
			test.assertErr(t, err)
			test.assertID(t, id)
		})
	}
}

type firstGetNotFoundReactor struct {
	resourceName      string
	resourceNamespace string
	gvr               schema.GroupVersionResource
	called            bool
}

func (r *firstGetNotFoundReactor) Handles(action kubetesting.Action) bool {
	// Target get requests for our specific resource
	gvr := action.GetResource()
	if gvr.Group != r.gvr.Group || gvr.Version != r.gvr.Version || gvr.Resource != r.gvr.Resource {
		return false
	}
	if action.GetNamespace() != r.resourceNamespace {
		return false
	}
	if action.GetVerb() != "get" {
		return false
	}
	getAction, ok := action.(kubetesting.GetAction)
	if !ok {
		return false
	}
	if getAction.GetName() != r.resourceName {
		return false
	}
	return true
}

func (r *firstGetNotFoundReactor) React(action kubetesting.Action) (bool, runtime.Object, error) {
	if !r.Handles(action) {
		return false, nil, nil
	}

	// If we got called once arleady, we don't do anything
	if r.called {
		return false, nil, nil
	}

	// This is the first call, we simulate a missing resource
	r.called = true
	return true, nil, errors.NewNotFound(action.GetResource().GroupResource(), r.resourceName)
}

func TestCreateKubeUpdaterIDIfEmpty(t *testing.T) {
	releaseName := "test-release"
	releaseNamespace := "test-namespace"
	t.Setenv(kubernetes.ReleaseNameEnv, releaseName)
	t.Setenv(kubernetes.NamespaceEnv, releaseNamespace)
	secretName := releaseName + "-shared-state"
	existingUUID := uuid.New()

	reactor := &firstGetNotFoundReactor{
		resourceName:      secretName,
		resourceNamespace: releaseNamespace,
		gvr: schema.GroupVersionResource{
			Version:  "v1",
			Resource: "secrets",
			Group:    "",
		},
	}

	existingSecret := &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: releaseNamespace,
		},
		Data: map[string][]byte{
			"." + teleport.UpdaterIDKubeBackendKey: []byte(existingUUID.String()),
		},
	}

	client := fake.NewClientset(existingSecret)
	client.ReactionChain = append([]kubetesting.Reactor{reactor}, client.ReactionChain...)
	backend, err := kubernetes.NewSharedWithClient(client)
	require.NoError(t, err)

	err = CreateKubeUpdaterIDIfEmpty(t.Context(), backend)
	require.NoError(t, err)
	id, err := LookupKubeUpdaterID(t.Context(), backend)
	require.NoError(t, err)
	require.Equal(t, existingUUID, id)
	require.True(t, reactor.called)
}
