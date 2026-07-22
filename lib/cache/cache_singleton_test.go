// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
)

type testSingletonFuncs153[T types.Resource153] struct {
	newResource func() T
	create      func(context.Context, T) (T, error)
	get         func(context.Context) (T, error)
	cacheGet    func(context.Context) (T, error)
	update      func(context.Context, T) (T, error)
	delete      func(context.Context) error
	setup       func(T)
	modify      func(T)
	cmpOpts     []cmp.Option
}

func (f *testSingletonFuncs153[T]) CheckAndSetDefaults(t *testing.T) {
	if f.setup == nil {
		f.setup = func(t T) {
			metadata := t.GetMetadata()
			if !metadata.HasExpires() {
				metadata.SetExpires(timestamppb.New(time.Now().Add(30 * time.Minute)))
			} else {
				expiry := metadata.GetExpires().AsTime()
				metadata.SetExpires(timestamppb.New(expiry.Add(30 * time.Minute)))
			}
			metadata.SetLabels(map[string]string{"label": "value1"})
		}
	}
	if f.modify == nil {
		f.modify = func(t T) {
			metadata := t.GetMetadata()
			if !metadata.HasExpires() {
				metadata.SetExpires(timestamppb.New(time.Now().Add(30 * time.Minute)))
			} else {
				expiry := metadata.GetExpires().AsTime()
				metadata.SetExpires(timestamppb.New(expiry.Add(30 * time.Minute)))
			}
			metadata.GetLabels()["label"] = "value2"
		}
	}
	if f.cmpOpts == nil {
		f.cmpOpts = []cmp.Option{
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
			protocmp.Transform(),
			cmpopts.EquateEmpty(),
		}
	}

	require.NotNil(t, f.newResource, "newResource function must be provided")
	require.NotNil(t, f.create, "create function must be provided")
	require.NotNil(t, f.get, "get function must be provided")
	require.NotNil(t, f.cacheGet, "cacheGet function must be provided")
	require.NotNil(t, f.update, "update function must be provided")
	require.NotNil(t, f.delete, "delete function must be provided")
}

// testSingleton153 is a wrapper for testing singleton resources conforming to [types.Resource153]
// Must be ran within [synctest.Test].
func testSingleton153[T types.Resource153](t *testing.T, p *testPack, funcs testSingletonFuncs153[T]) {
	funcs.CheckAndSetDefaults(t)
	ctx := t.Context()

	// Ensure the cache is healthy before proceeding to
	// prevent running the tests falling back to upstream reads.
	require.True(t, p.cache.ok)

	// Ensure the singleton doesn't already exist.
	_, err := funcs.get(ctx)
	require.True(t, trace.IsNotFound(err))

	_, err = funcs.cacheGet(ctx)
	require.True(t, trace.IsNotFound(err))

	// Create a resource.
	res := funcs.newResource()
	funcs.setup(res)
	cmpOpts := funcs.cmpOpts

	created, err := funcs.create(ctx, res)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(res, created, cmpOpts...))

	// Check that the resource is now in the backend.
	backendRes, err := funcs.get(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(res, backendRes, cmpOpts...))

	// Wait until the information has been replicated to the cache.
	synctest.Wait()

	// Make sure a single cache get works.
	cachedRes, err := funcs.cacheGet(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(res, cachedRes, cmpOpts...))

	funcs.modify(res)
	updated, err := funcs.update(ctx, res)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(updated, res, cmpOpts...))

	updatedBackendRes, err := funcs.get(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(updatedBackendRes, res, cmpOpts...))
	require.NotEmpty(t, cmp.Diff(backendRes, updatedBackendRes, cmpOpts...), "update did not change the resource")

	p.cache.ok = false
	// Ensure fallback works when cache is unhealthy.
	fallbackRes, err := funcs.cacheGet(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(res, fallbackRes, cmpOpts...))
	p.cache.ok = true

	err = funcs.delete(ctx)
	require.NoError(t, err)

	_, err = funcs.get(ctx)
	require.True(t, trace.IsNotFound(err))

	// Wait until the delete event hits the cache.
	synctest.Wait()

	// Ensure the resource is deleted from the cache.
	_, err = funcs.cacheGet(ctx)
	require.True(t, trace.IsNotFound(err))
}

