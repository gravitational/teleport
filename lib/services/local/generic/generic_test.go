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

package generic

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
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

	memBackend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service, err := NewService(&ServiceConfig[*testResource]{
		Backend:       memBackend,
		ResourceKind:  "generic resource",
		PageLimit:     200,
		BackendPrefix: "generic_prefix",
		UnmarshalFunc: unmarshalResource,
		MarshalFunc:   marshalResource,
	})
	require.NoError(t, err)

	// Create a couple test resources.
	r1 := newTestResource("r1")
	r2 := newTestResource("r2")

	// Initially we expect no resources.
	out, nextToken, err := service.ListResources(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)

	// Create both resources.
	err = service.CreateResource(ctx, r1)
	require.NoError(t, err)
	err = service.CreateResource(ctx, r2)
	require.NoError(t, err)

	// Fetch all resources using paging default.
	out, nextToken, err = service.ListResources(ctx, 0, "")
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
		out, nextToken, err = service.ListResources(ctx, 1, nextToken)
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

	// Fetch a list of all resources
	allResources, err := service.GetResources(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(paginatedOut, allResources,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Fetch a specific service provider.
	r, err := service.GetResource(ctx, r2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(r2, r,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Try to fetch a resource that doesn't exist.
	_, err = service.GetResource(ctx, "doesnotexist")
	require.ErrorIs(t, err, trace.NotFound(`generic resource "doesnotexist" doesn't exist`))

	// Try to create the same resource.
	err = service.CreateResource(ctx, r1)
	require.ErrorIs(t, err, trace.AlreadyExists(`generic resource "r1" already exists`))

	// Update a resource.
	r1.SetStaticLabels(map[string]string{"newlabel": "newvalue"})
	err = service.UpdateResource(ctx, r1)
	require.NoError(t, err)
	r, err = service.GetResource(ctx, r1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(r1, r,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Update a resource that doesn't exist.
	doesNotExist := newTestResource("doesnotexist")
	err = service.UpdateResource(ctx, doesNotExist)
	require.ErrorIs(t, err, trace.NotFound(`generic resource "doesnotexist" doesn't exist`))

	// Delete a resource.
	err = service.DeleteResource(ctx, r1.GetName())
	require.NoError(t, err)
	out, nextToken, err = service.ListResources(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]*testResource{r2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Upsert a resource (create).
	err = service.UpsertResource(ctx, r1)
	require.NoError(t, err)
	out, nextToken, err = service.ListResources(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]*testResource{r1, r2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Upsert a resource (update).
	r1.SetStaticLabels(map[string]string{"newerlabel": "newervalue"})
	err = service.UpsertResource(ctx, r1)
	require.NoError(t, err)
	out, nextToken, err = service.ListResources(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]*testResource{r1, r2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Update and swap a value
	require.NoError(t, service.UpdateAndSwapResource(ctx, r2.GetName(), func(tr *testResource) error {
		tr.SetStaticLabels(map[string]string{"updateandswap": "labelvalue"})
		return nil
	}))
	r2.SetStaticLabels(map[string]string{"updateandswap": "labelvalue"})
	out, nextToken, err = service.ListResources(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]*testResource{r1, r2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Try to delete a resource that doesn't exist.
	err = service.DeleteResource(ctx, "doesnotexist")
	require.ErrorIs(t, err, trace.NotFound(`generic resource "doesnotexist" doesn't exist`))

	// Test running while locked.
	err = service.RunWhileLocked(ctx, "test-lock", time.Second*5, func(ctx context.Context, backend backend.Backend) error {
		item, err := backend.Get(ctx, service.MakeKey(r1.GetName()))
		require.NoError(t, err)

		r, err = unmarshalResource(item.Value)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(r1, r,
			cmpopts.IgnoreFields(types.Metadata{}, "ID"),
		))

		return nil
	})
	require.NoError(t, err)

	// Delete all resources.
	err = service.DeleteAllResources(ctx)
	require.NoError(t, err)
	out, nextToken, err = service.ListResources(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)
}
