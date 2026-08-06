// Package resourceregistry is a small prototype for registering canonical
// resource CRUD shape once and deriving consumer-specific adapters elsewhere.
package resourceregistry

import (
	"context"
	"fmt"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// NameID is the common identifier shape for ordinary name-addressed resources.
type NameID string

func (n NameID) String() string {
	return string(n)
}

// Page describes a paginated list request.
type Page struct {
	Size  int
	Token string
}

// Reader is the common read surface for one resource type.
type Reader[T any, ID comparable] interface {
	Get(context.Context, ID) (T, error)
	List(context.Context, Page) ([]T, string, error)
}

// Client is the common CRUD surface for one resource type.
type Client[T any, ID comparable] interface {
	Reader[T, ID]
	Create(context.Context, T) (T, error)
	Update(context.Context, T) (T, error)
	Delete(context.Context, ID) error
}

// Upserter is deliberately optional. Consumers like tctl can use it for
// --force-style behavior when available without making it part of ordinary CRUD.
type Upserter[T any] interface {
	Upsert(context.Context, T) (T, error)
}

// MarshalFunc converts a resource to its wire/storage representation.
type MarshalFunc[T any] func(T, ...services.MarshalOption) ([]byte, error)

// UnmarshalFunc converts a wire/storage representation into a resource.
type UnmarshalFunc[T any] func([]byte, ...services.MarshalOption) (T, error)

// Spec is the small, canonical description of one resource type.
//
// It intentionally does not contain Terraform schemas, tctl table output,
// cache indexes, or fuzzing policy. Those belong in package-local adapters
// that can be derived from Spec and supplemented with their own hooks.
type Spec[T any, ID comparable] struct {
	Kind string

	New      func() T
	Clone    func(T) T
	ID       func(T) ID
	Revision func(T) string

	Marshal   MarshalFunc[T]
	Unmarshal UnmarshalFunc[T]
	Validate  func(T) error

	ReadClient func(any) (Reader[T, ID], error)
	Client     func(any) (Client[T, ID], error)
}

func (s Spec[T, ID]) check() error {
	if s.Kind == "" {
		return trace.BadParameter("missing resource kind")
	}
	if s.New == nil {
		return trace.BadParameter("missing New function for %q", s.Kind)
	}
	if s.Clone == nil {
		return trace.BadParameter("missing Clone function for %q", s.Kind)
	}
	if s.ID == nil {
		return trace.BadParameter("missing ID function for %q", s.Kind)
	}
	if s.Revision == nil {
		return trace.BadParameter("missing Revision function for %q", s.Kind)
	}
	if s.Marshal == nil {
		return trace.BadParameter("missing Marshal function for %q", s.Kind)
	}
	if s.Unmarshal == nil {
		return trace.BadParameter("missing Unmarshal function for %q", s.Kind)
	}
	if s.Validate == nil {
		return trace.BadParameter("missing Validate function for %q", s.Kind)
	}
	if s.Client == nil {
		return trace.BadParameter("missing Client function for %q", s.Kind)
	}
	return nil
}

// ReaderFor returns a read-only resource client for the given implementation.
func (s Spec[T, ID]) ReaderFor(client any) (Reader[T, ID], error) {
	if s.ReadClient != nil {
		reader, err := s.ReadClient(client)
		return reader, trace.Wrap(err)
	}

	resourceClient, err := s.Client(client)
	return resourceClient, trace.Wrap(err)
}

type entry interface {
	kind() string
	typeName() string
}

type typedEntry[T any, ID comparable] struct {
	spec Spec[T, ID]
}

func (e typedEntry[T, ID]) kind() string {
	return e.spec.Kind
}

func (e typedEntry[T, ID]) typeName() string {
	return fmt.Sprintf("%T", *new(T))
}

// Registry stores type-erased resource specs keyed by kind.
type Registry struct {
	mu      sync.RWMutex
	entries map[string]entry
}

// New returns an empty registry.
func New() *Registry {
	return &Registry{
		entries: make(map[string]entry),
	}
}

var (
	defaultRegistryOnce sync.Once
	defaultRegistry     *Registry
)

// Default returns the shared registry used by consumers that want the common
// resource catalog.
func Default() *Registry {
	defaultRegistryOnce.Do(func() {
		defaultRegistry = New()
		MustRegister(defaultRegistry, AccessMonitoringRuleSpec())
		MustRegister(defaultRegistry, RoleSpec())
	})
	return defaultRegistry
}

// Register adds a typed spec to the registry.
func Register[T any, ID comparable](r *Registry, spec Spec[T, ID]) error {
	if r == nil {
		return trace.BadParameter("missing registry")
	}
	if err := spec.check(); err != nil {
		return trace.Wrap(err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.entries[spec.Kind]; ok {
		return trace.AlreadyExists("resource kind %q is already registered", spec.Kind)
	}
	r.entries[spec.Kind] = typedEntry[T, ID]{spec: spec}
	return nil
}

// MustRegister adds a typed spec to the registry and panics on failure.
func MustRegister[T any, ID comparable](r *Registry, spec Spec[T, ID]) {
	if err := Register(r, spec); err != nil {
		panic(err)
	}
}

// Get returns the typed spec for kind.
func Get[T any, ID comparable](r *Registry, kind string) (Spec[T, ID], error) {
	var zero Spec[T, ID]
	if r == nil {
		return zero, trace.BadParameter("missing registry")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[kind]
	if !ok {
		return zero, trace.NotFound("resource kind %q is not registered", kind)
	}

	typed, ok := e.(typedEntry[T, ID])
	if !ok {
		return zero, trace.BadParameter("resource kind %q is registered with a different concrete type (%s)", kind, e.typeName())
	}
	return typed.spec, nil
}

// MustGet returns a typed spec and panics if the registry is not configured as
// expected.
func MustGet[T any, ID comparable](r *Registry, kind string) Spec[T, ID] {
	spec, err := Get[T, ID](r, kind)
	if err != nil {
		panic(err)
	}
	return spec
}

// Kinds returns the registered kinds.
func (r *Registry) Kinds() []string {
	if r == nil {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	kinds := make([]string, 0, len(r.entries))
	for kind := range r.entries {
		kinds = append(kinds, kind)
	}
	return kinds
}

// ToResource153 bridges either a modern RFD-153 resource or a legacy
// types.Resource into the common representation used by type-erased consumers.
func ToResource153[T any](resource T) (types.Resource153, error) {
	switch r := any(resource).(type) {
	case types.Resource153:
		return r, nil
	case types.Resource:
		return types.LegacyToResource153(r), nil
	default:
		return nil, trace.BadParameter("resource type %T is neither types.Resource153 nor types.Resource", resource)
	}
}

// FromResource153 converts a type-erased resource back to the typed resource
// used by a Spec.
func FromResource153[T any](resource types.Resource153) (T, error) {
	if typed, ok := any(resource).(T); ok {
		return typed, nil
	}

	if legacy, ok := resource.(interface{ UnwrapT() types.Resource }); ok {
		return types.ConvertResource[T](legacy.UnwrapT())
	}
	return types.ConvertResource[T](types.Resource153ToLegacy(resource))
}
