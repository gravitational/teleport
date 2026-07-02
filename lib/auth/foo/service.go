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

package foo

import (
	"context"
	"iter"

	"github.com/gravitational/trace"

	foov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/foos"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

type Reader interface {
	GetFoo(ctx context.Context, req *foov1.GetFooRequest) (*foov1.Foo, error)
	RangeFoos(ctx context.Context, req *foov1.ListFoosRequest, startKey, endKey string) iter.Seq2[*foov1.Foo, error]
}

type Writer interface {
	CreateFoo(ctx context.Context, foo *foov1.Foo) (*foov1.Foo, error)
	UpdateFoo(ctx context.Context, foo *foov1.Foo) (*foov1.Foo, error)
	UpsertFoo(ctx context.Context, foo *foov1.Foo) (*foov1.Foo, error)
	DeleteFoo(ctx context.Context, req *foov1.DeleteFooRequest) error
}

type Config struct {
	ScopedAuthorizer authz.ScopedAuthorizer
	Reader           Reader
	Writer           Writer
}

type Service struct {
	cfg *Config
	foov1.UnimplementedFooServiceServer
}

func NewService(cfg *Config) *Service {
	return &Service{
		cfg: cfg,
	}
}

func (s *Service) CreateFoo(ctx context.Context, req *foov1.CreateFooRequest) (*foov1.CreateFooResponse, error) {
	authzContext, err := s.cfg.ScopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx := authzContext.RuleContext()
	ruleCtx.Resource153 = req.GetFoo()
	if err := authzContext.CheckerContext.Decision(ctx, req.GetFoo().GetScope(), func(checker *services.ScopedAccessChecker) error {
		return checker.CheckAccessToRules(&ruleCtx, foos.Kind, types.VerbCreate)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.cfg.Writer.CreateFoo(ctx, req.GetFoo())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return foov1.CreateFooResponse_builder{
		Foo: created,
	}.Build(), nil
}

func (s *Service) UpdateFoo(ctx context.Context, req *foov1.UpdateFooRequest) (*foov1.UpdateFooResponse, error) {
	authzContext, err := s.cfg.ScopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx := authzContext.RuleContext()
	ruleCtx.Resource153 = req.GetFoo()
	if err := authzContext.CheckerContext.Decision(ctx, req.GetFoo().GetScope(), func(checker *services.ScopedAccessChecker) error {
		return checker.CheckAccessToRules(&ruleCtx, foos.Kind, types.VerbUpdate)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := s.cfg.Writer.UpdateFoo(ctx, req.GetFoo())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return foov1.UpdateFooResponse_builder{
		Foo: updated,
	}.Build(), nil
}

func (s *Service) UpsertFoo(ctx context.Context, req *foov1.UpsertFooRequest) (*foov1.UpsertFooResponse, error) {
	authzContext, err := s.cfg.ScopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx := authzContext.RuleContext()
	ruleCtx.Resource153 = req.GetFoo()
	if err := authzContext.CheckerContext.Decision(ctx, req.GetFoo().GetScope(), func(checker *services.ScopedAccessChecker) error {
		return checker.CheckAccessToRules(&ruleCtx, foos.Kind, types.VerbCreate, types.VerbUpdate)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	upserted, err := s.cfg.Writer.UpsertFoo(ctx, req.GetFoo())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return foov1.UpsertFooResponse_builder{
		Foo: upserted,
	}.Build(), nil
}

func (s *Service) GetFoo(ctx context.Context, req *foov1.GetFooRequest) (*foov1.GetFooResponse, error) {
	authzContext, err := s.cfg.ScopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx := authzContext.RuleContext()

	// do a pre-check to weed out requests that definitely won't be authorized.
	if err := authzContext.CheckerContext.CheckMaybeHasAccessToRules(&ruleCtx, foos.Kind, types.VerbReadNoSecrets); err != nil {
		return nil, trace.Wrap(err)
	}

	preAuthzRsp, err := s.cfg.Reader.GetFoo(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx.Resource153 = preAuthzRsp
	if err := authzContext.CheckerContext.Decision(ctx, preAuthzRsp.GetScope(), func(checker *services.ScopedAccessChecker) error {
		return checker.CheckAccessToRules(&ruleCtx, foos.Kind, types.VerbReadNoSecrets)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return foov1.GetFooResponse_builder{
		Foo: preAuthzRsp,
	}.Build(), nil
}

func (s *Service) ListFoos(ctx context.Context, req *foov1.ListFoosRequest) (*foov1.ListFoosResponse, error) {
	authzContext, err := s.cfg.ScopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// do a pre-check to weed out requests that definitely won't be authorized.
	ruleCtx := authzContext.RuleContext()
	if err := authzContext.CheckerContext.CheckMaybeHasAccessToRules(&ruleCtx, foos.Kind, types.VerbReadNoSecrets, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	// list method scope filters must use identity-based defaults per RFD 0229i
	req.SetScopeFilter(authzContext.CheckerContext.ResolveScopeFilter(req.GetScopeFilter()))

	stream := stream.FilterMap(
		s.cfg.Reader.RangeFoos(ctx, req, req.GetPageToken(), ""),
		func(foo *foov1.Foo) (*foov1.Foo, bool) {
			// Skip foos the caller is not authorized to see
			ruleCtx := authzContext.RuleContext()
			ruleCtx.Resource153 = foo
			if err := authzContext.CheckerContext.Decision(ctx, foo.GetScope(), func(checker *services.ScopedAccessChecker) error {
				return checker.CheckAccessToRules(&ruleCtx, foos.Kind, types.VerbReadNoSecrets, types.VerbList)
			}); err != nil {
				return nil, false
			}
			return foo, true
		},
	)
	page, nextPageToken, err := generic.CollectPageAndCursor(
		stream,
		int(req.GetPageSize()),
		foos.MakeCursor,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return foov1.ListFoosResponse_builder{
		Foos:          page,
		NextPageToken: nextPageToken,
	}.Build(), nil
}

func (s *Service) DeleteFoo(ctx context.Context, req *foov1.DeleteFooRequest) (*foov1.DeleteFooResponse, error) {
	authzContext, err := s.cfg.ScopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx := authzContext.RuleContext()

	// do a pre-check to weed out requests that definitely won't be authorized.
	if err := authzContext.CheckerContext.CheckMaybeHasAccessToRules(&ruleCtx, foos.Kind, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	getResp, err := s.cfg.Reader.GetFoo(ctx, foov1.GetFooRequest_builder{
		Name:  req.GetName(),
		Scope: req.GetScope(),
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx.Resource153 = getResp
	if err := authzContext.CheckerContext.Decision(ctx, getResp.GetScope(), func(checker *services.ScopedAccessChecker) error {
		return checker.CheckAccessToRules(&ruleCtx, foos.Kind, types.VerbDelete)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.cfg.Writer.DeleteFoo(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}

	return foov1.DeleteFooResponse_builder{}.Build(), nil
}
