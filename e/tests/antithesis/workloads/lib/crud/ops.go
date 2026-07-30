package crud

import (
	"context"

	"github.com/gravitational/teleport/api/types"
)

// ResourceOps contains typed read operations and clone support for one resource
// representation.
type ResourceOps[T any] interface {
	Clone(T) T
	Get(context.Context, string) (T, error)
	List(context.Context, int, string) ([]T, string, error)
}

// ResourceOpsR contains read operations and clone support for legacy
// pre-RFD-153 resources.
type ResourceOpsR interface {
	CloneR(types.Resource) types.Resource
	GetR(context.Context, string) (types.Resource, error)
	ListR(context.Context, int, string) ([]types.Resource, string, error)
}

// ResourceOps153 contains read operations and clone support for RFD-153
// resources represented as types.Resource153.
type ResourceOps153 interface {
	Clone153(types.Resource153) types.Resource153
	Get153(context.Context, string) (types.Resource153, error)
	List153(context.Context, int, string) ([]types.Resource153, string, error)
}

// KindResourceOps is a type-erased view of CRUD operations for generic test
// composition across multiple resource kinds.
type KindResourceOps interface {
	ResourceOps153
	Kind() string
	NewResource(string) (types.Resource153, error)
	Create(context.Context, types.Resource153) (types.Resource153, error)
	Update(context.Context, types.Resource153) (types.Resource153, error)
	Delete(context.Context, string) error
	DeleteAll(context.Context) error
	SupportsDeleteAll() bool
	Equal(a, b types.Resource153) bool
}
