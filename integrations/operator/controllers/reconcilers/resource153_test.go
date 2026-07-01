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
	require.NotNil(t, resourceClient.store[ResourceKey{Name: name, Scope: scopeB}])

	err = kubeClient.Delete(t.Context(), cr)
	require.NoError(t, err)

	_, err = reconciler.Reconcile(t.Context(), req)
	require.NoError(t, err)

	require.Nil(t, resourceClient.store[ResourceKey{Name: name, Scope: scopeA}])
	require.NotNil(t, resourceClient.store[ResourceKey{Name: name, Scope: scopeB}])
}
