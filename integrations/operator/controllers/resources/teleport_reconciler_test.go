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

package resources

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

// NewFakeTeleportResource creates a FakeTeleportResource
func NewFakeTeleportResource(metadata types.Metadata) *FakeTeleportResource {
	return &FakeTeleportResource{metadata: metadata}
}

// FakeTeleportResource implements the TeleportResource interface for testing purposes.
// Its corresponding TeleportKubernetesResource is FakeTeleportKubernetesResource.
type FakeTeleportResource struct {
	metadata types.Metadata
}

// GetName implements TeleportResource interface.
func (f *FakeTeleportResource) GetName() string {
	return f.metadata.Name
}

// SetOrigin implements TeleportResource interface.
func (f *FakeTeleportResource) SetOrigin(origin string) {
	f.metadata.Labels[types.OriginLabel] = origin
}

// GetMetadata implements TeleportResource interface.
func (f *FakeTeleportResource) GetMetadata() types.Metadata {
	return f.metadata
}

// GetRevision implements TeleportResource interface.
func (f *FakeTeleportResource) GetRevision() string {
	return f.metadata.Revision
}

// SetRevision implements TeleportResource interface.
func (f *FakeTeleportResource) SetRevision(revision string) {
	f.metadata.Revision = revision
}

// FakeTeleportResourceClient implements the TeleportResourceClient interface
// for the FakeTeleportResource. It mimics a teleport server by tracking the
// resource state in its store.
type FakeTeleportResourceClient struct {
	store map[string]types.Metadata
}

// Get implements the TeleportResourceClient interface.
func (f *FakeTeleportResourceClient) Get(_ context.Context, name string) (*FakeTeleportResource, error) {
	metadata, ok := f.store[name]
	if !ok {
		return nil, trace.NotFound("%q not found", name)
	}
	return NewFakeTeleportResource(metadata), nil
}

// Create implements the TeleportResourceClient interface.
func (f *FakeTeleportResourceClient) Create(_ context.Context, t *FakeTeleportResource) error {
	_, ok := f.store[t.GetName()]
	if ok {
		return trace.AlreadyExists("%q already exists", t.GetName())
	}
	metadata := t.GetMetadata()
	metadata.SetRevision(uuid.New().String())
	f.store[t.GetName()] = metadata
	return nil
}

// Update implements the TeleportResourceClient interface.
func (f *FakeTeleportResourceClient) Update(_ context.Context, t *FakeTeleportResource) error {
	existing, ok := f.store[t.GetName()]
	if !ok {
		return trace.NotFound("%q not found", t.GetName())
	}
	if existing.Revision != t.GetRevision() {
		return trace.CompareFailed("revision mismatch")
	}
	metadata := t.GetMetadata()
	metadata.SetRevision(uuid.New().String())
	f.store[t.GetName()] = metadata
	return nil
}

// Delete implements the TeleportResourceClient interface.
func (f *FakeTeleportResourceClient) Delete(_ context.Context, name string) error {
	_, ok := f.store[name]
	if !ok {
		return trace.NotFound("%q not found", name)
	}
	delete(f.store, name)
	return nil

}

// resourceExists checks if a resource is in the store.
// This is use fr testing purposes.
func (f *FakeTeleportResourceClient) resourceExists(name string) bool {
	_, ok := f.store[name]
	return ok
}

// FakeTeleportKubernetesResource implements the TeleportKubernetesResource
// interface for testing purposes.
// Its corresponding TeleportResource is FakeTeleportResource.
type FakeTeleportKubernetesResource struct {
	kclient.Object
	status resources.Status
}

// ToTeleport implements the TeleportKubernetesResource interface.
func (f *FakeTeleportKubernetesResource) ToTeleport() *FakeTeleportResource {
	return &FakeTeleportResource{
		metadata: types.Metadata{
			Name:      f.GetName(),
			Namespace: defaults.Namespace,
			Labels:    map[string]string{},
		},
	}
}

// StatusConditions implements the TeleportKubernetesResource interface.
func (f *FakeTeleportKubernetesResource) StatusConditions() *[]metav1.Condition {
	return &f.status.Conditions
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
			name:  "delete non-existing resource",
			store: map[string]types.Metadata{},
			assertErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsNotFound(err))
			},
			resourceExists: false,
		},
		{
			name: "delete existing resource",
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
			name: "delete existing but not owned resource",
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
			resourceClient := &FakeTeleportResourceClient{tt.store}
			reconciler := TeleportResourceReconciler[*FakeTeleportResource, *FakeTeleportKubernetesResource]{
				resourceClient: resourceClient,
			}
			tt.assertErr(t, reconciler.Delete(ctx, kubeResource))
			require.Equal(t, tt.resourceExists, resourceClient.resourceExists(resourceName))
		})
	}
}
