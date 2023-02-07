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

package local

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// testResource for testing the generic service.
type testResource struct {
	types.ResourceHeader
}

func newTestResource(name string) *testResource {
	tr := &testResource{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: name,
			},
			Kind:    "test_resource",
			Version: types.V1,
		},
	}

	tr.CheckAndSetDefaults()
	return tr
}

// marshalResource marshals a generic resource.
func marshalResource(resource *testResource, opts ...services.MarshalOption) ([]byte, error) {
	if err := resource.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return utils.FastMarshal(resource)
}

// unmarshalResource unmarshals a generic resource.
func unmarshalResource(data []byte, opts ...services.MarshalOption) (*testResource, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}
	cfg, err := services.CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}

	var r testResource
	if err := utils.FastUnmarshal(data, &r); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := r.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		r.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		r.SetExpiry(cfg.Expires)
	}
	return &r, nil
}

// TestGenericCRUD tests backend operations with the generic service.
func TestGenericCRUD(t *testing.T) {
	ctx := context.Background()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service := genericResourceService[*testResource]{
		backend:                   backend,
		resourceHumanReadableName: "generic resource",
		limit:                     200,
		backendPrefix:             "generic_prefix",
		unmarshalFunc:             unmarshalResource,
		marshalFunc:               marshalResource,
	}

	// Create a couple test resources.
	r1 := newTestResource("r1")
	r2 := newTestResource("r2")

	// Initially we expect no resources.
	out, nextToken, err := service.listResources(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)

	// Create both resources.
	err = service.createResource(ctx, r1, r1.GetName())
	require.NoError(t, err)
	err = service.createResource(ctx, r2, r2.GetName())
	require.NoError(t, err)

	// Fetch all resources.
	out, nextToken, err = service.listResources(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]*testResource{r1, r2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Fetch a paginated list of resources.
	paginatedOut := make([]*testResource, 0, 2)
	numPages := 0
	for {
		numPages++
		out, nextToken, err = service.listResources(ctx, 1, nextToken)
		require.NoError(t, err)

		paginatedOut = append(paginatedOut, out...)
		if nextToken == "" {
			break
		}
	}

	require.Equal(t, 2, numPages)
	require.Empty(t, cmp.Diff([]*testResource{r1, r2}, paginatedOut,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Fetch a specific service provider.
	r, err := service.getResource(ctx, r2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(r2, r,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Try to fetch a resource that doesn't exist.
	_, err = service.getResource(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err))
	require.Equal(t, `generic resource "/generic_prefix/doesnotexist" doesn't exist`, err.Error())

	// Try to create the same resource.
	err = service.createResource(ctx, r1, r1.GetName())
	require.True(t, trace.IsAlreadyExists(err))

	// Update a resource
	r1.SetStaticLabels(map[string]string{"newlabel": "newvalue"})
	err = service.updateResource(ctx, r1, r1.GetName())
	require.NoError(t, err)
	r, err = service.getResource(ctx, r1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(r1, r,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Delete a resource.
	err = service.deleteResource(ctx, r1.GetName())
	require.NoError(t, err)
	out, nextToken, err = service.listResources(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]*testResource{r2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Try to delete a resource that doesn't exist.
	err = service.deleteResource(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err))
	require.Equal(t, `generic resource "/generic_prefix/doesnotexist" doesn't exist`, err.Error())

	// Delete all resources.
	err = service.deleteAllResources(ctx)
	require.NoError(t, err)
	out, nextToken, err = service.listResources(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)
}
