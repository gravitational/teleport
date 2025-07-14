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

package reconcilers

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources"
)

// newFakeTeleportResource creates a fakeTeleportResource
func newFakeTeleportResource(metadata types.Metadata) *fakeTeleportResource {
	return &fakeTeleportResource{metadata: metadata}
}

type fakeTeleportResource struct {
	metadata types.Metadata
}

func (r *fakeTeleportResource) GetMetadata() types.Metadata {
	return r.metadata
}
func (r *fakeTeleportResource) SetMetadata(metadata types.Metadata) {
	r.metadata = metadata
}

// fakeTeleportResourceClient implements the TeleportResourceClient interface
// for the fakeTeleportResource. It mimics a teleport server by tracking the
// Resource state in its store.
type fakeTeleportResourceClient struct {
	store map[string]types.Metadata
}

// Get implements the TeleportResourceClient interface.
func (f *fakeTeleportResourceClient) Get(_ context.Context, name string) (*fakeTeleportResource, error) {
	metadata, ok := f.store[name]
	if !ok {
		return nil, trace.NotFound("%q not found", name)
	}
	return newFakeTeleportResource(metadata), nil
}

// Create implements the TeleportResourceClient interface.
func (f *fakeTeleportResourceClient) Create(_ context.Context, t *fakeTeleportResource) error {
	name := t.GetMetadata().Name
	_, ok := f.store[name]
	if ok {
		return trace.AlreadyExists("%q already exists", name)
	}
	metadata := t.GetMetadata()
	metadata.SetRevision(uuid.New().String())
	f.store[name] = metadata
	return nil
}

// Update implements the TeleportResourceClient interface.
func (f *fakeTeleportResourceClient) Update(_ context.Context, t *fakeTeleportResource) error {
	name := t.GetMetadata().Name
	existing, ok := f.store[name]
	if !ok {
		return trace.NotFound("%q not found", name)
	}
	if existing.Revision != t.GetMetadata().Revision {
		return trace.CompareFailed("revision mismatch")
	}
	metadata := t.GetMetadata()
	metadata.SetRevision(uuid.New().String())
	f.store[name] = metadata
	return nil
}

// Delete implements the TeleportResourceClient interface.
func (f *fakeTeleportResourceClient) Delete(_ context.Context, name string) error {
	_, ok := f.store[name]
	if !ok {
		return trace.NotFound("%q not found", name)
	}
	delete(f.store, name)
	return nil

}

// resourceExists checks if a Resource is in the store.
// This is use fr testing purposes.
func (f *fakeTeleportResourceClient) resourceExists(name string) bool {
	_, ok := f.store[name]
	return ok
}

// fakeTeleportKubernetesResource implements the TeleportKubernetesResource
// interface for testing purposes.
// Its corresponding TeleportResource is fakeTeleportResource.
type fakeTeleportKubernetesResource struct {
	kclient.Object
	status resources.Status
}

// ToTeleport implements the TeleportKubernetesResource interface.
func (f *fakeTeleportKubernetesResource) ToTeleport() *fakeTeleportResource {
	return &fakeTeleportResource{
		metadata: types.Metadata{
			Name:      f.GetName(),
			Namespace: defaults.Namespace,
			Labels:    map[string]string{},
		},
	}
}

// StatusConditions implements the TeleportKubernetesResource interface.
func (f *fakeTeleportKubernetesResource) StatusConditions() *[]metav1.Condition {
	return &f.status.Conditions
}

type withMetadata interface {
	GetMetadata() types.Metadata
	SetMetadata(metadata types.Metadata)
}

type fakeResourceAdapter[T withMetadata] struct{}

func (f fakeResourceAdapter[T]) GetResourceName(res T) string {
	return res.GetMetadata().Name
}

func (f fakeResourceAdapter[T]) GetResourceRevision(res T) string {
	return res.GetMetadata().Revision
}

func (f fakeResourceAdapter[T]) GetResourceOrigin(res T) string {
	labels := res.GetMetadata().Labels
	if len(labels) == 0 {
		return ""
	}
	if origin, ok := labels[types.OriginLabel]; ok {
		return origin
	}
	return ""
}

func (f fakeResourceAdapter[T]) SetResourceRevision(res T, rev string) {
	metadata := res.GetMetadata()
	metadata.SetRevision(rev)
	res.SetMetadata(metadata)
}

