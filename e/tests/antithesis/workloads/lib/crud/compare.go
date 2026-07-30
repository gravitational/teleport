package crud

import (
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"
	"rsc.io/ordered"

	"github.com/gravitational/teleport/api/types"
)

// resourceSortKey returns a stable ordering key
func resourceSortKey(resource types.Resource153) string {
	return string(ordered.Encode(resource.GetKind(), resource.GetMetadata().GetName(), resource.GetSubKind(), resource.GetVersion()))
}

// sortByResourceKey sorts resources in place by their stable resource key,
// so that slice comparisons don't depend on the order resources happened to
// come back from the backend.
func sortByResourceKey[T types.Resource153](resources []T) {
	slices.SortFunc(resources, func(a, b T) int {
		return strings.Compare(resourceSortKey(a), resourceSortKey(b))
	})
}

// EqualProtoMessage returns an equality implementation for modern
// resources that are plain protobuf messages
//
//	Equal: EqualProtoMessage[*accessmonitoringrulesv1.AccessMonitoringRule],
func EqualProtoMessage[T proto.Message](a, b T) bool {
	return proto.Equal(a, b)
}

// legacyEqualable constrains legacy resource types that compare themselves IsEqual(T)
type legacyEqualable[T any] interface {
	types.Resource
	IsEqual(T) bool
}

// EqualLegacyResource returns an equality implementation for legacy
// (pre-153) resources whose concrete type implements IsEqual
//
//	Equal: EqualLegacyResource[*types.RoleV6],
func EqualLegacyResource[T legacyEqualable[T]](a, b types.Resource153) bool {
	la, aok := unwrapLegacyResource(a)
	lb, bok := unwrapLegacyResource(b)
	if !aok || !bok {
		return false
	}
	typedA, aok := la.(T)
	typedB, bok := lb.(T)
	if !aok || !bok {
		return false
	}
	return typedA.IsEqual(typedB)
}
