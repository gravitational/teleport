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

package generic

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
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
	Spec testResourceSpec
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

type testResourceSpec struct {
	PropA string
}

func newTestResourceWithSpec(name string, specPropA string) *testResource {
	tr := &testResource{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: name,
			},
			Kind:    "test_resource",
			Version: types.V1,
		},
		Spec: testResourceSpec{
			PropA: specPropA,
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
		return nil, trace.BadParameter("%s", err)
	}
	if err := r.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.Revision != "" {
		r.SetRevision(cfg.Revision)
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
		BackendPrefix: backend.NewKey("generic_prefix"),
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
	r1, err = service.CreateResource(ctx, r1)
	require.NoError(t, err)
	r2, err = service.CreateResource(ctx, r2)
	require.NoError(t, err)

	require.NotEmpty(t, r1.GetRevision())
	require.NotEmpty(t, r2.GetRevision())
	require.NotEqual(t, r1.GetRevision(), r2.GetRevision())

	// Fetch all resources using paging default.
	out, nextToken, err = service.ListResources(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]*testResource{r1, r2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
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
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Count all resources.
	count, err := service.CountResources(ctx)
	require.NoError(t, err)
	require.Equal(t, uint(2), count)

	// Fetch a list of all resources
	allResources, err := service.GetResources(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(paginatedOut, allResources,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Fetch a specific service provider.
	r, err := service.GetResource(ctx, r2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(r2, r,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Try to fetch a resource that doesn't exist.
	_, err = service.GetResource(ctx, "doesnotexist")
	require.ErrorIs(t, err, trace.NotFound(`generic resource "doesnotexist" doesn't exist`))

	// Try to create the same resource.
	_, err = service.CreateResource(ctx, r1)
	require.ErrorIs(t, err, trace.AlreadyExists(`generic resource "r1" already exists`))

	// Update a resource.
	r1.SetStaticLabels(map[string]string{"newlabel": "newvalue"})
	r1, err = service.UpdateResource(ctx, r1)
	require.NoError(t, err)
	r, err = service.GetResource(ctx, r1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(r1, r,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Update a resource that doesn't exist.
	doesNotExist := newTestResource("doesnotexist")
	_, err = service.UpdateResource(ctx, doesNotExist)
	require.ErrorIs(t, err, trace.NotFound(`generic resource "doesnotexist" doesn't exist`))

	// Delete a resource.
	err = service.DeleteResource(ctx, r1.GetName())
	require.NoError(t, err)
	out, nextToken, err = service.ListResources(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]*testResource{r2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Make sure count is updated.
	count, err = service.CountResources(ctx)
	require.NoError(t, err)
	require.Equal(t, uint(1), count)

	// Upsert a resource (create).
	r1, err = service.UpsertResource(ctx, r1)
	require.NoError(t, err)
	out, nextToken, err = service.ListResources(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]*testResource{r1, r2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Upsert a resource (update).
	r1.SetStaticLabels(map[string]string{"newerlabel": "newervalue"})
	r1, err = service.UpsertResource(ctx, r1)
	require.NoError(t, err)
	out, nextToken, err = service.ListResources(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]*testResource{r1, r2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Update and swap a value
	r2, err = service.UpdateAndSwapResource(ctx, r2.GetName(), func(tr *testResource) error {
		tr.SetStaticLabels(map[string]string{"updateandswap": "labelvalue"})
		return nil
	})
	require.NoError(t, err)
	r2.SetStaticLabels(map[string]string{"updateandswap": "labelvalue"})
	out, nextToken, err = service.ListResources(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]*testResource{r1, r2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Try to delete a resource that doesn't exist.
	err = service.DeleteResource(ctx, "doesnotexist")
	require.ErrorIs(t, err, trace.NotFound(`generic resource "doesnotexist" doesn't exist`))

	// Test running while locked.
	err = service.RunWhileLocked(ctx, []string{"test-lock"}, time.Second*5, func(ctx context.Context, b backend.Backend) error {
		item, err := b.Get(ctx, service.MakeKey(backend.NewKey(r1.GetName())))
		require.NoError(t, err)

		r, err = unmarshalResource(item.Value, services.WithRevision(item.Revision))
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(r1, r,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
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

	// Make sure count is updated.
	count, err = service.CountResources(ctx)
	require.NoError(t, err)
	require.Equal(t, uint(0), count)
}

func TestGenericListResourcesReturnNextResource(t *testing.T) {
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
		BackendPrefix: backend.NewKey("generic_prefix"),
		UnmarshalFunc: unmarshalResource,
		MarshalFunc:   marshalResource,
	})
	require.NoError(t, err)

	// Create a couple test resources.
	r1 := newTestResource("r1")
	r2 := newTestResource("r2")

	_, err = service.WithPrefix("a-unique-prefix").UpsertResource(ctx, r1)
	require.NoError(t, err)
	_, err = service.WithPrefix("another-unique-prefix").UpsertResource(ctx, r2)
	require.NoError(t, err)

	page, next, err := service.ListResourcesReturnNextResource(ctx, 1, "")
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*testResource{r1}, page,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))
	require.NotNil(t, next)

	page, next, err = service.ListResourcesReturnNextResource(ctx, 1, "another-unique-prefix"+string(backend.Separator)+backend.GetPaginationKey(*next))
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*testResource{r2}, page,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))
	require.Nil(t, next)
}

func TestGenericListResourcesWithMultiplePrefixes(t *testing.T) {
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
		BackendPrefix: backend.NewKey("generic_prefix"),
		UnmarshalFunc: unmarshalResource,
		MarshalFunc:   marshalResource,
	})
	require.NoError(t, err)

	// Create a couple test resources.
	r1 := newTestResource("r1")
	r2 := newTestResource("r2")

	_, err = service.WithPrefix("a-unique-prefix").UpsertResource(ctx, r1)
	require.NoError(t, err)
	_, err = service.WithPrefix("another-unique-prefix").UpsertResource(ctx, r2)
	require.NoError(t, err)

	page, next, err := service.ListResources(ctx, 1, "")
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*testResource{r1}, page,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))
	require.Equal(t, next, "another-unique-prefix"+string(backend.Separator)+r2.GetName())

	page, next, err = service.ListResources(ctx, 1, next)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*testResource{r2}, page,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))
	require.Empty(t, next)

	_, err = service.WithPrefix("a-unique-prefix").UpsertResource(ctx, r2)
	require.NoError(t, err)
	page, next, err = service.WithPrefix("a-unique-prefix").ListResources(ctx, 1, "")
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*testResource{r1}, page,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))
	require.Equal(t, next, r2.GetName())

	page, next, err = service.WithPrefix("a-unique-prefix").ListResources(ctx, 1, next)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*testResource{r2}, page,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))
	require.Empty(t, next)
}

