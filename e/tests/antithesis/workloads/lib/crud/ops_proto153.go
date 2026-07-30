package crud

import (
	"context"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

type protoResource153Ptr[S any] interface {
	types.ProtoResource153
	*S
}

type resourceOpsProto153[T protoResource153Ptr[S], S any, R ResourceOps[T]] struct {
	ops R
}

func (r *resourceOpsProto153[T, S, R]) Clone(resource T) T {
	return r.ops.Clone(resource)
}

func (r *resourceOpsProto153[T, S, R]) CloneR(resource types.Resource) types.Resource {
	typed, err := types.ConvertResource[T](resource)
	if err != nil {
		assert.Unreachable("No proto RFD 153 resource exists such that it cannot be converted from types.Resource", map[string]any{"kind": resource.GetKind()})
		panic("cannot convert proto RFD 153 resource: " + err.Error())
	}
	return types.ProtoResource153ToLegacy(r.ops.Clone(typed))
}

func (r *resourceOpsProto153[T, S, R]) Clone153(resource types.Resource153) types.Resource153 {
	return r.ops.Clone(mustCastResource[T](resource))
}

func (r *resourceOpsProto153[T, S, R]) Get(ctx context.Context, name string) (T, error) {
	return r.ops.Get(ctx, name)
}

func (r *resourceOpsProto153[T, S, R]) GetR(ctx context.Context, name string) (types.Resource, error) {
	resource, err := r.ops.Get(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.ProtoResource153ToLegacy(resource), nil
}

func (r *resourceOpsProto153[T, S, R]) Get153(ctx context.Context, name string) (types.Resource153, error) {
	return r.ops.Get(ctx, name)
}

func (r *resourceOpsProto153[T, S, R]) List(ctx context.Context, pageSize int, pageToken string) ([]T, string, error) {
	return r.ops.List(ctx, pageSize, pageToken)
}

func (r *resourceOpsProto153[T, S, R]) ListR(ctx context.Context, pageSize int, pageToken string) ([]types.Resource, string, error) {
	resources, next, err := r.ops.List(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	out := make([]types.Resource, 0, len(resources))
	for _, resource := range resources {
		out = append(out, types.ProtoResource153ToLegacy(resource))
	}
	return out, next, nil
}

func (r *resourceOpsProto153[T, S, R]) List153(ctx context.Context, pageSize int, pageToken string) ([]types.Resource153, string, error) {
	resources, next, err := r.ops.List(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	out := make([]types.Resource153, 0, len(resources))
	for _, resource := range resources {
		out = append(out, resource)
	}
	return out, next, nil
}
