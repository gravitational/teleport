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

package local

import (
	"context"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

// DatabaseServicesService manages DatabaseService resources in the backend.
type DatabaseServicesService struct {
	backend.Backend
}

// NewDatabaseServicesService creates a new DatabaseServicesService.
func NewDatabaseServicesService(backend backend.Backend) *DatabaseServicesService {
	return &DatabaseServicesService{Backend: backend}
}

// UpsertDatabaseService creates or updates (by name) a DatabaseService resource.
func (s *DatabaseServicesService) UpsertDatabaseService(ctx context.Context, service types.DatabaseService) (*types.KeepAlive, error) {
	if err := services.CheckAndSetDefaults(service); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := types.ValidateNamespaceDefault(service.GetNamespace()); err != nil {
		return nil, trace.Wrap(err)
	}

	rev := service.GetRevision()
	value, err := services.MarshalDatabaseService(service)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(databaseServicePrefix, service.GetName()),
		Value:    value,
		Expires:  service.Expiry(),
		Revision: rev,
	}
	_, err = s.Put(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if service.Expiry().IsZero() {
		return &types.KeepAlive{}, nil
	}
	return &types.KeepAlive{
		Type:      types.KeepAlive_DATABASE_SERVICE,
		Namespace: apidefaults.Namespace,
		Name:      service.GetName(),
		HostID:    service.GetName(),
		Expires:   service.Expiry(),
	}, nil
}

// DeleteDatabaseService removes the specified DatabaseService resource.
func (s *DatabaseServicesService) DeleteDatabaseService(ctx context.Context, name string) error {
	err := s.Delete(ctx, backend.NewKey(databaseServicePrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("databaseService %q doesn't exist", name)
		}
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllDatabaseServices removes all DatabaseService resources.
func (s *DatabaseServicesService) DeleteAllDatabaseServices(ctx context.Context) error {
	startKey := backend.ExactKey(databaseServicePrefix)
	err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

const (
	databaseServicePrefix = "databaseService"
)