func TestGenericListResourcesWithFilter(t *testing.T) {
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
		BackendPrefix: backend.NewKey("generic_prefix"),
		UnmarshalFunc: unmarshalResource,
		MarshalFunc:   marshalResource,
	})
	require.NoError(t, err)

	// Create a couple test resources.
	r1 := newTestResource("r1")
	r2 := newTestResource("r2")

	_, err = service.WithPrefix("a-unique-prefix").UpsertResource(ctx, r1)
	require.NoError(t, err)
	_, err = service.WithPrefix("another-unique-prefix").UpsertResource(ctx, r2)
	require.NoError(t, err)

	page, nextKey, err := service.ListResourcesWithFilter(ctx, 1, "", func(r *testResource) bool {
		return r.Metadata.Name == "r1"
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*testResource{r1}, page,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))
	require.Equal(t, "", nextKey)

	page, nextKey, err = service.ListResourcesWithFilter(ctx, 1, "", func(r *testResource) bool {
		return r.Metadata.Name == "r2"
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*testResource{r2}, page,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))
	require.Equal(t, "", nextKey)
}

func TestGenericListResourcesWithFilterForScale(t *testing.T) {
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
		BackendPrefix: backend.NewKey("my-prefix"),
		UnmarshalFunc: unmarshalResource,
		MarshalFunc:   marshalResource,
	})
	require.NoError(t, err)

	totalResourcesPerProp := 100
	totalProps := 100
	var totalResources []*testResource
	for i := 0; i < totalResourcesPerProp; i++ {
		for j := 0; j < totalProps; j++ {
			r := newTestResourceWithSpec(uuid.NewString(), strconv.Itoa(j))
			totalResources = append(totalResources, r)
		}
	}

	r := rand.New(rand.NewPCG(uint64(82), uint64(123)))
	r.Shuffle(len(totalResources), func(i, j int) {
		totalResources[i], totalResources[j] = totalResources[j], totalResources[i]
	})

	for _, r := range totalResources {
		_, err = service.UpsertResource(ctx, r)
		require.NoError(t, err)
	}

	pageSizes := []int{1, 2, 3, 5, 7, 100_000}
	for _, pageSize := range pageSizes {
		testingProp := strconv.Itoa(r.IntN(totalProps))
		t.Run(fmt.Sprintf("pageSize=%d,prop=%s", pageSize, testingProp), func(t *testing.T) {
			var startingKey string
			var foundResourcesPropAEquals []*testResource
			for {
				var totalMatchedElements atomic.Uint64
				page, nextKey, err := service.ListResourcesWithFilter(ctx, pageSize, startingKey, func(r *testResource) bool {
					if r.Spec.PropA == testingProp {
						totalMatchedElements.Add(1)
						return true
					}

					return false
				})
				require.NoError(t, err)
				// At most, there's an extra comparison to ensure the next key is valid and there are actually more elements.
				require.LessOrEqual(t, totalMatchedElements.Load(), uint64(pageSize+1))
				foundResourcesPropAEquals = append(foundResourcesPropAEquals, page...)

				// A page must never contain more items than the page size limit.
				require.LessOrEqual(t, len(page), pageSize)
				if nextKey == "" {
					// A page can contain 0 elements but only when there's no matching elements.
					// This is never true for our current test setup.
					require.NotEmpty(t, page)
					break
				}
				startingKey = nextKey
			}
			require.Len(t, foundResourcesPropAEquals, totalResourcesPerProp)
		})
	}
}

