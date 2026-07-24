/*
Copyright 2024 Gravitational, Inc.

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

package compare

// IsEqual will be used instead of cmp.Equal if a resource implements it.
type IsEqual[T any] interface {
	IsEqual(T) bool
}

// Equal compares two objects for semantic equality using their IsEqual method.
// The type T must implement IsEqual(T) bool and Clone() T methods.
//
// NOTE: this function by default CLONE the input objects before comparison to avoid
// modifying the originals. Use WithSkipClone() to skip cloning if the inputs can be
// safely modified. Use WithTransform() to apply custom field resets before comparison.
// Use WithEqualFunc() to provide a custom equality function instead of using IsEqual.
//
// Example:
//
// Suppose you want to compare a resource during reconciliation.
// You may already know that some spec fields should be excluded from the comparison,
// because they are not relevant for your use case.
//
// For example, AccessList reconciliation may build objects on the fly where many fields
// are unknown in the desired state but are populated by other flows.
// In that case, you may want to reset/ignore those fields before comparing.
//
//	isEqual := compare.Equal(memberFromDesiredState, memberFromBackend,
//	    compare.WithTransform(func(m *AccessListMember) {
//	        m.Spec.IneligibleStatus = ""
//	        m.Spec.AddedBy = ""
//	        m.Spec.Reason = ""
//	        m.Spec.Expires = time.Time{}
//	        m.Spec.Joined = time.Time{}
//	    }),
//	)
func Equal[T resource[T]](a, b T, opts ...EqualOption[T]) bool {
	cfg := equalConfig[T]{}
	for _, opt := range opts {
		opt(&cfg)
	}
	if !cfg.skipClone {
		a = a.Clone()
		b = b.Clone()
	}
	for _, resetFn := range cfg.transformFuncs {
		resetFn(a)
		resetFn(b)
	}
	if cfg.equalFunc != nil {
		return cfg.equalFunc(a, b)
	}
	return a.IsEqual(b)
}

// WithSkipClone configures Equal to skip cloning and directly mutate the input objects.
// Use this option only when you're certain the input objects can be safely modified
// (e.g., they're already clones or will be discarded after comparison).
func WithSkipClone[T any]() EqualOption[T] {
	return func(c *equalConfig[T]) {
		c.skipClone = true
	}
}

// WithTransform configures Equal to apply a custom field reset function before comparison.
// Multiple reset functions can be added and will be applied in order.
func WithTransform[T any](resetFn func(T)) EqualOption[T] {
	return func(c *equalConfig[T]) {
		c.transformFuncs = append(c.transformFuncs, resetFn)
	}
}

// WithEqualFunc configures Equal to use a custom equality function instead of IsEqual method.
// This is useful when you want to use a different equality method (for example IsEqualV2)
func WithEqualFunc[T any](equalFn func(T, T) bool) EqualOption[T] {
	return func(c *equalConfig[T]) {
		c.equalFunc = equalFn
	}
}

type resource[T any] interface {
	IsEqual[T]
	Clone() T
}

// EqualOption is a functional option for configuring the behavior of Equal.
type EqualOption[T any] func(*equalConfig[T])

type equalConfig[T any] struct {
	skipClone      bool
	transformFuncs []func(T)
	equalFunc      func(T, T) bool
}
