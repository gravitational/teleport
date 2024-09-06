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

package dbobjectv1

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobject"
)

// Backend interface for manipulating DatabaseObject resources.
type Backend interface {
	services.DatabaseObjects
}

// DatabaseObjectServiceConfig holds configuration options for
// the DatabaseObjects gRPC service.
type DatabaseObjectServiceConfig struct {
	Authorizer authz.Authorizer
	Backend    Backend
	Logger     logrus.FieldLogger
}

// NewDatabaseObjectService returns a new instance of the DatabaseObjectService.
func NewDatabaseObjectService(cfg DatabaseObjectServiceConfig) (*DatabaseObjectService, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required")
	}
	if cfg.Backend == nil {
		return nil, trace.BadParameter("backend service is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = logrus.WithField(teleport.ComponentKey, "db_object")
	}
	return &DatabaseObjectService{
		logger:     cfg.Logger,
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
	}, nil
}

// DatabaseObjectService implements the teleport.dbobject.v1.DatabaseObjectService RPC service.
type DatabaseObjectService struct {
	// UnsafeDatabaseObjectServiceServer is embedded to opt out of forward compatibility for this service.
	// Added methods to DatabaseObjectServiceServer will result in compilation errors, which is what we want.
	pb.UnsafeDatabaseObjectServiceServer

	backend    Backend
	authorizer authz.Authorizer
	logger     logrus.FieldLogger
}

func (rs *DatabaseObjectService) authorize(ctx context.Context, adminAction bool, verb string, additionalVerbs ...string) error {
	authCtx, err := rs.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindDatabaseObject, verb, additionalVerbs...)
	if err != nil {
		return trace.Wrap(err)
	}

	if adminAction {
		err = authCtx.AuthorizeAdminAction()
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// GetDatabaseObject gets a DatabaseObject by name. It will return an error if the DatabaseObject does not exist.
func (rs *DatabaseObjectService) GetDatabaseObject(ctx context.Context, req *pb.GetDatabaseObjectRequest) (*pb.DatabaseObject, error) {
	err := rs.authorize(ctx, false, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Name == "" {
		return nil, trace.BadParameter("name: must be non-empty")
	}

	out, err := rs.backend.GetDatabaseObject(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// ListDatabaseObjects lists all DatabaseObjects.
func (rs *DatabaseObjectService) ListDatabaseObjects(
	ctx context.Context, req *pb.ListDatabaseObjectsRequest,
) (*pb.ListDatabaseObjectsResponse, error) {
	err := rs.authorize(ctx, false, types.VerbRead, types.VerbList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out, next, err := rs.backend.ListDatabaseObjects(ctx, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &pb.ListDatabaseObjectsResponse{
		Objects:       out,
		NextPageToken: next,
	}, nil
}

// CreateDatabaseObject creates a new DatabaseObject. It will return an error if the DatabaseObject already
// exists.
func (rs *DatabaseObjectService) CreateDatabaseObject(
	ctx context.Context, req *pb.CreateDatabaseObjectRequest,
) (*pb.DatabaseObject, error) {
	err := rs.authorize(ctx, true, types.VerbCreate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = databaseobject.ValidateDatabaseObject(req.Object)
	if err != nil {
		return nil, trace.Wrap(err, "validating object")
	}

	out, err := rs.backend.CreateDatabaseObject(ctx, req.Object)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil

}

// UpsertDatabaseObject creates a new DatabaseObject or forcefully updates an existing DatabaseObject.
// This is a function rather than a method so that it can be used by the gRPC service
// and the auth server init code when dealing with resources to be applied at startup.
func UpsertDatabaseObject(
	ctx context.Context,
	backend Backend,
	object *pb.DatabaseObject,
) (*pb.DatabaseObject, error) {
	if err := databaseobject.ValidateDatabaseObject(object); err != nil {
		return nil, trace.Wrap(err, "validating object")
	}
	out, err := backend.UpsertDatabaseObject(ctx, object)
	return out, trace.Wrap(err)
}

// UpdateDatabaseObject updates an existing DatabaseObject. It will throw an error if the DatabaseObject does
// not exist.
func (rs *DatabaseObjectService) UpdateDatabaseObject(
	ctx context.Context, req *pb.UpdateDatabaseObjectRequest,
) (*pb.DatabaseObject, error) {
	err := rs.authorize(ctx, true, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = databaseobject.ValidateDatabaseObject(req.Object)
	if err != nil {
		return nil, trace.Wrap(err, "validating object")
	}

	object, err := rs.backend.UpdateDatabaseObject(ctx, req.Object)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return object, nil
}

// UpsertDatabaseObject creates a new DatabaseObject or forcefully updates an existing DatabaseObject.
func (rs *DatabaseObjectService) UpsertDatabaseObject(ctx context.Context, req *pb.UpsertDatabaseObjectRequest) (*pb.DatabaseObject, error) {
	err := rs.authorize(ctx, true, types.VerbUpdate, types.VerbCreate)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	object, err := UpsertDatabaseObject(ctx, rs.backend, req.GetObject())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return object, nil
}

// DeleteDatabaseObject deletes an existing DatabaseObject. It will throw an error if the DatabaseObject does
// not exist.
func (rs *DatabaseObjectService) DeleteDatabaseObject(
	ctx context.Context, req *pb.DeleteDatabaseObjectRequest,
) (*emptypb.Empty, error) {
	err := rs.authorize(ctx, true, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = rs.backend.DeleteDatabaseObject(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}
