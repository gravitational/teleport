/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package dbobjectimportrulev1

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/databaseobjectimportrule"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// Backend encapsulates required backend methods.
type Backend interface {
	services.DatabaseObjectImportRule
}

// DatabaseObjectImportRuleServiceConfig holds configuration options for
// the DatabaseObjectImportRules gRPC service.
type DatabaseObjectImportRuleServiceConfig struct {
	Authorizer authz.Authorizer
	Backend    Backend
	Logger     logrus.FieldLogger
}

// NewDatabaseObjectImportRuleService returns a new instance of the DatabaseObjectImportRuleService.
func NewDatabaseObjectImportRuleService(cfg DatabaseObjectImportRuleServiceConfig) (*DatabaseObjectImportRuleService, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required")
	}
	if cfg.Backend == nil {
		return nil, trace.BadParameter("backend service is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = logrus.WithField(trace.Component, "DatabaseObjectImportRule.service")
	}
	return &DatabaseObjectImportRuleService{
		logger:     cfg.Logger,
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
	}, nil
}

// DatabaseObjectImportRuleService implements the teleport.dbobjectimportrule.v1.DatabaseObjectImportRuleService RPC service.
type DatabaseObjectImportRuleService struct {
	pb.UnimplementedDatabaseObjectImportRuleServiceServer

	backend    Backend
	authorizer authz.Authorizer
	logger     logrus.FieldLogger
}

// GetDatabaseObjectImportRule gets a DatabaseObjectImportRule by name. It will throw an error if the DatabaseObjectImportRule does not exist.
func (bs *DatabaseObjectImportRuleService) GetDatabaseObjectImportRule(ctx context.Context, req *pb.GetDatabaseObjectImportRuleRequest) (*pb.DatabaseObjectImportRule, error) {
	_, err := authz.AuthorizeWithVerbs(
		ctx, bs.logger, bs.authorizer, false, types.KindDatabaseObjectImportRule, types.VerbRead,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if req.Name == "" {
		return nil, trace.BadParameter("name: must be non-empty")
	}
	out, err := bs.backend.GetDatabaseObjectImportRule(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// ListDatabaseObjectImportRules lists all DatabaseObjectImportRules.
func (bs *DatabaseObjectImportRuleService) ListDatabaseObjectImportRules(
	ctx context.Context, req *pb.ListDatabaseObjectImportRulesRequest,
) (*pb.ListDatabaseObjectImportRulesResponse, error) {
	_, err := authz.AuthorizeWithVerbs(
		ctx, bs.logger, bs.authorizer, false, types.KindDatabaseObjectImportRule, types.VerbList,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, next, err := bs.backend.ListDatabaseObjectImportRules(ctx, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &pb.ListDatabaseObjectImportRulesResponse{
		Rules:         out,
		NextPageToken: next,
	}, nil
}

// CreateDatabaseObjectImportRule creates a new DatabaseObjectImportRule. It will throw an error if the DatabaseObjectImportRule already
// exists.
func (bs *DatabaseObjectImportRuleService) CreateDatabaseObjectImportRule(
	ctx context.Context, req *pb.CreateDatabaseObjectImportRuleRequest,
) (*pb.DatabaseObjectImportRule, error) {
	_, err := authz.AuthorizeWithVerbs(
		ctx, bs.logger, bs.authorizer, false, types.KindDatabaseObjectImportRule, types.VerbCreate,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := databaseobjectimportrule.ValidateDatabaseObjectImportRule(req.Rule); err != nil {
		return nil, trace.Wrap(err, "validating rule")
	}
	out, err := bs.backend.CreateDatabaseObjectImportRule(ctx, req.Rule)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil

}

// UpsertDatabaseObjectImportRule creates a new DatabaseObjectImportRule or forcefully updates an existing DatabaseObjectImportRule.
// This is a function rather than a method so that it can be used by the gRPC service
// and the auth server init code when dealing with resources to be applied at startup.
func UpsertDatabaseObjectImportRule(
	ctx context.Context,
	backend Backend,
	rule *pb.DatabaseObjectImportRule,
) error {
	if err := databaseobjectimportrule.ValidateDatabaseObjectImportRule(rule); err != nil {
		return trace.Wrap(err, "validating rule")
	}
	return trace.Wrap(backend.UpsertDatabaseObjectImportRule(ctx, rule), "upserting rule")
}

// UpdateDatabaseObjectImportRule updates an existing DatabaseObjectImportRule. It will throw an error if the DatabaseObjectImportRule does
// not exist.
func (bs *DatabaseObjectImportRuleService) UpdateDatabaseObjectImportRule(
	ctx context.Context, req *pb.UpdateDatabaseObjectImportRuleRequest,
) (*pb.DatabaseObjectImportRule, error) {
	authCtx, err := authz.AuthorizeWithVerbs(
		ctx, bs.logger, bs.authorizer, false, types.KindDatabaseObjectImportRule, types.VerbUpdate,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authz.AuthorizeAdminAction(ctx, authCtx); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := databaseobjectimportrule.ValidateDatabaseObjectImportRule(req.Rule); err != nil {
		return nil, trace.Wrap(err, "validating rule")
	}

	rule, err := bs.backend.UpdateDatabaseObjectImportRule(ctx, req.Rule)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rule, nil
}

// DeleteDatabaseObjectImportRule deletes an existing DatabaseObjectImportRule. It will throw an error if the DatabaseObjectImportRule does
// not exist.
func (bs *DatabaseObjectImportRuleService) DeleteDatabaseObjectImportRule(
	ctx context.Context, req *pb.DeleteDatabaseObjectImportRuleRequest,
) (*emptypb.Empty, error) {
	authCtx, err := authz.AuthorizeWithVerbs(
		ctx, bs.logger, bs.authorizer, false, types.KindDatabaseObjectImportRule, types.VerbDelete,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = authz.AuthorizeAdminAction(ctx, authCtx); err != nil {
		return nil, trace.Wrap(err)
	}
	err = bs.backend.DeleteDatabaseObjectImportRule(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}