func (f fakeResourceAdapter[T]) SetResourceLabels(res T, labels map[string]string) {
	metadata := res.GetMetadata()
	metadata.Labels = labels
	res.SetMetadata(metadata)
}

func TestTeleportResourceReconciler_Delete(t *testing.T) {
	ctx := context.Background()
	resourceName := "test"
	kubeResource := &unstructured.Unstructured{}
	kubeResource.SetName(resourceName)

	tests := []struct {
		name           string
		store          map[string]types.Metadata
		assertErr      require.ErrorAssertionFunc
		resourceExists bool
	}{
		{
			name:  "delete non-existing Resource",
			store: map[string]types.Metadata{},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err))
			},
			resourceExists: false,
		},
		{
			name: "delete existing Resource",
			store: map[string]types.Metadata{
				resourceName: {
					Name:   resourceName,
					Labels: map[string]string{types.OriginLabel: types.OriginKubernetes},
				},
			},
			assertErr:      require.NoError,
			resourceExists: false,
		},
		{
			name: "delete existing but not owned Resource",
			store: map[string]types.Metadata{
				resourceName: {
					Name:   resourceName,
					Labels: map[string]string{},
				},
			},
			assertErr:      require.NoError,
			resourceExists: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resourceClient := &fakeTeleportResourceClient{tt.store}
			reconciler := resourceReconciler[*fakeTeleportResource, *fakeTeleportKubernetesResource]{
				resourceClient: resourceClient,
				adapter:        fakeResourceAdapter[*fakeTeleportResource]{},
			}
			tt.assertErr(t, reconciler.Delete(ctx, kubeResource))
			require.Equal(t, tt.resourceExists, resourceClient.resourceExists(resourceName))
		})
	}
}

func TestCheckOwnership(t *testing.T) {
	emptyStore := map[string]types.Metadata{}
	rc := &fakeTeleportResourceClient{emptyStore}
	reconciler := resourceReconciler[*fakeTeleportResource, *fakeTeleportKubernetesResource]{
		resourceClient: rc,
		adapter:        fakeResourceAdapter[*fakeTeleportResource]{},
	}
	tests := []struct {
		name                    string
		existingResource        *fakeTeleportResource
		expectedConditionStatus metav1.ConditionStatus
		expectedConditionReason string
		isOwned                 bool
	}{
		{
			name: "existing owned Resource",
			existingResource: &fakeTeleportResource{
				metadata: types.Metadata{
					Name:   "existing owned user",
					Labels: map[string]string{types.OriginLabel: types.OriginKubernetes},
				},
			},
			expectedConditionStatus: metav1.ConditionTrue,
			expectedConditionReason: ConditionReasonOriginLabelMatching,
			isOwned:                 true,
		},
		{
			name: "existing unowned Resource (no label)",
			existingResource: &fakeTeleportResource{
				metadata: types.Metadata{
					Name: "existing unowned user without label",
				},
			},
			expectedConditionStatus: metav1.ConditionFalse,
			expectedConditionReason: ConditionReasonOriginLabelNotMatching,
			isOwned:                 false,
		},
		{
			name: "existing unowned Resource (bad origin)",
			existingResource: &fakeTeleportResource{
				metadata: types.Metadata{
					Name:   "existing owned user without origin label",
					Labels: map[string]string{types.OriginLabel: types.OriginConfigFile},
				},
			},
			expectedConditionStatus: metav1.ConditionFalse,
			expectedConditionReason: ConditionReasonOriginLabelNotMatching,
			isOwned:                 false,
		},
		{
			name: "existing unowned Resource (no origin)",
			existingResource: &fakeTeleportResource{
				metadata: types.Metadata{
					Name:   "existing owned user without origin label",
					Labels: map[string]string{"foo": "bar"},
				},
			},
			expectedConditionStatus: metav1.ConditionFalse,
			expectedConditionReason: ConditionReasonOriginLabelNotMatching,
			isOwned:                 false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			condition, isOwned := reconciler.checkOwnership(tc.existingResource)

			require.Equal(t, tc.isOwned, isOwned)
			require.Equal(t, ConditionTypeTeleportResourceOwned, condition.Type)
			require.Equal(t, tc.expectedConditionStatus, condition.Status)
			require.Equal(t, tc.expectedConditionReason, condition.Reason)
		})
	}
}
