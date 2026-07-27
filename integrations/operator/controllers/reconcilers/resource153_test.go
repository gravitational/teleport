/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package reconcilers

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	accessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/lib/scopes/access"
)

type fakeScopedRoleClient struct {
	store map[ResourceKey]*accessv1.ScopedRole
}

func (f *fakeScopedRoleClient) Get(_ context.Context, key ResourceKey) (*accessv1.ScopedRole, error) {
	role, ok := f.store[key]
	if !ok {
		return nil, trace.NotFound("%q not found", key.String())
	}
	return role, nil
}

func (f *fakeScopedRoleClient) Create(_ context.Context, role *accessv1.ScopedRole) error {
	key := ResourceKey{Name: role.GetMetadata().GetName(), Scope: role.GetScope()}
	if _, ok := f.store[key]; ok {
		return trace.AlreadyExists("%q already exists", key.String())
	}
	role.GetMetadata().SetRevision(uuid.New().String())
	f.store[key] = role
	return nil
}

func (f *fakeScopedRoleClient) Update(_ context.Context, role *accessv1.ScopedRole) error {
	key := ResourceKey{Name: role.GetMetadata().GetName(), Scope: role.GetScope()}
	existing, ok := f.store[key]
	if !ok {
		return trace.NotFound("%q not found", key.String())
	}
	if existing.GetMetadata().GetRevision() != role.GetMetadata().GetRevision() {
		return trace.CompareFailed("revision mismatch")
	}
	role.GetMetadata().SetRevision(uuid.New().String())
	f.store[key] = role
	return nil
}

func (f *fakeScopedRoleClient) Delete(_ context.Context, key ResourceKey) error {
	if _, ok := f.store[key]; !ok {
		return trace.NotFound("%q not found", key.String())
	}
	delete(f.store, key)
	return nil
}

func TestScopedResource153Reconciler(t *testing.T) {
	t.Parallel()
	const (
		name      = "test-role"
		namespace = "default"
		scopeA    = "/team/a"
		scopeB    = "/team/b"
	)

	cr := &resourcesv1.TeleportScopedRoleV1{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resourcesv1.GroupVersion.String(),
			Kind:       "TeleportScopedRoleV1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Scope: scopeA,
		Spec:  &resourcesv1.TeleportScopedRoleV1Spec{},
	}

	scopedRole := accessv1.ScopedRole_builder{
		Kind:    access.KindScopedRole,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name:   name,
			Labels: map[string]string{types.OriginLabel: types.OriginKubernetes},
		}.Build(),
		Scope: scopeB,
		Spec:  &accessv1.ScopedRoleSpec{},
	}.Build()

	resourceClient := &fakeScopedRoleClient{store: map[ResourceKey]*accessv1.ScopedRole{{Name: name, Scope: scopeB}: scopedRole}}
	kubeClient := fake.NewClientBuilder().
		WithScheme(controllers.Scheme).
		WithStatusSubresource(&resourcesv1.TeleportScopedRoleV1{}).
		WithObjects(cr).
		Build()
	reconciler, err := NewTeleportScopedResource153Reconciler[*accessv1.ScopedRole, *resourcesv1.TeleportScopedRoleV1](
		kubeClient,
		resourceClient,
		Config{Scoped: true},
		OperatorMetadata{
			Namespace: "ns",
			ID:        "id",
			TokenName: "token",
			Scope:     scopeA,
			Owner:     "test@example.com",
		},
	)
	require.NoError(t, err)

	req := ctrl.Request{NamespacedName: k8stypes.NamespacedName{Name: name, Namespace: namespace}}

	// First reconciliation adds the deletion finalizer and exits.
	_, err = reconciler.Reconcile(t.Context(), req)
	require.NoError(t, err)

	// Second reconciliation performs the Teleport upsert.
	_, err = reconciler.Reconcile(t.Context(), req)
	require.NoError(t, err)

	roleA := resourceClient.store[ResourceKey{Name: name, Scope: scopeA}]
	require.NotNil(t, roleA)
	require.Equal(t, types.OriginKubernetes, roleA.GetMetadata().GetLabels()[types.OriginLabel])
	require.Equal(t, name, roleA.GetMetadata().GetLabels()[customResourceNameLabel])
	require.NotNil(t, resourceClient.store[ResourceKey{Name: name, Scope: scopeB}])

	err = kubeClient.Delete(t.Context(), cr)
	require.NoError(t, err)

	_, err = reconciler.Reconcile(t.Context(), req)
	require.NoError(t, err)

	require.Nil(t, resourceClient.store[ResourceKey{Name: name, Scope: scopeA}])
	require.NotNil(t, resourceClient.store[ResourceKey{Name: name, Scope: scopeB}])
}

