package crud

import (
	"fmt"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport/api/types"
)

// CloneProto clones modern resources.
//
//	Clone: CloneProto[*accessmonitoringrulesv1.AccessMonitoringRule],
func CloneProto[T proto.Message](resource T) T {
	return proto.CloneOf(resource)
}

// legacyCloneable constrains legacy resource types that clone themselves via
// a Clone() T method
type legacyCloneable[T any] interface {
	types.Resource
	Clone() T
}

// CloneLegacyResourceR returns a [ResourceOpsR.CloneR] implementation for
// legacy (pre-153) resources whose type declares Clone().
func CloneLegacyResourceR[T legacyCloneable[T]](resource types.Resource) types.Resource {
	typed, ok := resource.(T)
	if !ok {
		assert.Unreachable("No legacy resource using CloneLegacyResourceR exists such that cannot be casted into legacyCloneable", map[string]any{"kind": resource.GetKind()})
		panic(fmt.Sprintf("CloneLegacyResourceR: legacy resource kind %q received unexpected legacy type %T", resource.GetKind(), resource))
	}
	return typed.Clone()
}

// CloneLegacyResource clones legacy (pre-153) resources whose type declares
// Clone().
//
//	Clone: CloneLegacyResource[types.Role],
//
// Panics if resource does not unwrap to a legacy resource of type T.
func CloneLegacyResource[T legacyCloneable[T]](resource types.Resource153) types.Resource153 {
	legacy, ok := unwrapLegacyResource(resource)
	if !ok {
		assert.Unreachable("No legacy resource using CloneLegacyResource such that cannot be unwrapped from types.Resource153", map[string]any{"kind": resource.GetKind()})
		panic(fmt.Sprintf("CloneLegacyResource: resource kind %q (%T) is not a legacy-wrapped resource", resource.GetKind(), resource))
	}
	typed, ok := legacy.(T)
	if !ok {
		assert.Unreachable("No legacy resource using CloneLegacyResource exists such that cannot be casted into legacyCloneable", map[string]any{"kind": resource.GetKind()})
		panic(fmt.Sprintf("CloneLegacyResource: legacy resource kind %q received unexpected legacy type %T", resource.GetKind(), legacy))
	}
	return types.LegacyToResource153(typed.Clone())
}

// legacyCopyable constrains legacy resource types that clone themselves via
// a Copy() T method
type legacyCopyable[T any] interface {
	types.Resource
	Copy() T
}

// CopyLegacyResourceR returns a [ResourceOpsR.CloneR] implementation for
// legacy (pre-153) resources whose concrete type declares Copy().
func CopyLegacyResourceR[T legacyCopyable[T]](resource types.Resource) types.Resource {
	typed, ok := resource.(T)
	if !ok {
		assert.Unreachable("No legacy resource using CopyLegacyResourceR exists such that cannot be casted into legacyCopyable", map[string]any{"kind": resource.GetKind()})
		panic(fmt.Sprintf("CopyLegacyResourceR: legacy resource kind %q received unexpected legacy type %T", resource.GetKind(), resource))
	}
	return typed.Copy()
}

// CopyLegacyResource clones legacy (pre-153) resources whose concrete type
// declares Copy().
//
//	Clone: CopyLegacyResource[*types.AppV3],
//
// Panics if resource does not unwrap to a legacy resource of type T
func CopyLegacyResource[T legacyCopyable[T]](resource types.Resource153) types.Resource153 {
	legacy, ok := unwrapLegacyResource(resource)
	if !ok {
		assert.Unreachable("No legacy resource using CopyLegacyResource exists such that cannot be unwrapped from types.Resource153", map[string]any{"kind": resource.GetKind()})
		panic(fmt.Sprintf("CopyLegacyResource: resource kind %q (%T) is not a legacy-wrapped resource", resource.GetKind(), resource))

	}
	typed, ok := legacy.(T)
	if !ok {
		assert.Unreachable("No legacy resource using CopyLegacyResource exists such that cannot be casted into legacyCopyable", map[string]any{"kind": resource.GetKind()})
		panic(fmt.Sprintf("CopyLegacyResource: legacy resource kind %q received unexpected legacy type %T", resource.GetKind(), legacy))
	}
	return types.LegacyToResource153(typed.Copy())
}
