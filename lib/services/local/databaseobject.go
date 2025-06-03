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

	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

// DatabaseObjectService manages database objects in the backend.
type DatabaseObjectService struct {
	service *generic.ServiceWrapper[*dbobjectv1.DatabaseObject]
}

func (s *DatabaseObjectService) DeleteAllDatabaseObjects(ctx context.Context) error {
	return trace.Wrap(s.service.DeleteAllResources(ctx))
}

func (s *DatabaseObjectService) UpsertDatabaseObject(ctx context.Context, object *dbobjectv1.DatabaseObject) (*dbobjectv1.DatabaseObject, error) {
	out, err := s.service.UpsertResource(ctx, object)
	return out, trace.Wrap(err)
}

func (s *DatabaseObjectService) UpdateDatabaseObject(ctx context.Context, object *dbobjectv1.DatabaseObject) (*dbobjectv1.DatabaseObject, error) {
	out, err := s.service.UpdateResource(ctx, object)
	return out, trace.Wrap(err)
}

func (s *DatabaseObjectService) CreateDatabaseObject(ctx context.Context, object *dbobjectv1.DatabaseObject) (*dbobjectv1.DatabaseObject, error) {
	out, err := s.service.CreateResource(ctx, object)
	return out, trace.Wrap(err)
}

func (s *DatabaseObjectService) GetDatabaseObject(ctx context.Context, name string) (*dbobjectv1.DatabaseObject, error) {
	out, err := s.service.GetResource(ctx, name)
	return out, trace.Wrap(err)
}

func (s *DatabaseObjectService) DeleteDatabaseObject(ctx context.Context, name string) error {
	return trace.Wrap(s.service.DeleteResource(ctx, name))
}

func (s *DatabaseObjectService) ListDatabaseObjects(ctx context.Context, size int, pageToken string) ([]*dbobjectv1.DatabaseObject, string, error) {
	out, next, err := s.service.ListResources(ctx, size, pageToken)
	return out, next, trace.Wrap(err)
}

const (
	databaseObjectPrefix = "databaseObjectPrefix"
)

func NewDatabaseObjectService(b backend.Backend) (*DatabaseObjectService, error) {
	service, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*dbobjectv1.DatabaseObject]{
			Backend:       b,
			ResourceKind:  types.KindDatabaseObject,
			BackendPrefix: backend.NewKey(databaseObjectPrefix),
			//nolint:staticcheck // SA1019. Using this marshaler for json compatibility.
			MarshalFunc: services.FastMarshalProtoResourceDeprecated[*dbobjectv1.DatabaseObject],
			//nolint:staticcheck // SA1019. Using this unmarshaler for json compatibility.
			UnmarshalFunc: services.FastUnmarshalProtoResourceDeprecated[*dbobjectv1.DatabaseObject],
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &DatabaseObjectService{service: service}, nil
}