type fakeResource153 struct {
	ScopedResource153
	metadata *headerv1.Metadata
	scope    string
}

func (r *fakeResource153) GetMetadata() *headerv1.Metadata {
	return r.metadata
}

func (r *fakeResource153) GetScope() string {
	return r.scope
}

func TestResource153Adapter_CheckOwnership(t *testing.T) {
	const (
		testID        = "test-id"
		conflictingID = "conflicting-id"
	)
	tests := []struct {
		name           string
		resource       *fakeResource153
		expected       bool
		expectedReason string
	}{
		{
			name:           "no labels",
			resource:       &fakeResource153{metadata: headerv1.Metadata_builder{Labels: map[string]string{}}.Build()},
			expected:       false,
			expectedReason: ownershipIssueMissingOriginLabel,
		},
		{
			name:           "labels but no origin",
			resource:       &fakeResource153{metadata: headerv1.Metadata_builder{Labels: map[string]string{"foo": "bar"}}.Build()},
			expected:       false,
			expectedReason: ownershipIssueMissingOriginLabel,
		},
		{
			name:     "origin label match",
			resource: &fakeResource153{metadata: headerv1.Metadata_builder{Labels: map[string]string{types.OriginLabel: types.OriginKubernetes}}.Build()},
			expected: true,
		},
	}

	adapter := Resource153Adapter[*fakeResource153]{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owned, reason := adapter.CheckOwnership(tt.resource, OperatorMetadata{})
			require.Equal(t, tt.expected, owned)
			require.Equal(t, tt.expectedReason, reason)
		})
	}
}

func TestScopedResource153Adapter_CheckOwnership(t *testing.T) {
	const (
		testID        = "test-id"
		conflictingID = "conflicting-id"
		testScope     = "/team/a"
	)
	tests := []struct {
		name           string
		resource       *fakeResource153
		expected       bool
		expectedReason string
	}{
		{
			name:           "unscoped - no labels",
			resource:       &fakeResource153{metadata: headerv1.Metadata_builder{Labels: map[string]string{}}.Build()},
			expected:       false,
			expectedReason: ownershipIssueMissingOriginLabel,
		},
		{
			name:           "uncscope - labels but no origin",
			resource:       &fakeResource153{metadata: headerv1.Metadata_builder{Labels: map[string]string{"foo": "bar"}}.Build()},
			expected:       false,
			expectedReason: ownershipIssueMissingOriginLabel,
		},
		{
			name:           "unscoped - origin label mismatch",
			resource:       &fakeResource153{metadata: headerv1.Metadata_builder{Labels: map[string]string{types.OriginLabel: types.OriginDefaults}}.Build()},
			expected:       false,
			expectedReason: fmt.Sprintf(ownershipIssueMismatchOriginLabel, types.OriginDefaults),
		},
		{
			name:     "unscoped - origin label match but no id",
			resource: &fakeResource153{metadata: headerv1.Metadata_builder{Labels: map[string]string{types.OriginLabel: types.OriginKubernetes}}.Build()},
			expected: true,
		},
		{
			name:     "unscoped - origin label match but mismatch id",
			resource: &fakeResource153{metadata: headerv1.Metadata_builder{Labels: map[string]string{types.OriginLabel: types.OriginKubernetes, OperatorIDLabel: conflictingID}}.Build()},
			expected: true,
		},
		{
			name:     "unscoped - origin label match and matching id",
			resource: &fakeResource153{metadata: headerv1.Metadata_builder{Labels: map[string]string{types.OriginLabel: types.OriginKubernetes, OperatorIDLabel: testID}}.Build()},
			expected: true,
		},
		{
			name: "scoped - no labels",
			resource: &fakeResource153{
				metadata: headerv1.Metadata_builder{Labels: map[string]string{}}.Build(),
				scope:    testScope,
			},
			expected:       false,
			expectedReason: ownershipIssueMissingOriginLabel,
		},
		{
			name: "scoped - labels but no origin",
			resource: &fakeResource153{
				metadata: headerv1.Metadata_builder{Labels: map[string]string{"foo": "bar"}}.Build(),
				scope:    testScope,
			},
			expected:       false,
			expectedReason: ownershipIssueMissingOriginLabel,
		},
		{
			name: "scoped - origin label mismatch",
			resource: &fakeResource153{
				metadata: headerv1.Metadata_builder{Labels: map[string]string{types.OriginLabel: types.OriginDefaults}}.Build(),
				scope:    testScope,
			},
			expected:       false,
			expectedReason: fmt.Sprintf(ownershipIssueMismatchOriginLabel, types.OriginDefaults),
		},
		{
			name: "scoped - origin label match but no id",
			resource: &fakeResource153{
				metadata: headerv1.Metadata_builder{Labels: map[string]string{types.OriginLabel: types.OriginKubernetes}}.Build(),
				scope:    testScope,
			},
			expected:       false,
			expectedReason: ownershipIssueMissingOperatorID,
		},
		{
			name: "scoped - origin label match but mismatch id",
			resource: &fakeResource153{
				metadata: headerv1.Metadata_builder{Labels: map[string]string{types.OriginLabel: types.OriginKubernetes, OperatorIDLabel: conflictingID}}.Build(),
				scope:    testScope,
			},
			expected:       false,
			expectedReason: fmt.Sprintf(ownershipIssueMismatchOperatorID, conflictingID, testID),
		},
		{
			name: "scoped - origin label match and matching id",
			resource: &fakeResource153{
				metadata: headerv1.Metadata_builder{Labels: map[string]string{types.OriginLabel: types.OriginKubernetes, OperatorIDLabel: testID}}.Build(),
				scope:    testScope,
			},
			expected: true,
		},
	}

	adapter := ScopedResource153Adapter[*fakeResource153]{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owned, reason := adapter.CheckOwnership(tt.resource, OperatorMetadata{ID: testID})
			require.Equal(t, tt.expected, owned)
			require.Equal(t, tt.expectedReason, reason)
		})
	}
}

