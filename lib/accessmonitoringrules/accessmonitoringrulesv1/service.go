/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package accessmonitoringrulesv1

import (
	"context"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types"
	conv "github.com/gravitational/teleport/api/types/accessmonitoringrule/convert/v1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ServiceConfig holds configuration options for the access monitoring rules service.
type ServiceConfig struct {
	Backend    services.AccessMonitoringRules
	Authorizer authz.Authorizer
	Cache      Cache
}

// Cache is the subset of the cached resources that the service queries.
type Cache interface {
	services.AccessMonitoringRules
}

// Service implements the teleport.accessmonitoringrules.v1.AccessMonitoringRulesService RPC service.
type Service struct {
	accessmonitoringrulesv1.UnimplementedAccessMonitoringRulesServiceServer

	backend    services.AccessMonitoringRules
	authorizer authz.Authorizer
	cache      Cache
}

func NewService(cfg *ServiceConfig) (*Service, error) {
	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache is required")
	}

	return &Service{
		backend:    cfg.Backend,
		authorizer: cfg.Authorizer,
		cache:      cfg.Cache,
	}, nil
}

// CreateAccessMonitoringRule creates the specified access monitoring rule.
func (s *Service) CreateAccessMonitoringRule(ctx context.Context, req *accessmonitoringrulesv1.CreateAccessMonitoringRuleRequest) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindAccessMonitoringRule, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	amr, err := conv.FromProto(req.Rule)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	created, err := s.backend.CreateAccessMonitoringRule(ctx, amr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return conv.ToProto(created), nil
}

// UpdateAccessMonitoringRule updates the specified access monitoring rule.
func (s *Service) UpdateAccessMonitoringRule(ctx context.Context, req *accessmonitoringrulesv1.UpdateAccessMonitoringRuleRequest) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindAccessMonitoringRule, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	amr, err := conv.FromProto(req.Rule)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	created, err := s.backend.UpdateAccessMonitoringRule(ctx, amr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return conv.ToProto(created), nil
}

// GetAccessMonitoringRule gets the specified access monitoring rule.
func (s *Service) GetAccessMonitoringRule(ctx context.Context, req *accessmonitoringrulesv1.GetAccessMonitoringRuleRequest) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindAccessMonitoringRule, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	got, err := s.backend.GetAccessMonitoringRule(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return conv.ToProto(got), nil
}

// DeleteAccessMonitoringRule deletes the specified access monitoring rule.
func (s *Service) DeleteAccessMonitoringRule(ctx context.Context, req *accessmonitoringrulesv1.DeleteAccessMonitoringRuleRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindAccessMonitoringRule, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.backend.DeleteAccessMonitoringRule(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// DeleteAllAccessMonitoringRules deletes all access monitoring rules.
func (s *Service) DeleteAllAccessMonitoringRules(ctx context.Context) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindAccessMonitoringRule, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.backend.DeleteAllAccessMonitoringRules(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// UpsertAccessMonitoringRule upserts the specified access monitoring rule.
func (s *Service) UpsertAccessMonitoringRule(ctx context.Context, req *accessmonitoringrulesv1.UpsertAccessMonitoringRuleRequest) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindAccessMonitoringRule, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	amr, err := conv.FromProto(req.Rule)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	created, err := s.backend.UpsertAccessMonitoringRule(ctx, amr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return conv.ToProto(created), nil
}

// ListAccessMonitoringRule lists current access monitoring rules.
func (s *Service) ListAccessMonitoringRules(ctx context.Context, req *accessmonitoringrulesv1.ListAccessMonitoringRulesRequest) (*accessmonitoringrulesv1.ListAccessMonitoringRulesResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindAccessMonitoringRule, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	results, nextToken, err := s.cache.ListAccessMonitoringRules(ctx, int(req.PageSize), req.PageToken)
	amrs := make([]*accessmonitoringrulesv1.AccessMonitoringRule, len(results))
	for i, r := range results {
		amrs[i] = conv.ToProto(r)
	}
	return &accessmonitoringrulesv1.ListAccessMonitoringRulesResponse{
		Rules:         amrs,
		NextPageToken: nextToken,
	}, nil
}
