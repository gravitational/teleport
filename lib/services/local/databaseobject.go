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

package local

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/teleport/lib/utils"
)

// databaseObjectService manages database objects in the backend.
type databaseObjectService struct {
	service *generic.ServiceWrapper[*dbobjectv1.DatabaseObject]
}

var _ services.DatabaseObjects = (*databaseObjectService)(nil)

func (s *databaseObjectService) UpsertDatabaseObject(ctx context.Context, object *dbobjectv1.DatabaseObject) (*dbobjectv1.DatabaseObject, error) {
	out, err := s.service.UpsertResource(ctx, object)
	return out, trace.Wrap(err)
}

func (s *databaseObjectService) UpdateDatabaseObject(ctx context.Context, object *dbobjectv1.DatabaseObject) (*dbobjectv1.DatabaseObject, error) {
	out, err := s.service.UpdateResource(ctx, object)
	return out, trace.Wrap(err)
}

func (s *databaseObjectService) CreateDatabaseObject(ctx context.Context, object *dbobjectv1.DatabaseObject) (*dbobjectv1.DatabaseObject, error) {
	out, err := s.service.CreateResource(ctx, object)
	return out, trace.Wrap(err)
}

func (s *databaseObjectService) GetDatabaseObject(ctx context.Context, name string) (*dbobjectv1.DatabaseObject, error) {
	out, err := s.service.GetResource(ctx, name)
	return out, trace.Wrap(err)
}

func (s *databaseObjectService) DeleteDatabaseObject(ctx context.Context, name string) error {
	return trace.Wrap(s.service.DeleteResource(ctx, name))
}

func (s *databaseObjectService) ListDatabaseObjects(ctx context.Context, size int, pageToken string) ([]*dbobjectv1.DatabaseObject, string, error) {
	out, next, err := s.service.ListResources(ctx, size, pageToken)
	return out, next, trace.Wrap(err)
}

const (
	databaseObjectPrefix = "databaseObjectPrefix"
)

func NewDatabaseObjectService(backend backend.Backend) (services.DatabaseObjects, error) {
	service, err := generic.NewServiceWrapper(backend,
		types.KindDatabaseObject,
		databaseObjectPrefix,
		marshalDatabaseObject,
		unmarshalDatabaseObject)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &databaseObjectService{service: service}, nil
}

func marshalDatabaseObject(object *dbobjectv1.DatabaseObject, opts ...services.MarshalOption) ([]byte, error) {
	cfg, err := services.CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cfg.PreserveResourceID {
		object = proto.Clone(object).(*dbobjectv1.DatabaseObject)
		//nolint:staticcheck // SA1019. Deprecated, but still needed.
		object.Metadata.Id = 0
		object.Metadata.Revision = ""
	}
	data, err := utils.FastMarshal(object)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

func unmarshalDatabaseObject(data []byte, opts ...services.MarshalOption) (*dbobjectv1.DatabaseObject, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing DatabaseObject data")
	}
	cfg, err := services.CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var obj dbobjectv1.DatabaseObject
	err = utils.FastUnmarshal(data, &obj)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		//nolint:staticcheck // SA1019. Id is deprecated, but still needed.
		obj.Metadata.Id = cfg.ID
	}
	if cfg.Revision != "" {
		obj.Metadata.Revision = cfg.Revision
	}
	if !cfg.Expires.IsZero() {
		obj.Metadata.Expires = timestamppb.New(cfg.Expires)
	}
	return &obj, nil
}