type testLegacySingletonFuncs[T types.Resource] struct {
	newResource func() T
	create      func(context.Context, T) error
	get         func(context.Context) (T, error)
	cacheGet    func(context.Context) (T, error)
	update      func(context.Context, T) error
	delete      func(context.Context) error
	setup       func(T)
	modify      func(T)
	cmpOpts     []cmp.Option
}

func (f *testLegacySingletonFuncs[T]) CheckAndSetDefaults(t *testing.T) {
	if f.setup == nil {
		f.setup = func(t T) {
			if t.Expiry().IsZero() {
				t.SetExpiry(time.Now().Add(30 * time.Minute))
			} else {
				t.SetExpiry(t.Expiry().Add(30 * time.Minute))
			}
		}
	}
	if f.modify == nil {
		f.modify = func(t T) {
			if t.Expiry().IsZero() {
				t.SetExpiry(time.Now().Add(30 * time.Minute))
			} else {
				t.SetExpiry(t.Expiry().Add(30 * time.Minute))
			}
		}
	}
	if f.cmpOpts == nil {
		f.cmpOpts = []cmp.Option{
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
			cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
		}
	}

	require.NotNil(t, f.newResource, "newResource function must be provided")
	require.NotNil(t, f.create, "create function must be provided")
	require.NotNil(t, f.get, "get function must be provided")
	require.NotNil(t, f.cacheGet, "cacheGet function must be provided")
	require.NotNil(t, f.update, "update function must be provided")
	require.NotNil(t, f.delete, "delete function must be provided")
}

// testLegacySingleton is a wrapper for testing resources conforming to [types.Resource]
// Must be ran within [synctest.Test].
func testLegacySingleton[T types.Resource](t *testing.T, p *testPack, funcs testLegacySingletonFuncs[T]) {
	funcs.CheckAndSetDefaults(t)
	ctx := t.Context()

	// Ensure the cache is healthy before proceeding to
	// prevent running the tests falling back to upstream reads.
	require.True(t, p.cache.ok)

	// Ensure the singleton doesn't already exist.
	_, err := funcs.get(ctx)
	require.True(t, trace.IsNotFound(err))

	_, err = funcs.cacheGet(ctx)
	require.True(t, trace.IsNotFound(err))

	res := funcs.newResource()
	funcs.setup(res)
	cmpOpts := funcs.cmpOpts

	// Create a resource.
	err = funcs.create(ctx, res)
	require.NoError(t, err)

	// Check that the resource is now in the backend.
	backendRes, err := funcs.get(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(res, backendRes, cmpOpts...))

	// Wait until the information has been replicated to the cache.
	synctest.Wait()

	// Make sure a single cache get works.
	cachedRes, err := funcs.cacheGet(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(res, cachedRes, cmpOpts...))

	funcs.modify(res)
	err = funcs.update(ctx, res)
	require.NoError(t, err)

	updatedBackendRes, err := funcs.get(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(updatedBackendRes, res, cmpOpts...))
	require.NotEmpty(t, cmp.Diff(backendRes, updatedBackendRes, cmpOpts...), "update did not change the resource")

	p.cache.ok = false
	// Ensure fallback works when cache is unhealthy.
	fallbackRes, err := funcs.cacheGet(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(res, fallbackRes, cmpOpts...))
	p.cache.ok = true

	err = funcs.delete(ctx)
	require.NoError(t, err)

	_, err = funcs.get(ctx)
	require.True(t, trace.IsNotFound(err))

	// Wait until the delete event hits the cache.
	synctest.Wait()

	// Ensure the resource is deleted from the cache.
	_, err = funcs.cacheGet(ctx)
	require.True(t, trace.IsNotFound(err))
}