func TestResource153Adapter_SetLabels(t *testing.T) {
	adapter := Resource153Adapter[*fakeResource153]{}
	resource := &fakeResource153{metadata: headerv1.Metadata_builder{Labels: map[string]string{}}.Build()}
	adapter.SetResourceLabels(resource, map[string]string{
		"foo": "bar",
	}, OperatorMetadata{
		Namespace: "unused",
		ID:        "unused",
		TokenName: "unused",
		Scope:     "unused",
		Owner:     "unused",
	}, customResourceMetadata{
		name:      "unused",
		namespace: "unused",
		gvk:       "unused",
	})
	require.Equal(t, map[string]string{
		"foo":             "bar",
		types.OriginLabel: types.OriginKubernetes,
	}, resource.metadata.GetLabels())
}

func TestScopedResource153Adapter_SetLabels(t *testing.T) {
	const testScope = "/team/a"
	initialLabels := map[string]string{
		"foo": "bar",
	}
	kubeLabels := map[string]string{
		"kube": "label",
	}

	operatorMetadata := OperatorMetadata{
		Namespace: "namespace",
		ID:        "id",
		TokenName: "token",
		Scope:     "scope",
		Owner:     "owner",
	}
	resourceMetadata := customResourceMetadata{
		namespace: "cr-namespace",
		name:      "name",
		gvk:       "gvk",
	}

	tests := []struct {
		name           string
		resource       *fakeResource153
		expectedLabels map[string]string
	}{
		{
			name: "unscoped",
			resource: &fakeResource153{
				metadata: headerv1.Metadata_builder{Labels: initialLabels}.Build(),
			},
			expectedLabels: map[string]string{
				"kube":            "label",
				types.OriginLabel: types.OriginKubernetes,
			},
		},
		{
			name: "scoped",
			resource: &fakeResource153{
				metadata: headerv1.Metadata_builder{Labels: initialLabels}.Build(),
				scope:    testScope,
			},
			expectedLabels: map[string]string{
				"kube":                       "label",
				types.OriginLabel:            types.OriginKubernetes,
				OperatorIDLabel:              operatorMetadata.ID,
				operatorNamespaceLabel:       operatorMetadata.Namespace,
				operatorOwnerLabel:           operatorMetadata.Owner,
				operatorTokenNameLabel:       operatorMetadata.TokenName,
				customResourceNameLabel:      resourceMetadata.name,
				customResourceNamespaceLabel: resourceMetadata.namespace,
				customResourceGVKLabel:       resourceMetadata.gvk,
			},
		},
	}

	adapter := ScopedResource153Adapter[*fakeResource153]{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter.SetResourceLabels(tt.resource, kubeLabels, operatorMetadata, resourceMetadata)
			require.Equal(t, tt.expectedLabels, tt.resource.metadata.GetLabels())
		})
	}
}
