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
	"log/slog"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobjectimportrule"
)

// Backend interface for manipulating DatabaseObjectImportRule resources.
type Backend interface {
	services.DatabaseObjectImportRules
}

// DatabaseObjectImportRuleServiceConfig holds configuration options for
// the DatabaseObjectImportRules gRPC service.
type DatabaseObjectImportRuleServiceConfig struct {
	Authorizer authz.Authorizer
	Backend    Backend
	Logger     *slog.Logger
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
		cfg.Logger = slog.With(teleport.ComponentKey, "db_obj_import_rule")
	}
	return &DatabaseObjectImportRuleService{
		logger:     cfg.Logger,
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
	}, nil
}

// DatabaseObjectImportRuleService implements the teleport.dbobjectimportrule.v1.DatabaseObjectImportRuleService RPC service.
type DatabaseObjectImportRuleService struct {
	// UnsafeDatabaseObjectImportRuleServiceServer is embedded to opt out of forward compatibility for this service.
	// Added methods to DatabaseObjectImportRuleServiceServer will result in compilation errors, which is what we want.
	pb.UnsafeDatabaseObjectImportRuleServiceServer

	backend    Backend
	authorizer authz.Authorizer
	logger     *slog.Logger
}

func (rs *DatabaseObjectImportRuleService) authorize(ctx context.Context, adminAction bool, verb string, additionalVerbs ...string) error {
	authCtx, err := rs.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindDatabaseObjectImportRule, verb, additionalVerbs...)
	if err != nil {
		return trace.Wrap(err)
	}

	if adminAction {
		err = authCtx.AuthorizeAdminActionAllowReusedMFA()
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// GetDatabaseObjectImportRule gets a DatabaseObjectImportRule by name. It will return an error if the DatabaseObjectImportRule does not exist.
func (rs *DatabaseObjectImportRuleService) GetDatabaseObjectImportRule(ctx context.Context, req *pb.GetDatabaseObjectImportRuleRequest) (*pb.DatabaseObjectImportRule, error) {
	err := rs.authorize(ctx, false, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Name == "" {
		return nil, trace.BadParameter("name: must be non-empty")
	}

	out, err := rs.backend.GetDatabaseObjectImportRule(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// ListDatabaseObjectImportRules lists all DatabaseObjectImportRules.
func (rs *DatabaseObjectImportRuleService) ListDatabaseObjectImportRules(
	ctx context.Context, req *pb.ListDatabaseObjectImportRulesRequest,
) (*pb.ListDatabaseObjectImportRulesResponse, error) {
	err := rs.authorize(ctx, false, types.VerbRead, types.VerbList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out, next, err := rs.backend.ListDatabaseObjectImportRules(ctx, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &pb.ListDatabaseObjectImportRulesResponse{
		Rules:         out,
		NextPageToken: next,
	}, nil
}

// CreateDatabaseObjectImportRule creates a new DatabaseObjectImportRule. It will return an error if the DatabaseObjectImportRule already
// exists.
func (rs *DatabaseObjectImportRuleService) CreateDatabaseObjectImportRule(
	ctx context.Context, req *pb.CreateDatabaseObjectImportRuleRequest,
) (*pb.DatabaseObjectImportRule, error) {
	err := rs.authorize(ctx, true, types.VerbCreate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = databaseobjectimportrule.ValidateDatabaseObjectImportRule(req.Rule)
	if err != nil {
		return nil, trace.Wrap(err, "validating rule")
	}

	out, err := rs.backend.CreateDatabaseObjectImportRule(ctx, req.Rule)
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
) (*pb.DatabaseObjectImportRule, error) {
	if err := databaseobjectimportrule.ValidateDatabaseObjectImportRule(rule); err != nil {
		return nil, trace.Wrap(err, "validating rule")
	}
	out, err := backend.UpsertDatabaseObjectImportRule(ctx, rule)
	return out, trace.Wrap(err)
}

// UpdateDatabaseObjectImportRule updates an existing DatabaseObjectImportRule. It will throw an error if the DatabaseObjectImportRule does
// not exist.
func (rs *DatabaseObjectImportRuleService) UpdateDatabaseObjectImportRule(
	ctx context.Context, req *pb.UpdateDatabaseObjectImportRuleRequest,
) (*pb.DatabaseObjectImportRule, error) {
	err := rs.authorize(ctx, true, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = databaseobjectimportrule.ValidateDatabaseObjectImportRule(req.Rule)
	if err != nil {
		return nil, trace.Wrap(err, "validating rule")
	}

	rule, err := rs.backend.UpdateDatabaseObjectImportRule(ctx, req.Rule)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rule, nil
}

// UpsertDatabaseObjectImportRule creates a new DatabaseObjectImportRule or forcefully updates an existing DatabaseObjectImportRule.
func (rs *DatabaseObjectImportRuleService) UpsertDatabaseObjectImportRule(ctx context.Context, req *pb.UpsertDatabaseObjectImportRuleRequest) (*pb.DatabaseObjectImportRule, error) {
	err := rs.authorize(ctx, true, types.VerbUpdate, types.VerbCreate)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rule, err := UpsertDatabaseObjectImportRule(ctx, rs.backend, req.GetRule())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rule, nil
}

// DeleteDatabaseObjectImportRule deletes an existing DatabaseObjectImportRule. It will throw an error if the DatabaseObjectImportRule does
// not exist.
func (rs *DatabaseObjectImportRuleService) DeleteDatabaseObjectImportRule(
	ctx context.Context, req *pb.DeleteDatabaseObjectImportRuleRequest,
) (*emptypb.Empty, error) {
	err := rs.authorize(ctx, true, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = rs.backend.DeleteDatabaseObjectImportRule(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}
