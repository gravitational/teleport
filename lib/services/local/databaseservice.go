/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
	if err := service.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.MarshalDatabaseService(service)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(databaseServicePrefix, service.GetName()),
		Value:   value,
		Expires: service.Expiry(),
		ID:      service.GetResourceID(),
	}
	lease, err := s.Put(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if service.Expiry().IsZero() {
		return &types.KeepAlive{}, nil
	}
	return &types.KeepAlive{
		Type:      types.KeepAlive_DATABASE_SERVICE,
		LeaseID:   lease.ID,
		Namespace: apidefaults.Namespace,
		Name:      service.GetName(),
		HostID:    service.GetName(),
		Expires:   service.Expiry(),
	}, nil
}

// DeleteDatabaseService removes the specified DatabaseService resource.
func (s *DatabaseServicesService) DeleteDatabaseService(ctx context.Context, name string) error {
	err := s.Delete(ctx, backend.Key(databaseServicePrefix, name))
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
	startKey := backend.Key(databaseServicePrefix)
	err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

const (
	databaseServicePrefix = "databaseService"
)