func TestGenericValidation(t *testing.T) {
	ctx := context.Background()

	memBackend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	validationErr := trace.BadParameter("invalid test resource")
	service, err := NewService(&ServiceConfig[*testResource]{
		Backend:       memBackend,
		ResourceKind:  "generic resource",
		PageLimit:     200,
		BackendPrefix: backend.NewKey("generic_prefix"),
		UnmarshalFunc: unmarshalResource,
		MarshalFunc:   marshalResource,
		ValidateFunc:  func(tr *testResource) error { return validationErr },
	})
	require.NoError(t, err)

	r1 := newTestResource("r1")

	_, err = service.CreateResource(ctx, r1)
	require.ErrorIs(t, err, validationErr)

	_, err = service.UpdateResource(ctx, r1)
	require.ErrorIs(t, err, validationErr)

	_, err = service.UpsertResource(ctx, r1)
	require.ErrorIs(t, err, validationErr)
}

func TestGenericKeyOverride(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	memBackend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	defer memBackend.Close()

	service, err := NewService(&ServiceConfig[*testResource]{
		Backend:       memBackend,
		ResourceKind:  "generic resource",
		PageLimit:     200,
		BackendPrefix: backend.NewKey("generic_prefix"),
		UnmarshalFunc: unmarshalResource,
		MarshalFunc:   marshalResource,
		NameKeyFunc:   func(string) string { return "llama" },
	})
	require.NoError(t, err)

	r1 := newTestResource("r1")

	// Create the test resource
	created, err := service.CreateResource(ctx, r1)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(r1, created, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	// Validate that the resource is stored under the custom key
	item, err := memBackend.Get(ctx, backend.NewKey("generic_prefix", "llama"))
	require.NoError(t, err)
	require.NotNil(t, item)

	// Validate that the default key does not exist
	item, err = memBackend.Get(ctx, backend.NewKey("generic_prefix", r1.GetName()))
	require.Error(t, err)
	require.Nil(t, item)

	// Update the test resource
	updated, err := service.UpdateResource(ctx, created)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(r1, updated, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	// Validate that the resource is stored under the custom key
	item, err = memBackend.Get(ctx, backend.NewKey("generic_prefix", "llama"))
	require.NoError(t, err)
	require.NotNil(t, item)

	// Validate that the default key still does not exist
	item, err = memBackend.Get(ctx, backend.NewKey("generic_prefix", r1.GetName()))
	require.Error(t, err)
	require.Nil(t, item)

	// Upsert the test resource
	upserted, err := service.UpsertResource(ctx, updated)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(r1, upserted, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	// Validate that the resource is stored under the custom key
	item, err = memBackend.Get(ctx, backend.NewKey("generic_prefix", "llama"))
	require.NoError(t, err)
	require.NotNil(t, item)

	// Validate that the default key still does not exist
	item, err = memBackend.Get(ctx, backend.NewKey("generic_prefix", r1.GetName()))
	require.Error(t, err)
	require.Nil(t, item)

	// Compare and swap the resource
	swapped, err := service.UpdateAndSwapResource(ctx, "llama", func(tr *testResource) error { return nil })
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(r1, swapped, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	// Validate that the resource is stored under the custom key
	item, err = memBackend.Get(ctx, backend.NewKey("generic_prefix", "llama"))
	require.NoError(t, err)
	require.NotNil(t, item)

	// Validate that the default key still does not exist
	item, err = memBackend.Get(ctx, backend.NewKey("generic_prefix", r1.GetName()))
	require.Error(t, err)
	require.Nil(t, item)

	// Validate that getting the resource through the service uses the overridden name
	_, err = service.GetResource(ctx, r1.GetName())
	require.NoError(t, err)
	_, err = service.GetResource(ctx, "llama")
	require.NoError(t, err)
	_, err = service.GetResource(ctx, "notllama")
	require.NoError(t, err)

	// Validate that deleting the resource also uses the overridden name
	err = service.DeleteResource(ctx, "notllama")
	require.NoError(t, err)
	_, err = memBackend.Get(ctx, backend.NewKey("generic_prefix", "llama"))
	require.ErrorAs(t, err, new(*trace.NotFoundError))
}
