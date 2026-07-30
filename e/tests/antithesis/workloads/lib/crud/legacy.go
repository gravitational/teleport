package crud

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

func unwrapLegacy[T types.Resource](resource types.Resource153) (T, error) {
	legacy, ok := unwrapLegacyResource(resource)
	if !ok {
		return *new(T), trace.BadParameter("resource kind %q received unexpected resource type %T", resource.GetKind(), resource)
	}
	typed, err := types.ConvertResource[T](legacy)
	if err != nil {
		return *new(T), trace.Wrap(err)
	}
	return typed, nil
}

func unwrapLegacyResource(resource types.Resource153) (types.Resource, bool) {
	unwrapper, ok := resource.(interface{ UnwrapT() types.Resource })
	if !ok {
		return nil, false
	}
	return unwrapper.UnwrapT(), true
}

func legacySliceToResource153[T types.Resource](resources []T) []types.Resource153 {
	out := make([]types.Resource153, 0, len(resources))
	for _, resource := range resources {
		out = append(out, types.LegacyToResource153(resource))
	}
	return out
}

func legacySliceToResource[T types.Resource](resources []T) []types.Resource {
	out := make([]types.Resource, 0, len(resources))
	for _, resource := range resources {
		out = append(out, resource)
	}
	return out
}
