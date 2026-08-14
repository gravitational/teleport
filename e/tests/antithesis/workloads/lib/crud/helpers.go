package crud

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	iterstream "github.com/gravitational/teleport/lib/itertools/stream"
)

// ListAll collects all resources visible through ops.
func ListAll(ctx context.Context, ops KindResourceOps, pageSize int) ([]types.Resource153, error) {
	return iterstream.Collect(clientutils.ResourcesWithPageSize(ctx, ops.List153, pageSize))
}

// SetStaticLabel sets one static label on a resource without aliasing the
// resource's existing label map.
func SetStaticLabel(resource types.Resource153, key, value string) error {
	if key == "" {
		return trace.BadParameter("label key is required")
	}

	if legacy, ok := unwrapLegacyResource(resource); ok {
		labeled, ok := legacy.(types.ResourceWithLabels)
		if !ok {
			return trace.BadParameter("resource %s does not support static labels", resource.GetKind())
		}

		labels := maps.Clone(labeled.GetStaticLabels())
		if labels == nil {
			labels = make(map[string]string)
		}
		labels[key] = value
		labeled.SetStaticLabels(labels)
		return nil
	}

	metadata := resource.GetMetadata()
	if metadata == nil {
		return trace.BadParameter("resource %s has no metadata", resource.GetKind())
	}

	labels := maps.Clone(metadata.Labels)
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[key] = value
	metadata.Labels = labels
	return nil
}

// EqualResourceSlices reports whether a and b contain equivalent resources
// of the same kind, irrespective of order. Both slices are sorted by their
// stable resource key (kind, name, subkind, version) and then compared using
// equal, such as [KindResourceOps.Equal].
func EqualResourceSlices[T types.Resource153](a, b []T, equal func(T, T) bool) bool {
	if len(a) != len(b) {
		return false
	}

	as := slices.Clone(a)
	bs := slices.Clone(b)
	sortByResourceKey(as)
	sortByResourceKey(bs)

	return slices.EqualFunc(as, bs, equal)
}

func mustCastResource[T types.Resource153](resource types.Resource153) T {
	typed, err := castResource[T](resource.GetKind(), resource)
	if err != nil {
		assert.Unreachable("No CRUD resource exists such that casting it from generic resource panics", map[string]any{"kind": resource.GetKind()})
		panic(fmt.Sprintf("cannot cast resource %T: %v", resource, err))
	}
	return typed
}

func castResource[T types.Resource153](kind string, resource types.Resource153) (T, error) {
	typed, ok := any(resource).(T)
	if !ok {
		return *new(T), trace.BadParameter("resource kind %q received unexpected resource type %T", kind, resource)
	}
	return typed, nil
}
