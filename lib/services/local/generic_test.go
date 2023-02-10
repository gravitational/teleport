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
	"time"

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

	clock := clockwork.NewFakeClock()
	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := genericResourceService[*testResource]{
		backend:       backend,
		resourceKind:  "generic resource",
		limit:         200,
		backendPrefix: "generic_prefix",
		unmarshalFunc: unmarshalResource,
		marshalFunc:   marshalResource,
	}

	// Create a couple test resource objects.
	r1 := newTestResource("r1")
	r2 := newTestResource("r2")

	// Initially we expect no resources.
	out, nextToken, err := service.listResources(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)

	// Create a resource.
	err = service.createResource(ctx, r1, r1.GetName())
	require.NoError(t, err)

	// Create a resource with validators and a lock.
	postCheckValidatorCalled := false
	service.modificationPostCheckValidator = func(resource *testResource, name string) error {
		require.Empty(t, cmp.Diff(r2, resource,
			cmpopts.IgnoreFields(types.Metadata{}, "ID"),
		))
		require.Equal(t, r2.GetName(), name)
		postCheckValidatorCalled = true
		return nil
	}
	preModifyValidatorCalled := false
	service.preModifyValidator = func(ctx context.Context, resource *testResource, name string) error {
		require.Empty(t, cmp.Diff(r2, resource,
			cmpopts.IgnoreFields(types.Metadata{}, "ID"),
		))
		require.Equal(t, r2.GetName(), name)
		preModifyValidatorCalled = true
		return nil
	}

	service.modificationLockName = "test-lock"
	service.modificationLockTTL = time.Second * 5
	err = service.createResource(ctx, r2, r2.GetName())
	require.NoError(t, err)
	require.True(t, postCheckValidatorCalled)
	require.True(t, preModifyValidatorCalled)

	// Check post check validator failure.
	postCheckValidatorCalled = false
	preModifyValidatorCalled = false
	service.modificationPostCheckValidator = func(resource *testResource, name string) error {
		postCheckValidatorCalled = true
		return trace.BadParameter("expected error")
	}
	err = service.createResource(ctx, newTestResource("failure"), "failure")
	require.ErrorIs(t, err, trace.BadParameter("expected error"))
	require.True(t, postCheckValidatorCalled)
	require.False(t, preModifyValidatorCalled)

	// Check pre modify validator failure.
	postCheckValidatorCalled = false
	preModifyValidatorCalled = false
	service.modificationPostCheckValidator = nil
	service.preModifyValidator = func(ctx context.Context, resource *testResource, name string) error {
		preModifyValidatorCalled = true
		return trace.BadParameter("expected error")
	}
	err = service.createResource(ctx, newTestResource("failure"), "failure")
	require.ErrorIs(t, err, trace.BadParameter("expected error"))
	require.False(t, postCheckValidatorCalled)
	require.True(t, preModifyValidatorCalled)

	// Lock failure
	postCheckValidatorCalled = false
	preModifyValidatorCalled = false
	service.preModifyValidator = func(ctx context.Context, resource *testResource, name string) error {
		preModifyValidatorCalled = true
		select {
		case <-ctx.Done():
			return trace.BadParameter("lock failed as expected")
		case <-time.After(time.Second * 10):
			return trace.BadParameter("timeout waiting for lock to fail")
		}
	}

	createFinished := make(chan struct{})
	go func() {
		err = service.createResource(ctx, newTestResource("failure"), "failure")
		close(createFinished)
	}()

	// Increment the fake clock repeatedly so that we're sure we cause the `Clock().After()` in the backend
	// helper to trigger after it's been instantiated.
AdvanceClockLoop:
	for {
		select {
		case <-createFinished:
			break AdvanceClockLoop
		case <-time.After(time.Millisecond * 50):
			clock.Advance(time.Hour)
		}
	}

	require.ErrorIs(t, err, trace.BadParameter("lock failed as expected"))
	require.False(t, postCheckValidatorCalled)
	require.True(t, preModifyValidatorCalled)

	// Unset validators.
	service.modificationPostCheckValidator = nil
	service.preModifyValidator = nil

	// Fetch all resources using paging default.
	out, nextToken, err = service.listResources(ctx, 0, "")
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
	require.ErrorIs(t, err, trace.NotFound(`generic resource "doesnotexist" doesn't exist`))

	// Try to create the same resource.
	err = service.createResource(ctx, r1, r1.GetName())
	require.ErrorIs(t, err, trace.AlreadyExists(`generic resource "r1" already exists`))

	// Update a resource.
	r1.SetStaticLabels(map[string]string{"newlabel": "newvalue"})
	err = service.updateResource(ctx, r1, r1.GetName())
	require.NoError(t, err)
	r, err = service.getResource(ctx, r1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(r1, r,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Update a resource with a post check validator.
	postCheckValidatorCalled = false
	service.modificationPostCheckValidator = func(resource *testResource, name string) error {
		require.Empty(t, cmp.Diff(r1, resource,
			cmpopts.IgnoreFields(types.Metadata{}, "ID"),
		))
		postCheckValidatorCalled = true
		return nil
	}
	r1.SetStaticLabels(map[string]string{"newlabel2": "newvalue2"})
	err = service.updateResource(ctx, r1, r1.GetName())
	require.NoError(t, err)
	r, err = service.getResource(ctx, r1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(r1, r,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))
	require.True(t, postCheckValidatorCalled)

	// Update a resource that doesn't exist.
	err = service.updateResource(ctx, r1, "doesnotexist")
	require.ErrorIs(t, err, trace.NotFound(`generic resource "doesnotexist" doesn't exist`))

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
	require.ErrorIs(t, err, trace.NotFound(`generic resource "doesnotexist" doesn't exist`))

	// Delete all resources.
	err = service.deleteAllResources(ctx)
	require.NoError(t, err)
	out, nextToken, err = service.listResources(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)
}
