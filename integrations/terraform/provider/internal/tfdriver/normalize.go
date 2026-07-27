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

package tfdriver

import (
	"context"

	"github.com/gravitational/trace"
)

// ResourceNormalizer prepares a Teleport resource before create and update.
type ResourceNormalizer[T any] interface {
	// NormalizeCreate prepares a resource before create.
	NormalizeCreate(context.Context, *T) error
	// NormalizeUpdate prepares a resource before update.
	NormalizeUpdate(context.Context, *T) error
}

// ResourceNormalizerFuncs adapts functions to ResourceNormalizer.
type ResourceNormalizerFuncs[T any] struct {
	Create func(context.Context, *T) error
	Update func(context.Context, *T) error
}

// NormalizeCreate prepares a resource before create.
func (n ResourceNormalizerFuncs[T]) NormalizeCreate(ctx context.Context, resource *T) error {
	if n.Create == nil {
		return nil
	}
	return n.Create(ctx, resource)
}

// NormalizeUpdate prepares a resource before update.
func (n ResourceNormalizerFuncs[T]) NormalizeUpdate(ctx context.Context, resource *T) error {
	if n.Update == nil {
		return nil
	}
	return n.Update(ctx, resource)
}

// ResourceNormalizers runs a list of normalizers.
type ResourceNormalizers[T any] []ResourceNormalizer[T]

// NormalizeCreate prepares a resource before create.
func (n ResourceNormalizers[T]) NormalizeCreate(ctx context.Context, resource *T) error {
	for _, normalizer := range n {
		if err := normalizer.NormalizeCreate(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// NormalizeUpdate prepares a resource before update.
func (n ResourceNormalizers[T]) NormalizeUpdate(ctx context.Context, resource *T) error {
	for _, normalizer := range n {
		if err := normalizer.NormalizeUpdate(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// CheckAndSetDefaults checks defaults on create and update.
func CheckAndSetDefaults[T any]() ResourceNormalizer[T] {
	return ResourceNormalizerFuncs[T]{
		Create: func(_ context.Context, resource *T) error {
			defaulter, ok := any(resource).(interface{ CheckAndSetDefaults() error })
			if !ok {
				return trace.BadParameter("%T does not implement CheckAndSetDefaults", resource)
			}
			return trace.Wrap(defaulter.CheckAndSetDefaults())
		},
		Update: func(ctx context.Context, resource *T) error {
			defaulter, ok := any(resource).(interface{ CheckAndSetDefaults() error })
			if !ok {
				return trace.BadParameter("%T does not implement CheckAndSetDefaults", resource)
			}
			return trace.Wrap(defaulter.CheckAndSetDefaults())
		},
	}
}

// ForceKind sets a Teleport resource kind on create and update.
func ForceKind[T any](kind string) ResourceNormalizer[T] {
	return ResourceNormalizerFuncs[T]{
		Create: func(_ context.Context, resource *T) error {
			setter, ok := any(resource).(interface{ SetKind(string) })
			if !ok {
				return trace.BadParameter("%T does not implement SetKind", resource)
			}
			setter.SetKind(kind)
			return nil
		},
		Update: func(ctx context.Context, resource *T) error {
			setter, ok := any(resource).(interface{ SetKind(string) })
			if !ok {
				return trace.BadParameter("%T does not implement SetKind", resource)
			}
			setter.SetKind(kind)
			return nil
		},
	}
}
