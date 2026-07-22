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

package local

import (
	"context"
	"iter"

	"github.com/gravitational/trace"

	foov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/foos"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

func fooUnscopedWatchPrefix() backend.Key {
	return backend.ExactKey("foo")
}

func fooScopedWatchPrefix() backend.Key {
	return backend.ExactKey("scoped", "foo")
}

// FooService is a storage service for Foos.
type FooService struct {
	service *generic.ScopeAwareServiceWrapper[*foov1.Foo]
}

func NewFooService(bk backend.Backend) (*FooService, error) {
	service, err := generic.NewScopeAwareServiceWrapper(generic.ScopeAwareServiceWrapperConfig[*foov1.Foo]{
		Backend:               bk,
		ResourceKind:          foos.Kind,
		UnscopedBackendPrefix: backend.NewKey("foo"),
		ScopedBackendPrefix:   backend.NewKey("scoped", "foo"),
		MarshalFunc:           services.MarshalProtoResource[*foov1.Foo],
		UnmarshalFunc:         services.UnmarshalProtoResource[*foov1.Foo],
		ValidateFunc:          foos.StrongValidate,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &FooService{
		service: service,
	}, nil
}

// CreateFoo creates a new Foo resource in the backend.
func (s *FooService) CreateFoo(ctx context.Context, foo *foov1.Foo) (*foov1.Foo, error) {
	return s.service.CreateResource(ctx, foo)
}

// UpdateFoo updates an existing Foo in the backend.
func (s *FooService) UpdateFoo(ctx context.Context, foo *foov1.Foo) (*foov1.Foo, error) {
	return s.service.ConditionalUpdateResource(ctx, foo)
}

// UpsertFoo creates a new Foo or replaces an existing Foo in the backend.
func (s *FooService) UpsertFoo(ctx context.Context, foo *foov1.Foo) (*foov1.Foo, error) {
	return s.service.UpsertResource(ctx, foo)
}

// GetFoo returns a single Foo matching the request
func (s *FooService) GetFoo(ctx context.Context, req *foov1.GetFooRequest) (*foov1.Foo, error) {
	// TBD: should we make the generic service accept anything implementing the
	// following interface instead of scopes.QualifiedName?
	//
	//   type ScopedResourceRequest {
	//     GetScope() string
	//     GetName() string
	//   }
	return s.service.GetResource(ctx, scopes.QualifiedName{
		Scope: req.GetScope(),
		Name:  req.GetName(),
	})
}

// ListFoos returns a page of Foos and the token to find the next page of items.
func (s *FooService) ListFoos(ctx context.Context, req *foov1.ListFoosRequest) ([]*foov1.Foo, string, error) {
	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return nil, "", trace.Wrap(err)
	}
	return s.service.ListResourcesWithFilter(ctx, int(req.GetPageSize()), req.GetPageToken(), func(foo *foov1.Foo) bool {
		return scopes.MatchScope(scopeFilter, foo.GetScope())
	})
}

// RangeFoos ranges over all foos matching any scope filter specified in the
// request, between startKey and endKey interpreted as scoped resource cursors.
func (s *FooService) RangeFoos(ctx context.Context, req *foov1.ListFoosRequest, startKey, endKey string) iter.Seq2[*foov1.Foo, error] {
	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return stream.Fail[*foov1.Foo](trace.Wrap(err))
	}
	return stream.FilterMap(s.service.Resources(ctx, startKey, endKey), func(foo *foov1.Foo) (*foov1.Foo, bool) {
		return foo, scopes.MatchScope(scopeFilter, foo.GetScope())
	})
}

// DeleteFoo removes a matching Foo resource
func (s *FooService) DeleteFoo(ctx context.Context, req *foov1.DeleteFooRequest) error {
	// TBD: should we make the generic service accept anything implementing the
	// following interface instead of scopes.QualifiedName?
	//
	//   type ScopedResourceRequest {
	//     GetScope() string
	//     GetName() string
	//   }
	return s.service.DeleteResource(ctx, scopes.QualifiedName{
		Scope: req.GetScope(),
		Name:  req.GetName(),
	})
}
