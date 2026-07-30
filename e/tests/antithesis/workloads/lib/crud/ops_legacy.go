package crud

import (
	"context"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

type resourceOpsLegacy[R ResourceOps[T], T types.Resource] struct {
	ops R
}

func (r *resourceOpsLegacy[R, T]) Clone(resource T) T {
	return r.ops.Clone(resource)
}

func mustConvertResource[T any](resource types.Resource) T {
	typed, err := types.ConvertResource[T](resource)
	if err != nil {
		assert.Unreachable("No CRUD resource exists such that it cannot be converted from types.Resource", map[string]any{"kind": resource.GetKind()})
		panic("cannot convert resource: " + err.Error())
	}
	return typed
}

func (r *resourceOpsLegacy[R, T]) CloneR(resource types.Resource) types.Resource {
	return r.ops.Clone(mustConvertResource[T](resource))
}

func mustUnwrapLegacy[T types.Resource](resource types.Resource153) T {
	typed, err := unwrapLegacy[T](resource)
	if err != nil {
		assert.Unreachable("No legacy CRUD resource exists such that it cannot be unwrapped from types.Resource153", map[string]any{"kind": resource.GetKind()})
		panic("cannot unwrap legacy resource: " + err.Error())
	}
	return typed
}

func (r *resourceOpsLegacy[R, T]) Clone153(resource types.Resource153) types.Resource153 {
	return types.LegacyToResource153(r.ops.Clone(mustUnwrapLegacy[T](resource)))
}

func (r *resourceOpsLegacy[R, T]) Get(ctx context.Context, name string) (T, error) {
	return r.ops.Get(ctx, name)
}

func (r *resourceOpsLegacy[R, T]) GetR(ctx context.Context, name string) (types.Resource, error) {
	return r.ops.Get(ctx, name)
}

func (r *resourceOpsLegacy[R, T]) Get153(ctx context.Context, name string) (types.Resource153, error) {
	resource, err := r.ops.Get(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.LegacyToResource153(resource), nil
}

func (r *resourceOpsLegacy[R, T]) List(ctx context.Context, pageSize int, pageToken string) ([]T, string, error) {
	return r.ops.List(ctx, pageSize, pageToken)
}

func (r *resourceOpsLegacy[R, T]) ListR(ctx context.Context, pageSize int, pageToken string) ([]types.Resource, string, error) {
	resources, next, err := r.ops.List(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return legacySliceToResource(resources), next, nil
}

func (r *resourceOpsLegacy[R, T]) List153(ctx context.Context, pageSize int, pageToken string) ([]types.Resource153, string, error) {
	resources, next, err := r.ops.List(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return legacySliceToResource153(resources), next, nil
}
